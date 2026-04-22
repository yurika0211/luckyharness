package onebot

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yurika0211/luckyharness/internal/gateway"
)

// ── handleEvent ──────────────────────────────────────────────────────────────

func TestHandleEvent_NonMessagePostType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	// Non-message events should be ignored
	event := map[string]any{
		"post_type": "meta_event",
	}
	raw, _ := json.Marshal(event)

	var handlerCalled bool
	adapter.SetHandler(func(_ context.Context, _ *gateway.Message) error {
		handlerCalled = true
		return nil
	})

	adapter.handleEvent(raw)
	assert.False(t, handlerCalled, "handler should not be called for non-message events")
}

func TestHandleEvent_SelfMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	cfg.BotQQID = "12345"
	adapter := NewAdapter(cfg)

	event := map[string]any{
		"post_type":    "message",
		"message_type": "private",
		"user_id":      float64(12345), // same as BotQQID
		"message_id":   float64(1),
		"raw_message":  "self message",
	}
	raw, _ := json.Marshal(event)

	var handlerCalled bool
	adapter.SetHandler(func(_ context.Context, _ *gateway.Message) error {
		handlerCalled = true
		return nil
	})

	adapter.handleEvent(raw)
	assert.False(t, handlerCalled, "handler should not be called for self messages")
}

func TestHandleEvent_PrivateMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	cfg.AutoLike = false
	cfg.ShowTyping = false
	adapter := NewAdapter(cfg)

	event := map[string]any{
		"post_type":    "message",
		"message_type": "private",
		"user_id":      float64(99999),
		"message_id":   float64(42),
		"raw_message":  "hello bot",
	}
	raw, _ := json.Marshal(event)

	var receivedMsg *gateway.Message
	adapter.SetHandler(func(_ context.Context, msg *gateway.Message) error {
		receivedMsg = msg
		return nil
	})

	adapter.handleEvent(raw)
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "hello bot", receivedMsg.Text)
	assert.Equal(t, gateway.ChatPrivate, receivedMsg.Chat.Type)
}

func TestHandleEvent_GroupMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	cfg.AutoLike = false
	cfg.ShowTyping = false
	adapter := NewAdapter(cfg)

	event := map[string]any{
		"post_type":    "message",
		"message_type": "group",
		"user_id":      float64(99999),
		"group_id":     float64(888888),
		"message_id":   float64(43),
		"raw_message":  "group hello",
	}
	raw, _ := json.Marshal(event)

	var receivedMsg *gateway.Message
	adapter.SetHandler(func(_ context.Context, msg *gateway.Message) error {
		receivedMsg = msg
		return nil
	})

	adapter.handleEvent(raw)
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "group hello", receivedMsg.Text)
	assert.Equal(t, gateway.ChatGroup, receivedMsg.Chat.Type)
}

func TestHandleEvent_InvalidJSON(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	// Should not panic on invalid JSON
	adapter.handleEvent([]byte("not json"))
}

func TestHandleEvent_NoHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	cfg.AutoLike = false
	cfg.ShowTyping = false
	adapter := NewAdapter(cfg)

	event := map[string]any{
		"post_type":    "message",
		"message_type": "private",
		"user_id":      float64(99999),
		"message_id":   float64(1),
		"raw_message":  "test",
	}
	raw, _ := json.Marshal(event)

	// Should not panic when no handler set
	adapter.handleEvent(raw)
}

// ── parseGroupID edge cases ──────────────────────────────────────────────────

func TestParseGroupID_SmallNumber(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	// Small numbers are not group IDs
	id, isGroup := adapter.parseGroupID("100")
	assert.False(t, isGroup)
	assert.Equal(t, int64(100), id)
}

func TestParseGroupID_Zero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	id, isGroup := adapter.parseGroupID("0")
	assert.False(t, isGroup)
	assert.Equal(t, int64(0), id)
}

func TestParseGroupID_Negative(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	_, isGroup := adapter.parseGroupID("-1")
	assert.False(t, isGroup)
}

func TestParseGroupID_LargeNumber(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	id, isGroup := adapter.parseGroupID("999999999")
	assert.True(t, isGroup)
	assert.Equal(t, int64(999999999), id)
}

// ── splitMessage edge cases ──────────────────────────────────────────────────

func TestSplitMessage_Empty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	chunks := adapter.splitMessage("")
	assert.Len(t, chunks, 1)
	assert.Equal(t, "", chunks[0])
}

func TestSplitMessage_SingleChar(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	chunks := adapter.splitMessage("a")
	assert.Len(t, chunks, 1)
	assert.Equal(t, "a", chunks[0])
}

func TestSplitMessage_MaxLenClamp(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	cfg.MaxMessageLen = 10000 // over 4500 limit
	adapter := NewAdapter(cfg)

	longMsg := ""
	for i := 0; i < 5000; i++ {
		longMsg += "x"
	}
	chunks := adapter.splitMessage(longMsg)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), 4500)
	}
}

func TestSplitMessage_NewlineBoundary(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	cfg.MaxMessageLen = 20
	adapter := NewAdapter(cfg)

	msg := "line1\nline2\nline3\nline4"
	chunks := adapter.splitMessage(msg)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), 20)
	}
}

// ── Adapter Start validation ─────────────────────────────────────────────────

func TestAdapterStart_NoAPIBase(t *testing.T) {
	cfg := Config{} // no APIBase
	adapter := NewAdapter(cfg)

	err := adapter.Start(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_base is required")
}

// ── Adapter SendWithReply not running ────────────────────────────────────────

func TestAdapterSendWithReply_NotRunning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	err := adapter.SendWithReply(nil, "123", "1", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

// ── Adapter Stop ─────────────────────────────────────────────────────────────

func TestAdapterStop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	err := adapter.Stop()
	assert.NoError(t, err)
	assert.False(t, adapter.IsRunning())
}

// ── Config defaults ──────────────────────────────────────────────────────────

func TestConfig_DefaultWebhookPath(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 4000, cfg.MaxMessageLen)
	assert.True(t, cfg.ShowTyping)
	assert.True(t, cfg.AutoLike)
	assert.Equal(t, 1, cfg.LikeTimes)
}

// ── SetHandler ───────────────────────────────────────────────────────────────

func TestAdapter_SetHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	adapter.SetHandler(func(_ context.Context, _ *gateway.Message) error {
		return nil
	})
	assert.NotNil(t, adapter.handler)
}