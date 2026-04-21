// Package workflow provides DAG-based workflow orchestration for LuckyHarness.
// It supports task dependencies, parallel execution, conditional branching,
// output passing, YAML definitions, persistence, and event callbacks.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Core Types
// ---------------------------------------------------------------------------

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusSkipped   TaskStatus = "skipped"
)

// Condition defines when a task should execute based on upstream results.
type Condition struct {
	// TaskID is the upstream task to check.
	TaskID string `json:"taskId,omitempty" yaml:"taskId,omitempty"`
	// Status matches the task status (completed, failed, etc.)
	Status string `json:"status,omitempty" yaml:"status,omitempty"`
	// Output matches an output field value (simple key=value)
	Output string `json:"output,omitempty" yaml:"output,omitempty"`
}

// Eval evaluates the condition against a task result.
func (c *Condition) Eval(result *TaskResult) bool {
	if c.TaskID == "" {
		return true
	}
	if result == nil {
		return false
	}
	if c.Status != "" && string(result.Status) != c.Status {
		return false
	}
	if c.Output != "" {
		parts := strings.SplitN(c.Output, "=", 2)
		if len(parts) != 2 {
			return false
		}
		key, expected := parts[0], parts[1]
		// Check output map
		if m, ok := result.Output.(map[string]interface{}); ok {
			if val, found := m[key]; found {
				return fmt.Sprintf("%v", val) == expected
			}
		}
		return false
	}
	return true
}

// Task represents a single unit of work in a workflow.
type Task struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Action      string                 `json:"action" yaml:"action"` // "http", "script", "tool", "agent"
	Params      map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
	DependsOn   []string               `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`
	Condition   *Condition             `json:"condition,omitempty" yaml:"condition,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	RetryCount  int                    `json:"retryCount,omitempty" yaml:"retryCount,omitempty"`
	RetryDelay  time.Duration          `json:"retryDelay,omitempty" yaml:"retryDelay,omitempty"`
}

// TaskResult contains the result of a task execution.
type TaskResult struct {
	TaskID    string         `json:"taskId"`
	Status    TaskStatus     `json:"status"`
	Output    interface{}    `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	StartTime time.Time      `json:"startTime"`
	EndTime   time.Time      `json:"endTime"`
	Duration  time.Duration  `json:"duration"`
}

// Workflow represents a DAG-based workflow definition.
type Workflow struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	Tasks       []*Task   `json:"tasks" yaml:"tasks"`
	Version     string    `json:"version,omitempty" yaml:"version,omitempty"`
	CreatedAt   time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" yaml:"updatedAt"`
}

// WorkflowInstance represents a running instance of a workflow.
type WorkflowInstance struct {
	ID         string                 `json:"id"`
	WorkflowID string                 `json:"workflowId"`
	Status     TaskStatus             `json:"status"`
	Results    map[string]*TaskResult `json:"results"`
	StartTime  time.Time              `json:"startTime"`
	EndTime    time.Time              `json:"endTime,omitempty"`
	Context    context.Context        `json:"-"`
	CancelFunc context.CancelFunc     `json:"-"`
	mu         sync.RWMutex
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// NewWorkflow creates a new workflow with the given tasks.
func NewWorkflow(name string, tasks []*Task) *Workflow {
	now := time.Now()
	return &Workflow{
		ID:        uuid.New().String(),
		Name:      name,
		Tasks:     tasks,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewWorkflowInstance creates a new instance of a workflow.
func NewWorkflowInstance(workflowID string) *WorkflowInstance {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkflowInstance{
		ID:         uuid.New().String(),
		WorkflowID: workflowID,
		Status:     StatusPending,
		Results:    make(map[string]*TaskResult),
		Context:    ctx,
		CancelFunc: cancel,
	}
}

// ---------------------------------------------------------------------------
// Validation & DAG
// ---------------------------------------------------------------------------

// Validate checks if the workflow is valid.
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(w.Tasks) == 0 {
		return fmt.Errorf("workflow must have at least one task")
	}

	taskIDs := make(map[string]bool)
	for _, task := range w.Tasks {
		if task.ID == "" {
			return fmt.Errorf("task ID is required")
		}
		if taskIDs[task.ID] {
			return fmt.Errorf("duplicate task ID: %s", task.ID)
		}
		taskIDs[task.ID] = true
		if task.Name == "" {
			return fmt.Errorf("task %s: name is required", task.ID)
		}
		if task.Action == "" {
			return fmt.Errorf("task %s: action is required", task.ID)
		}
		// Validate condition references
		if task.Condition != nil && task.Condition.TaskID != "" {
			if !taskIDs[task.Condition.TaskID] {
				return fmt.Errorf("task %s: condition references non-existent task %s", task.ID, task.Condition.TaskID)
			}
		}
	}

	for _, task := range w.Tasks {
		for _, dep := range task.DependsOn {
			if !taskIDs[dep] {
				return fmt.Errorf("task %s: depends on non-existent task %s", task.ID, dep)
			}
			if dep == task.ID {
				return fmt.Errorf("task %s: cannot depend on itself", task.ID)
			}
		}
	}

	if err := w.detectCycle(); err != nil {
		return err
	}
	return nil
}

// detectCycle uses DFS to detect cycles in the DAG.
func (w *Workflow) detectCycle() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	taskMap := make(map[string]*Task)
	for _, task := range w.Tasks {
		taskMap[task.ID] = task
	}

	var dfs func(taskID string) error
	dfs = func(taskID string) error {
		visited[taskID] = true
		recStack[taskID] = true
		task := taskMap[taskID]
		for _, dep := range task.DependsOn {
			if !visited[dep] {
				if err := dfs(dep); err != nil {
					return err
				}
			} else if recStack[dep] {
				return fmt.Errorf("cycle detected: task %s -> %s", taskID, dep)
			}
		}
		recStack[taskID] = false
		return nil
	}

	for _, task := range w.Tasks {
		if !visited[task.ID] {
			if err := dfs(task.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetExecutionOrder returns tasks in topological order.
func (w *Workflow) GetExecutionOrder() ([]string, error) {
	inDegree := make(map[string]int)
	for _, task := range w.Tasks {
		inDegree[task.ID] = 0
	}
	for _, task := range w.Tasks {
		for range task.DependsOn {
			inDegree[task.ID]++
		}
	}

	queue := make([]string, 0)
	for taskID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, taskID)
		}
	}

	result := make([]string, 0, len(w.Tasks))
	for len(queue) > 0 {
		taskID := queue[0]
		queue = queue[1:]
		result = append(result, taskID)
		for _, task := range w.Tasks {
			for _, dep := range task.DependsOn {
				if dep == taskID {
					inDegree[task.ID]--
					if inDegree[task.ID] == 0 {
						queue = append(queue, task.ID)
					}
				}
			}
		}
	}

	if len(result) != len(w.Tasks) {
		return nil, fmt.Errorf("cycle detected in workflow")
	}
	return result, nil
}

// GetReadyTasks returns tasks that are ready to execute (all dependencies completed).
func (w *Workflow) GetReadyTasks(completed map[string]bool) []*Task {
	ready := make([]*Task, 0)
	for _, task := range w.Tasks {
		if completed[task.ID] {
			continue
		}
		allDepsCompleted := true
		for _, dep := range task.DependsOn {
			if !completed[dep] {
				allDepsCompleted = false
				break
			}
		}
		if allDepsCompleted {
			ready = append(ready, task)
		}
	}
	return ready
}

// ---------------------------------------------------------------------------
// Serialization
// ---------------------------------------------------------------------------

// ToJSON serializes the workflow to JSON.
func (w *Workflow) ToJSON() ([]byte, error) {
	return json.MarshalIndent(w, "", "  ")
}

// FromJSON deserializes a workflow from JSON.
func FromJSON(data []byte) (*Workflow, error) {
	var workflow Workflow
	if err := json.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}
	return &workflow, nil
}

// ToYAML serializes the workflow to YAML.
func (w *Workflow) ToYAML() ([]byte, error) {
	return yaml.Marshal(w)
}

// FromYAML deserializes a workflow from YAML.
func FromYAML(data []byte) (*Workflow, error) {
	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}
	return &workflow, nil
}

// LoadFromFile loads a workflow from a JSON or YAML file.
func LoadFromFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return FromYAML(data)
	case ".json":
		return FromJSON(data)
	default:
		return nil, fmt.Errorf("unsupported workflow file format: %s", ext)
	}
}

// SaveToFile saves a workflow to a JSON or YAML file.
func SaveToFile(w *Workflow, path string) error {
	var data []byte
	var err error
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		data, err = w.ToYAML()
	case ".json":
		data, err = w.ToJSON()
	default:
		return fmt.Errorf("unsupported workflow file format: %s", ext)
	}
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ---------------------------------------------------------------------------
// Instance Helpers
// ---------------------------------------------------------------------------

// SetStatus updates the instance status safely.
func (i *WorkflowInstance) SetStatus(status TaskStatus) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Status = status
}

// GetStatus returns the current status safely.
func (i *WorkflowInstance) GetStatus() TaskStatus {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.Status
}

// SetResult stores a task result safely.
func (i *WorkflowInstance) SetResult(taskID string, result *TaskResult) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Results[taskID] = result
}

// GetResult retrieves a task result safely.
func (i *WorkflowInstance) GetResult(taskID string) (*TaskResult, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	result, ok := i.Results[taskID]
	return result, ok
}

// Snapshot returns a copy of the instance for safe reading.
func (i *WorkflowInstance) Snapshot() *WorkflowInstance {
	i.mu.RLock()
	defer i.mu.RUnlock()
	results := make(map[string]*TaskResult, len(i.Results))
	for k, v := range i.Results {
		resultCopy := *v
		results[k] = &resultCopy
	}
	return &WorkflowInstance{
		ID:         i.ID,
		WorkflowID: i.WorkflowID,
		Status:     i.Status,
		Results:    results,
		StartTime:  i.StartTime,
		EndTime:    i.EndTime,
	}
}

// SetStartTime sets the start time safely.
func (i *WorkflowInstance) SetStartTime(t time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.StartTime = t
}

// SetEndTime sets the end time safely.
func (i *WorkflowInstance) SetEndTime(t time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.EndTime = t
}

// Cancel cancels the workflow instance.
func (i *WorkflowInstance) Cancel() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.CancelFunc != nil {
		i.CancelFunc()
	}
	i.Status = StatusFailed
}

// ---------------------------------------------------------------------------
// WF-2: Persistence
// ---------------------------------------------------------------------------

// FileStore persists workflow definitions and instances to disk.
type FileStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStore creates a file-based workflow store.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{baseDir: baseDir}
}

// SaveWorkflow persists a workflow definition.
func (s *FileStore) SaveWorkflow(w *Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.baseDir, "workflows", w.ID+".json")
	return SaveToFile(w, path)
}

// LoadWorkflow loads a workflow definition by ID.
func (s *FileStore) LoadWorkflow(id string) (*Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path := filepath.Join(s.baseDir, "workflows", id+".json")
	return LoadFromFile(path)
}

// SaveInstance persists a workflow instance.
func (s *FileStore) SaveInstance(inst *WorkflowInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := inst.Snapshot()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.baseDir, "instances", inst.ID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadInstance loads a workflow instance by ID.
func (s *FileStore) LoadInstance(id string) (*WorkflowInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path := filepath.Join(s.baseDir, "instances", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var inst WorkflowInstance
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	inst.Context = ctx
	inst.CancelFunc = cancel
	return &inst, nil
}

// ListWorkflows lists all persisted workflow IDs.
func (s *FileStore) ListWorkflows() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dir := filepath.Join(s.baseDir, "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// WF-3: Event Callbacks
// ---------------------------------------------------------------------------

// EventType identifies a workflow event.
type EventType string

const (
	EventWorkflowStart   EventType = "workflow_start"
	EventWorkflowComplete EventType = "workflow_complete"
	EventTaskStart       EventType = "task_start"
	EventTaskComplete    EventType = "task_complete"
	EventTaskFailed      EventType = "task_failed"
	EventTaskSkipped     EventType = "task_skipped"
)

// Event represents a workflow lifecycle event.
type Event struct {
	Type       EventType     `json:"type"`
	WorkflowID string        `json:"workflowId"`
	InstanceID string        `json:"instanceId"`
	TaskID     string        `json:"taskId,omitempty"`
	TaskName   string        `json:"taskName,omitempty"`
	Status     TaskStatus    `json:"status,omitempty"`
	Result     *TaskResult   `json:"result,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// EventHandler processes workflow events.
type EventHandler func(evt Event)

// EventEmitter manages event handlers.
type EventEmitter struct {
	handlers []EventHandler
	mu       sync.RWMutex
}

// NewEventEmitter creates a new event emitter.
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{}
}

// On registers an event handler.
func (em *EventEmitter) On(handler EventHandler) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.handlers = append(em.handlers, handler)
}

// Emit sends an event to all registered handlers.
func (em *EventEmitter) Emit(evt Event) {
	em.mu.RLock()
	handlers := make([]EventHandler, len(em.handlers))
	copy(handlers, em.handlers)
	em.mu.RUnlock()

	for _, h := range handlers {
		h(evt)
	}
}

// Len returns the number of registered handlers.
func (em *EventEmitter) Len() int {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return len(em.handlers)
}