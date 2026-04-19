package soul

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	s := Default()
	if s.Content == "" {
		t.Error("default soul should not be empty")
	}
}

func TestSystemPrompt(t *testing.T) {
	s := Default()
	prompt := s.SystemPrompt()
	if prompt == "" {
		t.Error("system prompt should not be empty")
	}
}

func TestLoadAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "SOUL.md")
	content := "# Test Soul\nYou are a test agent."
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Content != content {
		t.Errorf("expected %q, got %q", content, s.Content)
	}

	// Modify and reload
	newContent := "# Updated\nYou are updated."
	os.WriteFile(path, []byte(newContent), 0644)
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if s.Content != newContent {
		t.Errorf("expected %q after reload, got %q", newContent, s.Content)
	}
}
