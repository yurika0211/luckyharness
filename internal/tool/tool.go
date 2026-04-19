package tool

// Tool 代表一个可调用的工具
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]Param
	Handler     func(args map[string]any) (string, error)
}

// Param 代表工具参数
type Param struct {
	Type        string
	Description string
	Required    bool
	Default     any
}

// Registry 管理所有已注册的工具
type Registry struct {
	tools map[string]*Tool
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register 注册工具
func (r *Registry) Register(tool *Tool) {
	r.tools[tool.Name] = tool
}

// Get 获取工具
func (r *Registry) Get(name string) (*Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List 列出所有工具
func (r *Registry) List() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// Call 调用工具
func (r *Registry) Call(name string, args map[string]any) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", ErrToolNotFound{name: name}
	}
	return t.Handler(args)
}

// ErrToolNotFound 工具未找到错误
type ErrToolNotFound struct {
	name string
}

func (e ErrToolNotFound) Error() string {
	return "tool not found: " + e.name
}
