package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yurika0211/luckyharness/internal/collab"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/contextx"
	"github.com/yurika0211/luckyharness/internal/embedder"
	"github.com/yurika0211/luckyharness/internal/gateway"
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
	tmplMgr      *soul.TemplateManager // v0.19.0: SOUL 模板管理器
	provider     provider.Provider       // 当前活跃 provider (可能是 FallbackChain)
	registry     *provider.Registry     // provider 注册表
	catalog      *provider.ModelCatalog  // 模型目录
	tokenStore   *provider.TokenStore    // token 存储
	memory       *memory.Store
	sessions     *session.Manager
	tools        *tool.Registry
	gateway      *tool.Gateway          // 统一工具网关
	msgGateway   *gateway.GatewayManager // v0.6.0: 消息平台网关
	mcpClient    *tool.MCPClient         // MCP 客户端
	delegate     *tool.DelegateManager   // 子代理委派管理器
	contextWin   *contextx.ContextWindow // 上下文窗口管理器
	ragManager   *rag.RAGManager         // RAG 知识库管理器
	ragPersist   *rag.Persistence        // RAG 持久化
	streamIndexer *rag.StreamIndexer     // v0.23.0: 流式索引器
	embedderReg  *embedder.Registry      // v0.21.0: 嵌入模型注册表
	collabReg    *collab.Registry        // v0.22.0: Agent 协作注册表
	collabMgr    *collab.DelegateManager // v0.22.0: 协作任务管理器
	skills       []*tool.SkillInfo       // v0.35.0: 已加载的 skill 列表
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

	// v0.19.0: 创建 SOUL 模板管理器
	tmplMgr := soul.NewTemplateManager()

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
	toolGateway := tool.NewGateway(tools)

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
	// v0.21.0: 使用 Embedder Registry 管理嵌入模型
	embedderReg := embedder.NewRegistry()
	mockEmb := embedder.NewMockEmbedder(128)
	embedderReg.Register("mock-128", mockEmb)

	// 注册 OpenAI embedder (如果配置了 API key)
	if c.APIKey != "" {
		openaiEmb := embedder.NewOpenAIEmbedder(embedder.OpenAIEmbedderConfig{
			APIKey:  c.APIKey,
			BaseURL: c.APIBase,
		})
		embedderReg.Register("openai-default", openaiEmb)
	}

	// 使用 active embedder (带缓存)
	activeEmb := embedder.NewCachedEmbedder(embedderReg.Active(), 512)

	ragConfig := rag.DefaultRAGConfig()

	var ragManager *rag.RAGManager
	var ragPersist *rag.Persistence

	// 尝试使用 SQLite 后端
	ragDBPath := cfg.HomeDir() + "/rag/luckyharness.db"
	ragMgr, err := rag.NewRAGManagerWithSQLite(activeEmb, ragConfig, ragDBPath)
	if err != nil {
		// SQLite 不可用时降级到内存 + JSON 持久化
		ragManager = rag.NewRAGManager(activeEmb, ragConfig)
		ragPersist = rag.NewPersistence(cfg.HomeDir() + "/rag")
		if ragPersist.Exists() {
			if docCount, loadErr := ragPersist.Load(ragManager); loadErr == nil && docCount > 0 {
				// loaded successfully
			}
		}
	} else {
		ragManager = ragMgr
		// SQLite 后端自动持久化，不需要 JSON 持久化
		// 但保留 ragPersist 用于迁移旧数据
		ragPersist = rag.NewPersistence(cfg.HomeDir() + "/rag")
		if ragPersist.Exists() {
			// 迁移旧 JSON 数据到 SQLite
			tempMgr := rag.NewRAGManager(activeEmb, ragConfig)
			if docCount, loadErr := ragPersist.Load(tempMgr); loadErr == nil && docCount > 0 {
				for _, docID := range tempMgr.ListDocuments() {
					if doc, ok := tempMgr.GetDocument(docID); ok {
						ragManager.IndexText(doc.Path, doc.Title, "")
					}
				}
			}
		}
	}

	// v0.22.0: 创建 Agent 协作注册表和管理器
	collabReg := collab.NewRegistry()
	// 注册本地 Agent
	collabReg.Register(&collab.AgentProfile{
		ID:           "local-agent",
		Name:         "Local Agent",
		Description:  "The primary local agent",
		Capabilities: []string{"chat", "code", "analysis", "research"},
		Status:       collab.StatusOnline,
	})
	// 创建协作任务管理器（使用默认 handler，实际执行由 Agent Loop 驱动）
	collabMgr := collab.NewDelegateManager(collabReg, nil)

	// v0.23.0: 创建流式索引器
	streamIndexer := rag.NewStreamIndexer(ragManager, rag.DefaultStreamConfig())

	a := &Agent{
		cfg:        cfg,
		soul:       s,
		tmplMgr:    tmplMgr,
		provider:   p,
		registry:   registry,
		catalog:    catalog,
		tokenStore: tokenStore,
		memory:     mem,
		sessions:   sessions,
		tools:      tools,
		gateway:    toolGateway,
		msgGateway: gateway.NewGatewayManager(),
		mcpClient:  mcpClient,
		delegate:   delegateMgr,
		contextWin: contextWin,
		ragManager:  ragManager,
		ragPersist:  ragPersist,
		streamIndexer: streamIndexer,
		embedderReg: embedderReg,
		collabReg:   collabReg,
		collabMgr:   collabMgr,
	}

	// v0.35.0: 自动加载 skills 目录
	skillsDir := cfg.HomeDir() + "/skills"
	if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
		if count, err := a.LoadSkills(skillsDir); err == nil && count > 0 {
			fmt.Printf("[agent] loaded %d skills from %s\n", count, skillsDir)
		}
	}

	return a, nil
}

// Chat 执行一次对话
func (a *Agent) Chat(ctx context.Context, userInput string) (string, error) {
	sess := a.sessions.New()
	return a.chatWithSession(ctx, sess, userInput)
}

// ChatWithSession 在已有会话中继续对话，实现多轮上下文。
func (a *Agent) ChatWithSession(ctx context.Context, sessionID string, userInput string) (string, error) {
	sess, ok := a.sessions.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	return a.chatWithSession(ctx, sess, userInput)
}

// chatWithSession 是 Chat/ChatWithSession 的共享实现。
func (a *Agent) chatWithSession(ctx context.Context, sess *session.Session, userInput string) (string, error) {

	// 优先使用 RunLoop（支持 function calling / 工具调用）
	loopCfg := DefaultLoopConfig()
	loopCfg.AutoApprove = true // Telegram 场景自动批准工具调用

	result, err := a.RunLoopWithSession(ctx, sess, userInput, loopCfg)
	if err != nil {
		// 如果 RunLoop 失败，回退到简单流式聊天
		response, chatErr := a.chatStreamSimple(ctx, sess, userInput)
		if chatErr != nil {
			return "", fmt.Errorf("runloop: %w; fallback chat: %w", err, chatErr)
		}
		return response, nil
	}

	response := result.Response

	// 保存到会话
	sess.AddMessage("user", userInput)
	sess.AddMessage("assistant", response)
	_ = sess.Save()

	// 自动记忆
	a.chatCount++
	a.memory.SaveShortTerm("User: "+userInput, "conversation")
	a.memory.SaveShortTerm("Assistant: "+truncate(response, 200), "conversation")

	if a.chatCount%10 == 0 {
		a.memory.Decay(0.05)
	}
	if a.chatCount%20 == 0 {
		a.autoSummarize()
	}

	return response, nil
}

// chatStreamSimple 是不使用工具的简单流式聊天（作为 RunLoop 的回退）。
func (a *Agent) chatStreamSimple(ctx context.Context, sess *session.Session, userInput string) (string, error) {

	// 构建消息列表：system + 记忆 + RAG + 会话历史 + 新消息
	messages := []provider.Message{
		{Role: "system", Content: a.soul.SystemPrompt()},
	}

	// 加入分层记忆上下文
	messages = a.buildMemoryContext(messages)

	// 加入 RAG 检索上下文
	messages = a.buildRAGContext(ctx, messages, userInput)

	// 加入已有会话历史（多轮对话上下文）
	existingMsgs := sess.GetMessages()
	if len(existingMsgs) > 0 {
		messages = append(messages, existingMsgs...)
	}

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

// TemplateManager 返回 SOUL 模板管理器
func (a *Agent) TemplateManager() *soul.TemplateManager {
	return a.tmplMgr
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

// MsgGateway 返回消息平台网关管理器 (v0.6.0)
func (a *Agent) MsgGateway() *gateway.GatewayManager {
	return a.msgGateway
}

// LoadSkills 从目录加载 Skill 插件
func (a *Agent) LoadSkills(skillsDir string) (int, error) {
	loader := tool.NewSkillLoader(skillsDir)
	skills, err := loader.LoadAll()
	if err != nil {
		return 0, fmt.Errorf("load skills: %w", err)
	}

	a.skills = skills
	tool.RegisterSkillTools(a.tools, skills, nil)

	// v0.35.0: 注册 skill_read 工具，让 LLM 能读取 SKILL.md 内容
	a.tools.Register(&tool.Tool{
		Name:        "skill_read",
		Description: "读取指定 skill 的 SKILL.md 内容，了解该 skill 的完整使用方法和步骤。当用户请求涉及某个 skill 的能力时，先调用此工具读取 SKILL.md，再按指引操作。",
		Category:    tool.CatSkill,
		Permission:  tool.PermAuto,
		Enabled:     true,
		Parameters: map[string]tool.Param{
			"name": {
				Type:        "string",
				Description: "Skill 名称（如 web-search, summarize, rewrite 等）",
				Required:    true,
			},
		},
		Handler: a.handleSkillRead(),
	})

	return len(skills), nil
}

// Skills 返回已加载的 skill 列表
func (a *Agent) Skills() []*tool.SkillInfo {
	return a.skills
}

// handleSkillRead 返回 skill_read 工具的 handler
func (a *Agent) handleSkillRead() func(args map[string]any) (string, error) {
	return func(args map[string]any) (string, error) {
		name, _ := args["name"].(string)
		if name == "" {
			// 没指定名称，返回所有 skill 列表
			var b strings.Builder
			b.WriteString("Available skills:\n")
			for _, s := range a.skills {
				b.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
			}
			return b.String(), nil
		}

		// 查找匹配的 skill
		for _, s := range a.skills {
			if s.Name == name || strings.EqualFold(s.Name, name) {
				skillFile := filepath.Join(s.Dir, "SKILL.md")
				data, err := os.ReadFile(skillFile)
				if err != nil {
					return "", fmt.Errorf("read SKILL.md for %s: %w", name, err)
				}
				return string(data), nil
			}
		}

		// 模糊匹配
		var candidates []string
		lowerName := strings.ToLower(name)
		for _, s := range a.skills {
			if strings.Contains(strings.ToLower(s.Name), lowerName) {
				candidates = append(candidates, s.Name)
			}
		}
		if len(candidates) > 0 {
			return fmt.Sprintf("Skill '%s' not found. Did you mean: %s?", name, strings.Join(candidates, ", ")), nil
		}

		return fmt.Sprintf("Skill '%s' not found. Use skill_read without name to list all skills.", name), nil
	}
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

// RAGPersist 返回 RAG 持久化管理器
func (a *Agent) RAGPersist() *rag.Persistence {
	return a.ragPersist
}

// StreamIndexer 返回流式索引器 (v0.23.0)
func (a *Agent) StreamIndexer() *rag.StreamIndexer {
	return a.streamIndexer
}

// EmbedderRegistry 返回嵌入模型注册表
func (a *Agent) EmbedderRegistry() *embedder.Registry {
	return a.embedderReg
}

// AgentRegistry 返回 Agent 协作注册表 (v0.22.0)
func (a *Agent) AgentRegistry() *collab.Registry {
	return a.collabReg
}

// CollabManager 返回协作任务管理器 (v0.22.0)
func (a *Agent) CollabManager() *collab.DelegateManager {
	return a.collabMgr
}

// Close 释放资源，保存持久化数据
func (a *Agent) Close() error {
	var firstErr error

	// SQLite 后端自动持久化，只需关闭连接
	if s := a.ragManager.SQLiteStore(); s != nil {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close sqlite store: %w", err)
		}
	} else if a.ragPersist != nil && a.ragManager != nil {
		// 内存后端：关闭时保存到 JSON
		if err := a.ragPersist.Save(a.ragManager); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("save RAG index: %w", err)
		}
	}

	return firstErr
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
