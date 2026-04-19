package provider

import (
	"context"
	"fmt"
	"io"
)

// Message 代表一条对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response 代表 Provider 的响应
type Response struct {
	Content    string
	TokensUsed int
	Model      string
	FinishReason string
}

// StreamChunk 代表流式响应的一个片段
type StreamChunk struct {
	Content   string
	Done      bool
	Model     string
}

// Provider 是 LLM 提供商的统一接口
type Provider interface {
	// Name 返回提供商名称
	Name() string

	// Chat 发送消息并获取完整响应
	Chat(ctx context.Context, messages []Message) (*Response, error)

	// ChatStream 发送消息并获取流式响应
	ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)

	// Validate 验证配置是否有效
	Validate() error
}

// Config 是 Provider 的配置
type Config struct {
	Name      string
	APIKey    string
	APIBase   string
	Model     string
	MaxTokens int
	Temperature float64
}

// Registry 管理所有已注册的 Provider
type Registry struct {
	providers map[string]Provider
	factories map[string]func(Config) Provider
}

// NewRegistry 创建 Provider 注册表
func NewRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
		factories: make(map[string]func(Config) Provider),
	}
	// 注册内置 Provider
	r.RegisterFactory("openai", NewOpenAIProvider)
	r.RegisterFactory("openai-compatible", NewOpenAICompatibleProvider)
	return r
}

// RegisterFactory 注册 Provider 工厂函数
func (r *Registry) RegisterFactory(name string, factory func(Config) Provider) {
	r.factories[name] = factory
}

// Create 创建 Provider 实例
func (r *Registry) Create(name string, cfg Config) (Provider, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s (available: %v)", name, r.Available())
	}
	p := factory(cfg)
	r.providers[name] = p
	return p, nil
}

// Get 获取已创建的 Provider
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not created: %s", name)
	}
	return p, nil
}

// Available 返回所有可用的 Provider 名称
func (r *Registry) Available() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// Resolve 根据 Config 自动解析并创建 Provider
func (r *Registry) Resolve(cfg Config) (Provider, error) {
	name := cfg.Name
	if name == "" {
		name = "openai"
	}

	// 已创建则复用
	if p, ok := r.providers[name]; ok {
		return p, nil
	}

	return r.Create(name, cfg)
}

// --- OpenAI Provider ---

// OpenAIProvider 实现 OpenAI API 调用
type OpenAIProvider struct {
	cfg Config
}

// NewOpenAIProvider 创建 OpenAI Provider
func NewOpenAIProvider(cfg Config) Provider {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}
	return &OpenAIProvider{cfg: cfg}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Validate() error {
	if p.cfg.APIKey == "" {
		return fmt.Errorf("openai: api_key is required")
	}
	return nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	// v0.1.0: 使用 OpenAI-compatible 的通用实现
	return NewOpenAICompatibleProvider(p.cfg).Chat(ctx, messages)
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	return NewOpenAICompatibleProvider(p.cfg).ChatStream(ctx, messages)
}

// --- OpenAI-Compatible Provider ---

// OpenAICompatibleProvider 兼容 OpenAI API 格式的通用 Provider
type OpenAICompatibleProvider struct {
	cfg Config
}

// NewOpenAICompatibleProvider 创建兼容 OpenAI API 的 Provider
func NewOpenAICompatibleProvider(cfg Config) Provider {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}
	return &OpenAICompatibleProvider{cfg: cfg}
}

func (p *OpenAICompatibleProvider) Name() string { return "openai-compatible" }

func (p *OpenAICompatibleProvider) Validate() error {
	if p.cfg.APIKey == "" {
		return fmt.Errorf("%s: api_key is required", p.cfg.Name)
	}
	if p.cfg.APIBase == "" {
		return fmt.Errorf("%s: api_base is required", p.cfg.Name)
	}
	return nil
}

func (p *OpenAICompatibleProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	// v0.1.0: 基础 HTTP 实现
	// 完整实现将在 v0.2.0 补充流式和非流式调用
	return nil, fmt.Errorf("Chat: not yet implemented (use ChatStream)")
}

func (p *OpenAICompatibleProvider) ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 64)

	go func() {
		defer close(ch)
		// v0.1.0: 占位实现
		// v0.2.0 将实现完整的 SSE 流式调用
		ch <- StreamChunk{
			Content: "LuckyHarness v0.1.0 — streaming not yet connected to API. Configure provider and upgrade to v0.2.0.",
			Done:    true,
			Model:   p.cfg.Model,
		}
	}()

	return ch, nil
}

// Ensure interfaces are satisfied
var (
	_ Provider = (*OpenAIProvider)(nil)
	_ Provider = (*OpenAICompatibleProvider)(nil)
	_ io.Closer = (*Registry)(nil)
)

func (r *Registry) Close() error {
	return nil
}
