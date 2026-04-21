package telegram

import (
	"context"
	"fmt"
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

// sendChunk sends a single message chunk to Telegram.
func (a *Adapter) sendChunk(_ context.Context, chatID int64, replyTo int, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if replyTo > 0 {
		msg.ReplyToMessageID = replyTo
	}
	msg.ParseMode = tgbotapi.ModeMarkdown

	_, err := a.bot.Send(msg)
	if err != nil {
		// If Markdown fails, try plain text
		msg.ParseMode = ""
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