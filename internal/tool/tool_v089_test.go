package tool

import (
	"testing"
)

// TestCallWithShellContext 测试 CallWithShellContext 的各种场景
func TestCallWithShellContext(t *testing.T) {
	reg := NewRegistry()

	// 注册一个 ShellAware 工具
	reg.Register(&Tool{
		Name:       "test_shell",
		Handler: func(args map[string]any) (string, error) {
			cwd, _ := args["_cwd"].(string)
			return "cwd=" + cwd, nil
		},
		ShellAware: true,
		Enabled:    true,
	})

	// 注册一个普通工具
	normalTool := &Tool{
		Name:       "test_normal",
		Handler: func(args map[string]any) (string, error) {
			return "ok", nil
		},
		ShellAware: false,
	}
	reg.Register(normalTool)
	normalTool.Enabled = false // 手动禁用

	t.Run("tool not found", func(t *testing.T) {
		_, err := reg.CallWithShellContext("nonexistent", nil, nil)
		if err == nil {
			t.Error("expected error for nonexistent tool")
		}
	})

	t.Run("tool disabled", func(t *testing.T) {
		_, err := reg.CallWithShellContext("test_normal", nil, nil)
		if err == nil {
			t.Error("expected error for disabled tool")
		}
	})

	t.Run("shell context injection", func(t *testing.T) {
		sc := &ShellContext{
			Cwd: "/test/path",
			Env: map[string]string{"TEST": "value"},
		}
		result, err := reg.CallWithShellContext("test_shell", nil, sc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "cwd=/test/path" {
			t.Errorf("expected 'cwd=/test/path', got '%s'", result)
		}
	})

	t.Run("shell context nil", func(t *testing.T) {
		// 当 sc 为 nil 时不应崩溃
		result, err := reg.CallWithShellContext("test_shell", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// cwd 应该为空
		if result != "cwd=" {
			t.Errorf("expected 'cwd=', got '%s'", result)
		}
	})
}

// TestErrorFunctions 测试各种错误类型
func TestErrorFunctions(t *testing.T) {
	t.Run("ErrToolNotFound", func(t *testing.T) {
		err := ErrToolNotFound{name: "test"}
		if err.Error() != "tool not found: test" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("ErrToolDisabled", func(t *testing.T) {
		err := ErrToolDisabled{name: "test"}
		if err.Error() != "tool disabled: test" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("ErrToolDenied", func(t *testing.T) {
		err := ErrToolDenied{name: "test"}
		if err.Error() != "tool denied: test" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})
}

// TestOutputCompression 测试输出压缩相关函数
func TestOutputCompression(t *testing.T) {
	t.Run("DefaultOutputCompressConfig", func(t *testing.T) {
		config := DefaultOutputCompressConfig()
		if config.MaxChars != 2048 {
			t.Errorf("expected MaxChars=2048, got %d", config.MaxChars)
		}
		if !config.EnableTruncate {
			t.Error("expected EnableTruncate=true")
		}
		if !config.EnableDedup {
			t.Error("expected EnableDedup=true")
		}
	})

	t.Run("CompressOutput", func(t *testing.T) {
		config := OutputCompressConfig{
			MaxChars:     10,
			EnableTruncate: true,
		}
		result := CompressOutput("hello world", config)
		// 截断后会添加提示，长度会超过原 MaxChars
		if len(result) < 10 {
			t.Errorf("expected length >= 10, got %d", len(result))
		}
	})

	t.Run("ParallelCompressOutputs", func(t *testing.T) {
		outputs := map[string]string{
			"a": "hello",
			"b": "world",
		}
		config := OutputCompressConfig{MaxChars: 100}
		result := ParallelCompressOutputs(outputs, config)
		if len(result) != 2 {
			t.Errorf("expected 2 outputs, got %d", len(result))
		}
	})

	t.Run("DedupOutputs", func(t *testing.T) {
		outputs := map[string]string{
			"a": "same",
			"b": "same",
			"c": "different",
		}
		result := DedupOutputs(outputs)
		if len(result) != 2 {
			t.Errorf("expected 2 unique outputs, got %d", len(result))
		}
	})

	t.Run("TruncateOutputs", func(t *testing.T) {
		outputs := map[string]string{
			"a": "hello",
			"b": "world",
		}
		result := TruncateOutputs(outputs, 3)
		// 截断后会添加提示
		if result["a"][:3] != "hel" {
			t.Errorf("expected start with 'hel', got '%s'", result["a"])
		}
	})

	t.Run("TruncateOutput", func(t *testing.T) {
		result := TruncateOutput("hello world", 5)
		// 截断后会添加提示
		if result[:5] != "hello" {
			t.Errorf("expected start with 'hello', got '%s'", result)
		}
	})
}

// TestGetQuota 测试 usage_tracker 的 GetQuota
func TestGetQuota(t *testing.T) {
	tracker := NewUsageTracker()

	t.Run("get quota for unknown tool", func(t *testing.T) {
		quota := tracker.GetQuota("user123", "unknown_tool")
		if quota != nil {
			t.Error("expected nil quota for unknown tool")
		}
	})
}
