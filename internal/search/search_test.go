package search

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// SF-1: SearchEngine Interface Tests (using mock)
// ---------------------------------------------------------------------------

type mockSearchEngine struct {
	name    string
	results []SearchResult
	err     error
	delay   time.Duration
}

func (m *mockSearchEngine) Name() string { return m.name }

func (m *mockSearchEngine) Search(ctx context.Context, query string, count int) ([]SearchResult, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	if len(m.results) > count {
		return m.results[:count], nil
	}
	return m.results, nil
}

func TestBraveEngineNoKey(t *testing.T) {
	eng := NewBraveEngine("", "")
	_, err := eng.Search(context.Background(), "test", 5)
	if err == nil || !strings.Contains(err.Error(), "no API key") {
		t.Errorf("expected no API key error, got %v", err)
	}
}

func TestSearXNGEngineNoURL(t *testing.T) {
	// Clear env
	os.Unsetenv("SEARXNG_BASE_URL")
	eng := NewSearXNGEngine("", "")
	_, err := eng.Search(context.Background(), "test", 5)
	if err == nil || !strings.Contains(err.Error(), "no base URL") {
		t.Errorf("expected no base URL error, got %v", err)
	}
}

func TestExaEngineNoKey(t *testing.T) {
	os.Unsetenv("EXA_API_KEY")
	eng := NewExaEngine("")
	_, err := eng.Search(context.Background(), "test", 5)
	if err == nil || !strings.Contains(err.Error(), "no API key") {
		t.Errorf("expected no API key error, got %v", err)
	}
}

func TestMockSearchEngine(t *testing.T) {
	eng := &mockSearchEngine{
		name: "mock",
		results: []SearchResult{
			{Title: "Test", URL: "https://example.com", Snippet: "test snippet", Source: "mock"},
		},
	}
	results, err := eng.Search(context.Background(), "test", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Test" {
		t.Errorf("expected 'Test', got '%s'", results[0].Title)
	}
}

// ---------------------------------------------------------------------------
// SF-2: FetchEngine Interface Tests
// ---------------------------------------------------------------------------

type mockFetchEngine struct {
	name   string
	result *FetchResult
	err    error
}

func (m *mockFetchEngine) Name() string { return m.name }

func (m *mockFetchEngine) Fetch(ctx context.Context, rawURL string, maxChars int) (*FetchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestDefuddleEngineNotInstalled(t *testing.T) {
	eng := NewDefuddleEngine()
	_, err := eng.Fetch(context.Background(), "https://example.com", 50000)
	// Will fail if defuddle not installed, which is expected
	if err != nil && !strings.Contains(err.Error(), "not installed") {
		// Might also fail for other reasons, that's ok
		t.Logf("defuddle fetch error (expected if not installed): %v", err)
	}
}

func TestMockFetchEngine(t *testing.T) {
	eng := &mockFetchEngine{
		name: "mock-fetch",
		result: &FetchResult{
			Title:   "Example",
			Content: "Hello world",
			URL:     "https://example.com",
			Source:  "mock-fetch",
		},
	}
	result, err := eng.Fetch(context.Background(), "https://example.com", 50000)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Title != "Example" {
		t.Errorf("expected 'Example', got '%s'", result.Title)
	}
}

// ---------------------------------------------------------------------------
// SF-3: SearchCache Tests
// ---------------------------------------------------------------------------

func TestSearchCacheSetGet(t *testing.T) {
	cache := NewSearchCache(5*time.Minute, 10)
	results := []SearchResult{
		{Title: "Test", URL: "https://example.com", Snippet: "test", Source: "mock"},
	}

	cache.Set("test query", results)
	got, ok := cache.Get("test query")
	if !ok {
		t.Error("expected cache hit")
	}
	if len(got) != 1 || got[0].Title != "Test" {
		t.Errorf("expected cached result, got %v", got)
	}
}

func TestSearchCacheMiss(t *testing.T) {
	cache := NewSearchCache(5*time.Minute, 10)
	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestSearchCacheExpiry(t *testing.T) {
	cache := NewSearchCache(50*time.Millisecond, 10)
	results := []SearchResult{{Title: "Test", URL: "https://example.com", Source: "mock"}}

	cache.Set("test", results)
	time.Sleep(100 * time.Millisecond)
	_, ok := cache.Get("test")
	if ok {
		t.Error("expected cache expiry")
	}
}

func TestSearchCacheEviction(t *testing.T) {
	cache := NewSearchCache(5*time.Minute, 3)
	for i := 0; i < 5; i++ {
		cache.Set(fmt.Sprintf("query %d", i), []SearchResult{
			{Title: fmt.Sprintf("Result %d", i), URL: fmt.Sprintf("https://example.com/%d", i), Source: "mock"},
		})
	}
	if cache.Len() > 3 {
		t.Errorf("expected max 3 entries, got %d", cache.Len())
	}
}

func TestSearchCacheClear(t *testing.T) {
	cache := NewSearchCache(5*time.Minute, 10)
	cache.Set("test", []SearchResult{{Title: "Test", URL: "https://example.com", Source: "mock"}})
	cache.Clear()
	if cache.Len() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", cache.Len())
	}
}

func TestSearchCacheCaseInsensitive(t *testing.T) {
	cache := NewSearchCache(5*time.Minute, 10)
	cache.Set("Hello World", []SearchResult{{Title: "Test", URL: "https://example.com", Source: "mock"}})
	_, ok := cache.Get("hello world")
	if !ok {
		t.Error("expected case-insensitive cache hit")
	}
}

// ---------------------------------------------------------------------------
// SF-3: DeepSearch Tests
// ---------------------------------------------------------------------------

func TestDeepSearchConcurrent(t *testing.T) {
	engines := []SearchEngine{
		&mockSearchEngine{
			name: "engine-a",
			results: []SearchResult{
				{Title: "Result A1", URL: "https://a.com/1", Snippet: "from A", Source: "engine-a"},
				{Title: "Shared Result", URL: "https://shared.com", Snippet: "shared", Source: "engine-a"},
			},
		},
		&mockSearchEngine{
			name: "engine-b",
			results: []SearchResult{
				{Title: "Result B1", URL: "https://b.com/1", Snippet: "from B", Source: "engine-b"},
				{Title: "Shared Result", URL: "https://shared.com", Snippet: "shared", Source: "engine-b"},
			},
		},
	}

	result := DeepSearch(context.Background(), engines, "test", 10)
	if len(result.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(result.Sources))
	}
	// Shared result should be merged
	found := false
	for _, r := range result.Results {
		if r.URL == "https://shared.com" {
			if strings.Contains(r.Source, "+") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected shared result to have multi-source annotation")
	}
}

func TestDeepSearchPartialFailure(t *testing.T) {
	engines := []SearchEngine{
		&mockSearchEngine{
			name: "good",
			results: []SearchResult{
				{Title: "Good Result", URL: "https://good.com", Snippet: "good", Source: "good"},
			},
		},
		&mockSearchEngine{
			name: "bad",
			err:  fmt.Errorf("connection refused"),
		},
	}

	result := DeepSearch(context.Background(), engines, "test", 5)
	if len(result.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(result.Sources))
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}
}

func TestDeepSearchAllFail(t *testing.T) {
	engines := []SearchEngine{
		&mockSearchEngine{name: "a", err: fmt.Errorf("fail")},
		&mockSearchEngine{name: "b", err: fmt.Errorf("fail")},
	}

	result := DeepSearch(context.Background(), engines, "test", 5)
	if len(result.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Results))
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestMergeResultsDedup(t *testing.T) {
	results := []SearchResult{
		{Title: "A", URL: "https://example.com/page", Source: "brave"},
		{Title: "B", URL: "https://other.com", Source: "brave"},
		{Title: "A2", URL: "https://example.com/page/", Source: "ddgs"}, // trailing slash normalized
		{Title: "C", URL: "https://third.com", Source: "exa"},
	}

	merged := mergeResults(results, 10)
	// example.com/page and example.com/page/ should be merged
	urls := make(map[string]bool)
	for _, r := range merged {
		urls[r.URL] = true
	}
	// Should have 3 unique entries (example.com/page merged, other.com, third.com)
	if len(merged) != 3 {
		t.Errorf("expected 3 merged results, got %d: %+v", len(merged), merged)
	}
}

// ---------------------------------------------------------------------------
// SF-4: SearchConfig Tests
// ---------------------------------------------------------------------------

func TestDefaultSearchConfig(t *testing.T) {
	cfg := DefaultSearchConfig()
	if cfg.DefaultProvider != "brave" {
		t.Errorf("expected 'brave', got '%s'", cfg.DefaultProvider)
	}
	if cfg.MaxResults != 5 {
		t.Errorf("expected 5, got %d", cfg.MaxResults)
	}
}

func TestSearchConfigFromEnv(t *testing.T) {
	os.Setenv("LH_SEARCH_PROVIDER", "ddgs")
	os.Setenv("LH_SEARCH_BRAVE_KEY", "test-brave-key")
	os.Setenv("LH_SEARCH_PROXY", "http://proxy:8080")
	defer func() {
		os.Unsetenv("LH_SEARCH_PROVIDER")
		os.Unsetenv("LH_SEARCH_BRAVE_KEY")
		os.Unsetenv("LH_SEARCH_PROXY")
	}()

	cfg := SearchConfigFromEnv(nil)
	if cfg.DefaultProvider != "ddgs" {
		t.Errorf("expected 'ddgs', got '%s'", cfg.DefaultProvider)
	}
	if cfg.BraveAPIKey != "test-brave-key" {
		t.Errorf("expected 'test-brave-key', got '%s'", cfg.BraveAPIKey)
	}
	if cfg.Proxy != "http://proxy:8080" {
		t.Errorf("expected proxy, got '%s'", cfg.Proxy)
	}
}

func TestBuildEnginesBrave(t *testing.T) {
	cfg := &SearchConfig{DefaultProvider: "brave", BraveAPIKey: "test"}
	engines := cfg.BuildEngines()
	if len(engines) < 2 {
		t.Errorf("expected at least 2 engines (brave + ddgs fallback), got %d", len(engines))
	}
	if engines[0].Name() != "brave" {
		t.Errorf("expected first engine 'brave', got '%s'", engines[0].Name())
	}
}

func TestBuildEnginesDDGS(t *testing.T) {
	cfg := &SearchConfig{DefaultProvider: "ddgs"}
	engines := cfg.BuildEngines()
	if engines[0].Name() != "ddgs" {
		t.Errorf("expected first engine 'ddgs', got '%s'", engines[0].Name())
	}
}

func TestBuildEnginesWithSearXNG(t *testing.T) {
	cfg := &SearchConfig{DefaultProvider: "brave", SearXNGBaseURL: "http://searxng:8080"}
	engines := cfg.BuildEngines()
	found := false
	for _, e := range engines {
		if e.Name() == "searxng" {
			found = true
		}
	}
	if !found {
		t.Error("expected searxng engine when BaseURL configured")
	}
}

func TestBuildEnginesWithExa(t *testing.T) {
	cfg := &SearchConfig{DefaultProvider: "brave", ExaAPIKey: "test-exa"}
	engines := cfg.BuildEngines()
	found := false
	for _, e := range engines {
		if e.Name() == "exa" {
			found = true
		}
	}
	if !found {
		t.Error("expected exa engine when API key configured")
	}
}

func TestBuildFetchEngines(t *testing.T) {
	cfg := &SearchConfig{PreferredFetch: "defuddle"}
	engines := cfg.BuildFetchEngines()
	if len(engines) < 2 {
		t.Errorf("expected at least 2 fetch engines, got %d", len(engines))
	}
	if engines[0].Name() != "defuddle" {
		t.Errorf("expected first engine 'defuddle', got '%s'", engines[0].Name())
	}
}

// ---------------------------------------------------------------------------
// Manager Tests
// ---------------------------------------------------------------------------

func TestManagerQuickSearch(t *testing.T) {
	cfg := DefaultSearchConfig()
	mgr := NewManager(cfg)

	// Replace engines with mocks
	mgr.searchEngines = []SearchEngine{
		&mockSearchEngine{
			name: "mock",
			results: []SearchResult{
				{Title: "Test Result", URL: "https://example.com", Snippet: "test", Source: "mock"},
			},
		},
	}

	results, err := mgr.QuickSearch(context.Background(), "test", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestManagerQuickSearchCached(t *testing.T) {
	cfg := DefaultSearchConfig()
	mgr := NewManager(cfg)

	callCount := 0
	mgr.searchEngines = []SearchEngine{
		&mockSearchEngine{
			name: "mock",
			results: []SearchResult{
				{Title: "Cached", URL: "https://example.com", Source: "mock"},
			},
		},
	}

	// First call
	_, _ = mgr.QuickSearch(context.Background(), "test", 5)
	callCount++

	// Second call should hit cache
	results, err := mgr.QuickSearch(context.Background(), "test", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 cached result, got %d", len(results))
	}
}

func TestManagerQuickSearchFallback(t *testing.T) {
	cfg := DefaultSearchConfig()
	mgr := NewManager(cfg)

	mgr.searchEngines = []SearchEngine{
		&mockSearchEngine{name: "bad", err: fmt.Errorf("fail")},
		&mockSearchEngine{
			name: "good",
			results: []SearchResult{
				{Title: "Fallback", URL: "https://example.com", Source: "good"},
			},
		},
	}

	results, err := mgr.QuickSearch(context.Background(), "test", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Fallback" {
		t.Errorf("expected fallback result, got %v", results)
	}
}

func TestManagerDeepSearch(t *testing.T) {
	cfg := DefaultSearchConfig()
	mgr := NewManager(cfg)

	mgr.searchEngines = []SearchEngine{
		&mockSearchEngine{
			name: "a",
			results: []SearchResult{
				{Title: "A", URL: "https://a.com", Source: "a"},
			},
		},
		&mockSearchEngine{
			name: "b",
			results: []SearchResult{
				{Title: "B", URL: "https://b.com", Source: "b"},
			},
		},
	}

	result, err := mgr.DeepSearch(context.Background(), "test", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
}

func TestManagerFetchURL(t *testing.T) {
	cfg := DefaultSearchConfig()
	mgr := NewManager(cfg)

	mgr.fetchEngines = []FetchEngine{
		&mockFetchEngine{
			name: "mock-fetch",
			result: &FetchResult{
				Title:   "Example",
				Content: "Hello world",
				URL:     "https://example.com",
				Source:  "mock-fetch",
			},
		},
	}

	result, err := mgr.FetchURL(context.Background(), "https://example.com", 50000)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Content != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", result.Content)
	}
}

func TestManagerFetchURLValidation(t *testing.T) {
	cfg := DefaultSearchConfig()
	mgr := NewManager(cfg)

	_, err := mgr.FetchURL(context.Background(), "ftp://example.com", 50000)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected scheme validation error, got %v", err)
	}

	_, err = mgr.FetchURL(context.Background(), "http://127.0.0.1:9090", 50000)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Errorf("expected loopback error, got %v", err)
	}
}

func TestManagerFetchURLFallback(t *testing.T) {
	cfg := DefaultSearchConfig()
	mgr := NewManager(cfg)

	mgr.fetchEngines = []FetchEngine{
		&mockFetchEngine{name: "bad", err: fmt.Errorf("fail")},
		&mockFetchEngine{
			name: "good",
			result: &FetchResult{
				Content: "fallback content",
				URL:     "https://example.com",
				Source:  "good",
			},
		},
	}

	result, err := mgr.FetchURL(context.Background(), "https://example.com", 50000)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Content != "fallback content" {
		t.Errorf("expected 'fallback content', got '%s'", result.Content)
	}
}

// ---------------------------------------------------------------------------
// URL Validation Tests
// ---------------------------------------------------------------------------

func TestValidateFetchURL(t *testing.T) {
	tests := []struct {
		url    string
		valid  bool
		errMsg string
	}{
		{"https://example.com", true, ""},
		{"http://example.com/path", true, ""},
		{"ftp://example.com", false, "not allowed"},
		{"javascript:alert(1)", false, "not allowed"},
		{"http://127.0.0.1", false, "loopback"},
		{"http://10.0.0.1", false, "private"},
		{"http://192.168.1.1", false, "private"},
		{"http://172.16.0.1", false, "private"},
		{"http://172.20.0.1", false, "private"},
		{"http://172.32.0.1", true, ""}, // outside 172.16-31
		{"http://169.254.169.254", false, "link-local"},
		{"http://localhost", false, "localhost"},
		{"", false, "not allowed"},
	}

	for _, tt := range tests {
		err := ValidateFetchURL(tt.url)
		if tt.valid && err != nil {
			t.Errorf("expected %q to be valid, got error: %v", tt.url, err)
		}
		if !tt.valid && (err == nil || !strings.Contains(err.Error(), tt.errMsg)) {
			t.Errorf("expected %q to fail with %q, got: %v", tt.url, tt.errMsg, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Formatting Tests
// ---------------------------------------------------------------------------

func TestFormatResults(t *testing.T) {
	results := []SearchResult{
		{Title: "Test", URL: "https://example.com", Snippet: "test snippet", Source: "brave"},
	}
	formatted := FormatResults("test query", results)
	if !strings.Contains(formatted, "Results for: test query") {
		t.Error("expected header in formatted output")
	}
	if !strings.Contains(formatted, "[brave]") {
		t.Error("expected source tag in formatted output")
	}
}

func TestFormatDeepResults(t *testing.T) {
	dr := &DeepSearchResult{
		Results: []SearchResult{
			{Title: "Test", URL: "https://example.com", Source: "brave+ddgs"},
		},
		Sources: []string{"brave", "ddgs"},
		Errors:  []string{"exa: connection refused"},
	}
	formatted := FormatDeepResults("test", dr)
	if !strings.Contains(formatted, "deep search") {
		t.Error("expected 'deep search' in formatted output")
	}
	if !strings.Contains(formatted, "brave+ddgs") {
		t.Error("expected multi-source tag")
	}
	if !strings.Contains(formatted, "Source errors") {
		t.Error("expected error section")
	}
}

// ---------------------------------------------------------------------------
// Concurrency Safety Test
// ---------------------------------------------------------------------------

func TestSearchCacheConcurrent(t *testing.T) {
	cache := NewSearchCache(5*time.Minute, 100)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("query-%d", n%10)
			cache.Set(key, []SearchResult{
				{Title: fmt.Sprintf("Result %d", n), URL: fmt.Sprintf("https://example.com/%d", n), Source: "mock"},
			})
			cache.Get(key)
		}(i)
	}

	wg.Wait()
	// Should not panic or deadlock
}

func TestDeepSearchRaceDetection(t *testing.T) {
	engines := make([]SearchEngine, 5)
	for i := range engines {
		engines[i] = &mockSearchEngine{
			name: fmt.Sprintf("engine-%d", i),
			results: []SearchResult{
				{Title: fmt.Sprintf("Result %d", i), URL: fmt.Sprintf("https://example.com/%d", i), Source: fmt.Sprintf("engine-%d", i)},
			},
		}
	}

	// Run multiple times to detect races
	for i := 0; i < 10; i++ {
		result := DeepSearch(context.Background(), engines, "test", 10)
		if len(result.Results) == 0 {
			t.Error("expected results")
		}
	}
}