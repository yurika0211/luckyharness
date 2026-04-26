package agent

import (
	"strings"
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
	if cfg.RepeatToolCallLimit != 3 {
		t.Errorf("expected repeat limit 3, got %d", cfg.RepeatToolCallLimit)
	}
	if cfg.ToolOnlyIterationLimit != 3 {
		t.Errorf("expected tool-only iteration limit 3, got %d", cfg.ToolOnlyIterationLimit)
	}
	if cfg.DuplicateFetchLimit != 1 {
		t.Errorf("expected duplicate fetch limit 1, got %d", cfg.DuplicateFetchLimit)
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

func TestShouldForceSearchSynthesis(t *testing.T) {
	if shouldForceSearchSynthesis(1, 2) {
		t.Fatal("should not force synthesis with insufficient evidence")
	}
	if shouldForceSearchSynthesis(2, 1) {
		t.Fatal("should not force synthesis with insufficient tool-only iterations")
	}
	if !shouldForceSearchSynthesis(2, 2) {
		t.Fatal("expected synthesis to be forced")
	}
}

func TestIsUsefulSearchEvidence(t *testing.T) {
	if !isUsefulSearchEvidence("web_search", "Results for: 四川大学 食堂\n\n1. Test") {
		t.Fatal("expected non-empty search results to count as evidence")
	}
	if isUsefulSearchEvidence("web_search", "No results found for '四川大学 食堂' (all search sources failed)") {
		t.Fatal("expected no-results output not to count as evidence")
	}
	if isUsefulSearchEvidence("shell", "Results for: 四川大学 食堂") {
		t.Fatal("non-search tools should not count as search evidence")
	}
}

func TestCompactToolResultForContext(t *testing.T) {
	long := "Results for: test\n\n" + strings.Repeat("x", 5000)
	got := compactToolResultForContext("web_search", long)
	if len(got) >= len(long) {
		t.Fatal("expected web_search result to be compacted")
	}
	if !strings.Contains(got, "truncated for context") {
		t.Fatal("expected truncation marker")
	}
}

func TestNormalizedToolTarget(t *testing.T) {
	args := `{"url":"https://Example.com/path#frag","max_chars":5000}`
	got := normalizedToolTarget("web_fetch", args)
	if got != "https://example.com/path" {
		t.Fatalf("unexpected normalized target: %q", got)
	}
	if normalizedToolTarget("web_search", args) != "" {
		t.Fatal("non-web_fetch tool should not have target")
	}
}

func TestExecuteToolMaybeDedupSkipsDuplicateFetch(t *testing.T) {
	a := &Agent{}
	repeats := map[string]int{"https://example.com/path": 2}
	last := map[string]string{"https://example.com/path": "Fetched content"}
	out, err := a.executeToolMaybeDedup("web_fetch", `{"url":"https://example.com/path"}`, true, nil, repeats, last, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Skipped duplicate web_fetch") {
		t.Fatalf("expected duplicate skip message, got %q", out)
	}
	if !strings.Contains(out, "Fetched content") {
		t.Fatalf("expected cached content to be reused, got %q", out)
	}
}
