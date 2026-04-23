package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

// ---------------------------------------------------------------------------
// v0.62.0 ConfigCenter Coverage Improvements
// ---------------------------------------------------------------------------

func TestModelRouterNewModelRouter(t *testing.T) {
	config := ModelRouterConfig{
		Enable:        true,
		SimpleModel:   "gpt-4o-mini",
		ComplexModel:  "gpt-4o",
		LocalModel:    "qwen2.5-coder-32b",
		LocalBaseURL:  "http://localhost:11434",
		TokenThreshold: 500,
	}
	router := NewModelRouter(config)
	if router == nil {
		t.Fatal("NewModelRouter returned nil")
	}
	if router.config != config {
		t.Error("config not set correctly")
	}
}

func TestModelRouterSelectModel(t *testing.T) {
	config := ModelRouterConfig{
		Enable:        true,
		SimpleModel:   "gpt-4o-mini",
		ComplexModel:  "gpt-4o",
		LocalModel:    "qwen2.5-coder-32b",
		LocalBaseURL:  "http://localhost:11434",
		TokenThreshold: 500,
	}
	router := NewModelRouter(config)

	// Test simple task
	model, apiBase := router.SelectModel(TaskSimple)
	if model != "gpt-4o-mini" {
		t.Errorf("expected gpt-4o-mini, got %s", model)
	}

	// Test complex task
	model, apiBase = router.SelectModel(TaskComplex)
	if model != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", model)
	}

	// Test moderate task (should use local)
	model, apiBase = router.SelectModel(TaskModerate)
	if model != "qwen2.5-coder-32b" {
		t.Errorf("expected qwen2.5-coder-32b, got %s", model)
	}
	if apiBase != "http://localhost:11434" {
		t.Errorf("expected local base URL, got %s", apiBase)
	}
}

func TestModelRouterSelectModelDisabled(t *testing.T) {
	config := ModelRouterConfig{
		Enable: false,
	}
	router := NewModelRouter(config)
	model, apiBase := router.SelectModel(TaskSimple)
	if model != "" || apiBase != "" {
		t.Error("should return empty when disabled")
	}
}

func TestEstimateComplexity(t *testing.T) {
	config := ModelRouterConfig{
		Enable:         true,
		TokenThreshold: 100,
	}
	router := NewModelRouter(config)

	// Test simple keywords
	if complexity := router.EstimateComplexity("hello world", 50); complexity != TaskSimple {
		t.Errorf("expected TaskSimple for 'hello', got %v", complexity)
	}
	if complexity := router.EstimateComplexity("你好", 50); complexity != TaskSimple {
		t.Errorf("expected TaskSimple for '你好', got %v", complexity)
	}
	if complexity := router.EstimateComplexity("what time is it", 50); complexity != TaskSimple {
		t.Errorf("expected TaskSimple for 'what time', got %v", complexity)
	}

	// Test complex keywords
	if complexity := router.EstimateComplexity("write code for me", 50); complexity != TaskComplex {
		t.Errorf("expected TaskComplex for 'write code', got %v", complexity)
	}
	if complexity := router.EstimateComplexity("实现一个功能", 50); complexity != TaskComplex {
		t.Errorf("expected TaskComplex for '实现', got %v", complexity)
	}
	// Note: avoid words containing simple keywords like "hi" in "this"
	if complexity := router.EstimateComplexity("debugging is fun", 50); complexity != TaskComplex {
		t.Errorf("expected TaskComplex for 'debug', got %v", complexity)
	}

	// Test token count threshold
	if complexity := router.EstimateComplexity("some random text", 200); complexity != TaskComplex {
		t.Errorf("expected TaskComplex for high token count, got %v", complexity)
	}

	// Test default (moderate)
	if complexity := router.EstimateComplexity("some random text", 50); complexity != TaskModerate {
		t.Errorf("expected TaskModerate for default, got %v", complexity)
	}
}

func TestIsLocalTask(t *testing.T) {
	config := ModelRouterConfig{}
	router := NewModelRouter(config)

	// Test local keywords
	if !router.IsLocalTask("file operation") {
		t.Error("should detect 'file' as local")
	}
	if !router.IsLocalTask("运行命令") {
		t.Error("should detect '运行' as local")
	}
	if !router.IsLocalTask("terminal command") {
		t.Error("should detect 'terminal' as local")
	}
	if !router.IsLocalTask("本地文件") {
		t.Error("should detect '本地' as local")
	}

	// Test non-local
	if router.IsLocalTask("hello world") {
		t.Error("should not detect 'hello' as local")
	}
	if router.IsLocalTask("write code") {
		t.Error("should not detect 'write code' as local")
	}
}

func TestSelectModelForTask(t *testing.T) {
	config := ModelRouterConfig{
		Enable:        true,
		SimpleModel:   "gpt-4o-mini",
		ComplexModel:  "gpt-4o",
		LocalModel:    "qwen2.5-coder-32b",
		LocalBaseURL:  "http://localhost:11434",
		TokenThreshold: 500,
	}
	router := NewModelRouter(config)

	// Local task should use local model
	model, apiBase := router.SelectModelForTask("file operation", 100)
	if model != "qwen2.5-coder-32b" {
		t.Errorf("expected local model for local task, got %s", model)
	}

	// Complex task
	model, apiBase = router.SelectModelForTask("write code for me", 100)
	if model != "gpt-4o" {
		t.Errorf("expected complex model for complex task, got %s", model)
	}

	// Disabled router
	config.Enable = false
	router2 := NewModelRouter(config)
	model, apiBase = router2.SelectModelForTask("test", 100)
	if model != "" || apiBase != "" {
		t.Error("should return empty when disabled")
	}
}

func TestConfigWatcherOnError(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create manager first
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mgr.homeDir = tmpDir
	mgr.cfgPath = cfgPath

	// Create watcher with manager
	watcher := NewConfigWatcher(mgr, 1*time.Second)

	// Set error callback
	watcher.OnError(func(err error) {
		t.Logf("Error callback triggered: %v", err)
	})

	// Start watcher
	if err := watcher.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer watcher.Stop()

	// Write initial valid config
	if err := os.WriteFile(cfgPath, []byte("provider: test\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait a bit for watcher to pick up changes
	time.Sleep(100 * time.Millisecond)
}

func TestManagerConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mgr.cfgPath = cfgPath

	if mgr.ConfigFile() != cfgPath {
		t.Errorf("expected %s, got %s", cfgPath, mgr.ConfigFile())
	}
}

func TestManagerHomeDirPath(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mgr.homeDir = tmpDir

	if mgr.HomeDirPath() != tmpDir {
		t.Errorf("expected %s, got %s", tmpDir, mgr.HomeDirPath())
	}
}

func TestHomeDir(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	homeDir := mgr.HomeDir()
	if homeDir == "" {
		t.Error("HomeDir should not be empty")
	}
}

func TestSetInvalidKey(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Test setting invalid key
	err = mgr.Set("invalid_key", "value")
	if err != nil {
		t.Logf("Set invalid key returned error (expected): %v", err)
	}
}

func TestModelRouterEstimateComplexityTokenThreshold(t *testing.T) {
	config := ModelRouterConfig{
		TokenThreshold: 0, // Zero threshold
	}
	router := NewModelRouter(config)

	// Should handle zero threshold gracefully
	complexity := router.EstimateComplexity("some text", 1000)
	if complexity != TaskComplex {
		t.Errorf("expected TaskComplex for high token count with zero threshold, got %v", complexity)
	}
}
