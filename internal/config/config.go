package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config 代表 LuckyHarness 的运行时配置
type Config struct {
	Provider    string            `yaml:"provider"`
	APIKey      string            `yaml:"api_key"`
	APIBase     string            `yaml:"api_base,omitempty"`
	Model       string            `yaml:"model"`
	SoulPath    string            `yaml:"soul_path,omitempty"`
	MaxTokens   int               `yaml:"max_tokens"`
	Temperature float64           `yaml:"temperature"`
	Extra       map[string]string `yaml:"extra,omitempty"`

	// v0.3.0: 降级链配置
	Fallbacks []FallbackEntry `yaml:"fallbacks,omitempty"`

	// v0.37.0: Web 搜索配置
	WebSearch WebSearchConfig `yaml:"web_search,omitempty"`

	// v0.40.0: 流式输出模式 (native=真流式, simulated=非流式获取+模拟推送)
	StreamMode string `yaml:"stream_mode,omitempty"`

	// v0.43.0: 记忆系统配置
	Memory MemoryConfig `yaml:"memory,omitempty"`

	// v0.45.0: 模型路由配置
	ModelRouter ModelRouterConfig `yaml:"model_router,omitempty"`
}

// MemoryConfig 记忆系统配置
type MemoryConfig struct {
	ShortTermMaxTurns  int `yaml:"short_term_max_turns,omitempty"`  // 短期记忆最大轮数（默认 10）
	MidTermExpireDays  int `yaml:"midterm_expire_days,omitempty"`   // 中期记忆过期天数（默认 90）
	MidTermMaxSummaries int `yaml:"midterm_max_summaries,omitempty"` // 中期记忆最大摘要数（默认 100）
}

// ModelRouterConfig 模型路由配置
type ModelRouterConfig struct {
	Enable       bool   `yaml:"enable,omitempty"`        // 是否启用模型路由
	SimpleModel  string `yaml:"simple_model,omitempty"`  // 简单任务模型（便宜/快速）
	ComplexModel string `yaml:"complex_model,omitempty"` // 复杂任务模型（强/慢）
	LocalModel   string `yaml:"local_model,omitempty"`   // 本地模型（ollama）
	LocalBaseURL string `yaml:"local_base_url,omitempty"` // 本地模型 API 地址
	
	// 自动路由阈值
	TokenThreshold int `yaml:"token_threshold,omitempty"` // 超过此 token 数视为复杂任务（默认 500）
}

// TaskComplexity 任务复杂度
type TaskComplexity int

const (
	TaskSimple  TaskComplexity = iota // 简单任务：问候、简单问答
	TaskModerate                      // 中等任务：一般查询、简单分析
	TaskComplex                       // 复杂任务：代码生成、复杂分析、多步骤推理
)

// ModelRouter 模型路由器
type ModelRouter struct {
	config ModelRouterConfig
}

// NewModelRouter 创建模型路由器
func NewModelRouter(config ModelRouterConfig) *ModelRouter {
	return &ModelRouter{config: config}
}

// SelectModel 根据任务复杂度选择模型
func (r *ModelRouter) SelectModel(complexity TaskComplexity) (model string, apiBase string) {
	if !r.config.Enable {
		return "", "" // 未启用路由，使用默认配置
	}

	switch complexity {
	case TaskSimple:
		// 简单任务使用便宜模型
		if r.config.SimpleModel != "" {
			return r.config.SimpleModel, ""
		}
	case TaskComplex:
		// 复杂任务使用强模型
		if r.config.ComplexModel != "" {
			return r.config.ComplexModel, ""
		}
	default:
		// 中等任务：如果有本地模型优先使用本地
		if r.config.LocalModel != "" {
			return r.config.LocalModel, r.config.LocalBaseURL
		}
	}
	
	return "", ""
}

// EstimateComplexity 根据输入估算任务复杂度
func (r *ModelRouter) EstimateComplexity(input string, tokenCount int) TaskComplexity {
	inputLower := strings.ToLower(input)
	
	// 简单任务关键词
	simpleKeywords := []string{
		"hello", "hi", "hey", "good morning", "good night",
		"谢谢", "你好", "再见", "早上好", "晚安",
		"what time", "current time", "date",
	}
	
	for _, kw := range simpleKeywords {
		if strings.Contains(inputLower, kw) {
			return TaskSimple
		}
	}
	
	// 复杂任务关键词
	complexKeywords := []string{
		"write code", "implement", "create a program", "build",
		"analyze", "compare", "explain in detail", "step by step",
		"optimize", "refactor", "debug", "design",
		"编写代码", "实现", "创建程序", "构建",
		"分析", "比较", "详细解释", "逐步",
		"优化", "重构", "调试", "设计",
	}
	
	for _, kw := range complexKeywords {
		if strings.Contains(inputLower, kw) {
			return TaskComplex
		}
	}
	
	// 根据 token 数判断
	if tokenCount > r.config.TokenThreshold {
		if r.config.TokenThreshold <= 0 {
			r.config.TokenThreshold = 500
		}
		return TaskComplex
	}
	
	// 默认为中等任务
	return TaskModerate
}

// IsLocalTask 判断是否为本地任务（涉及本地文件/命令）
func (r *ModelRouter) IsLocalTask(input string) bool {
	localKeywords := []string{
		"file", "directory", "folder", "path",
		"run", "execute", "command", "terminal", "shell",
		"local", "localhost",
		"文件", "目录", "文件夹", "路径",
		"运行", "执行", "命令", "终端",
	}
	
	inputLower := strings.ToLower(input)
	for _, kw := range localKeywords {
		if strings.Contains(inputLower, kw) {
			return true
		}
	}
	
	return false
}

// SelectModelForTask 根据任务描述自动选择模型
func (r *ModelRouter) SelectModelForTask(taskDescription string, tokenCount int) (model string, apiBase string) {
	if !r.config.Enable {
		return "", ""
	}
	
	// 如果是本地任务，优先使用本地模型
	if r.IsLocalTask(taskDescription) && r.config.LocalModel != "" {
		return r.config.LocalModel, r.config.LocalBaseURL
	}
	
	// 估算复杂度
	complexity := r.EstimateComplexity(taskDescription, tokenCount)
	return r.SelectModel(complexity)
}

// WebSearchConfig 网络搜索配置（照 nanobot WebSearchConfig 设计）
type WebSearchConfig struct {
	Provider    string `yaml:"provider,omitempty"`    // brave, ddgs, searxng（默认 brave）
	APIKey      string `yaml:"api_key,omitempty"`     // Brave / Tavily / Jina API key
	BaseURL     string `yaml:"base_url,omitempty"`    // SearXNG 自部署地址
	MaxResults  int    `yaml:"max_results,omitempty"` // 最大结果数（默认 5）
	Proxy       string `yaml:"proxy,omitempty"`       // HTTP/SOCKS5 代理
}

// FallbackEntry 是降级链中的一个节点配置
type FallbackEntry struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key,omitempty"`
	APIBase  string `yaml:"api_base,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Provider:    "openai",
		Model:       "gpt-4o",
		SoulPath:    filepath.Join(home, ".luckyharness", "SOUL.md"),
		MaxTokens:   4096,
		Temperature: 0.7,
		Extra:       make(map[string]string),
		Memory: MemoryConfig{
			ShortTermMaxTurns:   10,
			MidTermExpireDays:   90,
			MidTermMaxSummaries: 100,
		},
	}
}

// Manager 管理配置的加载和保存
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	homeDir  string
	cfgPath  string
}

// NewManager 创建配置管理器
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	return NewManagerWithDir(filepath.Join(home, ".luckyharness"))
}

// NewManagerWithDir 创建指定目录的配置管理器（用于测试隔离）
func NewManagerWithDir(homeDir string) (*Manager, error) {
	cfgPath := filepath.Join(homeDir, "config.yaml")

	m := &Manager{
		config:  DefaultConfig(),
		homeDir: homeDir,
		cfgPath: cfgPath,
	}

	return m, nil
}

// Load 从磁盘加载配置
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 使用默认配置
		}
		return fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	m.config = &cfg
	return nil
}

// Save 保存配置到磁盘
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := os.MkdirAll(m.homeDir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(m.cfgPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// Get 获取当前配置的只读副本
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := *m.config
	return &cp
}

// Set 修改配置项
func (m *Manager) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch key {
	case "provider":
		m.config.Provider = value
	case "api_key":
		m.config.APIKey = value
	case "api_base":
		m.config.APIBase = value
	case "model":
		m.config.Model = value
	case "soul_path":
		m.config.SoulPath = value
	case "max_tokens":
		var n int
		fmt.Sscanf(value, "%d", &n)
		m.config.MaxTokens = n
	case "temperature":
		var f float64
		fmt.Sscanf(value, "%f", &f)
		m.config.Temperature = f
	// v0.37.0: web_search 子配置
	case "web_search.provider":
		m.config.WebSearch.Provider = value
	case "web_search.api_key":
		m.config.WebSearch.APIKey = value
	case "web_search.base_url":
		m.config.WebSearch.BaseURL = value
	case "web_search.max_results":
		var n int
		fmt.Sscanf(value, "%d", &n)
		m.config.WebSearch.MaxResults = n
	case "web_search.proxy":
		m.config.WebSearch.Proxy = value
	case "stream_mode":
		m.config.StreamMode = value
	default:
		if m.config.Extra == nil {
			m.config.Extra = make(map[string]string)
		}
		m.config.Extra[key] = value
	}
	return nil
}

// HomeDir 返回 LuckyHarness 主目录
func (m *Manager) HomeDir() string {
	return m.homeDir
}

// InitHome 初始化主目录结构
func (m *Manager) InitHome() error {
	dirs := []string{
		m.homeDir,
		filepath.Join(m.homeDir, "sessions"),
		filepath.Join(m.homeDir, "memory"),
		filepath.Join(m.homeDir, "logs"),
		filepath.Join(m.homeDir, "skills"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	// 写入默认 SOUL.md
	soulPath := filepath.Join(m.homeDir, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		defaultSoul := DefaultSoul()
		if err := os.WriteFile(soulPath, []byte(defaultSoul), 0644); err != nil {
			return fmt.Errorf("write SOUL.md: %w", err)
		}
	}

	return nil
}
