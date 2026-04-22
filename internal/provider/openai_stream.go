package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// openaiChatRequest 是发送给 OpenAI API 的请求体
type openaiChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openaiMessage     `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
	Tools       []openaiTool        `json:"tools,omitempty"`
	ToolChoice  any                 `json:"tool_choice,omitempty"`
}

// openaiMessage 是 OpenAI API 的消息格式
type openaiMessage struct {
	Role       string              `json:"role"`
	Content    string              `json:"content,omitempty"`
	ToolCalls  []openaiToolCallResp `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"` // v0.16.0: tool 消息的 call ID
	Name       string              `json:"name,omitempty"`         // v0.16.0: tool 消息的函数名
}

// openaiToolCallResp 是 OpenAI 响应中的工具调用格式
type openaiToolCallResp struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// openaiTool 是 OpenAI function calling 的工具定义
type openaiTool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

// toolFunction 是工具的函数定义
type toolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// openaiChatResponse 是 OpenAI API 的响应体
type openaiChatResponse struct {
	ID      string          `json:"id"`
	Choices []openaiChoice  `json:"choices"`
	Usage   *openaiUsage    `json:"usage,omitempty"`
}

type openaiChoice struct {
	Index        int            `json:"index"`
	Message      openaiMessage  `json:"message"`
	Delta        *openaiDelta   `json:"delta,omitempty"`
	FinishReason string         `json:"finish_reason"`
}

type openaiDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []deltaToolCall `json:"tool_calls,omitempty"`
}

type deltaToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openaiSSEEvent 是 SSE 事件
type openaiSSEEvent struct {
	Data string
}

// callOpenAI 执行 OpenAI API 调用（非流式）
// 支持文本响应和工具调用解析
func callOpenAI(ctx context.Context, cfg Config, messages []Message, opts CallOptions) (*Response, error) {
	reqBody := openaiChatRequest{
		Model:       cfg.Model,
		Messages:    toOpenAIMessages(messages),
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
		Stream:      false,
	}

	// v0.16.0: 添加 function calling 工具定义
	if len(opts.Tools) > 0 {
		tools := make([]openaiTool, 0, len(opts.Tools))
		for _, t := range opts.Tools {
			fn, _ := t["function"].(map[string]any)
			tools = append(tools, openaiTool{
				Type:     "function",
				Function: newToolFunction(fn),
			})
		}
		reqBody.Tools = tools
		if opts.ToolChoice != nil {
			reqBody.ToolChoice = opts.ToolChoice
		} else {
			reqBody.ToolChoice = "auto"
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.APIBase+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	
	// v0.56.0: 添加额外请求头（如 User-Agent）
	for k, v := range cfg.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp openaiChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := chatResp.Choices[0]
	result := &Response{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Model:        cfg.Model,
	}
	if chatResp.Usage != nil {
		result.TokensUsed = chatResp.Usage.TotalTokens
	}

	// 解析工具调用
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	// v0.55.0: 非流式返回空 content 但有 completion_tokens 时，
	// 用流式重试（某些 API 代理如 api.boaiak.com 的 gpt-5.4-mini 非流式不返回 content）
	hasUsage := chatResp.Usage != nil && chatResp.Usage.CompletionTokens > 0
	if result.Content == "" && len(result.ToolCalls) == 0 && hasUsage {
		log.Printf("[provider] non-stream empty content with %d completion_tokens, retrying stream (model=%s)", chatResp.Usage.CompletionTokens, cfg.Model)
		streamResult, err := retryWithStream(ctx, cfg, messages, opts)
		if err == nil && streamResult != nil && (streamResult.Content != "" || len(streamResult.ToolCalls) > 0) {
			log.Printf("[provider] stream retry OK: content_len=%d, tool_calls=%d", len(streamResult.Content), len(streamResult.ToolCalls))
			return streamResult, nil
		}
		if err != nil {
			log.Printf("[provider] stream retry failed: %v", err)
		} else {
			log.Printf("[provider] stream retry also empty: content_len=%d", len(streamResult.Content))
		}
	}

	return result, nil
}

// retryWithStream 非流式返回空 content 时，用流式重试获取完整响应
func retryWithStream(ctx context.Context, cfg Config, messages []Message, opts CallOptions) (*Response, error) {
	ch, err := callOpenAIStream(ctx, cfg, messages, opts)
	if err != nil {
		return nil, err
	}

	var content strings.Builder
	var toolCalls []ToolCall
	toolCallAcc := make(map[int]*deltaToolCall)

	for chunk := range ch {
		if chunk.Content != "" {
			content.WriteString(chunk.Content)
		}
		if len(chunk.ToolCallDeltas) > 0 {
			for _, dtc := range chunk.ToolCallDeltas {
				existing, ok := toolCallAcc[dtc.Index]
				if !ok {
					toolCallAcc[dtc.Index] = &deltaToolCall{
						Index: dtc.Index,
						ID:    dtc.ID,
						Type:  "function",
					}
					if dtc.Name != "" {
						toolCallAcc[dtc.Index].Function.Name = dtc.Name
					}
					if dtc.Arguments != "" {
						toolCallAcc[dtc.Index].Function.Arguments = dtc.Arguments
					}
				} else {
					if dtc.ID != "" {
						existing.ID = dtc.ID
					}
					if dtc.Name != "" {
						existing.Function.Name += dtc.Name
					}
					if dtc.Arguments != "" {
						existing.Function.Arguments += dtc.Arguments
					}
				}
			}
		}
		if chunk.Done {
			break
		}
	}

	// 组装 tool calls
	for i := 0; i < len(toolCallAcc); i++ {
		if tc, ok := toolCallAcc[i]; ok && tc.Function.Name != "" {
			toolCalls = append(toolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	return &Response{
		Content:   content.String(),
		ToolCalls: toolCalls,
		Model:     cfg.Model,
	}, nil
}

// callOpenAIStream 执行 OpenAI API 流式调用
// 支持文本内容和工具调用的流式解析
func callOpenAIStream(ctx context.Context, cfg Config, messages []Message, opts CallOptions) (<-chan StreamChunk, error) {
	reqBody := openaiChatRequest{
		Model:       cfg.Model,
		Messages:    toOpenAIMessages(messages),
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
		Stream:      true,
	}

	// v0.16.0: 添加 function calling 工具定义
	if len(opts.Tools) > 0 {
		tools := make([]openaiTool, 0, len(opts.Tools))
		for _, t := range opts.Tools {
			fn, _ := t["function"].(map[string]any)
			tools = append(tools, openaiTool{
				Type:     "function",
				Function: newToolFunction(fn),
			})
		}
		reqBody.Tools = tools
		if opts.ToolChoice != nil {
			reqBody.ToolChoice = opts.ToolChoice
		} else {
			reqBody.ToolChoice = "auto"
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.APIBase+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	
	// v0.56.0: 添加额外请求头（如 User-Agent）
	for k, v := range cfg.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 128)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			// SSE 格式: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// 流结束标记
			if data == "[DONE]" {
				ch <- StreamChunk{Done: true, Model: cfg.Model}
				return
			}

			var chatResp openaiChatResponse
			if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
				continue
			}

			if len(chatResp.Choices) == 0 {
				continue
			}

			choice := chatResp.Choices[0]

			// 处理文本内容
			if choice.Delta != nil && choice.Delta.Content != "" {
				ch <- StreamChunk{
					Content: choice.Delta.Content,
					Model:   cfg.Model,
				}
			}

			// 处理工具调用（流式增量）— v0.40.0: 结构化传递
			if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
				deltas := make([]StreamToolCallDelta, 0, len(choice.Delta.ToolCalls))
				for _, dtc := range choice.Delta.ToolCalls {
					deltas = append(deltas, StreamToolCallDelta{
						Index:    dtc.Index,
						ID:       dtc.ID,
						Name:     dtc.Function.Name,
						Arguments: dtc.Function.Arguments,
					})
				}
				ch <- StreamChunk{
					ToolCallDeltas: deltas,
					Model:          cfg.Model,
				}
			}

			if choice.FinishReason == "stop" || choice.FinishReason == "length" {
				ch <- StreamChunk{Done: true, Model: cfg.Model}
				return
			}

			// 工具调用完成
			if choice.FinishReason == "tool_calls" {
				ch <- StreamChunk{Done: true, Model: cfg.Model}
				return
			}
		}
	}()

	return ch, nil
}

// toOpenAIMessages 将通用 Message 转换为 OpenAI 格式
func toOpenAIMessages(messages []Message) []openaiMessage {
	result := make([]openaiMessage, 0, len(messages))
	for _, m := range messages {
		msg := openaiMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		// v0.16.0: 处理 tool_calls（assistant 消息）
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = make([]openaiToolCallResp, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				msg.ToolCalls[i] = openaiToolCallResp{
					ID:   tc.ID,
					Type: "function",
				}
				msg.ToolCalls[i].Function.Name = tc.Name
				msg.ToolCalls[i].Function.Arguments = tc.Arguments
			}
		}
		result = append(result, msg)
	}
	return result
}

// toolFunction 从 map 创建 toolFunction 结构
func newToolFunction(fn map[string]any) toolFunction {
	tf := toolFunction{}
	if name, ok := fn["name"].(string); ok {
		tf.Name = name
	}
	if desc, ok := fn["description"].(string); ok {
		tf.Description = desc
	}
	if params, ok := fn["parameters"].(map[string]any); ok {
		tf.Parameters = params
	}
	return tf
}
