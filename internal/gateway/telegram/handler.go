package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/cron"
	"github.com/yurika0211/luckyharness/internal/gateway"
)

// Handler processes Telegram bot commands and messages with per-chat session management.
type Handler struct {
	adapter *Adapter
	agent   *agent.Agent

	mu       sync.RWMutex
	sessions map[string]string // chatID → sessionID

	// v0.44.0: chatID→sessionID 映射持久化
	dataDir string
}

// chatSessionsData 是持久化的 chatID→sessionID 映射
type chatSessionsData struct {
	ChatSessions map[string]string `json:"chat_sessions"`
}

// NewHandler creates a new Telegram command handler.
func NewHandler(adapter *Adapter, a *agent.Agent) *Handler {
	return &Handler{
		adapter:  adapter,
		agent:    a,
		sessions: make(map[string]string),
		dataDir:  "", // 默认不持久化，需 SetDataDir 启用
	}
}

// SetDataDir 设置数据目录并从磁盘恢复 chatID→sessionID 映射
func (h *Handler) SetDataDir(dir string) {
	h.mu.Lock()
	h.dataDir = dir
	h.mu.Unlock()

	// 确保目录存在
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Printf("[telegram] warning: failed to create data dir %s: %v\n", dir, err)
		return
	}

	// 从磁盘恢复映射
	h.loadChatSessions()
}

// chatSessionsPath 返回持久化文件路径
func (h *Handler) chatSessionsPath() string {
	if h.dataDir == "" {
		return ""
	}
	return filepath.Join(h.dataDir, "chat_sessions.json")
}

// loadChatSessions 从磁盘加载 chatID→sessionID 映射
func (h *Handler) loadChatSessions() {
	path := h.chatSessionsPath()
	if path == "" {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// 文件不存在是正常的
		return
	}

	var csd chatSessionsData
	if err := json.Unmarshal(data, &csd); err != nil {
		fmt.Printf("[telegram] warning: failed to parse chat_sessions.json: %v\n", err)
		return
	}

	h.mu.Lock()
	for chatID, sessionID := range csd.ChatSessions {
		// 验证 session 是否还存在
		if _, ok := h.agent.Sessions().Get(sessionID); ok {
			h.sessions[chatID] = sessionID
		}
	}
	h.mu.Unlock()

	fmt.Printf("[telegram] restored %d chat→session mappings from disk\n", len(h.sessions))
}

// saveChatSessions 持久化 chatID→sessionID 映射到磁盘
func (h *Handler) saveChatSessions() {
	path := h.chatSessionsPath()
	if path == "" {
		return
	}

	h.mu.RLock()
	csd := chatSessionsData{
		ChatSessions: make(map[string]string, len(h.sessions)),
	}
	for k, v := range h.sessions {
		csd.ChatSessions[k] = v
	}
	h.mu.RUnlock()

	data, err := json.MarshalIndent(csd, "", "  ")
	if err != nil {
		fmt.Printf("[telegram] warning: failed to marshal chat_sessions: %v\n", err)
		return
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		fmt.Printf("[telegram] warning: failed to write chat_sessions.json: %v\n", err)
	}
}

// getSessionID returns the session ID for a chat, creating one if needed.
func (h *Handler) getSessionID(chatID string) string {
	h.mu.RLock()
	if sid, ok := h.sessions[chatID]; ok {
		h.mu.RUnlock()
		return sid
	}
	h.mu.RUnlock()

	// Create new session via agent
	sess := h.agent.Sessions().New()
	h.mu.Lock()
	h.sessions[chatID] = sess.ID
	h.mu.Unlock()
	h.saveChatSessions()
	return sess.ID
}

// resetSession creates a new session for the chat, discarding the old one.
func (h *Handler) resetSession(chatID string) string {
	sess := h.agent.Sessions().New()
	h.mu.Lock()
	h.sessions[chatID] = sess.ID
	h.mu.Unlock()
	h.saveChatSessions()
	return sess.ID
}

// hasSession checks if a chat already has an assigned session.
func (h *Handler) hasSession(chatID string) bool {
	h.mu.RLock()
	_, ok := h.sessions[chatID]
	h.mu.RUnlock()
	return ok
}

// setSessionID directly sets the session ID for a chat (for testing).
func (h *Handler) setSessionID(chatID, sessionID string) {
	h.mu.Lock()
	h.sessions[chatID] = sessionID
	h.mu.Unlock()
}

// HandleMessage processes an incoming gateway message.
func (h *Handler) HandleMessage(ctx context.Context, msg *gateway.Message) error {
	if msg.IsCommand {
		return h.handleCommand(ctx, msg)
	}

	// v0.36.0: 如果有附件，构造多媒体描述
	inputText := msg.Text
	if len(msg.Attachments) > 0 {
		var mediaDesc strings.Builder
		mediaDesc.WriteString(inputText)
		if inputText != "" {
			mediaDesc.WriteString("\n\n")
		}
		mediaDesc.WriteString("[多媒体内容]\n")
		for i, att := range msg.Attachments {
			switch att.Type {
			case gateway.AttachmentImage:
				mediaDesc.WriteString(fmt.Sprintf("📷 图片 %d: %s (URL: %s)\n", i+1, att.FileName, att.FileURL))
			case gateway.AttachmentAudio:
				mediaDesc.WriteString(fmt.Sprintf("🎤 语音 %d: %s (URL: %s)\n", i+1, att.FileName, att.FileURL))
			case gateway.AttachmentVideo:
				mediaDesc.WriteString(fmt.Sprintf("🎬 视频 %d: %s (URL: %s)\n", i+1, att.FileName, att.FileURL))
			case gateway.AttachmentDocument:
				mediaDesc.WriteString(fmt.Sprintf("📎 文件 %d: %s (%s, URL: %s)\n", i+1, att.FileName, att.MimeType, att.FileURL))
			}
		}
		inputText = mediaDesc.String()
	}

	// Regular text in private chats → forward to Agent
	if msg.Chat.Type == gateway.ChatPrivate {
		return h.handleChat(ctx, msg, inputText)
	}

	// Group chats: only respond if mentioned or replied to (already filtered by adapter)
	return h.handleChat(ctx, msg, inputText)
}

// handleCommand dispatches bot commands.
func (h *Handler) handleCommand(ctx context.Context, msg *gateway.Message) error {
	switch msg.Command {
	case "start":
		return h.handleStart(ctx, msg)
	case "help":
		return h.handleHelp(ctx, msg)
	case "chat":
		return h.handleChat(ctx, msg, msg.Args)
	case "model":
		return h.handleModel(ctx, msg)
	case "soul":
		return h.handleSoul(ctx, msg)
	case "tools":
		return h.handleTools(ctx, msg)
	case "reset":
		return h.handleReset(ctx, msg)
	case "history":
		return h.handleHistory(ctx, msg)
	case "session":
		return h.handleSession(ctx, msg)
	// v0.36.0: 新增命令
	case "skills":
		return h.handleSkills(ctx, msg)
	case "cron":
		return h.handleCron(ctx, msg)
	case "metrics":
		return h.handleMetrics(ctx, msg)
	case "health":
		return h.handleHealth(ctx, msg)
	default:
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("Unknown command: /%s\nType /help for available commands.", msg.Command))
	}
}

// handleStart sends a welcome message.
func (h *Handler) handleStart(ctx context.Context, msg *gateway.Message) error {
	welcome := `🍀 *LuckyHarness Bot*

I'm an AI assistant powered by LuckyHarness.

*Available commands:*
/chat _message_ — Send a message to the AI
/model — Show current model
/soul — Show current SOUL info
/tools — List available tools
/skills — List loaded skills
/cron — Manage scheduled tasks
/metrics — Show usage metrics
/health — System health check
/reset — Reset conversation
/history — Show conversation history
/session — Show current session info
/help — Show this help

You can also just type a message directly!
Send me photos, voice messages, or files!`

	return h.adapter.Send(ctx, msg.Chat.ID, welcome)
}

// handleHelp lists available commands.
func (h *Handler) handleHelp(ctx context.Context, msg *gateway.Message) error {
	help := `*Available Commands:*

/start — Welcome message
/help — This help message
/chat _message_ — Send a message to the AI
/model \[name] — Get/set current model
/soul — Show current SOUL info
/tools — List available tools
/skills — List loaded skills
/cron \[list|add|remove] — Manage scheduled tasks
/metrics — Show usage metrics
/health — System health check
/reset — Reset conversation
/history — Show conversation history
/session — Show current session info

*Tips:*
• In private chats, just type your message directly
• In groups, mention @bot or reply to a bot message
• Each chat has its own conversation session
• Send photos, voice, or files for multimodal processing`

	return h.adapter.Send(ctx, msg.Chat.ID, help)
}

// handleChat sends a message to the agent and returns the response.
// Uses streaming output with thinking/tool-call visualization when available.
func (h *Handler) handleChat(ctx context.Context, msg *gateway.Message, text string) error {
	if strings.TrimSpace(text) == "" {
		return h.adapter.Send(ctx, msg.Chat.ID, "Please provide a message. Usage: /chat <message>")
	}

	// 收到消息后给用户点赞 👍
	go h.adapter.ReactToMessage(msg.Chat.ID, msg.ID, "👍")

	sessionID := h.getSessionID(msg.Chat.ID)

	// 群聊中在输入文本前加上发送者名字，让 agent 知道是谁在说话
	inputText := text
	if msg.IsGroupTrigger && msg.Sender.DisplayName() != "" {
		inputText = fmt.Sprintf("[%s]: %s", msg.Sender.DisplayName(), text)
	}

	// 尝试流式输出（Adapter 已实现 StreamGateway）
	sender, err := h.adapter.SendStream(ctx, msg.Chat.ID, msg.ID)
	if err == nil {
		return h.handleChatStream(ctx, sender, msg, inputText, sessionID)
	}
	// SendStream 失败，回退到非流式

	// 回退到非流式
	return h.handleChatSync(ctx, msg, inputText, sessionID)
}

// handleChatStream 流式对话处理（Telegram 专用）
func (h *Handler) handleChatStream(ctx context.Context, sender gateway.StreamSender, msg *gateway.Message, text, sessionID string) error {
	// 启动 typing indicator（每 5 秒刷新一次，直到完成）
	typingCtx, typingCancel := context.WithCancel(context.Background())
	defer typingCancel()
	go h.adapter.SendTypingLoop(typingCtx, msg.Chat.ID)

	// 启动流式对话
	events, err := h.agent.ChatWithSessionStream(ctx, sessionID, text)
	if err != nil {
		// session 可能坏了，重试
		if strings.Contains(err.Error(), "session not found") {
			h.resetSession(msg.Chat.ID)
			sessionID = h.getSessionID(msg.Chat.ID)
			events, err = h.agent.ChatWithSessionStream(ctx, sessionID, text)
		}
		if err != nil {
			sender.SetResult(fmt.Sprintf("❌ Error: %s", truncateString(err.Error(), 200)))
			sender.Finish()
			return nil
		}
	}

	var finalContent strings.Builder
	toolCallCount := 0

	for evt := range events {
		switch evt.Type {
		case agent.ChatEventThinking:
			// 思考状态作为消息前缀展示，不清空已有内容
			sender.SetThinking(evt.Content)

		case agent.ChatEventToolCall:
			toolCallCount++
			// 工具调用作为消息前缀展示
			sender.SetToolCall(evt.Name, evt.Args)

		case agent.ChatEventToolResult:
			// 工具结果展示后切回思考状态
			sender.SetThinking(fmt.Sprintf("Continuing... (%d tools used)", toolCallCount))

		case agent.ChatEventContent:
			// 内容流式追加
			finalContent.WriteString(evt.Content)
			sender.Append(evt.Content)

		case agent.ChatEventDone:
			if finalContent.Len() == 0 {
				finalContent.WriteString(evt.Content)
			}
			// 最终结果替换整个消息
			sender.SetResult(finalContent.String())
			sender.Finish()

		case agent.ChatEventError:
			errMsg := evt.Err.Error()
			if len(errMsg) > 200 {
				errMsg = errMsg[:197] + "..."
			}
			sender.SetResult(fmt.Sprintf("❌ Error: %s", errMsg))
			sender.Finish()
		}
	}

	return nil
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// handleChatSync 非流式对话处理（回退方案）
func (h *Handler) handleChatSync(ctx context.Context, msg *gateway.Message, text, sessionID string) error {
	// 启动 typing indicator
	typingCtx, typingCancel := context.WithCancel(context.Background())
	defer typingCancel()
	go h.adapter.SendTypingLoop(typingCtx, msg.Chat.ID)

	response, err := h.agent.ChatWithSession(ctx, sessionID, text)
	if err != nil {
		// If session is broken, try with a fresh session
		if strings.Contains(err.Error(), "session not found") {
			h.resetSession(msg.Chat.ID)
			sessionID = h.getSessionID(msg.Chat.ID)
			response, err = h.agent.ChatWithSession(ctx, sessionID, text)
		}
		if err != nil {
			errMsg := fmt.Sprintf("❌ Error: %s", err.Error())
			if len(errMsg) > 200 {
				errMsg = errMsg[:200] + "..."
			}
			return h.adapter.Send(ctx, msg.Chat.ID, errMsg)
		}
	}

	// 群聊中始终 reply to 原消息，方便上下文追踪
	if msg.Chat.Type != gateway.ChatPrivate {
		return h.adapter.SendWithReply(ctx, msg.Chat.ID, msg.ID, response)
	}
	return h.adapter.Send(ctx, msg.Chat.ID, response)
}

// handleModel shows or sets the current model.
func (h *Handler) handleModel(ctx context.Context, msg *gateway.Message) error {
	if msg.Args == "" {
		// Show current model
		cfg := h.agent.Config().Get()
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("Current model: %s (provider: %s)", cfg.Model, cfg.Provider))
	}

	// Set model
	if err := h.agent.SwitchModel(msg.Args); err != nil {
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("❌ Failed to switch model: %s", err.Error()))
	}

	return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("✅ Switched to model: %s", msg.Args))
}

// handleSoul shows the current SOUL info.
func (h *Handler) handleSoul(ctx context.Context, msg *gateway.Message) error {
	s := h.agent.Soul()
	if s == nil {
		return h.adapter.Send(ctx, msg.Chat.ID, "No SOUL configured.")
	}

	prompt := s.SystemPrompt()
	if len(prompt) > 500 {
		prompt = prompt[:500] + "..."
	}

	return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("🧠 *Current SOUL:*\n\n%s", prompt))
}

// handleTools lists available tools.
func (h *Handler) handleTools(ctx context.Context, msg *gateway.Message) error {
	tools := h.agent.Tools()
	allTools := tools.List()

	if len(allTools) == 0 {
		return h.adapter.Send(ctx, msg.Chat.ID, "No tools available.")
	}

	var sb strings.Builder
	sb.WriteString("🔧 *Available Tools:*\n\n")

	for i, t := range allTools {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n... and %d more", len(allTools)-20))
			break
		}
		status := "✅"
		if !t.Enabled {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s %s — %s\n", status, t.Name, t.Description))
	}

	return h.adapter.Send(ctx, msg.Chat.ID, sb.String())
}

// handleReset resets the conversation for this chat.
func (h *Handler) handleReset(ctx context.Context, msg *gateway.Message) error {
	newID := h.resetSession(msg.Chat.ID)
	return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("🔄 Conversation reset. New session: `%s`", newID[:8]))
}

// handleHistory shows the conversation history for this chat.
func (h *Handler) handleHistory(ctx context.Context, msg *gateway.Message) error {
	sessionID := h.getSessionID(msg.Chat.ID)

	sess, ok := h.agent.Sessions().Get(sessionID)
	if !ok {
		return h.adapter.Send(ctx, msg.Chat.ID, "No conversation history.")
	}

	messages := sess.GetMessages()
	if len(messages) == 0 {
		return h.adapter.Send(ctx, msg.Chat.ID, "No messages in this session yet.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📜 *History* (%d messages):\n\n", len(messages)))

	maxShow := 20
	start := 0
	if len(messages) > maxShow {
		start = len(messages) - maxShow
		sb.WriteString(fmt.Sprintf("_(showing last %d of %d)_\n\n", maxShow, len(messages)))
	}

	for i := start; i < len(messages); i++ {
		m := messages[i]
		role := ""
		switch m.Role {
		case "user":
			role = "👤"
		case "assistant":
			role = "🤖"
		case "tool":
			role = "🔧"
		default:
			role = "💬"
		}

		content := m.Content
		if len(content) > 80 {
			content = content[:80] + "..."
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", role, content))
	}

	return h.adapter.Send(ctx, msg.Chat.ID, sb.String())
}

// handleSession shows current session info.
func (h *Handler) handleSession(ctx context.Context, msg *gateway.Message) error {
	h.mu.RLock()
	sessionID, ok := h.sessions[msg.Chat.ID]
	h.mu.RUnlock()

	if !ok {
		return h.adapter.Send(ctx, msg.Chat.ID, "No active session. Send a message to start one!")
	}

	sess, ok := h.agent.Sessions().Get(sessionID)
	if !ok {
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("Session `%s` not found. It may have been cleaned up.", sessionID[:8]))
	}

	info := fmt.Sprintf("📋 *Session Info:*\n\n• ID: `%s`\n• Title: %s\n• Messages: %d\n• Created: %s\n• Updated: %s",
		sessionID[:8],
		sess.Title,
		sess.MessageCount(),
		sess.CreatedAt.Format("2006-01-02 15:04"),
		sess.UpdatedAt.Format("2006-01-02 15:04"),
	)

	return h.adapter.Send(ctx, msg.Chat.ID, info)
}

// handleSkills lists loaded skills.
func (h *Handler) handleSkills(ctx context.Context, msg *gateway.Message) error {
	skills := h.agent.Skills()
	if len(skills) == 0 {
		return h.adapter.Send(ctx, msg.Chat.ID, "No skills loaded.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎯 *Loaded Skills* (%d):\n\n", len(skills)))

	maxShow := 30
	for i, s := range skills {
		if i >= maxShow {
			sb.WriteString(fmt.Sprintf("\n... and %d more", len(skills)-maxShow))
			break
		}
		desc := s.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		sb.WriteString(fmt.Sprintf("• %s — %s\n", s.Name, desc))
	}

	return h.adapter.Send(ctx, msg.Chat.ID, sb.String())
}

// handleCron manages scheduled tasks.
func (h *Handler) handleCron(ctx context.Context, msg *gateway.Message) error {
	engine := h.agent.CronEngine()
	args := strings.TrimSpace(msg.Args)

	if args == "" || args == "list" {
		// List all cron jobs
		jobs := engine.ListJobs()
		if len(jobs) == 0 {
			return h.adapter.Send(ctx, msg.Chat.ID, "⏰ No scheduled tasks. Use /cron add <name> <interval|cron> <prompt>")
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("⏰ *Scheduled Tasks* (%d):\n\n", len(jobs)))
		for _, job := range jobs {
			status := "🟢"
			switch job.Status {
			case cron.StatusRunning:
				status = "🔵"
			case cron.StatusPaused:
				status = "🟡"
			case cron.StatusFailed:
				status = "🔴"
			}
			sb.WriteString(fmt.Sprintf("%s %s — %s\n  Schedule: %s | Runs: %d\n", status, job.ID, job.Name, job.Schedule, job.RunCount))
		}
		return h.adapter.Send(ctx, msg.Chat.ID, sb.String())
	}

	parts := strings.Fields(args)
	if len(parts) < 1 {
		return h.adapter.Send(ctx, msg.Chat.ID, "Usage: /cron [list|add|remove|pause|resume]")
	}

	switch parts[0] {
	case "add":
		if len(parts) < 4 {
			return h.adapter.Send(ctx, msg.Chat.ID, "Usage: /cron add <id> <name> <interval_seconds> <prompt>")
		}
		id := parts[1]
		name := parts[2]
		var intervalSec int
		if _, err := fmt.Sscanf(parts[3], "%d", &intervalSec); err != nil || intervalSec <= 0 {
			return h.adapter.Send(ctx, msg.Chat.ID, "❌ Invalid interval. Must be positive integer (seconds).")
		}
		prompt := strings.Join(parts[4:], " ")
		if prompt == "" {
			prompt = name
		}

		err := engine.AddJobWithMeta(id, name, prompt, cron.IntervalSchedule{Interval: time.Duration(intervalSec) * time.Second}, func() error {
			// 执行时调用 agent chat，并把结果发回创建任务的聊天
			chatCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			response, err := h.agent.Chat(chatCtx, prompt)
			if err != nil {
				// 执行失败，通知聊天
				_ = h.adapter.Send(context.Background(), msg.Chat.ID,
					fmt.Sprintf("⏰ 定时任务 [%s] 执行失败: %s", name, truncateString(err.Error(), 200)))
				return err
			}

			// 执行成功，把 agent 回复发到聊天
			if response != "" {
				resultMsg := fmt.Sprintf("⏰ 定时任务 [%s] 执行结果:\n\n%s", name, response)
				_ = h.adapter.Send(context.Background(), msg.Chat.ID, resultMsg)
			}
			return nil
		}, map[string]string{
			"chatID":      msg.Chat.ID,
			"triggerType": "cron",
		})
		if err != nil {
			return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("❌ Failed to add job: %s", err.Error()))
		}
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("✅ Job `%s` added: %s (every %ds)", id, name, intervalSec))

	case "remove":
		if len(parts) < 2 {
			return h.adapter.Send(ctx, msg.Chat.ID, "Usage: /cron remove <id>")
		}
		if err := engine.RemoveJob(parts[1]); err != nil {
			return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		}
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("✅ Job `%s` removed", parts[1]))

	case "pause":
		if len(parts) < 2 {
			return h.adapter.Send(ctx, msg.Chat.ID, "Usage: /cron pause <id>")
		}
		if err := engine.PauseJob(parts[1]); err != nil {
			return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		}
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("⏸ Job `%s` paused", parts[1]))

	case "resume":
		if len(parts) < 2 {
			return h.adapter.Send(ctx, msg.Chat.ID, "Usage: /cron resume <id>")
		}
		if err := engine.ResumeJob(parts[1]); err != nil {
			return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		}
		return h.adapter.Send(ctx, msg.Chat.ID, fmt.Sprintf("▶️ Job `%s` resumed", parts[1]))

	default:
		return h.adapter.Send(ctx, msg.Chat.ID, "Usage: /cron [list|add|remove|pause|resume]")
	}
}

// handleMetrics shows usage metrics.
func (h *Handler) handleMetrics(ctx context.Context, msg *gateway.Message) error {
	m := h.agent.Metrics()
	snapshot := m.Snapshot()

	info := fmt.Sprintf("📊 *Metrics:*\n\n• Total requests: %d\n• Tool calls: %d\n• Errors: %d\n• Uptime: %s",
		snapshot.TotalRequests,
		snapshot.ToolCalls,
		snapshot.ErrorRequests,
		snapshot.Uptime,
	)

	return h.adapter.Send(ctx, msg.Chat.ID, info)
}

// handleHealth shows system health.
func (h *Handler) handleHealth(ctx context.Context, msg *gateway.Message) error {
	var sb strings.Builder
	sb.WriteString("🏥 *System Health:*\n\n")

	// Agent 状态
	sb.WriteString("• Agent: ✅ Running\n")

	// Cron 引擎
	cronEngine := h.agent.CronEngine()
	if cronEngine.IsRunning() {
		sb.WriteString(fmt.Sprintf("• Cron Engine: ✅ Running (%d jobs)\n", cronEngine.JobCount()))
	} else {
		sb.WriteString("• Cron Engine: ❌ Stopped\n")
	}

	// Skills
	skills := h.agent.Skills()
	sb.WriteString(fmt.Sprintf("• Skills: ✅ %d loaded\n", len(skills)))

	// Sessions
	sb.WriteString("• Sessions: ✅ Active\n")

	// Memory
	mem := h.agent.Memory()
	if mem != nil {
		sb.WriteString("• Memory: ✅ Active\n")
	}

	// Metrics
	m := h.agent.Metrics()
	snapshot := m.Snapshot()
	sb.WriteString(fmt.Sprintf("• Total requests: %d\n", snapshot.TotalRequests))

	return h.adapter.Send(ctx, msg.Chat.ID, sb.String())
}