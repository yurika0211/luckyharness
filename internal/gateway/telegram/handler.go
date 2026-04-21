package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/gateway"
)

// Handler processes Telegram bot commands and messages with per-chat session management.
type Handler struct {
	adapter *Adapter
	agent   *agent.Agent

	mu       sync.RWMutex
	sessions map[string]string // chatID → sessionID
}

// NewHandler creates a new Telegram command handler.
func NewHandler(adapter *Adapter, a *agent.Agent) *Handler {
	return &Handler{
		adapter:  adapter,
		agent:    a,
		sessions: make(map[string]string),
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
	return sess.ID
}

// resetSession creates a new session for the chat, discarding the old one.
func (h *Handler) resetSession(chatID string) string {
	sess := h.agent.Sessions().New()
	h.mu.Lock()
	h.sessions[chatID] = sess.ID
	h.mu.Unlock()
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

	// Regular text in private chats → forward to Agent
	if msg.Chat.Type == gateway.ChatPrivate {
		return h.handleChat(ctx, msg, msg.Text)
	}

	// Group chats: only respond if mentioned or replied to (already filtered by adapter)
	return h.handleChat(ctx, msg, msg.Text)
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
/reset — Reset conversation
/history — Show conversation history
/session — Show current session info
/help — Show this help

You can also just type a message directly!`

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
/reset — Reset conversation
/history — Show conversation history
/session — Show current session info

*Tips:*
• In private chats, just type your message directly
• In groups, mention @bot or reply to a bot message
• Each chat has its own conversation session`

	return h.adapter.Send(ctx, msg.Chat.ID, help)
}

// handleChat sends a message to the agent and returns the response.
// Uses per-chat session management for multi-turn context.
func (h *Handler) handleChat(ctx context.Context, msg *gateway.Message, text string) error {
	if strings.TrimSpace(text) == "" {
		return h.adapter.Send(ctx, msg.Chat.ID, "Please provide a message. Usage: /chat <message>")
	}

	sessionID := h.getSessionID(msg.Chat.ID)

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

	if msg.ReplyTo != nil {
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