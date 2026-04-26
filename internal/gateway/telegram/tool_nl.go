package telegram

import (
	"encoding/json"
	"fmt"
	"strings"
)

func humanizeThinkingProgress(content string) string {
	content = clipOneLine(content, 180)
	if content == "" {
		return ""
	}
	return ensureSentenceSuffix("我先理一下当前进度：" + content)
}

func humanizeToolCallProgress(step int, name, args string) string {
	if step <= 0 {
		step = 1
	}
	n := strings.TrimSpace(name)
	if n == "" {
		n = "unknown_tool"
	}
	lowerName := strings.ToLower(n)
	parsed := parseToolCallArgs(args)

	if strings.Contains(lowerName, "skill") {
		if skillName := pickArg(parsed, "skill", "skill_name", "name", "id"); skillName != "" {
			return fmt.Sprintf("先做第 %d 步：准备调用技能「%s」，补齐这类任务的处理链路。", step, clipOneLine(skillName, 60))
		}
		return fmt.Sprintf("先做第 %d 步：准备调用技能链路，补齐这个问题的专业上下文。", step)
	}

	switch lowerName {
	case "web_search":
		if q := pickArg(parsed, "query", "q", "keyword"); q != "" {
			return fmt.Sprintf("先做第 %d 步：联网搜索 %s，确认可用信息源。", step, quoteAndTrim(q, 90))
		}
		return fmt.Sprintf("先做第 %d 步：联网搜索相关资料，确认外部信息。", step)
	case "web_fetch":
		if u := pickArg(parsed, "url", "uri", "link"); u != "" {
			return fmt.Sprintf("先做第 %d 步：打开页面并核对原文：%s。", step, clipOneLine(u, 100))
		}
		return fmt.Sprintf("先做第 %d 步：读取网页正文，提取关键事实。", step)
	case "shell":
		if cmd := pickArg(parsed, "cmd", "command", "script"); cmd != "" {
			return fmt.Sprintf("先做第 %d 步：执行命令检查当前状态：%s。", step, clipOneLine(cmd, 100))
		}
		return fmt.Sprintf("先做第 %d 步：执行命令核对当前环境。", step)
	case "file_read":
		if p := pickArg(parsed, "path", "file", "filepath", "filename"); p != "" {
			return fmt.Sprintf("先做第 %d 步：查看文件 %s，确认现状。", step, clipOneLine(p, 100))
		}
		return fmt.Sprintf("先做第 %d 步：读取文件内容，确认上下文。", step)
	case "file_write":
		if p := pickArg(parsed, "path", "file", "filepath", "filename"); p != "" {
			return fmt.Sprintf("先做第 %d 步：更新文件 %s，落地本次修改。", step, clipOneLine(p, 100))
		}
		return fmt.Sprintf("先做第 %d 步：写入文件，落地本次修改。", step)
	case "recall":
		if q := pickArg(parsed, "query", "q", "keyword"); q != "" {
			return fmt.Sprintf("先做第 %d 步：回查历史上下文 %s。", step, quoteAndTrim(q, 90))
		}
		return fmt.Sprintf("先做第 %d 步：回查历史上下文，避免遗漏。", step)
	case "remember":
		return fmt.Sprintf("先做第 %d 步：记录关键信息，方便后续复用。", step)
	case "current_time":
		return fmt.Sprintf("先做第 %d 步：确认当前时间与时区信息。", step)
	}

	if keyHint := pickArg(parsed,
		"query", "url", "path", "cmd", "command",
		"title", "task_id", "name"); keyHint != "" {
		return fmt.Sprintf("先做第 %d 步：调用工具 %s，处理 %s。", step, n, clipOneLine(keyHint, 100))
	}
	return fmt.Sprintf("先做第 %d 步：调用工具 %s，补齐这一步信息。", step, n)
}

func humanizeToolResultProgress(step int, name, result string) string {
	if step <= 0 {
		step = 1
	}
	n := strings.TrimSpace(name)
	lowerName := strings.ToLower(n)

	switch lowerName {
	case "web_search":
		return fmt.Sprintf("第 %d 步完成：搜索结果已拿到，我继续筛选最相关的内容。", step)
	case "web_fetch":
		return fmt.Sprintf("第 %d 步完成：页面内容已读取，我继续提取关键细节。", step)
	case "shell":
		return fmt.Sprintf("第 %d 步完成：命令已经执行完，我继续根据输出推进。", step)
	case "file_read":
		return fmt.Sprintf("第 %d 步完成：目标文件已读完，我继续定位关键信息。", step)
	case "file_write":
		return fmt.Sprintf("第 %d 步完成：文件已更新，我继续做后续校验。", step)
	case "recall":
		return fmt.Sprintf("第 %d 步完成：历史上下文已核对，我继续往下处理。", step)
	case "remember":
		return fmt.Sprintf("第 %d 步完成：关键信息已记录。", step)
	case "current_time":
		return fmt.Sprintf("第 %d 步完成：时间信息已确认。", step)
	}

	if strings.Contains(lowerName, "skill") {
		return fmt.Sprintf("第 %d 步完成：技能链路已执行完成，我继续整理结论。", step)
	}

	if summary := humanizeToolResult(name, result); summary != "" {
		summary = strings.TrimPrefix(summary, "我")
		summary = strings.TrimSpace(summary)
		return ensureSentenceSuffix(fmt.Sprintf("第 %d 步完成：%s", step, summary))
	}
	return fmt.Sprintf("第 %d 步完成：工具调用已返回结果。", step)
}

func wrapFinalConclusion(finalOutput string) string {
	finalOutput = strings.TrimSpace(finalOutput)
	if finalOutput == "" {
		return "结论：当前轮没有可展示的最终内容。"
	}
	return "结论：\n" + finalOutput
}

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

func humanizeToolResult(name, result string) string {
	n := strings.TrimSpace(name)
	text := clipOneLine(result, 160)
	if text == "" {
		return ""
	}

	switch n {
	case "web_search":
		if strings.Contains(text, "Results for:") {
			return "我先找到了一批相关搜索结果，里面已经有官方页面和学生经验帖。"
		}
		if strings.Contains(strings.ToLower(text), "no results found") {
			return "我试着搜了一轮，但这一轮没有拿到有效结果。"
		}
		return "我先查到了一些和这个问题直接相关的搜索结果。"

	case "web_fetch":
		if strings.Contains(strings.ToLower(text), "failed to fetch") {
			return "我尝试读取网页正文，但这一页没有成功抓下来。"
		}
		return "我把网页正文也读了一遍，拿到了一些可直接引用的细节。"

	case "file_read":
		return "我读到了文件里的关键片段。"

	case "recall":
		if strings.Contains(text, "没有找到") {
			return "我先查了历史上下文，不过这件事之前没有现成记录。"
		}
		return "我先回顾了一下之前相关的上下文。"

	case "current_time":
		return "我顺手确认了当前时间信息。"
	}

	return fmt.Sprintf("我已经拿到了 %s 这一步的结果。", n)
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

func ensureSentenceSuffix(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "。") || strings.HasSuffix(s, "！") || strings.HasSuffix(s, "？") ||
		strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?") {
		return s
	}
	return s + "。"
}
