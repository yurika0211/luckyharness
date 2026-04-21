package gateway

import (
	"errors"
	"sync"
	"testing"
)

// mockStreamSender 用于测试
type mockStreamSender struct {
	mu        sync.Mutex
	content   string
	thinking  string
	toolCall  string
	finished  bool
	appends   []string
	thinkSets []string
}

func (m *mockStreamSender) Append(content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.content += content
	m.appends = append(m.appends, content)
	return nil
}

func (m *mockStreamSender) SetThinking(label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.thinking = label
	m.thinkSets = append(m.thinkSets, label)
	return nil
}

func (m *mockStreamSender) SetToolCall(name, args string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolCall = name
	return nil
}

func (m *mockStreamSender) SetResult(content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.content = content
	return nil
}

func (m *mockStreamSender) Finish() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finished = true
	return nil
}

func (m *mockStreamSender) MessageID() string { return "1" }

func TestInlineMiddleware(t *testing.T) {
	sender := &mockStreamSender{}
	mw := NewInlineMiddleware(sender)

	// Thinking
	if !mw.Process(0, ChatEventData{Content: "Thinking..."}) {
		t.Fatal("should continue after thinking")
	}

	// ToolCall
	if !mw.Process(1, ChatEventData{Name: "shell", Args: "ls"}) {
		t.Fatal("should continue after tool call")
	}

	// Content
	if !mw.Process(3, ChatEventData{Content: "Hello "}) {
		t.Fatal("should continue after content")
	}
	if !mw.Process(3, ChatEventData{Content: "World"}) {
		t.Fatal("should continue after content")
	}

	// Done
	if mw.Process(4, ChatEventData{Content: "Hello World"}) {
		t.Fatal("should stop after done")
	}

	if sender.content != "Hello World" {
		t.Fatalf("expected 'Hello World', got %q", sender.content)
	}
	if !sender.finished {
		t.Fatal("should be finished")
	}
}

func TestInlineMiddlewareError(t *testing.T) {
	sender := &mockStreamSender{}
	mw := NewInlineMiddleware(sender)

	if mw.Process(5, ChatEventData{Err: errors.New("test error")}) {
		t.Fatal("should stop after error")
	}

	if !sender.finished {
		t.Fatal("should be finished after error")
	}
}

func TestQuietMiddleware(t *testing.T) {
	sender := &mockStreamSender{}
	mw := NewQuietMiddleware(sender)

	// Thinking — should be ignored
	if !mw.Process(0, ChatEventData{Content: "Thinking..."}) {
		t.Fatal("should continue after thinking")
	}

	// ToolCall — should be ignored
	if !mw.Process(1, ChatEventData{Name: "shell", Args: "ls"}) {
		t.Fatal("should continue after tool call")
	}

	// ToolResult — should be ignored
	if !mw.Process(2, ChatEventData{Name: "shell", Result: "output"}) {
		t.Fatal("should continue after tool result")
	}

	// Content — should be appended
	if !mw.Process(3, ChatEventData{Content: "Hello"}) {
		t.Fatal("should continue after content")
	}

	// Done
	if mw.Process(4, ChatEventData{Content: "Hello"}) {
		t.Fatal("should stop after done")
	}

	if sender.content != "Hello" {
		t.Fatalf("expected 'Hello', got %q", sender.content)
	}

	// Thinking/tool calls should not have been set
	if len(sender.thinkSets) != 0 {
		t.Fatalf("quiet mode should not set thinking, got %v", sender.thinkSets)
	}
}

func TestSeparateMiddleware(t *testing.T) {
	sender := &mockStreamSender{}
	gw := &mockGateway{name: "test"}
	mw := NewSeparateMiddleware(sender, gw, "123")

	// Thinking — should send separate message
	if !mw.Process(0, ChatEventData{Content: "Thinking..."}) {
		t.Fatal("should continue after thinking")
	}

	// ToolCall — should send separate message
	if !mw.Process(1, ChatEventData{Name: "shell", Args: "ls"}) {
		t.Fatal("should continue after tool call")
	}

	// Content — should append to main message
	if !mw.Process(3, ChatEventData{Content: "Hello"}) {
		t.Fatal("should continue after content")
	}

	// Done
	if mw.Process(4, ChatEventData{Content: "Hello"}) {
		t.Fatal("should stop after done")
	}

	// Check separate messages were sent
	if len(gw.sentMsgs) < 2 {
		t.Fatalf("expected at least 2 separate messages, got %d: %v", len(gw.sentMsgs), gw.sentMsgs)
	}

	// Main message should have content
	if sender.content != "Hello" {
		t.Fatalf("expected 'Hello', got %q", sender.content)
	}
}