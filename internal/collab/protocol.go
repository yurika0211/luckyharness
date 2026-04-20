package collab

import (
	"encoding/json"
	"fmt"
	"time"
)

// MessageType 协作消息类型
type MessageType string

const (
	MsgTaskAssign    MessageType = "task_assign"    // 分配任务
	MsgTaskResult    MessageType = "task_result"    // 任务结果
	MsgTaskProgress  MessageType = "task_progress"  // 进度更新
	MsgTaskCancel    MessageType = "task_cancel"    // 取消任务
	MsgHeartbeat     MessageType = "heartbeat"      // 心跳
	MsgRegister      MessageType = "register"        // 注册
	MsgDeregister    MessageType = "deregister"      // 注销
	MsgBroadcast     MessageType = "broadcast"       // 广播
	MsgDebateTurn    MessageType = "debate_turn"     // 辩论轮次
	MsgDebateVote    MessageType = "debate_vote"     // 辩论投票
	MsgError         MessageType = "error"           // 错误
)

// Priority 消息优先级
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Message 协作消息
type Message struct {
	ID          string                 `json:"id"`
	Type        MessageType            `json:"type"`
	From        string                 `json:"from"`         // 发送者 Agent ID
	To          string                 `json:"to"`           // 接收者 Agent ID（空=广播）
	Priority    Priority               `json:"priority"`
	Payload     map[string]any         `json:"payload"`
	Timestamp   time.Time              `json:"timestamp"`
	Correlation string                 `json:"correlation"`  // 关联 ID（请求-响应配对）
	TTL         int                    `json:"ttl"`          // 生存时间（跳数），0=无限
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// NewMessage 创建新消息
func NewMessage(msgType MessageType, from, to string, payload map[string]any) *Message {
	return &Message{
		ID:          generateID(),
		Type:        msgType,
		From:        from,
		To:          to,
		Priority:    PriorityNormal,
		Payload:     payload,
		Timestamp:   time.Now(),
		Correlation: "",
		TTL:         0,
		Metadata:    make(map[string]string),
	}
}

// WithPriority 设置优先级
func (m *Message) WithPriority(p Priority) *Message {
	m.Priority = p
	return m
}

// WithCorrelation 设置关联 ID
func (m *Message) WithCorrelation(id string) *Message {
	m.Correlation = id
	return m
}

// WithTTL 设置 TTL
func (m *Message) WithTTL(ttl int) *Message {
	m.TTL = ttl
	return m
}

// WithMetadata 添加元数据
func (m *Message) WithMetadata(key, value string) *Message {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
	return m
}

// Encode 序列化消息为 JSON
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage 从 JSON 反序列化消息
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("decode message: %w", err)
	}
	return &msg, nil
}

// Validate 验证消息有效性
func (m *Message) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("message ID is required")
	}
	if m.Type == "" {
		return fmt.Errorf("message type is required")
	}
	if m.From == "" {
		return fmt.Errorf("sender (from) is required")
	}
	if m.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}
	return nil
}

// generateID 生成唯一消息 ID
func generateID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}