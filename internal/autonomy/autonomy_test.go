package autonomy

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TaskQueue tests
// ---------------------------------------------------------------------------

func TestTaskQueueAdd(t *testing.T) {
	q := NewTaskQueue(16)

	task := q.Add("Test task", "Do something", PriorityNormal, []string{"test"})

	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}
	if task.State != TaskReady {
		t.Fatalf("expected ready state, got %s", task.State)
	}
	if task.Priority != PriorityNormal {
		t.Fatalf("expected normal priority, got %s", task.Priority)
	}
}

func TestTaskQueuePull(t *testing.T) {
	q := NewTaskQueue(16)

	// Pull from empty queue
	if task := q.Pull("w1"); task != nil {
		t.Fatal("expected nil from empty queue")
	}

	// Add tasks with different priorities
	low := q.Add("Low task", "", PriorityLow, nil)
	high := q.Add("High task", "", PriorityHigh, nil)
	normal := q.Add("Normal task", "", PriorityNormal, nil)

	// Should pull highest priority first
	pulled := q.Pull("w1")
	if pulled.ID != high.ID {
		t.Fatalf("expected high priority task, got %s (priority: %s)", pulled.ID, pulled.Priority)
	}
	if pulled.State != TaskInProgress {
		t.Fatalf("expected in_progress state, got %s", pulled.State)
	}

	// Next should be normal
	pulled = q.Pull("w2")
	if pulled.ID != normal.ID {
		t.Fatalf("expected normal priority task, got %s", pulled.ID)
	}

	// Then low
	pulled = q.Pull("w3")
	if pulled.ID != low.ID {
		t.Fatalf("expected low priority task, got %s", pulled.ID)
	}
}

func TestTaskQueueComplete(t *testing.T) {
	q := NewTaskQueue(16)

	task := q.Add("Test", "", PriorityNormal, nil)
	q.Pull("w1")

	if err := q.Complete(task.ID, "done!"); err != nil {
		t.Fatal(err)
	}

	got, ok := q.Get(task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	if got.State != TaskDone {
		t.Fatalf("expected done state, got %s", got.State)
	}
	if got.Result != "done!" {
		t.Fatalf("expected result 'done!', got %s", got.Result)
	}
}

func TestTaskQueueFail(t *testing.T) {
	q := NewTaskQueue(16)

	task := q.Add("Test", "", PriorityNormal, nil)
	q.Pull("w1")

	// Fail with retry
	if err := q.Fail(task.ID, "oops", true); err != nil {
		t.Fatal(err)
	}

	got, _ := q.Get(task.ID)
	if got.State != TaskReady {
		t.Fatalf("expected ready state (retry), got %s", got.State)
	}

	// Fail without retry
	q.Pull("w1")
	if err := q.Fail(task.ID, "fatal", false); err != nil {
		t.Fatal(err)
	}

	got, _ = q.Get(task.ID)
	if got.State != TaskBlocked {
		t.Fatalf("expected blocked state, got %s", got.State)
	}
}

func TestTaskQueueBlockUnblock(t *testing.T) {
	q := NewTaskQueue(16)

	task := q.Add("Test", "", PriorityNormal, nil)

	if err := q.Block(task.ID, "waiting for approval"); err != nil {
		t.Fatal(err)
	}

	got, _ := q.Get(task.ID)
	if got.State != TaskBlocked {
		t.Fatalf("expected blocked state, got %s", got.State)
	}

	if err := q.Unblock(task.ID); err != nil {
		t.Fatal(err)
	}

	got, _ = q.Get(task.ID)
	if got.State != TaskReady {
		t.Fatalf("expected ready state after unblock, got %s", got.State)
	}
}

func TestTaskQueueStats(t *testing.T) {
	q := NewTaskQueue(16)

	q.Add("T1", "", PriorityNormal, nil)
	q.Add("T2", "", PriorityHigh, nil)
	q.Add("T3", "", PriorityLow, nil)

	ready, inProgress, _, _ := q.Stats()
	if ready != 3 {
		t.Fatalf("expected 3 ready, got %d", ready)
	}

	q.Pull("w1")
	ready, inProgress, _, _ = q.Stats()
	if inProgress != 1 {
		t.Fatalf("expected 1 in_progress, got %d", inProgress)
	}
	if ready != 2 {
		t.Fatalf("expected 2 ready, got %d", ready)
	}
}

func TestTaskQueueConcurrentAccess(t *testing.T) {
	q := NewTaskQueue(64)

	var wg sync.WaitGroup
	const goroutines = 10
	const tasksPerGoroutine = 50

	// Concurrent adds
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < tasksPerGoroutine; j++ {
				q.Add(fmt.Sprintf("Task-%d-%d", id, j), "", PriorityNormal, nil)
			}
		}(i)
	}
	wg.Wait()

	ready, _, _, _ := q.Stats()
	if ready != goroutines*tasksPerGoroutine {
		t.Fatalf("expected %d tasks, got %d", goroutines*tasksPerGoroutine, ready)
	}

	// Concurrent pulls
	var pulled int64
	var pullMu sync.Mutex
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < tasksPerGoroutine; j++ {
				if q.Pull(fmt.Sprintf("w-%d", id)) != nil {
					pullMu.Lock()
					pulled++
					pullMu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	if pulled != int64(goroutines*tasksPerGoroutine) {
		t.Fatalf("expected %d pulled, got %d", goroutines*tasksPerGoroutine, pulled)
	}
}

// ---------------------------------------------------------------------------
// HeartbeatEngine tests
// ---------------------------------------------------------------------------

func TestHeartbeatIsActiveHour(t *testing.T) {
	cfg := DefaultHeartbeatConfig()
	cfg.ActiveStart = 6
	cfg.ActiveEnd = 23

	hb := NewHeartbeatEngine(cfg, nil, NewTaskQueue(16))

	// 10 AM should be active
	if !hb.isActiveHour(time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("10 AM should be active")
	}

	// 3 AM should not be active
	if hb.isActiveHour(time.Date(2026, 4, 21, 3, 0, 0, 0, time.UTC)) {
		t.Fatal("3 AM should not be active")
	}
}

func TestHeartbeatIsActiveHourWrapMidnight(t *testing.T) {
	cfg := DefaultHeartbeatConfig()
	cfg.ActiveStart = 22
	cfg.ActiveEnd = 6

	hb := NewHeartbeatEngine(cfg, nil, NewTaskQueue(16))

	// 11 PM should be active
	if !hb.isActiveHour(time.Date(2026, 4, 21, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("11 PM should be active (wraps midnight)")
	}

	// 3 AM should be active
	if !hb.isActiveHour(time.Date(2026, 4, 21, 3, 0, 0, 0, time.UTC)) {
		t.Fatal("3 AM should be active (wraps midnight)")
	}

	// 10 AM should not be active
	if hb.isActiveHour(time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("10 AM should not be active (wraps midnight)")
	}
}

func TestHeartbeatTrigger(t *testing.T) {
	q := NewTaskQueue(16)

	// Add some tasks
	q.Add("Task 1", "", PriorityHigh, nil)
	q.Add("Task 2", "", PriorityNormal, nil)

	// Trigger without pool (passive mode check)
	cfg := DefaultHeartbeatConfig()
	cfg.Mode = HeartbeatPassive
	hb := NewHeartbeatEngine(cfg, nil, q)

	event := hb.Trigger(context.Background())
	if event == nil {
		t.Fatal("expected non-nil event")
	}
}

// ---------------------------------------------------------------------------
// AutonomyKit integration test
// ---------------------------------------------------------------------------

func TestAutonomyKitAddTask(t *testing.T) {
	q := NewTaskQueue(16)

	kit := &AutonomyKit{
		queue: q,
	}

	task := kit.AddTask("Research topic", "Deep dive into X", PriorityHigh, []string{"research"})
	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}

	ready, _, _, _ := q.Stats()
	if ready != 1 {
		t.Fatalf("expected 1 ready task, got %d", ready)
	}
}

func TestAutonomyKitStatus(t *testing.T) {
	q := NewTaskQueue(16)
	pool := NewWorkerPool(DefaultPoolConfig(), nil, q)
	hb := NewHeartbeatEngine(DefaultHeartbeatConfig(), pool, q)

	kit := &AutonomyKit{
		queue:     q,
		pool:      pool,
		heartbeat: hb,
	}

	q.Add("Task 1", "", PriorityNormal, nil)
	q.Add("Task 2", "", PriorityHigh, nil)

	status := kit.Status()
	if status.QueueReady != 2 {
		t.Fatalf("expected 2 ready, got %d", status.QueueReady)
	}
}

// ---------------------------------------------------------------------------
// ToolDefinitions tests
// ---------------------------------------------------------------------------

func TestToolQueueAdd(t *testing.T) {
	q := NewTaskQueue(16)
	kit := &AutonomyKit{queue: q}
	td := NewToolDefinitions(kit)

	result, err := td.HandleQueueAdd(map[string]any{
		"title":       "Test task",
		"description": "Do something",
		"priority":    "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	ready, _, _, _ := q.Stats()
	if ready != 1 {
		t.Fatalf("expected 1 ready task, got %d", ready)
	}
}

func TestToolQueueList(t *testing.T) {
	q := NewTaskQueue(16)
	kit := &AutonomyKit{queue: q}
	td := NewToolDefinitions(kit)

	q.Add("Task 1", "", PriorityNormal, nil)
	q.Add("Task 2", "", PriorityHigh, nil)

	result, err := td.HandleQueueList(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestToolQueueUpdate(t *testing.T) {
	q := NewTaskQueue(16)
	kit := &AutonomyKit{queue: q}
	td := NewToolDefinitions(kit)

	task := q.Add("Test", "", PriorityNormal, nil)
	q.Pull("w1")

	// Complete
	_, err := td.HandleQueueUpdate(map[string]any{
		"task_id": task.ID,
		"action":  "complete",
		"result":  "All done!",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := q.Get(task.ID)
	if got.State != TaskDone {
		t.Fatalf("expected done, got %s", got.State)
	}
}

func TestToolQueueUpdateBlock(t *testing.T) {
	q := NewTaskQueue(16)
	kit := &AutonomyKit{queue: q}
	td := NewToolDefinitions(kit)

	task := q.Add("Test", "", PriorityNormal, nil)

	_, err := td.HandleQueueUpdate(map[string]any{
		"task_id": task.ID,
		"action":  "block",
		"reason":  "needs approval",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := q.Get(task.ID)
	if got.State != TaskBlocked {
		t.Fatalf("expected blocked, got %s", got.State)
	}

	// Unblock
	_, err = td.HandleQueueUpdate(map[string]any{
		"task_id": task.ID,
		"action":  "unblock",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ = q.Get(task.ID)
	if got.State != TaskReady {
		t.Fatalf("expected ready after unblock, got %s", got.State)
	}
}

func TestToolQueueUpdateInvalidAction(t *testing.T) {
	q := NewTaskQueue(16)
	kit := &AutonomyKit{queue: q}
	td := NewToolDefinitions(kit)

	task := q.Add("Test", "", PriorityNormal, nil)

	_, err := td.HandleQueueUpdate(map[string]any{
		"task_id": task.ID,
		"action":  "explode",
	})
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestToolStatus(t *testing.T) {
	q := NewTaskQueue(16)
	pool := NewWorkerPool(DefaultPoolConfig(), nil, q)
	hb := NewHeartbeatEngine(DefaultHeartbeatConfig(), pool, q)
	kit := &AutonomyKit{queue: q, pool: pool, heartbeat: hb}
	td := NewToolDefinitions(kit)

	q.Add("Task 1", "", PriorityNormal, nil)

	result, err := td.HandleStatus(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestParseTaskPriority(t *testing.T) {
	tests := []struct {
		input    string
		expected TaskPriority
	}{
		{"low", PriorityLow},
		{"normal", PriorityNormal},
		{"high", PriorityHigh},
		{"critical", PriorityCritical},
		{"", PriorityNormal},
		{"unknown", PriorityNormal},
	}

	for _, tt := range tests {
		got := ParseTaskPriority(tt.input)
		if got != tt.expected {
			t.Errorf("ParseTaskPriority(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestTaskQueueCleanDone(t *testing.T) {
	q := NewTaskQueue(16)

	task := q.Add("Old task", "", PriorityNormal, nil)
	q.Pull("w1")
	q.Complete(task.ID, "done")

	// Task was just completed, shouldn't be cleaned yet
	removed := q.CleanDone(1 * time.Hour)
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}

	// Clean with very short duration
	removed = q.CleanDone(0)
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	_, _, _, done := q.Stats()
	if done != 0 {
		t.Fatalf("expected 0 done after clean, got %d", done)
	}
}

func TestTaskQueueListByState(t *testing.T) {
	q := NewTaskQueue(16)

	q.Add("T1", "", PriorityNormal, nil)
	q.Add("T2", "", PriorityHigh, nil)
	t3 := q.Add("T3", "", PriorityLow, nil)

	q.Pull("w1") // pulls T2 (highest priority)
	q.Block(t3.ID, "waiting")

	ready := q.ListByState(TaskReady)
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready, got %d", len(ready))
	}

	inProgress := q.ListByState(TaskInProgress)
	if len(inProgress) != 1 {
		t.Fatalf("expected 1 in_progress, got %d", len(inProgress))
	}

	blocked := q.ListByState(TaskBlocked)
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked, got %d", len(blocked))
	}
}

func TestTaskQueueStatsAll(t *testing.T) {
	q := NewTaskQueue(16)
	q.Add("T1", "", PriorityNormal, nil)
	q.Add("T2", "", PriorityHigh, nil)
	t3 := q.Add("T3", "", PriorityLow, nil)
	q.Pull("w1")
	q.Block(t3.ID, "waiting")

	ready, inProgress, blocked, done := q.Stats()
	_ = blocked
	_ = done
	if ready != 1 || inProgress != 1 {
		t.Fatalf("expected 1 ready + 1 in_progress, got %d ready + %d in_progress", ready, inProgress)
	}
}