package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/memory"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/session"
	"github.com/yurika0211/luckyharness/internal/soul"
	"github.com/yurika0211/luckyharness/internal/tool"
)

// Agent 是 LuckyHarness 的核心 Agent
type Agent struct {
	cfg          *config.Manager
	soul         *soul.Soul
	provider     provider.Provider       // 当前活跃 provider (可能是 FallbackChain)
	registry     *provider.Registry     // provider 注册表
	catalog      *provider.ModelCatalog  // 模型目录
	tokenStore   *provider.TokenStore    // token 存储
	memory       *memory.Store
	sessions     *session.Manager
	tools        *tool.Registry
}

// New 创建 Agent
func New(cfg *config.Manager) (*Agent, error) {
	c := cfg.Get()

	// 加载 SOUL
	var s *soul.Soul
	soulPath := c.SoulPath
	if soulPath != "" {
		loaded, err := soul.Load(soulPath)
		if err != nil {
			s = soul.Default()
		} else {
			s = loaded
		}
	} else {
		s = soul.Default()
	}

	// 创建 Provider 注册表
	registry := provider.NewRegistry()

	// 创建模型目录
	catalog := provider.NewModelCatalog()

	// 创建 Token 存储
	tokenStore, err := provider.NewTokenStore(cfg.HomeDir() + "/tokens")
	if err != nil {
		tokenStore = nil // 非关键错误
	}

	// 解析 Provider（支持降级链）
	var p provider.Provider
	if len(c.Fallbacks) > 0 {
		// 使用降级链模式
		fallbackConfigs := make([]provider.FallbackConfig, 0, len(c.Fallbacks)+1)
		// 第一个是主 provider
		fallbackConfigs = append(fallbackConfigs, provider.FallbackConfig{
			Name:    c.Provider,
			APIKey:  c.APIKey,
			APIBase: c.APIBase,
			Model:   c.Model,
		})
		// 后续是降级 provider
		for _, fb := range c.Fallbacks {
			fallbackConfigs = append(fallbackConfigs, provider.FallbackConfig{
				Name:    fb.Provider,
				APIKey:  fb.APIKey,
				APIBase: fb.APIBase,
				Model:   fb.Model,
			})
		}
		chain, err := provider.NewFallbackChain(fallbackConfigs, registry)
		if err != nil {
			return nil, fmt.Errorf("create fallback chain: %w", err)
		}
		p = chain
	} else {
		// 单 provider 模式
		pCfg := provider.Config{
			Name:        c.Provider,
			APIKey:      c.APIKey,
			APIBase:     c.APIBase,
			Model:       c.Model,
			MaxTokens:   c.MaxTokens,
			Temperature: c.Temperature,
		}
		p, err = registry.Resolve(pCfg)
		if err != nil {
			return nil, fmt.Errorf("resolve provider: %w", err)
		}
	}

	// 创建记忆存储
	mem, err := memory.NewStore(cfg.HomeDir() + "/memory")
	if err != nil {
		return nil, fmt.Errorf("init memory: %w", err)
	}

	// 创建会话管理器
	sessions, err := session.NewManager(cfg.HomeDir() + "/sessions")
	if err != nil {
		return nil, fmt.Errorf("init sessions: %w", err)
	}

	return &Agent{
		cfg:        cfg,
		soul:       s,
		provider:   p,
		registry:   registry,
		catalog:    catalog,
		tokenStore: tokenStore,
		memory:     mem,
		sessions:   sessions,
		tools:      tool.NewRegistry(),
	}, nil
}

// Chat 执行一次对话
func (a *Agent) Chat(ctx context.Context, userInput string) (string, error) {
	sess := a.sessions.New()

	// 构建消息列表
	messages := []provider.Message{
		{Role: "system", Content: a.soul.SystemPrompt()},
	}

	// 加入记忆上下文
	recent := a.memory.Recent(5)
	if len(recent) > 0 {
		var memCtx strings.Builder
		memCtx.WriteString("[Recent Memory]\n")
		for _, e := range recent {
			memCtx.WriteString("- " + e.Content + "\n")
		}
		messages = append(messages, provider.Message{Role: "system", Content: memCtx.String()})
	}

	// 加入用户消息
	sess.AddMessage("user", userInput)
	messages = append(messages, provider.Message{Role: "user", Content: userInput})

	// 调用 Provider
	ch, err := a.provider.ChatStream(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("chat: %w", err)
	}

	var result strings.Builder
	for chunk := range ch {
		result.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}

	response := result.String()
	sess.AddMessage("assistant", response)

	// 保存会话
	_ = sess.Save()

	return response, nil
}

// ChatStream 执行流式对话
func (a *Agent) ChatStream(ctx context.Context, userInput string) (<-chan provider.StreamChunk, error) {
	sess := a.sessions.New()

	messages := []provider.Message{
		{Role: "system", Content: a.soul.SystemPrompt()},
	}

	recent := a.memory.Recent(5)
	if len(recent) > 0 {
		var memCtx strings.Builder
		memCtx.WriteString("[Recent Memory]\n")
		for _, e := range recent {
			memCtx.WriteString("- " + e.Content + "\n")
		}
		messages = append(messages, provider.Message{Role: "system", Content: memCtx.String()})
	}

	sess.AddMessage("user", userInput)
	messages = append(messages, provider.Message{Role: "user", Content: userInput})

	return a.provider.ChatStream(ctx, messages)
}

// Remember 保存一条记忆
func (a *Agent) Remember(content, category string) error {
	return a.memory.Save(content, category)
}

// Recall 搜索记忆
func (a *Agent) Recall(query string) []memory.Entry {
	return a.memory.Search(query)
}

// Soul 返回当前 SOUL
func (a *Agent) Soul() *soul.Soul {
	return a.soul
}

// Tools 返回工具注册表
func (a *Agent) Tools() *tool.Registry {
	return a.tools
}

// Catalog 返回模型目录
func (a *Agent) Catalog() *provider.ModelCatalog {
	return a.catalog
}

// Provider 返回当前 provider
func (a *Agent) Provider() provider.Provider {
	return a.provider
}

// Registry 返回 provider 注册表
func (a *Agent) Registry() *provider.Registry {
	return a.registry
}

// SwitchModel 切换模型（通过 catalog 推断 provider）
func (a *Agent) SwitchModel(modelID string) error {
	providerName, err := a.catalog.ResolveProvider(modelID)
	if err != nil {
		return fmt.Errorf("resolve provider for model %s: %w", modelID, err)
	}

	cfg := a.cfg.Get()
	pCfg := provider.Config{
		Name:    providerName,
		APIKey:  cfg.APIKey,
		APIBase: cfg.APIBase,
		Model:   modelID,
	}

	p, err := a.registry.Resolve(pCfg)
	if err != nil {
		return fmt.Errorf("create provider %s: %w", providerName, err)
	}

	a.provider = p
	return nil
}
