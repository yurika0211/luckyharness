package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestEngine(t *testing.T) (*WorkflowEngine, *ServerHandlers, *gin.Engine) {
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("test", func(ctx context.Context, task *Task) (interface{}, error) {
		return "ok", nil
	})

	engine := NewWorkflowEngine(executor, 5)
	handlers := NewServerHandlers(engine)

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return engine, handlers, router
}

func TestListWorkflows(t *testing.T) {
	_, _, router := setupTestEngine(t)

	req := httptest.NewRequest("GET", "/api/v1/workflows", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "workflows")
	assert.Contains(t, resp, "count")
}

func TestCreateWorkflow(t *testing.T) {
	_, _, router := setupTestEngine(t)

	workflowReq := CreateWorkflowRequest{
		Name: "test-workflow",
		Tasks: []*Task{
			{ID: "t1", Name: "Task 1", Action: "test"},
		},
	}

	body, err := json.Marshal(workflowReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/workflows", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var workflow Workflow
	err = json.Unmarshal(w.Body.Bytes(), &workflow)
	require.NoError(t, err)
	assert.Equal(t, "test-workflow", workflow.Name)
	assert.Len(t, workflow.Tasks, 1)
}

func TestCreateWorkflowInvalid(t *testing.T) {
	_, _, router := setupTestEngine(t)

	workflowReq := map[string]interface{}{
		"name": "", // Missing name
	}

	body, err := json.Marshal(workflowReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/workflows", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetWorkflow(t *testing.T) {
	engine, _, router := setupTestEngine(t)

	// Create a workflow first
	workflow := NewWorkflow("test-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "test"},
	})
	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/workflows/"+workflow.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var retrieved Workflow
	err = json.Unmarshal(w.Body.Bytes(), &retrieved)
	require.NoError(t, err)
	assert.Equal(t, workflow.ID, retrieved.ID)
}

func TestGetWorkflowNotFound(t *testing.T) {
	_, _, router := setupTestEngine(t)

	req := httptest.NewRequest("GET", "/api/v1/workflows/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteWorkflow(t *testing.T) {
	engine, _, router := setupTestEngine(t)

	workflow := NewWorkflow("test-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "test"},
	})
	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/workflows/"+workflow.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify deleted
	_, ok := engine.GetWorkflow(workflow.ID)
	assert.False(t, ok)
}

func TestListInstances(t *testing.T) {
	_, _, router := setupTestEngine(t)

	req := httptest.NewRequest("GET", "/api/v1/workflow-instances", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "instances")
	assert.Contains(t, resp, "count")
}

func TestStartWorkflow(t *testing.T) {
	engine, _, router := setupTestEngine(t)

	// Register action handler
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("test", func(ctx context.Context, task *Task) (interface{}, error) {
		return "ok", nil
	})
	engine.executor = executor

	workflow := NewWorkflow("test-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "test"},
	})
	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	startReq := StartWorkflowRequest{
		WorkflowID: workflow.ID,
	}

	body, err := json.Marshal(startReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/workflow-instances", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var instance WorkflowInstance
	err = json.Unmarshal(w.Body.Bytes(), &instance)
	require.NoError(t, err)
	assert.NotEmpty(t, instance.ID)
	assert.Equal(t, workflow.ID, instance.WorkflowID)
}

func TestStartWorkflowNotFound(t *testing.T) {
	_, _, router := setupTestEngine(t)

	startReq := StartWorkflowRequest{
		WorkflowID: "nonexistent",
	}

	body, err := json.Marshal(startReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/workflow-instances", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetInstance(t *testing.T) {
	engine, _, router := setupTestEngine(t)

	// Register action handler
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("test", func(ctx context.Context, task *Task) (interface{}, error) {
		return "ok", nil
	})
	engine.executor = executor

	workflow := NewWorkflow("test-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "test"},
	})
	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(workflow.ID)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/workflow-instances/"+instance.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetInstanceNotFound(t *testing.T) {
	_, _, router := setupTestEngine(t)

	req := httptest.NewRequest("GET", "/api/v1/workflow-instances/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetInstanceResults(t *testing.T) {
	engine, _, router := setupTestEngine(t)

	// Register action handler
	executor := NewDefaultExecutor()
	executor.RegisterActionHandler("test", func(ctx context.Context, task *Task) (interface{}, error) {
		return "ok", nil
	})
	engine.executor = executor

	workflow := NewWorkflow("test-workflow", []*Task{
		{ID: "t1", Name: "Task 1", Action: "test"},
	})
	err := engine.RegisterWorkflow(workflow)
	require.NoError(t, err)

	instance, err := engine.StartWorkflow(workflow.ID)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/workflow-instances/"+instance.ID+"/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "instanceId")
	assert.Contains(t, resp, "status")
	assert.Contains(t, resp, "results")
}