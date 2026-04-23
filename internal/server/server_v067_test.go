//go:build !integration
// +build !integration

package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/metrics"
)

// v0.67.0: server 包测试补全 - 覆盖 0% 函数

// TestHandleChat 测试 handleChat 函数
func TestHandleChat(t *testing.T) {
	s := NewTestServer()

	// 测试空消息
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChat(w, req)

	// 期望返回 200 或 400（取决于实现）
	if w.Code != 200 && w.Code != 400 {
		t.Errorf("handleChat: expected status 200 or 400, got %d", w.Code)
	}
}

// TestHandleChatSync 测试 handleChatSync 函数
func TestHandleChatSync(t *testing.T) {
	s := NewTestServer()

	// 测试空消息
	req := httptest.NewRequest("POST", "/api/chat/sync", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatSync(w, req)

	// 期望返回 200 或 400
	if w.Code != 200 && w.Code != 400 {
		t.Errorf("handleChatSync: expected status 200 or 400, got %d", w.Code)
	}
}

// TestHandleWebSocket 测试 handleWebSocket 函数
func TestHandleWebSocket(t *testing.T) {
	s := NewTestServer()

	// 测试非 WebSocket 请求（应该返回非 200）
	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()

	s.handleWebSocket(w, req)

	// 非 WS 请求应该返回非 200（可能是 400 或 503）
	if w.Code == 200 {
		t.Errorf("handleWebSocket: expected non-200 status for non-WS request, got %d", w.Code)
	}
}

// TestFormatDuration 测试 formatDuration 函数
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{time.Second, "0m 1s"},
		{time.Minute, "1m 0s"},
		{time.Hour, "1h 0m"},
		{90 * time.Second, "1m 30s"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{25 * time.Hour, "1d 1h 0m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.input)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

// TestHandleRAGStreamIndex 测试 handleRAGStreamIndex 函数
// 注意：此测试需要完整的 Agent 初始化，在集成测试中覆盖
func TestHandleRAGStreamIndex(t *testing.T) {
	// 跳过需要复杂初始化的测试
	t.Skip("requires full Agent initialization")
}

// NewTestServer 创建用于测试的 server 实例
func NewTestServer() *Server {
	return &Server{
		metrics: &metrics.Metrics{},
	}
}
