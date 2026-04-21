package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// WebSearchTool 网络搜索（多源降级：Brave → ddgs → DDG Lite → SearXNG）
// 照 skills/web-search/SKILL.md 设计：三源降级 + 搜索策略 + 来源标注
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

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "brave"
	}

	if mode == "deep" {
		return handleDeepSearch(cfg, query, count, provider)
	}

	// ── quick 模式：按优先级尝试搜索源，任一成功即返回 ──
	// 降级链：Brave → ddgs → DDG Lite → SearXNG

	// 1. Brave Search API
	if provider == "brave" || (provider != "ddgs" && provider != "searxng") {
		if result, err := searchWithBrave(cfg, query, count); err == nil && result != "" {
			return annotateSource(result, "Brave"), nil
		}
	}

	// 2. ddgs Python 包（绕过 DDG 验证码，推荐降级方案）
	if provider == "ddgs" || provider == "brave" {
		if result, err := searchWithDDGS(query, count); err == nil && result != "" {
			return annotateSource(result, "DDG (ddgs)"), nil
		}
	}

	// 3. DDG Lite curl（可能遇到验证码，最后降级）
	if result, err := searchWithDDGLite(query, count); err == nil && result != "" {
		return annotateSource(result, "DDG Lite"), nil
	}

	// 4. SearXNG 自部署
	if provider == "searxng" || cfg.BaseURL != "" {
		if result, err := searchWithSearXNG(cfg, query, count); err == nil && result != "" {
			return annotateSource(result, "SearXNG"), nil
		}
	}

	return fmt.Sprintf("No results found for '%s' (all search sources failed)", query), nil
}

// handleDeepSearch 深度搜索模式：多源交叉验证，合并去重
// 照 SKILL.md「深度调研」策略：多源搜索 → 合并去重 → 标注来源
func handleDeepSearch(cfg *WebSearchConfig, query string, count int, provider string) (string, error) {
	type sourceResult struct {
		source  string
		entries []searchEntry
	}

	var sources []sourceResult

	// 从多个源收集结果
	collect := func(name string, entries []searchEntry) {
		if len(entries) > 0 {
			sources = append(sources, sourceResult{source: name, entries: entries})
		}
	}

	// 1. Brave
	if provider == "brave" || (provider != "ddgs" && provider != "searxng") {
		if entries, err := searchWithBraveEntries(cfg, query, count); err == nil {
			collect("Brave", entries)
		}
	}

	// 2. ddgs
	if entries, err := searchWithDDGSEntries(query, count); err == nil {
		collect("DDG", entries)
	}

	// 3. SearXNG
	if provider == "searxng" || cfg.BaseURL != "" {
		if entries, err := searchWithSearXNGEntries(cfg, query, count); err == nil {
			collect("SearXNG", entries)
		}
	}

	if len(sources) == 0 {
		return fmt.Sprintf("No results found for '%s' (all search sources failed)", query), nil
	}

	// 合并去重（按 URL 去重，保留多源标注）
	seen := make(map[string]*mergedEntry)
	var order []string // 保持插入顺序

	for _, src := range sources {
		for _, e := range src.entries {
			normalizedURL := normalizeURL(e.URL)
			if existing, ok := seen[normalizedURL]; ok {
				existing.sources = append(existing.sources, src.source)
			} else {
				seen[normalizedURL] = &mergedEntry{
					title:   e.Title,
					url:     e.URL,
					snippet: e.Snippet,
					sources: []string{src.source},
				}
				order = append(order, normalizedURL)
			}
		}
	}

	// 格式化输出
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Results for: %s (deep search, %d sources)\n\n", query, len(sources)))
	for i, urlKey := range order {
		if i >= count {
			break
		}
		e := seen[urlKey]
		sourceTag := strings.Join(e.sources, "+")
		b.WriteString(fmt.Sprintf("%d. %s [%s]\n   %s\n", i+1, e.title, sourceTag, e.url))
		if e.snippet != "" {
			b.WriteString(fmt.Sprintf("   %s\n", e.snippet))
		}
		b.WriteString("\n")
	}

	result := b.String()
	if len(result) > 12000 {
		result = result[:12000] + "\n... (truncated)"
	}
	return result, nil
}

// searchEntry 统一的搜索结果条目
type searchEntry struct {
	Title   string
	URL     string
	Snippet string
}

// mergedEntry 合并去重后的条目
type mergedEntry struct {
	title   string
	url     string
	snippet string
	sources []string
}

// normalizeURL 简单 URL 归一化（去尾斜杠、去 fragment、小写 host）
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}

// annotateSource 给搜索结果标注来源
func annotateSource(result, source string) string {
	// 在 "Results for:" 行后插入来源标注
	return strings.Replace(result, "Results for:", "[Source: "+source+"] Results for:", 1)
}

// ── Brave Search API ─────────────────────────────────────────────────────────

func searchWithBrave(cfg *WebSearchConfig, query string, count int) (string, error) {
	entries, err := searchWithBraveEntries(cfg, query, count)
	if err != nil {
		return "", err
	}
	return formatEntries(query, entries, count), nil
}

func searchWithBraveEntries(cfg *WebSearchConfig, query string, count int) ([]searchEntry, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("BRAVE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("brave: no API key configured")
	}

	reqURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		urlEncode(query), count)

	cmd := exec.Command("curl", "-s", "-L", reqURL,
		"-H", "Accept: application/json",
		"-H", "X-Subscription-Token: "+apiKey,
		"--max-time", "10",
	)
	if cfg.Proxy != "" {
		cmd.Args = append(cmd.Args, "--proxy", cfg.Proxy)
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("brave api request failed: %w", err)
	}

	output := stdout.String()
	if output == "" {
		return nil, fmt.Errorf("brave: empty response")
	}

	var resp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("brave: parse response failed: %w", err)
	}

	if len(resp.Web.Results) == 0 {
		return nil, fmt.Errorf("brave: no results")
	}

	entries := make([]searchEntry, 0, len(resp.Web.Results))
	for _, r := range resp.Web.Results {
		entries = append(entries, searchEntry{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return entries, nil
}

// ── ddgs Python 包 ───────────────────────────────────────────────────────────

func searchWithDDGS(query string, count int) (string, error) {
	entries, err := searchWithDDGSEntries(query, count)
	if err != nil {
		return "", err
	}
	return formatEntries(query, entries, count), nil
}

func searchWithDDGSEntries(query string, count int) ([]searchEntry, error) {
	script := fmt.Sprintf(
		`import json; from ddgs import DDGS; ddgs=DDGS(timeout=10); results=ddgs.text(%q, max_results=%d); print(json.dumps(results, ensure_ascii=False))`,
		query, count,
	)

	cmd := exec.Command("python3", "-c", script)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ddgs search failed: %w", err)
	}

	output := stdout.String()
	if output == "" || output == "[]" {
		return nil, fmt.Errorf("ddgs returned empty results")
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		return nil, fmt.Errorf("ddgs: parse failed: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("ddgs returned empty results")
	}

	entries := make([]searchEntry, 0, len(results))
	for _, r := range results {
		title, _ := r["title"].(string)
		href, _ := r["href"].(string)
		body, _ := r["body"].(string)
		entries = append(entries, searchEntry{Title: title, URL: href, Snippet: body})
	}
	return entries, nil
}

// ── DDG Lite curl ────────────────────────────────────────────────────────────

func searchWithDDGLite(query string, count int) (string, error) {
	cmd := exec.Command("curl", "-s", "-L",
		"https://lite.duckduckgo.com/lite/",
		"-d", "q="+urlEncode(query),
		"-d", "kl=cn-zh",
		"--max-time", "10",
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("curl ddg lite failed: %w", err)
	}

	output := stdout.String()
	if output == "" {
		return "", fmt.Errorf("ddg lite returned empty response")
	}

	// 检测验证码/反爬页面
	if strings.Contains(output, "challenge-form") ||
		strings.Contains(output, "anomaly-modal") ||
		strings.Contains(output, "confirm this search was made by a human") {
		return "", fmt.Errorf("ddg lite returned captcha/challenge page")
	}

	result := parseDDGLiteHTML(output, count)
	if result == "" {
		return "", fmt.Errorf("ddg lite: no parseable results")
	}

	if len(result) > 5000 {
		result = result[:5000] + "\n... (truncated)"
	}
	return result, nil
}

// ── SearXNG 自部署 ──────────────────────────────────────────────────────────

func searchWithSearXNG(cfg *WebSearchConfig, query string, count int) (string, error) {
	entries, err := searchWithSearXNGEntries(cfg, query, count)
	if err != nil {
		return "", err
	}
	return formatEntries(query, entries, count), nil
}

func searchWithSearXNGEntries(cfg *WebSearchConfig, query string, count int) ([]searchEntry, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("SEARXNG_BASE_URL")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("searxng: no base URL configured")
	}

	reqURL := fmt.Sprintf("%s/search?q=%s&format=json&limit=%d",
		strings.TrimRight(baseURL, "/"), urlEncode(query), count)

	cmd := exec.Command("curl", "-s", "-L", reqURL,
		"-H", "User-Agent: RightClaw/1.0",
		"--max-time", "10",
	)
	if cfg.Proxy != "" {
		cmd.Args = append(cmd.Args, "--proxy", cfg.Proxy)
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("searxng request failed: %w", err)
	}

	output := stdout.String()
	if output == "" {
		return nil, fmt.Errorf("searxng: empty response")
	}

	var resp struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("searxng: parse response failed: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("searxng: no results")
	}

	entries := make([]searchEntry, 0, len(resp.Results))
	for _, r := range resp.Results {
		entries = append(entries, searchEntry{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return entries, nil
}

// ── HTML 解析辅助 ────────────────────────────────────────────────────────────

// formatEntries 将 searchEntry 列表格式化为可读文本
func formatEntries(query string, entries []searchEntry, count int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Results for: %s\n\n", query))
	for i, e := range entries {
		if i >= count {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, e.Title, e.URL))
		if e.Snippet != "" {
			b.WriteString(fmt.Sprintf("   %s\n", e.Snippet))
		}
		b.WriteString("\n")
	}
	result := b.String()
	if len(result) > 8000 {
		result = result[:8000] + "\n... (truncated)"
	}
	return result
}

func parseDDGLiteHTML(html string, count int) string {
	var b strings.Builder
	b.WriteString("Results (DDG Lite):\n\n")

	linkRe := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>(.*?)</a>`)

	links := linkRe.FindAllStringSubmatch(html, -1)
	snippets := snippetRe.FindAllStringSubmatch(html, -1)

	n := len(links)
	if n > count {
		n = count
	}

	for i := 0; i < n; i++ {
		url := links[i][1]
		title := stripHTMLTags(links[i][2])
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, title, url))
		if i < len(snippets) {
			snippet := stripHTMLTags(snippets[i][1])
			if snippet != "" {
				b.WriteString(fmt.Sprintf("   %s\n", snippet))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

func stripHTMLTags(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}

func urlEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// validateFetchURL 校验 URL 是否安全（SSRF 防护）
// 仅允许 http/https scheme，禁止私有 IP 和云元数据地址
func validateFetchURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	// 仅允许 http/https
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme %q not allowed (only http/https)", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}

	// 禁止私有 IP / 回环 / 链路本地 / 云元数据
	privateRanges := []struct {
		prefix string
		name   string
	}{
		{"127.", "loopback"},
		{"10.", "private"},
		{"192.168.", "private"},
		{"169.254.", "link-local/metadata"},
		{"0.", "unspecified"},
		{"::1", "loopback"},
		{"fc", "unique-local"},  // IPv6 fc00::/7
		{"fd", "unique-local"},  // IPv6 fd00::/8
		{"fe80", "link-local"},  // IPv6 fe80::/10
	}
	lowerHost := strings.ToLower(host)
	for _, r := range privateRanges {
		if strings.HasPrefix(lowerHost, r.prefix) {
			return fmt.Errorf("host %q is %s address (not allowed)", host, r.name)
		}
	}

	// 172.16.0.0/12 范围检查
	if strings.HasPrefix(host, "172.") {
		parts := strings.SplitN(host, ".", 3)
		if len(parts) >= 2 {
			if second, err := strconv.Atoi(parts[1]); err == nil && second >= 16 && second <= 31 {
				return fmt.Errorf("host %q is private address (not allowed)", host)
			}
		}
	}

	// 禁止 localhost
	if lowerHost == "localhost" {
		return fmt.Errorf("localhost not allowed")
	}

	return nil
}

// ── WebFetchTool ─────────────────────────────────────────────────────────────

// WebFetchTool 抓取 URL 内容（照 SKILL.md 设计：Defuddle → Jina → curl 降级）
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

	// SSRF 防护：校验 URL scheme
	if err := validateFetchURL(fetchURL); err != nil {
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

	// 策略 1: Defuddle CLI（照 SKILL.md：优先用 Defuddle 提取干净 Markdown）
	if result, err := fetchWithDefuddle(fetchURL, maxChars); err == nil && result != "" {
		return result, nil
	}

	// 策略 2: Jina Reader API（免费额度，提取正文效果好）
	if result, err := fetchWithJina(cfg, fetchURL, maxChars); err == nil && result != "" {
		return result, nil
	}

	// 策略 3: curl + strip HTML（本地降级）
	if result, err := fetchWithCurl(cfg, fetchURL, maxChars); err == nil && result != "" {
		return result, nil
	}

	return fmt.Sprintf("Failed to fetch %s (all methods failed)", fetchURL), nil
}

// fetchWithDefuddle 使用 defuddle CLI 提取网页正文为干净 Markdown
func fetchWithDefuddle(fetchURL string, maxChars int) (string, error) {
	// 检查 defuddle 是否可用
	cmd := exec.Command("which", "defuddle")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("defuddle not installed")
	}

	cmd = exec.Command("defuddle", "parse", fetchURL, "--md")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("defuddle parse failed: %w", err)
	}

	output := stdout.String()
	if output == "" {
		return "", fmt.Errorf("defuddle: empty result")
	}

	if len(output) > maxChars {
		output = output[:maxChars] + "\n... (truncated)"
	}

	return output, nil
}

func fetchWithJina(cfg *WebSearchConfig, url string, maxChars int) (string, error) {
	jinaKey := os.Getenv("JINA_API_KEY")

	curlArgs := []string{"-s", "-L",
		"https://r.jina.ai/" + url,
		"-H", "Accept: application/json",
		"-H", "User-Agent: RightClaw/1.0",
		"--max-time", "20",
	}
	if jinaKey != "" {
		curlArgs = append(curlArgs, "-H", "Authorization: Bearer "+jinaKey)
	}
	if cfg.Proxy != "" {
		curlArgs = append(curlArgs, "--proxy", cfg.Proxy)
	}

	cmd := exec.Command("curl", curlArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("jina fetch failed: %w", err)
	}

	output := stdout.String()
	if output == "" {
		return "", fmt.Errorf("jina: empty response")
	}

	// 解析 Jina JSON 响应
	var resp struct {
		Data struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return "", fmt.Errorf("jina: parse failed: %w", err)
	}

	if resp.Data.Content == "" {
		return "", fmt.Errorf("jina: no content extracted")
	}

	var b strings.Builder
	if resp.Data.Title != "" {
		b.WriteString(fmt.Sprintf("# %s\n\n", resp.Data.Title))
	}
	content := resp.Data.Content
	if len(content) > maxChars {
		content = content[:maxChars] + "\n... (truncated)"
	}
	b.WriteString(content)

	return b.String(), nil
}

func fetchWithCurl(cfg *WebSearchConfig, url string, maxChars int) (string, error) {
	curlArgs := []string{"-s", "-L", url,
		"-H", "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_2) AppleWebKit/537.36",
		"--max-time", "15",
	}
	if cfg.Proxy != "" {
		curlArgs = append(curlArgs, "--proxy", cfg.Proxy)
	}

	cmd := exec.Command("curl", curlArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("curl fetch failed: %w", err)
	}

	output := stdout.String()
	if output == "" {
		return "", fmt.Errorf("curl: empty response")
	}

	// 简单提取：去 HTML 标签，保留文本
	text := stripHTMLTags(output)
	text = normalizeWhitespace(text)

	if len(text) > maxChars {
		text = text[:maxChars] + "\n... (truncated)"
	}

	if len(text) < 50 {
		return "", fmt.Errorf("curl: too little content extracted")
	}

	return text, nil
}

func normalizeWhitespace(s string) string {
	// 多个空白字符合并为一个空格
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
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
