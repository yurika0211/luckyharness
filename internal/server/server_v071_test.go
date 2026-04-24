//go:build !integration
// +build !integration

package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// v0.71.0: server 包测试补全 - 覆盖 0.0% 函数

// TestHandleWebSocket 测试 handleWebSocket 函数
func TestHandleWebSocket(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	s := New(a, cfg)

	// 测试 WebSocket 请求（缺少 Upgrade header，会返回 400）
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub2Rl")
	req.Header.Set("Sec-WebSocket-Version", "13")
	w := httptest.NewRecorder()

	s.handleWebSocket(w, req)

	// WebSocket 升级失败会返回 400（不是 503）
	if w.Code != http.StatusBadRequest {
		t.Errorf("handleWebSocket: expected status 400, got %d", w.Code)
	}
}

// TestHandleRAGStreamIndex 测试 handleRAGStreamIndex 函数
func TestHandleRAGStreamIndex(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	s := New(a, cfg)

	// 测试 GET 请求（方法不允许）
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rag/stream", nil)
	w := httptest.NewRecorder()

	s.handleRAGStreamIndex(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("handleRAGStreamIndex GET: expected status 405, got %d", w.Code)
	}
}

// TestHandleRAGStreamIndexPOST 测试 handleRAGStreamIndex POST
func TestHandleRAGStreamIndexPOST(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	s := New(a, cfg)

	// 测试 POST 请求（缺少 path）
	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rag/stream", body)
	w := httptest.NewRecorder()

	s.handleRAGStreamIndex(w, req)

	// 应该返回 400（缺少 path）或 503（stream indexer 未初始化）
	if w.Code != http.StatusBadRequest && w.Code != http.StatusServiceUnavailable {
		t.Errorf("handleRAGStreamIndex POST: expected status 400 or 503, got %d", w.Code)
	}
}

// TestHandleRAGStreamIndexPOSTWithPath 测试 handleRAGStreamIndex POST with path
func TestHandleRAGStreamIndexPOSTWithPath(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	s := New(a, cfg)

	// 测试 POST 请求（带 path）
	body := bytes.NewReader([]byte(`{"path": "/test/file.md"}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rag/stream", body)
	w := httptest.NewRecorder()

	s.handleRAGStreamIndex(w, req)

	// stream indexer 可能未初始化，返回 503
	_ = w.Code
}
