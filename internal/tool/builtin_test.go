package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinToolsRegistration(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	expected := []string{"shell", "file_read", "file_write", "file_list", "web_search", "web_fetch", "current_time"}
	for _, name := range expected {
		tool, ok := r.Get(name)
		if !ok {
			t.Errorf("builtin tool %s not registered", name)
			continue
		}
		if tool.Category != CatBuiltin {
			t.Errorf("expected CatBuiltin for %s, got %s", name, tool.Category)
		}
		if tool.Source != "builtin" {
			t.Errorf("expected source=builtin for %s, got %s", name, tool.Source)
		}
	}

	if r.Count() != len(expected) {
		t.Errorf("expected %d builtin tools, got %d", len(expected), r.Count())
	}
}

func TestCurrentTimeTool(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	result, err := r.Call("current_time", map[string]any{})
	if err != nil {
		t.Fatalf("current_time call: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty time result")
	}
}

func TestFileReadWriteTool(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	// 创建临时目录
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// 写文件
	writeResult, err := r.Call("file_write", map[string]any{
		"path":    testFile,
		"content": "Hello, LuckyHarness!",
	})
	if err != nil {
		t.Fatalf("file_write: %v", err)
	}
	if writeResult == "" {
		t.Error("expected write result")
	}

	// 读文件
	readResult, err := r.Call("file_read", map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("file_read: %v", err)
	}
	if readResult == "" {
		t.Error("expected read result")
	}
}

func TestFileListTool(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	// 创建临时目录和文件
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644)

	result, err := r.Call("file_list", map[string]any{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("file_list: %v", err)
	}
	if result == "" {
		t.Error("expected list result")
	}
}

func TestShellTool(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	result, err := r.Call("shell", map[string]any{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("shell call: %v", err)
	}
	if result == "" {
		t.Error("expected shell result")
	}
}

func TestPathTraversal(t *testing.T) {
	err := validatePath("../../etc/passwd")
	if err == nil {
		t.Error("expected path traversal error")
	}

	err = validatePath("/tmp/safe/path")
	if err != nil {
		t.Errorf("safe path should pass: %v", err)
	}
}

func TestToolPermissions(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	// 只读工具应该是 auto
	readPerm, _ := r.CheckPermission("file_read")
	if readPerm != PermAuto {
		t.Errorf("file_read should be auto, got %s", readPerm)
	}

	// 写操作应该是 approve
	writePerm, _ := r.CheckPermission("file_write")
	if writePerm != PermApprove {
		t.Errorf("file_write should be approve, got %s", writePerm)
	}

	// shell 应该是 approve
	shellPerm, _ := r.CheckPermission("shell")
	if shellPerm != PermApprove {
		t.Errorf("shell should be approve, got %s", shellPerm)
	}

	// current_time 应该是 auto
	timePerm, _ := r.CheckPermission("current_time")
	if timePerm != PermAuto {
		t.Errorf("current_time should be auto, got %s", timePerm)
	}
}

// ---------------------------------------------------------------------------
// v0.40.0: Search Tool Integration Tests
// ---------------------------------------------------------------------------

func TestWebSearchToolRegistration(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "ddgs", MaxResults: 3}
	r := NewRegistry()
	RegisterBuiltinToolsWithConfig(r, cfg)

	searchTool, ok := r.Get("web_search")
	if !ok {
		t.Fatal("web_search tool not registered")
	}
	if searchTool.Category != CatBuiltin {
		t.Errorf("expected CatBuiltin, got %s", searchTool.Category)
	}
	if searchTool.Permission != PermApprove {
		t.Errorf("expected PermApprove, got %s", searchTool.Permission)
	}

	fetchTool, ok := r.Get("web_fetch")
	if !ok {
		t.Fatal("web_fetch tool not registered")
	}
	if fetchTool.Category != CatBuiltin {
		t.Errorf("expected CatBuiltin, got %s", fetchTool.Category)
	}
}

func TestWebSearchConfigConversion(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *WebSearchConfig
		wantProv string
	}{
		{"nil config", nil, "brave"},
		{"brave with key", &WebSearchConfig{Provider: "brave", APIKey: "test-key"}, "brave"},
		{"ddgs", &WebSearchConfig{Provider: "ddgs", MaxResults: 10}, "ddgs"},
		{"searxng with base", &WebSearchConfig{Provider: "searxng", BaseURL: "http://localhost:8080"}, "searxng"},
		{"exa with key", &WebSearchConfig{Provider: "exa", APIKey: "exa-key"}, "exa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchCfg := tt.cfg.toSearchConfig()
			if searchCfg.DefaultProvider != tt.wantProv {
				t.Errorf("provider: got %q, want %q", searchCfg.DefaultProvider, tt.wantProv)
			}
		})
	}
}

func TestWebSearchConfigAPIKeyMapping(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "brave", APIKey: "brave-key"}
	searchCfg := cfg.toSearchConfig()
	if searchCfg.BraveAPIKey != "brave-key" {
		t.Errorf("BraveAPIKey: got %q, want %q", searchCfg.BraveAPIKey, "brave-key")
	}

	cfg = &WebSearchConfig{Provider: "exa", APIKey: "exa-key"}
	searchCfg = cfg.toSearchConfig()
	if searchCfg.ExaAPIKey != "exa-key" {
		t.Errorf("ExaAPIKey: got %q, want %q", searchCfg.ExaAPIKey, "exa-key")
	}

	cfg = &WebSearchConfig{Provider: "jina", APIKey: "jina-key"}
	searchCfg = cfg.toSearchConfig()
	if searchCfg.JinaAPIKey != "jina-key" {
		t.Errorf("JinaAPIKey: got %q, want %q", searchCfg.JinaAPIKey, "jina-key")
	}
}

func TestWebSearchConfigProxyAndBaseURL(t *testing.T) {
	cfg := &WebSearchConfig{
		Provider:   "searxng",
		BaseURL:    "http://search.local:8080",
		MaxResults: 7,
		Proxy:      "http://proxy:3128",
	}
	searchCfg := cfg.toSearchConfig()

	if searchCfg.SearXNGBaseURL != "http://search.local:8080" {
		t.Errorf("SearXNGBaseURL: got %q", searchCfg.SearXNGBaseURL)
	}
	if searchCfg.MaxResults != 7 {
		t.Errorf("MaxResults: got %d, want 7", searchCfg.MaxResults)
	}
	if searchCfg.Proxy != "http://proxy:3128" {
		t.Errorf("Proxy: got %q", searchCfg.Proxy)
	}
}

func TestWebSearchToolMissingQuery(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "ddgs"}
	r := NewRegistry()
	RegisterBuiltinToolsWithConfig(r, cfg)

	_, err := r.Call("web_search", map[string]any{})
	if err == nil {
		t.Error("expected error for missing query")
	}
}

func TestWebFetchToolMissingURL(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "ddgs"}
	r := NewRegistry()
	RegisterBuiltinToolsWithConfig(r, cfg)

	_, err := r.Call("web_fetch", map[string]any{})
	if err == nil {
		t.Error("expected error for missing url")
	}
}

func TestWebFetchToolInvalidURL(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "ddgs"}
	r := NewRegistry()
	RegisterBuiltinToolsWithConfig(r, cfg)

	// SSRF: private IP should be rejected
	_, err := r.Call("web_fetch", map[string]any{
		"url": "http://192.168.1.1/secret",
	})
	if err == nil {
		t.Error("expected error for private IP URL")
	}
}

func TestWebFetchToolLocalhost(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "ddgs"}
	r := NewRegistry()
	RegisterBuiltinToolsWithConfig(r, cfg)

	_, err := r.Call("web_fetch", map[string]any{
		"url": "http://localhost:8080/admin",
	})
	if err == nil {
		t.Error("expected error for localhost URL")
	}
}

func TestWebSearchToolCountClamping(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "ddgs", MaxResults: 5}
	tool := WebSearchTool(cfg)

	// Verify tool parameters include count
	if _, ok := tool.Parameters["count"]; !ok {
		t.Error("web_search tool missing count parameter")
	}
	// Verify tool parameters include mode
	if _, ok := tool.Parameters["mode"]; !ok {
		t.Error("web_search tool missing mode parameter")
	}
}

func TestWebFetchToolParameters(t *testing.T) {
	cfg := &WebSearchConfig{Provider: "ddgs"}
	tool := WebFetchTool(cfg)

	if _, ok := tool.Parameters["url"]; !ok {
		t.Error("web_fetch tool missing url parameter")
	}
	if _, ok := tool.Parameters["max_chars"]; !ok {
		t.Error("web_fetch tool missing max_chars parameter")
	}
}

func TestDefaultWebSearchConfig(t *testing.T) {
	cfg := defaultWebSearchConfig()
	if cfg.Provider != "brave" {
		t.Errorf("default provider: got %q, want %q", cfg.Provider, "brave")
	}
	if cfg.MaxResults != 5 {
		t.Errorf("default max results: got %d, want 5", cfg.MaxResults)
	}
}
