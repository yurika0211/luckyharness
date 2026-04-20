package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/memory"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/tool"
)

// Server 是 LuckyHarness 的 HTTP API Server
type Server struct {
	mu      sync.RWMutex
	server  *http.Server
	agent   *agent.Agent
	config  ServerConfig
	running bool

	// 限流
	rateLimiter *rateLimiter

	// 统计
	stats ServerStats
}

// ServerConfig API Server 配置
type ServerConfig struct {
	Addr        string   `yaml:"addr,omitempty"`         // 监听地址，默认 :9090
	APIKeys     []string `yaml:"api_keys,omitempty"`     // API Key 白名单（空=不鉴权）
	EnableCORS  bool     `yaml:"enable_cors,omitempty"`  // 启用 CORS，默认 true
	CORSOrigins []string `yaml:"cors_origins,omitempty"` // CORS 允许的源
	RateLimit   int      `yaml:"rate_limit,omitempty"`   // 每分钟请求限制，默认 60
}

// DefaultServerConfig 返回默认配置
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:        ":9090",
		EnableCORS:  true,
		CORSOrigins: []string{"*"},
		RateLimit:   60,
	}
}

// ServerStats 服务器统计
type ServerStats struct {
	mu           sync.RWMutex
	TotalReqs    int64
	ChatReqs     int64
	ErrorReqs    int64
	StartTime    time.Time
	LastReqTime  time.Time
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message    string            `json:"message"`
	SessionID  string            `json:"session_id,omitempty"`
	Stream     bool              `json:"stream,omitempty"`
	MaxIter    int               `json:"max_iterations,omitempty"`
	AutoApprove bool             `json:"auto_approve,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Response   string        `json:"response"`
	SessionID  string        `json:"session_id"`
	Iterations int           `json:"iterations"`
	TokensUsed int           `json:"tokens_used"`
	ToolCalls  []toolCallInfo `json:"tool_calls,omitempty"`
	Duration   string        `json:"duration"`
}

type toolCallInfo struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Duration  string `json:"duration"`
}

// MemoryEntry 记忆条目
type MemoryEntry struct {
	ID          string  `json:"id"`
	Content     string  `json:"content"`
	Category    string  `json:"category"`
	Tier        string  `json:"tier"`
	Importance  float64 `json:"importance"`
	AccessCount int     `json:"access_count"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

// New 创建 API Server
func New(a *agent.Agent, cfg ServerConfig) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":9090"
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 60
	}

	return &Server{
		agent:       a,
		config:      cfg,
		rateLimiter: newRateLimiter(cfg.RateLimit),
		stats: ServerStats{
			StartTime: time.Now(),
		},
	}
}

// Start 启动 API Server
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	mux := http.NewServeMux()

	// API v1 路由
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/chat/sync", s.handleChatSync)
	mux.HandleFunc("/api/v1/sessions", s.handleSessions)
	mux.HandleFunc("/api/v1/memory", s.handleMemory)
	mux.HandleFunc("/api/v1/memory/recall", s.handleMemoryRecall)
	mux.HandleFunc("/api/v1/memory/stats", s.handleMemoryStats)
	mux.HandleFunc("/api/v1/tools", s.handleTools)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/soul", s.handleSoul)

	// 根路由
	mux.HandleFunc("/", s.handleRoot)

	var handler http.Handler = mux

	// 中间件链
	handler = s.recoveryMiddleware(handler)
	handler = s.loggingMiddleware(handler)
	handler = s.rateLimitMiddleware(handler)
	handler = s.authMiddleware(handler)
	if s.config.EnableCORS {
		handler = s.corsMiddleware(handler)
	}

	s.server = &http.Server{
		Addr:         s.config.Addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // SSE 需要较长超时
		IdleTimeout:  120 * time.Second,
	}

	s.running = true
	s.mu.Unlock()

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API server error: %v\n", err)
		}
	}()

	fmt.Printf("🚀 LuckyHarness API Server running at http://localhost%s\n", s.config.Addr)
	fmt.Printf("   API: /api/v1/chat | /api/v1/health | /api/v1/stats\n")
	return nil
}

// Stop 停止 API Server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	s.running = false
	return nil
}

// IsRunning 返回是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Stats 返回服务器统计
func (s *Server) Stats() ServerStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()
	return s.stats
}

// ===== 路由处理 =====

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"version":   "v0.12.0",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// handleChat 流式聊天 (SSE)
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	s.stats.mu.Lock()
	s.stats.ChatReqs++
	s.stats.TotalReqs++
	s.stats.LastReqTime = time.Now()
	s.stats.mu.Unlock()

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}

	if req.Message == "" {
		s.sendError(w, "message is required", http.StatusBadRequest, "")
		return
	}

	start := time.Now()

	loopCfg := agent.DefaultLoopConfig()
	if req.MaxIter > 0 {
		loopCfg.MaxIterations = req.MaxIter
	}
	loopCfg.AutoApprove = req.AutoApprove

	ctx := r.Context()

	// SSE 流式响应
	flusher, ok := w.(http.Flusher)
	if !ok {
		// 不支持 SSE，降级为同步
		s.doChatSync(w, r, req, loopCfg, ctx, start)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events, err := s.agent.RunLoopStream(ctx, req.Message, loopCfg)
	if err != nil {
		s.sendSSEError(w, flusher, err.Error())
		return
	}

	for event := range events {
		data, _ := json.Marshal(map[string]interface{}{
			"type":      eventTypeString(event.Type),
			"content":   event.Content,
			"iteration": event.Iteration,
		})

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		if event.Type == agent.EventDone || event.Type == agent.EventError {
			break
		}
	}

	duration := time.Since(start)
	summary, _ := json.Marshal(map[string]interface{}{
		"type":     "complete",
		"duration": duration.String(),
	})
	fmt.Fprintf(w, "data: %s\n\n", summary)
	flusher.Flush()
}

// handleChatSync 同步聊天
func (s *Server) handleChatSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	s.stats.mu.Lock()
	s.stats.ChatReqs++
	s.stats.TotalReqs++
	s.stats.LastReqTime = time.Now()
	s.stats.mu.Unlock()

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}

	if req.Message == "" {
		s.sendError(w, "message is required", http.StatusBadRequest, "")
		return
	}

	start := time.Now()

	loopCfg := agent.DefaultLoopConfig()
	if req.MaxIter > 0 {
		loopCfg.MaxIterations = req.MaxIter
	}
	loopCfg.AutoApprove = req.AutoApprove

	ctx := r.Context()
	s.doChatSync(w, r, req, loopCfg, ctx, start)
}

func (s *Server) doChatSync(w http.ResponseWriter, r *http.Request, req ChatRequest, loopCfg agent.LoopConfig, ctx context.Context, start time.Time) {
	result, err := s.agent.RunLoop(ctx, req.Message, loopCfg)
	if err != nil {
		s.stats.mu.Lock()
		s.stats.ErrorReqs++
		s.stats.mu.Unlock()
		s.sendError(w, "chat failed", http.StatusInternalServerError, err.Error())
		return
	}

	duration := time.Since(start)

	resp := ChatResponse{
		Response:   result.Response,
		SessionID:  req.SessionID,
		Iterations: result.Iterations,
		TokensUsed: result.TokensUsed,
		Duration:   duration.String(),
	}

	for _, tc := range result.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, toolCallInfo{
			Name:      tc.Name,
			Arguments: tc.Arguments,
			Result:    tc.Result,
			Duration:  tc.Duration.String(),
		})
	}

	s.sendJSON(w, http.StatusOK, resp)
}

// handleSessions 会话列表
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	// Agent 暴露 session manager
	sessions := s.agent.Sessions().List()
	type sessionInfo struct {
		ID        string `json:"id"`
		MessageCount int  `json:"message_count"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	var infos []sessionInfo
	for _, sess := range sessions {
		msgs := sess.GetMessages()
		infos = append(infos, sessionInfo{
			ID:           sess.ID,
			MessageCount: len(msgs),
			CreatedAt:    sess.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    sess.UpdatedAt.Format(time.RFC3339),
		})
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": infos,
		"count":    len(infos),
	})
}

// handleMemory 记忆管理
func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 列出所有记忆
		stats := s.agent.MemoryStats()
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"stats": map[string]int{
				"short":  stats[memory.TierShort],
				"medium": stats[memory.TierMedium],
				"long":   stats[memory.TierLong],
			},
		})

	case http.MethodPost:
		// 保存记忆
		var body struct {
			Content  string `json:"content"`
			Category string `json:"category"`
			LongTerm bool   `json:"long_term"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.sendError(w, "invalid request body", http.StatusBadRequest, err.Error())
			return
		}

		if body.Content == "" {
			s.sendError(w, "content is required", http.StatusBadRequest, "")
			return
		}

		if body.Category == "" {
			body.Category = "user"
		}

		var err error
		if body.LongTerm {
			err = s.agent.RememberLongTerm(body.Content, body.Category)
		} else {
			err = s.agent.Remember(body.Content, body.Category)
		}

		if err != nil {
			s.sendError(w, "save memory failed", http.StatusInternalServerError, err.Error())
			return
		}

		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"status":    "saved",
			"long_term": body.LongTerm,
		})

	default:
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
	}
}

// handleMemoryRecall 记忆搜索
func (s *Server) handleMemoryRecall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendError(w, "query parameter 'q' is required", http.StatusBadRequest, "")
		return
	}

	results := s.agent.Recall(query)
	var entries []MemoryEntry
	for _, e := range results {
		entries = append(entries, MemoryEntry{
			ID:          e.ID,
			Content:     e.Content,
			Category:    e.Category,
			Tier:        e.Tier.String(),
			Importance:  e.Importance,
			AccessCount: e.AccessCount,
		})
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"results": entries,
		"count":   len(entries),
	})
}

// handleMemoryStats 记忆统计
func (s *Server) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	stats := s.agent.MemoryStats()
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"short_term":  stats[memory.TierShort],
		"medium_term": stats[memory.TierMedium],
		"long_term":   stats[memory.TierLong],
		"total":       stats[memory.TierShort] + stats[memory.TierMedium] + stats[memory.TierLong],
	})
}

// handleTools 工具列表
func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	tools := s.agent.Tools()
	allTools := tools.List()

	type toolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Permission  string `json:"permission"`
		Enabled     bool   `json:"enabled"`
	}

	var infos []toolInfo
	for _, t := range allTools {
		infos = append(infos, toolInfo{
			Name:        t.Name,
			Description: t.Description,
			Category:    string(t.Category),
			Permission:  permString(t.Permission),
			Enabled:     t.Enabled,
		})
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"tools": infos,
		"count": len(infos),
	})
}

// handleStats 服务器统计
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	stats := s.Stats()
	uptime := time.Since(stats.StartTime)

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"total_requests": stats.TotalReqs,
		"chat_requests":  stats.ChatReqs,
		"error_requests": stats.ErrorReqs,
		"uptime":         uptime.String(),
		"start_time":     stats.StartTime.Format(time.RFC3339),
		"last_request":   stats.LastReqTime.Format(time.RFC3339),
		"version":        "v0.12.0",
	})
}

// handleSoul SOUL 信息
func (s *Server) handleSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "method not allowed", http.StatusMethodNotAllowed, "")
		return
	}

	soul := s.agent.Soul()
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"system_prompt": soul.SystemPrompt(),
	})
}

// handleRoot 根路由
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"name":     "LuckyHarness API",
		"version":  "v0.12.0",
		"endpoints": []string{
			"POST /api/v1/chat       — 流式聊天 (SSE)",
			"POST /api/v1/chat/sync  — 同步聊天",
			"GET  /api/v1/sessions   — 会话列表",
			"GET  /api/v1/memory     — 记忆统计",
			"POST /api/v1/memory     — 保存记忆",
			"GET  /api/v1/memory/recall?q= — 搜索记忆",
			"GET  /api/v1/memory/stats    — 记忆统计",
			"GET  /api/v1/tools      — 工具列表",
			"GET  /api/v1/stats      — 服务器统计",
			"GET  /api/v1/soul       — SOUL 信息",
			"GET  /api/v1/health     — 健康检查",
		},
	})
}

// ===== 中间件 =====

// corsMiddleware CORS 中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.EnableCORS {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		allowed := false

		if len(s.config.CORSOrigins) == 0 {
			allowed = true
		} else {
			for _, o := range s.config.CORSOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
		}

		if allowed {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware API Key 认证中间件
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 无配置 API Key 则跳过认证
		if len(s.config.APIKeys) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// 健康检查不需要认证
		if r.URL.Path == "/api/v1/health" || r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		// 从 Header 或 Query 获取 API Key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.Header.Get("Authorization")
			if strings.HasPrefix(apiKey, "Bearer ") {
				apiKey = strings.TrimPrefix(apiKey, "Bearer ")
			}
		}
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey == "" {
			s.sendError(w, "api key required", http.StatusUnauthorized, "provide X-API-Key header or api_key query param")
			return
		}

		valid := false
		for _, k := range s.config.APIKeys {
			if k == apiKey {
				valid = true
				break
			}
		}

		if !valid {
			s.sendError(w, "invalid api key", http.StatusForbidden, "")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware 限流中间件
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if !s.rateLimiter.Allow(ip) {
			s.sendError(w, "rate limit exceeded", http.StatusTooManyRequests,
				fmt.Sprintf("limit: %d req/min", s.config.RateLimit))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware 日志中间件
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[%s] %s %s %v\n", start.Format("15:04:05"), r.Method, r.URL.Path, time.Since(start))
	})
}

// recoveryMiddleware 恢复中间件
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.stats.mu.Lock()
				s.stats.ErrorReqs++
				s.stats.mu.Unlock()
				s.sendError(w, "internal server error", http.StatusInternalServerError,
					fmt.Sprintf("%v", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ===== 辅助函数 =====

func (s *Server) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) sendError(w http.ResponseWriter, msg string, code int, details string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   msg,
		Code:    code,
		Details: details,
	})
}

func (s *Server) sendSSEError(w io.Writer, flusher http.Flusher, msg string) {
	data, _ := json.Marshal(map[string]interface{}{
		"type":  "error",
		"error": msg,
	})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func eventTypeString(t agent.EventType) string {
	switch t {
	case agent.EventReason:
		return "reason"
	case agent.EventAct:
		return "act"
	case agent.EventObserve:
		return "observe"
	case agent.EventContent:
		return "content"
	case agent.EventDone:
		return "done"
	case agent.EventError:
		return "error"
	default:
		return "unknown"
	}
}

func permString(p tool.PermissionLevel) string {
	switch p {
	case tool.PermAuto:
		return "auto"
	case tool.PermApprove:
		return "approve"
	case tool.PermDeny:
		return "deny"
	default:
		return "unknown"
	}
}

// ===== 限流器 =====

type rateLimiter struct {
	mu      sync.RWMutex
	limit   int
	clients map[string]*clientBucket
}

type clientBucket struct {
	count    int
	resetAt  time.Time
}

func newRateLimiter(limit int) *rateLimiter {
	rl := &rateLimiter{
		limit:   limit,
		clients: make(map[string]*clientBucket),
	}

	// 后台清理过期桶
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *rateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, ok := rl.clients[ip]
	if !ok || now.After(bucket.resetAt) {
		rl.clients[ip] = &clientBucket{
			count:   1,
			resetAt: now.Add(time.Minute),
		}
		return true
	}

	bucket.count++
	return bucket.count <= rl.limit
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, bucket := range rl.clients {
		if now.After(bucket.resetAt) {
			delete(rl.clients, ip)
		}
	}
}

// Ensure Agent exposes Sessions
// We need to add Sessions() method to Agent
var _ provider.Provider = (*provider.OpenAIProvider)(nil)
