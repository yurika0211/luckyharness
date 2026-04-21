package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RegisterBuiltinTools 注册所有内置工具
func RegisterBuiltinTools(r *Registry) {
	r.Register(ShellTool())
	r.Register(FileReadTool())
	r.Register(FileWriteTool())
	r.Register(FileListTool())
	r.Register(WebSearchTool())
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
		for k, v := range env {
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

// WebSearchTool 网络搜索
func WebSearchTool() *Tool {
	return &Tool{
		Name:        "web_search",
		Description: "Search the web for information. Returns search results with titles, URLs, and snippets.",
		Category:    CatBuiltin,
		Source:      "builtin",
		Permission:  PermApprove, // 网络请求需要审批
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
		},
		Handler: handleWebSearch,
	}
}

func handleWebSearch(args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	count := 5
	if c, ok := args["count"]; ok {
		switch v := c.(type) {
		case float64:
			count = int(v)
		case int:
			count = v
		}
	}

	// 策略 1: 使用 ddgs Python 包（绕过 DDG 验证码）
	if result, err := searchWithDDGS(query, count); err == nil && result != "" {
		return result, nil
	}

	// 策略 2: 降级到 curl DDG Lite（可能遇到验证码）
	if result, err := searchWithDDGLite(query, count); err == nil && result != "" {
		return result, nil
	}

	return fmt.Sprintf("No results found for '%s' (all search sources failed)", query), nil
}

// searchWithDDGS 使用 ddgs Python 包搜索（推荐，绕过验证码）
func searchWithDDGS(query string, count int) (string, error) {
	// 用 python3 -c 调用 ddgs，输出 JSON
	script := fmt.Sprintf(
		`import json; from ddgs import DDGS; ddgs=DDGS(timeout=10); results=ddgs.text(%q, max_results=%d); print(json.dumps(results, ensure_ascii=False))`,
		query, count,
	)

	cmd := exec.Command("python3", "-c", script)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ddgs search failed: %w", err)
	}

	output := stdout.String()
	if output == "" || output == "[]" {
		return "", fmt.Errorf("ddgs returned empty results")
	}

	// 解析 JSON 结果
	var results []map[string]any
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		// JSON 解析失败，返回原始输出
		if len(output) > 5000 {
			output = output[:5000] + "\n... (truncated)"
		}
		return output, nil
	}

	if len(results) == 0 {
		return "", fmt.Errorf("ddgs returned empty results")
	}

	// 格式化为可读文本
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Results for: %s\n\n", query))
	for i, r := range results {
		if i >= count {
			break
		}
		title, _ := r["title"].(string)
		href, _ := r["href"].(string)
		body, _ := r["body"].(string)
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, title, href))
		if body != "" {
			b.WriteString(fmt.Sprintf("   %s\n", body))
		}
		b.WriteString("\n")
	}

	result := b.String()
	if len(result) > 8000 {
		result = result[:8000] + "\n... (truncated)"
	}
	return result, nil
}

// searchWithDDGLite 使用 curl 调用 DDG Lite（降级方案，可能遇到验证码）
func searchWithDDGLite(query string, count int) (string, error) {
	cmd := exec.Command("curl", "-s", "-L",
		"https://lite.duckduckgo.com/lite/",
		"-d", "q="+query,
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

	// 简单解析 HTML 结果
	result := parseDDGLiteHTML(output, count)
	if result == "" {
		return "", fmt.Errorf("ddg lite: no parseable results")
	}

	if len(result) > 5000 {
		result = result[:5000] + "\n... (truncated)"
	}
	return result, nil
}

// parseDDGLiteHTML 从 DDG Lite HTML 中提取搜索结果
func parseDDGLiteHTML(html string, count int) string {
	var b strings.Builder
	b.WriteString("Results (DDG Lite):\n\n")

	// DDG Lite 结果在 <a class="result__a"> 中
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

// stripHTMLTags 去除 HTML 标签
func stripHTMLTags(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
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
