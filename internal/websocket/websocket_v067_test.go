//go:build !integration
// +build !integration

package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// v0.67.0: websocket 包测试补全 - 覆盖 handler 相关 0% 函数

// TestHandleMessageUnknownType 测试 HandleMessage 处理未知消息类型
func TestHandleMessageUnknownType(t *testing.T) {
	client := &Client{
		ID:        "test-client",
		SessionID: "test-session-4",
		Send:      make(chan *Message, 10),
	}

	msg := &Message{
		Type:      "UNKNOWN_TYPE",
		SessionID: "test-session-4",
		Data:      json.RawMessage([]byte("{}")),
	}

	handler := NewAgentHandler(nil)
	// 不应该 panic
	handler.HandleMessage(client, msg)
}

// TestHandleMessageInvalidData 测试 HandleMessage 处理无效数据
func TestHandleMessageInvalidData(t *testing.T) {
	client := &Client{
		ID:        "test-client",
		SessionID: "test-session-3",
		Send:      make(chan *Message, 10),
	}

	// 创建无效数据的消息
	invalidData := json.RawMessage([]byte("invalid_json"))
	msg := &Message{
		Type:      TypeChat,
		SessionID: "test-session-3",
		Data:      invalidData,
	}

	handler := NewAgentHandler(nil)
	handler.HandleMessage(client, msg)

	// 等待消息处理
	time.Sleep(100 * time.Millisecond)

	// 验证收到错误消息
	select {
	case received := <-client.Send:
		if received.Type != TypeError {
			t.Errorf("HandleMessage invalid data: expected TypeError, got %v", received.Type)
		}
	default:
		t.Error("HandleMessage invalid data: expected error message")
	}
}

// TestCancelSession 测试 CancelSession 函数
func TestCancelSession(t *testing.T) {
	handler := NewAgentHandler(nil)

	// 添加一个 pending cancel
	ctx, cancel := context.WithCancel(context.Background())
	handler.pending["test-session"] = cancel

	// 验证 pending 存在
	if handler.PendingCount() != 1 {
		t.Errorf("PendingCount before cancel: expected 1, got %d", handler.PendingCount())
	}

	// 取消 session
	handler.CancelSession("test-session")

	// 验证 context 被取消
	select {
	case <-ctx.Done():
		// 符合预期
	default:
		t.Error("CancelSession: context should be cancelled")
	}

	// 验证 pending 被移除
	if handler.PendingCount() != 0 {
		t.Errorf("PendingCount after cancel: expected 0, got %d", handler.PendingCount())
	}
}

// TestPendingCount 测试 PendingCount 函数
func TestPendingCount(t *testing.T) {
	handler := NewAgentHandler(nil)

	if handler.PendingCount() != 0 {
		t.Errorf("PendingCount empty: expected 0, got %d", handler.PendingCount())
	}

	// 添加 pending
	handler.pending["session1"] = func() {}
	handler.pending["session2"] = func() {}

	if handler.PendingCount() != 2 {
		t.Errorf("PendingCount with 2 sessions: expected 2, got %d", handler.PendingCount())
	}
}

// TestParseData 测试 ParseData 函数
func TestParseData(t *testing.T) {
	// 创建测试消息
	msg, err := NewMessage(TypeChat, "test-session", ChatData{
		Message: "hello",
		Stream:  true,
	})
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	var data ChatData
	if err := msg.ParseData(&data); err != nil {
		t.Fatalf("ParseData failed: %v", err)
	}

	if data.Message != "hello" {
		t.Errorf("ParseData: expected message 'hello', got '%s'", data.Message)
	}

	if !data.Stream {
		t.Error("ParseData: expected Stream to be true")
	}
}

// TestNewMessage 测试 NewMessage 函数
func TestNewMessage(t *testing.T) {
	msg, err := NewMessage(TypeChat, "test-session", ChatData{
		Message: "test",
		Stream:  false,
	})

	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	if msg.Type != TypeChat {
		t.Errorf("NewMessage: expected TypeChat, got %v", msg.Type)
	}

	if msg.SessionID != "test-session" {
		t.Errorf("NewMessage: expected session_id 'test-session', got '%s'", msg.SessionID)
	}
}
