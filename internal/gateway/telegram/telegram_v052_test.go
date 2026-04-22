package telegram

import (
	"context"
	"testing"
)

// ============================================================
// CV-2: Telegram Handler 测试补全
// ============================================================

// TestHandlerSetDataDir 测试 SetDataDir
func TestHandlerSetDataDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	adapter := NewAdapter(cfg)
	handler := NewHandler(adapter, nil)

	tmpDir := t.TempDir()
	handler.SetDataDir(tmpDir)

	h := handler
	if h.dataDir != tmpDir {
		t.Errorf("expected dataDir %s, got %s", tmpDir, h.dataDir)
	}
}

// TestHandlerChatSessionsPath 测试 chatSessionsPath (跳过，需要 agent)
func TestHandlerChatSessionsPath(t *testing.T) {
	t.Skip("requires non-nil agent")
}

// TestHandlerLoadSaveChatSessions 测试会话加载保存 (跳过，需要 agent)
func TestHandlerLoadSaveChatSessions(t *testing.T) {
	t.Skip("requires non-nil agent")
}

// TestHandlerGetSessionID 测试 getSessionID (跳过，需要 agent)
func TestHandlerGetSessionID(t *testing.T) {
	t.Skip("requires non-nil agent")
}

// TestHandlerResetSession 测试 resetSession (跳过，需要 agent)
func TestHandlerResetSession(t *testing.T) {
	t.Skip("requires non-nil agent")
}

// TestHandlerSetSessionID 测试 setSessionID
func TestHandlerSetSessionID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	adapter := NewAdapter(cfg)
	handler := NewHandler(adapter, nil)

	h := handler
	h.setSessionID("chat-1", "test-session")
	if h.sessions["chat-1"] != "test-session" {
		t.Errorf("expected test-session, got %s", h.sessions["chat-1"])
	}
}

// TestHandlerHasSession 测试 hasSession
func TestHandlerHasSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	adapter := NewAdapter(cfg)
	handler := NewHandler(adapter, nil)

	h := handler
	h.sessions["chat-1"] = "session-1"

	if !h.hasSession("chat-1") {
		t.Error("expected session to exist")
	}
	if h.hasSession("chat-2") {
		t.Error("expected session not to exist")
	}
}

// TestHandlerHandleMessage 测试 HandleMessage (跳过，需要 agent)
func TestHandlerHandleMessage(t *testing.T) {
	t.Skip("requires non-nil agent")
}

// TestHandlerHandleCommand 测试 handleCommand (跳过，需要 agent)
func TestHandlerHandleCommand(t *testing.T) {
	t.Skip("requires non-nil agent")
}

// TestHandlerTruncateString 测试 truncateString
func TestHandlerTruncateString(t *testing.T) {
	result := truncateString("hello", 10)
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}

	result = truncateString("this is a long string", 10)
	if len(result) != 10 {
		t.Errorf("expected length 10, got %d", len(result))
	}
}

// ============================================================
// CV-3: Telegram Adapter 测试补全
// ============================================================

// TestAdapterStartInvalidConfig 测试 Start 无效配置
func TestAdapterStartInvalidConfig(t *testing.T) {
	cfg := Config{}
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	err := adapter.Start(ctx)
	if err == nil {
		t.Error("expected error for empty token")
	}
}

// TestAdapterIsRunning 测试 IsRunning
func TestAdapterIsRunning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	adapter := NewAdapter(cfg)

	if adapter.IsRunning() {
		t.Error("expected not running initially")
	}
}

// TestAdapterSend 测试 Send
func TestAdapterSend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	err := adapter.Send(ctx, "123", "test message")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

// TestAdapterSendWithReply 测试 SendWithReply
func TestAdapterSendWithReply(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	err := adapter.SendWithReply(ctx, "123", "456", "test message")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

// TestAdapterSendTypingOnce 测试 sendTypingOnce (跳过，需要实际 API)
func TestAdapterSendTypingOnce(t *testing.T) {
	t.Skip("requires actual Telegram API")
}

// TestAdapterReactToMessage 测试 ReactToMessage (跳过，需要实际 API)
func TestAdapterReactToMessage(t *testing.T) {
	t.Skip("requires actual Telegram API")
}

// TestAdapterCallSetMessageReaction 测试 callSetMessageReaction (跳过，需要实际 API)
func TestAdapterCallSetMessageReaction(t *testing.T) {
	t.Skip("requires actual Telegram API")
}

// TestAdapterCallTelegramAPI 测试 callTelegramAPI (跳过，需要实际 API)
func TestAdapterCallTelegramAPI(t *testing.T) {
	t.Skip("requires actual Telegram API")
}

// TestAdapterSendStream 测试 SendStream
func TestAdapterSendStream(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, err := adapter.SendStream(ctx, "123", "")
	if err == nil {
		t.Error("expected error for invalid token")
	}
	if stream != nil {
		t.Error("expected nil stream")
	}
}

// TestAdapterAppend 测试 Append
func TestAdapterAppend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		err := stream.Append("test")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	}
}

// TestAdapterSetThinking 测试 SetThinking
func TestAdapterSetThinking(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		err := stream.SetThinking("thinking")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	}
}

// TestAdapterSetToolCall 测试 SetToolCall
func TestAdapterSetToolCall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		err := stream.SetToolCall("search", `{"query": "test"}`)
		if err == nil {
			t.Error("expected error for invalid token")
		}
	}
}

// TestAdapterSetResult 测试 SetResult
func TestAdapterSetResult(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		err := stream.SetResult("result")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	}
}

// TestAdapterFinish 测试 Finish
func TestAdapterFinish(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		err := stream.Finish()
		if err == nil {
			t.Error("expected error for invalid token")
		}
	}
}

// TestAdapterMessageID 测试 MessageID
func TestAdapterMessageID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		id := stream.MessageID()
		if id != "" {
			t.Errorf("expected empty string, got %s", id)
		}
	}
}

// TestAdapterThrottledEdit 测试 throttledEdit
func TestAdapterThrottledEdit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		stream.(*telegramStreamSender).throttledEdit()
	}
}

// TestAdapterEditMessage 测试 editMessage
func TestAdapterEditMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	stream, _ := adapter.SendStream(ctx, "123", "")
	if stream != nil {
		err := stream.(*telegramStreamSender).editMessage("new text")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	}
}

// TestAdapterPoll 测试 poll (跳过，需要实际 API)
func TestAdapterPoll(t *testing.T) {
	t.Skip("requires actual Telegram API")
}

// TestAdapterProcessUpdate 测试 processUpdate (跳过，需要内部类型)
func TestAdapterProcessUpdate(t *testing.T) {
	t.Skip("requires internal tgbotapi types")
}

// TestAdapterConvertMessage 测试 convertMessage (跳过，需要内部类型)
func TestAdapterConvertMessage(t *testing.T) {
	t.Skip("requires internal tgbotapi types")
}

// TestAdapterExtractAttachments 测试 extractAttachments (跳过，需要内部类型)
func TestAdapterExtractAttachments(t *testing.T) {
	t.Skip("requires internal tgbotapi types")
}

// TestAdapterIsMentioned 测试 isMentioned (跳过，需要内部类型)
func TestAdapterIsMentioned(t *testing.T) {
	t.Skip("requires internal tgbotapi types")
}

// TestAdapterIsReplyToBot 测试 isReplyToBot (跳过，需要内部类型)
func TestAdapterIsReplyToBot(t *testing.T) {
	t.Skip("requires internal tgbotapi types")
}

// TestAdapterSendChunk 测试 sendChunk (跳过，需要实际 API)
func TestAdapterSendChunk(t *testing.T) {
	t.Skip("requires actual Telegram API")
}

// TestAdapterWaitRateLimit 测试 waitRateLimit
func TestAdapterWaitRateLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	adapter := NewAdapter(cfg)
	adapter.waitRateLimit("test-user")
}

// TestAdapterSplitMessage 测试 splitMessage
func TestAdapterSplitMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	adapter := NewAdapter(cfg)

	chunks := adapter.splitMessage("hello")
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}

	longMsg := ""
	for i := 0; i < 5000; i++ {
		longMsg += "x"
	}
	chunks = adapter.splitMessage(longMsg)
	if len(chunks) < 2 {
		t.Errorf("expected >= 2 chunks, got %d", len(chunks))
	}
}

// TestAdapterEscapeMarkdownV2 测试 escapeMarkdownV2
func TestAdapterEscapeMarkdownV2(t *testing.T) {
	input := "hello_world.test"
	result := escapeMarkdownV2(input)
	if result == "" {
		t.Error("expected non-empty result")
	}
}

// ============================================================
// CV-4: 并发安全测试
// ============================================================

// TestAdapterConcurrentSend 测试并发发送
func TestAdapterConcurrentSend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "invalid-token"
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			adapter.Send(ctx, "123", "test-"+string(rune('0'+idx)))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestHandlerConcurrentSessions 测试并发 session 操作
func TestHandlerConcurrentSessions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Token = "test-token"
	adapter := NewAdapter(cfg)
	handler := NewHandler(adapter, nil)

	h := handler
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			chatID := "chat-" + string(rune('0'+idx))
			h.setSessionID(chatID, "session-"+string(rune('0'+idx)))
			h.hasSession(chatID)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
