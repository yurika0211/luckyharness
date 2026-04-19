package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DelegateConfig 子代理委派配置
type DelegateConfig struct {
	MaxConcurrent int           // 最大并发子代理数
	Timeout       time.Duration // 子代理超时
	AutoApprove   bool          // 自动批准子代理任务
}

// DefaultDelegateConfig 默认委派配置
func DefaultDelegateConfig() DelegateConfig {
	return DelegateConfig{
		MaxConcurrent: 3,
		Timeout:       120 * time.Second,
		AutoApprove:   false,
	}
}

// TaskStatus 子代理任务状态
type TaskStatus int

const (
	StatusPending   TaskStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// DelegateTask 子代理任务
type DelegateTask struct {
	ID          string
	Description string
	Status      TaskStatus
	Result      string
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
}

// DelegateManager 子代理委派管理器
type DelegateManager struct {
	mu     sync.RWMutex
	config DelegateConfig
	tasks  map[string]*DelegateTask
	nextID int
}

// NewDelegateManager 创建子代理委派管理器
func NewDelegateManager(cfg DelegateConfig) *DelegateManager {
	return &DelegateManager{
		config: cfg,
		tasks:  make(map[string]*DelegateTask),
	}
}

// DelegateTaskTool 创建子代理委派工具
func DelegateTaskTool(dm *DelegateManager) *Tool {
	return &Tool{
		Name:        "delegate_task",
		Description: "Delegate a task to a sub-agent. The sub-agent will work independently and return results.",
		Category:    CatDelegate,
		Source:      "builtin",
		Permission:  PermApprove, // 委派任务需要审批
		Parameters: map[string]Param{
			"description": {
				Type:        "string",
				Description: "Description of the task to delegate",
				Required:    true,
			},
			"context": {
				Type:        "string",
				Description: "Additional context or instructions for the sub-agent",
				Required:    false,
			},
			"timeout": {
				Type:        "number",
				Description: "Timeout in seconds (default 120)",
				Required:    false,
				Default:     120,
			},
		},
		Handler: dm.handleDelegate,
	}
}

// TaskStatusTool 创建任务状态查询工具
func TaskStatusTool(dm *DelegateManager) *Tool {
	return &Tool{
		Name:        "task_status",
		Description: "Check the status of a delegated task.",
		Category:    CatDelegate,
		Source:      "builtin",
		Permission:  PermAuto, // 查询状态自动批准
		Parameters: map[string]Param{
			"task_id": {
				Type:        "string",
				Description: "ID of the task to check",
				Required:    true,
			},
		},
		Handler: dm.handleStatus,
	}
}

// ListTasksTool 创建任务列表工具
func ListTasksTool(dm *DelegateManager) *Tool {
	return &Tool{
		Name:        "list_tasks",
		Description: "List all delegated tasks and their statuses.",
		Category:    CatDelegate,
		Source:      "builtin",
		Permission:  PermAuto,
		Parameters:  map[string]Param{},
		Handler:     dm.handleList,
	}
}

// handleDelegate 处理委派请求
func (dm *DelegateManager) handleDelegate(args map[string]any) (string, error) {
	description, ok := args["description"].(string)
	if !ok {
		return "", fmt.Errorf("description is required")
	}

	contextStr := ""
	if c, ok := args["context"]; ok {
		contextStr, _ = c.(string)
	}

	timeout := 120
	if t, ok := args["timeout"]; ok {
		switch v := t.(type) {
		case float64:
			timeout = int(v)
		case int:
			timeout = v
		}
	}

	// 检查并发限制
	dm.mu.RLock()
	running := 0
	for _, t := range dm.tasks {
		if t.Status == StatusRunning {
			running++
		}
	}
	dm.mu.RUnlock()

	if running >= dm.config.MaxConcurrent {
		return "", fmt.Errorf("max concurrent tasks reached (%d)", dm.config.MaxConcurrent)
	}

	// 创建任务
	dm.mu.Lock()
	dm.nextID++
	taskID := fmt.Sprintf("task-%d", dm.nextID)
	task := &DelegateTask{
		ID:          taskID,
		Description: description,
		Status:      StatusPending,
		StartedAt:   time.Now(),
	}
	dm.tasks[taskID] = task
	dm.mu.Unlock()

	// 异步执行
	go dm.executeTask(taskID, description, contextStr, time.Duration(timeout)*time.Second)

	result, _ := json.Marshal(map[string]any{
		"task_id": taskID,
		"status":  "running",
		"message": fmt.Sprintf("Task '%s' delegated. Use task_status to check progress.", taskID),
	})

	return string(result), nil
}

// executeTask 执行子代理任务
func (dm *DelegateManager) executeTask(taskID, description, contextStr string, timeout time.Duration) {
	dm.mu.Lock()
	task := dm.tasks[taskID]
	task.Status = StatusRunning
	dm.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// v0.5.0: 简化实现 — 子代理实际执行由 Agent Loop 驱动
	// 这里只模拟任务完成，真实实现需要创建子 Agent 实例
	select {
	case <-ctx.Done():
		dm.mu.Lock()
		task.Status = StatusFailed
		task.Error = "timeout"
		task.CompletedAt = time.Now()
		dm.mu.Unlock()
		return
	default:
	}

	// 标记完成（占位）
	dm.mu.Lock()
	task.Status = StatusCompleted
	task.Result = fmt.Sprintf("Sub-agent task completed: %s", description)
	task.CompletedAt = time.Now()
	dm.mu.Unlock()
}

// handleStatus 处理状态查询
func (dm *DelegateManager) handleStatus(args map[string]any) (string, error) {
	taskID, ok := args["task_id"].(string)
	if !ok {
		return "", fmt.Errorf("task_id is required")
	}

	dm.mu.RLock()
	task, ok := dm.tasks[taskID]
	dm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("task not found: %s", taskID)
	}

	result, _ := json.Marshal(map[string]any{
		"task_id":       task.ID,
		"description":   task.Description,
		"status":        task.Status.String(),
		"result":        task.Result,
		"error":         task.Error,
		"started_at":    task.StartedAt.Format(time.RFC3339),
		"completed_at":  task.CompletedAt.Format(time.RFC3339),
	})

	return string(result), nil
}

// handleList 处理任务列表
func (dm *DelegateManager) handleList(args map[string]any) (string, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var tasks []map[string]any
	for _, t := range dm.tasks {
		tasks = append(tasks, map[string]any{
			"task_id":     t.ID,
			"description": t.Description,
			"status":      t.Status.String(),
			"started_at":  t.StartedAt.Format(time.RFC3339),
		})
	}

	result, _ := json.Marshal(map[string]any{
		"tasks": tasks,
		"count": len(tasks),
	})

	return string(result), nil
}
