package config

import (
	"fmt"
	"os"
	"path/filepath"
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

	lhHome := filepath.Join(home, ".luckyharness")
	cfgPath := filepath.Join(lhHome, "config.yaml")

	m := &Manager{
		config:  DefaultConfig(),
		homeDir: lhHome,
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
