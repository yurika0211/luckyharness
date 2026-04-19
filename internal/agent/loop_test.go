package agent

import (
	"testing"
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
