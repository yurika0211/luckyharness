// Package eval provides an evaluation and benchmarking framework for LuckyHarness.
// It supports defining test cases, running evaluations against agent outputs,
// collecting metrics, and generating reports.
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// EB-1: Evaluator Interface
// ---------------------------------------------------------------------------

// Score represents a single evaluation score.
type Score struct {
	Name      string  `json:"name" yaml:"name"`           // e.g. "accuracy", "relevance"
	Value     float64 `json:"value" yaml:"value"`          // 0.0 – 1.0
	Weight    float64 `json:"weight,omitempty" yaml:"weight,omitempty"` // default 1.0
	Reasoning string  `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
}

// EvalInput is the input to an evaluator.
type EvalInput struct {
	Query       string                 `json:"query" yaml:"query"`
	Context     map[string]interface{} `json:"context,omitempty" yaml:"context,omitempty"`
	ToolsCalled []ToolCallRecord       `json:"toolsCalled,omitempty" yaml:"toolsCalled,omitempty"`
}

// ToolCallRecord records a single tool call made during evaluation.
type ToolCallRecord struct {
	Name       string                 `json:"name" yaml:"name"`
	Params     map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
	Result     interface{}            `json:"result,omitempty" yaml:"result,omitempty"`
	Success    bool                   `json:"success" yaml:"success"`
	Duration   time.Duration          `json:"duration,omitempty" yaml:"duration,omitempty"`
}

// EvalOutput is the agent's output being evaluated.
type EvalOutput struct {
	Response   string                 `json:"response" yaml:"response"`
	ToolsUsed  []ToolCallRecord       `json:"toolsUsed,omitempty" yaml:"toolsUsed,omitempty"`
	TokenUsage TokenUsage             `json:"tokenUsage" yaml:"tokenUsage"`
	Latency    time.Duration          `json:"latency" yaml:"latency"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	Prompt     int `json:"prompt" yaml:"prompt"`
	Completion int `json:"completion" yaml:"completion"`
	Total      int `json:"total" yaml:"total"`
}

// ExpectedOutput describes what we expect from the agent.
type ExpectedOutput struct {
	ResponseContains []string               `json:"responseContains,omitempty" yaml:"responseContains,omitempty"`
	ResponseRegex   string                 `json:"responseRegex,omitempty" yaml:"responseRegex,omitempty"`
	ToolsExpected   []string               `json:"toolsExpected,omitempty" yaml:"toolsExpected,omitempty"`
	MaxLatency      time.Duration          `json:"maxLatency,omitempty" yaml:"maxLatency,omitempty"`
	MaxTokens       int                    `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`
	Custom          map[string]interface{} `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// Evaluator is the core interface for evaluating agent outputs.
type Evaluator interface {
	// Name returns the evaluator's identifier.
	Name() string
	// Evaluate scores an output against expected results.
	Evaluate(ctx context.Context, input EvalInput, output EvalOutput, expected ExpectedOutput) (Score, error)
}

// ---------------------------------------------------------------------------
// EB-2: Built-in Evaluators (Metrics)
// ---------------------------------------------------------------------------

// AccuracyEvaluator checks if the response contains expected substrings.
type AccuracyEvaluator struct{}

func (e *AccuracyEvaluator) Name() string { return "accuracy" }

func (e *AccuracyEvaluator) Evaluate(_ context.Context, _ EvalInput, output EvalOutput, expected ExpectedOutput) (Score, error) {
	if len(expected.ResponseContains) == 0 {
		return Score{Name: "accuracy", Value: 1.0, Weight: 1.0, Reasoning: "no expected substrings defined"}, nil
	}

	matched := 0
	for _, sub := range expected.ResponseContains {
		if containsIgnoreCase(output.Response, sub) {
			matched++
		}
	}

	value := float64(matched) / float64(len(expected.ResponseContains))
	reasoning := fmt.Sprintf("matched %d/%d expected substrings", matched, len(expected.ResponseContains))
	return Score{Name: "accuracy", Value: value, Weight: 1.0, Reasoning: reasoning}, nil
}

// RelevanceEvaluator checks if the response is non-empty and reasonably sized.
type RelevanceEvaluator struct {
	MinLength int // minimum response length to be considered relevant
}

func (e *RelevanceEvaluator) Name() string { return "relevance" }

func (e *RelevanceEvaluator) Evaluate(_ context.Context, input EvalInput, output EvalOutput, _ ExpectedOutput) (Score, error) {
	minLen := e.MinLength
	if minLen == 0 {
		minLen = 10
	}

	resp := output.Response
	if len(resp) == 0 {
		return Score{Name: "relevance", Value: 0.0, Weight: 1.0, Reasoning: "empty response"}, nil
	}

	// Basic heuristic: response should be at least minLen chars and reference the query
	score := 0.5 // base score for non-empty
	if len(resp) >= minLen {
		score += 0.3
	}
	// Check if any significant word from the query appears in the response
	queryWords := extractSignificantWords(input.Query)
	matchedWords := 0
	for _, w := range queryWords {
		if containsIgnoreCase(resp, w) {
			matchedWords++
		}
	}
	if len(queryWords) > 0 && matchedWords > 0 {
		score += 0.2 * (float64(matchedWords) / float64(len(queryWords)))
	}
	if score > 1.0 {
		score = 1.0
	}

	reasoning := fmt.Sprintf("response length=%d, query word overlap=%d/%d", len(resp), matchedWords, len(queryWords))
	return Score{Name: "relevance", Value: score, Weight: 1.0, Reasoning: reasoning}, nil
}

// LatencyEvaluator checks if the response time is within acceptable bounds.
type LatencyEvaluator struct{}

func (e *LatencyEvaluator) Name() string { return "latency" }

func (e *LatencyEvaluator) Evaluate(_ context.Context, _ EvalInput, output EvalOutput, expected ExpectedOutput) (Score, error) {
	if expected.MaxLatency == 0 {
		return Score{Name: "latency", Value: 1.0, Weight: 0.5, Reasoning: "no latency constraint defined"}, nil
	}

	if output.Latency <= expected.MaxLatency {
		reasoning := fmt.Sprintf("latency %v within limit %v", output.Latency, expected.MaxLatency)
		return Score{Name: "latency", Value: 1.0, Weight: 0.5, Reasoning: reasoning}, nil
	}

	// Linear decay: score decreases as latency exceeds the limit
	ratio := float64(expected.MaxLatency) / float64(output.Latency)
	reasoning := fmt.Sprintf("latency %v exceeds limit %v (ratio=%.2f)", output.Latency, expected.MaxLatency, ratio)
	return Score{Name: "latency", Value: ratio, Weight: 0.5, Reasoning: reasoning}, nil
}

// TokenUsageEvaluator checks if token consumption is within budget.
type TokenUsageEvaluator struct{}

func (e *TokenUsageEvaluator) Name() string { return "token_usage" }

func (e *TokenUsageEvaluator) Evaluate(_ context.Context, _ EvalInput, output EvalOutput, expected ExpectedOutput) (Score, error) {
	if expected.MaxTokens == 0 {
		return Score{Name: "token_usage", Value: 1.0, Weight: 0.3, Reasoning: "no token budget defined"}, nil
	}

	if output.TokenUsage.Total <= expected.MaxTokens {
		reasoning := fmt.Sprintf("token usage %d within budget %d", output.TokenUsage.Total, expected.MaxTokens)
		return Score{Name: "token_usage", Value: 1.0, Weight: 0.3, Reasoning: reasoning}, nil
	}

	ratio := float64(expected.MaxTokens) / float64(output.TokenUsage.Total)
	reasoning := fmt.Sprintf("token usage %d exceeds budget %d (ratio=%.2f)", output.TokenUsage.Total, expected.MaxTokens, ratio)
	return Score{Name: "token_usage", Value: ratio, Weight: 0.3, Reasoning: reasoning}, nil
}

// ToolCallAccuracyEvaluator checks if the expected tools were called.
type ToolCallAccuracyEvaluator struct{}

func (e *ToolCallAccuracyEvaluator) Name() string { return "tool_call_accuracy" }

func (e *ToolCallAccuracyEvaluator) Evaluate(_ context.Context, _ EvalInput, output EvalOutput, expected ExpectedOutput) (Score, error) {
	if len(expected.ToolsExpected) == 0 {
		return Score{Name: "tool_call_accuracy", Value: 1.0, Weight: 0.8, Reasoning: "no expected tools defined"}, nil
	}

	// Build set of tools actually called
	calledTools := make(map[string]bool)
	for _, tc := range output.ToolsUsed {
		calledTools[tc.Name] = true
	}

	matched := 0
	for _, tool := range expected.ToolsExpected {
		if calledTools[tool] {
			matched++
		}
	}

	value := float64(matched) / float64(len(expected.ToolsExpected))
	reasoning := fmt.Sprintf("called %d/%d expected tools", matched, len(expected.ToolsExpected))
	return Score{Name: "tool_call_accuracy", Value: value, Weight: 0.8, Reasoning: reasoning}, nil
}

// ---------------------------------------------------------------------------
// EB-3: BenchmarkRunner
// ---------------------------------------------------------------------------

// TestCase is a single evaluation test case.
type TestCase struct {
	ID          string         `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Input       EvalInput      `json:"input" yaml:"input"`
	Expected    ExpectedOutput `json:"expected" yaml:"expected"`
	Tags        []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// TestCaseResult holds the result of evaluating a single test case.
type TestCaseResult struct {
	TestCaseID string  `json:"testCaseId" yaml:"testCaseId"`
	Scores     []Score `json:"scores" yaml:"scores"`
	WeightedScore float64 `json:"weightedScore" yaml:"weightedScore"`
	Pass      bool    `json:"pass" yaml:"pass"`
	Error     string  `json:"error,omitempty" yaml:"error,omitempty"`
	Duration  time.Duration `json:"duration" yaml:"duration"`
}

// BenchmarkResult holds the aggregate result of a benchmark run.
type BenchmarkResult struct {
	ID          string           `json:"id" yaml:"id"`
	Name        string           `json:"name" yaml:"name"`
	StartTime   time.Time        `json:"startTime" yaml:"startTime"`
	EndTime     time.Time        `json:"endTime" yaml:"endTime"`
	Duration    time.Duration    `json:"duration" yaml:"duration"`
	TotalCases  int              `json:"totalCases" yaml:"totalCases"`
	PassedCases int              `json:"passedCases" yaml:"passedCases"`
	FailedCases int              `json:"failedCases" yaml:"failedCases"`
	Results     []TestCaseResult `json:"results" yaml:"results"`
	AvgScore    float64          `json:"avgScore" yaml:"avgScore"`
	PassRate    float64          `json:"passRate" yaml:"passRate"`
}

// AgentRunner is the interface for running an agent against a test input.
// The caller provides an implementation that connects to the actual agent.
type AgentRunner interface {
	// Run executes the agent with the given input and returns its output.
	Run(ctx context.Context, input EvalInput) (EvalOutput, error)
}

// BenchmarkRunner orchestrates evaluation runs.
type BenchmarkRunner struct {
	evaluators []Evaluator
	runner     AgentRunner
	passThreshold float64 // weighted score threshold to pass (0.0–1.0)
}

// NewBenchmarkRunner creates a new benchmark runner.
func NewBenchmarkRunner(runner AgentRunner, passThreshold float64) *BenchmarkRunner {
	if passThreshold <= 0 {
		passThreshold = 0.7
	}
	return &BenchmarkRunner{
		evaluators:    defaultEvaluators(),
		runner:        runner,
		passThreshold: passThreshold,
	}
}

// AddEvaluator adds a custom evaluator.
func (br *BenchmarkRunner) AddEvaluator(e Evaluator) {
	br.evaluators = append(br.evaluators, e)
}

// SetEvaluators replaces all evaluators.
func (br *BenchmarkRunner) SetEvaluators(evaluators []Evaluator) {
	br.evaluators = evaluators
}

// Run executes all test cases and returns the benchmark result.
func (br *BenchmarkRunner) Run(ctx context.Context, cases []TestCase) *BenchmarkResult {
	result := &BenchmarkResult{
		ID:        uuid.New().String(),
		StartTime: time.Now(),
		TotalCases: len(cases),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, tc := range cases {
		wg.Add(1)
		go func(tc TestCase) {
			defer wg.Done()
			tcr := br.runCase(ctx, tc)
			mu.Lock()
			result.Results = append(result.Results, *tcr)
			mu.Unlock()
		}(tc)
	}

	wg.Wait()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Compute aggregates
	totalScore := 0.0
	for _, tcr := range result.Results {
		if tcr.Pass {
			result.PassedCases++
		} else {
			result.FailedCases++
		}
		totalScore += tcr.WeightedScore
	}

	if len(result.Results) > 0 {
		result.AvgScore = totalScore / float64(len(result.Results))
	}
	if result.TotalCases > 0 {
		result.PassRate = float64(result.PassedCases) / float64(result.TotalCases)
	}

	return result
}

// RunSingle executes a single test case.
func (br *BenchmarkRunner) RunSingle(ctx context.Context, tc TestCase) *TestCaseResult {
	return br.runCase(ctx, tc)
}

func (br *BenchmarkRunner) runCase(ctx context.Context, tc TestCase) *TestCaseResult {
	start := time.Now()
	tcr := &TestCaseResult{TestCaseID: tc.ID}

	// Run the agent
	output, err := br.runner.Run(ctx, tc.Input)
	if err != nil {
		tcr.Error = err.Error()
		tcr.Pass = false
		tcr.Duration = time.Since(start)
		return tcr
	}

	// Run all evaluators
	totalWeight := 0.0
	weightedSum := 0.0
	for _, ev := range br.evaluators {
		score, err := ev.Evaluate(ctx, tc.Input, output, tc.Expected)
		if err != nil {
			tcr.Scores = append(tcr.Scores, Score{
				Name:      ev.Name(),
				Value:     0.0,
				Weight:    1.0,
				Reasoning: fmt.Sprintf("evaluator error: %v", err),
			})
			continue
		}
		tcr.Scores = append(tcr.Scores, score)
		w := score.Weight
		if w == 0 {
			w = 1.0
		}
		weightedSum += score.Value * w
		totalWeight += w
	}

	if totalWeight > 0 {
		tcr.WeightedScore = weightedSum / totalWeight
	}
	tcr.Pass = tcr.WeightedScore >= br.passThreshold
	tcr.Duration = time.Since(start)

	return tcr
}

// ---------------------------------------------------------------------------
// EB-4: Test Case Format (YAML loading)
// ---------------------------------------------------------------------------

// LoadTestCasesFromDir loads all YAML test case files from a directory.
func LoadTestCasesFromDir(dir string) ([]TestCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var cases []TestCase
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		fileCases, err := LoadTestCasesFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		cases = append(cases, fileCases...)
	}

	return cases, nil
}

// LoadTestCasesFromFile loads test cases from a single YAML file.
// The file can contain a single TestCase or a list of TestCase.
func LoadTestCasesFromFile(path string) ([]TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	// Try as list first
	var cases []TestCase
	if err := yaml.Unmarshal(data, &cases); err == nil && len(cases) > 0 && cases[0].ID != "" {
		return cases, nil
	}

	// Try as single case
	var tc TestCase
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	if tc.ID == "" {
		tc.ID = uuid.New().String()
	}

	return []TestCase{tc}, nil
}

// SaveTestCases saves test cases to a YAML file.
func SaveTestCases(path string, cases []TestCase) error {
	data, err := yaml.Marshal(cases)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ---------------------------------------------------------------------------
// Report Generation
// ---------------------------------------------------------------------------

// ReportFormat defines the output format for benchmark reports.
type ReportFormat string

const (
	ReportJSON ReportFormat = "json"
	ReportYAML ReportFormat = "yaml"
	ReportText ReportFormat = "text"
)

// GenerateReport generates a benchmark report in the specified format.
func GenerateReport(result *BenchmarkResult, format ReportFormat) (string, error) {
	switch format {
	case ReportJSON:
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal json: %w", err)
		}
		return string(data), nil

	case ReportYAML:
		data, err := yaml.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("marshal yaml: %w", err)
		}
		return string(data), nil

	case ReportText:
		return generateTextReport(result), nil

	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}

// SaveReport saves a benchmark report to a file.
func SaveReport(path string, result *BenchmarkResult, format ReportFormat) error {
	content, err := GenerateReport(result, format)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func generateTextReport(r *BenchmarkResult) string {
	report := fmt.Sprintf("Benchmark Report: %s\n", r.Name)
	report += fmt.Sprintf("ID: %s\n", r.ID)
	report += fmt.Sprintf("Duration: %v\n", r.Duration)
	report += fmt.Sprintf("Total: %d | Passed: %d | Failed: %d\n", r.TotalCases, r.PassedCases, r.FailedCases)
	report += fmt.Sprintf("Pass Rate: %.1f%% | Avg Score: %.3f\n\n", r.PassRate*100, r.AvgScore)

	for _, tcr := range r.Results {
		status := "✅ PASS"
		if !tcr.Pass {
			status = "❌ FAIL"
		}
		report += fmt.Sprintf("  %s [%s] score=%.3f duration=%v\n", status, tcr.TestCaseID, tcr.WeightedScore, tcr.Duration)
		if tcr.Error != "" {
			report += fmt.Sprintf("    Error: %s\n", tcr.Error)
		}
		for _, s := range tcr.Scores {
			report += fmt.Sprintf("    %-20s %.3f (weight=%.1f) %s\n", s.Name+":", s.Value, s.Weight, s.Reasoning)
		}
		report += "\n"
	}

	return report
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defaultEvaluators() []Evaluator {
	return []Evaluator{
		&AccuracyEvaluator{},
		&RelevanceEvaluator{},
		&LatencyEvaluator{},
		&TokenUsageEvaluator{},
		&ToolCallAccuracyEvaluator{},
	}
}

func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	subLower := toLower(substr)
	return contains(sLower, subLower)
}

func toLower(s string) string {
	// Simple ASCII lowercase; sufficient for evaluation matching
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractSignificantWords(query string) []string {
	// Simple word extraction: split by spaces, filter short words
	words := make(map[string]bool)
	start := -1
	for i := 0; i <= len(query); i++ {
		if i < len(query) && query[i] != ' ' {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 {
				word := query[start:i]
				if len(word) >= 3 {
					words[toLower(word)] = true
				}
				start = -1
			}
		}
	}

	result := make([]string, 0, len(words))
	for w := range words {
		result = append(result, w)
	}
	return result
}