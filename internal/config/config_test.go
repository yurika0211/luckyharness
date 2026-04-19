package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Provider != "openai" {
		t.Errorf("expected provider openai, got %s", cfg.Provider)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", cfg.Model)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", cfg.MaxTokens)
	}
}

func TestManagerSetAndGet(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	mgr.Set("provider", "anthropic")
	mgr.Set("model", "claude-3")
	mgr.Set("max_tokens", "8192")
	mgr.Set("temperature", "0.5")

	cfg := mgr.Get()
	if cfg.Provider != "anthropic" {
		t.Errorf("expected anthropic, got %s", cfg.Provider)
	}
	if cfg.Model != "claude-3" {
		t.Errorf("expected claude-3, got %s", cfg.Model)
	}
	if cfg.MaxTokens != 8192 {
		t.Errorf("expected 8192, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.5 {
		t.Errorf("expected 0.5, got %f", cfg.Temperature)
	}
}

func TestManagerSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, ".luckyharness")

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// Override paths for test
	mgr.homeDir = homeDir
	mgr.cfgPath = filepath.Join(homeDir, "config.yaml")

	mgr.Set("provider", "ollama")
	mgr.Set("model", "llama3")

	if err := mgr.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load into new manager
	mgr2, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager2: %v", err)
	}
	mgr2.homeDir = homeDir
	mgr2.cfgPath = filepath.Join(homeDir, "config.yaml")

	if err := mgr2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cfg := mgr2.Get()
	if cfg.Provider != "ollama" {
		t.Errorf("expected ollama, got %s", cfg.Provider)
	}
	if cfg.Model != "llama3" {
		t.Errorf("expected llama3, got %s", cfg.Model)
	}
}

func TestInitHome(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mgr.homeDir = tmpDir

	if err := mgr.InitHome(); err != nil {
		t.Fatalf("InitHome: %v", err)
	}

	// Check directories
	dirs := []string{"sessions", "memory", "logs", "skills"}
	for _, d := range dirs {
		path := filepath.Join(tmpDir, d)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("directory %s not created", d)
		}
	}

	// Check SOUL.md
	soulPath := filepath.Join(tmpDir, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		t.Error("SOUL.md not created")
	}
}
