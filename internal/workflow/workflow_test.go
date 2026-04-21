package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestWorkflow() *Workflow {
	return NewWorkflow("test-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "echo", Params: map[string]interface{}{"msg": "hello"}},
		{ID: "t2", Name: "Task 2", Action: "echo", Params: map[string]interface{}{"msg": "world"}, DependsOn: []string{"t1"}},
		{ID: "t3", Name: "Task 3", Action: "echo", Params: map[string]interface{}{"msg": "!"}, DependsOn: []string{"t1"}},
		{ID: "t4", Name: "Task 4", Action: "echo", Params: map[string]interface{}{"msg": "done"}, DependsOn: []string{"t2", "t3"}},
	})
}

func newTestExecutor() *DefaultExecutor {
	ex := NewDefaultExecutor()
	ex.RegisterActionHandler("echo", func(ctx context.Context, task *Task) (interface{}, error) {
		msg, _ := task.Params["msg"].(string)
		return map[string]interface{}{"message": msg}, nil
	})
	ex.RegisterActionHandler("fail", func(ctx context.Context, task *Task) (interface{}, error) {
		return nil, fmt.Errorf("intentional failure")
	})
	ex.RegisterActionHandler("slow", func(ctx context.Context, task *Task) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "slow-done", nil
	})
	return ex
}

// ---------------------------------------------------------------------------
// WF-1: Condition & Output Passing Tests
// ---------------------------------------------------------------------------

func TestConditionEvalStatus(t *testing.T) {
	cond := &Condition{TaskID: "t1", Status: "completed"}
	result := &TaskResult{TaskID: "t1", Status: StatusCompleted}
	if !cond.Eval(result) {
		t.Error("expected condition to match completed status")
	}

	result.Status = StatusFailed
	if cond.Eval(result) {
		t.Error("expected condition to not match failed status")
	}
}

func TestConditionEvalOutput(t *testing.T) {
	cond := &Condition{TaskID: "t1", Output: "status=ok"}
	result := &TaskResult{
		TaskID: "t1",
		Status: StatusCompleted,
		Output: map[string]interface{}{"status": "ok"},
	}
	if !cond.Eval(result) {
		t.Error("expected condition to match output")
	}

	result.Output = map[string]interface{}{"status": "error"}
	if cond.Eval(result) {
		t.Error("expected condition to not match output")
	}
}

func TestConditionEvalNilResult(t *testing.T) {
	cond := &Condition{TaskID: "t1", Status: "completed"}
	if cond.Eval(nil) {
		t.Error("expected condition to not match nil result")
	}
}

func TestConditionEvalEmptyTaskID(t *testing.T) {
	cond := &Condition{}
	if !cond.Eval(nil) {
		t.Error("empty condition should always pass")
	}
}

func TestConditionEvalInvalidOutput(t *testing.T) {
	cond := &Condition{TaskID: "t1", Output: "invalid"}
	result := &TaskResult{TaskID: "t1", Status: StatusCompleted, Output: "string"}
	if cond.Eval(result) {
		t.Error("expected condition to fail with invalid output format")
	}
}

func TestConditionalBranching(t *testing.T) {
	wf := NewWorkflow("conditional-test", []*Task{
		{ID: "check", Name: "Check", Action: "echo", Params: map[string]interface{}{"msg": "check"}},
		{ID: "on-success", Name: "On Success", Action: "echo", Params: map[string]interface{}{"msg": "success"}, DependsOn: []string{"check"}, Condition: &Condition{TaskID: "check", Status: "completed"}},
		{ID: "on-failure", Name: "On Failure", Action: "echo", Params: map[string]interface{}{"msg": "failure"}, DependsOn: []string{"check"}, Condition: &Condition{TaskID: "check", Status: "failed"}},
	})

	engine := NewWorkflowEngine(newTestExecutor(), 5)
	if err := engine.RegisterWorkflow(wf); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	inst, err := engine.StartWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	snap := inst.Snapshot()
	if snap.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", snap.Status)
	}

	// on-success should be completed
	if r, ok := snap.Results["on-success"]; ok {
		if r.Status != StatusCompleted {
			t.Errorf("expected on-success completed, got %s", r.Status)
		}
	}

	// on-failure should be skipped
	if r, ok := snap.Results["on-failure"]; ok {
		if r.Status != StatusSkipped {
			t.Errorf("expected on-failure skipped, got %s", r.Status)
		}
	}
}

func TestConditionInvalidReference(t *testing.T) {
	wf := NewWorkflow("bad-cond", []*Task{
		{ID: "t1", Name: "T1", Action: "echo", Condition: &Condition{TaskID: "nonexistent", Status: "completed"}},
	})
	if err := wf.Validate(); err == nil {
		t.Error("expected validation error for invalid condition reference")
	}
}

// ---------------------------------------------------------------------------
// WF-2: YAML & Persistence Tests
// ---------------------------------------------------------------------------

func TestYAMLRoundTrip(t *testing.T) {
	wf := newTestWorkflow()
	data, err := wf.ToYAML()
	if err != nil {
		t.Fatalf("YAML marshal failed: %v", err)
	}

	parsed, err := FromYAML(data)
	if err != nil {
		t.Fatalf("YAML unmarshal failed: %v", err)
	}

	if parsed.Name != wf.Name {
		t.Errorf("expected name %q, got %q", wf.Name, parsed.Name)
	}
	if len(parsed.Tasks) != len(wf.Tasks) {
		t.Errorf("expected %d tasks, got %d", len(wf.Tasks), len(parsed.Tasks))
	}
}

func TestJSONRoundTrip(t *testing.T) {
	wf := newTestWorkflow()
	data, err := wf.ToJSON()
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	parsed, err := FromJSON(data)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.Name != wf.Name {
		t.Errorf("expected name %q, got %q", wf.Name, parsed.Name)
	}
}

func TestFileSaveLoadJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	wf := newTestWorkflow()
	if err := SaveToFile(wf, path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Name != wf.Name {
		t.Errorf("expected name %q, got %q", wf.Name, loaded.Name)
	}
}

func TestFileSaveLoadYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.yaml")

	wf := newTestWorkflow()
	if err := SaveToFile(wf, path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Name != wf.Name {
		t.Errorf("expected name %q, got %q", wf.Name, loaded.Name)
	}
}

func TestFileStoreWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	wf := newTestWorkflow()
	if err := store.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow failed: %v", err)
	}

	loaded, err := store.LoadWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("load workflow failed: %v", err)
	}
	if loaded.Name != wf.Name {
		t.Errorf("expected name %q, got %q", wf.Name, loaded.Name)
	}
}

func TestFileStoreInstance(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	inst := NewWorkflowInstance("wf-123")
	inst.SetStatus(StatusCompleted)
	inst.SetStartTime(time.Now().Add(-time.Minute))
	inst.SetEndTime(time.Now())
	inst.SetResult("t1", &TaskResult{
		TaskID: "t1",
		Status: StatusCompleted,
		Output: "hello",
	})

	if err := store.SaveInstance(inst); err != nil {
		t.Fatalf("save instance failed: %v", err)
	}

	loaded, err := store.LoadInstance(inst.ID)
	if err != nil {
		t.Fatalf("load instance failed: %v", err)
	}
	if loaded.WorkflowID != inst.WorkflowID {
		t.Errorf("expected workflowID %q, got %q", inst.WorkflowID, loaded.WorkflowID)
	}
}

func TestFileStoreListWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	wf1 := NewWorkflow("wf1", []*Task{{ID: "t1", Name: "T1", Action: "echo"}})
	wf2 := NewWorkflow("wf2", []*Task{{ID: "t1", Name: "T1", Action: "echo"}})

	_ = store.SaveWorkflow(wf1)
	_ = store.SaveWorkflow(wf2)

	ids, err := store.ListWorkflows()
	if err != nil {
		t.Fatalf("list workflows failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 workflows, got %d", len(ids))
	}
}

func TestLoadUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.xml")
	os.WriteFile(path, []byte("<xml/>"), 0644)

	_, err := LoadFromFile(path)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

// ---------------------------------------------------------------------------
// WF-3: Event Callback Tests
// ---------------------------------------------------------------------------

func TestEventEmitterOnEmit(t *testing.T) {
	em := NewEventEmitter()
	var received []Event
	var mu sync.Mutex

	em.On(func(evt Event) {
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
	})

	em.Emit(Event{Type: EventTaskStart, TaskID: "t1", Timestamp: time.Now()})
	em.Emit(Event{Type: EventTaskComplete, TaskID: "t1", Timestamp: time.Now()})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Errorf("expected 2 events, got %d", len(received))
	}
}

func TestEventEmitterMultiple(t *testing.T) {
	em := NewEventEmitter()
	count1, count2 := 0, 0
	var mu sync.Mutex

	em.On(func(evt Event) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	em.On(func(evt Event) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	em.Emit(Event{Type: EventWorkflowStart, Timestamp: time.Now()})

	mu.Lock()
	defer mu.Unlock()
	if count1 != 1 || count2 != 1 {
		t.Errorf("expected both handlers called once, got %d and %d", count1, count2)
	}
}

func TestEventEmitterLen(t *testing.T) {
	em := NewEventEmitter()
	if em.Len() != 0 {
		t.Errorf("expected 0 handlers, got %d", em.Len())
	}
	em.On(func(evt Event) {})
	em.On(func(evt Event) {})
	if em.Len() != 2 {
		t.Errorf("expected 2 handlers, got %d", em.Len())
	}
}

func TestWorkflowEvents(t *testing.T) {
	engine := NewWorkflowEngine(newTestExecutor(), 5)
	var events []Event
	var mu sync.Mutex

	engine.Emitter().On(func(evt Event) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})

	wf := newTestWorkflow()
	_ = engine.RegisterWorkflow(wf)
	_, _ = engine.StartWorkflow(wf.ID)

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	// Should have: workflow_start + 4 task_start + 4 task_complete + workflow_complete = 10
	if len(events) < 4 {
		t.Errorf("expected at least 4 events, got %d", len(events))
	}

	// Check workflow start event
	found := false
	for _, e := range events {
		if e.Type == EventWorkflowStart {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected workflow_start event")
	}
}

// ---------------------------------------------------------------------------
// Core Workflow Tests (existing, updated)
// ---------------------------------------------------------------------------

func TestWorkflowValidation(t *testing.T) {
	wf := newTestWorkflow()
	if err := wf.Validate(); err != nil {
		t.Errorf("valid workflow failed validation: %v", err)
	}
}

func TestWorkflowValidationNoName(t *testing.T) {
	wf := NewWorkflow("", []*Task{{ID: "t1", Name: "T1", Action: "echo"}})
	if err := wf.Validate(); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestWorkflowValidationNoTasks(t *testing.T) {
	wf := NewWorkflow("test", []*Task{})
	if err := wf.Validate(); err == nil {
		t.Error("expected error for no tasks")
	}
}

func TestWorkflowValidationDuplicateID(t *testing.T) {
	wf := NewWorkflow("test", []*Task{
		{ID: "t1", Name: "T1", Action: "echo"},
		{ID: "t1", Name: "T2", Action: "echo"},
	})
	if err := wf.Validate(); err == nil {
		t.Error("expected error for duplicate task ID")
	}
}

func TestWorkflowValidationSelfDep(t *testing.T) {
	wf := NewWorkflow("test", []*Task{
		{ID: "t1", Name: "T1", Action: "echo", DependsOn: []string{"t1"}},
	})
	if err := wf.Validate(); err == nil {
		t.Error("expected error for self-dependency")
	}
}

func TestWorkflowValidationCycle(t *testing.T) {
	wf := NewWorkflow("test", []*Task{
		{ID: "t1", Name: "T1", Action: "echo", DependsOn: []string{"t2"}},
		{ID: "t2", Name: "T2", Action: "echo", DependsOn: []string{"t1"}},
	})
	if err := wf.Validate(); err == nil {
		t.Error("expected error for cycle")
	}
}

func TestGetExecutionOrder(t *testing.T) {
	wf := newTestWorkflow()
	order, err := wf.GetExecutionOrder()
	if err != nil {
		t.Fatalf("execution order failed: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(order))
	}
	// t1 must come before t2 and t3, which must come before t4
	t1Idx, t2Idx, t3Idx, t4Idx := -1, -1, -1, -1
	for i, id := range order {
		switch id {
		case "t1": t1Idx = i
		case "t2": t2Idx = i
		case "t3": t3Idx = i
		case "t4": t4Idx = i
		}
	}
	if t1Idx > t2Idx || t1Idx > t3Idx || t2Idx > t4Idx || t3Idx > t4Idx {
		t.Errorf("invalid execution order: %v", order)
	}
}

func TestGetReadyTasks(t *testing.T) {
	wf := newTestWorkflow()
	ready := wf.GetReadyTasks(map[string]bool{})
	if len(ready) != 1 || ready[0].ID != "t1" {
		t.Errorf("expected only t1 ready, got %v", ready)
	}

	ready = wf.GetReadyTasks(map[string]bool{"t1": true})
	if len(ready) != 2 {
		t.Errorf("expected t2 and t3 ready, got %d", len(ready))
	}
}

func TestWorkflowExecution(t *testing.T) {
	engine := NewWorkflowEngine(newTestExecutor(), 5)
	wf := newTestWorkflow()
	_ = engine.RegisterWorkflow(wf)

	inst, err := engine.StartWorkflow(wf.ID)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	snap := inst.Snapshot()
	if snap.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", snap.Status)
	}

	for _, id := range []string{"t1", "t2", "t3", "t4"} {
		r, ok := snap.Results[id]
		if !ok {
			t.Errorf("missing result for %s", id)
			continue
		}
		if r.Status != StatusCompleted {
			t.Errorf("task %s: expected completed, got %s", id, r.Status)
		}
	}
}

func TestWorkflowFailure(t *testing.T) {
	ex := NewDefaultExecutor()
	ex.RegisterActionHandler("echo", func(ctx context.Context, task *Task) (interface{}, error) {
		return "ok", nil
	})
	ex.RegisterActionHandler("fail", func(ctx context.Context, task *Task) (interface{}, error) {
		return nil, fmt.Errorf("boom")
	})

	engine := NewWorkflowEngine(ex, 5)
	wf := NewWorkflow("fail-test", []*Task{
		{ID: "t1", Name: "T1", Action: "fail"},
		{ID: "t2", Name: "T2", Action: "echo", DependsOn: []string{"t1"}},
	})
	_ = engine.RegisterWorkflow(wf)

	inst, _ := engine.StartWorkflow(wf.ID)
	time.Sleep(500 * time.Millisecond)

	snap := inst.Snapshot()
	if snap.Status != StatusFailed {
		t.Errorf("expected failed, got %s", snap.Status)
	}

	r, ok := snap.Results["t2"]
	if !ok || r.Status != StatusSkipped {
		t.Errorf("expected t2 skipped, got %v", r)
	}
}

func TestWorkflowRetry(t *testing.T) {
	attempts := 0
	ex := NewDefaultExecutor()
	ex.RegisterActionHandler("flaky", func(ctx context.Context, task *Task) (interface{}, error) {
		attempts++
		if attempts < 3 {
			return nil, fmt.Errorf("not yet")
		}
		return "finally", nil
	})

	engine := NewWorkflowEngine(ex, 5)
	wf := NewWorkflow("retry-test", []*Task{
		{ID: "t1", Name: "T1", Action: "flaky", RetryCount: 3, RetryDelay: 50 * time.Millisecond},
	})
	_ = engine.RegisterWorkflow(wf)

	inst, _ := engine.StartWorkflow(wf.ID)
	time.Sleep(500 * time.Millisecond)

	snap := inst.Snapshot()
	if snap.Status != StatusCompleted {
		t.Errorf("expected completed after retry, got %s", snap.Status)
	}
}

func TestWorkflowCancel(t *testing.T) {
	ex := NewDefaultExecutor()
	ex.RegisterActionHandler("slow", func(ctx context.Context, task *Task) (interface{}, error) {
		time.Sleep(5 * time.Second)
		return "done", nil
	})

	engine := NewWorkflowEngine(ex, 5)
	wf := NewWorkflow("cancel-test", []*Task{
		{ID: "t1", Name: "T1", Action: "slow"},
	})
	_ = engine.RegisterWorkflow(wf)

	inst, _ := engine.StartWorkflow(wf.ID)
	time.Sleep(100 * time.Millisecond)
	inst.Cancel()
	time.Sleep(200 * time.Millisecond)

	if inst.GetStatus() != StatusFailed {
		t.Errorf("expected failed after cancel, got %s", inst.GetStatus())
	}
}

func TestWorkflowParallelExecution(t *testing.T) {
	engine := NewWorkflowEngine(newTestExecutor(), 5)
	wf := NewWorkflow("parallel-test", []*Task{
		{ID: "t1", Name: "T1", Action: "echo", Params: map[string]interface{}{"msg": "a"}},
		{ID: "t2", Name: "T2", Action: "echo", Params: map[string]interface{}{"msg": "b"}},
		{ID: "t3", Name: "T3", Action: "echo", Params: map[string]interface{}{"msg": "c"}, DependsOn: []string{"t1", "t2"}},
	})
	_ = engine.RegisterWorkflow(wf)

	inst, _ := engine.StartWorkflow(wf.ID)
	time.Sleep(500 * time.Millisecond)

	snap := inst.Snapshot()
	if snap.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", snap.Status)
	}
}

func TestWorkflowWithStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	engine := NewWorkflowEngine(newTestExecutor(), 5)
	engine.SetStore(store)

	wf := newTestWorkflow()
	_ = engine.RegisterWorkflow(wf)

	inst, _ := engine.StartWorkflow(wf.ID)
	time.Sleep(500 * time.Millisecond)

	// Check workflow was persisted
	loaded, err := store.LoadWorkflow(wf.ID)
	if err != nil {
		t.Errorf("workflow not persisted: %v", err)
	}
	if loaded.Name != wf.Name {
		t.Errorf("expected %q, got %q", wf.Name, loaded.Name)
	}

	// Check instance was persisted
	loadedInst, err := store.LoadInstance(inst.ID)
	if err != nil {
		t.Errorf("instance not persisted: %v", err)
	}
	if loadedInst.WorkflowID != wf.ID {
		t.Errorf("expected workflowID %q, got %q", wf.ID, loadedInst.WorkflowID)
	}
}

// ---------------------------------------------------------------------------
// Concurrency Safety
// ---------------------------------------------------------------------------

func TestWorkflowConcurrentStart(t *testing.T) {
	engine := NewWorkflowEngine(newTestExecutor(), 10)
	wf := newTestWorkflow()
	_ = engine.RegisterWorkflow(wf)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = engine.StartWorkflow(wf.ID)
		}()
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// All instances should complete
	instances := engine.ListInstances()
	for _, inst := range instances {
		if inst.GetStatus() != StatusCompleted {
			t.Errorf("instance %s: expected completed, got %s", inst.ID, inst.GetStatus())
		}
	}
}