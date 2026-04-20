package provider

import (
	"context"
)

// CallOptions 是 API 调用的额外选项（用于 function calling）
type CallOptions struct {
	Tools       []map[string]any // OpenAI function calling 工具定义
	ToolChoice  any              // "auto" | "none" | {"type":"function","function":{"name":"xxx"}}
	MaxToolCalls int             // 单次响应最大工具调用数（0 = 不限制）
}

// ChatWithOptions 发送消息并获取完整响应（支持 function calling）
type ChatWithOptionsFunc func(ctx context.Context, messages []Message, opts CallOptions) (*Response, error)

// ChatStreamWithOptions 发送消息并获取流式响应（支持 function calling）
type ChatStreamWithOptionsFunc func(ctx context.Context, messages []Message, opts CallOptions) (<-chan StreamChunk, error)

// FunctionCallingProvider 扩展 Provider 接口，支持 function calling
type FunctionCallingProvider interface {
	Provider
	// ChatWithOptions 发送消息（支持 function calling 选项）
	ChatWithOptions(ctx context.Context, messages []Message, opts CallOptions) (*Response, error)
	// ChatStreamWithOptions 发送消息流式（支持 function calling 选项）
	ChatStreamWithOptions(ctx context.Context, messages []Message, opts CallOptions) (<-chan StreamChunk, error)
}

// DefaultCallOptions 返回默认调用选项
func DefaultCallOptions() CallOptions {
	return CallOptions{
		ToolChoice:   "auto",
		MaxToolCalls: 5,
	}
}
