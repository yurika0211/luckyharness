package onebot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/gateway"
)

// TestAdapterStartAndStop 测试 Start 和 Stop 流程
func TestAdapterStartAndStop(t *testing.T) {
	// 创建 mock HTTP server 模拟 OneBot API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get_login_info" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok","data":{"user_id":123456,"nickname":"test_bot"}}`))
		}
	}))
	defer mockServer.Close()

	cfg := DefaultConfig()
	cfg.APIBase = mockServer.URL
	adapter := NewAdapter(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 测试 Start
	err := adapter.Start(ctx)
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	// 验证 running 状态
	if !adapter.running {
		t.Error("adapter.running should be true after Start")
	}

	// 测试 IsRunning
	if !adapter.IsRunning() {
		t.Error("IsRunning should return true")
	}

	// 测试 Stop
	err = adapter.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// 验证停止后状态
	if adapter.running {
		t.Error("adapter.running should be false after Stop")
	}
}

// TestAdapterStartWithoutAPIBase 测试 Start 缺少 APIBase 的情况
func TestAdapterStartWithoutAPIBase(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = ""
	adapter := NewAdapter(cfg)

	ctx := context.Background()
	err := adapter.Start(ctx)

	if err == nil {
		t.Error("Start should fail without APIBase")
	}
	if !strings.Contains(err.Error(), "api_base is required") {
		t.Errorf("Error should mention api_base, got: %v", err)
	}
}

// TestSendWithReply 测试 SendWithReply 发送带回复的消息
func TestSendWithReply(t *testing.T) {
	// 创建 mock HTTP server
	requestReceived := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/send_msg" {
			requestReceived = true
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok","data":{"message_id":"123"}}`))
		}
	}))
	defer mockServer.Close()

	cfg := DefaultConfig()
	cfg.APIBase = mockServer.URL
	adapter := NewAdapter(cfg)
	adapter.running = true // 手动设置 running 状态

	ctx := context.Background()
	replyToMsgID := "original_msg_456"
	message := "This is a reply"

	err := adapter.SendWithReply(ctx, "test_chat", replyToMsgID, message)
	if err != nil {
		t.Errorf("SendWithReply failed: %v", err)
	}

	if !requestReceived {
		t.Error("send_msg API should be called")
	}
}

// TestSendWithReplyNotRunning 测试 SendWithReply 在 adapter 未运行时失败
func TestSendWithReplyNotRunning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:9999"
	adapter := NewAdapter(cfg)
	adapter.running = false // 确保未运行

	ctx := context.Background()
	err := adapter.SendWithReply(ctx, "test_chat", "msg_id", "test")

	if err == nil {
		t.Error("SendWithReply should fail when adapter is not running")
	}
	if !strings.Contains(err.Error(), "adapter not running") {
		t.Errorf("Error should mention 'adapter not running', got: %v", err)
	}
}

// TestSendWithReplyLongMessage 测试 SendWithReply 分割长消息
func TestSendWithReplyLongMessage(t *testing.T) {
	requestCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/send_msg" {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer mockServer.Close()

	cfg := DefaultConfig()
	cfg.APIBase = mockServer.URL
	cfg.MaxMessageLen = 100 // 设置较小的分割阈值
	adapter := NewAdapter(cfg)
	adapter.running = true

	ctx := context.Background()
	// 创建一条超过 MaxMessageLen 的消息
	longMessage := strings.Repeat("A", 250) // 250 字符，应该被分割成 3 段 (100, 100, 50)

	err := adapter.SendWithReply(ctx, "test_chat", "reply_to", longMessage)
	if err != nil {
		t.Errorf("SendWithReply with long message failed: %v", err)
	}

	// 验证消息被分割发送
	if requestCount < 2 {
		t.Errorf("Long message should be split into multiple requests, got %d requests", requestCount)
	}
}

// TestAdapterSetHandler 测试 SetHandler
func TestAdapterSetHandler(t *testing.T) {
	cfg := DefaultConfig()
	adapter := NewAdapter(cfg)

	// 初始 handler 应为 nil
	if adapter.handler != nil {
		t.Error("Initial handler should be nil")
	}

	// 设置 handler
	handler := func(ctx context.Context, msg *gateway.Message) error {
		return nil
	}
	adapter.SetHandler(handler)

	if adapter.handler == nil {
		t.Error("Handler should be set")
	}
}

// TestAdapterWithName 测试 Name 方法
func TestAdapterWithName(t *testing.T) {
	cfg := DefaultConfig()
	adapter := NewAdapter(cfg)

	name := adapter.Name()
	if name != "onebot" {
		t.Errorf("Name should return 'onebot', got '%s'", name)
	}
}

// TestAdapterStartWithWebhook 测试 Start 在配置 Webhook 时的行为
func TestAdapterStartWithWebhook(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get_login_info" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok","data":{"user_id":123456,"nickname":"test_bot"}}`))
		}
	}))
	defer mockServer.Close()

	cfg := DefaultConfig()
	cfg.APIBase = mockServer.URL
	// 不设置 WSURL，会尝试启动 webhook server
	adapter := NewAdapter(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := adapter.Start(ctx)
	if err != nil {
		t.Errorf("Start with webhook config failed: %v", err)
	}

	// 给一点时间启动 server
	time.Sleep(100 * time.Millisecond)

	// 验证 running 状态
	if !adapter.running {
		t.Error("adapter should be running")
	}

	adapter.Stop()
}
