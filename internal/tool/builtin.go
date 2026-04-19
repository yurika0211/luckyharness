package tool

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	workdir := ""
	if w, ok := args["workdir"]; ok {
		workdir, _ = w.(string)
	}

	ctx := time.Duration(timeout) * time.Second
	cmd := exec.Command("sh", "-c", command)
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

// WebSearchTool 网络搜索（占位，后续接入真实 API）
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

	// v0.5.0: 占位实现，使用 shell curl 调用 DuckDuckGo
	// 后续版本将接入专用搜索 API
	count := 5
	if c, ok := args["count"]; ok {
		switch v := c.(type) {
		case float64:
			count = int(v)
		case int:
			count = v
		}
	}

	_ = count // suppress unused

	// 使用 curl 调用 DuckDuckGo Lite
	cmd := exec.Command("curl", "-s", "-L",
		"https://lite.duckduckgo.com/lite/",
		"-d", "q="+query,
		"-d", "format=json",
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Web search for '%s': API not yet configured. Query received.", query), nil
	}

	output := stdout.String()
	if len(output) == 0 {
		return fmt.Sprintf("No results found for '%s'", query), nil
	}

	// 截断
	if len(output) > 5000 {
		output = output[:5000] + "\n... (truncated)"
	}

	return output, nil
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
