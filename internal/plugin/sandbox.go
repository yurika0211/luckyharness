package plugin

import (
	"fmt"
	"time"
)

// Sandbox 插件沙箱 — 权限控制 + 资源限制
type Sandbox struct {
	mu       map[string]*SandboxConfig // plugin name -> config
	limits   ResourceLimits             // 全局默认限制
	enforcer PermissionEnforcer        // 权限执行器
}

// SandboxConfig 插件沙箱配置
type SandboxConfig struct {
	Permissions []Permission   // 允许的权限
	Limits      ResourceLimits // 资源限制
	Restricted  bool           // 是否为受限模式（默认 true）
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	MaxMemoryMB     int           // 最大内存（MB）
	MaxCPUPercent   int           // 最大 CPU 使用率（%）
	MaxGoroutines   int           // 最大并发 goroutine 数
	Timeout         time.Duration // 执行超时
	MaxOutputBytes  int           // 最大输出字节数
	MaxCallsPerMin  int           // 每分钟最大调用次数
}

// DefaultResourceLimits 默认资源限制
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxMemoryMB:    256,
		MaxCPUPercent:  50,
		MaxGoroutines:  10,
		Timeout:        30 * time.Second,
		MaxOutputBytes: 1024 * 1024, // 1MB
		MaxCallsPerMin: 60,
	}
}

// PermissionEnforcer 权限执行器接口
type PermissionEnforcer interface {
	// CheckPermission 检查插件是否有指定权限
	CheckPermission(pluginName string, perm Permission) error
	// CheckRateLimit 检查调用频率限制
	CheckRateLimit(pluginName string) error
	// CheckResourceLimit 检查资源限制
	CheckResourceLimit(pluginName string, resource string, amount int) error
}

// DefaultEnforcer 默认权限执行器
type DefaultEnforcer struct {
	callCounts map[string]*callCounter
}

type callCounter struct {
	counts   []time.Time
	maxPerMin int
}

// NewSandbox 创建插件沙箱
func NewSandbox(limits ResourceLimits) *Sandbox {
	return &Sandbox{
		mu:       make(map[string]*SandboxConfig),
		limits:   limits,
		enforcer: &DefaultEnforcer{callCounts: make(map[string]*callCounter)},
	}
}

// NewDefaultSandbox 创建默认沙箱
func NewDefaultSandbox() *Sandbox {
	return NewSandbox(DefaultResourceLimits())
}

// RegisterPlugin 为插件注册沙箱配置
func (s *Sandbox) RegisterPlugin(name string, manifest *Manifest) {
	config := &SandboxConfig{
		Permissions: manifest.Permissions,
		Limits:      s.limits,
		Restricted:  true,
	}
	s.mu[name] = config
}

// UnregisterPlugin 注销插件的沙箱配置
func (s *Sandbox) UnregisterPlugin(name string) {
	delete(s.mu, name)
}

// CheckPermission 检查插件是否有指定权限
func (s *Sandbox) CheckPermission(pluginName string, perm Permission) error {
	config, ok := s.mu[pluginName]
	if !ok {
		return fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}

	// admin 权限始终需要显式声明
	if perm == PermAdmin {
		for _, p := range config.Permissions {
			if p == PermAdmin {
				return nil
			}
		}
		return fmt.Errorf("plugin %s does not have admin permission", pluginName)
	}

	// 检查权限列表
	for _, p := range config.Permissions {
		if p == perm {
			return nil
		}
	}

	return fmt.Errorf("plugin %s does not have %s permission", pluginName, perm)
}

// CheckRateLimit 检查调用频率限制
func (s *Sandbox) CheckRateLimit(pluginName string) error {
	config, ok := s.mu[pluginName]
	if !ok {
		return fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}

	// 简单的频率限制检查（实际实现需要更精确的滑动窗口）
	// 这里只做接口定义，具体逻辑由 enforcer 实现
	_ = config.Limits.MaxCallsPerMin
	return nil
}

// GetLimits 获取插件的资源限制
func (s *Sandbox) GetLimits(pluginName string) (ResourceLimits, error) {
	config, ok := s.mu[pluginName]
	if !ok {
		return ResourceLimits{}, fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}
	return config.Limits, nil
}

// SetLimits 设置插件的资源限制
func (s *Sandbox) SetLimits(pluginName string, limits ResourceLimits) error {
	config, ok := s.mu[pluginName]
	if !ok {
		return fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}
	config.Limits = limits
	return nil
}

// IsRestricted 检查插件是否为受限模式
func (s *Sandbox) IsRestricted(pluginName string) bool {
	config, ok := s.mu[pluginName]
	if !ok {
		return true // 未知插件默认受限
	}
	return config.Restricted
}

// SetRestricted 设置插件受限模式
func (s *Sandbox) SetRestricted(pluginName string, restricted bool) error {
	config, ok := s.mu[pluginName]
	if !ok {
		return fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}
	config.Restricted = restricted
	return nil
}

// GrantPermission 授予插件额外权限
func (s *Sandbox) GrantPermission(pluginName string, perm Permission) error {
	config, ok := s.mu[pluginName]
	if !ok {
		return fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}

	// 检查是否已有
	for _, p := range config.Permissions {
		if p == perm {
			return nil // 已有
		}
	}

	config.Permissions = append(config.Permissions, perm)
	return nil
}

// RevokePermission 撤销插件权限
func (s *Sandbox) RevokePermission(pluginName string, perm Permission) error {
	config, ok := s.mu[pluginName]
	if !ok {
		return fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}

	newPerms := make([]Permission, 0, len(config.Permissions))
	for _, p := range config.Permissions {
		if p != perm {
			newPerms = append(newPerms, p)
		}
	}
	config.Permissions = newPerms
	return nil
}

// GetPermissions 获取插件的所有权限
func (s *Sandbox) GetPermissions(pluginName string) ([]Permission, error) {
	config, ok := s.mu[pluginName]
	if !ok {
		return nil, fmt.Errorf("plugin not in sandbox: %s", pluginName)
	}
	return config.Permissions, nil
}

// --- DefaultEnforcer 实现 ---

// CheckPermission 实现 PermissionEnforcer
func (de *DefaultEnforcer) CheckPermission(pluginName string, perm Permission) error {
	// 默认执行器不做实际检查，由 Sandbox 管理
	return nil
}

// CheckRateLimit 实现 PermissionEnforcer
func (de *DefaultEnforcer) CheckRateLimit(pluginName string) error {
	// 简单的频率限制实现
	counter, ok := de.callCounts[pluginName]
	if !ok {
		counter = &callCounter{maxPerMin: 60}
		de.callCounts[pluginName] = counter
	}

	now := time.Now()
	// 清理超过1分钟的记录
	var recent []time.Time
	for _, t := range counter.counts {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	counter.counts = recent

	if len(counter.counts) >= counter.maxPerMin {
		return fmt.Errorf("rate limit exceeded for plugin %s", pluginName)
	}

	counter.counts = append(counter.counts, now)
	return nil
}

// CheckResourceLimit 实现 PermissionEnforcer
func (de *DefaultEnforcer) CheckResourceLimit(pluginName string, resource string, amount int) error {
	// 默认不做实际限制检查
	return nil
}

// FormatPermissions 格式化权限列表为文本
func FormatPermissions(perms []Permission) string {
	if len(perms) == 0 {
		return "(none)"
	}
	names := make([]string, len(perms))
	for i, p := range perms {
		names[i] = string(p)
	}
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}