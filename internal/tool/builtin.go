package tool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yurika0211/luckyharness/internal/search"
)

// RegisterBuiltinTools 注册所有内置工具
func RegisterBuiltinTools(r *Registry) {
	RegisterBuiltinToolsWithConfig(r, nil)
}

// RegisterBuiltinToolsWithConfig 注册所有内置工具（带搜索配置）
func RegisterBuiltinToolsWithConfig(r *Registry, searchCfg *WebSearchConfig) {
	r.Register(ShellTool())
	r.Register(FileReadTool())
	r.Register(FileWriteTool())
	r.Register(FileListTool())
	r.Register(WebSearchTool(searchCfg))
	r.Register(WebFetchTool(searchCfg))
	r.Register(CurrentTimeTool())
}

// ShellTool 执行 shell 命令
func ShellTool() *Tool {
	return &Tool{
		Name:        "shell",
		Description: "Execute a shell command and return its output. Use for system operations, file manipulation, and running scripts.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermApprove, // shell 命令需要审批
		ShellAware:  true,
		Parameters: map[string]Param{
			"command": {
				Type:        "string",
				Description: "The shell command to execute",
				Required:    true,
			},
			"timeout": {
				Type:        "number",
				Description: "Timeout in seconds (default 30)",
				Required:    false,
				Default:     30,
			},
			"workdir": {
				Type:        "string",
				Description: "Working directory for the command",
				Required:    false,
			},
		},
		Handler: handleShell,
	}
}

func handleShell(args map[string]any) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command is required")
	}

	timeout := 30
	if t, ok := args["timeout"]; ok {
		switch v := t.(type) {
		case float64:
			timeout = int(v)
		case int:
			timeout = v
		}
	}
	// 硬上限 300 秒
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		timeout = 300
	}

	// 从 shell context 注入的值
	cwd, _ := args["_cwd"].(string)
	env, _ := args["_env"].(map[string]string)

	workdir := cwd
	if w, ok := args["workdir"]; ok {
		if ws, ok := w.(string); ok && ws != "" {
			workdir = ws
		}
	}

	// 构建 shell 前缀：注入环境变量
	prefix := ""
	if len(env) > 0 {
		// 合法环境变量名正则：字母/下划线开头，后跟字母/数字/下划线
		validEnvKey := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
		for k, v := range env {
			// 校验 key 防止 shell 注入
			if !validEnvKey.MatchString(k) {
				continue
			}
			// 转义单引号
			escaped := strings.ReplaceAll(v, "'", "'\\''")
			prefix += fmt.Sprintf("export %s='%s'; ", k, escaped)
		}
	}
	fullCommand := prefix + command

	ctx := time.Duration(timeout) * time.Second
	cmd := exec.Command("sh", "-c", fullCommand)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		output := stdout.String()
		if stderr.Len() > 0 {
			output += "\n[stderr]\n" + stderr.String()
		}
		if err != nil {
			output += fmt.Sprintf("\n[exit code: %v]", err)
		}
		// 截断过长输出
		if len(output) > 10000 {
			output = output[:10000] + "\n... (truncated)"
		}
		return output, nil
	case <-time.After(ctx):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("command timed out after %d seconds", timeout)
	}
}

// FileReadTool 读取文件内容
func FileReadTool() *Tool {
	return &Tool{
		Name:        "file_read",
		Description: "Read the contents of a file. Returns the file content as text.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermAuto, // 读文件自动批准
		Parameters: map[string]Param{
			"path": {
				Type:        "string",
				Description: "Path to the file to read",
				Required:    true,
			},
			"offset": {
				Type:        "number",
				Description: "Line number to start reading from (1-indexed)",
				Required:    false,
				Default:     1,
			},
			"limit": {
				Type:        "number",
				Description: "Maximum number of lines to read",
				Required:    false,
				Default:     2000,
			},
		},
		Handler: handleFileRead,
	}
}

func handleFileRead(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	// 路径安全检查
	if err := validatePath(path); err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	offset := 1
	if o, ok := args["offset"]; ok {
		switch v := o.(type) {
		case float64:
			offset = int(v)
		case int:
			offset = v
		}
	}
	if offset < 1 {
		offset = 1
	}

	limit := 2000
	if l, ok := args["limit"]; ok {
		switch v := l.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		}
	}

	start := offset - 1
	if start >= len(lines) {
		return "", fmt.Errorf("offset %d exceeds file length %d", offset, len(lines))
	}

	end := start + limit
	if end > len(lines) {
		end = len(lines)
	}

	// 带行号输出
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(fmt.Sprintf("%d| %s\n", i+1, lines[i]))
	}

	return b.String(), nil
}

// FileWriteTool 写入文件
func FileWriteTool() *Tool {
	return &Tool{
		Name:        "file_write",
		Description: "Write content to a file. Creates parent directories if needed.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermApprove, // 写文件需要审批
		Parameters: map[string]Param{
			"path": {
				Type:        "string",
				Description: "Path to the file to write",
				Required:    true,
			},
			"content": {
				Type:        "string",
				Description: "Content to write to the file",
				Required:    true,
			},
		},
		Handler: handleFileWrite,
	}
}

func handleFileWrite(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	// 路径安全检查
	if err := validatePath(path); err != nil {
		return "", err
	}

	// 创建父目录
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("Written %d bytes to %s", len(content), path), nil
}

// FileListTool 列出目录内容
func FileListTool() *Tool {
	return &Tool{
		Name:        "file_list",
		Description: "List the contents of a directory.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermAuto, // 列目录自动批准
		Parameters: map[string]Param{
			"path": {
				Type:        "string",
				Description: "Path to the directory to list",
				Required:    true,
			},
			"recursive": {
				Type:        "boolean",
				Description: "Whether to list recursively",
				Required:    false,
				Default:     false,
			},
		},
		Handler: handleFileList,
	}
}

func handleFileList(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	recursive := false
	if r, ok := args["recursive"]; ok {
		recursive, _ = r.(bool)
	}

	// 路径安全检查
	if err := validatePath(path); err != nil {
		return "", err
	}

	var b strings.Builder

	if recursive {
		err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(path, walkPath)
			if info.IsDir() {
				b.WriteString(fmt.Sprintf("  📁 %s/\n", rel))
			} else {
				b.WriteString(fmt.Sprintf("  📄 %s (%d bytes)\n", rel, info.Size()))
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("walk directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("read directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				b.WriteString(fmt.Sprintf("  📁 %s/\n", entry.Name()))
			} else {
				info, _ := entry.Info()
				b.WriteString(fmt.Sprintf("  📄 %s (%d bytes)\n", entry.Name(), info.Size()))
			}
		}
	}

	return b.String(), nil
}

// WebSearchConfig 搜索配置（从 config.Manager 传入）
// 保留兼容接口，内部转换到 search.SearchConfig
type WebSearchConfig struct {
	Provider   string // brave, ddgs, searxng（默认 brave）
	APIKey     string // Brave / Tavily / Jina API key
	BaseURL    string // SearXNG 自部署地址
	MaxResults int    // 最大结果数（默认 5）
	Proxy      string // HTTP/SOCKS5 代理
}

// defaultWebSearchConfig 返回默认搜索配置
func defaultWebSearchConfig() *WebSearchConfig {
	return &WebSearchConfig{
		Provider:   "brave",
		MaxResults: 5,
	}
}

// toSearchConfig 将 WebSearchConfig 转换为 search.SearchConfig
func (c *WebSearchConfig) toSearchConfig() *search.SearchConfig {
	cfg := search.DefaultSearchConfig()
	if c != nil {
		if c.Provider != "" {
			cfg.DefaultProvider = c.Provider
		}
		if c.APIKey != "" {
			// Map APIKey to the correct field based on provider
			switch c.Provider {
			case "brave":
				cfg.BraveAPIKey = c.APIKey
			case "exa":
				cfg.ExaAPIKey = c.APIKey
			case "jina":
				cfg.JinaAPIKey = c.APIKey
			default:
				cfg.BraveAPIKey = c.APIKey
			}
		}
		if c.BaseURL != "" {
			cfg.SearXNGBaseURL = c.BaseURL
		}
		if c.MaxResults > 0 {
			cfg.MaxResults = c.MaxResults
		}
		if c.Proxy != "" {
			cfg.Proxy = c.Proxy
		}
	}
	// Apply environment variable overrides
	cfg = search.SearchConfigFromEnv(cfg)
	return cfg
}

// getSearchManager creates a search.Manager from WebSearchConfig
func getSearchManagerFromConfig(cfg *WebSearchConfig) *search.Manager {
	return search.NewManager(cfg.toSearchConfig())
}

// WebSearchTool 网络搜索（委托 internal/search 包）
// 支持 quick（单源降级）和 deep（多源交叉验证）模式
func WebSearchTool(cfg *WebSearchConfig) *Tool {
	if cfg == nil {
		cfg = defaultWebSearchConfig()
	}
	return &Tool{
		Name:        "web_search",
		Description: "Search the web for information. Returns search results with titles, URLs, and snippets. Supports multiple providers with automatic fallback (Brave → DDG → SearXNG). Use mode='deep' for multi-source cross-validation.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermApprove,
		Parameters: map[string]Param{
			"query": {
				Type:        "string",
				Description: "Search query",
				Required:    true,
			},
			"count": {
				Type:        "number",
				Description: "Number of results (1-10)",
				Required:    false,
				Default:     5,
			},
			"mode": {
				Type:        "string",
				Description: "Search mode: 'quick' (single source), 'deep' (multi-source cross-validation, merges results from multiple providers)",
				Required:    false,
				Default:     "quick",
			},
		},
		Handler: func(args map[string]any) (string, error) {
			return handleWebSearch(cfg, args)
		},
	}
}

func handleWebSearch(cfg *WebSearchConfig, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	count := cfg.MaxResults
	if count <= 0 {
		count = 5
	}
	if c, ok := args["count"]; ok {
		switch v := c.(type) {
		case float64:
			count = int(v)
		case int:
			count = v
		}
	}
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	mode := "quick"
	if m, ok := args["mode"].(string); ok {
		mode = strings.ToLower(m)
	}

	mgr := getSearchManagerFromConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if mode == "deep" {
		dr, err := mgr.DeepSearch(ctx, query, count)
		if err != nil {
			return fmt.Sprintf("No results found for '%s' (deep search failed: %v)", query, err), nil
		}
		result := search.FormatDeepResults(query, dr)
		if len(result) > 12000 {
			result = result[:12000] + "\n... (truncated)"
		}
		return result, nil
	}

	// quick mode
	results, err := mgr.QuickSearch(ctx, query, count)
	if err != nil {
		return fmt.Sprintf("No results found for '%s' (search failed: %v)", query, err), nil
	}
	result := search.FormatResults(query, results)
	if len(result) > 8000 {
		result = result[:8000] + "\n... (truncated)"
	}
	return result, nil
}

// WebFetchTool 抓取 URL 内容（委托 internal/search 包）
// 降级链：Defuddle → Jina → curl
func WebFetchTool(cfg *WebSearchConfig) *Tool {
	if cfg == nil {
		cfg = defaultWebSearchConfig()
	}
	return &Tool{
		Name:        "web_fetch",
		Description: "Fetch and extract readable content from a URL. Returns page title and text content. Automatically uses Defuddle (best quality) → Jina Reader → curl fallback.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermApprove,
		Parameters: map[string]Param{
			"url": {
				Type:        "string",
				Description: "URL to fetch",
				Required:    true,
			},
			"max_chars": {
				Type:        "number",
				Description: "Maximum characters to return (default 50000)",
				Required:    false,
				Default:     50000,
			},
		},
		Handler: func(args map[string]any) (string, error) {
			return handleWebFetch(cfg, args)
		},
	}
}

func handleWebFetch(cfg *WebSearchConfig, args map[string]any) (string, error) {
	fetchURL, ok := args["url"].(string)
	if !ok {
		return "", fmt.Errorf("url is required")
	}

	// SSRF 防护：校验 URL
	if err := search.ValidateFetchURL(fetchURL); err != nil {
		return "", fmt.Errorf("url validation failed: %w", err)
	}

	maxChars := 50000
	if mc, ok := args["max_chars"]; ok {
		switch v := mc.(type) {
		case float64:
			maxChars = int(v)
		case int:
			maxChars = v
		}
	}

	mgr := getSearchManagerFromConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mgr.FetchURL(ctx, fetchURL, maxChars)
	if err != nil {
		return fmt.Sprintf("Failed to fetch %s (all methods failed: %v)", fetchURL, err), nil
	}

	var b strings.Builder
	if result.Title != "" {
		b.WriteString(fmt.Sprintf("# %s\n\n", result.Title))
	}
	content := result.Content
	if len(content) > maxChars {
		content = content[:maxChars] + "\n... (truncated)"
	}
	b.WriteString(content)

	return b.String(), nil
}

// CurrentTimeTool 获取当前时间
func CurrentTimeTool() *Tool {
	return &Tool{
		Name:        "current_time",
		Description: "Get the current date and time.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermAuto,
		Parameters:  map[string]Param{},
		Handler:     handleCurrentTime,
	}
}

func handleCurrentTime(args map[string]any) (string, error) {
	now := time.Now()
	return fmt.Sprintf("Current time: %s (%s)", now.Format("2006-01-02 15:04:05"), now.Location()), nil
}

// validatePath 路径安全检查（防止路径遍历）
func validatePath(path string) error {
	// 清理路径
	clean := filepath.Clean(path)

	// 检查路径遍历
	if strings.Contains(clean, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}

	return nil
}
