package plugin

import (
	"fmt"
	"strings"
	"time"
)

// PluginStatus 插件状态
type PluginStatus string

const (
	StatusInstalled  PluginStatus = "installed"
	StatusAvailable  PluginStatus = "available"
	StatusUpdate     PluginStatus = "update_available"
	StatusDisabled   PluginStatus = "disabled"
	StatusError      PluginStatus = "error"
)

// Permission 插件权限
type Permission string

const (
	PermFileSystem Permission = "filesystem" // 文件系统访问
	PermNetwork    Permission = "network"    // 网络访问
	PermMemory     Permission = "memory"     // 记忆系统访问
	PermTool       Permission = "tool"       // 工具注册
	PermRAG        Permission = "rag"        // RAG 知识库访问
	PermSession    Permission = "session"    // 会话访问
	PermConfig     Permission = "config"     // 配置修改
	PermAdmin      Permission = "admin"      // 管理员操作
)

// Manifest 插件清单（plugin.yaml 格式）
type Manifest struct {
	// 基本信息
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Author      string   `yaml:"author" json:"author"`
	Description string   `yaml:"description" json:"description"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Homepage    string   `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository  string   `yaml:"repository,omitempty" json:"repository,omitempty"`

	// 入口
	Entry    string   `yaml:"entry" json:"entry"`       // 入口文件（相对于插件目录）
	Type     string   `yaml:"type" json:"type"`         // 插件类型: skill, tool, provider, hook
	Tags     []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// 依赖
	Dependencies []Dependency `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	MinVersion   string       `yaml:"min_version,omitempty" json:"min_version,omitempty"` // 最低 LuckyHarness 版本

	// 权限
	Permissions []Permission `yaml:"permissions,omitempty" json:"permissions,omitempty"`

	// 配置
	ConfigSchema map[string]ConfigField `yaml:"config_schema,omitempty" json:"config_schema,omitempty"`

	// 运行时（安装后填充）
	InstallPath string    `yaml:"-" json:"install_path,omitempty"`
	InstalledAt time.Time `yaml:"-" json:"installed_at,omitempty"`
	Checksum    string    `yaml:"-" json:"checksum,omitempty"` // 安装包校验和
}

// Dependency 插件依赖
type Dependency struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"` // semver range
}

// ConfigField 插件配置字段定义
type ConfigField struct {
	Type        string `yaml:"type" json:"type"`                 // string, int, bool, float
	Default     any    `yaml:"default,omitempty" json:"default,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
}

// Validate 验证 Manifest 完整性
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if m.Entry == "" {
		return fmt.Errorf("plugin entry is required")
	}
	if m.Type == "" {
		return fmt.Errorf("plugin type is required")
	}

	// 验证类型
	validTypes := map[string]bool{
		"skill":    true,
		"tool":     true,
		"provider": true,
		"hook":     true,
	}
	if !validTypes[m.Type] {
		return fmt.Errorf("invalid plugin type: %s (must be skill, tool, provider, or hook)", m.Type)
	}

	// 验证名称格式（小写字母、数字、连字符）
	for _, ch := range m.Name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return fmt.Errorf("invalid plugin name: %s (lowercase letters, digits, hyphens only)", m.Name)
		}
	}

	// 验证版本格式（semver）
	if !isValidVersion(m.Version) {
		return fmt.Errorf("invalid version format: %s (expected semver)", m.Version)
	}

	return nil
}

// FullName 返回插件的完整标识（name@version）
func (m *Manifest) FullName() string {
	return fmt.Sprintf("%s@%s", m.Name, m.Version)
}

// HasPermission 检查插件是否有指定权限
func (m *Manifest) HasPermission(perm Permission) bool {
	for _, p := range m.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// HasDependency 检查插件是否依赖指定插件
func (m *Manifest) HasDependency(name string) bool {
	for _, d := range m.Dependencies {
		if d.Name == name {
			return true
		}
	}
	return false
}

// isValidVersion 检查版本号是否为有效 semver
func isValidVersion(v string) bool {
	if v == "" {
		return false
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				// 允许预发布后缀如 1.0.0-alpha
				if ch == '-' && len(p) > 0 {
					return true
				}
				return false
			}
		}
	}
	return true
}