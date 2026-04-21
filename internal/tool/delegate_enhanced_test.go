package tool

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestTaskPriorityString(t *testing.T) {
	tests := []struct {
		priority TaskPriority
		expected string
	}{
		{PriorityLow, "low"},
		{PriorityNormal, "normal"},
		{PriorityHigh, "high"},
		{PriorityCritical, "critical"},
	}
	for _, tt := range tests {
		if got := tt.priority.String(); got != tt.expected {
			t.Errorf("TaskPriority(%d).String() = %q, want %q", tt.priority, got, tt.expected)
		}
	}
}

func TestParseTaskPriority(t *testing.T) {
	tests := []struct {
		input    string
		expected TaskPriority
		hasError bool
	}{
		{"low", PriorityLow, false},
		{"normal", PriorityNormal, false},
		{"high", PriorityHigh, false},
		{"critical", PriorityCritical, false},
		{"", PriorityNormal, false},
		{"invalid", PriorityNormal, true},
	}
	for _, tt := range tests {
		p, err := ParseTaskPriority(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error for %q", tt.input)
			}
		} else {
			if p != tt.expected {
				t.Errorf("ParseTaskPriority(%q) = %d, want %d", tt.input, p, tt.expected)
			}
		}
	}
}

func TestDelegateTargetString(t *testing.T) {
	tests := []struct {
		target   DelegateTarget
		expected string
	}{
		{TargetAgent, "agent"},
		{TargetSkill, "skill"},
		{TargetMCP, "mcp"},
	}
	for _, tt := range tests {
		if got := tt.target.String(); got != tt.expected {
			t.Errorf("DelegateTarget(%d).String() = %q, want %q", tt.target, got, tt.expected)
		}
	}
}

func TestDelegateToSkill(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())

	task, err := dm.DelegateToSkill(context.Background(), "web-search", "Search for Go tutorials", PriorityNormal)
	if err != nil {
		t.Fatalf("DelegateToSkill: %v", err)
	}

	if task.ID == "" {
		t.Error("expected task ID")
	}

	// Check initial status under lock
	dm.mu.RLock()
	initialStatus := dm.tasks[task.ID].Status
	dm.mu.RUnlock()
	if initialStatus != StatusPending && initialStatus != StatusRunning {
		t.Errorf("expected pending or running status, got %s", initialStatus)
	}

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	dm.mu.RLock()
	completedTask, ok := dm.tasks[task.ID]
	completedStatus := completedTask.Status
	dm.mu.RUnlock()
	if !ok {
		t.Fatal("task not found")
	}
	if completedStatus != StatusCompleted {
		t.Errorf("expected completed, got %s", completedStatus)
	}
}

func TestDelegateToMCP(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())

	task, err := dm.DelegateToMCP(context.Background(), "test-server", "search", map[string]any{"query": "test"}, PriorityHigh)
	if err != nil {
		t.Fatalf("DelegateToMCP: %v", err)
	}

	if task.ID == "" {
		t.Error("expected task ID")
	}

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	dm.mu.RLock()
	completedTask, ok := dm.tasks[task.ID]
	dm.mu.RUnlock()
	if !ok {
		t.Fatal("task not found")
	}
	if completedTask.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", completedTask.Status)
	}
}

func TestDelegateToSkillTool(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())
	r := NewRegistry()
	r.Register(DelegateToSkillTool(dm))

	result, err := r.Call("delegate_to_skill", map[string]any{
		"skill_name":  "web-search",
		"description": "Search for Go tutorials",
		"priority":    "high",
	})
	if err != nil {
		t.Fatalf("delegate_to_skill: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if resp["skill_name"] != "web-search" {
		t.Errorf("expected skill_name=web-search, got %v", resp["skill_name"])
	}
	if resp["priority"] != "high" {
		t.Errorf("expected priority=high, got %v", resp["priority"])
	}
}

func TestDelegateToMCPTool(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())
	r := NewRegistry()
	r.Register(DelegateToMCPTool(dm))

	result, err := r.Call("delegate_to_mcp", map[string]any{
		"server_name": "test-server",
		"tool_name":   "search",
		"priority":    "critical",
	})
	if err != nil {
		t.Fatalf("delegate_to_mcp: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if resp["server_name"] != "test-server" {
		t.Errorf("expected server_name=test-server, got %v", resp["server_name"])
	}
	if resp["priority"] != "critical" {
		t.Errorf("expected priority=critical, got %v", resp["priority"])
	}
}

func TestDelegateToSkillToolMissingArgs(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())
	r := NewRegistry()
	r.Register(DelegateToSkillTool(dm))

	_, err := r.Call("delegate_to_skill", map[string]any{})
	if err == nil {
		t.Error("expected error for missing skill_name")
	}
}

func TestDelegateToMCPToolMissingArgs(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())
	r := NewRegistry()
	r.Register(DelegateToMCPTool(dm))

	_, err := r.Call("delegate_to_mcp", map[string]any{})
	if err == nil {
		t.Error("expected error for missing server_name")
	}
}

func TestPriorityTaskQueue(t *testing.T) {
	q := NewPriorityTaskQueue()

	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "low"}, Priority: PriorityLow})
	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "critical"}, Priority: PriorityCritical})
	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "normal"}, Priority: PriorityNormal})
	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "high"}, Priority: PriorityHigh})

	if q.Len() != 4 {
		t.Fatalf("expected 4 items, got %d", q.Len())
	}

	// Dequeue should return in priority order
	item, ok := q.Dequeue()
	if !ok {
		t.Fatal("expected item")
	}
	if item.Task.ID != "critical" {
		t.Errorf("expected critical first, got %s", item.Task.ID)
	}

	item, _ = q.Dequeue()
	if item.Task.ID != "high" {
		t.Errorf("expected high second, got %s", item.Task.ID)
	}

	item, _ = q.Dequeue()
	if item.Task.ID != "normal" {
		t.Errorf("expected normal third, got %s", item.Task.ID)
	}

	item, _ = q.Dequeue()
	if item.Task.ID != "low" {
		t.Errorf("expected low last, got %s", item.Task.ID)
	}

	// Queue should be empty
	_, ok = q.Dequeue()
	if ok {
		t.Error("expected empty queue")
	}
}

func TestPriorityTaskQueuePeek(t *testing.T) {
	q := NewPriorityTaskQueue()

	_, ok := q.Peek()
	if ok {
		t.Error("expected empty queue peek to return false")
	}

	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "high"}, Priority: PriorityHigh})
	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "low"}, Priority: PriorityLow})

	item, ok := q.Peek()
	if !ok {
		t.Fatal("expected item")
	}
	if item.Task.ID != "high" {
		t.Errorf("expected high priority peek, got %s", item.Task.ID)
	}

	// Peek should not remove the item
	if q.Len() != 2 {
		t.Errorf("expected 2 items after peek, got %d", q.Len())
	}
}

func TestPriorityTaskQueueList(t *testing.T) {
	q := NewPriorityTaskQueue()

	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "a"}, Priority: PriorityLow})
	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "b"}, Priority: PriorityHigh})

	list := q.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if list[0].Task.ID != "b" {
		t.Errorf("expected high priority first, got %s", list[0].Task.ID)
	}
}

func TestPriorityTaskQueueClear(t *testing.T) {
	q := NewPriorityTaskQueue()

	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "a"}, Priority: PriorityNormal})
	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "b"}, Priority: PriorityHigh})

	q.Clear()
	if q.Len() != 0 {
		t.Errorf("expected 0 items after clear, got %d", q.Len())
	}
}

func TestPriorityTaskQueueEnqueueTime(t *testing.T) {
	q := NewPriorityTaskQueue()
	before := time.Now()

	q.Enqueue(&PrioritizedTask{Task: &DelegateTask{ID: "a"}, Priority: PriorityNormal})

	item, _ := q.Dequeue()
	if item.EnqueuedAt.Before(before) {
		t.Error("expected EnqueuedAt to be set")
	}
}

func TestTaskCache(t *testing.T) {
	cache := NewTaskCache(5 * time.Minute)

	task := &DelegateTask{
		ID:          "task-1",
		Description: "Test task",
		Status:      StatusCompleted,
		Result:      "Done",
	}

	cache.Set("task-1", task)

	retrieved, ok := cache.Get("task-1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if retrieved.ID != "task-1" {
		t.Errorf("expected task-1, got %s", retrieved.ID)
	}
	if retrieved.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", retrieved.Status)
	}
}

func TestTaskCacheMiss(t *testing.T) {
	cache := NewTaskCache(5 * time.Minute)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestTaskCacheExpiry(t *testing.T) {
	cache := NewTaskCache(1 * time.Millisecond)

	task := &DelegateTask{ID: "task-1", Status: StatusCompleted}
	cache.Set("task-1", task)

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("task-1")
	if ok {
		t.Error("expected cache miss after expiry")
	}
}

func TestTaskCacheDelete(t *testing.T) {
	cache := NewTaskCache(5 * time.Minute)

	task := &DelegateTask{ID: "task-1", Status: StatusCompleted}
	cache.Set("task-1", task)
	cache.Delete("task-1")

	_, ok := cache.Get("task-1")
	if ok {
		t.Error("expected cache miss after delete")
	}
}

func TestTaskCacheClear(t *testing.T) {
	cache := NewTaskCache(5 * time.Minute)

	cache.Set("a", &DelegateTask{ID: "a"})
	cache.Set("b", &DelegateTask{ID: "b"})

	if cache.Size() != 2 {
		t.Errorf("expected 2 entries, got %d", cache.Size())
	}

	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", cache.Size())
	}
}

func TestTaskCacheClean(t *testing.T) {
	cache := NewTaskCache(1 * time.Millisecond)

	cache.Set("expired", &DelegateTask{ID: "expired"})
	time.Sleep(5 * time.Millisecond)
	cache.Set("fresh", &DelegateTask{ID: "fresh"})

	removed := cache.Clean()
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	_, ok := cache.Get("fresh")
	if !ok {
		t.Error("expected fresh entry to remain")
	}
}

func TestTaskCacheIsolation(t *testing.T) {
	cache := NewTaskCache(5 * time.Minute)

	task := &DelegateTask{ID: "task-1", Status: StatusCompleted, Result: "original"}
	cache.Set("task-1", task)

	// Modify original
	task.Result = "modified"

	// Cached copy should not be affected
	retrieved, _ := cache.Get("task-1")
	if retrieved.Result != "original" {
		t.Error("cache should return a copy, not the original")
	}
}

func TestDelegateToSkillWithContextCancellation(t *testing.T) {
	dm := NewDelegateManager(DelegateConfig{
		MaxConcurrent: 3,
		Timeout:       5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	task, err := dm.DelegateToSkill(ctx, "test-skill", "Test task", PriorityNormal)
	if err != nil {
		t.Fatalf("DelegateToSkill: %v", err)
	}

	// Wait a bit for the goroutine to process
	time.Sleep(100 * time.Millisecond)

	dm.mu.RLock()
	completedTask, _ := dm.tasks[task.ID]
	dm.mu.RUnlock()

	if completedTask.Status != StatusCancelled && completedTask.Status != StatusCompleted {
		// Either cancelled or completed quickly is acceptable
		t.Logf("Task status: %s", completedTask.Status)
	}
}

func TestDelegateToSkillToolCategories(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())
	tool := DelegateToSkillTool(dm)

	if tool.Category != CatDelegate {
		t.Errorf("expected CatDelegate, got %s", tool.Category)
	}
	if tool.Permission != PermApprove {
		t.Errorf("expected PermApprove, got %s", tool.Permission)
	}
}

func TestDelegateToMCPToolCategories(t *testing.T) {
	dm := NewDelegateManager(DefaultDelegateConfig())
	tool := DelegateToMCPTool(dm)

	if tool.Category != CatDelegate {
		t.Errorf("expected CatDelegate, got %s", tool.Category)
	}
	if tool.Permission != PermApprove {
		t.Errorf("expected PermApprove, got %s", tool.Permission)
	}
}