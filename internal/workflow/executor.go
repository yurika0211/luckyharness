package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Executor defines the interface for executing tasks.
type Executor interface {
	Execute(ctx context.Context, task *Task) (interface{}, error)
}

// DefaultExecutor is the default implementation of Executor.
type DefaultExecutor struct {
	actionHandlers map[string]ActionHandler
	mu             sync.RWMutex
}

// ActionHandler is a function that handles a specific action type.
type ActionHandler func(ctx context.Context, task *Task) (interface{}, error)

// NewDefaultExecutor creates a new default executor.
func NewDefaultExecutor() *DefaultExecutor {
	return &DefaultExecutor{
		actionHandlers: make(map[string]ActionHandler),
	}
}

// RegisterActionHandler registers a handler for a specific action type.
func (e *DefaultExecutor) RegisterActionHandler(action string, handler ActionHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.actionHandlers[action] = handler
}

// Execute executes a task using the registered handler.
func (e *DefaultExecutor) Execute(ctx context.Context, task *Task) (interface{}, error) {
	e.mu.RLock()
	handler, ok := e.actionHandlers[task.Action]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler registered for action: %s", task.Action)
	}

	return handler(ctx, task)
}

// WorkflowEngine orchestrates workflow execution.
type WorkflowEngine struct {
	executor   Executor
	instances  map[string]*WorkflowInstance
	workflows  map[string]*Workflow
	mu         sync.RWMutex
	workerPool chan struct{}
	maxWorkers int
}

// NewWorkflowEngine creates a new workflow engine.
func NewWorkflowEngine(executor Executor, maxWorkers int) *WorkflowEngine {
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	return &WorkflowEngine{
		executor:   executor,
		instances:  make(map[string]*WorkflowInstance),
		workflows:  make(map[string]*Workflow),
		maxWorkers: maxWorkers,
		workerPool: make(chan struct{}, maxWorkers),
	}
}

// RegisterWorkflow registers a workflow definition.
func (e *WorkflowEngine) RegisterWorkflow(workflow *Workflow) error {
	if err := workflow.Validate(); err != nil {
		return fmt.Errorf("invalid workflow: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workflows[workflow.ID] = workflow
	return nil
}

// GetWorkflow retrieves a workflow by ID.
func (e *WorkflowEngine) GetWorkflow(id string) (*Workflow, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	workflow, ok := e.workflows[id]
	return workflow, ok
}

// ListWorkflows returns all registered workflows.
func (e *WorkflowEngine) ListWorkflows() []*Workflow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	workflows := make([]*Workflow, 0, len(e.workflows))
	for _, w := range e.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

// StartWorkflow creates and starts a new workflow instance.
func (e *WorkflowEngine) StartWorkflow(workflowID string) (*WorkflowInstance, error) {
	workflow, ok := e.GetWorkflow(workflowID)
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	instance := NewWorkflowInstance(workflowID)
	ready := make(chan struct{})
	instance.readyCh = ready
	e.mu.Lock()
	e.instances[instance.ID] = instance
	e.mu.Unlock()

	go e.runWorkflow(instance, workflow, ready)

	// Wait for workflow to start
	<-ready

	return instance, nil
}

// GetInstance retrieves a workflow instance by ID.
func (e *WorkflowEngine) GetInstance(id string) (*WorkflowInstance, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	instance, ok := e.instances[id]
	return instance, ok
}

// ListInstances returns all workflow instances.
func (e *WorkflowEngine) ListInstances() []*WorkflowInstance {
	e.mu.RLock()
	defer e.mu.RUnlock()
	instances := make([]*WorkflowInstance, 0, len(e.instances))
	for _, i := range e.instances {
		instances = append(instances, i)
	}
	return instances
}

// CancelInstance cancels a running workflow instance.
func (e *WorkflowEngine) CancelInstance(id string) error {
	instance, ok := e.GetInstance(id)
	if !ok {
		return fmt.Errorf("instance not found: %s", id)
	}
	instance.Cancel()
	return nil
}

// runWorkflow executes the workflow DAG.
func (e *WorkflowEngine) runWorkflow(instance *WorkflowInstance, workflow *Workflow, ready chan<- struct{}) {
	instance.SetStatus(StatusRunning)
	instance.SetStartTime(time.Now())
	
	// Signal that workflow has started
	if ready != nil {
		close(ready)
	}

	defer func() {
		instance.SetEndTime(time.Now())
		if instance.GetStatus() == StatusRunning {
			instance.SetStatus(StatusCompleted)
		}
	}()

	completed := make(map[string]bool)
	failed := make(map[string]bool)

	for len(completed)+len(failed) < len(workflow.Tasks) {
		// Check for cancellation
		select {
		case <-instance.Context.Done():
			instance.SetStatus(StatusFailed)
			return
		default:
		}

		// Get ready tasks
		ready := workflow.GetReadyTasks(completed)
		if len(ready) == 0 {
			// No ready tasks but not all completed - check for failures
			if len(failed) > 0 {
				instance.SetStatus(StatusFailed)
				return
			}
			// Wait and retry
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Execute ready tasks in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, task := range ready {
			wg.Add(1)

			// Mark as "in progress" to avoid re-execution
			mu.Lock()
			completed[task.ID] = true // Temporary mark
			mu.Unlock()

			go func(t *Task) {
				defer wg.Done()

				// Acquire worker slot
				e.workerPool <- struct{}{}
				defer func() { <-e.workerPool }()

				result := e.executeTask(instance.Context, t)

				mu.Lock()
				if result.Status == StatusCompleted {
					completed[t.ID] = true
					delete(failed, t.ID)
				} else {
					delete(completed, t.ID)
					failed[t.ID] = true
				}
				mu.Unlock()

				instance.SetResult(t.ID, result)
			}(task)
		}

		wg.Wait()

		// If any task failed and we have dependent tasks, mark them as skipped
		if len(failed) > 0 {
			for _, task := range workflow.Tasks {
				if failed[task.ID] {
					continue
				}
				for _, dep := range task.DependsOn {
					if failed[dep] && !completed[task.ID] {
						instance.SetResult(task.ID, &TaskResult{
							TaskID: task.ID,
							Status: StatusSkipped,
							Error:  fmt.Sprintf("dependency %s failed", dep),
						})
						completed[task.ID] = true
					}
				}
			}
		}
	}

	// Check final status
	if len(failed) > 0 {
		instance.SetStatus(StatusFailed)
	} else {
		instance.SetStatus(StatusCompleted)
	}
}

// executeTask executes a single task with retry logic.
func (e *WorkflowEngine) executeTask(ctx context.Context, task *Task) *TaskResult {
	result := &TaskResult{
		TaskID:    task.ID,
		StartTime: time.Now(),
	}

	maxRetries := task.RetryCount
	if maxRetries <= 0 {
		maxRetries = 0
	}

	retryDelay := task.RetryDelay
	if retryDelay <= 0 {
		retryDelay = time.Second
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		// Check context
		select {
		case <-ctx.Done():
			result.Status = StatusFailed
			result.Error = "task cancelled"
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		default:
		}

		// Execute with timeout
		taskCtx := ctx
		if task.Timeout > 0 {
			var cancel context.CancelFunc
			taskCtx, cancel = context.WithTimeout(ctx, task.Timeout)
			defer cancel()
		}

		output, err := e.executor.Execute(taskCtx, task)
		if err == nil {
			result.Status = StatusCompleted
			result.Output = output
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		}
		lastErr = err
	}

	result.Status = StatusFailed
	result.Error = lastErr.Error()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	return result
}

// DeleteWorkflow removes a workflow definition.
func (e *WorkflowEngine) DeleteWorkflow(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.workflows, id)
	return nil
}

// DeleteInstance removes a workflow instance.
func (e *WorkflowEngine) DeleteInstance(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if instance, ok := e.instances[id]; ok {
		instance.Cancel()
	}
	delete(e.instances, id)
	return nil
}