package telegram

import (
	"encoding/json"
	"fmt"
	"strings"
)

func humanizeToolCall(name, args string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		n = "unknown_tool"
	}

	parsed := parseToolCallArgs(args)

	switch n {
	case "web_search":
		if q := pickArg(parsed, "query", "q", "keyword"); q != "" {
			return fmt.Sprintf("正在联网搜索：%s", quoteAndTrim(q, 80))
		}
		return "正在联网搜索相关信息"

	case "web_fetch":
		if u := pickArg(parsed, "url", "uri", "link"); u != "" {
			return fmt.Sprintf("正在读取网页：%s", clipOneLine(u, 90))
		}
		return "正在读取网页内容"

	case "shell":
		if cmd := pickArg(parsed, "cmd", "command", "script"); cmd != "" {
			return fmt.Sprintf("正在执行命令：%s", clipOneLine(cmd, 90))
		}
		return "正在执行终端命令"

	case "file_read":
		if p := pickArg(parsed, "path", "file", "filepath", "filename"); p != "" {
			return fmt.Sprintf("正在读取文件：%s", clipOneLine(p, 90))
		}
		return "正在读取文件"

	case "file_write":
		if p := pickArg(parsed, "path", "file", "filepath", "filename"); p != "" {
			return fmt.Sprintf("正在写入文件：%s", clipOneLine(p, 90))
		}
		return "正在写入文件"

	case "current_time":
		return "正在查询当前时间"

	case "remember":
		if content := pickArg(parsed, "content", "text", "memory"); content != "" {
			return fmt.Sprintf("正在保存记忆：%s", clipOneLine(content, 80))
		}
		return "正在保存记忆"

	case "recall":
		if q := pickArg(parsed, "query", "q", "keyword"); q != "" {
			return fmt.Sprintf("正在检索记忆：%s", quoteAndTrim(q, 80))
		}
		return "正在检索记忆"
	}

	if keyHint := pickArg(parsed,
		"query", "url", "path", "cmd", "command",
		"title", "task_id", "name"); keyHint != "" {
		return fmt.Sprintf("正在调用工具 %s：%s", n, clipOneLine(keyHint, 90))
	}
	return fmt.Sprintf("正在调用工具 %s", n)
}

func parseToolCallArgs(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func pickArg(args map[string]any, keys ...string) string {
	if len(args) == 0 {
		return ""
	}
	for _, key := range keys {
		v, ok := args[key]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			if s := strings.TrimSpace(x); s != "" {
				return s
			}
		case []any:
			if len(x) > 0 {
				s := strings.TrimSpace(fmt.Sprintf("%v", x[0]))
				if s != "" {
					return s
				}
			}
		default:
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func clipOneLine(s string, max int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if s == "" {
		return s
	}
	if max <= 0 {
		max = 80
	}
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func quoteAndTrim(s string, max int) string {
	s = clipOneLine(s, max)
	if s == "" {
		return s
	}
	return "「" + s + "」"
}
