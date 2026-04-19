package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yurika0211/luckyharness/internal/provider"
)

// ParseToolCalls 从 LLM 响应中解析工具调用
// 支持 OpenAI function calling 格式和文本格式
func ParseToolCalls(resp *provider.Response) []provider.ToolCall {
	// 优先使用结构化的 ToolCalls
	if len(resp.ToolCalls) > 0 {
		return resp.ToolCalls
	}

	// 尝试从文本中解析工具调用
	// 格式: ```tool\n{"name": "xxx", "arguments": {...}}\n```
	return parseTextToolCalls(resp.Content)
}

// parseTextToolCalls 从文本中解析工具调用
func parseTextToolCalls(content string) []provider.ToolCall {
	var calls []provider.ToolCall

	// 查找 ```tool ... ``` 块
	blocks := extractCodeBlocks(content, "tool")
	for _, block := range blocks {
		var call struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(block), &call); err != nil {
			continue
		}
		if call.Name == "" {
			continue
		}

		argsJSON, _ := json.Marshal(call.Arguments)
		calls = append(calls, provider.ToolCall{
			Name:      call.Name,
			Arguments: string(argsJSON),
		})
	}

	// 也支持 <tool_call> XML 格式
	xmlCalls := parseXMLToolCalls(content)
	calls = append(calls, xmlCalls...)

	return calls
}

// parseXMLToolCalls 解析 XML 格式的工具调用
// 格式: <tool_call>{"name": "xxx", "arguments": {...}}</tool_call>
func parseXMLToolCalls(content string) []provider.ToolCall {
	var calls []provider.ToolCall
	tag := "tool_call"

	for {
		start := strings.Index(content, "<"+tag+">")
		if start == -1 {
			break
		}
		end := strings.Index(content, "</"+tag+">")
		if end == -1 {
			break
		}

		inner := content[start+len(tag)+2 : end]
		content = content[end+len(tag)+3:]

		var call struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(inner)), &call); err != nil {
			continue
		}
		if call.Name == "" {
			continue
		}

		argsJSON, _ := json.Marshal(call.Arguments)
		calls = append(calls, provider.ToolCall{
			Name:      call.Name,
			Arguments: string(argsJSON),
		})
	}

	return calls
}

// extractCodeBlocks 提取指定语言的代码块
func extractCodeBlocks(content, lang string) []string {
	var blocks []string
	marker := "```" + lang

	for {
		idx := strings.Index(content, marker)
		if idx == -1 {
			break
		}

		after := content[idx+len(marker):]
		end := strings.Index(after, "```")
		if end == -1 {
			break
		}

		block := strings.TrimSpace(after[:end])
		blocks = append(blocks, block)
		content = after[end+3:]
	}

	return blocks
}

// FormatToolResult 格式化工具结果为消息
func FormatToolResult(toolName string, result string, err error) string {
	if err != nil {
		return fmt.Sprintf("[Tool Error: %s] %v", toolName, err)
	}
	return fmt.Sprintf("[Tool Result: %s] %s", toolName, result)
}
