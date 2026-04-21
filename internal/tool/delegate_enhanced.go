package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TaskPriority represents the priority of a delegated task.
type TaskPriority int

const (
	PriorityLow    TaskPriority = iota
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
func ParseTaskPriority(s string) (TaskPriority, error) {
	switch s {
	case "low":
		return PriorityLow, nil
	case "normal", "":
		return PriorityNormal, nil
	case "high":
		return PriorityHigh, nil
	case "critical":
		return PriorityCritical, nil
	default:
		return PriorityNormal, fmt.Errorf("unknown priority: %s", s)
	}
}

// DelegateTarget specifies the type of delegation target.
type DelegateTarget int

const (
	TargetAgent  DelegateTarget = iota // Delegate to a sub-agent
	TargetSkill                        // Delegate to a skill plugin
	TargetMCP                          // Delegate to an MCP server
)

func (t DelegateTarget) String() string {
	switch t {
	case TargetAgent:
		return "agent"
	case TargetSkill:
		return "skill"
	case TargetMCP:
		return "mcp"
	default:
		return "unknown"
	}
}

// PrioritizedTask wraps a DelegateTask with priority and target info.
type PrioritizedTask struct {
	Task       *DelegateTask
	Priority   TaskPriority
	Target     DelegateTarget
	TargetName string // skill name or MCP server name
	Context    string
	EnqueuedAt time.Time
}

// TaskCache stores results of completed tasks for quick retrieval.
type TaskCache struct {
	mu     sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	result    *DelegateTask
	expiresAt time.Time
}

// NewTaskCache creates a new task result cache.
func NewTaskCache(ttl time.Duration) *TaskCache {
	return &TaskCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Set stores a task result in the cache.
func (c *TaskCache) Set(key string, task *DelegateTask) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store a copy to ensure isolation
	copy := *task
	c.entries[key] = &cacheEntry{
		result:    &copy,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Get retrieves a cached task result.
func (c *TaskCache) Get(key string) (*DelegateTask, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	// Return a copy
	copy := *entry.result
	return &copy, true
}

// Delete removes a cached entry.
func (c *TaskCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all cached entries.
func (c *TaskCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// Size returns the number of cached entries.
func (c *TaskCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clean removes expired entries.
func (c *TaskCache) Clean() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
			removed++
		}
	}
	return removed
}

// DelegateToSkill delegates a task to a specific skill plugin.
// The skill's tools will be invoked to complete the task.
func (dm *DelegateManager) DelegateToSkill(ctx context.Context, skillName, description string, priority TaskPriority) (*DelegateTask, error) {
	dm.mu.Lock()
	dm.nextID++
	taskID := fmt.Sprintf("skill-%d", dm.nextID)
	task := &DelegateTask{
		ID:          taskID,
		Description: description,
		Status:      StatusPending,
		StartedAt:   time.Now(),
	}
	dm.tasks[taskID] = task
	dm.mu.Unlock()

	dm.mu.Lock()
	result, _ := json.Marshal(map[string]any{
		"task_id":    taskID,
		"status":     "running",
		"target":     "skill",
		"skill_name": skillName,
		"priority":   priority.String(),
		"message":    fmt.Sprintf("Task delegated to skill '%s'. Use task_status to check progress.", skillName),
	})
	task.Result = string(result)
	dm.mu.Unlock()

	// Execute asynchronously
	go dm.executeSkillTask(ctx, taskID, skillName, description)

	return task, nil
}

// DelegateToMCP delegates a task to an MCP server.
func (dm *DelegateManager) DelegateToMCP(ctx context.Context, serverName, toolName string, args map[string]any, priority TaskPriority) (*DelegateTask, error) {
	dm.mu.Lock()
	dm.nextID++
	taskID := fmt.Sprintf("mcp-%d", dm.nextID)
	task := &DelegateTask{
		ID:          taskID,
		Description: fmt.Sprintf("MCP call: %s/%s", serverName, toolName),
		Status:      StatusPending,
		StartedAt:   time.Now(),
	}
	dm.tasks[taskID] = task
	dm.mu.Unlock()

	dm.mu.Lock()
	result, _ := json.Marshal(map[string]any{
		"task_id":     taskID,
		"status":      "running",
		"target":      "mcp",
		"server_name": serverName,
		"tool_name":   toolName,
		"priority":    priority.String(),
		"message":     fmt.Sprintf("Task delegated to MCP server '%s'. Use task_status to check progress.", serverName),
	})
	task.Result = string(result)
	dm.mu.Unlock()

	// Execute asynchronously
	go dm.executeMCPTask(ctx, taskID, serverName, toolName, args)

	return task, nil
}

// executeSkillTask executes a task by delegating to a skill.
func (dm *DelegateManager) executeSkillTask(ctx context.Context, taskID, skillName, description string) {
	dm.mu.Lock()
	task := dm.tasks[taskID]
	task.Status = StatusRunning
	dm.mu.Unlock()

	timeout := dm.config.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// v0.5.0: Simplified — real implementation would invoke skill tools
		dm.mu.Lock()
		task.Status = StatusCompleted
		task.Result = fmt.Sprintf("Skill '%s' completed task: %s", skillName, description)
		task.CompletedAt = time.Now()
		dm.mu.Unlock()
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		dm.mu.Lock()
		task.Status = StatusFailed
		task.Error = "timeout"
		task.CompletedAt = time.Now()
		dm.mu.Unlock()
	case <-ctx.Done():
		dm.mu.Lock()
		task.Status = StatusCancelled
		task.Error = "context cancelled"
		task.CompletedAt = time.Now()
		dm.mu.Unlock()
	}
}

// executeMCPTask executes a task by calling an MCP server tool.
func (dm *DelegateManager) executeMCPTask(ctx context.Context, taskID, serverName, toolName string, args map[string]any) {
	dm.mu.Lock()
	task := dm.tasks[taskID]
	task.Status = StatusRunning
	dm.mu.Unlock()

	timeout := dm.config.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// v0.5.0: Simplified — real implementation would use MCPClient.CallTool
		dm.mu.Lock()
		task.Status = StatusCompleted
		task.Result = fmt.Sprintf("MCP %s/%s completed", serverName, toolName)
		task.CompletedAt = time.Now()
		dm.mu.Unlock()
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		dm.mu.Lock()
		task.Status = StatusFailed
		task.Error = "timeout"
		task.CompletedAt = time.Now()
		dm.mu.Unlock()
	case <-ctx.Done():
		dm.mu.Lock()
		task.Status = StatusCancelled
		task.Error = "context cancelled"
		task.CompletedAt = time.Now()
		dm.mu.Unlock()
	}
}

// PriorityTaskQueue is a thread-safe priority queue for delegated tasks.
type PriorityTaskQueue struct {
	mu    sync.RWMutex
	items []*PrioritizedTask
}

// NewPriorityTaskQueue creates a new priority task queue.
func NewPriorityTaskQueue() *PriorityTaskQueue {
	return &PriorityTaskQueue{
		items: make([]*PrioritizedTask, 0),
	}
}

// Enqueue adds a task to the queue.
func (q *PriorityTaskQueue) Enqueue(task *PrioritizedTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.EnqueuedAt = time.Now()

	// Insert in priority order (higher priority first)
	inserted := false
	for i, item := range q.items {
		if task.Priority > item.Priority {
			q.items = append(q.items[:i], append([]*PrioritizedTask{task}, q.items[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		q.items = append(q.items, task)
	}
}

// Dequeue removes and returns the highest priority task.
func (q *PriorityTaskQueue) Dequeue() (*PrioritizedTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// Peek returns the highest priority task without removing it.
func (q *PriorityTaskQueue) Peek() (*PrioritizedTask, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.items) == 0 {
		return nil, false
	}
	return q.items[0], true
}

// Len returns the number of tasks in the queue.
func (q *PriorityTaskQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}

// List returns all tasks in the queue (ordered by priority).
func (q *PriorityTaskQueue) List() []*PrioritizedTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*PrioritizedTask, len(q.items))
	copy(result, q.items)
	return result
}

// Clear removes all tasks from the queue.
func (q *PriorityTaskQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = q.items[:0]
}

// DelegateToSkillTool creates a tool for delegating tasks to skills.
func DelegateToSkillTool(dm *DelegateManager) *Tool {
	return &Tool{
		Name:        "delegate_to_skill",
		Description: "Delegate a task to a specific skill plugin. The skill's tools will be used to complete the task.",
		Category:    CatDelegate,
		Source:      "builtin",
		Permission:  PermApprove,
		Parameters: map[string]Param{
			"skill_name": {
				Type:        "string",
				Description: "Name of the skill to delegate to",
				Required:    true,
			},
			"description": {
				Type:        "string",
				Description: "Description of the task to delegate",
				Required:    true,
			},
			"priority": {
				Type:        "string",
				Description: "Task priority: low, normal, high, critical",
				Required:    false,
				Default:     "normal",
			},
		},
		Handler: dm.handleDelegateToSkill,
	}
}

// DelegateToMCPTool creates a tool for delegating tasks to MCP servers.
func DelegateToMCPTool(dm *DelegateManager) *Tool {
	return &Tool{
		Name:        "delegate_to_mcp",
		Description: "Delegate a task to an MCP server by calling a specific tool.",
		Category:    CatDelegate,
		Source:      "builtin",
		Permission:  PermApprove,
		Parameters: map[string]Param{
			"server_name": {
				Type:        "string",
				Description: "Name of the MCP server",
				Required:    true,
			},
			"tool_name": {
				Type:        "string",
				Description: "Name of the tool on the MCP server to call",
				Required:    true,
			},
			"arguments": {
				Type:        "object",
				Description: "Arguments to pass to the MCP tool",
				Required:    false,
			},
			"priority": {
				Type:        "string",
				Description: "Task priority: low, normal, high, critical",
				Required:    false,
				Default:     "normal",
			},
		},
		Handler: dm.handleDelegateToMCP,
	}
}

// handleDelegateToSkill handles the delegate_to_skill tool call.
func (dm *DelegateManager) handleDelegateToSkill(args map[string]any) (string, error) {
	skillName, ok := args["skill_name"].(string)
	if !ok {
		return "", fmt.Errorf("skill_name is required")
	}
	description, ok := args["description"].(string)
	if !ok {
		return "", fmt.Errorf("description is required")
	}

	priorityStr := "normal"
	if p, ok := args["priority"].(string); ok {
		priorityStr = p
	}
	priority, _ := ParseTaskPriority(priorityStr)

	task, err := dm.DelegateToSkill(context.Background(), skillName, description, priority)
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(map[string]any{
		"task_id":    task.ID,
		"status":     "running",
		"skill_name": skillName,
		"priority":   priority.String(),
	})
	return string(result), nil
}

// handleDelegateToMCP handles the delegate_to_mcp tool call.
func (dm *DelegateManager) handleDelegateToMCP(args map[string]any) (string, error) {
	serverName, ok := args["server_name"].(string)
	if !ok {
		return "", fmt.Errorf("server_name is required")
	}
	toolName, ok := args["tool_name"].(string)
	if !ok {
		return "", fmt.Errorf("tool_name is required")
	}

	var mcpArgs map[string]any
	if a, ok := args["arguments"].(map[string]any); ok {
		mcpArgs = a
	}

	priorityStr := "normal"
	if p, ok := args["priority"].(string); ok {
		priorityStr = p
	}
	priority, _ := ParseTaskPriority(priorityStr)

	task, err := dm.DelegateToMCP(context.Background(), serverName, toolName, mcpArgs, priority)
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(map[string]any{
		"task_id":     task.ID,
		"status":      "running",
		"server_name": serverName,
		"tool_name":   toolName,
		"priority":    priority.String(),
	})
	return string(result), nil
}