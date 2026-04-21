package gateway

import "context"

// Gateway is the interface that all messaging platform adapters must implement.
type Gateway interface {
	// Name returns the platform name (e.g., "telegram", "discord")
	Name() string

	// Start starts the gateway connection
	Start(ctx context.Context) error

	// Stop gracefully shuts down the gateway
	Stop() error

	// Send sends a message to a chat
	Send(ctx context.Context, chatID string, message string) error

	// SendWithReply sends a message as a reply to a specific message
	SendWithReply(ctx context.Context, chatID string, replyToMsgID string, message string) error

	// IsRunning returns whether the gateway is currently connected
	IsRunning() bool
}

// StreamGateway extends Gateway with streaming message support.
// Adapters that support real-time message editing (like Telegram) should implement this.
type StreamGateway interface {
	Gateway

	// SendStream sends a message and returns a StreamSender for live updates.
	// The caller writes chunks to the sender; the adapter handles rendering.
	SendStream(ctx context.Context, chatID string, replyToMsgID string) (StreamSender, error)
}

// StreamSender allows incremental message updates for streaming output.
type StreamSender interface {
	// Append adds content to the current message and updates the platform.
	Append(content string) error

	// SetThinking shows a "thinking" indicator (e.g., 🧠 Thinking...).
	SetThinking(label string) error

	// SetToolCall shows a tool call indicator (e.g., 🔧 Calling shell...).
	SetToolCall(name, args string) error

	// SetResult replaces the thinking/tool-call indicator with the final result.
	SetResult(content string) error

	// Finish finalizes the message. No more updates allowed.
	Finish() error

	// MessageID returns the platform message ID (for reply chaining).
	MessageID() string
}

// StreamMiddleware 接收 ChatEvent 流，决定如何渲染到平台。
// 不同实现控制思考/工具调用的展示方式。
type StreamMiddleware interface {
	// Process 处理一个事件，返回 true 表示继续，false 表示流结束
	Process(eventType int, event ChatEventData) bool

	// Close 结束中间件，清理资源
	Close()
}

// ChatEventData 是 ChatEvent 的中间件友好格式
type ChatEventData struct {
	Content string // 文本内容
	Name    string // 工具名
	Args    string // 工具参数
	Result  string // 工具结果
	Err     error  // 错误
}