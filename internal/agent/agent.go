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
	cfg      *config.Manager
	soul     *soul.Soul
	provider provider.Provider
	memory   *memory.Store
	sessions *session.Manager
	tools    *tool.Registry
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

	// 创建 Provider
	registry := provider.NewRegistry()
	pCfg := provider.Config{
		Name:        c.Provider,
		APIKey:      c.APIKey,
		APIBase:     c.APIBase,
		Model:       c.Model,
		MaxTokens:   c.MaxTokens,
		Temperature: c.Temperature,
	}
	p, err := registry.Resolve(pCfg)
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
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
		cfg:      cfg,
		soul:     s,
		provider: p,
		memory:   mem,
		sessions: sessions,
		tools:    tool.NewRegistry(),
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
