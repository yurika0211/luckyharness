package tool

import (
	"testing"
	"time"
)

func TestGatewayExecute(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)
	gw := NewGateway(r)

	// 测试自动批准的工具
	result, err := gw.Execute("current_time", map[string]any{}, "user1")
	if err != nil {
		t.Fatalf("execute current_time: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success")
	}
	if result.ToolName != "current_time" {
		t.Errorf("expected current_time, got %s", result.ToolName)
	}
}

func TestGatewayExecuteNotFound(t *testing.T) {
	r := NewRegistry()
	gw := NewGateway(r)

	_, err := gw.Execute("nonexistent", map[string]any{}, "user1")
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
}

func TestGatewayExecuteDisabled(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)
	r.Disable("current_time")
	gw := NewGateway(r)

	_, err := gw.Execute("current_time", map[string]any{}, "user1")
	if err == nil {
		t.Fatal("expected error for disabled tool")
	}
}

func TestGatewayExecuteWithTracking(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)
	gw := NewGateway(r)

	// 执行多次
	for i := 0; i < 3; i++ {
		_, _ = gw.Execute("current_time", map[string]any{}, "user1")
	}

	// 检查用量
	stats := gw.Tracker().GetUsage("user1", "current_time")
	if stats.TotalCalls != 3 {
		t.Errorf("expected 3 calls, got %d", stats.TotalCalls)
	}
	if stats.SuccessCalls != 3 {
		t.Errorf("expected 3 success, got %d", stats.SuccessCalls)
	}
}

func TestGatewayResultFormat(t *testing.T) {
	result := &GatewayResult{
		ToolName: "test",
		Output:   "hello world",
		Success:  true,
		Duration: 100 * time.Millisecond,
	}
	formatted := result.Format()
	if !containsStr(formatted, "✅") {
		t.Errorf("expected ✅ in format, got: %s", formatted)
	}

	result.Success = false
	formatted = result.Format()
	if !containsStr(formatted, "❌") {
		t.Errorf("expected ❌ in format, got: %s", formatted)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && findSubstr(s, sub)))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
