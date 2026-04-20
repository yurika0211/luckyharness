package websocket

import (
	"context"
	"fmt"
	"sync"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/logger"
)

// AgentHandler 将 WebSocket 消息桥接到 Agent Loop
type AgentHandler struct {
	agent   *agent.Agent
	pending map[string]context.CancelFunc // sessionID → cancel
	mu      sync.Mutex
}

// NewAgentHandler 创建 Agent 消息处理器
func NewAgentHandler(a *agent.Agent) *AgentHandler {
	return &AgentHandler{
		agent:   a,
		pending: make(map[string]context.CancelFunc),
	}
}

// HandleMessage 处理来自 WebSocket 客户端的消息
func (h *AgentHandler) HandleMessage(client *Client, msg *Message) {
	switch msg.Type {
	case TypeChat:
		h.handleChat(client, msg)
	case TypeStreamAck:
		// 流式确认，暂不处理
		logger.Debug("stream ack received", "client_id", client.ID, "msg_id", msg.ID)
	default:
		logger.Warn("unknown message type", "type", msg.Type, "client_id", client.ID)
	}
}

// handleChat 处理聊天消息
func (h *AgentHandler) handleChat(client *Client, msg *Message) {
	var data ChatData
	if err := msg.ParseData(&data); err != nil {
		errMsg, _ := NewMessage(TypeError, client.SessionID, ErrorData{
			Code:    "INVALID_DATA",
			Message: fmt.Sprintf("invalid chat data: %v", err),
		})
		client.Send <- errMsg
		return
	}

	// 发送 thinking 状态
	status, _ := NewMessage(TypeStatus, client.SessionID, StatusData{
		State:   "thinking",
		Message: "processing your message",
	})
	client.Send <- status

	// 取消该 session 之前的请求
	h.mu.Lock()
	if cancel, ok := h.pending[client.SessionID]; ok {
		cancel()
		delete(h.pending, client.SessionID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.pending[client.SessionID] = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.pending, client.SessionID)
			h.mu.Unlock()
		}()

		if data.Stream {
			h.streamChat(ctx, client, data, msg.ID)
		} else {
			h.syncChat(ctx, client, data, msg.ID)
		}
	}()
}

// syncChat 同步聊天（等待完整响应）
func (h *AgentHandler) syncChat(ctx context.Context, client *Client, data ChatData, parentID string) {
	// 发送 executing 状态
	status, _ := NewMessage(TypeStatus, client.SessionID, StatusData{
		State:   "executing",
		Message: "agent is running",
	})
	client.Send <- status

	// 调用 Agent
	result, err := h.agent.Chat(ctx, data.Message)
	if err != nil {
		errMsg, _ := NewMessage(TypeError, client.SessionID, ErrorData{
			Code:    "AGENT_ERROR",
			Message: err.Error(),
		})
		errMsg.ParentID = parentID
		client.Send <- errMsg
		return
	}

	// 发送完整响应
	endMsg, _ := NewMessage(TypeStreamEnd, client.SessionID, StreamEndData{
		FullResponse: result,
		Iterations:   1,
	})
	endMsg.ParentID = parentID
	client.Send <- endMsg

	// 发送 idle 状态
	idle, _ := NewMessage(TypeStatus, client.SessionID, StatusData{
		State: "idle",
	})
	client.Send <- idle
}

// streamChat 流式聊天（逐块推送）
func (h *AgentHandler) streamChat(ctx context.Context, client *Client, data ChatData, parentID string) {
	// 发送 executing 状态
	status, _ := NewMessage(TypeStatus, client.SessionID, StatusData{
		State:   "executing",
		Message: "agent is streaming",
	})
	client.Send <- status

	// 使用 Agent 的流式接口
	streamCh, err := h.agent.ChatStream(ctx, data.Message)
	if err != nil {
		errMsg, _ := NewMessage(TypeError, client.SessionID, ErrorData{
			Code:    "AGENT_ERROR",
			Message: err.Error(),
		})
		errMsg.ParentID = parentID
		client.Send <- errMsg
		return
	}

	var fullResponse string
	for chunk := range streamCh {
		fullResponse += chunk.Content

		chunkMsg, _ := NewMessage(TypeStreamChunk, client.SessionID, StreamChunkData{
			Content: chunk.Content,
			Done:    chunk.Done,
		})
		chunkMsg.ParentID = parentID
		client.Send <- chunkMsg
	}

	// 发送流式结束
	endMsg, _ := NewMessage(TypeStreamEnd, client.SessionID, StreamEndData{
		FullResponse: fullResponse,
		Iterations:   1,
	})
	endMsg.ParentID = parentID
	client.Send <- endMsg

	// 发送 idle 状态
	idle, _ := NewMessage(TypeStatus, client.SessionID, StatusData{
		State: "idle",
	})
	client.Send <- idle
}

// CancelSession 取消指定 session 的进行中请求
func (h *AgentHandler) CancelSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cancel, ok := h.pending[sessionID]; ok {
		cancel()
		delete(h.pending, sessionID)
	}
}

// PendingCount 返回进行中的请求数
func (h *AgentHandler) PendingCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.pending)
}