package tool

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// PermissionLevel 工具权限级别
type PermissionLevel int

const (
	PermAuto    PermissionLevel = iota // 自动批准（只读/安全操作）
	PermApprove                         // 需要用户批准（写操作/网络请求）
	PermDeny                            // 禁止使用
)

func (p PermissionLevel) String() string {
	switch p {
	case PermAuto:
		return "auto"
	case PermApprove:
		return "approve"
	case PermDeny:
		return "deny"
	default:
		return "unknown"
	}
}

// Category 工具分类
type Category string

const (
	CatBuiltin   Category = "builtin"   // 内置工具
	CatSkill     Category = "skill"     // Skill 插件工具
	CatMCP       Category = "mcp"      // MCP Server 工具
	CatDelegate  Category = "delegate" // 子代理委派
)

// Tool 代表一个可调用的工具
type Tool struct {
	Name         string
	Description   string
	Parameters   map[string]Param
	Handler      func(args map[string]any) (string, error)
	Permission   PermissionLevel // 权限级别
	Category     Category        // 工具分类
	Source       string          // 来源（skill名/mcp server名/builtin）
	Enabled      bool            // 是否启用
	// ShellAware 标记该工具需要 shell 上下文注入（cwd + env）
	ShellAware bool
	// ParallelSafe 标记该工具可安全并发执行（无状态、无副作用冲突）
	// 如 web_search, web_fetch, file_read, current_time, recall 等
	// shell, file_write 等有状态依赖的工具不应标记
	ParallelSafe bool
}

// Param 代表工具参数
type Param struct {
	Type        string
	Description string
	Required    bool
	Default     any
}

// ToOpenAIFormat 转换为 OpenAI function calling 格式
func (t *Tool) ToOpenAIFormat() map[string]any {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{},
		"required":    []string{},
	}

	props := make(map[string]any)
	var required []string
	for name, p := range t.Parameters {
		props[name] = map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if p.Default != nil {
			props[name].(map[string]any)["default"] = p.Default
		}
		if p.Required {
			required = append(required, name)
		}
	}
	params["properties"] = props
	if len(required) > 0 {
		params["required"] = required
	} else {
		params["required"] = []string{}
	}

	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  params,
		},
	}
}

// Registry 管理所有已注册的工具
type Registry struct {
	mu       sync.RWMutex
	tools    map[string]*Tool
	permConf map[string]PermissionLevel // 工具权限覆盖配置
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]*Tool),
		permConf: make(map[string]PermissionLevel),
	}
}

// Register 注册工具
func (r *Registry) Register(tool *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tool.Enabled = true
	r.tools[tool.Name] = tool
}

// Unregister 注销工具
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get 获取工具
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List 列出所有工具
func (r *Registry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

// ListByCategory 按分类列出工具
func (r *Registry) ListByCategory(cat Category) []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var tools []*Tool
	for _, t := range r.tools {
		if t.Category == cat {
			tools = append(tools, t)
		}
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

// ListEnabled 列出所有启用的工具
func (r *Registry) ListEnabled() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var tools []*Tool
	for _, t := range r.tools {
		if t.Enabled {
			tools = append(tools, t)
		}
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

// Call 调用工具
func (r *Registry) Call(name string, args map[string]any) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return "", ErrToolNotFound{name: name}
	}
	if !t.Enabled {
		return "", ErrToolDisabled{name: name}
	}

	// 检查权限覆盖
	perm := t.Permission
	if override, has := r.permConf[name]; has {
		perm = override
	}
	if perm == PermDeny {
		return "", ErrToolDenied{name: name}
	}

	return t.Handler(args)
}

// ShellContext 提供 shell 环境状态给工具调用
type ShellContext struct {
	Cwd string            // 当前工作目录
	Env map[string]string // 自定义环境变量
}

// CallWithShellContext 执行工具并注入 shell 上下文
// 对于 ShellAware 的工具，自动在 args 中注入 _cwd 和 _env
func (r *Registry) CallWithShellContext(name string, args map[string]any, sc *ShellContext) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return "", ErrToolNotFound{name: name}
	}
	if !t.Enabled {
		return "", ErrToolDisabled{name: name}
	}

	// 检查权限覆盖
	perm := t.Permission
	if override, has := r.permConf[name]; has {
		perm = override
	}
	if perm == PermDeny {
		return "", ErrToolDenied{name: name}
	}

	// 注入 shell 上下文
	if t.ShellAware && sc != nil {
		if args == nil {
			args = make(map[string]any)
		}
		args["_cwd"] = sc.Cwd
		args["_env"] = sc.Env
	}

	return t.Handler(args)
}

// CheckPermission 检查工具权限（不执行）
func (r *Registry) CheckPermission(name string) (PermissionLevel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	if !ok {
		return PermDeny, ErrToolNotFound{name: name}
	}

	perm := t.Permission
	if override, has := r.permConf[name]; has {
		perm = override
	}
	return perm, nil
}

// SetPermissionOverride 设置工具权限覆盖
func (r *Registry) SetPermissionOverride(name string, perm PermissionLevel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tools[name]; !ok {
		return ErrToolNotFound{name: name}
	}
	r.permConf[name] = perm
	return nil
}

// Enable 启用工具
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.tools[name]
	if !ok {
		return ErrToolNotFound{name: name}
	}
	t.Enabled = true
	return nil
}

// Disable 禁用工具
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.tools[name]
	if !ok {
		return ErrToolNotFound{name: name}
	}
	t.Enabled = false
	return nil
}

// Count 返回工具数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// FormatToolList 格式化工具列表为文本
func (r *Registry) FormatToolList() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	categories := map[Category][]*Tool{}
	for _, t := range r.tools {
		categories[t.Category] = append(categories[t.Category], t)
	}

	for _, cat := range []Category{CatBuiltin, CatSkill, CatMCP, CatDelegate} {
		tools, ok := categories[cat]
		if !ok || len(tools) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n[%s]\n", cat))
		for _, t := range tools {
			status := "✅"
			if !t.Enabled {
				status = "❌"
			}
			permLabel := ""
			switch t.Permission {
			case PermAuto:
				permLabel = "🟢"
			case PermApprove:
				permLabel = "🟡"
			case PermDeny:
				permLabel = "🔴"
			}
			b.WriteString(fmt.Sprintf("  %s %s %s: %s\n", status, permLabel, t.Name, t.Description))
		}
	}
	return b.String()
}

// --- 错误类型 ---

// ErrToolNotFound 工具未找到错误
type ErrToolNotFound struct {
	name string
}

func (e ErrToolNotFound) Error() string {
	return "tool not found: " + e.name
}

// ErrToolDisabled 工具已禁用
type ErrToolDisabled struct {
	name string
}

func (e ErrToolDisabled) Error() string {
	return "tool disabled: " + e.name
}

// ErrToolDenied 工具权限拒绝
type ErrToolDenied struct {
	name string
}

func (e ErrToolDenied) Error() string {
	return "tool denied: " + e.name
}
