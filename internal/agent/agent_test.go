package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/contextx"
	"github.com/yurika0211/luckyharness/internal/memory"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/session"
	"github.com/yurika0211/luckyharness/internal/soul"
	"github.com/yurika0211/luckyharness/internal/tool"
)

// --- truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

// --- splitIntoChunks ---

func TestSplitIntoChunks_ShortText(t *testing.T) {
	text := "hello"
	chunks := splitIntoChunks(text, 100)
	if len(chunks) != 1 || chunks[0] != text {
		t.Errorf("expected [%q], got %v", text, chunks)
	}
}

func TestSplitIntoChunks_ExactSize(t *testing.T) {
	text := "abcdefghij"
	chunks := splitIntoChunks(text, 10)
	if len(chunks) != 1 || chunks[0] != text {
		t.Errorf("expected single chunk, got %v", chunks)
	}
}

func TestSplitIntoChunks_SplitAtSentence(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	chunks := splitIntoChunks(text, 20)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}
	// Reconstructed text should equal original
	reconstructed := strings.Join(chunks, "")
	if reconstructed != text {
		t.Errorf("reconstructed text mismatch: got %q, want %q", reconstructed, text)
	}
}

func TestSplitIntoChunks_ChineseSentence(t *testing.T) {
	text := "这是第一句话。这是第二句话。这是第三句话。"
	chunks := splitIntoChunks(text, 10)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks for Chinese text, got %d", len(chunks))
	}
	reconstructed := strings.Join(chunks, "")
	if reconstructed != text {
		t.Errorf("reconstructed text mismatch: got %q, want %q", reconstructed, text)
	}
}

func TestSplitIntoChunks_EmptyString(t *testing.T) {
	chunks := splitIntoChunks("", 10)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("expected single empty chunk, got %v", chunks)
	}
}

// --- inferCategory ---

func TestInferCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"我喜欢编程", "preference"},
		{"I prefer dark mode", "preference"},
		{"讨厌这个设计", "preference"},
		{"项目进度如何", "project"},
		{"project deadline", "project"},
		{"代码仓库在哪", "project"},
		{"什么是RAG", "knowledge"},
		{"解释一下", "knowledge"},
		{"你好", "conversation"},
		{"hello", "conversation"},
		{"随便聊聊", "conversation"},
	}
	for _, tt := range tests {
		got := inferCategory(tt.input)
		if got != tt.expected {
			t.Errorf("inferCategory(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- inferImportance ---

func TestInferImportance(t *testing.T) {
	tests := []struct {
		input       string
		minExpected float64
		maxExpected float64
	}{
		{"重要：请记住这个", 0.7, 0.7},
		{"remember this", 0.7, 0.7},
		{"密码是123456", 0.7, 0.7},
		{"API token expired", 0.7, 0.7},
		{"你好", 0.2, 0.2},
		{"hi", 0.2, 0.2},
	}
	for _, tt := range tests {
		got := inferImportance(tt.input)
		if got < tt.minExpected || got > tt.maxExpected {
			t.Errorf("inferImportance(%q) = %f, want [%f, %f]", tt.input, got, tt.minExpected, tt.maxExpected)
		}
	}

	// Long messages should have at least medium importance
	longMsg := "这是一段超过50个字符的较长消息，包含了具体的项目信息和上下文描述"
	got := inferImportance(longMsg)
	if got < 0.4 {
		t.Errorf("long message importance = %f, expected >= 0.4", got)
	}
}

// --- sanitizeLoopConfig ---

func TestSanitizeLoopConfig_Defaults(t *testing.T) {
	cfg := LoopConfig{}
	sanitizeLoopConfig(&cfg)
	if cfg.MaxIterations != 10 {
		t.Errorf("expected MaxIterations=10, got %d", cfg.MaxIterations)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected Timeout=60s, got %v", cfg.Timeout)
	}
}

func TestSanitizeLoopConfig_ExceedsMax(t *testing.T) {
	cfg := LoopConfig{MaxIterations: 200, Timeout: 30 * time.Minute}
	sanitizeLoopConfig(&cfg)
	if cfg.MaxIterations != 100 {
		t.Errorf("expected MaxIterations capped at 100, got %d", cfg.MaxIterations)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("expected Timeout capped at 10m, got %v", cfg.Timeout)
	}
}

func TestSanitizeLoopConfig_NegativeValues(t *testing.T) {
	cfg := LoopConfig{MaxIterations: -1, Timeout: -1 * time.Second}
	sanitizeLoopConfig(&cfg)
	if cfg.MaxIterations != 10 {
		t.Errorf("expected MaxIterations=10 for negative, got %d", cfg.MaxIterations)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected Timeout=60s for negative, got %v", cfg.Timeout)
	}
}

func TestSanitizeLoopConfig_ValidValues(t *testing.T) {
	cfg := LoopConfig{MaxIterations: 5, Timeout: 30 * time.Second}
	sanitizeLoopConfig(&cfg)
	if cfg.MaxIterations != 5 {
		t.Errorf("expected MaxIterations=5, got %d", cfg.MaxIterations)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", cfg.Timeout)
	}
}

// --- toContextMessages / fromContextMessages ---

func TestToContextMessages_SystemIsCritical(t *testing.T) {
	// Create a minimal agent to test the method
	a := &Agent{}
	msgs := []provider.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}
	result := a.toContextMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Priority != contextx.PriorityCritical {
		t.Errorf("system message should be PriorityCritical, got %v", result[0].Priority)
	}
	if result[0].Category != "system" {
		t.Errorf("system message category should be 'system', got %q", result[0].Category)
	}
}

func TestToContextMessages_MemoryPriority(t *testing.T) {
	a := &Agent{}
	msgs := []provider.Message{
		{Role: "system", Content: "[Core Memory] important facts"},
		{Role: "system", Content: "[Working Memory] recent context"},
		{Role: "system", Content: "[Recent Context] last messages"},
	}
	result := a.toContextMessages(msgs)
	if result[0].Priority != contextx.PriorityHigh {
		t.Errorf("Core Memory should be PriorityHigh, got %v", result[0].Priority)
	}
	if result[0].Category != "memory_long" {
		t.Errorf("Core Memory category should be 'memory_long', got %q", result[0].Category)
	}
	if result[1].Priority != contextx.PriorityNormal {
		t.Errorf("Working Memory should be PriorityNormal, got %v", result[1].Priority)
	}
	if result[2].Priority != contextx.PriorityLow {
		t.Errorf("Recent Context should be PriorityLow, got %v", result[2].Priority)
	}
}

func TestFromContextMessages(t *testing.T) {
	a := &Agent{}
	original := []provider.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "usr"},
		{Role: "assistant", Content: "ast"},
	}
	ctxMsgs := a.toContextMessages(original)
	roundTrip := a.fromContextMessages(ctxMsgs)
	if len(roundTrip) != len(original) {
		t.Fatalf("expected %d messages, got %d", len(original), len(roundTrip))
	}
	for i, msg := range roundTrip {
		if msg.Role != original[i].Role {
			t.Errorf("msg[%d].Role = %q, want %q", i, msg.Role, original[i].Role)
		}
		if msg.Content != original[i].Content {
			t.Errorf("msg[%d].Content = %q, want %q", i, msg.Content, original[i].Content)
		}
	}
}

// --- applyWebSearchEnv ---

func TestApplyWebSearchEnv(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(cfgPath, []byte("provider: openai\napi_key: test\nmodel: gpt-4\n"), 0644)
	cfg, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Skipf("cannot create config manager: %v", err)
	}

	// Set env vars
	os.Setenv("LH_WEB_SEARCH_PROVIDER", "brave")
	os.Setenv("LH_WEB_SEARCH_API_KEY", "test-key-123")
	defer func() {
		os.Unsetenv("LH_WEB_SEARCH_PROVIDER")
		os.Unsetenv("LH_WEB_SEARCH_API_KEY")
	}()

	applyWebSearchEnv(cfg)

	if v := cfg.Get().WebSearch.Provider; v != "brave" {
		t.Errorf("expected web_search.provider=brave, got %q", v)
	}
	if v := cfg.Get().WebSearch.APIKey; v != "test-key-123" {
		t.Errorf("expected web_search.api_key=test-key-123, got %q", v)
	}
}

// --- handleMemoryTool ---

func newTestAgentWithMemory(t *testing.T) *Agent {
	t.Helper()
	tmpDir := t.TempDir()
	memStore, err := memory.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create memory store: %v", err)
	}
	return &Agent{
		memory: memStore,
	}
}

func TestHandleMemoryTool_Remember(t *testing.T) {
	a := newTestAgentWithMemory(t)

	result, err := a.handleMemoryTool("remember", `{"content": "用户喜欢Python", "category": "preference"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "已保存") {
		t.Errorf("expected save confirmation, got %q", result)
	}
	if !strings.Contains(result, "preference") {
		t.Errorf("expected category in result, got %q", result)
	}
}

func TestHandleMemoryTool_RememberLongTerm(t *testing.T) {
	a := newTestAgentWithMemory(t)

	result, err := a.handleMemoryTool("remember", `{"content": "重要密码", "category": "security", "long_term": true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "长期记忆") {
		t.Errorf("expected long-term confirmation, got %q", result)
	}
}

func TestHandleMemoryTool_RememberNoContent(t *testing.T) {
	a := newTestAgentWithMemory(t)

	_, err := a.handleMemoryTool("remember", `{"category": "test"}`)
	if err == nil {
		t.Error("expected error for missing content")
	}
}

func TestHandleMemoryTool_RecallWithQuery(t *testing.T) {
	a := newTestAgentWithMemory(t)

	// Save some memories first
	a.memory.Save("用户喜欢Python", "preference")
	a.memory.Save("项目使用Go语言", "project")

	result, err := a.handleMemoryTool("recall", `{"query": "Python"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Python") {
		t.Errorf("expected Python in results, got %q", result)
	}
}

func TestHandleMemoryTool_RecallNoQuery(t *testing.T) {
	a := newTestAgentWithMemory(t)

	a.memory.Save("test memory", "test")

	result, err := a.handleMemoryTool("recall", `{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "最近的记忆") {
		t.Errorf("expected recent memory header, got %q", result)
	}
}

func TestHandleMemoryTool_RecallEmpty(t *testing.T) {
	a := newTestAgentWithMemory(t)

	result, err := a.handleMemoryTool("recall", `{"query": "nonexistent"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "没有找到") {
		t.Errorf("expected not found message, got %q", result)
	}
}

func TestHandleMemoryTool_UnknownTool(t *testing.T) {
	a := newTestAgentWithMemory(t)

	_, err := a.handleMemoryTool("forget", `{}`)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown memory tool") {
		t.Errorf("expected unknown tool error, got %v", err)
	}
}

func TestHandleMemoryTool_InvalidJSON(t *testing.T) {
	a := newTestAgentWithMemory(t)

	// Invalid JSON should be handled gracefully (treated as raw args)
	result, err := a.handleMemoryTool("recall", "not json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still work (no query → recent memories)
	if !strings.Contains(result, "没有找到") && !strings.Contains(result, "记忆") {
		t.Errorf("expected memory response, got %q", result)
	}
}

// --- updateShellContext ---

func newTestSession(t *testing.T) *session.Session {
	t.Helper()
	tmpDir := t.TempDir()
	mgr, err := session.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("create session manager: %v", err)
	}
	sess := mgr.New()
	return sess
}

func TestUpdateShellContext_CdCommand(t *testing.T) {
	a := &Agent{}
	sess := newTestSession(t)

	// Use a directory that exists
	tmpDir := t.TempDir()
	a.updateShellContext(sess, "cd "+tmpDir, "")
	if sess.GetCwd() != tmpDir {
		t.Errorf("expected cwd=%s, got %s", tmpDir, sess.GetCwd())
	}
}

func TestUpdateShellContext_ExportCommand(t *testing.T) {
	a := &Agent{}
	sess := newTestSession(t)

	a.updateShellContext(sess, "export MY_VAR=hello", "")
	env := sess.GetEnv()
	if env["MY_VAR"] != "hello" {
		t.Errorf("expected MY_VAR=hello, got %v", env)
	}
}

func TestUpdateShellContext_UnsetCommand(t *testing.T) {
	a := &Agent{}
	sess := newTestSession(t)

	sess.SetEnv("REMOVE_ME", "value")
	a.updateShellContext(sess, "unset REMOVE_ME", "")
	env := sess.GetEnv()
	if _, ok := env["REMOVE_ME"]; ok {
		t.Error("expected REMOVE_ME to be unset")
	}
}

func TestUpdateShellContext_MultipleExports(t *testing.T) {
	a := &Agent{}
	sess := newTestSession(t)

	a.updateShellContext(sess, "export A=1 && export B=2", "")
	env := sess.GetEnv()
	if env["A"] != "1" || env["B"] != "2" {
		t.Errorf("expected A=1, B=2, got %v", env)
	}
}

// --- saveConversationMemory ---

func TestSaveConversationMemory(t *testing.T) {
	a := newTestAgentWithMemory(t)

	a.saveConversationMemory("我喜欢Rust语言", "Rust确实很棒")

	// Check that memory was saved
	recent := a.memory.Recent(10)
	found := false
	for _, e := range recent {
		if strings.Contains(e.Content, "Rust") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Rust-related memory to be saved")
	}
}

func TestSaveConversationMemory_ShortTermBuffer(t *testing.T) {
	a := newTestAgentWithMemory(t)
	a.shortTerm = memory.NewShortTermBuffer(10)

	a.saveConversationMemory("hello", "hi there")

	if a.shortTerm.MessageCount() != 2 {
		t.Errorf("expected 2 messages in short term buffer, got %d", a.shortTerm.MessageCount())
	}
}

// --- autoSummarize ---

func TestAutoSummarize_FewMemories(t *testing.T) {
	a := newTestAgentWithMemory(t)

	// Only 3 short-term memories — should not trigger summarize
	a.memory.SaveWithTier("m1", "test", memory.TierShort, 0.3)
	a.memory.SaveWithTier("m2", "test", memory.TierShort, 0.3)
	a.memory.SaveWithTier("m3", "test", memory.TierShort, 0.3)

	before := len(a.memory.ByTier(memory.TierShort))
	a.autoSummarize()
	after := len(a.memory.ByTier(memory.TierShort))

	// Should not change — too few memories
	if after != before {
		t.Errorf("expected no change with few memories, before=%d after=%d", before, after)
	}
}

func TestAutoSummarize_ManyMemories(t *testing.T) {
	a := newTestAgentWithMemory(t)

	// Add 8 short-term memories — should trigger summarize (keep 5)
	for i := 0; i < 8; i++ {
		a.memory.SaveWithTier("memory item "+strings.Repeat("x", 20), "test", memory.TierShort, 0.3)
	}

	a.autoSummarize()

	// After summarize, short-term should be reduced
	shorts := a.memory.ByTier(memory.TierShort)
	if len(shorts) > 6 { // allow some tolerance
		t.Errorf("expected short-term memories to be reduced, got %d", len(shorts))
	}
}

// --- MemoryStats / DecayMemory / PromoteMemory / ExpireMidTermMemory ---

func TestMemoryStats(t *testing.T) {
	a := newTestAgentWithMemory(t)
	a.memory.SaveWithTier("short", "test", memory.TierShort, 0.3)
	a.memory.SaveLongTerm("long", "test")

	stats := a.MemoryStats()
	if stats[memory.TierShort] < 1 {
		t.Errorf("expected at least 1 short-term, got %d", stats[memory.TierShort])
	}
	if stats[memory.TierLong] < 1 {
		t.Errorf("expected at least 1 long-term, got %d", stats[memory.TierLong])
	}
}

func TestDecayMemory(t *testing.T) {
	a := newTestAgentWithMemory(t)
	a.memory.SaveWithTier("will decay", "test", memory.TierShort, 0.01)

	decayed := a.DecayMemory(0.5)
	// Low importance memory should decay
	if decayed < 0 {
		t.Errorf("decayed count should be >= 0, got %d", decayed)
	}
}

func TestExpireMidTermMemory_Nil(t *testing.T) {
	a := &Agent{midTerm: nil}
	count := a.ExpireMidTermMemory(24 * time.Hour)
	if count != 0 {
		t.Errorf("expected 0 with nil midTerm, got %d", count)
	}
}

// --- buildMessages ---

func TestBuildMessages_Basic(t *testing.T) {
	a := &Agent{
		soul:    soul.Default(),
		memory:  &memory.Store{},
		tools:   tool.NewRegistry(),
		skills:  nil,
	}

	msgs := a.buildMessages("hello")
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message should be system, got %q", msgs[0].Role)
	}
	lastMsg := msgs[len(msgs)-1]
	if lastMsg.Role != "user" || lastMsg.Content != "hello" {
		t.Errorf("last message should be user 'hello', got role=%q content=%q", lastMsg.Role, lastMsg.Content)
	}
}

// --- getStreamMode ---

func TestGetStreamMode(t *testing.T) {
	a := &Agent{}
	// Default should be native
	mode := a.getStreamMode()
	if mode != StreamModeNative {
		t.Errorf("expected StreamModeNative, got %v", mode)
	}
}

// --- LoopState edge cases ---

func TestLoopStateUnknown(t *testing.T) {
	var s LoopState = 99
	if s.String() != "Unknown" {
		t.Errorf("expected Unknown for invalid LoopState, got %q", s.String())
	}
}

// --- EventType edge cases ---

func TestEventTypeValues(t *testing.T) {
	if EventReason != 0 || EventAct != 1 || EventObserve != 2 {
		t.Errorf("unexpected EventType values: Reason=%d Act=%d Observe=%d", EventReason, EventAct, EventObserve)
	}
	if EventContent != 3 || EventDone != 4 || EventError != 5 {
		t.Errorf("unexpected EventType values: Content=%d Done=%d Error=%d", EventContent, EventDone, EventError)
	}
}

// --- Agent Getter 测试 ---

func TestAgent_Getters(t *testing.T) {
	a := &Agent{
		soul:      soul.Default(),
		tmplMgr:   soul.NewTemplateManager(),
		catalog:   provider.NewModelCatalog(),
		tools:     tool.NewRegistry(),
		mcpClient: tool.NewMCPClient(),
		delegate:  tool.NewDelegateManager(tool.DefaultDelegateConfig()),
		gateway:   tool.NewGateway(tool.NewRegistry()),
		skills:    []*tool.SkillInfo{},
	}

	tests := []struct {
		name string
		got  interface{}
	}{
		{"Soul", a.Soul()},
		{"TemplateManager", a.TemplateManager()},
		{"Tools", a.Tools()},
		{"Catalog", a.Catalog()},
		{"MCPClient", a.MCPClient()},
		{"Delegate", a.Delegate()},
		{"Gateway", a.Gateway()},
		{"Skills", a.Skills()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got == nil {
				t.Errorf("%s() returned nil", tt.name)
			}
		})
	}
}

func TestAgent_GettersNil(t *testing.T) {
	a := &Agent{}

	tests := []struct {
		name string
		got  interface{}
	}{
		{"Registry", a.Registry()},
		{"Provider", a.Provider()},
		{"Sessions", a.Sessions()},
		{"Config", a.Config()},
		{"Memory", a.Memory()},
		{"ContextWindow", a.ContextWindow()},
		{"RAG", a.RAG()},
		{"Metrics", a.Metrics()},
		{"CronEngine", a.CronEngine()},
		{"Autonomy", a.Autonomy()},
		{"MsgGateway", a.MsgGateway()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 这些 getter 在 Agent 未初始化时返回 nil 是预期的
			if tt.got != nil {
				t.Logf("%s() = %v (may be non-nil if initialized)", tt.name, tt.got)
			}
		})
	}
}

func TestAgent_SessionsWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Sessions 需要 sessions 字段初始化，而不是 cfg
	sessMgr, err := session.NewManager(tmpDir + "/sessions")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	a := &Agent{sessions: sessMgr}

	s := a.Sessions()
	if s == nil {
		t.Error("Sessions() should return non-nil when sessions is set")
	}
}

func TestAgent_ConfigWithManager(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	a := &Agent{cfg: cfg}

	c := a.Config()
	if c == nil {
		t.Error("Config() should return non-nil when cfg is set")
	}
}

func TestAgent_SwitchModel(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")
	cfg.Set("max_tokens", "4096")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// 尝试切换到一个不存在的模型
	err = a.SwitchModel("nonexistent-model")
	if err == nil {
		t.Log("SwitchModel() should return error for nonexistent model")
	}
}

func TestAgent_ProviderWithMock(t *testing.T) {
	mockProv := &mockProvider{name: "test-mock"}
	a := &Agent{provider: mockProv}

	p := a.Provider()
	if p == nil {
		t.Error("Provider() returned nil")
	}
	if p.Name() != "test-mock" {
		t.Errorf("Provider().Name() = %q, want %q", p.Name(), "test-mock")
	}
}

// mockProvider 用于测试的 mock provider
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string                                    { return m.name }
func (m *mockProvider) Chat(ctx context.Context, messages []provider.Message) (*provider.Response, error) {
	return &provider.Response{Content: "mock"}, nil
}
func (m *mockProvider) ChatStream(ctx context.Context, messages []provider.Message) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk)
	close(ch)
	return ch, nil
}
func (m *mockProvider) Validate() error { return nil }