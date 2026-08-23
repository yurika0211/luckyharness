package weixin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yurika0211/luckyagent/internal/gateway"
)

func TestPostRejectsILinkBusinessErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ret":1,"errcode":401,"errmsg":"token secret-token is invalid"}`))
	}))
	defer server.Close()

	adapter := NewAdapter(Config{BaseURL: server.URL, Token: "secret-token"})
	err := adapter.post(context.Background(), epSendMsg, map[string]string{"text": "hello"}, nil)
	if err == nil {
		t.Fatal("expected business error")
	}
	if !strings.Contains(err.Error(), "business error") {
		t.Fatalf("expected iLink business error, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("business error leaked token: %v", err)
	}
}

func TestHandleRecordStoresGroupContextTokenByChatID(t *testing.T) {
	adapter := NewAdapter(Config{AccountID: "bot", GroupPolicy: "open"})
	var handled string
	adapter.SetHandler(func(_ context.Context, message *gateway.Message) error {
		handled = message.Chat.ID
		return nil
	})

	adapter.handleRecord(context.Background(), incomingRecord{
		MessageID:    "message-1",
		FromUserID:   "user-1",
		RoomID:       "room-1",
		ContextToken: "group-context",
		ItemList:     []incomingItem{{Type: 1, TextItem: textIn{Content: "hello"}}},
	})

	if handled != "room-1" {
		t.Fatalf("expected handler to receive group chat, got %q", handled)
	}
	adapter.mu.RLock()
	token := adapter.contextToken["room-1"]
	adapter.mu.RUnlock()
	if token != "group-context" {
		t.Fatalf("expected group context token, got %q", token)
	}
}

func TestHandleRecordRecordsHandlerFailure(t *testing.T) {
	adapter := NewAdapter(Config{AccountID: "bot", DMPolicy: "open", Token: "secret-token"})
	adapter.SetHandler(func(context.Context, *gateway.Message) error {
		return errors.New("upstream secret-token failed")
	})

	adapter.handleRecord(context.Background(), incomingRecord{
		FromUserID: "user-1",
		ItemList:   []incomingItem{{Type: 1, TextItem: textIn{Content: "hello"}}},
	})

	status := adapter.Diagnostics()
	if status.HandlerFailures != 1 {
		t.Fatalf("expected one handler failure, got %d", status.HandlerFailures)
	}
	if strings.Contains(status.LastError, "secret-token") {
		t.Fatalf("diagnostics leaked token: %q", status.LastError)
	}
}

func TestPollRetryDelayIsBounded(t *testing.T) {
	if got := pollRetryDelay(1); got.String() != "2s" {
		t.Fatalf("unexpected first retry delay: %s", got)
	}
	if got := pollRetryDelay(8); got.String() != "30s" {
		t.Fatalf("expected capped retry delay, got %s", got)
	}
}
