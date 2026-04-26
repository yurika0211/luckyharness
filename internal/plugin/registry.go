package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry 插件注册中心
type Registry struct {
	mu         sync.RWMutex
	plugins    map[string]*PluginEntry // name -> entry
	pluginsDir string                  // 插件安装目录
	index      *RemoteIndex            // 远程仓库索引（可选）
}

// PluginEntry 已安装的插件条目
type PluginEntry struct {
	Manifest  *Manifest
	Status    PluginStatus
	Config    map[string]any // 插件配置
	Error     string         // 错误信息（如果状态为 error）
	UpdatedAt time.Time
}

// NewRegistry 创建插件注册中心
func NewRegistry(pluginsDir string) *Registry {
	return &Registry{
		plugins:    make(map[string]*PluginEntry),
		pluginsDir: pluginsDir,
	}
}

// Register 注册插件
func (r *Registry) Register(manifest *Manifest) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins[manifest.Name] = &PluginEntry{
		Manifest:  manifest,
		Status:    StatusInstalled,
		Config:    make(map[string]any),
		UpdatedAt: time.Now(),
	}
	return nil
}

// Unregister 注销插件
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.plugins[name]; !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	delete(r.plugins, name)
	return nil
}

// Get 获取插件
func (r *Registry) Get(name string) (*PluginEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.plugins[name]
	if !ok {
		return nil, false
	}
	return entry, true
}

// List 列出所有插件
func (r *Registry) List() []*PluginEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*PluginEntry, 0, len(r.plugins))
	for _, e := range r.plugins {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Manifest.Name < entries[j].Manifest.Name
	})
	return entries
}

// ListByType 按类型列出插件
func (r *Registry) ListByType(pluginType string) []*PluginEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*PluginEntry
	for _, e := range r.plugins {
		if e.Manifest.Type == pluginType {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Manifest.Name < entries[j].Manifest.Name
	})
	return entries
}

// ListByStatus 按状态列出插件
func (r *Registry) ListByStatus(status PluginStatus) []*PluginEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*PluginEntry
	for _, e := range r.plugins {
		if e.Status == status {
			entries = append(entries, e)
		}
	}
	return entries
}

// UpdateStatus 更新插件状态
func (r *Registry) UpdateStatus(name string, status PluginStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	entry.Status = status
	entry.UpdatedAt = time.Now()
	return nil
}

// SetConfig 设置插件配置
func (r *Registry) SetConfig(name string, key string, value any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	if entry.Config == nil {
		entry.Config = make(map[string]any)
	}
	entry.Config[key] = value
	entry.UpdatedAt = time.Now()
	return nil
}

// GetConfig 获取插件配置
func (r *Registry) GetConfig(name string, key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.plugins[name]
	if !ok {
		return nil, false
	}
	if entry.Config == nil {
		return nil, false
	}
	val, exists := entry.Config[key]
	return val, exists
}

// SetError 设置插件错误状态
func (r *Registry) SetError(name string, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.plugins[name]
	if !ok {
		return
	}
	entry.Status = StatusError
	entry.Error = errMsg
	entry.UpdatedAt = time.Now()
}

// Count 返回插件数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// CountByType 按类型统计插件数量
func (r *Registry) CountByType(pluginType string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, e := range r.plugins {
		if e.Manifest.Type == pluginType {
			count++
		}
	}
	return count
}

// CheckDependencies 检查插件依赖是否满足
func (r *Registry) CheckDependencies(manifest *Manifest) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var missing []string
	for _, dep := range manifest.Dependencies {
		if _, ok := r.plugins[dep.Name]; !ok {
			missing = append(missing, dep.Name)
		}
	}
	return missing
}

// LoadFromDisk 从磁盘加载已安装的插件
func (r *Registry) LoadFromDisk() error {
	if r.pluginsDir == "" {
		return nil
	}

	entries, err := os.ReadDir(r.pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在，没有插件
		}
		return fmt.Errorf("read plugins dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(r.pluginsDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.yaml")

		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			// 记录错误但继续加载其他插件
			r.mu.Lock()
			r.plugins[entry.Name()] = &PluginEntry{
				Manifest: &Manifest{
					Name: entry.Name(),
					Type: "unknown",
				},
				Status: StatusError,
				Error:  fmt.Sprintf("load manifest: %v", err),
			}
			r.mu.Unlock()
			continue
		}

		manifest.InstallPath = pluginDir
		r.Register(manifest)
	}

	return nil
}

// RemoteIndex 远程仓库索引
type RemoteIndex struct {
	mu      sync.RWMutex
	entries map[string]*RemoteEntry // name -> entry
	url     string                  // 仓库 URL
}

// RemoteEntry 远程仓库中的插件条目
type RemoteEntry struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
	DownloadURL string   `json:"download_url"`
	Checksum    string   `json:"checksum"`
}

// NewRemoteIndex 创建远程索引
func NewRemoteIndex(url string) *RemoteIndex {
	return &RemoteIndex{
		entries: make(map[string]*RemoteEntry),
		url:     url,
	}
}

// Search 搜索远程仓库
func (ri *RemoteIndex) Search(query string) []*RemoteEntry {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	var results []*RemoteEntry
	lowerQuery := fmt.Sprintf("%s", query)
	for _, e := range ri.entries {
		if matchRemoteEntry(e, lowerQuery) {
			results = append(results, e)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results
}

// Get 获取远程插件信息
func (ri *RemoteIndex) Get(name string) (*RemoteEntry, bool) {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	e, ok := ri.entries[name]
	return e, ok
}

// Add 添加远程插件条目
func (ri *RemoteIndex) Add(entry *RemoteEntry) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.entries[entry.Name] = entry
}

// Count 返回远程插件数量
func (ri *RemoteIndex) Count() int {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return len(ri.entries)
}

// matchRemoteEntry 检查远程条目是否匹配查询
func matchRemoteEntry(e *RemoteEntry, query string) bool {
	lower := stringsToLower(e.Name + " " + e.Description + " " + e.Author + " " + strings.Join(e.Tags, " "))
	return stringsContains(lower, query)
}

// helper to avoid importing strings package for simple ops
func stringsToLower(s string) string {
	result := make([]byte, len(s))
	for i, ch := range s {
		if ch >= 'A' && ch <= 'Z' {
			result[i] = byte(ch + 32)
		} else {
			result[i] = byte(ch)
		}
	}
	return string(result)
}

func stringsContains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
