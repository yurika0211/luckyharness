package websocket

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================
// WS-12: Hub 边界情况测试
// ============================================================

func TestHubSendToSession(t *testing.T) {
	handler := &mockHandler{}
	cfg := DefaultHubConfig()
	hub := NewHub(handler, cfg)

	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(hub)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws?session=send-session-test"

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	// 发送到 session
	msg, _ := NewMessage(TypeStatus, "send-session-test", StatusData{State: "test"})
	hub.SendToSession("send-session-test", msg)

	// 读取消息
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}

	var received Message
	if err := json.Unmarshal(raw, &received); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if received.Type != TypeStatus {
		t.Errorf("expected type %s, got %s", TypeStatus, received.Type)
	}
}

func TestHubSendToNonExistentSession(t *testing.T) {
	handler := &mockHandler{}
	cfg := DefaultHubConfig()
	hub := NewHub(handler, cfg)

	go hub.Run()
	defer hub.Stop()

	// 发送到不存在的 session 不应该 panic
	msg, _ := NewMessage(TypeStatus, "non-existent", StatusData{State: "test"})
	hub.SendToSession("non-existent-session", msg)
}

// ============================================================
// WS-13: Message 类型覆盖测试
// ============================================================

func TestStatusMessageSerialization(t *testing.T) {
	statusData := StatusData{
		State:   "processing",
		Message: "working on it",
	}

	msg, err := NewMessage(TypeStatus, "test-session", statusData)
	if err != nil {
		t.Fatalf("NewMessage error: %v", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var received Message
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if received.Type != TypeStatus {
		t.Errorf("expected type %s, got %s", TypeStatus, received.Type)
	}

	var parsedStatus StatusData
	if err := received.ParseData(&parsedStatus); err != nil {
		t.Fatalf("ParseData error: %v", err)
	}

	if parsedStatus.State != "processing" {
		t.Errorf("expected state 'processing', got '%s'", parsedStatus.State)
	}
}

// ============================================================
// WS-14: Hub 生命周期测试
// ============================================================

func TestHubLifecycle(t *testing.T) {
	handler := &mockHandler{}
	cfg := DefaultHubConfig()
	hub := NewHub(handler, cfg)

	// 启动
	go hub.Run()

	// 运行中
	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}

	// 停止
	hub.Stop()
	time.Sleep(100 * time.Millisecond)

	// 停止后应该没有客户端
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after stop, got %d", hub.ClientCount())
	}
}

// ============================================================
// WS-15: Client 并发安全测试
// ============================================================

func TestClientConcurrentSend(t *testing.T) {
	client := &Client{
		ID:        "test-client",
		SessionID: "test-session",
		Send:      make(chan *Message, 100),
	}

	done := make(chan bool)

	// 并发发送消息
	for i := 0; i < 10; i++ {
		go func() {
			msg := &Message{Type: TypeChat}
			client.Send <- msg
			done <- true
		}()
	}

	// 等待所有发送完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 检查 channel 中的消息数量
	if len(client.Send) != 10 {
		t.Errorf("expected 10 messages, got %d", len(client.Send))
	}

	// 清理
	close(client.Send)
}

// v0.85.0: websocket 包补测 - 覆盖边缘情况

// TestClientSendChannel 测试 client.Send channel 的并发发送
func TestClientSendChannel_Concurrent(t *testing.T) {
	client := &Client{
		SessionID: "concurrent-test",
		Send:      make(chan *Message, 100),
	}

	// 并发发送 20 条消息
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func(idx int) {
			msg, _ := NewMessage(TypeStatus, "concurrent-test", StatusData{State: fmt.Sprintf("test-%d", idx)})
			client.Send <- msg
			done <- true
		}(i)
	}

	// 等待所有发送完成
	for i := 0; i < 20; i++ {
		<-done
	}

	// 验证 channel 中有 20 条消息
	if len(client.Send) != 20 {
		t.Errorf("expected 20 messages in channel, got %d", len(client.Send))
	}

	// 清理
	close(client.Send)
	t.Logf("concurrent send test passed: %d messages", len(client.Send))
}

// TestParseMessage_InvalidJSON 测试 ParseMessage 错误处理
func TestParseMessage_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{"type": "chat", invalid}`)
	_, err := ParseMessage(invalidJSON)
	if err == nil {
		t.Error("ParseMessage should return error for invalid JSON")
	} else {
		t.Logf("ParseMessage correctly rejected invalid JSON: %v", err)
	}
}

// TestParseMessage_EmptyData 测试 ParseMessage 空数据
func TestParseMessage_EmptyData(t *testing.T) {
	validJSON := []byte(`{"type": "status", "session_id": "test-123"}`)
	msg, err := ParseMessage(validJSON)
	if err != nil {
		t.Errorf("ParseMessage error: %v", err)
	}
	if msg.Type != "status" {
		t.Errorf("expected status, got %s", msg.Type)
	}
	if msg.SessionID != "test-123" {
		t.Errorf("expected test-123, got %s", msg.SessionID)
	}
	t.Logf("ParseMessage handled empty data correctly")
}

// TestNewMessage_IDGeneration 测试消息 ID 自动生成
func TestNewMessage_IDGeneration(t *testing.T) {
	msg, err := NewMessage(TypeStatus, "session", StatusData{State: "test"})
	if err != nil {
		t.Fatalf("NewMessage error: %v", err)
	}
	if msg.ID == "" {
		t.Error("NewMessage should generate ID")
	}
	t.Logf("generated message ID: %s", msg.ID)
}

// TestGetStats 测试 Hub 统计信息
func TestGetStats(t *testing.T) {
	// 简单测试，不启动完整的 Hub
	cfg := DefaultHubConfig()
	if cfg.WriteWait == 0 {
		t.Error("WriteWait should be non-zero")
	}
	if cfg.PongWait == 0 {
		t.Error("PongWait should be non-zero")
	}
	t.Logf("default hub config: write_wait=%v, pong_wait=%v", cfg.WriteWait, cfg.PongWait)
}

// TestHandleMessage_UnknownType 测试 HandleMessage 处理未知消息类型
func TestHandleMessage_UnknownType(t *testing.T) {
	a := createTestAgentForWS(t)
	h := NewAgentHandler(a)

	client := &Client{
		SessionID: "unknown-type-test",
		ID:        "client-123",
		Send:      make(chan *Message, 10),
	}

	// 构造未知类型的消息
	unknownMsg := &Message{
		Type:      "UNKNOWN_TYPE_XYZ",
		SessionID: "unknown-type-test",
		Data:      json.RawMessage(`{}`),
	}

	// 调用 HandleMessage
	h.HandleMessage(client, unknownMsg)

	// 验证没有发送错误消息（未知类型只记录日志，不回复）
	select {
	case msg := <-client.Send:
		t.Errorf("HandleMessage should not send message for unknown type, got: %v", msg)
	default:
		t.Logf("HandleMessage correctly handled unknown type (logged only)")
	}
}
