package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkflow(t *testing.T) {
	tasks := []*Task{
		{ID: "task1", Name: "First Task", Action: "test"},
		{ID: "task2", Name: "Second Task", Action: "test", DependsOn: []string{"task1"}},
	}

	workflow := NewWorkflow("test-workflow", tasks)

	assert.NotEmpty(t, workflow.ID)
	assert.Equal(t, "test-workflow", workflow.Name)
	assert.Len(t, workflow.Tasks, 2)
	assert.False(t, workflow.CreatedAt.IsZero())
	assert.False(t, workflow.UpdatedAt.IsZero())
}

func TestWorkflowValidate(t *testing.T) {
	tests := []struct {
		name      string
		workflow  *Workflow
		wantError bool
	}{
		{
			name: "valid workflow",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test"},
				},
			},
			wantError: false,
		},
		{
			name: "missing name",
			workflow: &Workflow{
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test"},
				},
			},
			wantError: true,
		},
		{
			name: "no tasks",
			workflow: &Workflow{
				Name:  "test",
				Tasks: []*Task{},
			},
			wantError: true,
		},
		{
			name: "duplicate task ID",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test"},
					{ID: "t1", Name: "Task 2", Action: "test"},
				},
			},
			wantError: true,
		},
		{
			name: "missing task ID",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "", Name: "Task 1", Action: "test"},
				},
			},
			wantError: true,
		},
		{
			name: "missing task name",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "", Action: "test"},
				},
			},
			wantError: true,
		},
		{
			name: "missing task action",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: ""},
				},
			},
			wantError: true,
		},
		{
			name: "invalid dependency",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test", DependsOn: []string{"nonexistent"}},
				},
			},
			wantError: true,
		},
		{
			name: "self dependency",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test", DependsOn: []string{"t1"}},
				},
			},
			wantError: true,
		},
		{
			name: "circular dependency",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test", DependsOn: []string{"t2"}},
					{ID: "t2", Name: "Task 2", Action: "test", DependsOn: []string{"t1"}},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.workflow.Validate()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetExecutionOrder(t *testing.T) {
	tests := []struct {
		name     string
		workflow *Workflow
		want     []string
	}{
		{
			name: "single task",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test"},
				},
			},
			want: []string{"t1"},
		},
		{
			name: "linear chain",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test"},
					{ID: "t2", Name: "Task 2", Action: "test", DependsOn: []string{"t1"}},
					{ID: "t3", Name: "Task 3", Action: "test", DependsOn: []string{"t2"}},
				},
			},
			want: []string{"t1", "t2", "t3"},
		},
		{
			name: "parallel tasks",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test"},
					{ID: "t2", Name: "Task 2", Action: "test"},
					{ID: "t3", Name: "Task 3", Action: "test", DependsOn: []string{"t1", "t2"}},
				},
			},
			want: []string{"t1", "t2", "t3"},
		},
		{
			name: "diamond dependency",
			workflow: &Workflow{
				Name: "test",
				Tasks: []*Task{
					{ID: "t1", Name: "Task 1", Action: "test"},
					{ID: "t2", Name: "Task 2", Action: "test", DependsOn: []string{"t1"}},
					{ID: "t3", Name: "Task 3", Action: "test", DependsOn: []string{"t1"}},
					{ID: "t4", Name: "Task 4", Action: "test", DependsOn: []string{"t2", "t3"}},
				},
			},
			want: []string{"t1", "t2", "t3", "t4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := tt.workflow.GetExecutionOrder()
			require.NoError(t, err)
			assert.Equal(t, tt.want, order)
		})
	}
}

func TestGetReadyTasks(t *testing.T) {
	workflow := &Workflow{
		Name: "test",
		Tasks: []*Task{
			{ID: "t1", Name: "Task 1", Action: "test"},
			{ID: "t2", Name: "Task 2", Action: "test", DependsOn: []string{"t1"}},
			{ID: "t3", Name: "Task 3", Action: "test", DependsOn: []string{"t1"}},
			{ID: "t4", Name: "Task 4", Action: "test", DependsOn: []string{"t2", "t3"}},
		},
	}

	// Initially, only t1 is ready
	ready := workflow.GetReadyTasks(map[string]bool{})
	assert.Len(t, ready, 1)
	assert.Equal(t, "t1", ready[0].ID)

	// After t1 completes, t2 and t3 are ready
	ready = workflow.GetReadyTasks(map[string]bool{"t1": true})
	assert.Len(t, ready, 2)

	// After t1, t2, t3 complete, t4 is ready
	ready = workflow.GetReadyTasks(map[string]bool{"t1": true, "t2": true, "t3": true})
	assert.Len(t, ready, 1)
	assert.Equal(t, "t4", ready[0].ID)
}

func TestWorkflowJSON(t *testing.T) {
	workflow := &Workflow{
		ID:          "test-id",
		Name:        "test-workflow",
		Description: "A test workflow",
		Tasks: []*Task{
			{ID: "t1", Name: "Task 1", Action: "test"},
			{ID: "t2", Name: "Task 2", Action: "test", DependsOn: []string{"t1"}},
		},
		Version:   "1.0",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := workflow.ToJSON()
	require.NoError(t, err)

	parsed, err := FromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, workflow.ID, parsed.ID)
	assert.Equal(t, workflow.Name, parsed.Name)
	assert.Equal(t, workflow.Description, parsed.Description)
	assert.Len(t, parsed.Tasks, 2)
	assert.Equal(t, workflow.Version, parsed.Version)
}

func TestWorkflowInstance(t *testing.T) {
	instance := NewWorkflowInstance("workflow-123")

	assert.NotEmpty(t, instance.ID)
	assert.Equal(t, "workflow-123", instance.WorkflowID)
	assert.Equal(t, StatusPending, instance.Status)
	assert.NotNil(t, instance.Context)
	assert.NotNil(t, instance.CancelFunc)

	// Test status update
	instance.SetStatus(StatusRunning)
	assert.Equal(t, StatusRunning, instance.GetStatus())

	// Test result storage
	result := &TaskResult{
		TaskID: "task-1",
		Status: StatusCompleted,
		Output: "success",
	}
	instance.SetResult("task-1", result)

	retrieved, ok := instance.GetResult("task-1")
	assert.True(t, ok)
	assert.Equal(t, result, retrieved)

	// Test cancel
	instance.Cancel()
	assert.Equal(t, StatusFailed, instance.GetStatus())
}

func TestDefaultExecutor(t *testing.T) {
	executor := NewDefaultExecutor()

	// Register a handler
	executor.RegisterActionHandler("test", func(ctx context.Context, task *Task) (interface{}, error) {
		return "test-output", nil
	})

	// Execute
	task := &Task{ID: "t1", Name: "Test", Action: "test"}
	output, err := executor.Execute(context.Background(), task)
	require.NoError(t, err)
	assert.Equal(t, "test-output", output)

	// Unregistered action
	task2 := &Task{ID: "t2", Name: "Test", Action: "unknown"}
	_, err = executor.Execute(context.Background(), task2)
	assert.Error(t, err)
}

func TestWorkflowEngine(t *testing.T) {
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("test", func(ctx context.Context, task *Task) (interface{}, error) {
		return map[string]interface{}{"result": task.ID}, nil
	})

	engine := NewWorkflowEngine(executor, 5)

	// Register workflow
	workflow := NewWorkflow("test-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "test"},
		{ID: "t2", Name: "Task 2", Action: "test", DependsOn: []string{"t1"}},
		{ID: "t3", Name: "Task 3", Action: "test", DependsOn: []string{"t1"}},
		{ID: "t4", Name: "Task 4", Action: "test", DependsOn: []string{"t2", "t3"}},
	})

	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	// Get workflow
	retrieved, ok := engine.GetWorkflow(workflow.ID)
	assert.True(t, ok)
	assert.Equal(t, workflow, retrieved)

	// List workflows
	workflows := engine.ListWorkflows()
	assert.Len(t, workflows, 1)

	// Start workflow
	instance, err := engine.StartWorkflow(workflow.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, instance.ID)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	// Check instance status
	retrievedInstance, ok := engine.GetInstance(instance.ID)
	assert.True(t, ok)
	assert.Equal(t, StatusCompleted, retrievedInstance.GetStatus())

	// Check results
	assert.Len(t, retrievedInstance.Results, 4)

	// List instances
	instances := engine.ListInstances()
	assert.Len(t, instances, 1)

	// Delete workflow
	err = engine.DeleteWorkflow(workflow.ID)
	require.NoError(t, err)

	_, ok = engine.GetWorkflow(workflow.ID)
	assert.False(t, ok)
}

func TestWorkflowEngineWithFailure(t *testing.T) {
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("fail", func(ctx context.Context, task *Task) (interface{}, error) {
		return nil, assert.AnError
	})
	executor.RegisterActionHandler("success", func(ctx context.Context, task *Task) (interface{}, error) {
		return "ok", nil
	})

	engine := NewWorkflowEngine(executor, 5)

	workflow := NewWorkflow("fail-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "fail"},
		{ID: "t2", Name: "Task 2", Action: "success", DependsOn: []string{"t1"}},
	})

	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(workflow.ID)
	require.NoError(t, err)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	retrievedInstance, ok := engine.GetInstance(instance.ID)
	assert.True(t, ok)
	assert.Equal(t, StatusFailed, retrievedInstance.GetStatus())

	// t2 should be skipped
	result, ok := retrievedInstance.GetResult("t2")
	assert.True(t, ok)
	assert.Equal(t, StatusSkipped, result.Status)
}

func TestWorkflowEngineWithRetry(t *testing.T) {
	attempts := 0
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("retry", func(ctx context.Context, task *Task) (interface{}, error) {
		attempts++
		if attempts < 3 {
			return nil, assert.AnError
		}
		return "success", nil
	})

	engine := NewWorkflowEngine(executor, 5)

	workflow := NewWorkflow("retry-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "retry", RetryCount: 3, RetryDelay: 10 * time.Millisecond},
	})

	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(workflow.ID)
	require.NoError(t, err)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	retrievedInstance, ok := engine.GetInstance(instance.ID)
	assert.True(t, ok)
	assert.Equal(t, StatusCompleted, retrievedInstance.GetStatus())
	assert.Equal(t, 3, attempts)
}

func TestWorkflowEngineWithTimeout(t *testing.T) {
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("slow", func(ctx context.Context, task *Task) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return "done", nil
		}
	})

	engine := NewWorkflowEngine(executor, 5)

	workflow := NewWorkflow("timeout-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "slow", Timeout: 100 * time.Millisecond},
	})

	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(workflow.ID)
	require.NoError(t, err)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	retrievedInstance, ok := engine.GetInstance(instance.ID)
	assert.True(t, ok)
	assert.Equal(t, StatusFailed, retrievedInstance.GetStatus())

	result, ok := retrievedInstance.GetResult("t1")
	assert.True(t, ok)
	assert.Equal(t, StatusFailed, result.Status)
}

func TestWorkflowEngineCancel(t *testing.T) {
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("slow", func(ctx context.Context, task *Task) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return "done", nil
		}
	})

	engine := NewWorkflowEngine(executor, 5)

	workflow := NewWorkflow("cancel-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "slow"},
	})

	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(workflow.ID)
	require.NoError(t, err)

	// Cancel after a short delay
	time.Sleep(100 * time.Millisecond)
	err = engine.CancelInstance(instance.ID)
	require.NoError(t, err)

	// Wait for cancellation to take effect
	time.Sleep(100 * time.Millisecond)

	retrievedInstance, ok := engine.GetInstance(instance.ID)
	assert.True(t, ok)
	assert.Equal(t, StatusFailed, retrievedInstance.GetStatus())
}

func TestWorkflowJSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": "test-id",
		"name": "test-workflow",
		"description": "A test workflow",
		"tasks": [
			{"id": "t1", "name": "Task 1", "action": "test"},
			{"id": "t2", "name": "Task 2", "action": "test", "dependsOn": ["t1"]}
		],
		"version": "1.0"
	}`

	workflow, err := FromJSON([]byte(jsonData))
	require.NoError(t, err)

	assert.Equal(t, "test-id", workflow.ID)
	assert.Equal(t, "test-workflow", workflow.Name)
	assert.Len(t, workflow.Tasks, 2)
	assert.Equal(t, "t1", workflow.Tasks[0].ID)
	assert.Equal(t, "t2", workflow.Tasks[1].ID)
	assert.Equal(t, []string{"t1"}, workflow.Tasks[1].DependsOn)
}

func TestWorkflowJSONRoundTrip(t *testing.T) {
	original := &Workflow{
		ID:          "test-id",
		Name:        "test-workflow",
		Description: "A test workflow",
		Tasks: []*Task{
			{
				ID:        "t1",
				Name:      "Task 1",
				Action:    "http",
				Params:    map[string]interface{}{"url": "https://example.com"},
				DependsOn: []string{},
				Timeout:   30 * time.Second,
				RetryCount: 3,
				RetryDelay: 1 * time.Second,
			},
		},
		Version: "1.0",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	parsed, err := FromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, original.ID, parsed.ID)
	assert.Equal(t, original.Name, parsed.Name)
	assert.Equal(t, original.Tasks[0].Timeout, parsed.Tasks[0].Timeout)
	assert.Equal(t, original.Tasks[0].RetryCount, parsed.Tasks[0].RetryCount)
}