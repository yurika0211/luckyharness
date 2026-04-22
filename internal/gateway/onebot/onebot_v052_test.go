package onebot

import (
	"context"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/gateway"
)

// ============================================================
// CV-1: Adapter 包测试补全
// ============================================================

// TestAdapterStartInvalidConfig 测试 Start 无效配置
func TestAdapterStartInvalidConfig(t *testing.T) {
	adapter := NewAdapter(Config{})
	
	ctx := context.Background()
	err := adapter.Start(ctx)
	if err == nil {
		t.Error("expected error for empty APIBase")
	}
}

// TestAdapterIsRunning 测试 IsRunning
func TestAdapterIsRunning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)
	
	if adapter.IsRunning() {
		t.Error("expected not running initially")
	}
}

// TestAdapterSend 测试 Send
func TestAdapterSend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://invalid-url-that-will-fail"
	adapter := NewAdapter(cfg)
	
	ctx := context.Background()

	// 发送到无效 URL 应该失败
	err := adapter.Send(ctx, "12345", "test")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// TestAdapterSendWithReply 测试 SendWithReply
func TestAdapterSendWithReply(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://invalid-url-that-will-fail"
	adapter := NewAdapter(cfg)
	
	ctx := context.Background()

	// 发送到无效 URL 应该失败
	err := adapter.SendWithReply(ctx, "12345", "reply-to", "test")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// TestAdapterCheckAPI 测试 checkAPI
func TestAdapterCheckAPI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://invalid-url"
	adapter := NewAdapter(cfg)
	
	err := adapter.checkAPI()
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// TestAdapterSendTyping 测试 sendTyping
func TestAdapterSendTyping(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://invalid-url"
	adapter := NewAdapter(cfg)
	
	ctx := context.Background()
	// 不应该 panic
	adapter.sendTyping(ctx, "12345")
}

// TestAdapterSendLike 测试 sendLike
func TestAdapterSendLike(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://invalid-url"
	cfg.AutoLike = true
	cfg.LikeTimes = 1
	adapter := NewAdapter(cfg)
	
	ctx := context.Background()
	// 不应该 panic
	adapter.sendLike(ctx, "12345", cfg.LikeTimes)
}

// TestAdapterCallAPI 测试 callAPI
func TestAdapterCallAPI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://invalid-url"
	adapter := NewAdapter(cfg)
	
	result, err := adapter.callAPI("/test", nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

// TestAdapterWaitRateLimit 测试 waitRateLimit
func TestAdapterWaitRateLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)
	
	// 第一次调用不应该等待
	start := time.Now()
	adapter.waitRateLimit("test-user")
	elapsed := time.Since(start)
	
	if elapsed > 100*time.Millisecond {
		t.Errorf("expected no wait, waited %v", elapsed)
	}
}

// TestAdapterSplitMessageEdgeCases 测试 splitMessage 边界情况
func TestAdapterSplitMessageEdgeCases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)
	
	// 空字符串
	chunks := adapter.splitMessage("")
	// 空字符串可能返回 1 个空 chunk
	if len(chunks) > 1 {
		t.Errorf("expected 0 or 1 chunk for empty string, got %d", len(chunks))
	}
	
	// 正好等于最大长度的消息
	maxMsg := ""
	for i := 0; i < cfg.MaxMessageLen; i++ {
		maxMsg += "x"
	}
	chunks = adapter.splitMessage(maxMsg)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for max length message, got %d", len(chunks))
	}
	
	// 超过最大长度 1 个字符
	overMsg := maxMsg + "x"
	chunks = adapter.splitMessage(overMsg)
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks for over-length message, got %d", len(chunks))
	}
}

// TestAdapterParseGroupIDEdgeCases 测试 parseGroupID 边界情况
func TestAdapterParseGroupIDEdgeCases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)
	
	// 空字符串
	id, isGroup := adapter.parseGroupID("")
	if isGroup {
		t.Error("empty string should not be group ID")
	}
	
	// 非数字字符串
	id, isGroup = adapter.parseGroupID("abc123")
	if isGroup {
		t.Error("non-numeric string should not be group ID")
	}
	
	// 小数字（不是群 ID）
	id, isGroup = adapter.parseGroupID("12345")
	if isGroup {
		t.Error("small number should not be recognized as group ID")
	}
	
	// 大数字（群 ID）
	id, isGroup = adapter.parseGroupID("888888")
	if !isGroup {
		t.Error("large number should be recognized as group ID")
	}
	if id != 888888 {
		t.Errorf("expected 888888, got %d", id)
	}
}

// TestTruncateStr 测试 truncateStr 函数
func TestTruncateStr(t *testing.T) {
	// 短字符串不应该被截断
	short := "hello"
	result := truncateStr(short, 10)
	if result != short {
		t.Errorf("expected '%s', got '%s'", short, result)
	}

	// 长字符串应该被截断
	long := "this is a very long string"
	result = truncateStr(long, 10)
	if len(result) != 10 {
		t.Errorf("expected length 10, got %d", len(result))
	}
	// 实际截断结果可能包含省略号
	if result == "" {
		t.Error("expected non-empty result")
	}
}

// ============================================================
// CV-2: 并发安全测试
// ============================================================

// TestAdapterConcurrentSetHandler 测试 Adapter 并发安全
func TestAdapterConcurrentSetHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)
	
	done := make(chan bool)
	
	// 并发设置 handler
	for i := 0; i < 10; i++ {
		go func() {
			adapter.SetHandler(func(ctx context.Context, msg *gateway.Message) error {
				return nil
			})
			done <- true
		}()
	}
	
	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestAdapterConcurrentSend 测试 Adapter 并发发送
func TestAdapterConcurrentSend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://invalid-url"
	adapter := NewAdapter(cfg)
	
	ctx := context.Background()
	done := make(chan bool)
	
	// 并发发送消息
	for i := 0; i < 10; i++ {
		go func(idx int) {
			adapter.Send(ctx, "12345", "test-"+string(rune('0'+idx)))
			done <- true
		}(i)
	}
	
	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestAdapterConcurrentWaitRateLimit 测试 waitRateLimit 并发
func TestAdapterConcurrentWaitRateLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)
	
	done := make(chan bool)
	
	// 并发等待 rate limit
	for i := 0; i < 10; i++ {
		go func(idx int) {
			userID := "user-" + string(rune('0'+idx))
			adapter.waitRateLimit(userID)
			done <- true
		}(i)
	}
	
	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ============================================================
// CV-3: Config 测试
// ============================================================

func TestConfigValidation(t *testing.T) {
	// 测试 MaxMessageLen 默认值
	cfg := DefaultConfig()
	if cfg.MaxMessageLen <= 0 {
		t.Error("expected positive MaxMessageLen")
	}
	
	// 测试 LikeTimes 范围
	cfg.LikeTimes = 0
	adapter := NewAdapter(cfg)
	if adapter.cfg.LikeTimes != 1 {
		t.Errorf("expected LikeTimes=1 for 0, got %d", adapter.cfg.LikeTimes)
	}
	
	cfg.LikeTimes = 15
	adapter = NewAdapter(cfg)
	if adapter.cfg.LikeTimes != 10 {
		t.Errorf("expected LikeTimes clamped to 10, got %d", adapter.cfg.LikeTimes)
	}
}
