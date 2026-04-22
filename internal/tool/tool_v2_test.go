package tool

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// ── CallWithShellContext ──────────────────────────────────────────────────────

func TestCallWithShellContext_ShellAware(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "shell",
		Description: "Shell tool",
		Category:    CatBuiltin,
		Permission:  PermAuto,
		ShellAware:  true,
		Handler: func(args map[string]any) (string, error) {
			cwd, _ := args["_cwd"].(string)
			return "cwd=" + cwd, nil
		},
	})

	result, err := r.CallWithShellContext("shell", nil, &ShellContext{
		Cwd: "/home/user",
		Env: map[string]string{"PATH": "/usr/bin"},
	})
	if err != nil {
		t.Fatalf("CallWithShellContext: %v", err)
	}
	if !strings.Contains(result, "cwd=/home/user") {
		t.Errorf("expected cwd injection, got %q", result)
	}
}

func TestCallWithShellContext_NonShellAware(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "echo",
		Description: "Echo tool",
		Category:    CatBuiltin,
		Permission:  PermAuto,
		ShellAware:  false,
		Handler: func(args map[string]any) (string, error) {
			if _, ok := args["_cwd"]; ok {
				return "should not have _cwd", nil
			}
			return "ok", nil
		},
	})

	result, err := r.CallWithShellContext("echo", map[string]any{}, &ShellContext{
		Cwd: "/home/user",
	})
	if err != nil {
		t.Fatalf("CallWithShellContext: %v", err)
	}
	if result != "ok" {
		t.Errorf("non-shell-aware tool should not get context, got %q", result)
	}
}

func TestCallWithShellContext_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.CallWithShellContext("nonexistent", nil, nil)
	if _, ok := err.(ErrToolNotFound); !ok {
		t.Errorf("expected ErrToolNotFound, got %T: %v", err, err)
	}
}

func TestCallWithShellContext_Disabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:    "disabled",
		Permission: PermAuto,
		Handler: func(args map[string]any) (string, error) { return "x", nil },
	})
	r.Disable("disabled")

	_, err := r.CallWithShellContext("disabled", nil, nil)
	if _, ok := err.(ErrToolDisabled); !ok {
		t.Errorf("expected ErrToolDisabled, got %T: %v", err, err)
	}
}

func TestCallWithShellContext_Denied(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:       "denied",
		Permission: PermAuto,
		Handler:    func(args map[string]any) (string, error) { return "x", nil },
	})
	r.SetPermissionOverride("denied", PermDeny)

	_, err := r.CallWithShellContext("denied", nil, nil)
	if _, ok := err.(ErrToolDenied); !ok {
		t.Errorf("expected ErrToolDenied, got %T: %v", err, err)
	}
}

func TestCallWithShellContext_NilArgs(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:       "shell",
		Permission: PermAuto,
		ShellAware: true,
		Handler: func(args map[string]any) (string, error) {
			if args == nil {
				return "nil args", nil
			}
			cwd, _ := args["_cwd"].(string)
			return cwd, nil
		},
	})

	result, err := r.CallWithShellContext("shell", nil, &ShellContext{Cwd: "/test"})
	if err != nil {
		t.Fatalf("CallWithShellContext: %v", err)
	}
	if result != "/test" {
		t.Errorf("expected /test, got %q", result)
	}
}

// ── Error types ──────────────────────────────────────────────────────────────

func TestErrToolNotFound(t *testing.T) {
	err := ErrToolNotFound{name: "missing"}
	if err.Error() != "tool not found: missing" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestErrToolDisabled(t *testing.T) {
	err := ErrToolDisabled{name: "off"}
	if err.Error() != "tool disabled: off" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestErrToolDenied(t *testing.T) {
	err := ErrToolDenied{name: "nope"}
	if err.Error() != "tool denied: nope" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// ── Category is string type ──────────────────────────────────────────────────

func TestCategoryString(t *testing.T) {
	cats := []struct {
		cat Category
		exp string
	}{
		{CatBuiltin, "builtin"},
		{CatSkill, "skill"},
		{CatMCP, "mcp"},
		{CatDelegate, "delegate"},
	}
	for _, tt := range cats {
		if string(tt.cat) != tt.exp {
			t.Errorf("Category = %q, want %q", tt.cat, tt.exp)
		}
	}
}

// ── UsageTracker: GetQuota ───────────────────────────────────────────────────

func TestGetQuota(t *testing.T) {
	tracker := NewUsageTracker()
	// No quota set
	if q := tracker.GetQuota("user1", "tool1"); q != nil {
		t.Error("expected nil quota when not set")
	}

	// Set and get
	tracker.SetQuota("user1", "tool1", "daily", 100)
	q := tracker.GetQuota("user1", "tool1")
	if q == nil {
		t.Fatal("expected quota, got nil")
	}
	if q.Limit != 100 {
		t.Errorf("expected limit 100, got %d", q.Limit)
	}
	if q.Window != "daily" {
		t.Errorf("expected window daily, got %q", q.Window)
	}
}

// ── UsageTracker: IncrementUsage with reset ──────────────────────────────────

func TestIncrementUsage_ResetOnExpiry(t *testing.T) {
	tracker := NewUsageTracker()
	tracker.SetQuota("user1", "tool1", "daily", 10)

	// Manually set quota to expired
	key := "user1:tool1"
	tracker.mu.Lock()
	tracker.quotas[key].Used = 9
	tracker.quotas[key].ResetAt = time.Now().Add(-time.Hour) // expired
	tracker.mu.Unlock()

	// Increment should reset
	tracker.IncrementUsage("user1", "tool1")

	tracker.mu.RLock()
	q := tracker.quotas[key]
	tracker.mu.RUnlock()

	if q.Used != 1 {
		t.Errorf("expected used=1 after reset+increment, got %d", q.Used)
	}
	if q.ResetAt.Before(time.Now()) {
		t.Error("ResetAt should be in the future after reset")
	}
}

func TestIncrementUsage_NoQuota(t *testing.T) {
	tracker := NewUsageTracker()
	// Should not panic when no quota exists
	tracker.IncrementUsage("user1", "tool1")
}

func TestIncrementUsage_Concurrent(t *testing.T) {
	tracker := NewUsageTracker()
	tracker.SetQuota("user1", "tool1", "daily", 1000)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.IncrementUsage("user1", "tool1")
		}()
	}
	wg.Wait()

	q := tracker.GetQuota("user1", "tool1")
	if q.Used != 100 {
		t.Errorf("expected 100 concurrent increments, got %d", q.Used)
	}
}

// ── UsageTracker: CheckQuota expired ─────────────────────────────────────────

func TestCheckQuota_ExpiredQuota(t *testing.T) {
	tracker := NewUsageTracker()
	tracker.SetQuota("user1", "tool1", "daily", 5)

	// Manually exhaust and expire
	key := "user1:tool1"
	tracker.mu.Lock()
	tracker.quotas[key].Used = 5
	tracker.quotas[key].ResetAt = time.Now().Add(-time.Hour)
	tracker.mu.Unlock()

	// Should return true because quota expired
	if !tracker.CheckQuota("user1", "tool1") {
		t.Error("expected quota to be available after expiry")
	}
}

// ── UsageTracker: SetQuota errors ────────────────────────────────────────────

func TestSetQuota_NegativeLimit(t *testing.T) {
	tracker := NewUsageTracker()
	err := tracker.SetQuota("user1", "tool1", "daily", -1)
	if err == nil {
		t.Error("expected error for negative limit")
	}
}

func TestSetQuota_InvalidWindow(t *testing.T) {
	tracker := NewUsageTracker()
	err := tracker.SetQuota("user1", "tool1", "weekly", 10)
	if err == nil {
		t.Error("expected error for invalid window")
	}
}

func TestSetQuota_ValidWindows(t *testing.T) {
	tracker := NewUsageTracker()
	for _, w := range []string{"hourly", "daily", "monthly"} {
		if err := tracker.SetQuota("user1", "tool1", w, 10); err != nil {
			t.Errorf("SetQuota(%q): %v", w, err)
		}
	}
}

// ── UsageTracker: Record limit ───────────────────────────────────────────────

func TestRecord_Truncation(t *testing.T) {
	tracker := NewUsageTracker()
	for i := 0; i < 1100; i++ {
		tracker.Record("user1", "tool1", time.Millisecond, true)
	}
	stats := tracker.GetUsage("user1", "tool1")
	if stats.TotalCalls != 1000 {
		t.Errorf("expected 1000 records after truncation, got %d", stats.TotalCalls)
	}
}

// ── Pure functions: normalizeURL ─────────────────────────────────────────────

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"https://Example.COM/path/", "https://example.com/path"},
		{"https://example.com/path#fragment", "https://example.com/path"},
		{"not-a-url", "not-a-url"},
	}
	for _, tt := range tests {
		got := normalizeURL(tt.input)
		if got != tt.expect {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

// ── Pure functions: annotateSource ───────────────────────────────────────────

func TestAnnotateSource(t *testing.T) {
	result := annotateSource("Results for: test query", "brave")
	if !strings.Contains(result, "[Source: brave]") {
		t.Errorf("annotateSource missing source tag: %q", result)
	}
}

// ── Pure functions: formatEntries ────────────────────────────────────────────

func TestFormatEntries(t *testing.T) {
	entries := []searchEntry{
		{Title: "Go", URL: "https://go.dev", Snippet: "The Go language"},
		{Title: "Rust", URL: "https://rust-lang.org", Snippet: ""},
	}
	result := formatEntries("test", entries, 2)
	if !strings.Contains(result, "Results for: test") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "Go") || !strings.Contains(result, "Rust") {
		t.Error("missing entries")
	}
	if !strings.Contains(result, "The Go language") {
		t.Error("missing snippet")
	}
}

func TestFormatEntries_Truncation(t *testing.T) {
	// Generate entries that exceed 8000 chars
	var entries []searchEntry
	for i := 0; i < 200; i++ {
		entries = append(entries, searchEntry{
			Title:   strings.Repeat("A", 50),
			URL:     "https://example.com/" + strings.Repeat("x", 30),
			Snippet: strings.Repeat("S", 50),
		})
	}
	result := formatEntries("big", entries, 200)
	if len(result) > 8100 { // allow some margin for truncation suffix
		t.Errorf("result too long: %d chars", len(result))
	}
	if !strings.Contains(result, "truncated") {
		t.Error("expected truncation marker")
	}
}

func TestFormatEntries_CountLimit(t *testing.T) {
	entries := []searchEntry{
		{Title: "A", URL: "https://a.com"},
		{Title: "B", URL: "https://b.com"},
		{Title: "C", URL: "https://c.com"},
	}
	result := formatEntries("test", entries, 2)
	if strings.Contains(result, "C") {
		t.Error("should only show 2 entries, but C appeared")
	}
}

// ── Pure functions: stripHTMLTags ────────────────────────────────────────────

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"<b>bold</b>", "bold"},
		{"no tags", "no tags"},
		{"<a href='x'>link</a> text", "link text"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripHTMLTags(tt.input)
		if got != tt.expect {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

// ── Pure functions: urlEncode ────────────────────────────────────────────────

func TestURLEncode(t *testing.T) {
	got := urlEncode("hello world")
	if strings.Contains(got, " ") {
		t.Errorf("urlEncode should encode spaces: %q", got)
	}
	if !strings.Contains(got, "%20") {
		t.Errorf("urlEncode should use %%20 for spaces: %q", got)
	}
}

// ── Pure functions: normalizeWhitespace ──────────────────────────────────────

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"  hello   world  ", "hello world"},
		{"single", "single"},
		{"\t\n  spaces\t\n", "spaces"},
	}
	for _, tt := range tests {
		got := normalizeWhitespace(tt.input)
		if got != tt.expect {
			t.Errorf("normalizeWhitespace(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

// ── Pure functions: validateFetchURL ─────────────────────────────────────────

func TestValidateFetchURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com", false},
		{"ftp denied", "ftp://example.com", true},
		{"javascript denied", "javascript:alert(1)", true},
		{"localhost denied", "http://localhost:8080", true},
		{"loopback denied", "http://127.0.0.1/", true},
		{"private 10.x denied", "http://10.0.0.1/", true},
		{"private 192.168 denied", "http://192.168.1.1/", true},
		{"private 172.16 denied", "http://172.16.0.1/", true},
		{"private 172.31 denied", "http://172.31.0.1/", true},
		{"public 172.32 ok", "http://172.32.0.1/", false},
		{"public 172.15 ok", "http://172.15.0.1/", false},
		{"link-local denied", "http://169.254.169.254/", true},
		{"unspecified denied", "http://0.0.0.0/", true},
		{"empty host denied", "http://", true},
		{"invalid url", "://bad", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFetchURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFetchURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// ── Pure functions: parseDDGLiteHTML ─────────────────────────────────────────

func TestParseDDGLiteHTML(t *testing.T) {
	html := `<a class="result__a" href="https://go.dev">Go Language</a>
<a class="result__snippet">The Go programming language</a>
<a class="result__a" href="https://rust-lang.org">Rust Language</a>`

	result := parseDDGLiteHTML(html, 5)
	if !strings.Contains(result, "Go Language") {
		t.Error("missing Go entry")
	}
	if !strings.Contains(result, "Rust Language") {
		t.Error("missing Rust entry")
	}
	if !strings.Contains(result, "DDG Lite") {
		t.Error("missing DDG Lite header")
	}
}

func TestParseDDGLiteHTML_CountLimit(t *testing.T) {
	html := `<a class="result__a" href="https://a.com">A</a>
<a class="result__a" href="https://b.com">B</a>
<a class="result__a" href="https://c.com">C</a>`

	result := parseDDGLiteHTML(html, 2)
	if strings.Contains(result, "C") {
		t.Error("should only show 2 results")
	}
}

func TestParseDDGLiteHTML_Empty(t *testing.T) {
	result := parseDDGLiteHTML("", 5)
	if !strings.Contains(result, "DDG Lite") {
		t.Error("should still have header")
	}
}

// ── Enable/Disable nonexistent ───────────────────────────────────────────────

func TestEnableNonexistent(t *testing.T) {
	r := NewRegistry()
	err := r.Enable("nonexistent")
	if _, ok := err.(ErrToolNotFound); !ok {
		t.Errorf("expected ErrToolNotFound, got %T: %v", err, err)
	}
}

func TestDisableNonexistent(t *testing.T) {
	r := NewRegistry()
	err := r.Disable("nonexistent")
	if _, ok := err.(ErrToolNotFound); !ok {
		t.Errorf("expected ErrToolNotFound, got %T: %v", err, err)
	}
}

func TestSetPermissionOverrideNonexistent(t *testing.T) {
	r := NewRegistry()
	err := r.SetPermissionOverride("nonexistent", PermDeny)
	if _, ok := err.(ErrToolNotFound); !ok {
		t.Errorf("expected ErrToolNotFound, got %T: %v", err, err)
	}
}

func TestCheckPermissionNonexistent(t *testing.T) {
	r := NewRegistry()
	_, err := r.CheckPermission("nonexistent")
	if _, ok := err.(ErrToolNotFound); !ok {
		t.Errorf("expected ErrToolNotFound, got %T: %v", err, err)
	}
}

// ── FormatToolList with disabled tools ───────────────────────────────────────

func TestFormatToolList_DisabledTool(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "active",
		Description: "Active tool",
		Category:    CatBuiltin,
		Permission:  PermAuto,
	})
	r.Register(&Tool{
		Name:        "inactive",
		Description: "Inactive tool",
		Category:    CatBuiltin,
		Permission:  PermApprove,
	})
	r.Disable("inactive")

	list := r.FormatToolList()
	if !strings.Contains(list, "❌") {
		t.Error("expected ❌ for disabled tool")
	}
	if !strings.Contains(list, "✅") {
		t.Error("expected ✅ for enabled tool")
	}
	if !strings.Contains(list, "🟡") {
		t.Error("expected 🟡 for approve permission")
	}
}

// ── ToOpenAIFormat with nested params ────────────────────────────────────────

func TestToOpenAIFormat_NestedParams(t *testing.T) {
	tool := &Tool{
		Name:        "complex",
		Description: "Complex tool",
		Parameters: map[string]Param{
			"query": {
				Type:        "string",
				Description: "Search query",
				Required:    true,
			},
			"count": {
				Type:        "number",
				Description: "Number of results",
				Required:    false,
				Default:     5,
			},
		},
	}

	fmt := tool.ToOpenAIFormat()
	fn, ok := fmt["function"].(map[string]any)
	if !ok {
		t.Fatal("expected function key")
	}
	if fn["name"] != "complex" {
		t.Errorf("expected name=complex, got %v", fn["name"])
	}
	params, ok := fn["parameters"].(map[string]any)
	if !ok {
		t.Fatal("expected parameters key")
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties key")
	}
	if _, ok := props["query"]; !ok {
		t.Error("missing query property")
	}
	if _, ok := props["count"]; !ok {
		t.Error("missing count property")
	}
	// Check default value
	countProp := props["count"].(map[string]any)
	if countProp["default"] != 5 {
		t.Errorf("expected default=5, got %v", countProp["default"])
	}
}

// ── ParallelSafe flag on builtin tools ───────────────────────────────────────

func TestBuiltinToolsParallelSafe(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinTools(r)

	parallelSafe := []string{"web_search", "web_fetch", "current_time", "recall"}
	notParallelSafe := []string{"shell", "remember"}

	for _, name := range parallelSafe {
		tool, ok := r.Get(name)
		if !ok {
			t.Errorf("tool %s not found", name)
			continue
		}
		if !tool.ParallelSafe {
			t.Errorf("%s should be ParallelSafe", name)
		}
	}

	for _, name := range notParallelSafe {
		tool, ok := r.Get(name)
		if !ok {
			t.Errorf("tool %s not found", name)
			continue
		}
		if tool.ParallelSafe {
			t.Errorf("%s should NOT be ParallelSafe", name)
		}
	}
}

// ── RegisterBuiltinToolsWithConfig ───────────────────────────────────────────

func TestRegisterBuiltinToolsWithConfig(t *testing.T) {
	r := NewRegistry()
	cfg := &WebSearchConfig{
		APIKey: "test-key",
		Proxy:  "http://proxy:8080",
	}
	RegisterBuiltinToolsWithConfig(r, cfg)

	// Should have same tools as RegisterBuiltinTools
	expected := []string{"shell", "file_read", "file_write", "file_list", "web_search", "web_fetch", "current_time", "remember", "recall"}
	if r.Count() != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), r.Count())
	}
}

// ── Duplicate Register ───────────────────────────────────────────────────────

func TestDuplicateRegister(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{Name: "dup", Description: "first"})
	r.Register(&Tool{Name: "dup", Description: "second"})

	tool, _ := r.Get("dup")
	if tool.Description != "second" {
		t.Errorf("expected second registration to win, got %q", tool.Description)
	}
}

// ── ListEnabled with all disabled ────────────────────────────────────────────

func TestListEnabled_AllDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{Name: "a", Category: CatBuiltin})
	r.Register(&Tool{Name: "b", Category: CatBuiltin})
	r.Disable("a")
	r.Disable("b")

	enabled := r.ListEnabled()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled tools, got %d", len(enabled))
	}
}

// ── UsageTracker: GetAllUsage empty ──────────────────────────────────────────

func TestGetAllUsage_NoRecords(t *testing.T) {
	tracker := NewUsageTracker()
	result := tracker.GetAllUsage("nonexistent")
	if result != nil {
		t.Errorf("expected nil for nonexistent user, got %v", result)
	}
}

// ── UsageTracker: GetUsage no records ────────────────────────────────────────

func TestGetUsage_NoRecords(t *testing.T) {
	tracker := NewUsageTracker()
	stats := tracker.GetUsage("nonexistent", "tool1")
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 calls, got %d", stats.TotalCalls)
	}
	if stats.ToolName != "tool1" {
		t.Errorf("expected tool1, got %q", stats.ToolName)
	}
}

// ── UsageTracker: mixed success/failure ──────────────────────────────────────

func TestGetUsage_MixedResults(t *testing.T) {
	tracker := NewUsageTracker()
	tracker.Record("user1", "tool1", 100*time.Millisecond, true)
	tracker.Record("user1", "tool1", 200*time.Millisecond, false)
	tracker.Record("user1", "tool1", 300*time.Millisecond, true)

	stats := tracker.GetUsage("user1", "tool1")
	if stats.TotalCalls != 3 {
		t.Errorf("expected 3 calls, got %d", stats.TotalCalls)
	}
	if stats.SuccessCalls != 2 {
		t.Errorf("expected 2 success, got %d", stats.SuccessCalls)
	}
	if stats.FailedCalls != 1 {
		t.Errorf("expected 1 failure, got %d", stats.FailedCalls)
	}
	if stats.AvgDuration != 200*time.Millisecond {
		t.Errorf("expected avg 200ms, got %v", stats.AvgDuration)
	}
}

// ── UsageStats.Format ────────────────────────────────────────────────────────

func TestUsageStatsFormat_ZeroCalls(t *testing.T) {
	stats := UsageStats{ToolName: "test", TotalCalls: 0}
	s := stats.Format()
	if !strings.Contains(s, "0 calls") {
		t.Errorf("Format with 0 calls: %q", s)
	}
}

func TestUsageStatsFormat_WithCalls(t *testing.T) {
	stats := UsageStats{
		ToolName:     "shell",
		TotalCalls:   10,
		SuccessCalls: 8,
		AvgDuration:  50 * time.Millisecond,
	}
	s := stats.Format()
	if !strings.Contains(s, "80% success") {
		t.Errorf("Format: %q", s)
	}
}