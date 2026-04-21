package onebot

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

	"github.com/gorilla/websocket"

	"github.com/yurika0211/luckyharness/internal/gateway"
)

// Adapter implements gateway.Gateway for QQ via OneBot protocol.
type Adapter struct {
	cfg     Config
	client  *http.Client
	running bool
	cancel  context.CancelFunc

	mu        sync.RWMutex
	handler   gateway.MessageHandler
	rateLimit map[string]*rateBucket

	// WebSocket connection for receiving events
	wsConn *websocket.Conn
}

type rateBucket struct {
	lastSent time.Time
}

// NewAdapter creates a new OneBot adapter.
func NewAdapter(cfg Config) *Adapter {
	if cfg.MaxMessageLen <= 0 {
		cfg.MaxMessageLen = 4000
	}
	if cfg.LikeTimes <= 0 {
		cfg.LikeTimes = 1
	}
	if cfg.LikeTimes > 10 {
		cfg.LikeTimes = 10
	}

	return &Adapter{
		cfg:       cfg,
		client:    &http.Client{Timeout: 30 * time.Second},
		rateLimit: make(map[string]*rateBucket),
	}
}

// Name returns the platform name.
func (a *Adapter) Name() string {
	return "onebot"
}

// SetHandler sets the message handler callback.
func (a *Adapter) SetHandler(handler gateway.MessageHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handler = handler
}

// Start connects to OneBot and begins receiving events.
func (a *Adapter) Start(ctx context.Context) error {
	if a.cfg.APIBase == "" {
		return fmt.Errorf("onebot: api_base is required")
	}

	pollCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.running = true

	// Verify API connectivity
	if err := a.checkAPI(); err != nil {
		return fmt.Errorf("onebot: API check failed: %w", err)
	}

	// Start receiving events via WebSocket or polling
	if a.cfg.WSURL != "" {
		go a.listenWebSocket(pollCtx)
	} else {
		// Fallback: no event source configured
		// In this mode, messages are received via HTTP webhook
		go a.startWebhookServer(pollCtx)
	}

	return nil
}

// Stop gracefully shuts down the adapter.
func (a *Adapter) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.wsConn != nil {
		a.wsConn.Close()
	}
	a.running = false
	return nil
}

// Send sends a message to a QQ chat.
func (a *Adapter) Send(ctx context.Context, chatID string, message string) error {
	if !a.running {
		return fmt.Errorf("onebot: adapter not running")
	}

	chunks := a.splitMessage(message)
	for _, chunk := range chunks {
		if err := a.sendQQMessage(ctx, chatID, chunk, ""); err != nil {
			return err
		}
		a.waitRateLimit(chatID)
	}

	return nil
}

// SendWithReply sends a message as a reply to a specific message.
func (a *Adapter) SendWithReply(ctx context.Context, chatID string, replyToMsgID string, message string) error {
	if !a.running {
		return fmt.Errorf("onebot: adapter not running")
	}

	chunks := a.splitMessage(message)
	for _, chunk := range chunks {
		if err := a.sendQQMessage(ctx, chatID, chunk, replyToMsgID); err != nil {
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

// --- OneBot API calls ---

// checkAPI verifies the OneBot API is reachable.
func (a *Adapter) checkAPI() error {
	_, err := a.callAPI("get_login_info", nil)
	return err
}

// sendTyping sends a "typing" indicator to a QQ chat.
// OneBot v11: set_typing (not standard, but NapCat/Lagrange support it)
// OneBot v11 alternative: send_group_msg with empty text + typing segment
func (a *Adapter) sendTyping(ctx context.Context, chatID string) error {
	if !a.cfg.ShowTyping {
		return nil
	}

	groupID, isGroup := a.parseGroupID(chatID)
	params := map[string]any{}

	if isGroup {
		params["group_id"] = groupID
	} else {
		params["user_id"] = chatID
	}

	// Try set_typing first (NapCat/Lagrange extension)
	_, err := a.callAPI("set_typing", params)
	if err != nil {
		// Fallback: some implementations don't support set_typing
		// That's OK, typing indicator is best-effort
		return nil
	}
	return err
}

// sendLike sends a like (thumb up) to a QQ user.
func (a *Adapter) sendLike(ctx context.Context, userID string, times int) error {
	if !a.cfg.AutoLike {
		return nil
	}

	params := map[string]any{
		"user_id": userID,
		"times":   times,
	}

	_, err := a.callAPI("send_like", params)
	return err
}

// sendQQMessage sends a text message to a QQ chat.
func (a *Adapter) sendQQMessage(ctx context.Context, chatID string, message string, replyTo string) error {
	groupID, isGroup := a.parseGroupID(chatID)

	params := map[string]any{
		"message": message,
	}

	if isGroup {
		params["group_id"] = groupID
	} else {
		params["user_id"] = chatID
	}

	if replyTo != "" {
		// OneBot v11 reply format
		params["message"] = fmt.Sprintf("[CQ:reply,id=%s]%s", replyTo, message)
	}

	apiName := "send_msg"
	if isGroup {
		apiName = "send_group_msg"
	}

	_, err := a.callAPI(apiName, params)
	return err
}

// callAPI makes a call to the OneBot HTTP API.
func (a *Adapter) callAPI(action string, params map[string]any) (map[string]any, error) {
	reqBody := map[string]any{
		"action": action,
	}
	if params != nil {
		reqBody["params"] = params
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", a.cfg.APIBase+"/"+action, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if a.cfg.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.AccessToken)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API call %s: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API call %s: status %d: %s", action, resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Check OneBot status field
	if status, ok := result["status"].(string); ok && status != "ok" {
		return nil, fmt.Errorf("API call %s: status=%s, data=%v", action, status, result["data"])
	}

	data, _ := result["data"].(map[string]any)
	return data, nil
}

// --- Event handling ---

// listenWebSocket connects to the OneBot WebSocket and processes events.
func (a *Adapter) listenWebSocket(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.Dial(a.cfg.WSURL, nil)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		a.wsConn = conn

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				conn.Close()
				break
			}

			a.handleEvent(message)
		}

		// Reconnect after delay
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// startWebhookServer starts an HTTP server for receiving OneBot events via webhook.
func (a *Adapter) startWebhookServer(ctx context.Context) {
	path := a.cfg.WebhookPath
	if path == "" {
		path = "/onebot/event"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		a.handleEvent(body)
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    ":6701",
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	server.ListenAndServe()
}

// handleEvent processes a OneBot event.
func (a *Adapter) handleEvent(raw []byte) {
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		return
	}

	postType, _ := event["post_type"].(string)
	if postType != "message" {
		return
	}

	msgType, _ := event["message_type"].(string)
	userID := fmt.Sprintf("%v", event["user_id"])
	groupID := fmt.Sprintf("%v", event["group_id"])
	messageID := fmt.Sprintf("%v", event["message_id"])
	rawMessage, _ := event["raw_message"].(string)

	// Skip messages from self
	if a.cfg.BotQQID != "" && userID == a.cfg.BotQQID {
		return
	}

	chatID := userID
	chatType := gateway.ChatPrivate
	if msgType == "group" {
		chatID = groupID
		chatType = gateway.ChatGroup
	}

	msg := &gateway.Message{
		ID:   messageID,
		Chat: gateway.Chat{ID: chatID, Type: chatType},
		Sender: gateway.User{
			ID: userID,
		},
		Text:      rawMessage,
		Timestamp: time.Now(),
	}

	// Auto like on message received
	if a.cfg.AutoLike && userID != "" {
		go a.sendLike(context.Background(), userID, a.cfg.LikeTimes)
	}

	// Send typing indicator
	if a.cfg.ShowTyping {
		go a.sendTyping(context.Background(), chatID)
	}

	a.mu.RLock()
	handler := a.handler
	a.mu.RUnlock()

	if handler != nil {
		handler(context.Background(), msg)
	}
}

// --- Helpers ---

// parseGroupID checks if a chatID is a group ID (numeric) and returns it.
func (a *Adapter) parseGroupID(chatID string) (int64, bool) {
	// QQ group IDs are numeric
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return 0, false
	}
	// Group IDs are typically > 100000
	if id > 100000 {
		return id, true
	}
	return id, false
}

// splitMessage splits a message into chunks that fit within QQ's message limit.
func (a *Adapter) splitMessage(message string) []string {
	maxLen := a.cfg.MaxMessageLen
	if maxLen > 4500 {
		maxLen = 4500
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
	minInterval := 500 * time.Millisecond
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}

	bucket.lastSent = time.Now()
}