package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/contextx"
	"github.com/yurika0211/luckyharness/internal/memory"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/rag"
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
	gateway      *tool.Gateway          // 统一工具网关
	mcpClient    *tool.MCPClient         // MCP 客户端
	delegate     *tool.DelegateManager   // 子代理委派管理器
	contextWin   *contextx.ContextWindow // 上下文窗口管理器
	ragManager   *rag.RAGManager         // RAG 知识库管理器
	chatCount    int // 对话计数，用于触发自动摘要
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

	// 创建工具注册表并注册内置工具
	tools := tool.NewRegistry()
	tool.RegisterBuiltinTools(tools)

	// 创建子代理委派管理器
	delegateMgr := tool.NewDelegateManager(tool.DefaultDelegateConfig())
	tools.Register(tool.DelegateTaskTool(delegateMgr))
	tools.Register(tool.TaskStatusTool(delegateMgr))
	tools.Register(tool.ListTasksTool(delegateMgr))

	// 创建 MCP 客户端
	mcpClient := tool.NewMCPClient()

	// 创建统一工具网关
	gateway := tool.NewGateway(tools)

	// 创建上下文窗口管理器
	contextWin := contextx.NewContextWindow(contextx.WindowConfig{
		MaxTokens:            c.MaxTokens,
		ReservedTokens:      c.MaxTokens / 4, // 为回复预留 1/4
		Strategy:             contextx.TrimLowPriority,
		SlidingWindowSize:    10,
		MaxConversationTurns: 50,
		MemoryBudget:         800,
		SummarizeThreshold:  0.8,
	})

	// 创建 RAG 知识库管理器
	ragEmbedder := rag.NewMockEmbedder(128) // v0.14.0: 默认 mock, 后续支持配置 OpenAI
	ragManager := rag.NewRAGManager(ragEmbedder, rag.DefaultRAGConfig())

	return &Agent{
		cfg:        cfg,
		soul:       s,
		provider:   p,
		registry:   registry,
		catalog:    catalog,
		tokenStore: tokenStore,
		memory:     mem,
		sessions:   sessions,
		tools:      tools,
		gateway:    gateway,
		mcpClient:  mcpClient,
		delegate:   delegateMgr,
		contextWin: contextWin,
		ragManager: ragManager,
	}, nil
}

// Chat 执行一次对话
func (a *Agent) Chat(ctx context.Context, userInput string) (string, error) {
	sess := a.sessions.New()

	// 构建消息列表
	messages := []provider.Message{
		{Role: "system", Content: a.soul.SystemPrompt()},
	}

	// 加入分层记忆上下文
	messages = a.buildMemoryContext(messages)

	// 加入 RAG 检索上下文
	messages = a.buildRAGContext(ctx, messages, userInput)

	// 加入用户消息
	sess.AddMessage("user", userInput)
	messages = append(messages, provider.Message{Role: "user", Content: userInput})

	// 上下文窗口管理：裁剪消息到窗口内
	contextMessages := a.toContextMessages(messages)
	fitted, trimResult := a.contextWin.Fit(contextMessages)
	if trimResult.Trimmed {
		messages = a.fromContextMessages(fitted)
	}

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

	// 自动记忆：将对话存为短期记忆
	a.chatCount++
	a.memory.SaveShortTerm("User: "+userInput, "conversation")
	a.memory.SaveShortTerm("Assistant: "+truncate(response, 200), "conversation")

	// 每 10 轮对话触发衰减
	if a.chatCount%10 == 0 {
		a.memory.Decay(0.05)
	}

	// 每 20 轮对话触发自动摘要
	if a.chatCount%20 == 0 {
		a.autoSummarize()
	}

	return response, nil
}

// ChatStream 执行流式对话
func (a *Agent) ChatStream(ctx context.Context, userInput string) (<-chan provider.StreamChunk, error) {
	sess := a.sessions.New()

	messages := []provider.Message{
		{Role: "system", Content: a.soul.SystemPrompt()},
	}

	messages = a.buildMemoryContext(messages)

	sess.AddMessage("user", userInput)
	messages = append(messages, provider.Message{Role: "user", Content: userInput})

	// 上下文窗口管理：裁剪消息到窗口内
	contextMessages := a.toContextMessages(messages)
	fitted, trimResult := a.contextWin.Fit(contextMessages)
	if trimResult.Trimmed {
		messages = a.fromContextMessages(fitted)
	}

	return a.provider.ChatStream(ctx, messages)
}

// buildMemoryContext 构建分层记忆上下文
func (a *Agent) buildMemoryContext(messages []provider.Message) []provider.Message {
	var memCtx strings.Builder

	// 长期记忆：全部注入（核心身份/偏好）
	longs := a.memory.ByTier(memory.TierLong)
	if len(longs) > 0 {
		memCtx.WriteString("[Core Memory — Long-term]\n")
		for _, e := range longs {
			memCtx.WriteString("- " + e.Content + "\n")
		}
		memCtx.WriteString("\n")
	}

	// 中期记忆：按权重取 top 10
	mediums := a.memory.ByTier(memory.TierMedium)
	if len(mediums) > 0 {
		memCtx.WriteString("[Working Memory — Medium-term]\n")
		limit := 10
		if len(mediums) < limit {
			limit = len(mediums)
		}
		for i := 0; i < limit; i++ {
			memCtx.WriteString("- " + mediums[i].Content + "\n")
		}
		memCtx.WriteString("\n")
	}

	// 短期记忆：最近 5 条
	shorts := a.memory.ByTier(memory.TierShort)
	if len(shorts) > 0 {
		memCtx.WriteString("[Recent Context — Short-term]\n")
		limit := 5
		if len(shorts) < limit {
			limit = len(shorts)
		}
		for i := 0; i < limit; i++ {
			memCtx.WriteString("- " + shorts[i].Content + "\n")
		}
	}

	if memCtx.Len() > 0 {
		messages = append(messages, provider.Message{Role: "system", Content: memCtx.String()})
	}

	return messages
}

// autoSummarize 自动摘要：将过多的短期记忆压缩为中期
func (a *Agent) autoSummarize() {
	shorts := a.memory.ByTier(memory.TierShort)
	if len(shorts) <= 5 {
		return // 短期记忆不多，不需要摘要
	}

	// 收集最早的短期记忆（保留最近 5 条）
	var toSummarize []string
	var ids []string
	for i := 0; i < len(shorts)-5; i++ {
		ids = append(ids, shorts[i].ID)
		toSummarize = append(toSummarize, shorts[i].Content)
	}

	if len(ids) == 0 {
		return
	}

	// 简单拼接摘要（v0.4.0: 后续可接入 LLM 生成更智能摘要）
	summary := strings.Join(toSummarize, " | ")
	if len(summary) > 500 {
		summary = summary[:500] + "..."
	}

	a.memory.Summarize(ids, summary, "conversation")
}

// Remember 保存一条中期记忆
func (a *Agent) Remember(content, category string) error {
	return a.memory.Save(content, category)
}

// RememberLongTerm 保存一条长期记忆
func (a *Agent) RememberLongTerm(content, category string) error {
	return a.memory.SaveLongTerm(content, category)
}

// Recall 搜索记忆
func (a *Agent) Recall(query string) []memory.Entry {
	return a.memory.Search(query)
}

// MemoryStats 返回记忆统计
func (a *Agent) MemoryStats() map[memory.Tier]int {
	return a.memory.Stats()
}

// DecayMemory 执行记忆衰减
func (a *Agent) DecayMemory(threshold float64) int {
	return a.memory.Decay(threshold)
}

// PromoteMemory 提升记忆层级
func (a *Agent) PromoteMemory(id string) error {
	return a.memory.Promote(id)
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

// MCPClient 返回 MCP 客户端
func (a *Agent) MCPClient() *tool.MCPClient {
	return a.mcpClient
}

// Delegate 返回子代理委派管理器
func (a *Agent) Delegate() *tool.DelegateManager {
	return a.delegate
}

// Gateway 返回统一工具网关
func (a *Agent) Gateway() *tool.Gateway {
	return a.gateway
}

// LoadSkills 从目录加载 Skill 插件
func (a *Agent) LoadSkills(skillsDir string) (int, error) {
	loader := tool.NewSkillLoader(skillsDir)
	skills, err := loader.LoadAll()
	if err != nil {
		return 0, fmt.Errorf("load skills: %w", err)
	}

	tool.RegisterSkillTools(a.tools, skills, nil)
	return len(skills), nil
}

// ConnectMCPServer 连接 MCP Server
func (a *Agent) ConnectMCPServer(name, url, apiKey string) {
	a.mcpClient.AddServer(tool.MCPServerConfig{
		Name:   name,
		URL:    url,
		APIKey: apiKey,
	})

	// 注册 MCP 工具
	tool.RegisterMCPTools(a.tools, a.mcpClient)
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Sessions 返回会话管理器
func (a *Agent) Sessions() *session.Manager {
	return a.sessions
}

// Config 返回配置管理器
func (a *Agent) Config() *config.Manager {
	return a.cfg
}

// ContextWindow 返回上下文窗口管理器
func (a *Agent) ContextWindow() *contextx.ContextWindow {
	return a.contextWin
}

// FitContext 裁剪消息列表到上下文窗口内
func (a *Agent) FitContext(messages []contextx.Message) ([]contextx.Message, contextx.TrimResult) {
	return a.contextWin.Fit(messages)
}

// ContextStats 返回上下文窗口统计
func (a *Agent) ContextStats(messages []contextx.Message) contextx.ContextStats {
	return a.contextWin.Stats(messages)
}

// RAG 返回 RAG 管理器
func (a *Agent) RAG() *rag.RAGManager {
	return a.ragManager
}

// buildRAGContext 构建 RAG 检索上下文
func (a *Agent) buildRAGContext(ctx context.Context, messages []provider.Message, query string) []provider.Message {
	if a.ragManager == nil {
		return messages
	}

	stats := a.ragManager.Stats()
	if stats.DocumentCount == 0 {
		return messages // 没有索引文档，跳过 RAG
	}

	ragCtx, _, err := a.ragManager.SearchWithContext(ctx, query)
	if err != nil || ragCtx == "" {
		return messages
	}

	return append(messages, provider.Message{Role: "system", Content: ragCtx})
}

// unused suppress
var _ = time.Second

// toContextMessages 将 provider.Message 转换为 contextx.Message
func (a *Agent) toContextMessages(messages []provider.Message) []contextx.Message {
	result := make([]contextx.Message, len(messages))
	for i, msg := range messages {
		priority := contextx.PriorityNormal
		category := msg.Role

		// system 消息是 critical
		if msg.Role == "system" {
			priority = contextx.PriorityCritical
			category = "system"
		}

		// 记忆上下文按层级分配优先级
		if msg.Role == "system" && len(msg.Content) > 0 {
			switch {
			case strings.HasPrefix(msg.Content, "[Core Memory"):
				priority = contextx.PriorityHigh
				category = "memory_long"
			case strings.HasPrefix(msg.Content, "[Working Memory"):
				priority = contextx.PriorityNormal
				category = "memory_medium"
			case strings.HasPrefix(msg.Content, "[Recent Context"):
				priority = contextx.PriorityLow
				category = "memory_short"
			}
		}

		result[i] = contextx.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Priority: priority,
			Category:  category,
			Timestamp: time.Now(),
		}
	}
	return result
}

// fromContextMessages 将 contextx.Message 转换回 provider.Message
func (a *Agent) fromContextMessages(messages []contextx.Message) []provider.Message {
	result := make([]provider.Message, len(messages))
	for i, msg := range messages {
		result[i] = provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return result
}
