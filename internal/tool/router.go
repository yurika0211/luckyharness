package tool

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ToolRouter 工具路由器
// 根据工具名、分类、来源等条件路由到正确的工具
type ToolRouter struct {
	mu       sync.RWMutex
	registry *Registry
	aliases  map[string]string   // 别名 -> 工具名
	routes   []RouteRule         // 路由规则（按优先级排序）
}

// RouteRule 路由规则
type RouteRule struct {
	Name        string   // 规则名
	Priority    int      // 优先级（越高越先匹配）
	ToolPattern string   // 工具名匹配模式（支持 * 通配符）
	Target      string   // 目标工具名
	Enabled     bool     // 是否启用
}

// NewToolRouter 创建工具路由器
func NewToolRouter(registry *Registry) *ToolRouter {
	return &ToolRouter{
		registry: registry,
		aliases:  make(map[string]string),
		routes:   make([]RouteRule, 0),
	}
}

// Resolve 解析工具名（别名 → 路由 → 原名）
func (r *ToolRouter) Resolve(name string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. 检查别名
	if target, ok := r.aliases[name]; ok {
		name = target
	}

	// 2. 检查路由规则（按优先级排序）
	for _, rule := range r.routes {
		if !rule.Enabled {
			continue
		}
		if matchPattern(rule.ToolPattern, name) {
			name = rule.Target
			break
		}
	}

	// 3. 检查工具是否存在
	if _, ok := r.registry.Get(name); !ok {
		return "", ErrToolNotFound{name: name}
	}

	return name, nil
}

// AddAlias 添加别名
func (r *ToolRouter) AddAlias(alias, target string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查目标工具是否存在
	if _, ok := r.registry.Get(target); !ok {
		return ErrToolNotFound{name: target}
	}

	r.aliases[alias] = target
	return nil
}

// RemoveAlias 移除别名
func (r *ToolRouter) RemoveAlias(alias string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.aliases, alias)
}

// ListAliases 列出所有别名
func (r *ToolRouter) ListAliases() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range r.aliases {
		result[k] = v
	}
	return result
}

// AddRoute 添加路由规则
func (r *ToolRouter) AddRoute(rule RouteRule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = append(r.routes, rule)

	// 按优先级排序
	sort.Slice(r.routes, func(i, j int) bool {
		return r.routes[i].Priority > r.routes[j].Priority
	})
}

// RemoveRoute 移除路由规则
func (r *ToolRouter) RemoveRoute(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, rule := range r.routes {
		if rule.Name == name {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			break
		}
	}
}

// ListRoutes 列出所有路由规则
func (r *ToolRouter) ListRoutes() []RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]RouteRule, len(r.routes))
	copy(result, r.routes)
	return result
}

// EnableRoute 启用路由规则
func (r *ToolRouter) EnableRoute(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.routes {
		if r.routes[i].Name == name {
			r.routes[i].Enabled = true
			return nil
		}
	}
	return fmt.Errorf("route not found: %s", name)
}

// DisableRoute 禁用路由规则
func (r *ToolRouter) DisableRoute(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.routes {
		if r.routes[i].Name == name {
			r.routes[i].Enabled = false
			return nil
		}
	}
	return fmt.Errorf("route not found: %s", name)
}

// FormatRoutes 格式化路由规则列表
func (r *ToolRouter) FormatRoutes() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	b.WriteString("Tool Routes:\n")
	for _, rule := range r.routes {
		status := "✅"
		if !rule.Enabled {
			status = "❌"
		}
		b.WriteString(fmt.Sprintf("  %s [%d] %s: %s → %s\n",
			status, rule.Priority, rule.Name, rule.ToolPattern, rule.Target))
	}

	if len(r.aliases) > 0 {
		b.WriteString("\nAliases:\n")
		for alias, target := range r.aliases {
			b.WriteString(fmt.Sprintf("  %s → %s\n", alias, target))
		}
	}

	return b.String()
}

// matchPattern 匹配工具名模式（支持 * 通配符）
func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == name {
		return true
	}
	// 简单前缀匹配: "web_*" 匹配 "web_search", "web_fetch"
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(name, prefix)
	}
	// 简单后缀匹配: "*_search" 匹配 "web_search"
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(name, suffix)
	}
	return false
}
