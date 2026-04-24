//go:build !integration
// +build !integration

package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/config"
)

// v0.72.0: websocket 包测试补全 - 覆盖 syncChat 和 streamChat

// createTestAgentForWS 创建测试用 Agent
func createTestAgentForWS(t *testing.T) *agent.Agent {
	t.Helper()
	tmpDir := t.TempDir()
	mgr, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("create config manager: %v", err)
	}
	a, err := agent.New(mgr)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return a
}

// TestSyncChat 测试 syncChat 函数
func TestSyncChat(t *testing.T) {
	a := createTestAgentForWS(t)
	h := NewAgentHandler(a)

	// 创建测试 client
	client := &Client{
		SessionID: "test-session",
		Send:      make(chan *Message, 10),
	}

	data := ChatData{
		Message: "Hello",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用 syncChat
	h.syncChat(ctx, client, data, "test-parent-id")

	// 验证消息推送
	select {
	case msg := <-client.Send:
		if msg.Type != TypeStatus {
			t.Errorf("expected TypeStatus, got %v", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("syncChat timed out")
	}
}

// TestSyncChatError 测试 syncChat 错误处理
func TestSyncChatError(t *testing.T) {
	a := createTestAgentForWS(t)
	h := NewAgentHandler(a)

	// 创建测试 client
	client := &Client{
		SessionID: "test-session",
		Send:      make(chan *Message, 10),
	}

	// 使用空消息触发错误
	data := ChatData{
		Message: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.syncChat(ctx, client, data, "test-parent-id")

	// 验证错误消息推送
	select {
	case msg := <-client.Send:
		// 应该收到 error 或 executing
		if msg.Type != TypeStatus && msg.Type != TypeError {
			t.Errorf("expected TypeStatus or TypeError, got %v", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("syncChat error test timed out")
	}
}

// TestStreamChat 测试 streamChat 函数
func TestStreamChat(t *testing.T) {
	a := createTestAgentForWS(t)
	h := NewAgentHandler(a)

	// 创建测试 client
	client := &Client{
		SessionID: "test-session",
		Send:      make(chan *Message, 10),
	}

	data := ChatData{
		Message: "Hello",
		Stream:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用 streamChat
	h.streamChat(ctx, client, data, "test-parent-id")

	// 验证消息推送
	select {
	case msg := <-client.Send:
		if msg.Type != TypeStatus {
			t.Errorf("expected TypeStatus, got %v", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("streamChat timed out")
	}
}

// TestStreamChatError 测试 streamChat 错误处理
func TestStreamChatError(t *testing.T) {
	a := createTestAgentForWS(t)
	h := NewAgentHandler(a)

	// 创建测试 client
	client := &Client{
		SessionID: "test-session",
		Send:      make(chan *Message, 10),
	}

	// 使用空消息触发错误
	data := ChatData{
		Message: "",
		Stream:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.streamChat(ctx, client, data, "test-parent-id")

	// 验证错误消息推送
	select {
	case msg := <-client.Send:
		if msg.Type != TypeStatus && msg.Type != TypeError {
			t.Errorf("expected TypeStatus or TypeError, got %v", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("streamChat error test timed out")
	}
}
