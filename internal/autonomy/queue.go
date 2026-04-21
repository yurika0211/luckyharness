// Package autonomy provides a native Agent Autonomy Kit for LuckyHarness.
// It enables proactive, self-directed agent work through:
//   - WorkerPool: goroutine-based concurrent agent execution
//   - TaskQueue: persistent priority task queue (Ready/InProgress/Blocked/Done)
//   - HeartbeatEngine: proactive heartbeat that does work, not just checks
//   - AutonomyKit: top-level orchestrator combining all components
package autonomy

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Task Queue
// ---------------------------------------------------------------------------

// TaskPriority represents task priority.
type TaskPriority int

const (
	PriorityLow TaskPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

func (p TaskPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ParseTaskPriority parses a priority string.
func ParseTaskPriority(s string) TaskPriority {
	switch s {
	case "low":
		return PriorityLow
	case "high":
		return PriorityHigh
	case "critical":
		return PriorityCritical
	default:
		return PriorityNormal
	}
}

// TaskState represents the state of a task in the queue.
type TaskState string

const (
	TaskReady      TaskState = "ready"
	TaskInProgress TaskState = "in_progress"
	TaskBlocked    TaskState = "blocked"
	TaskDone       TaskState = "done"
)

// QueueTask represents a task in the autonomy task queue.
type QueueTask struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Priority    TaskPriority      `json:"priority"`
	State       TaskState         `json:"state"`
	AssignedTo  string            `json:"assigned_to,omitempty"` // worker ID
	BlockReason string            `json:"block_reason,omitempty"`
	Result      string            `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   time.Time         `json:"started_at,omitempty"`
	CompletedAt time.Time         `json:"completed_at,omitempty"`
}

// TaskQueue is a concurrent-safe, persistent task queue.
type TaskQueue struct {
	mu     sync.RWMutex
	tasks  map[string]*QueueTask
	nextID atomic.Int64
	ready  chan *QueueTask // buffered channel for ready tasks
}

// NewTaskQueue creates a new task queue.
func NewTaskQueue(bufferSize int) *TaskQueue {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &TaskQueue{
		tasks: make(map[string]*QueueTask),
		ready: make(chan *QueueTask, bufferSize),
	}
}

// Add adds a new task to the queue.
func (q *TaskQueue) Add(title, description string, priority TaskPriority, tags []string) *QueueTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	id := fmt.Sprintf("tq-%d", q.nextID.Add(1))
	task := &QueueTask{
		ID:          id,
		Title:       title,
		Description: description,
		Priority:    priority,
		State:       TaskReady,
		Tags:        tags,
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
	}

	q.tasks[id] = task

	// Non-blocking send to ready channel
	select {
	case q.ready <- task:
	default:
		// channel full, task is still in map and can be pulled via Pull
	}

	return task
}

// Pull pulls the highest-priority ready task and marks it in-progress.
// Returns nil if no ready tasks.
func (q *TaskQueue) Pull(workerID string) *QueueTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	var best *QueueTask
	for _, t := range q.tasks {
		if t.State != TaskReady {
			continue
		}
		if best == nil || t.Priority > best.Priority {
			best = t
		}
	}

	if best == nil {
		return nil
	}

	best.State = TaskInProgress
	best.AssignedTo = workerID
	best.StartedAt = time.Now()

	return best
}

// PullChan returns a channel that yields ready tasks.
// Blocks until a task is available or context is cancelled.
func (q *TaskQueue) PullChan(ctx context.Context, workerID string) <-chan *QueueTask {
	out := make(chan *QueueTask, 1)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-q.ready:
				q.mu.Lock()
				// Re-check state (might have been claimed)
				if t, ok := q.tasks[task.ID]; ok && t.State == TaskReady {
					t.State = TaskInProgress
					t.AssignedTo = workerID
					t.StartedAt = time.Now()
					q.mu.Unlock()
					select {
					case out <- t:
					case <-ctx.Done():
						return
					}
					return
				}
				q.mu.Unlock()
				// Task was already claimed, try again
			default:
				// No task in channel, try Pull
				if t := q.Pull(workerID); t != nil {
					select {
					case out <- t:
					case <-ctx.Done():
						return
					}
					return
				}
				// Wait a bit before retrying
				select {
				case <-ctx.Done():
					return
				case <-time.After(500 * time.Millisecond):
				}
			}
		}
	}()

	return out
}

// Complete marks a task as done.
func (q *TaskQueue) Complete(taskID, result string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	t, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	t.State = TaskDone
	t.Result = result
	t.CompletedAt = time.Now()
	return nil
}

// Fail marks a task as failed (moves back to ready for retry, or blocked).
func (q *TaskQueue) Fail(taskID, errMsg string, retry bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	t, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	if retry {
		t.State = TaskReady
		t.AssignedTo = ""
		t.StartedAt = time.Time{}
		t.Error = errMsg
	} else {
		t.State = TaskBlocked
		t.BlockReason = errMsg
		t.Error = errMsg
		t.CompletedAt = time.Now()
	}
	return nil
}

// Block marks a task as blocked.
func (q *TaskQueue) Block(taskID, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	t, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	t.State = TaskBlocked
	t.BlockReason = reason
	t.AssignedTo = ""
	return nil
}

// Unblock moves a blocked task back to ready.
func (q *TaskQueue) Unblock(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	t, ok := q.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if t.State != TaskBlocked {
		return fmt.Errorf("task %s is not blocked", taskID)
	}
	t.State = TaskReady
	t.BlockReason = ""

	select {
	case q.ready <- t:
	default:
	}

	return nil
}

// Get retrieves a task by ID.
func (q *TaskQueue) Get(taskID string) (*QueueTask, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	t, ok := q.tasks[taskID]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// ListByState lists tasks filtered by state.
func (q *TaskQueue) ListByState(state TaskState) []*QueueTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*QueueTask
	for _, t := range q.tasks {
		if t.State == state {
			cp := *t
			result = append(result, &cp)
		}
	}
	return result
}

// ListAll lists all tasks.
func (q *TaskQueue) ListAll() []*QueueTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*QueueTask, 0, len(q.tasks))
	for _, t := range q.tasks {
		cp := *t
		result = append(result, &cp)
	}
	return result
}

// Stats returns queue statistics.
func (q *TaskQueue) Stats() (ready, inProgress, blocked, done int) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, t := range q.tasks {
		switch t.State {
		case TaskReady:
			ready++
		case TaskInProgress:
			inProgress++
		case TaskBlocked:
			blocked++
		case TaskDone:
			done++
		}
	}
	return
}

// CleanDone removes completed tasks older than the given duration.
func (q *TaskQueue) CleanDone(olderThan time.Duration) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	removed := 0
	for id, t := range q.tasks {
		if t.State == TaskDone && !t.CompletedAt.IsZero() && now.Sub(t.CompletedAt) > olderThan {
			delete(q.tasks, id)
			removed++
		}
	}
	return removed
}