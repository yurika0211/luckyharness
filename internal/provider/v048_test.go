package provider

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- v0.48.0 Provider Package Tests ---

// mockFCProvider implements both Provider and FunctionCallingProvider
type mockFCProvider struct {
	name         string
	chatErr      error
	streamErr    error
	validateErr  error
	chatResp     *Response
	streamChunks []StreamChunk
}

func (m *mockFCProvider) Name() string { return m.name }

func (m *mockFCProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	if m.chatResp != nil {
		return m.chatResp, nil
	}
	return &Response{Content: "mock response", Model: m.name}, nil
}

func (m *mockFCProvider) ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	ch := make(chan StreamChunk, len(m.streamChunks)+1)
	if len(m.streamChunks) > 0 {
		for _, sc := range m.streamChunks {
			ch <- sc
		}
	} else {
		ch <- StreamChunk{Content: "mock stream", Done: true, Model: m.name}
	}
	close(ch)
	return ch, nil
}

func (m *mockFCProvider) Validate() error {
	return m.validateErr
}

func (m *mockFCProvider) ChatWithOptions(ctx context.Context, messages []Message, opts CallOptions) (*Response, error) {
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	if m.chatResp != nil {
		return m.chatResp, nil
	}
	return &Response{Content: "mock fc response", Model: m.name}, nil
}

func (m *mockFCProvider) ChatStreamWithOptions(ctx context.Context, messages []Message, opts CallOptions) (<-chan StreamChunk, error) {
	return m.ChatStream(ctx, messages)
}

// --- Registry Tests ---

func TestRegistryCreateAndGet(t *testing.T) {
	r := NewRegistry()
	cfg := Config{Name: "openai", APIKey: "sk-test", Model: "gpt-4o"}

	p, err := r.Create("openai", cfg)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected openai, got %s", p.Name())
	}

	// Get should return same instance
	p2, err := r.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p2 != p {
		t.Error("expected same instance from Get")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for Get of uncreated provider")
	}
}

func TestRegistryResolveDefaultName(t *testing.T) {
	r := NewRegistry()
	cfg := Config{APIKey: "sk-test"} // Name is empty, should default to "openai"

	p, err := r.Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve with empty name: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected default to openai, got %s", p.Name())
	}
}

func TestRegistryClose(t *testing.T) {
	r := NewRegistry()
	if err := r.Close(); err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

func TestRegistryRegisterFactory(t *testing.T) {
	r := NewRegistry()
	r.RegisterFactory("custom", func(cfg Config) Provider {
		return &mockProvider{name: "custom"}
	})

	available := r.Available()
	found := false
	for _, name := range available {
		if name == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'custom' in available providers")
	}

	p, err := r.Create("custom", Config{})
	if err != nil {
		t.Fatalf("Create custom: %v", err)
	}
	if p.Name() != "custom" {
		t.Errorf("expected custom, got %s", p.Name())
	}
}

// --- Provider Constructor Tests ---

func TestOpenAIProviderCustomConfig(t *testing.T) {
	p := NewOpenAIProvider(Config{
		APIBase: "http://localhost:8080/v1",
		Model:   "gpt-3.5-turbo",
		APIKey:  "sk-test",
	})
	op := p.(*OpenAIProvider)
	if op.cfg.APIBase != "http://localhost:8080/v1" {
		t.Errorf("expected custom APIBase, got %s", op.cfg.APIBase)
	}
	if op.cfg.Model != "gpt-3.5-turbo" {
		t.Errorf("expected custom model, got %s", op.cfg.Model)
	}
}

func TestOpenAICompatibleProviderCustomConfig(t *testing.T) {
	p := NewOpenAICompatibleProvider(Config{
		Name:    "my-provider",
		APIBase: "http://my-api.example.com/v1",
		Model:   "my-model",
		APIKey:  "sk-test",
	})
	cp := p.(*OpenAICompatibleProvider)
	if cp.cfg.APIBase != "http://my-api.example.com/v1" {
		t.Errorf("expected custom APIBase, got %s", cp.cfg.APIBase)
	}
	if cp.cfg.Model != "my-model" {
		t.Errorf("expected custom model, got %s", cp.cfg.Model)
	}
}

func TestOpenAICompatibleProviderValidateNoBase(t *testing.T) {
	p := NewOpenAICompatibleProvider(Config{
		Name:   "test",
		APIKey: "sk-test",
		// APIBase will be set to default, so Validate should pass
	})
	if err := p.Validate(); err != nil {
		t.Errorf("expected valid with default APIBase, got: %v", err)
	}
}

func TestAnthropicProviderCustomConfig(t *testing.T) {
	p := NewAnthropicProvider(Config{
		APIBase: "http://anthropic-proxy.example.com",
		Model:   "claude-3-haiku-20240307",
		APIKey:  "sk-ant-test",
	})
	ap := p.(*AnthropicProvider)
	if ap.cfg.APIBase != "http://anthropic-proxy.example.com" {
		t.Errorf("expected custom APIBase, got %s", ap.cfg.APIBase)
	}
	if ap.cfg.Model != "claude-3-haiku-20240307" {
		t.Errorf("expected custom model, got %s", ap.cfg.Model)
	}
}

func TestOllamaProviderCustomConfig(t *testing.T) {
	p := NewOllamaProvider(Config{
		APIBase: "http://my-ollama:11434",
		Model:   "mistral",
	})
	op := p.(*OllamaProvider)
	if op.cfg.APIBase != "http://my-ollama:11434" {
		t.Errorf("expected custom APIBase, got %s", op.cfg.APIBase)
	}
	if op.cfg.Model != "mistral" {
		t.Errorf("expected custom model, got %s", op.cfg.Model)
	}
}

func TestOpenRouterProviderCustomConfig(t *testing.T) {
	p := NewOpenRouterProvider(Config{
		APIBase: "http://openrouter-proxy.example.com/v1",
		Model:   "anthropic/claude-3.5-sonnet",
		APIKey:  "sk-or-test",
	})
	op := p.(*OpenRouterProvider)
	if op.cfg.APIBase != "http://openrouter-proxy.example.com/v1" {
		t.Errorf("expected custom APIBase, got %s", op.cfg.APIBase)
	}
	if op.cfg.Model != "anthropic/claude-3.5-sonnet" {
		t.Errorf("expected custom model, got %s", op.cfg.Model)
	}
}

// --- FallbackChain Advanced Tests ---

func TestFallbackChainActiveProvider(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)

	ap := chain.ActiveProvider()
	if ap.Name() != "mock1" {
		t.Errorf("expected active provider mock1, got %s", ap.Name())
	}
}

func TestFallbackChainActiveIndex(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	if chain.ActiveIndex() != 0 {
		t.Errorf("expected active index 0, got %d", chain.ActiveIndex())
	}
}

func TestFallbackChainChainNames(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	names := chain.ChainNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "mock1" || names[1] != "mock2" {
		t.Errorf("expected [mock1, mock2], got %v", names)
	}
}

func TestFallbackChainResetAllCooldowns(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	chain.maxFails = 1
	chain.cooldown = 10 * time.Minute

	// Put both in cooldown
	chain.mu.Lock()
	chain.failCounts[0] = 3
	chain.cooldownAt[0] = time.Now().Add(10 * time.Minute)
	chain.failCounts[1] = 3
	chain.cooldownAt[1] = time.Now().Add(10 * time.Minute)
	chain.active = 1
	chain.mu.Unlock()

	if chain.isAvailable(0) || chain.isAvailable(1) {
		t.Error("both should be in cooldown")
	}

	chain.ResetAllCooldowns()

	if !chain.isAvailable(0) || !chain.isAvailable(1) {
		t.Error("both should be available after ResetAllCooldowns")
	}
	if chain.ActiveIndex() != 0 {
		t.Errorf("expected active index reset to 0, got %d", chain.ActiveIndex())
	}
}

func TestFallbackChainEmptyConfigs(t *testing.T) {
	registry := NewRegistry()
	_, err := NewFallbackChain(nil, registry)
	if err == nil {
		t.Error("expected error for empty configs")
	}
}

func TestFallbackChainStreamFallback(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1", streamErr: fmt.Errorf("stream error")}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	ch, err := chain.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var content string
	for chunk := range ch {
		content += chunk.Content
	}
	if content != "mock stream" {
		t.Errorf("expected 'mock stream', got %s", content)
	}
}

func TestFallbackChainAllStreamFail(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1", streamErr: fmt.Errorf("fail")}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2", streamErr: fmt.Errorf("fail")}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	_, err := chain.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error when all stream providers fail")
	}
}

func TestFallbackChainChatWithOptions(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockFCProvider{name: "mock1"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	resp, err := chain.ChatWithOptions(context.Background(), []Message{{Role: "user", Content: "hi"}}, CallOptions{
		Tools: []map[string]any{
			{"function": map[string]any{"name": "test_tool", "description": "A test tool"}},
		},
	})
	if err != nil {
		t.Fatalf("ChatWithOptions: %v", err)
	}
	if resp.Content != "mock fc response" {
		t.Errorf("expected 'mock fc response', got %s", resp.Content)
	}
}

func TestFallbackChainChatWithOptionsFallback(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockFCProvider{name: "mock1", chatErr: fmt.Errorf("fail")}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockFCProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	resp, err := chain.ChatWithOptions(context.Background(), []Message{{Role: "user", Content: "hi"}}, CallOptions{
		Tools: []map[string]any{
			{"function": map[string]any{"name": "test_tool", "description": "A test tool"}},
		},
	})
	if err != nil {
		t.Fatalf("ChatWithOptions: %v", err)
	}
	if resp.Model != "mock2" {
		t.Errorf("expected fallback to mock2, got %s", resp.Model)
	}
}

func TestFallbackChainChatWithOptionsNoTools(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockFCProvider{name: "mock1"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	// No tools → should fall back to regular Chat
	resp, err := chain.ChatWithOptions(context.Background(), []Message{{Role: "user", Content: "hi"}}, CallOptions{})
	if err != nil {
		t.Fatalf("ChatWithOptions no tools: %v", err)
	}
	if resp.Content != "mock response" {
		t.Errorf("expected 'mock response', got %s", resp.Content)
	}
}

func TestFallbackChainChatStreamWithOptions(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockFCProvider{name: "mock1"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	ch, err := chain.ChatStreamWithOptions(context.Background(), []Message{{Role: "user", Content: "hi"}}, CallOptions{
		Tools: []map[string]any{
			{"function": map[string]any{"name": "test_tool", "description": "A test tool"}},
		},
	})
	if err != nil {
		t.Fatalf("ChatStreamWithOptions: %v", err)
	}

	var content string
	for chunk := range ch {
		content += chunk.Content
	}
	if content != "mock stream" {
		t.Errorf("expected 'mock stream', got %s", content)
	}
}

func TestFallbackChainChatStreamWithOptionsFallback(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockFCProvider{name: "mock1", streamErr: fmt.Errorf("fail")}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockFCProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	ch, err := chain.ChatStreamWithOptions(context.Background(), []Message{{Role: "user", Content: "hi"}}, CallOptions{
		Tools: []map[string]any{
			{"function": map[string]any{"name": "test_tool", "description": "A test tool"}},
		},
	})
	if err != nil {
		t.Fatalf("ChatStreamWithOptions: %v", err)
	}

	var content string
	for chunk := range ch {
		content += chunk.Content
	}
	if content != "mock stream" {
		t.Errorf("expected 'mock stream' from mock2, got %s", content)
	}
}

func TestFallbackChainRecordSuccessSwitchBack(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)

	// Manually set active to index 1
	chain.mu.Lock()
	chain.active = 1
	chain.mu.Unlock()

	if chain.ActiveIndex() != 1 {
		t.Errorf("expected active index 1, got %d", chain.ActiveIndex())
	}

	// Successful call on index 0 should switch back (higher priority)
	chain.recordSuccess(0)

	if chain.ActiveIndex() != 0 {
		t.Errorf("expected active index 0 after success on higher priority, got %d", chain.ActiveIndex())
	}
}

func TestFallbackChainRecordSuccessSwitchForward(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)

	// Active is 0, success on index 1 should switch forward
	chain.recordSuccess(1)

	if chain.ActiveIndex() != 1 {
		t.Errorf("expected active index 1 after success on forward provider, got %d", chain.ActiveIndex())
	}
}

func TestFallbackChainRecordSuccessSameIndex(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)

	// Success on same index should not trigger switch callback
	switched := false
	chain.SetOnSwitch(func(from, to string) {
		switched = true
	})

	chain.recordSuccess(0)

	time.Sleep(50 * time.Millisecond)
	if switched {
		t.Error("should not trigger onSwitch for same index success")
	}
}

func TestFallbackChainAllUnavailable(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1", chatErr: fmt.Errorf("fail")}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2", chatErr: fmt.Errorf("fail")}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	chain.maxFails = 1
	chain.cooldown = 10 * time.Minute

	// Trigger cooldown on both
	chain.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	chain.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})

	// Both should be in cooldown now, nextAvailable should return -1
	idx := chain.nextAvailable()
	if idx != -1 {
		t.Errorf("expected -1 when all unavailable, got %d", idx)
	}
}

func TestFallbackChainIsAvailableOutOfRange(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)

	if chain.isAvailable(-1) {
		t.Error("negative index should not be available")
	}
	if chain.isAvailable(99) {
		t.Error("out of range index should not be available")
	}
}

func TestFallbackChainOnSwitchCallback(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1", chatErr: fmt.Errorf("fail")}
	})
	registry.RegisterFactory("mock2", func(cfg Config) Provider {
		return &mockProvider{name: "mock2"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
		{Name: "mock2", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)
	chain.maxFails = 1

	var mu sync.Mutex
	var fromName, toName string
	switched := false
	chain.SetOnSwitch(func(from, to string) {
		mu.Lock()
		defer mu.Unlock()
		fromName = from
		toName = to
		switched = true
	})

	chain.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !switched {
		t.Error("expected onSwitch callback")
	}
	if fromName != "mock1" || toName != "mock2" {
		t.Errorf("expected switch from mock1 to mock2, got %s to %s", fromName, toName)
	}
}

// --- TokenStore Advanced Tests ---

func TestTokenStoreIsExpiredNoExpiry(t *testing.T) {
	entry := &TokenEntry{
		Provider:    "openai",
		AccessToken: "sk-test",
		// ExpiresAt is zero
	}
	if entry.IsExpired() {
		t.Error("token with no expiry should not be expired")
	}
}

func TestTokenStoreIsExpiringSoonNoExpiry(t *testing.T) {
	entry := &TokenEntry{
		Provider:    "openai",
		AccessToken: "sk-test",
	}
	if entry.IsExpiringSoon() {
		t.Error("token with no expiry should not be expiring soon")
	}
}

func TestTokenStoreIsExpiredFuture(t *testing.T) {
	entry := &TokenEntry{
		Provider:    "openai",
		AccessToken: "sk-test",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	if entry.IsExpired() {
		t.Error("token expiring in 1 hour should not be expired")
	}
}

func TestTokenStoreRefreshIfNeededNotExpiring(t *testing.T) {
	dir := t.TempDir()
	ts, _ := NewTokenStore(dir)

	ts.Set(&TokenEntry{
		Provider:     "openai",
		AccessToken:  "sk-test-token-12345678",
		RefreshToken: "rt-test-refresh-12345678",
		ExpiresAt:    time.Now().Add(1 * time.Hour), // Not expiring soon
	})

	refreshed := false
	ok, err := ts.RefreshIfNeeded("openai", func(rt string) (*TokenEntry, error) {
		refreshed = true
		return &TokenEntry{Provider: "openai", AccessToken: "new-token-12345678"}, nil
	})
	if err != nil {
		t.Fatalf("RefreshIfNeeded: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if refreshed {
		t.Error("should not refresh when not expiring soon")
	}
}

func TestTokenStoreRefreshIfNeededExpiring(t *testing.T) {
	dir := t.TempDir()
	ts, _ := NewTokenStore(dir)

	ts.Set(&TokenEntry{
		Provider:     "openai",
		AccessToken:  "sk-test-token-12345678",
		RefreshToken: "rt-test-refresh-12345678",
		ExpiresAt:    time.Now().Add(3 * time.Minute), // Expiring soon
	})

	ok, err := ts.RefreshIfNeeded("openai", func(rt string) (*TokenEntry, error) {
		if rt != "rt-test-refresh-12345678" {
			t.Errorf("expected correct refresh token, got %s", rt)
		}
		return &TokenEntry{
			Provider:     "openai",
			AccessToken:  "new-access-token-12345678",
			RefreshToken: "new-refresh-token-12345678",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		}, nil
	})
	if err != nil {
		t.Fatalf("RefreshIfNeeded: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}

	// Verify new token is stored
	got, err := ts.Get("openai")
	if err != nil {
		t.Fatalf("Get after refresh: %v", err)
	}
	if got.AccessToken != "new-access-token-12345678" {
		t.Errorf("expected new access token, got %s", got.AccessToken)
	}
}

func TestTokenStoreRefreshIfNeededNoRefreshToken(t *testing.T) {
	dir := t.TempDir()
	ts, _ := NewTokenStore(dir)

	ts.Set(&TokenEntry{
		Provider:    "openai",
		AccessToken: "sk-test-token-12345678",
		ExpiresAt:   time.Now().Add(3 * time.Minute), // Expiring soon
		// No RefreshToken
	})

	ok, err := ts.RefreshIfNeeded("openai", func(rt string) (*TokenEntry, error) {
		return nil, nil
	})
	if ok {
		t.Error("expected ok=false when no refresh token")
	}
	if err == nil {
		t.Error("expected error when no refresh token")
	}
}

func TestTokenStoreRefreshIfNeededNotFound(t *testing.T) {
	dir := t.TempDir()
	ts, _ := NewTokenStore(dir)

	ok, err := ts.RefreshIfNeeded("nonexistent", func(rt string) (*TokenEntry, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("RefreshIfNeeded for nonexistent: %v", err)
	}
	if !ok {
		t.Error("expected ok=true for nonexistent (no refresh needed)")
	}
}

func TestTokenStoreRefreshIfNeededRefreshFails(t *testing.T) {
	dir := t.TempDir()
	ts, _ := NewTokenStore(dir)

	ts.Set(&TokenEntry{
		Provider:     "openai",
		AccessToken:  "sk-test-token-12345678",
		RefreshToken: "rt-test-refresh-12345678",
		ExpiresAt:    time.Now().Add(3 * time.Minute),
	})

	ok, err := ts.RefreshIfNeeded("openai", func(rt string) (*TokenEntry, error) {
		return nil, fmt.Errorf("refresh failed")
	})
	if ok {
		t.Error("expected ok=false when refresh fails")
	}
	if err == nil {
		t.Error("expected error when refresh fails")
	}
}

func TestTokenStorePersistence(t *testing.T) {
	dir := t.TempDir()

	// Create and save
	ts1, _ := NewTokenStore(dir)
	ts1.Set(&TokenEntry{
		Provider:    "openai",
		AccessToken: "sk-test-token-12345678",
	})
	ts1.Set(&TokenEntry{
		Provider:    "anthropic",
		AccessToken: "sk-ant-test-12345678",
	})

	// Load from same dir
	ts2, _ := NewTokenStore(dir)
	got, err := ts2.Get("openai")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.AccessToken != "sk-test-token-12345678" {
		t.Errorf("expected persisted token, got %s", got.AccessToken)
	}

	got2, err := ts2.Get("anthropic")
	if err != nil {
		t.Fatalf("Get anthropic after reload: %v", err)
	}
	if got2.AccessToken != "sk-ant-test-12345678" {
		t.Errorf("expected persisted anthropic token, got %s", got2.AccessToken)
	}
}

func TestTokenStoreListMasking(t *testing.T) {
	dir := t.TempDir()
	ts, _ := NewTokenStore(dir)

	ts.Set(&TokenEntry{
		Provider:     "openai",
		AccessToken:  "short",
		RefreshToken: "rt",
	})

	list := ts.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 token, got %d", len(list))
	}
	// Short tokens should not be masked (len <= 8)
	if list[0].AccessToken != "short" {
		t.Errorf("short token should not be masked, got %s", list[0].AccessToken)
	}
	if list[0].RefreshToken != "rt" {
		t.Errorf("short refresh token should not be masked, got %s", list[0].RefreshToken)
	}
}

func TestTokenStoreGetNotFound(t *testing.T) {
	dir := t.TempDir()
	ts, _ := NewTokenStore(dir)

	_, err := ts.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

// --- ModelCatalog Advanced Tests ---

func TestModelCatalogResolveProviderO1(t *testing.T) {
	catalog := NewModelCatalog()
	result, err := catalog.ResolveProvider("o1-preview")
	if err != nil {
		t.Fatalf("ResolveProvider o1-preview: %v", err)
	}
	if result != "openai" {
		t.Errorf("expected openai for o1-preview, got %s", result)
	}
}

func TestModelCatalogResolveProviderO3(t *testing.T) {
	catalog := NewModelCatalog()
	result, err := catalog.ResolveProvider("o3-mini")
	if err != nil {
		t.Fatalf("ResolveProvider o3-mini: %v", err)
	}
	if result != "openai" {
		t.Errorf("expected openai for o3-mini, got %s", result)
	}
}

func TestModelCatalogRegisterOverwrite(t *testing.T) {
	catalog := NewModelCatalog()

	// Register custom model with same ID as default
	custom := ModelInfo{
		ID:          "gpt-4o",
		Provider:    "custom",
		DisplayName: "Custom GPT-4o",
		Capabilities: []string{"chat"},
	}
	catalog.Register(custom)

	m, err := catalog.Get("gpt-4o")
	if err != nil {
		t.Fatalf("Get gpt-4o: %v", err)
	}
	if m.Provider != "custom" {
		t.Errorf("expected custom provider after overwrite, got %s", m.Provider)
	}
	if m.DisplayName != "Custom GPT-4o" {
		t.Errorf("expected custom display name, got %s", m.DisplayName)
	}
}

func TestModelCatalogListSorted(t *testing.T) {
	catalog := NewModelCatalog()
	models := catalog.List()

	// Verify sorted by provider then ID
	for i := 1; i < len(models); i++ {
		if models[i].Provider < models[i-1].Provider {
			t.Errorf("models not sorted by provider at index %d", i)
		}
		if models[i].Provider == models[i-1].Provider && models[i].ID < models[i-1].ID {
			t.Errorf("models not sorted by ID within provider at index %d", i)
		}
	}
}

func TestModelCatalogListByProviderEmpty(t *testing.T) {
	catalog := NewModelCatalog()
	models := catalog.ListByProvider("nonexistent")
	if len(models) != 0 {
		t.Errorf("expected 0 models for nonexistent provider, got %d", len(models))
	}
}

func TestModelCatalogFindByCapabilityNoMatch(t *testing.T) {
	catalog := NewModelCatalog()
	models := catalog.FindByCapability("nonexistent-capability")
	if len(models) != 0 {
		t.Errorf("expected 0 models for nonexistent capability, got %d", len(models))
	}
}

func TestModelCatalogFindByCapabilityStreaming(t *testing.T) {
	catalog := NewModelCatalog()
	models := catalog.FindByCapability("streaming")
	if len(models) == 0 {
		t.Error("expected streaming-capable models")
	}
}

// --- toOpenAIMessages Edge Cases ---

func TestToOpenAIMessagesEmpty(t *testing.T) {
	result := toOpenAIMessages([]Message{})
	if len(result) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result))
	}
}

func TestToOpenAIMessagesWithEmptyToolCalls(t *testing.T) {
	msgs := []Message{
		{Role: "assistant", Content: "Hello", ToolCalls: []ToolCall{}},
	}
	result := toOpenAIMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	// Empty ToolCalls should not create openai tool_calls
	if len(result[0].ToolCalls) != 0 {
		t.Errorf("expected 0 openai tool_calls for empty ToolCalls, got %d", len(result[0].ToolCalls))
	}
}

func TestToOpenAIMessagesWithToolCallIDAndName(t *testing.T) {
	msgs := []Message{
		{Role: "tool", Content: "result", ToolCallID: "call_123", Name: "get_weather"},
	}
	result := toOpenAIMessages(msgs)
	if result[0].ToolCallID != "call_123" {
		t.Errorf("expected tool_call_id 'call_123', got %s", result[0].ToolCallID)
	}
	if result[0].Name != "get_weather" {
		t.Errorf("expected name 'get_weather', got %s", result[0].Name)
	}
}

// --- StreamParser Edge Cases ---

func TestStreamParserEmptyFeed(t *testing.T) {
	sp := NewStreamParser()

	// Feed with no content and no tool calls
	result := sp.FeedDelta(&openaiDelta{})
	if !result {
		t.Error("expected FeedDelta to return true for empty delta")
	}
	if sp.GetContent() != "" {
		t.Errorf("expected empty content, got '%s'", sp.GetContent())
	}
	if sp.HasToolCalls() {
		t.Error("should not have tool calls from empty delta")
	}
}

func TestStreamParserBuildResponseNotDone(t *testing.T) {
	sp := NewStreamParser()
	sp.FeedDelta(&openaiDelta{Content: "Hello"})

	resp := sp.BuildResponse()
	if resp.FinishReason != "" {
		t.Errorf("expected empty FinishReason when not done, got '%s'", resp.FinishReason)
	}
}

func TestStreamParserGetModelEmpty(t *testing.T) {
	sp := NewStreamParser()
	if sp.GetModel() != "" {
		t.Errorf("expected empty model, got '%s'", sp.GetModel())
	}
}

func TestStreamParserIsDoneInitial(t *testing.T) {
	sp := NewStreamParser()
	if sp.IsDone() {
		t.Error("new parser should not be done")
	}
}

func TestStreamParserFeedDeltaMultipleToolCallsIncremental(t *testing.T) {
	sp := NewStreamParser()

	// First tool call starts
	sp.FeedDelta(&openaiDelta{
		ToolCalls: []deltaToolCall{
			{
				Index: 0,
				ID:    "call_1",
				Type:  "function",
				Function: struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}{Name: "tool_a"},
			},
		},
	})

	// Second tool call starts
	sp.FeedDelta(&openaiDelta{
		ToolCalls: []deltaToolCall{
			{
				Index: 1,
				ID:    "call_2",
				Type:  "function",
				Function: struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}{Name: "tool_b"},
			},
		},
	})

	// Append arguments to first
	sp.FeedDelta(&openaiDelta{
		ToolCalls: []deltaToolCall{
			{
				Index: 0,
				Function: struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}{Arguments: `{"key": "value"}`},
			},
		},
	})

	calls := sp.GetToolCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Name != "tool_a" {
		t.Errorf("expected tool_a, got %s", calls[0].Name)
	}
	if calls[0].Arguments != `{"key": "value"}` {
		t.Errorf("expected arguments for tool_a, got %s", calls[0].Arguments)
	}
	if calls[1].Name != "tool_b" {
		t.Errorf("expected tool_b, got %s", calls[1].Name)
	}
}

// --- CallOptions Tests ---

func TestCallOptionsWithMaxToolCalls(t *testing.T) {
	opts := CallOptions{
		Tools:        []map[string]any{{"function": map[string]any{"name": "test"}}},
		ToolChoice:   "auto",
		MaxToolCalls: 10,
	}
	if opts.MaxToolCalls != 10 {
		t.Errorf("expected MaxToolCalls=10, got %d", opts.MaxToolCalls)
	}
	if len(opts.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(opts.Tools))
	}
}

// --- Response and Message Struct Tests ---

func TestResponseWithToolCalls(t *testing.T) {
	resp := &Response{
		Content:      "",
		Model:        "gpt-4o",
		FinishReason: "tool_calls",
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "add", Arguments: `{"a":1}`},
			{ID: "call_2", Name: "sub", Arguments: `{"b":2}`},
		},
	}
	if len(resp.ToolCalls) != 2 {
		t.Errorf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected call_1, got %s", resp.ToolCalls[0].ID)
	}
}

func TestStreamChunkWithToolCallDeltas(t *testing.T) {
	chunk := StreamChunk{
		Content: "",
		Done:    false,
		Model:   "gpt-4o",
		ToolCallDeltas: []StreamToolCallDelta{
			{Index: 0, ID: "call_1", Name: "search", Arguments: `{"q`},
			{Index: 1, ID: "call_2", Name: "read", Arguments: `{"p`},
		},
	}
	if len(chunk.ToolCallDeltas) != 2 {
		t.Errorf("expected 2 deltas, got %d", len(chunk.ToolCallDeltas))
	}
	if chunk.ToolCallDeltas[0].Index != 0 {
		t.Errorf("expected index 0, got %d", chunk.ToolCallDeltas[0].Index)
	}
}

// --- Config Tests ---

func TestConfigFields(t *testing.T) {
	cfg := Config{
		Name:        "openai",
		APIKey:      "sk-test",
		APIBase:     "http://localhost:8080/v1",
		Model:       "gpt-4o",
		MaxTokens:   4096,
		Temperature: 0.7,
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected MaxTokens=4096, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("expected Temperature=0.7, got %f", cfg.Temperature)
	}
}

// --- FallbackChain Concurrent Access Test ---

func TestFallbackChainConcurrentChat(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterFactory("mock1", func(cfg Config) Provider {
		return &mockProvider{name: "mock1"}
	})

	configs := []FallbackConfig{
		{Name: "mock1", APIKey: "test", Model: "test"},
	}

	chain, _ := NewFallbackChain(configs, registry)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := chain.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent chat error: %v", err)
	}
}