package gateway

import (
	"context"
	"fmt"
	"sync"
)

// --- InlineMiddleware ---
// 思考/工具调用标签内嵌在消息中（当前默认行为）

// InlineMiddleware 将思考/工具调用标签内嵌在流式消息中
type InlineMiddleware struct {
	sender StreamSender
	mu     sync.Mutex
}

// NewInlineMiddleware 创建内嵌模式中间件
func NewInlineMiddleware(sender StreamSender) *InlineMiddleware {
	return &InlineMiddleware{sender: sender}
}

func (m *InlineMiddleware) Process(eventType int, data ChatEventData) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch eventType {
	case 0: // Thinking
		m.sender.SetThinking(data.Content)
	case 1: // ToolCall
		m.sender.SetToolCall(data.Name, data.Args)
	case 2: // ToolResult
		m.sender.SetThinking(fmt.Sprintf("📋 %s → %s", data.Name, data.Result))
	case 3: // Content
		m.sender.Append(data.Content)
	case 4: // Done
		m.sender.SetResult(data.Content)
		m.sender.Finish()
		return false
	case 5: // Error
		errMsg := data.Err.Error()
		if len(errMsg) > 200 {
			errMsg = errMsg[:197] + "..."
		}
		m.sender.SetResult(fmt.Sprintf("❌ Error: %s", errMsg))
		m.sender.Finish()
		return false
	}
	return true
}

func (m *InlineMiddleware) Close() {
	// nothing
}

// --- SeparateMiddleware ---
// 思考/工具调用单独发消息，正文流式输出

// SeparateMiddleware 思考/工具调用单独发消息，正文流式输出到主消息
type SeparateMiddleware struct {
	sender  StreamSender
	adapter Gateway // 用于发送独立消息
	chatID  string
	mu      sync.Mutex
}

// NewSeparateMiddleware 创建分离模式中间件
func NewSeparateMiddleware(sender StreamSender, adapter Gateway, chatID string) *SeparateMiddleware {
	return &SeparateMiddleware{
		sender:  sender,
		adapter: adapter,
		chatID:  chatID,
	}
}

func (m *SeparateMiddleware) Process(eventType int, data ChatEventData) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch eventType {
	case 0: // Thinking — 单独发一条消息
		if data.Content != "" {
			m.adapter.Send(context.Background(), m.chatID, fmt.Sprintf("🧠 %s", data.Content))
		}
	case 1: // ToolCall — 单独发一条消息
		m.adapter.Send(context.Background(), m.chatID, fmt.Sprintf("🔧 %s", data.Name))
	case 2: // ToolResult — 单独发一条消息
		shortResult := data.Result
		if len(shortResult) > 200 {
			shortResult = shortResult[:197] + "..."
		}
		m.adapter.Send(context.Background(), m.chatID, fmt.Sprintf("📋 %s → %s", data.Name, shortResult))
	case 3: // Content — 流式追加到主消息
		m.sender.Append(data.Content)
	case 4: // Done
		m.sender.SetResult(data.Content)
		m.sender.Finish()
		return false
	case 5: // Error
		errMsg := data.Err.Error()
		if len(errMsg) > 200 {
			errMsg = errMsg[:197] + "..."
		}
		m.sender.SetResult(fmt.Sprintf("❌ Error: %s", errMsg))
		m.sender.Finish()
		return false
	}
	return true
}

func (m *SeparateMiddleware) Close() {
	// nothing
}

// --- QuietMiddleware ---
// 安静模式：不显示思考/工具调用，只流式输出正文

// QuietMiddleware 安静模式，只输出正文内容
type QuietMiddleware struct {
	sender StreamSender
	mu     sync.Mutex
}

// NewQuietMiddleware 创建安静模式中间件
func NewQuietMiddleware(sender StreamSender) *QuietMiddleware {
	return &QuietMiddleware{sender: sender}
}

func (m *QuietMiddleware) Process(eventType int, data ChatEventData) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch eventType {
	case 0: // Thinking — 忽略
	case 1: // ToolCall — 忽略
	case 2: // ToolResult — 忽略
	case 3: // Content — 流式追加
		m.sender.Append(data.Content)
	case 4: // Done
		m.sender.SetResult(data.Content)
		m.sender.Finish()
		return false
	case 5: // Error
		errMsg := data.Err.Error()
		if len(errMsg) > 200 {
			errMsg = errMsg[:197] + "..."
		}
		m.sender.SetResult(fmt.Sprintf("❌ Error: %s", errMsg))
		m.sender.Finish()
		return false
	}
	return true
}

func (m *QuietMiddleware) Close() {
	// nothing
}