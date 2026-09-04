package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestNewLongConnectionEventMapsSDKMessage(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{EventID: "evt_123", AppID: "cli_app"}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: stringPointer("ou_sender"), UserId: stringPointer("u_sender")},
				SenderType: stringPointer("user"),
			},
			Message: &larkim.EventMessage{
				MessageId:   stringPointer("om_123"),
				CreateTime:  stringPointer("1710000000123"),
				ChatId:      stringPointer("oc_123"),
				ChatType:    stringPointer("group"),
				MessageType: stringPointer("text"),
				Content:     stringPointer(`{"text":"@_user_1 hello"}`),
				Mentions: []*larkim.MentionEvent{{
					Key:  stringPointer("@_user_1"),
					Name: stringPointer("LuckyAgent"),
					Id:   &larkim.UserId{OpenId: stringPointer("ou_bot")},
				}},
			},
		},
	}

	got, ok := newLongConnectionEvent(event)
	if !ok {
		t.Fatal("newLongConnectionEvent() returned ok=false")
	}
	if got.ID != "evt_123" || got.AppID != "cli_app" {
		t.Fatalf("unexpected event header: %#v", got)
	}
	if got.Message.Sender.SenderID.OpenID != "ou_sender" || got.Message.Sender.SenderID.UserID != "u_sender" || got.Message.Message.MessageID != "om_123" {
		t.Fatalf("unexpected mapped message: %#v", got.Message)
	}
	if len(got.Message.Message.Mentions) != 1 || got.Message.Message.Mentions[0].ID.OpenID != "ou_bot" {
		t.Fatalf("unexpected mapped mentions: %#v", got.Message.Message.Mentions)
	}
}

func TestSDKLongConnectionUsesDefaultHTTPClientWhenNoneIsConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback/ws/endpoint" {
			t.Fatalf("bootstrap path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 403, "msg": "invalid app credentials"})
	}))
	defer server.Close()

	client := newSDKLongConnection(Config{
		AppID:      "cli_test",
		AppSecret:  "secret",
		APIBaseURL: server.URL,
	}, func(context.Context, longConnectionEvent) error {
		return nil
	})
	if err := client.Start(context.Background()); err == nil {
		t.Fatal("expected bootstrap authentication error")
	}
}

func stringPointer(value string) *string { return &value }
