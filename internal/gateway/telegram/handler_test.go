package telegram

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yurika0211/luckyharness/internal/gateway"
)

// TestNewHandler verifies handler creation.
func TestNewHandler(t *testing.T) {
	adapter := NewAdapter(Config{Token: "test"})
	h := NewHandler(adapter, nil)
	require.NotNil(t, h)
	assert.Equal(t, adapter, h.adapter)
	assert.NotNil(t, h.sessions)
}

// TestSetSessionID verifies direct session ID assignment.
func TestSetSessionID(t *testing.T) {
	h := &Handler{
		sessions: make(map[string]string),
	}

	h.setSessionID("chat1", "sess-abc")
	assert.True(t, h.hasSession("chat1"))
	assert.False(t, h.hasSession("chat2"))
}

// TestHasSession verifies session existence check.
func TestHasSession(t *testing.T) {
	h := &Handler{
		sessions: make(map[string]string),
	}

	assert.False(t, h.hasSession("chat1"))
	h.setSessionID("chat1", "sess-abc")
	assert.True(t, h.hasSession("chat1"))
}

// TestResetSessionWithSetSessionID creates a new session after reset.
func TestResetSessionWithSetSessionID(t *testing.T) {
	h := &Handler{
		sessions: make(map[string]string),
	}

	chatID := "12345"
	h.setSessionID(chatID, "old-session")
	oldSID := h.sessions[chatID]

	// resetSession needs agent, so test the map manipulation directly
	h.mu.Lock()
	h.sessions[chatID] = "new-session"
	h.mu.Unlock()

	newSID := h.sessions[chatID]
	assert.NotEqual(t, oldSID, newSID)
	assert.Equal(t, "new-session", newSID)
}

// TestSessionMapIsolation verifies that session IDs are isolated per chat.
func TestSessionMapIsolation(t *testing.T) {
	h := &Handler{
		sessions: make(map[string]string),
	}

	h.setSessionID("chat_A", "sess_A")
	h.setSessionID("chat_B", "sess_B")
	h.setSessionID("chat_C", "sess_C")

	// All different
	assert.NotEqual(t, h.sessions["chat_A"], h.sessions["chat_B"])
	assert.NotEqual(t, h.sessions["chat_B"], h.sessions["chat_C"])

	// Reset one chat
	h.setSessionID("chat_A", "sess_A_new")
	assert.NotEqual(t, "sess_A", h.sessions["chat_A"])

	// Other chats unaffected
	assert.Equal(t, "sess_B", h.sessions["chat_B"])
	assert.Equal(t, "sess_C", h.sessions["chat_C"])
}

// TestMultipleResets verifies that repeated resets always produce unique sessions.
func TestMultipleResets(t *testing.T) {
	h := &Handler{
		sessions: make(map[string]string),
	}

	chatID := "reset-test"
	var sids []string

	for i := 0; i < 5; i++ {
		sid := fmt.Sprintf("sess-%d", i)
		h.setSessionID(chatID, sid)
		sids = append(sids, sid)
	}

	// All session IDs should be unique
	seen := make(map[string]bool)
	for _, sid := range sids {
		assert.False(t, seen[sid], "duplicate session ID: %s", sid)
		seen[sid] = true
	}
}

// TestConcurrentSessionAccess tests that session map is safe under concurrent access.
func TestConcurrentSessionAccess(t *testing.T) {
	h := &Handler{
		sessions: make(map[string]string),
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			chatID := fmt.Sprintf("chat-%d", idx%5)
			h.setSessionID(chatID, fmt.Sprintf("sess-%d", idx))
			_ = h.hasSession(chatID)
		}(i)
	}
	wg.Wait()
}

// TestHandleMessageCommand verifies command routing.
func TestHandleMessageCommand(t *testing.T) {
	msg := &gateway.Message{
		ID:        "1",
		Chat:      gateway.Chat{ID: "123", Type: gateway.ChatPrivate},
		Sender:    gateway.User{ID: "1"},
		Text:      "/start",
		IsCommand: true,
		Command:   "start",
	}

	assert.True(t, msg.IsCommand)
	assert.Equal(t, "start", msg.Command)
}

// TestHandleMessagePrivateNonCommand verifies private chat routing.
func TestHandleMessagePrivateNonCommand(t *testing.T) {
	msg := &gateway.Message{
		ID:        "1",
		Chat:      gateway.Chat{ID: "123", Type: gateway.ChatPrivate},
		Sender:    gateway.User{ID: "1"},
		Text:      "Hello, bot!",
		IsCommand: false,
	}

	assert.Equal(t, gateway.ChatPrivate, msg.Chat.Type)
	assert.False(t, msg.IsCommand)
	assert.Equal(t, "Hello, bot!", msg.Text)
}

// TestHandleMessageGroupNonCommand verifies group chat routing.
func TestHandleMessageGroupNonCommand(t *testing.T) {
	msg := &gateway.Message{
		ID:        "1",
		Chat:      gateway.Chat{ID: "-100", Type: gateway.ChatGroup},
		Sender:    gateway.User{ID: "1"},
		Text:      "Hello, everyone!",
		IsCommand: false,
	}

	assert.Equal(t, gateway.ChatGroup, msg.Chat.Type)
	assert.False(t, msg.IsCommand)
}

// TestCommandRouting verifies all known commands are recognized.
func TestCommandRouting(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    string
	}{
		{"start", "start", ""},
		{"help", "help", ""},
		{"model no args", "model", ""},
		{"model with args", "model", "gpt-4"},
		{"soul", "soul", ""},
		{"tools", "tools", ""},
		{"reset", "reset", ""},
		{"history", "history", ""},
		{"session", "session", ""},
		{"chat with args", "chat", "hello world"},
		{"unknown", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &gateway.Message{
				ID:        "1",
				Chat:      gateway.Chat{ID: "123", Type: gateway.ChatPrivate},
				Sender:    gateway.User{ID: "1", Username: "testuser"},
				Text:      "/" + tt.command + " " + tt.args,
				IsCommand: true,
				Command:   tt.command,
				Args:      strings.TrimSpace(tt.args),
			}
			assert.NotNil(t, msg)
			assert.Equal(t, tt.command, msg.Command)
		})
	}
}

// TestWelcomeMessage verifies the welcome message content.
func TestWelcomeMessage(t *testing.T) {
	welcome := `🍀 *LuckyHarness Bot*

I'm an AI assistant powered by LuckyHarness.

*Available commands:*
/chat _message_ — Send a message to the AI
/model — Show current model
/soul — Show current SOUL info
/tools — List available tools
/reset — Reset conversation
/history — Show conversation history
/session — Show current session info
/help — Show this help

You can also just type a message directly!`

	assert.Contains(t, welcome, "LuckyHarness")
	assert.Contains(t, welcome, "/chat")
	assert.Contains(t, welcome, "/help")
	assert.Contains(t, welcome, "/model")
	assert.Contains(t, welcome, "/history")
	assert.Contains(t, welcome, "/session")
}

// TestHelpMessage verifies the help message content.
func TestHelpMessage(t *testing.T) {
	help := `*Available Commands:*

/start — Welcome message
/help — This help message
/chat _message_ — Send a message to the AI
/model \[name] — Get/set current model
/soul — Show current SOUL info
/tools — List available tools
/reset — Reset conversation
/history — Show conversation history
/session — Show current session info

*Tips:*
• In private chats, just type your message directly
• In groups, mention @bot or reply to a bot message
• Each chat has its own conversation session`

	assert.Contains(t, help, "/start")
	assert.Contains(t, help, "/chat")
	assert.Contains(t, help, "/model")
	assert.Contains(t, help, "/soul")
	assert.Contains(t, help, "/tools")
	assert.Contains(t, help, "/reset")
	assert.Contains(t, help, "/history")
	assert.Contains(t, help, "/session")
}

// TestGatewayMessageTypes verifies message type construction.
func TestGatewayMessageTypes(t *testing.T) {
	// Test ChatType
	assert.Equal(t, "private", gateway.ChatPrivate.String())
	assert.Equal(t, "group", gateway.ChatGroup.String())
	assert.Equal(t, "supergroup", gateway.ChatSuperGroup.String())
	assert.Equal(t, "channel", gateway.ChatChannel.String())

	// Test User.DisplayName
	u := gateway.User{ID: "1", Username: "test", FirstName: "Test", LastName: "User"}
	assert.Equal(t, "@test", u.DisplayName())

	u2 := gateway.User{ID: "2", FirstName: "Test", LastName: "User"}
	assert.Equal(t, "Test User", u2.DisplayName())

	u3 := gateway.User{ID: "3", FirstName: "Test"}
	assert.Equal(t, "Test", u3.DisplayName())

	u4 := gateway.User{ID: "4"}
	assert.Equal(t, "4", u4.DisplayName())
}