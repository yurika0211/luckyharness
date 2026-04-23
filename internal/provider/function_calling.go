package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// GenerateCallID 生成唯一的 call_id，用于工具调用
// 格式："call_" + 16 字符随机字符串
func GenerateCallID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下随机数生成失败，使用时间戳
		return "call_" + hex.EncodeToString([]byte("fallback"))
	}
	return "call_" + hex.EncodeToString(b)
}

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

// DefaultCallOptions 返回默认调用选项（使用配置默认值）
func DefaultCallOptions(cfg Config) CallOptions {
	maxToolCalls := cfg.Limits.MaxToolCalls
	if maxToolCalls <= 0 {
		maxToolCalls = 5 // 默认值
	}
	return CallOptions{
		ToolChoice:   "auto",
		MaxToolCalls: maxToolCalls,
	}
}
