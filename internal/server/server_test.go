package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/tool"
)

// createTestAgent 创建测试用 Agent
func createTestAgent(t *testing.T) *agent.Agent {
	t.Helper()
	mgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("create config manager: %v", err)
	}
	// 不需要加载配置，使用默认值即可
	a, err := agent.New(mgr)
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return a
}

func TestNew(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	s := New(a, cfg)

	if s == nil {
		t.Fatal("server should not be nil")
	}
	if s.config.Addr != ":9090" {
		t.Errorf("expected addr :9090, got %s", s.config.Addr)
	}
	if s.config.RateLimit != 60 {
		t.Errorf("expected rate limit 60, got %d", s.config.RateLimit)
	}
	if s.rateLimiter == nil {
		t.Error("rate limiter should not be nil")
	}
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()
	if cfg.Addr != ":9090" {
		t.Errorf("expected :9090, got %s", cfg.Addr)
	}
	if !cfg.EnableCORS {
		t.Error("CORS should be enabled by default")
	}
	if cfg.RateLimit != 60 {
		t.Errorf("expected 60, got %d", cfg.RateLimit)
	}
}

func TestHandleHealth(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected ok, got %v", resp["status"])
	}
	if resp["version"] != "v0.12.0" {
		t.Errorf("expected v0.12.0, got %v", resp["version"])
	}
}

func TestHandleHealthMethodNotAllowed(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleRoot(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.handleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "LuckyHarness API" {
		t.Errorf("expected LuckyHarness API, got %v", resp["name"])
	}
}

func TestHandleMemoryStats(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/stats", nil)
	w := httptest.NewRecorder()
	s.handleMemoryStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleMemorySave(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	body := map[string]interface{}{
		"content":  "test memory content",
		"category": "test",
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMemory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleMemorySaveLongTerm(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	body := map[string]interface{}{
		"content":   "long term memory",
		"category":  "identity",
		"long_term": true,
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMemory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleMemoryEmptyContent(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	body := map[string]interface{}{
		"content": "",
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleMemory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleMemoryRecall(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/recall?q=test", nil)
	w := httptest.NewRecorder()
	s.handleMemoryRecall(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleMemoryRecallNoQuery(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/recall", nil)
	w := httptest.NewRecorder()
	s.handleMemoryRecall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTools(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	w := httptest.NewRecorder()
	s.handleTools(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	count, ok := resp["count"].(float64)
	if !ok || count < 1 {
		t.Errorf("expected at least 1 tool, got %v", resp["count"])
	}
}

func TestHandleStats(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["version"] != "v0.12.0" {
		t.Errorf("expected v0.12.0, got %v", resp["version"])
	}
}

func TestHandleSoul(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/soul", nil)
	w := httptest.NewRecorder()
	s.handleSoul(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleSessions(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	w := httptest.NewRecorder()
	s.handleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleChatSyncEmptyMessage(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	body := map[string]interface{}{
		"message": "",
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sync", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleChatSync(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleChatSyncInvalidBody(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sync", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleChatSync(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ===== 中间件测试 =====

func TestCORSMiddleware(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	cfg.EnableCORS = true
	s := New(a, cfg)

	handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// OPTIONS 预检
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("CORS origin header should be set")
	}
}

func TestCORSMiddlewareDisabled(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	cfg.EnableCORS = false
	s := New(a, cfg)

	handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// CORS 禁用时，OPTIONS 不应返回 204（无预检响应）
	if w.Code == http.StatusNoContent {
		t.Error("CORS disabled, OPTIONS should not return 204")
	}
	// 应该直接传给 next handler，返回 200
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (passthrough), got %d", w.Code)
	}
	// 不应有 CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS headers should not be set when CORS is disabled")
	}
}

func TestAuthMiddlewareNoKeys(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	cfg.APIKeys = nil // 无 API Key
	s := New(a, cfg)

	called := false
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called when no API keys configured")
	}
}

func TestAuthMiddlewareWithKeys(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	cfg.APIKeys = []string{"test-key-123"}
	s := New(a, cfg)

	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 无 API Key
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	// 错误 API Key
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	// 正确 API Key (Header)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// 正确 API Key (Bearer)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// 正确 API Key (Query)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stats?api_key=test-key-123", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddlewareHealthBypass(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	cfg.APIKeys = []string{"test-key-123"}
	s := New(a, cfg)

	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 健康检查不需要认证
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health endpoint should bypass auth, got %d", w.Code)
	}

	// 根路由不需要认证
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("root endpoint should bypass auth, got %d", w.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(3)

	for i := 0; i < 3; i++ {
		if !rl.Allow("127.0.0.1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow("127.0.0.1") {
		t.Error("4th request should be rate limited")
	}

	// 不同 IP 应该独立计数
	if !rl.Allow("192.168.1.1") {
		t.Error("different IP should be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	a := createTestAgent(t)
	cfg := DefaultServerConfig()
	cfg.RateLimit = 2
	s := New(a, cfg)

	handler := s.rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 前两个请求应该通过
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d should be allowed, got %d", i+1, w.Code)
		}
	}

	// 第三个应该被限流
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	handler := s.recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestServerStartStop(t *testing.T) {
	a := createTestAgent(t)
	cfg := ServerConfig{
		Addr:       ":0", // 随机端口
		EnableCORS: true,
		RateLimit:  60,
	}
	s := New(a, cfg)

	if err := s.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	if !s.IsRunning() {
		t.Error("server should be running")
	}

	// 重复启动应报错
	if err := s.Start(); err == nil {
		t.Error("starting already running server should fail")
	}

	// 等待服务启动
	time.Sleep(100 * time.Millisecond)

	if err := s.Stop(); err != nil {
		t.Fatalf("stop server: %v", err)
	}

	if s.IsRunning() {
		t.Error("server should not be running after stop")
	}
}

func TestServerStats(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	stats := s.Stats()
	if stats.TotalReqs != 0 {
		t.Errorf("initial total requests should be 0, got %d", stats.TotalReqs)
	}
	if stats.StartTime.IsZero() {
		t.Error("start time should not be zero")
	}
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		input    agent.EventType
		expected string
	}{
		{0, "reason"},    // EventReason
		{1, "act"},       // EventAct
		{2, "observe"},   // EventObserve
		{3, "content"},   // EventContent
		{4, "done"},      // EventDone
		{5, "error"},     // EventError
		{99, "unknown"},  // Unknown
	}

	for _, tt := range tests {
		result := eventTypeString(tt.input)
		if result != tt.expected {
			t.Errorf("eventTypeString(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestPermString(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"auto", 0, "auto"},
		{"approve", 1, "approve"},
		{"deny", 2, "deny"},
		{"unknown", 99, "unknown"},
	}

	for _, tt := range tests {
		result := permString(tool.PermissionLevel(tt.input))
		if result != tt.expected {
			t.Errorf("permString(%s) = %s, want %s", tt.name, result, tt.expected)
		}
	}
}

func TestSendJSON(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	w := httptest.NewRecorder()
	s.sendJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected json content type, got %s", ct)
	}
}

func TestSendError(t *testing.T) {
	a := createTestAgent(t)
	s := New(a, DefaultServerConfig())

	w := httptest.NewRecorder()
	s.sendError(w, "test error", http.StatusBadRequest, "details")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "test error" {
		t.Errorf("expected 'test error', got %s", resp.Error)
	}
	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Code)
	}
	if resp.Details != "details" {
		t.Errorf("expected 'details', got %s", resp.Details)
	}
}
