package session

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := NewSession("test-1", t.TempDir())
	if s.ID != "test-1" {
		t.Errorf("expected test-1, got %s", s.ID)
	}
	if len(s.Messages) != 0 {
		t.Errorf("expected empty messages, got %d", len(s.Messages))
	}
}

func TestAddMessage(t *testing.T) {
	s := NewSession("test-2", t.TempDir())
	s.AddMessage("user", "hello")
	s.AddMessage("assistant", "hi there")

	msgs := s.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestLastMessage(t *testing.T) {
	s := NewSession("test-3", t.TempDir())
	if last := s.LastMessage(); last != nil {
		t.Error("expected nil for empty session")
	}

	s.AddMessage("user", "first")
	s.AddMessage("assistant", "second")

	last := s.LastMessage()
	if last.Content != "second" {
		t.Errorf("expected 'second', got %s", last.Content)
	}
}

func TestSessionSave(t *testing.T) {
	dir := t.TempDir()
	s := NewSession("test-save", dir)
	s.AddMessage("user", "test message")

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestManagerNew(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s := m.New()
	if s.ID == "" {
		t.Error("expected non-empty session ID")
	}

	// Should be retrievable
	s2, ok := m.Get(s.ID)
	if !ok {
		t.Error("session not found after creation")
	}
	if s2.ID != s.ID {
		t.Errorf("expected %s, got %s", s.ID, s2.ID)
	}
}

func TestManagerList(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	m.New()
	m.New()
	m.New()

	sessions := m.List()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}
