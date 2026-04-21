package tool

import (
	"context"
	"testing"
	"time"
)

func TestSandboxExecute(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Category:    CatSkill,
		Source:      "test-skill",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "hello from sandbox", nil
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	result, err := sb.Execute("test_tool", map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "hello from sandbox" {
		t.Errorf("expected 'hello from sandbox', got %s", result)
	}
}

func TestSandboxExecuteNotFound(t *testing.T) {
	r := NewRegistry()
	sb := NewSkillSandbox(r, DefaultSandboxConfig())

	_, err := sb.Execute("nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
	_, ok := err.(*SandboxError)
	if !ok {
		t.Errorf("expected *SandboxError, got %T", err)
	}
}

func TestSandboxExecuteDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "disabled_tool",
		Description: "Disabled",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Handler: func(args map[string]any) (string, error) {
			return "should not reach", nil
		},
	})
	r.Disable("disabled_tool") // Registry.Register forces Enabled=true, so disable explicitly

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	_, err := sb.Execute("disabled_tool", nil)
	if err == nil {
		t.Error("expected error for disabled tool")
	}
}

func TestSandboxPanicRecovery(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "panic_tool",
		Description: "Panics",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			panic("something went wrong!")
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	_, err := sb.Execute("panic_tool", nil)
	if err == nil {
		t.Error("expected error for panicking tool")
	}
	_, ok := err.(*SandboxError)
	if !ok {
		t.Fatalf("expected *SandboxError, got %T", err)
	}
	if !err.(*SandboxError).Panic {
		t.Error("expected Panic=true in sandbox error")
	}
}

func TestSandboxTimeout(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "slow_tool",
		Description: "Slow tool",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			time.Sleep(5 * time.Second)
			return "should not reach", nil
		},
	})

	config := SandboxConfig{
		MaxExecutionTime: 100 * time.Millisecond,
		MaxOutputSize:    65536,
		EnableAuditLog:   true,
	}
	sb := NewSkillSandbox(r, config)

	_, err := sb.Execute("slow_tool", nil)
	if err == nil {
		t.Error("expected timeout error")
	}
	_, ok := err.(*SandboxError)
	if !ok {
		t.Fatalf("expected *SandboxError, got %T", err)
	}
	if !err.(*SandboxError).Timeout {
		t.Error("expected Timeout=true in sandbox error")
	}
}

func TestSandboxOutputTruncation(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "verbose_tool",
		Description: "Returns large output",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			// Generate large output
			result := make([]byte, 1000)
			for i := range result {
				result[i] = 'a'
			}
			return string(result), nil
		},
	})

	config := SandboxConfig{
		MaxExecutionTime: 5 * time.Second,
		MaxOutputSize:    100, // very small limit
		EnableAuditLog:   true,
	}
	sb := NewSkillSandbox(r, config)

	result, err := sb.Execute("verbose_tool", nil)
	// Output should be truncated
	if len(result) > 200 { // 100 + truncation message
		t.Errorf("expected truncated output, got %d bytes", len(result))
	}
	// Should have oversize error
	if err == nil {
		t.Error("expected oversize error")
	}
	_, ok := err.(*SandboxError)
	if !ok {
		t.Fatalf("expected *SandboxError, got %T", err)
	}
	if !err.(*SandboxError).Oversize {
		t.Error("expected Oversize=true")
	}
}

func TestSandboxAuditLog(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "audited_tool",
		Description: "Audited",
		Category:    CatSkill,
		Source:      "test-skill",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "ok", nil
		},
	})

	config := DefaultSandboxConfig()
	config.EnableAuditLog = true
	sb := NewSkillSandbox(r, config)

	sb.Execute("audited_tool", map[string]any{"key": "value"})
	sb.Execute("audited_tool", map[string]any{"key": "value2"})

	log := sb.AuditLog()
	if len(log) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(log))
	}

	if log[0].ToolName != "audited_tool" {
		t.Errorf("expected tool_name=audited_tool, got %s", log[0].ToolName)
	}
	if log[0].SkillName != "test-skill" {
		t.Errorf("expected skill_name=test-skill, got %s", log[0].SkillName)
	}
	if !log[0].Success {
		t.Error("expected success=true")
	}
}

func TestSandboxAuditLogDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "no_audit_tool",
		Description: "No audit",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "ok", nil
		},
	})

	config := SandboxConfig{
		MaxExecutionTime: 5 * time.Second,
		MaxOutputSize:    65536,
		EnableAuditLog:   false,
	}
	sb := NewSkillSandbox(r, config)

	sb.Execute("no_audit_tool", nil)

	log := sb.AuditLog()
	if len(log) != 0 {
		t.Errorf("expected 0 audit entries when disabled, got %d", len(log))
	}
}

func TestSandboxAuditLogForTool(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "tool_a",
		Description: "Tool A",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "a", nil
		},
	})
	r.Register(&Tool{
		Name:        "tool_b",
		Description: "Tool B",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "b", nil
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	sb.Execute("tool_a", nil)
	sb.Execute("tool_b", nil)
	sb.Execute("tool_a", nil)

	logA := sb.AuditLogForTool("tool_a")
	if len(logA) != 2 {
		t.Errorf("expected 2 entries for tool_a, got %d", len(logA))
	}

	logB := sb.AuditLogForTool("tool_b")
	if len(logB) != 1 {
		t.Errorf("expected 1 entry for tool_b, got %d", len(logB))
	}
}

func TestSandboxAuditLogForSkill(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "skill_a_tool1",
		Description: "Tool 1",
		Category:    CatSkill,
		Source:      "skill-a",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "1", nil
		},
	})
	r.Register(&Tool{
		Name:        "skill_b_tool1",
		Description: "Tool 1",
		Category:    CatSkill,
		Source:      "skill-b",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "1", nil
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	sb.Execute("skill_a_tool1", nil)
	sb.Execute("skill_b_tool1", nil)

	logA := sb.AuditLogForSkill("skill-a")
	if len(logA) != 1 {
		t.Errorf("expected 1 entry for skill-a, got %d", len(logA))
	}
}

func TestSandboxClearAuditLog(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "clear_test",
		Description: "Test",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "ok", nil
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	sb.Execute("clear_test", nil)

	if len(sb.AuditLog()) != 1 {
		t.Error("expected 1 entry before clear")
	}

	sb.ClearAuditLog()
	if len(sb.AuditLog()) != 0 {
		t.Error("expected 0 entries after clear")
	}
}

func TestSandboxAuditStats(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "stats_tool",
		Description: "Stats",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "ok", nil
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	sb.Execute("stats_tool", nil)
	sb.Execute("stats_tool", nil)

	stats := sb.AuditStats()
	if stats.TotalInvocations != 2 {
		t.Errorf("expected 2 invocations, got %d", stats.TotalInvocations)
	}
	if stats.SuccessfulInvocations != 2 {
		t.Errorf("expected 2 successful, got %d", stats.SuccessfulInvocations)
	}
	if stats.PanicCount != 0 {
		t.Errorf("expected 0 panics, got %d", stats.PanicCount)
	}
}

func TestSandboxExecuteWithContext(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "ctx_tool",
		Description: "Context tool",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			return "context ok", nil
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	ctx := context.Background()

	result, err := sb.ExecuteWithContext(ctx, "ctx_tool", nil)
	if err != nil {
		t.Fatalf("ExecuteWithContext: %v", err)
	}
	if result != "context ok" {
		t.Errorf("expected 'context ok', got %s", result)
	}
}

func TestSandboxExecuteWithContextCancelled(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "slow_ctx_tool",
		Description: "Slow context tool",
		Category:    CatSkill,
		Source:      "test",
		Permission:  PermAuto,
		Enabled:     true,
		Handler: func(args map[string]any) (string, error) {
			time.Sleep(5 * time.Second)
			return "should not reach", nil
		},
	})

	sb := NewSkillSandbox(r, DefaultSandboxConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := sb.ExecuteWithContext(ctx, "slow_ctx_tool", nil)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestSandboxErrorMessages(t *testing.T) {
	err := &SandboxError{ToolName: "test", Timeout: true}
	if err.Error() == "" {
		t.Error("expected non-empty timeout error message")
	}

	err = &SandboxError{ToolName: "test", Panic: true, Message: "boom"}
	if err.Error() == "" {
		t.Error("expected non-empty panic error message")
	}

	err = &SandboxError{ToolName: "test", Oversize: true}
	if err.Error() == "" {
		t.Error("expected non-empty oversize error message")
	}

	err = &SandboxError{ToolName: "test", Message: "generic"}
	if err.Error() == "" {
		t.Error("expected non-empty generic error message")
	}
}

func TestFormatAuditEntry(t *testing.T) {
	entry := AuditEntry{
		Timestamp: time.Now(),
		ToolName:  "test_tool",
		SkillName: "test-skill",
		Success:   true,
		Duration:  100 * time.Millisecond,
	}

	formatted := FormatAuditEntry(entry)
	if formatted == "" {
		t.Error("expected non-empty formatted entry")
	}
}

func TestTruncateArgs(t *testing.T) {
	args := map[string]any{
		"short": "hello",
		"long":  string(make([]byte, 300)),
	}

	result := truncateArgs(args, 200)
	if len(result["short"].(string)) != 5 {
		t.Error("short value should not be truncated")
	}
	longVal := result["long"].(string)
	if len(longVal) > 210 { // 200 + "..."
		t.Errorf("long value should be truncated, got %d chars", len(longVal))
	}
}

func TestDefaultSandboxConfig(t *testing.T) {
	config := DefaultSandboxConfig()
	if config.MaxExecutionTime != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", config.MaxExecutionTime)
	}
	if config.MaxOutputSize != 65536 {
		t.Errorf("expected 65536 output size, got %d", config.MaxOutputSize)
	}
	if !config.EnableAuditLog {
		t.Error("expected audit log enabled by default")
	}
}