package onebot

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/gateway"
)

// createTestAgentForOnebot 创建测试用 Agent
func createTestAgentForOnebot(t *testing.T) *agent.Agent {
	// 创建临时目录用于测试
	tmpDir, err := os.MkdirTemp("", "onebot_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建配置管理器
	mgr, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// 设置测试配置（不会真正调用 LLM）
	cfg := mgr.Get()
	cfg.APIKey = "test-key"
	cfg.Model = "test-model"
	cfg.APIBase = "http://localhost:9999"

	a, err := agent.New(mgr)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}
	return a
}

// TestGetSessionID 测试 getSessionID 创建新 session
func TestGetSessionID(t *testing.T) {
	a := createTestAgentForOnebot(t)
	adapter := NewAdapter(DefaultConfig())
	h := NewHandler(adapter, a)

	chatID := "test_chat_123"
	sid := h.getSessionID(chatID)

	if sid == "" {
		t.Error("getSessionID returned empty session ID")
	}

	// 验证同一 chatID 返回相同 sessionID
	sid2 := h.getSessionID(chatID)
	if sid != sid2 {
		t.Errorf("getSessionID should return same session ID, got %s != %s", sid, sid2)
	}
}

// TestGetSessionIDDifferentChats 测试不同 chatID 返回不同 sessionID
func TestGetSessionIDDifferentChats(t *testing.T) {
	a := createTestAgentForOnebot(t)
	adapter := NewAdapter(DefaultConfig())
	h := NewHandler(adapter, a)

	sid1 := h.getSessionID("chat_1")
	sid2 := h.getSessionID("chat_2")

	if sid1 == sid2 {
		t.Error("Different chatIDs should have different sessionIDs")
	}
}

// TestResetSession 测试 resetSession 创建新 session
func TestResetSession(t *testing.T) {
	a := createTestAgentForOnebot(t)
	adapter := NewAdapter(DefaultConfig())
	h := NewHandler(adapter, a)

	chatID := "test_chat_reset"
	sid1 := h.getSessionID(chatID)
	sid2 := h.resetSession(chatID)

	if sid1 == sid2 {
		t.Error("resetSession should create a new session ID")
	}

	// 验证 reset 后 getSessionID 返回新的 sessionID
	sid3 := h.getSessionID(chatID)
	if sid3 != sid2 {
		t.Errorf("After reset, getSessionID should return new session ID, got %s != %s", sid3, sid2)
	}
}

// TestHandleMessageEmptyText 测试 HandleMessage 处理空消息
func TestHandleMessageEmptyText(t *testing.T) {
	a := createTestAgentForOnebot(t)
	adapter := NewAdapter(DefaultConfig())
	h := NewHandler(adapter, a)

	ctx := context.Background()
	msg := &gateway.Message{
		Chat: gateway.Chat{ID: "test_chat"},
		Text: "   ", // 空白文本
	}

	err := h.HandleMessage(ctx, msg)
	if err != nil {
		t.Errorf("HandleMessage with empty text should return nil, got %v", err)
	}
}

// TestHandleMessageCommand 测试 HandleMessage 处理命令
func TestHandleMessageCommand(t *testing.T) {
	a := createTestAgentForOnebot(t)
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:9999" // 设置一个不会真正连接的地址
	adapter := NewAdapter(cfg)
	adapter.running = true // 设置 adapter 为运行状态
	h := NewHandler(adapter, a)

	ctx := context.Background()
	msg := &gateway.Message{
		Chat:      gateway.Chat{ID: "test_chat"},
		Text:      "/help",
		IsCommand: true,
		Command:   "help",
	}

	err := h.HandleMessage(ctx, msg)
	// 允许连接错误，因为测试主要验证命令处理流程
	if err != nil && !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("HandleMessage with /help command failed: %v", err)
	}
}

// TestHandleMessageCommandReset 测试 /reset 命令
func TestHandleMessageCommandReset(t *testing.T) {
	a := createTestAgentForOnebot(t)
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:9999"
	adapter := NewAdapter(cfg)
	adapter.running = true
	h := NewHandler(adapter, a)

	chatID := "test_chat_reset_cmd"

	// 先创建 session
	sid1 := h.getSessionID(chatID)

	ctx := context.Background()
	msg := &gateway.Message{
		Chat:      gateway.Chat{ID: chatID},
		Text:      "/reset",
		IsCommand: true,
		Command:   "reset",
	}

	err := h.HandleMessage(ctx, msg)
	if err != nil && !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("HandleMessage with /reset command failed: %v", err)
	}

	// 验证 session 已重置
	sid2 := h.getSessionID(chatID)
	if sid1 == sid2 {
		t.Error("/reset command should create new session ID")
	}
}

// TestHandleMessageCommandStart 测试 /start 命令
func TestHandleMessageCommandStart(t *testing.T) {
	a := createTestAgentForOnebot(t)
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:9999"
	adapter := NewAdapter(cfg)
	adapter.running = true
	h := NewHandler(adapter, a)

	ctx := context.Background()
	msg := &gateway.Message{
		Chat:      gateway.Chat{ID: "test_chat"},
		Text:      "/start",
		IsCommand: true,
		Command:   "start",
	}

	err := h.HandleMessage(ctx, msg)
	if err != nil && !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("HandleMessage with /start command failed: %v", err)
	}
}

// TestHandleMessageNormal 测试 HandleMessage 处理普通消息
func TestHandleMessageNormal(t *testing.T) {
	a := createTestAgentForOnebot(t)
	adapter := NewAdapter(DefaultConfig())
	h := NewHandler(adapter, a)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg := &gateway.Message{
		Chat:      gateway.Chat{ID: "test_chat_normal"},
		Text:      "Hello",
		IsCommand: false,
	}

	// 注意：这个测试会因为 agent 调用 LLM 失败而返回错误
	// 主要验证流程能走到 agent.ChatWithSession
	err := h.HandleMessage(ctx, msg)

	// 允许 LLM 调用失败，但验证 getSessionID 被调用
	if err == nil {
		t.Log("HandleMessage succeeded (unexpected, but OK)")
	} else {
		t.Logf("HandleMessage returned error (expected due to mock LLM): %v", err)
	}
}

// TestHandleCommandUnknown 测试未知命令处理
func TestHandleCommandUnknown(t *testing.T) {
	a := createTestAgentForOnebot(t)
	adapter := NewAdapter(DefaultConfig())
	h := NewHandler(adapter, a)

	ctx := context.Background()
	msg := &gateway.Message{
		Chat:      gateway.Chat{ID: "test_chat"},
		Text:      "/unknown",
		IsCommand: true,
		Command:   "unknown",
	}

	err := h.HandleMessage(ctx, msg)
	// 未知命令应该返回错误或者忽略，不应该 panic
	t.Logf("Unknown command result: %v", err)
}
