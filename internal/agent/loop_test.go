package agent

import (
	"testing"

	"github.com/yurika0211/luckyharness/internal/tool"
)

func TestLoopStateString(t *testing.T) {
	tests := []struct {
		state    LoopState
		expected string
	}{
		{StateReason, "Reason"},
		{StateAct, "Act"},
		{StateObserve, "Observe"},
		{StateDone, "Done"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("LoopState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestDefaultLoopConfig(t *testing.T) {
	cfg := DefaultLoopConfig()
	if cfg.MaxIterations != 10 {
		t.Errorf("expected 10 iterations, got %d", cfg.MaxIterations)
	}
	if cfg.AutoApprove != false {
		t.Error("expected AutoApprove false by default")
	}
}

func TestExtractRequiredToolNames(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(&tool.Tool{Name: "file_read", Enabled: true})
	reg.Register(&tool.Tool{Name: "current_time", Enabled: true})
	reg.Register(&tool.Tool{Name: "shell", Enabled: true})

	a := &Agent{tools: reg}
	got := a.extractRequiredToolNames("请必须调用 file_read 和 current_time，最后不要用 shell")

	if len(got) != 3 {
		t.Fatalf("expected 3 required tools, got %d (%v)", len(got), got)
	}
	if got[0] != "file_read" || got[1] != "current_time" || got[2] != "shell" {
		t.Fatalf("unexpected order: %v", got)
	}
}
