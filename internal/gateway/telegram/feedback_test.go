package telegram

import (
	"errors"
	"strings"
	"testing"

	"github.com/yurika0211/luckyagent/internal/gateway"
)

func TestTelegramAutoReactionPolicy(t *testing.T) {
	group := &gateway.Message{ID: "1", Chat: gateway.Chat{ID: "-100", Type: gateway.ChatSuperGroup}}
	if emoji, ok := telegramAutoReaction(group, false); !ok || emoji != "👍" {
		t.Fatalf("group reaction = %q, %t", emoji, ok)
	}

	group.ReplyTo = &gateway.Message{ID: "previous"}
	if emoji, ok := telegramAutoReaction(group, false); !ok || emoji != "👀" {
		t.Fatalf("reply reaction = %q, %t", emoji, ok)
	}

	private := &gateway.Message{ID: "2", Chat: gateway.Chat{ID: "user", Type: gateway.ChatPrivate}}
	if emoji, ok := telegramAutoReaction(private, false); !ok || emoji != "👍" {
		t.Fatalf("private reaction = %q, %t", emoji, ok)
	}
	private.ReplyTo = &gateway.Message{ID: "previous"}
	if emoji, ok := telegramAutoReaction(private, false); !ok || emoji != "👍" {
		t.Fatalf("private reply reaction = %q, %t", emoji, ok)
	}
	channel := &gateway.Message{ID: "3", Chat: gateway.Chat{ID: "-100200", Type: gateway.ChatChannel}}
	if emoji, ok := telegramAutoReaction(channel, false); !ok || emoji != "👍" {
		t.Fatalf("channel reaction = %q, %t", emoji, ok)
	}
	unknown := &gateway.Message{ID: "4", Chat: gateway.Chat{Type: gateway.ChatType(99)}}
	if _, ok := telegramAutoReaction(unknown, false); ok {
		t.Fatal("unknown chat types must not be auto-reacted")
	}
	if _, ok := telegramAutoReaction(group, true); ok {
		t.Fatal("disabled auto reaction must not react")
	}
}

func TestTelegramChatErrorFeedback(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{errors.New("billing_error: invalid subscription and insufficient balance"), "余额不足"},
		{errors.New("invalid api key"), "认证失败"},
		{errors.New("HTTP 429 rate limit"), "请求过于频繁"},
		{errors.New("protocol_not_supported: model does not support chat completions"), "Responses API"},
		{errors.New("context deadline exceeded"), "msg_gateway.telegram.chat_timeout_seconds"},
	}
	for _, test := range tests {
		if got := telegramChatErrorFeedback(test.err); !strings.Contains(got, test.want) {
			t.Fatalf("telegramChatErrorFeedback(%v) = %q, want %q", test.err, got, test.want)
		}
	}
}
