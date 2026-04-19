package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
}

// openaiMessage 是 OpenAI API 的消息格式
type openaiMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
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
func callOpenAI(cfg Config, messages []Message) (*Response, error) {
	reqBody := openaiChatRequest{
		Model:       cfg.Model,
		Messages:    toOpenAIMessages(messages),
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
		Stream:      false,
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

	return result, nil
}

// callOpenAIStream 执行 OpenAI API 流式调用
func callOpenAIStream(ctx context.Context, cfg Config, messages []Message) (<-chan StreamChunk, error) {
	reqBody := openaiChatRequest{
		Model:       cfg.Model,
		Messages:    toOpenAIMessages(messages),
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
		Stream:      true,
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 64)

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
			if choice.Delta != nil && choice.Delta.Content != "" {
				ch <- StreamChunk{
					Content: choice.Delta.Content,
					Model:   cfg.Model,
				}
			}

			if choice.FinishReason == "stop" || choice.FinishReason == "length" {
				ch <- StreamChunk{Done: true, Model: cfg.Model}
				return
			}
		}
	}()

	return ch, nil
}

// toOpenAIMessages 将通用 Message 转换为 OpenAI 格式
func toOpenAIMessages(messages []Message) []openaiMessage {
	result := make([]openaiMessage, len(messages))
	for i, m := range messages {
		result[i] = openaiMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}
