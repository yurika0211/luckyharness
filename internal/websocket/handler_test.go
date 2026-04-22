package websocket

import (
	"encoding/json"
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
