package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/yurika0211/luckyharness/internal/gateway"
)

// Adapter implements gateway.Gateway for Telegram.
type Adapter struct {
	cfg     Config
	bot     *tgbotapi.BotAPI
	running bool
	cancel  context.CancelFunc

	mu        sync.RWMutex
	handler   gateway.MessageHandler
	rateLimit map[string]*rateBucket

	// Bot username for mention detection
	botUsername string
}

// rateBucket implements simple per-chat rate limiting.
type rateBucket struct {
	lastSent time.Time
}

// NewAdapter creates a new Telegram adapter.
func NewAdapter(cfg Config) *Adapter {
	if cfg.MaxMessageLen <= 0 {
		cfg.MaxMessageLen = 4000
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 1
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = 30
	}

	return &Adapter{
		cfg:       cfg,
		rateLimit: make(map[string]*rateBucket),
	}
}

// Name returns the platform name.
func (a *Adapter) Name() string {
	return "telegram"
}

// SetHandler sets the message handler callback.
func (a *Adapter) SetHandler(handler gateway.MessageHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handler = handler
}

// Start connects to Telegram and begins polling for updates.
func (a *Adapter) Start(ctx context.Context) error {
	if a.cfg.Token == "" {
		return fmt.Errorf("telegram: bot token is required")
	}

	bot, err := tgbotapi.NewBotAPI(a.cfg.Token)
	if err != nil {
		return fmt.Errorf("telegram: create bot: %w", err)
	}

	a.bot = bot
	a.botUsername = bot.Self.UserName

	// Create cancellable context for the polling loop
	pollCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.running = true

	// Start polling in background
	go a.poll(pollCtx)

	return nil
}

// Stop gracefully shuts down the adapter.
func (a *Adapter) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.running = false
	return nil
}

// Send sends a message to a chat, splitting if necessary.
func (a *Adapter) Send(ctx context.Context, chatID string, message string) error {
	if !a.running || a.bot == nil {
		return fmt.Errorf("telegram: adapter not running")
	}

	chunks := a.splitMessage(message)
	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %q: %w", chatID, err)
	}

	for _, chunk := range chunks {
		if err := a.sendChunk(ctx, chatIDInt, 0, chunk); err != nil {
			return err
		}
		// Rate limit between chunks
		a.waitRateLimit(chatID)
	}

	return nil
}

// SendWithReply sends a message as a reply to a specific message.
func (a *Adapter) SendWithReply(ctx context.Context, chatID string, replyToMsgID string, message string) error {
	if !a.running || a.bot == nil {
		return fmt.Errorf("telegram: adapter not running")
	}

	chunks := a.splitMessage(message)
	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %q: %w", chatID, err)
	}

	replyToID, err := strconv.Atoi(replyToMsgID)
	if err != nil {
		return fmt.Errorf("telegram: invalid reply-to message ID %q: %w", replyToMsgID, err)
	}

	for i, chunk := range chunks {
		replyID := 0
		if i == 0 {
			replyID = replyToID
		}
		if err := a.sendChunk(ctx, chatIDInt, replyID, chunk); err != nil {
			return err
		}
		a.waitRateLimit(chatID)
	}

	return nil
}

// IsRunning returns whether the adapter is currently connected.
func (a *Adapter) IsRunning() bool {
	return a.running
}

// SendTypingLoop 持续发送 typing indicator，直到 ctx 被取消。
// Telegram 的 typing 状态持续 5 秒，所以每 4.5 秒刷新一次。
func (a *Adapter) SendTypingLoop(ctx context.Context, chatID string) {
	if a.bot == nil {
		return
	}
	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return
	}

	ticker := time.NewTicker(4500 * time.Millisecond)
	defer ticker.Stop()

	// 立即发一次
	a.sendTypingOnce(chatIDInt)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sendTypingOnce(chatIDInt)
		}
	}
}

// sendTypingOnce 发送一次 typing action
func (a *Adapter) sendTypingOnce(chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	a.bot.Request(action) // 忽略错误，typing 是 best-effort
}

// ReactToMessage 给消息添加 emoji reaction（👍 等）
// 使用 Telegram Bot API setMessageReaction（v5.5.1 不支持，直接 HTTP 调用）
func (a *Adapter) ReactToMessage(chatID string, messageID string, emoji string) {
	if a.bot == nil {
		return
	}

	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return
	}
	msgIDInt, err := strconv.Atoi(messageID)
	if err != nil {
		return
	}

	// 直接调 Telegram Bot API setMessageReaction
	go a.callSetMessageReaction(chatIDInt, msgIDInt, emoji)
}

// callSetMessageReaction 调用 Telegram setMessageReaction API
func (a *Adapter) callSetMessageReaction(chatID int64, messageID int, emoji string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setMessageReaction", a.bot.Token)

	payload := strings.NewReader(fmt.Sprintf(
		`{"chat_id":%d,"message_id":%d,"reaction":[{"type":"emoji","emoji":"%s"}]}`,
		chatID, messageID, emoji,
	))

	resp, err := http.Post(apiURL, "application/json", payload)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// best-effort，不处理响应
	_ = resp
}

// callTelegramAPI 调用 Telegram Bot API 的通用方法
func (a *Adapter) callTelegramAPI(method string, params url.Values) ([]byte, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", a.bot.Token, method)
	resp, err := http.PostForm(apiURL, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body []byte
	if _, err := resp.Body.Read(body); err != nil {
		return nil, err
	}
	return body, nil
}

// SendStream implements gateway.StreamGateway.
// Creates a streaming message that can be updated in real-time.
func (a *Adapter) SendStream(ctx context.Context, chatID string, replyToMsgID string) (gateway.StreamSender, error) {
	if !a.running || a.bot == nil {
		return nil, fmt.Errorf("telegram: adapter not running")
	}

	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("telegram: invalid chat ID %q: %w", chatID, err)
	}

	replyToID := 0
	if replyToMsgID != "" {
		replyToID, _ = strconv.Atoi(replyToMsgID)
	}

	// 发送初始 "思考中" 消息
	initialText := "🧠 Thinking..."
	msg := tgbotapi.NewMessage(chatIDInt, initialText)
	if replyToID > 0 {
		msg.ReplyToMessageID = replyToID
	}

	sent, err := a.bot.Send(msg)
	if err != nil {
		return nil, fmt.Errorf("telegram: send stream initial: %w", err)
	}

	return &telegramStreamSender{
		adapter:    a,
		chatID:     chatIDInt,
		messageID:  sent.MessageID,
		chatIDStr:  chatID,
		content:    "",
		thinking:   "🧠 Thinking...",
		editCount:  0,
		lastEdit:   time.Now(),
	}, nil
}

// telegramStreamSender implements gateway.StreamSender for Telegram.
type telegramStreamSender struct {
	adapter   *Adapter
	chatID    int64
	messageID int
	chatIDStr string

	mu        sync.Mutex
	content   string       // 已生成的正文内容
	thinking  string       // 当前思考/工具调用标签
	editCount int
	lastEdit  time.Time
	finished  bool
}

// minEditInterval 是两次消息编辑之间的最小间隔（避免触发 Telegram 限流）
const minEditInterval = 800 * time.Millisecond

// maxEdits 是单条消息最大编辑次数（超过后不再编辑，等 Finish 一次性更新）
const maxEdits = 40

func (s *telegramStreamSender) Append(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished {
		return fmt.Errorf("stream sender already finished")
	}

	s.content += content
	// 追加内容时清除思考标签
	s.thinking = ""
	return s.throttledEdit()
}

func (s *telegramStreamSender) SetThinking(label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished {
		return nil
	}

	s.thinking = fmt.Sprintf("🧠 %s", label)
	return s.throttledEdit()
}

func (s *telegramStreamSender) SetToolCall(name, args string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished {
		return nil
	}

	// 截断过长的参数
	shortArgs := args
	if len(shortArgs) > 80 {
		shortArgs = shortArgs[:77] + "..."
	}
	s.thinking = fmt.Sprintf("🔧 %s(%s)", name, shortArgs)
	return s.throttledEdit()
}

func (s *telegramStreamSender) SetResult(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished {
		return nil
	}

	s.content = content
	s.thinking = ""
	return s.throttledEdit()
}

func (s *telegramStreamSender) Finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished {
		return nil
	}
	s.finished = true
	s.thinking = ""

	// 最终编辑：显示完整内容
	display := s.renderContent()
	return s.editMessage(display)
}

func (s *telegramStreamSender) MessageID() string {
	return strconv.Itoa(s.messageID)
}

// throttledEdit 限流编辑：避免过于频繁调用 Telegram API
func (s *telegramStreamSender) throttledEdit() error {
	// 超过最大编辑次数，跳过中间编辑
	if s.editCount >= maxEdits {
		return nil
	}

	// 距离上次编辑太近，跳过
	if time.Since(s.lastEdit) < minEditInterval {
		return nil
	}

	display := s.renderContent()
	return s.editMessage(display)
}

// renderContent 渲染当前消息内容：思考标签 + 正文
func (s *telegramStreamSender) renderContent() string {
	var sb strings.Builder

	// 思考/工具调用标签（作为前缀）
	if s.thinking != "" {
		sb.WriteString(s.thinking)
		sb.WriteString("\n\n")
	}

	// 正文内容
	if s.content != "" {
		content := s.content
		// 预留思考标签的空间
		maxLen := 3900
		if s.thinking != "" {
			maxLen -= len(s.thinking) + 2
		}
		if len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}
		sb.WriteString(content)
	}

	// 如果两者都为空，显示默认思考状态
	if s.thinking == "" && s.content == "" {
		return "🧠 Thinking..."
	}

	return sb.String()
}

// editMessage 调用 Telegram API 编辑消息
func (s *telegramStreamSender) editMessage(text string) error {
	if s.adapter.bot == nil {
		return fmt.Errorf("bot not available")
	}

	edit := tgbotapi.NewEditMessageText(s.chatID, s.messageID, text)
	// 不使用 MarkdownV2，避免转义地狱——流式内容格式不可控
	edit.ParseMode = ""

	_, err := s.adapter.bot.Send(edit)
	if err != nil {
		// 编辑失败不中断流，静默忽略
		return nil
	}

	s.editCount++
	s.lastEdit = time.Now()
	return nil
}

// poll runs the long-polling loop.
func (a *Adapter) poll(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = a.cfg.PollTimeout

	updates := a.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			// v0.36.0: 处理所有消息类型（文本、图片、语音、视频、文件）
			if update.Message == nil {
				continue
			}
			a.processUpdate(ctx, update)
		}
	}
}

// processUpdate converts a Telegram update to a gateway.Message and dispatches it.
func (a *Adapter) processUpdate(ctx context.Context, update tgbotapi.Update) {
	tgMsg := update.Message
	if tgMsg == nil {
		return
	}

	chatID := strconv.FormatInt(tgMsg.Chat.ID, 10)

	// Check chat whitelist
	if !a.cfg.IsChatAllowed(chatID) {
		return
	}

	msg := a.convertMessage(tgMsg)

	// In group chats, only respond to @bot mentions or replies
	if msg.Chat.Type != gateway.ChatPrivate {
		if !a.isMentioned(tgMsg) && msg.ReplyTo == nil {
			return
		}
		// Strip @botusername from text
		if a.botUsername != "" {
			msg.Text = strings.ReplaceAll(msg.Text, "@"+a.botUsername, "")
			msg.Text = strings.TrimSpace(msg.Text)
			msg.Args = strings.TrimSpace(strings.TrimPrefix(msg.Args, "@"+a.botUsername))
		}
	}

	a.mu.RLock()
	handler := a.handler
	a.mu.RUnlock()

	if handler != nil {
		if err := handler(ctx, msg); err != nil {
			fmt.Printf("[telegram] handler error: %v\n", err)
		}
	}
}

// convertMessage converts a Telegram message to a gateway.Message.
func (a *Adapter) convertMessage(tgMsg *tgbotapi.Message) *gateway.Message {
	chatType := gateway.ChatPrivate
	switch tgMsg.Chat.Type {
	case "group":
		chatType = gateway.ChatGroup
	case "supergroup":
		chatType = gateway.ChatSuperGroup
	case "channel":
		chatType = gateway.ChatChannel
	}

	msg := &gateway.Message{
		ID: strconv.Itoa(tgMsg.MessageID),
		Chat: gateway.Chat{
			ID:       strconv.FormatInt(tgMsg.Chat.ID, 10),
			Type:     chatType,
			Title:    tgMsg.Chat.Title,
			Username: tgMsg.Chat.UserName,
		},
		Sender: gateway.User{
			ID:        strconv.FormatInt(tgMsg.From.ID, 10),
			Username:  tgMsg.From.UserName,
			FirstName: tgMsg.From.FirstName,
			LastName:  tgMsg.From.LastName,
		},
		Text:      tgMsg.Text,
		Timestamp: time.Unix(int64(tgMsg.Date), 0),
	}

	// v0.36.0: 提取多媒体附件
	a.extractAttachments(tgMsg, msg)

	// 如果没有文本但有附件，构造描述文本
	if msg.Text == "" && len(msg.Attachments) > 0 {
		var parts []string
		for _, att := range msg.Attachments {
			switch att.Type {
			case gateway.AttachmentImage:
				parts = append(parts, "[用户发送了一张图片]")
			case gateway.AttachmentAudio:
				parts = append(parts, "[用户发送了一段语音]")
			case gateway.AttachmentVideo:
				parts = append(parts, "[用户发送了一段视频]")
			case gateway.AttachmentDocument:
				parts = append(parts, fmt.Sprintf("[用户发送了文件: %s]", att.FileName))
			}
		}
		msg.Text = strings.Join(parts, " ")
	}

	// Parse command
	if tgMsg.IsCommand() {
		msg.IsCommand = true
		msg.Command = tgMsg.Command()
		msg.Args = tgMsg.CommandArguments()
	}

	// Parse reply
	if tgMsg.ReplyToMessage != nil {
		replyMsg := a.convertMessage(tgMsg.ReplyToMessage)
		msg.ReplyTo = replyMsg
	}

	return msg
}

// extractAttachments 从 Telegram 消息中提取多媒体附件
func (a *Adapter) extractAttachments(tgMsg *tgbotapi.Message, msg *gateway.Message) {
	if a.bot == nil {
		return
	}

	// 图片
	if tgMsg.Photo != nil && len(tgMsg.Photo) > 0 {
		// 取最大尺寸的图片
		photo := tgMsg.Photo[len(tgMsg.Photo)-1]
		att := gateway.Attachment{
			Type:     gateway.AttachmentImage,
			FileID:   photo.FileID,
			FileName: "photo.jpg",
			MimeType: "image/jpeg",
			FileSize: int64(photo.FileSize),
		}
		// 尝试下载图片
		if url, err := a.bot.GetFileDirectURL(photo.FileID); err == nil {
			att.FileURL = url
		}
		msg.Attachments = append(msg.Attachments, att)
	}

	// 语音消息
	if tgMsg.Voice != nil {
		att := gateway.Attachment{
			Type:     gateway.AttachmentAudio,
			FileID:   tgMsg.Voice.FileID,
			FileName: "voice.ogg",
			MimeType: tgMsg.Voice.MimeType,
			FileSize: int64(tgMsg.Voice.FileSize),
		}
		if url, err := a.bot.GetFileDirectURL(tgMsg.Voice.FileID); err == nil {
			att.FileURL = url
		}
		msg.Attachments = append(msg.Attachments, att)
	}

	// 音频文件
	if tgMsg.Audio != nil {
		att := gateway.Attachment{
			Type:     gateway.AttachmentAudio,
			FileID:   tgMsg.Audio.FileID,
			FileName: tgMsg.Audio.FileName,
			MimeType: tgMsg.Audio.MimeType,
			FileSize: int64(tgMsg.Audio.FileSize),
		}
		if url, err := a.bot.GetFileDirectURL(tgMsg.Audio.FileID); err == nil {
			att.FileURL = url
		}
		msg.Attachments = append(msg.Attachments, att)
	}

	// 视频
	if tgMsg.Video != nil {
		att := gateway.Attachment{
			Type:     gateway.AttachmentVideo,
			FileID:   tgMsg.Video.FileID,
			FileName: tgMsg.Video.FileName,
			MimeType: tgMsg.Video.MimeType,
			FileSize: int64(tgMsg.Video.FileSize),
		}
		if url, err := a.bot.GetFileDirectURL(tgMsg.Video.FileID); err == nil {
			att.FileURL = url
		}
		msg.Attachments = append(msg.Attachments, att)
	}

	// 文档
	if tgMsg.Document != nil {
		att := gateway.Attachment{
			Type:     gateway.AttachmentDocument,
			FileID:   tgMsg.Document.FileID,
			FileName: tgMsg.Document.FileName,
			MimeType: tgMsg.Document.MimeType,
			FileSize: int64(tgMsg.Document.FileSize),
		}
		if url, err := a.bot.GetFileDirectURL(tgMsg.Document.FileID); err == nil {
			att.FileURL = url
		}
		msg.Attachments = append(msg.Attachments, att)
	}
}

// isMentioned checks if the bot is mentioned in the message.
func (a *Adapter) isMentioned(tgMsg *tgbotapi.Message) bool {
	if a.botUsername == "" {
		return false
	}

	// Check text for @botusername
	if strings.Contains(tgMsg.Text, "@"+a.botUsername) {
		return true
	}

	// Check entities for mention
	for _, entity := range tgMsg.Entities {
		if entity.Type == "mention" {
			mention := tgMsg.Text[entity.Offset : entity.Offset+entity.Length]
			if mention == "@"+a.botUsername {
				return true
			}
		}
	}

	return false
}

// escapeMarkdownV2 转义 Telegram MarkdownV2 特殊字符
func escapeMarkdownV2(text string) string {
	special := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, ch := range special {
		text = strings.ReplaceAll(text, ch, "\\"+ch)
	}
	return text
}

// sendChunk sends a single message chunk to Telegram.
func (a *Adapter) sendChunk(_ context.Context, chatID int64, replyTo int, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if replyTo > 0 {
		msg.ReplyToMessageID = replyTo
	}
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	msg.Text = escapeMarkdownV2(text)

	_, err := a.bot.Send(msg)
	if err != nil {
		// If MarkdownV2 fails, try plain text
		msg.ParseMode = ""
		msg.Text = text
		_, err = a.bot.Send(msg)
		if err != nil {
			return fmt.Errorf("telegram: send message: %w", err)
		}
	}

	return nil
}

// splitMessage splits a message into chunks that fit within Telegram's 4096 char limit.
func (a *Adapter) splitMessage(message string) []string {
	maxLen := a.cfg.MaxMessageLen
	if maxLen > 4096 {
		maxLen = 4096
	}

	if len(message) <= maxLen {
		return []string{message}
	}

	var chunks []string
	for len(message) > 0 {
		chunkLen := maxLen
		if chunkLen > len(message) {
			chunkLen = len(message)
		}

		chunk := message[:chunkLen]

		// Try to split at newline boundary
		if chunkLen < len(message) {
			if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
				chunk = chunk[:idx+1]
				chunkLen = idx + 1
			}
		}

		chunks = append(chunks, chunk)
		message = message[chunkLen:]
	}

	return chunks
}

// waitRateLimit enforces per-chat rate limiting.
func (a *Adapter) waitRateLimit(chatID string) {
	a.mu.Lock()
	bucket, exists := a.rateLimit[chatID]
	if !exists {
		bucket = &rateBucket{}
		a.rateLimit[chatID] = bucket
	}
	a.mu.Unlock()

	elapsed := time.Since(bucket.lastSent)
	minInterval := time.Second / time.Duration(a.cfg.RateLimit)
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}

	bucket.lastSent = time.Now()
}