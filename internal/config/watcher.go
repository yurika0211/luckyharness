package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigWatcher 监控配置文件变化并自动重载
type ConfigWatcher struct {
	mu       sync.RWMutex
	config   *Config
	cfgPath  string
	homeDir  string
	interval time.Duration
	stopCh   chan struct{}
	onChange  func(oldCfg, newCfg *Config)
	onError   func(err error)
	running  bool
	lastMod  time.Time
}

// NewConfigWatcher 创建配置监控器
func NewConfigWatcher(mgr *Manager, interval time.Duration) *ConfigWatcher {
	return &ConfigWatcher{
		config:   mgr.Get(),
		cfgPath:  mgr.cfgPath,
		homeDir:  mgr.homeDir,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// OnChange 设置配置变化回调
func (w *ConfigWatcher) OnChange(fn func(oldCfg, newCfg *Config)) {
	w.onChange = fn
}

// OnError 设置错误回调
func (w *ConfigWatcher) OnError(fn func(err error)) {
	w.onError = fn
}

// Start 启动配置监控
func (w *ConfigWatcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	w.running = true
	w.mu.Unlock()

	// 记录初始修改时间
	if info, err := os.Stat(w.cfgPath); err == nil {
		w.lastMod = info.ModTime()
	}

	go w.watchLoop()
	return nil
}

// Stop 停止配置监控
func (w *ConfigWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		close(w.stopCh)
		w.running = false
		w.stopCh = make(chan struct{}) // 重新创建，允许再次启动
	}
}

// IsRunning 检查监控器是否在运行
func (w *ConfigWatcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// GetConfig 获取当前配置
func (w *ConfigWatcher) GetConfig() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cp := *w.config
	return &cp
}

// watchLoop 监控循环
func (w *ConfigWatcher) watchLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.checkAndReload()
		}
	}
}

// checkAndReload 检查文件变化并重载
func (w *ConfigWatcher) checkAndReload() {
	info, err := os.Stat(w.cfgPath)
	if err != nil {
		if !os.IsNotExist(err) {
			w.emitError(fmt.Errorf("stat config: %w", err))
		}
		return
	}

	// 检查修改时间
	if !info.ModTime().After(w.lastMod) {
		return // 没有变化
	}

	// 读取并解析新配置
	data, err := os.ReadFile(w.cfgPath)
	if err != nil {
		w.emitError(fmt.Errorf("read config: %w", err))
		return
	}

	var newCfg Config
	if err := yaml.Unmarshal(data, &newCfg); err != nil {
		w.emitError(fmt.Errorf("parse config: %w", err))
		return
	}

	// 获取旧配置
	w.mu.RLock()
	oldCfg := *w.config
	w.mu.RUnlock()

	// 更新配置
	w.mu.Lock()
	w.config = &newCfg
	w.lastMod = info.ModTime()
	w.mu.Unlock()

	// 触发回调
	if w.onChange != nil {
		w.onChange(&oldCfg, &newCfg)
	}
}

// emitError 触发错误回调
func (w *ConfigWatcher) emitError(err error) {
	if w.onError != nil {
		w.onError(err)
	}
}

// ForceReload 强制重载配置
func (w *ConfigWatcher) ForceReload() error {
	data, err := os.ReadFile(w.cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，使用默认配置
		}
		return fmt.Errorf("read config: %w", err)
	}

	var newCfg Config
	if err := yaml.Unmarshal(data, &newCfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	w.mu.Lock()
	oldCfg := *w.config
	w.config = &newCfg
	if info, err := os.Stat(w.cfgPath); err == nil {
		w.lastMod = info.ModTime()
	}
	w.mu.Unlock()

	if w.onChange != nil {
		w.onChange(&oldCfg, &newCfg)
	}

	return nil
}

// ConfigDiff 比较两个配置的差异
type ConfigDiff struct {
	ChangedFields []string
	OldValues     map[string]string
	NewValues     map[string]string
}

// DiffConfig 比较两个配置的差异
func DiffConfig(oldCfg, newCfg *Config) *ConfigDiff {
	diff := &ConfigDiff{
		ChangedFields: []string{},
		OldValues:     make(map[string]string),
		NewValues:     make(map[string]string),
	}

	if oldCfg.Provider != newCfg.Provider {
		diff.ChangedFields = append(diff.ChangedFields, "provider")
		diff.OldValues["provider"] = oldCfg.Provider
		diff.NewValues["provider"] = newCfg.Provider
	}
	if oldCfg.Model != newCfg.Model {
		diff.ChangedFields = append(diff.ChangedFields, "model")
		diff.OldValues["model"] = oldCfg.Model
		diff.NewValues["model"] = newCfg.Model
	}
	if oldCfg.APIBase != newCfg.APIBase {
		diff.ChangedFields = append(diff.ChangedFields, "api_base")
		diff.OldValues["api_base"] = oldCfg.APIBase
		diff.NewValues["api_base"] = newCfg.APIBase
	}
	if oldCfg.MaxTokens != newCfg.MaxTokens {
		diff.ChangedFields = append(diff.ChangedFields, "max_tokens")
		diff.OldValues["max_tokens"] = fmt.Sprintf("%d", oldCfg.MaxTokens)
		diff.NewValues["max_tokens"] = fmt.Sprintf("%d", newCfg.MaxTokens)
	}
	if oldCfg.Temperature != newCfg.Temperature {
		diff.ChangedFields = append(diff.ChangedFields, "temperature")
		diff.OldValues["temperature"] = fmt.Sprintf("%.2f", oldCfg.Temperature)
		diff.NewValues["temperature"] = fmt.Sprintf("%.2f", newCfg.Temperature)
	}
	if oldCfg.SoulPath != newCfg.SoulPath {
		diff.ChangedFields = append(diff.ChangedFields, "soul_path")
		diff.OldValues["soul_path"] = oldCfg.SoulPath
		diff.NewValues["soul_path"] = newCfg.SoulPath
	}

	return diff
}

// HasChanged 检查配置是否有变化
func (d *ConfigDiff) HasChanged() bool {
	return len(d.ChangedFields) > 0
}

// Format 格式化配置差异
func (d *ConfigDiff) Format() string {
	if !d.HasChanged() {
		return "No configuration changes"
	}

	result := "Configuration changes:\n"
	for _, field := range d.ChangedFields {
		result += fmt.Sprintf("  %s: %s → %s\n", field, d.OldValues[field], d.NewValues[field])
	}
	return result
}

// Ensure ConfigWatcher uses Manager's unexported fields
// We need to expose them or use a different approach.
// Let's add helper methods to Manager instead.

// WatchConfig 启动配置文件监控
func (m *Manager) WatchConfig(interval time.Duration) (*ConfigWatcher, error) {
	watcher := NewConfigWatcher(m, interval)
	return watcher, nil
}

// Reload 强制重载配置
func (m *Manager) Reload() error {
	return m.Load()
}

// ConfigFile 返回配置文件路径
func (m *Manager) ConfigFile() string {
	return m.cfgPath
}

// HomeDir 返回主目录
func (m *Manager) HomeDirPath() string {
	return m.homeDir
}

// Ensure filepath import is used
var _ = filepath.Base