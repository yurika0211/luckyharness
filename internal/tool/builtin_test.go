package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinToolsRegistration(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	expected := []string{"shell", "file_read", "file_write", "file_list", "web_search", "web_fetch", "current_time", "remember", "recall"}
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
