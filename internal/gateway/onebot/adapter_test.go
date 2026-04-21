package onebot

import (
	"testing"
)

func TestNewAdapter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	cfg.ShowTyping = true
	cfg.AutoLike = true
	cfg.LikeTimes = 3

	adapter := NewAdapter(cfg)

	if adapter.Name() != "onebot" {
		t.Errorf("expected name 'onebot', got %s", adapter.Name())
	}
	if !adapter.cfg.ShowTyping {
		t.Error("expected ShowTyping=true")
	}
	if !adapter.cfg.AutoLike {
		t.Error("expected AutoLike=true")
	}
	if adapter.cfg.LikeTimes != 3 {
		t.Errorf("expected LikeTimes=3, got %d", adapter.cfg.LikeTimes)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxMessageLen != 4000 {
		t.Errorf("expected MaxMessageLen=4000, got %d", cfg.MaxMessageLen)
	}
	if !cfg.ShowTyping {
		t.Error("expected ShowTyping=true by default")
	}
	if !cfg.AutoLike {
		t.Error("expected AutoLike=true by default")
	}
}

func TestSplitMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	// Short message should not be split
	short := "Hello, world!"
	chunks := adapter.splitMessage(short)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}

	// Long message should be split
	longMsg := ""
	for i := 0; i < 5000; i++ {
		longMsg += "x"
	}
	chunks = adapter.splitMessage(longMsg)
	if len(chunks) < 2 {
		t.Errorf("expected >= 2 chunks for long message, got %d", len(chunks))
	}
}

func TestParseGroupID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	// Numeric group ID
	id, isGroup := adapter.parseGroupID("123456789")
	if !isGroup {
		t.Error("expected group ID to be recognized")
	}
	if id != 123456789 {
		t.Errorf("expected 123456789, got %d", id)
	}

	// Non-numeric ID (like the hex hash)
	_, isGroup = adapter.parseGroupID("CE3B0091A010C0BF3EC6EF5A42443373")
	if isGroup {
		t.Error("expected hex hash to not be recognized as group ID")
	}
}

func TestLikeTimesClamp(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"

	// Too many likes should be clamped to 10
	cfg.LikeTimes = 20
	adapter := NewAdapter(cfg)
	if adapter.cfg.LikeTimes != 10 {
		t.Errorf("expected LikeTimes clamped to 10, got %d", adapter.cfg.LikeTimes)
	}

	// Zero likes should be set to 1
	cfg.LikeTimes = 0
	adapter = NewAdapter(cfg)
	if adapter.cfg.LikeTimes != 1 {
		t.Errorf("expected LikeTimes default to 1, got %d", adapter.cfg.LikeTimes)
	}
}

func TestAdapterNotRunning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIBase = "http://127.0.0.1:3000"
	adapter := NewAdapter(cfg)

	// Send should fail when not running
	err := adapter.Send(nil, "123", "test")
	if err == nil {
		t.Error("expected error when adapter not running")
	}
}