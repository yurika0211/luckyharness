package eval

import (
	"context"
	"os"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// EB-1: Evaluator Interface Tests
// ---------------------------------------------------------------------------

func TestAccuracyEvaluator(t *testing.T) {
	ev := &AccuracyEvaluator{}

	tests := []struct {
		name     string
		output   EvalOutput
		expected ExpectedOutput
		wantMin  float64 // minimum expected score
		wantMax  float64 // maximum expected score
	}{
		{
			name: "all substrings match",
			output: EvalOutput{Response: "The capital of France is Paris"},
			expected: ExpectedOutput{ResponseContains: []string{"capital", "Paris"}},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name: "partial match",
			output: EvalOutput{Response: "The capital of France is Lyon"},
			expected: ExpectedOutput{ResponseContains: []string{"capital", "Paris"}},
			wantMin: 0.49, wantMax: 0.51,
		},
		{
			name: "no match",
			output: EvalOutput{Response: "Germany is in Europe"},
			expected: ExpectedOutput{ResponseContains: []string{"capital", "Paris"}},
			wantMin: 0.0, wantMax: 0.0,
		},
		{
			name: "no expected substrings",
			output: EvalOutput{Response: "anything"},
			expected: ExpectedOutput{},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name: "case insensitive match",
			output: EvalOutput{Response: "THE CAPITAL IS PARIS"},
			expected: ExpectedOutput{ResponseContains: []string{"capital", "paris"}},
			wantMin: 1.0, wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := ev.Evaluate(context.Background(), EvalInput{}, tt.output, tt.expected)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score.Name != "accuracy" {
				t.Errorf("expected name=accuracy, got %s", score.Name)
			}
			if score.Value < tt.wantMin-0.01 || score.Value > tt.wantMax+0.01 {
				t.Errorf("expected score in [%.2f, %.2f], got %.3f", tt.wantMin, tt.wantMax, score.Value)
			}
		})
	}
}

func TestRelevanceEvaluator(t *testing.T) {
	ev := &RelevanceEvaluator{MinLength: 10}

	tests := []struct {
		name    string
		input   EvalInput
		output  EvalOutput
		wantMin float64
	}{
		{
			name:    "empty response",
			input:   EvalInput{Query: "what is Go"},
			output:  EvalOutput{Response: ""},
			wantMin: 0.0,
		},
		{
			name:    "short response",
			input:   EvalInput{Query: "what is Go"},
			output:  EvalOutput{Response: "hi"},
			wantMin: 0.5,
		},
		{
			name:    "good response with query overlap",
			input:   EvalInput{Query: "what is Go programming language"},
			output:  EvalOutput{Response: "Go is a programming language designed at Google"},
			wantMin: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := ev.Evaluate(context.Background(), tt.input, tt.output, ExpectedOutput{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score.Name != "relevance" {
				t.Errorf("expected name=relevance, got %s", score.Name)
			}
			if score.Value < tt.wantMin-0.01 {
				t.Errorf("expected score >= %.2f, got %.3f", tt.wantMin, score.Value)
			}
		})
	}
}

func TestLatencyEvaluator(t *testing.T) {
	ev := &LatencyEvaluator{}

	tests := []struct {
		name     string
		output   EvalOutput
		expected ExpectedOutput
		wantMin  float64
	}{
		{
			name:     "no constraint",
			output:   EvalOutput{Latency: 5 * time.Second},
			expected: ExpectedOutput{},
			wantMin:  1.0,
		},
		{
			name:     "within limit",
			output:   EvalOutput{Latency: 100 * time.Millisecond},
			expected: ExpectedOutput{MaxLatency: 200 * time.Millisecond},
			wantMin:  1.0,
		},
		{
			name:     "exceeds limit",
			output:   EvalOutput{Latency: 500 * time.Millisecond},
			expected: ExpectedOutput{MaxLatency: 200 * time.Millisecond},
			wantMin:  0.0, // will be < 1.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := ev.Evaluate(context.Background(), EvalInput{}, tt.output, tt.expected)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score.Value < tt.wantMin-0.01 {
				t.Errorf("expected score >= %.2f, got %.3f", tt.wantMin, score.Value)
			}
		})
	}
}

func TestTokenUsageEvaluator(t *testing.T) {
	ev := &TokenUsageEvaluator{}

	tests := []struct {
		name     string
		output   EvalOutput
		expected ExpectedOutput
		wantMin  float64
	}{
		{
			name:     "no budget",
			output:   EvalOutput{TokenUsage: TokenUsage{Total: 5000}},
			expected: ExpectedOutput{},
			wantMin:  1.0,
		},
		{
			name:     "within budget",
			output:   EvalOutput{TokenUsage: TokenUsage{Total: 500}},
			expected: ExpectedOutput{MaxTokens: 1000},
			wantMin:  1.0,
		},
		{
			name:     "exceeds budget",
			output:   EvalOutput{TokenUsage: TokenUsage{Total: 2000}},
			expected: ExpectedOutput{MaxTokens: 1000},
			wantMin:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := ev.Evaluate(context.Background(), EvalInput{}, tt.output, tt.expected)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score.Value < tt.wantMin-0.01 {
				t.Errorf("expected score >= %.2f, got %.3f", tt.wantMin, score.Value)
			}
		})
	}
}

func TestToolCallAccuracyEvaluator(t *testing.T) {
	ev := &ToolCallAccuracyEvaluator{}

	tests := []struct {
		name     string
		output   EvalOutput
		expected ExpectedOutput
		wantMin  float64
		wantMax  float64
	}{
		{
			name: "no expected tools",
			output: EvalOutput{ToolsUsed: []ToolCallRecord{{Name: "search"}}},
			expected: ExpectedOutput{},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name: "all expected tools called",
			output: EvalOutput{ToolsUsed: []ToolCallRecord{
				{Name: "search"}, {Name: "calculate"},
			}},
			expected: ExpectedOutput{ToolsExpected: []string{"search", "calculate"}},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name: "partial tool match",
			output: EvalOutput{ToolsUsed: []ToolCallRecord{{Name: "search"}}},
			expected: ExpectedOutput{ToolsExpected: []string{"search", "calculate"}},
			wantMin: 0.49, wantMax: 0.51,
		},
		{
			name: "no expected tools called",
			output: EvalOutput{ToolsUsed: []ToolCallRecord{{Name: "other"}}},
			expected: ExpectedOutput{ToolsExpected: []string{"search", "calculate"}},
			wantMin: 0.0, wantMax: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := ev.Evaluate(context.Background(), EvalInput{}, tt.output, tt.expected)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score.Value < tt.wantMin-0.01 || score.Value > tt.wantMax+0.01 {
				t.Errorf("expected score in [%.2f, %.2f], got %.3f", tt.wantMin, tt.wantMax, score.Value)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// EB-3: BenchmarkRunner Tests
// ---------------------------------------------------------------------------

// mockAgentRunner is a test implementation of AgentRunner.
type mockAgentRunner struct {
	response string
	err      error
	latency  time.Duration
	tokens   TokenUsage
	tools    []ToolCallRecord
}

func (m *mockAgentRunner) Run(_ context.Context, _ EvalInput) (EvalOutput, error) {
	if m.err != nil {
		return EvalOutput{}, m.err
	}
	return EvalOutput{
		Response:   m.response,
		Latency:    m.latency,
		TokenUsage: m.tokens,
		ToolsUsed:  m.tools,
	}, nil
}

func TestBenchmarkRunner(t *testing.T) {
	runner := &mockAgentRunner{
		response: "The capital of France is Paris",
		latency:  100 * time.Millisecond,
		tokens:   TokenUsage{Prompt: 50, Completion: 20, Total: 70},
		tools:    []ToolCallRecord{{Name: "search", Success: true}},
	}

	br := NewBenchmarkRunner(runner, 0.7)

	cases := []TestCase{
		{
			ID:   "tc-1",
			Name: "Capital query",
			Input: EvalInput{Query: "What is the capital of France?"},
			Expected: ExpectedOutput{
				ResponseContains: []string{"Paris"},
				ToolsExpected:    []string{"search"},
				MaxLatency:       200 * time.Millisecond,
				MaxTokens:        100,
			},
		},
		{
			ID:   "tc-2",
			Name: "Math query",
			Input: EvalInput{Query: "What is 2+2?"},
			Expected: ExpectedOutput{
				ResponseContains: []string{"4"},
				MaxTokens:        50, // will exceed
			},
		},
	}

	result := br.Run(context.Background(), cases)

	if result.TotalCases != 2 {
		t.Errorf("expected 2 total cases, got %d", result.TotalCases)
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
	if result.PassRate < 0 || result.PassRate > 1 {
		t.Errorf("pass rate out of range: %.2f", result.PassRate)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestBenchmarkRunnerAgentError(t *testing.T) {
	runner := &mockAgentRunner{err: context.DeadlineExceeded}
	br := NewBenchmarkRunner(runner, 0.7)

	cases := []TestCase{
		{ID: "err-1", Name: "error case", Input: EvalInput{Query: "test"}},
	}

	result := br.Run(context.Background(), cases)

	if result.TotalCases != 1 {
		t.Errorf("expected 1 total case, got %d", result.TotalCases)
	}
	if result.PassedCases != 0 {
		t.Errorf("expected 0 passed, got %d", result.PassedCases)
	}
	if result.FailedCases != 1 {
		t.Errorf("expected 1 failed, got %d", result.FailedCases)
	}
	if result.Results[0].Error == "" {
		t.Error("expected error message in result")
	}
}

func TestBenchmarkRunnerSingle(t *testing.T) {
	runner := &mockAgentRunner{
		response: "Hello world",
		latency:  50 * time.Millisecond,
		tokens:   TokenUsage{Total: 30},
	}
	br := NewBenchmarkRunner(runner, 0.5)

	tc := TestCase{
		ID:   "single-1",
		Name: "Single test",
		Input: EvalInput{Query: "Say hello"},
		Expected: ExpectedOutput{
			ResponseContains: []string{"Hello"},
		},
	}

	tcr := br.RunSingle(context.Background(), tc)

	if tcr.TestCaseID != "single-1" {
		t.Errorf("expected test case ID single-1, got %s", tcr.TestCaseID)
	}
	if !tcr.Pass {
		t.Errorf("expected pass, got fail (score=%.3f)", tcr.WeightedScore)
	}
}

func TestBenchmarkRunnerCustomEvaluators(t *testing.T) {
	runner := &mockAgentRunner{response: "test"}
	br := NewBenchmarkRunner(runner, 0.5)

	// Replace with a single custom evaluator
	br.SetEvaluators([]Evaluator{
		&AccuracyEvaluator{}, // only accuracy
	})

	tc := TestCase{
		ID:   "custom-1",
		Name: "Custom evaluator test",
		Input: EvalInput{Query: "test"},
		Expected: ExpectedOutput{
			ResponseContains: []string{"test"},
		},
	}

	tcr := br.RunSingle(context.Background(), tc)

	if len(tcr.Scores) != 1 {
		t.Errorf("expected 1 score, got %d", len(tcr.Scores))
	}
	if tcr.Scores[0].Name != "accuracy" {
		t.Errorf("expected accuracy score, got %s", tcr.Scores[0].Name)
	}
}

// ---------------------------------------------------------------------------
// EB-4: Test Case Loading Tests
// ---------------------------------------------------------------------------

func TestLoadTestCasesFromFile(t *testing.T) {
	// Create a temp YAML file
	tmpDir := t.TempDir()
	yamlContent := `- id: test-1
  name: Test Case 1
  input:
    query: "What is Go?"
  expected:
    responseContains:
      - "programming"
      - "language"
- id: test-2
  name: Test Case 2
  input:
    query: "What is 2+2?"
  expected:
    responseContains:
      - "4"
`
	path := tmpDir + "/cases.yaml"
	if err := writeFile(path, yamlContent); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cases, err := LoadTestCasesFromFile(path)
	if err != nil {
		t.Fatalf("LoadTestCasesFromFile: %v", err)
	}

	if len(cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(cases))
	}
	if cases[0].ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", cases[0].ID)
	}
	if cases[0].Input.Query != "What is Go?" {
		t.Errorf("expected query 'What is Go?', got %s", cases[0].Input.Query)
	}
	if len(cases[0].Expected.ResponseContains) != 2 {
		t.Errorf("expected 2 responseContains, got %d", len(cases[0].Expected.ResponseContains))
	}
}

func TestLoadTestCasesFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two YAML files
	yaml1 := `- id: dir-1
  name: Dir Test 1
  input:
    query: "test1"
  expected: {}
`
	yaml2 := `- id: dir-2
  name: Dir Test 2
  input:
    query: "test2"
  expected: {}
`
	if err := writeFile(tmpDir+"/a.yaml", yaml1); err != nil {
		t.Fatalf("write a.yaml: %v", err)
	}
	if err := writeFile(tmpDir+"/b.yml", yaml2); err != nil {
		t.Fatalf("write b.yml: %v", err)
	}
	// Non-YAML file should be ignored
	if err := writeFile(tmpDir+"/c.txt", "not yaml"); err != nil {
		t.Fatalf("write c.txt: %v", err)
	}

	cases, err := LoadTestCasesFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadTestCasesFromDir: %v", err)
	}

	if len(cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(cases))
	}
}

func TestSaveAndLoadTestCases(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/saved.yaml"

	cases := []TestCase{
		{
			ID:   "save-1",
			Name: "Saved Test",
			Input: EvalInput{Query: "hello"},
			Expected: ExpectedOutput{
				ResponseContains: []string{"world"},
				MaxTokens:        100,
			},
			Tags: []string{"smoke"},
		},
	}

	if err := SaveTestCases(path, cases); err != nil {
		t.Fatalf("SaveTestCases: %v", err)
	}

	loaded, err := LoadTestCasesFromFile(path)
	if err != nil {
		t.Fatalf("LoadTestCasesFromFile: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 case, got %d", len(loaded))
	}
	if loaded[0].ID != "save-1" {
		t.Errorf("expected ID save-1, got %s", loaded[0].ID)
	}
	if loaded[0].Input.Query != "hello" {
		t.Errorf("expected query 'hello', got %s", loaded[0].Input.Query)
	}
}

// ---------------------------------------------------------------------------
// Report Generation Tests
// ---------------------------------------------------------------------------

func TestGenerateReportText(t *testing.T) {
	result := &BenchmarkResult{
		ID:          "test-report",
		Name:        "Test Benchmark",
		Duration:    1 * time.Second,
		TotalCases:  2,
		PassedCases: 1,
		FailedCases: 1,
		PassRate:    0.5,
		AvgScore:    0.75,
		Results: []TestCaseResult{
			{
				TestCaseID:    "tc-1",
				WeightedScore: 0.9,
				Pass:          true,
				Scores: []Score{
					{Name: "accuracy", Value: 1.0, Weight: 1.0, Reasoning: "all matched"},
				},
			},
			{
				TestCaseID:    "tc-2",
				WeightedScore: 0.6,
				Pass:          false,
				Scores: []Score{
					{Name: "accuracy", Value: 0.5, Weight: 1.0, Reasoning: "partial match"},
				},
			},
		},
	}

	report, err := GenerateReport(result, ReportText)
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}

	if len(report) == 0 {
		t.Error("expected non-empty report")
	}
	// Check key content
	if !contains(report, "Test Benchmark") {
		t.Error("report missing benchmark name")
	}
	if !contains(report, "50.0%") {
		t.Error("report missing pass rate")
	}
}

func TestGenerateReportJSON(t *testing.T) {
	result := &BenchmarkResult{
		ID:         "json-test",
		Name:       "JSON Benchmark",
		TotalCases: 1,
		PassRate:   1.0,
		AvgScore:   0.95,
	}

	report, err := GenerateReport(result, ReportJSON)
	if err != nil {
		t.Fatalf("GenerateReport JSON: %v", err)
	}

	if !contains(report, "json-test") {
		t.Error("JSON report missing ID")
	}
}

func TestGenerateReportYAML(t *testing.T) {
	result := &BenchmarkResult{
		ID:         "yaml-test",
		Name:       "YAML Benchmark",
		TotalCases: 1,
		PassRate:   1.0,
		AvgScore:   0.95,
	}

	report, err := GenerateReport(result, ReportYAML)
	if err != nil {
		t.Fatalf("GenerateReport YAML: %v", err)
	}

	if !contains(report, "yaml-test") {
		t.Error("YAML report missing ID")
	}
}

// ---------------------------------------------------------------------------
// Helper Tests
// ---------------------------------------------------------------------------

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "HELLO", true},
		{"Hello", "xyz", false},
		{"", "", true},
		{"abc", "", true},
		{"", "abc", false},
	}

	for _, tt := range tests {
		got := containsIgnoreCase(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestExtractSignificantWords(t *testing.T) {
	words := extractSignificantWords("What is the Go programming language")
	if len(words) == 0 {
		t.Error("expected non-empty words")
	}

	// "is" is too short (< 3 chars), should be filtered
	for _, w := range words {
		if w == "is" {
			t.Errorf("short word should be filtered: %s", w)
		}
	}
	// "the" is exactly 3 chars, so it passes the >= 3 filter — that's fine
}

// ---------------------------------------------------------------------------
// Test Helpers
// ---------------------------------------------------------------------------

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}