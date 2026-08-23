package weixin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yurika0211/luckyagent/internal/gateway"
	"github.com/yurika0211/luckyagent/internal/logger"
)

const (
	epGetUpdates = "ilink/bot/getupdates"
	epSendMsg    = "ilink/bot/sendmessage"
)

type Adapter struct {
	cfg Config

	mu           sync.RWMutex
	handler      gateway.MessageHandler
	httpClient   *http.Client
	running      bool
	cancel       context.CancelFunc
	contextToken map[string]string
	syncBuf      string
	status       Status
}

// Status is a credential-free snapshot of the iLink gateway state. It is
// intended for diagnostics from the process that owns the gateway.
type Status struct {
	Running           bool      `json:"running"`
	Authenticated     bool      `json:"authenticated"`
	LastPollAt        time.Time `json:"last_poll_at,omitempty"`
	LastPollSuccessAt time.Time `json:"last_poll_success_at,omitempty"`
	LastMessageAt     time.Time `json:"last_message_at,omitempty"`
	LastSendAt        time.Time `json:"last_send_at,omitempty"`
	PollFailures      int64     `json:"poll_failures"`
	HandlerFailures   int64     `json:"handler_failures"`
	SendFailures      int64     `json:"send_failures"`
	LastError         string    `json:"last_error,omitempty"`
}

type apiEnvelope struct {
	Ret                int              `json:"ret"`
	ErrCode            int              `json:"errcode"`
	ErrMsg             string           `json:"errmsg"`
	GetUpdatesBuf      string           `json:"get_updates_buf"`
	LongPollingTimeout int              `json:"longpolling_timeout_ms"`
	Msgs               []incomingRecord `json:"msgs"`
}

type incomingRecord struct {
	MessageID    string         `json:"message_id"`
	FromUserID   string         `json:"from_user_id"`
	ToUserID     string         `json:"to_user_id"`
	RoomID       string         `json:"room_id"`
	ContextToken string         `json:"context_token"`
	ItemList     []incomingItem `json:"item_list"`
}

type incomingItem struct {
	Type     int    `json:"type"`
	TextItem textIn `json:"text_item"`
}

type textIn struct {
	Content string `json:"content"`
}

type sendMessagePayload struct {
	Msg outgoingMessage `json:"msg"`
}

type outgoingMessage struct {
	FromUserID   string         `json:"from_user_id"`
	ToUserID     string         `json:"to_user_id"`
	ClientID     string         `json:"client_id"`
	MessageType  int            `json:"message_type"`
	MessageState int            `json:"message_state"`
	ItemList     []outgoingItem `json:"item_list"`
	ContextToken string         `json:"context_token,omitempty"`
}

type outgoingItem struct {
	Type     int     `json:"type"`
	TextItem textOut `json:"text_item"`
}

type textOut struct {
	Text string `json:"text"`
}

func NewAdapter(cfg Config) *Adapter {
	def := DefaultConfig()
	if cfg.PollTimeoutMilliseconds <= 0 {
		cfg.PollTimeoutMilliseconds = def.PollTimeoutMilliseconds
	}
	if cfg.SendChunkDelayMS <= 0 {
		cfg.SendChunkDelayMS = def.SendChunkDelayMS
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = def.BaseURL
	}
	if strings.TrimSpace(cfg.DMPolicy) == "" {
		cfg.DMPolicy = def.DMPolicy
	}
	if strings.TrimSpace(cfg.GroupPolicy) == "" {
		cfg.GroupPolicy = def.GroupPolicy
	}
	return &Adapter{
		cfg:          cfg,
		httpClient:   &http.Client{Timeout: 45 * time.Second},
		contextToken: make(map[string]string),
	}
}

func (a *Adapter) Name() string { return "weixin" }

func (a *Adapter) SetHandler(handler gateway.MessageHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handler = handler
}

func (a *Adapter) Start(ctx context.Context) error {
	if strings.TrimSpace(a.cfg.Token) == "" {
		return fmt.Errorf("weixin: token is required")
	}
	if strings.TrimSpace(a.cfg.AccountID) == "" {
		return fmt.Errorf("weixin: account_id is required")
	}
	runCtx, cancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.cancel = cancel
	a.running = true
	a.status.Running = true
	a.mu.Unlock()
	go a.poll(runCtx)
	return nil
}

func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	a.running = false
	a.status.Running = false
	return nil
}

func (a *Adapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// Diagnostics returns a copy of the current credential-free runtime status.
func (a *Adapter) Diagnostics() Status {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func (a *Adapter) Send(ctx context.Context, chatID string, message string) error {
	return a.SendWithReply(ctx, chatID, "", message)
}

func (a *Adapter) SendWithReply(ctx context.Context, chatID string, _ string, message string) error {
	chunks := a.splitText(strings.TrimSpace(message))
	for i, chunk := range chunks {
		if err := a.sendChunk(ctx, chatID, chunk); err != nil {
			return err
		}
		if i < len(chunks)-1 && a.cfg.SendChunkDelayMS > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(a.cfg.SendChunkDelayMS) * time.Millisecond):
			}
		}
	}
	return nil
}

func (a *Adapter) splitText(message string) []string {
	if message == "" {
		return nil
	}
	if !a.cfg.SplitMultilineMessages && len(message) <= 4000 {
		return []string{message}
	}
	if a.cfg.SplitMultilineMessages {
		lines := strings.Split(message, "\n")
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				out = append(out, line)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	out := make([]string, 0, (len(message)/4000)+1)
	for len(message) > 4000 {
		out = append(out, message[:4000])
		message = message[4000:]
	}
	if message != "" {
		out = append(out, message)
	}
	return out
}

func (a *Adapter) sendChunk(ctx context.Context, chatID string, content string) error {
	payload := sendMessagePayload{
		Msg: outgoingMessage{
			FromUserID:   "",
			ToUserID:     chatID,
			ClientID:     "luckyagent-weixin-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			MessageType:  2,
			MessageState: 2,
			ItemList:     []outgoingItem{{Type: 1, TextItem: textOut{Text: content}}},
		},
	}
	a.mu.RLock()
	if token := strings.TrimSpace(a.contextToken[chatID]); token != "" {
		payload.Msg.ContextToken = token
	}
	a.mu.RUnlock()
	if err := a.post(ctx, epSendMsg, payload, nil); err != nil {
		a.recordSendFailure(err)
		return err
	}
	a.mu.Lock()
	a.status.LastSendAt = time.Now()
	a.mu.Unlock()
	return nil
}

func (a *Adapter) poll(ctx context.Context) {
	timeout := a.cfg.PollTimeoutMilliseconds
	if timeout <= 0 {
		timeout = 35000
	}
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		req := map[string]any{
			"get_updates_buf": a.syncBuf,
			"timeout_ms":      timeout,
			"base_info":       map[string]any{"channel_version": "2.2.0"},
		}
		a.mu.Lock()
		a.status.LastPollAt = time.Now()
		a.mu.Unlock()
		var resp apiEnvelope
		if err := a.post(ctx, epGetUpdates, req, &resp); err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			delay := pollRetryDelay(failures)
			a.recordPollFailure(err)
			logger.Warn("weixin gateway poll failed; retrying", "attempt", failures, "retry_after", delay.String(), "error", safeError(err, a.cfg.Token))
			if !waitForRetry(ctx, delay) {
				return
			}
			continue
		}
		failures = 0
		a.mu.Lock()
		a.status.Authenticated = true
		a.status.LastPollSuccessAt = time.Now()
		a.status.LastError = ""
		a.mu.Unlock()
		if resp.GetUpdatesBuf != "" {
			a.syncBuf = resp.GetUpdatesBuf
		}
		if resp.LongPollingTimeout > 0 {
			timeout = resp.LongPollingTimeout
		}
		for _, record := range resp.Msgs {
			a.handleRecord(ctx, record)
		}
	}
}

func pollRetryDelay(failures int) time.Duration {
	if failures < 1 {
		return 2 * time.Second
	}
	delay := 2 * time.Second
	for attempt := 1; attempt < failures && delay < 30*time.Second; attempt++ {
		delay *= 2
	}
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (a *Adapter) handleRecord(ctx context.Context, record incomingRecord) {
	senderID := strings.TrimSpace(record.FromUserID)
	if senderID == "" || senderID == strings.TrimSpace(a.cfg.AccountID) {
		return
	}
	chatID := senderID
	chatType := gateway.ChatPrivate
	if roomID := strings.TrimSpace(record.RoomID); roomID != "" {
		chatID = roomID
		chatType = gateway.ChatGroup
		if !a.cfg.isGroupAllowed(chatID) {
			return
		}
	} else if !a.cfg.isDMAllowed(senderID) {
		return
	}
	if token := strings.TrimSpace(record.ContextToken); token != "" {
		a.mu.Lock()
		a.contextToken[chatID] = token
		a.mu.Unlock()
	}
	texts := make([]string, 0, len(record.ItemList))
	for _, item := range record.ItemList {
		if item.Type == 1 && strings.TrimSpace(item.TextItem.Content) != "" {
			texts = append(texts, strings.TrimSpace(item.TextItem.Content))
		}
	}
	text := strings.TrimSpace(strings.Join(texts, "\n"))
	if text == "" {
		return
	}
	msg := &gateway.Message{
		ID:        strings.TrimSpace(record.MessageID),
		Chat:      gateway.Chat{ID: chatID, Type: chatType},
		Sender:    gateway.User{ID: senderID, Username: senderID, FirstName: senderID},
		Text:      text,
		Timestamp: time.Now(),
	}
	msg.IsCommand, msg.Command, msg.Args = parseCommand(text)
	a.mu.RLock()
	handler := a.handler
	a.mu.RUnlock()
	if handler != nil {
		if err := handler(ctx, msg); err != nil {
			a.recordHandlerFailure(err)
			logger.Warn("weixin gateway message handler failed", "chat_id", chatID, "error", safeError(err, a.cfg.Token))
			return
		}
		a.mu.Lock()
		a.status.LastMessageAt = time.Now()
		a.mu.Unlock()
	}
}

func (a *Adapter) post(ctx context.Context, endpoint string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.normalizedBaseURL()+"/"+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(a.cfg.Token))
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("iLink-App-Id", "bot")
	req.Header.Set("iLink-App-ClientVersion", "131584")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s", safeError(fmt.Errorf("weixin: http %d: %s", resp.StatusCode, responseErrorMessage(data)), a.cfg.Token))
	}
	if err := validateBusinessResponse(data); err != nil {
		return fmt.Errorf("%s", safeError(err, a.cfg.Token))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

func (a *Adapter) recordPollFailure(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.PollFailures++
	a.status.LastError = safeError(err, a.cfg.Token)
}

func (a *Adapter) recordHandlerFailure(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.HandlerFailures++
	a.status.LastError = safeError(err, a.cfg.Token)
}

func (a *Adapter) recordSendFailure(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.SendFailures++
	a.status.LastError = safeError(err, a.cfg.Token)
}

func validateBusinessResponse(data []byte) error {
	var envelope apiEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("weixin: decode response: %w", err)
	}
	if envelope.Ret == 0 && envelope.ErrCode == 0 {
		return nil
	}
	message := strings.TrimSpace(envelope.ErrMsg)
	if message == "" {
		message = "iLink returned a business error"
	}
	return fmt.Errorf("weixin: iLink business error (ret=%d errcode=%d): %s", envelope.Ret, envelope.ErrCode, truncateErrorMessage(message))
}

func responseErrorMessage(data []byte) string {
	var envelope apiEnvelope
	if json.Unmarshal(data, &envelope) == nil && strings.TrimSpace(envelope.ErrMsg) != "" {
		return truncateErrorMessage(envelope.ErrMsg)
	}
	return "request rejected"
}

func safeError(err error, token string) string {
	if err == nil {
		return ""
	}
	message := strings.ReplaceAll(err.Error(), strings.TrimSpace(token), "[redacted]")
	return truncateErrorMessage(message)
}

func truncateErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	const maxLen = 512
	if len(message) > maxLen {
		return message[:maxLen] + "…"
	}
	return message
}

func parseCommand(text string) (bool, string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return false, "", ""
	}
	if !strings.HasPrefix(text, "/") {
		return false, "", ""
	}
	text = strings.TrimPrefix(text, "/")
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(strings.TrimSpace(parts[0]))
	args := ""
	if len(parts) == 2 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd != "", cmd, args
}
