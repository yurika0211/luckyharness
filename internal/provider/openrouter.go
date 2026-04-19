package provider

import (
	"context"
	"fmt"
)

// --- OpenRouter Provider ---

// OpenRouterProvider 实现 OpenRouter API 调用
// OpenRouter 兼容 OpenAI API 格式，但有自己的 base URL 和 header
type OpenRouterProvider struct {
	cfg Config
}

func NewOpenRouterProvider(cfg Config) Provider {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://openrouter.ai/api/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "openai/gpt-4o"
	}
	return &OpenRouterProvider{cfg: cfg}
}

func (p *OpenRouterProvider) Name() string { return "openrouter" }

func (p *OpenRouterProvider) Validate() error {
	if p.cfg.APIKey == "" {
		return fmt.Errorf("openrouter: api_key is required")
	}
	return nil
}

func (p *OpenRouterProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	// OpenRouter 兼容 OpenAI API，复用 callOpenAI
	return callOpenAI(p.cfg, messages)
}

func (p *OpenRouterProvider) ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	return callOpenAIStream(ctx, p.cfg, messages)
}

// Ensure OpenRouterProvider implements Provider
var _ Provider = (*OpenRouterProvider)(nil)
