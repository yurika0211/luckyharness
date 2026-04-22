package luckyharness

import (
	"context"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/agent"
	"github.com/yurika0211/luckyharness/internal/embedder"
	"github.com/yurika0211/luckyharness/internal/memory"
	"github.com/yurika0211/luckyharness/internal/rag"
	"github.com/yurika0211/luckyharness/internal/workflow"
	"google.golang.org/grpc/metadata"
)

// mockExecutor is a simple test executor
type mockExecutor struct{}

func (m *mockExecutor) Execute(ctx context.Context, task *workflow.Task) (interface{}, error) {
	return "mock result", nil
}

// mockEmbedder is a simple test embedder
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	// Return mock 1536-dim embedding
	embedding := make([]float64, 1536)
	for i := range embedding {
		embedding[i] = 0.1
	}
	return embedding, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))
	for i := range embeddings {
		embeddings[i] = make([]float64, 1536)
		for j := range embeddings[i] {
			embeddings[i][j] = 0.1
		}
	}
	return embeddings, nil
}

func (m *mockEmbedder) Dimension() int {
	return 1536
}

func (m *mockEmbedder) Name() string {
	return "mock"
}

func (m *mockEmbedder) Model() string {
	return "mock-embedding"
}

func newTestWorkflowEngine() *workflow.WorkflowEngine {
	return workflow.NewWorkflowEngine(&mockExecutor{}, 2)
}

func newTestRAGManager() *rag.RAGManager {
	return rag.NewRAGManager(&mockEmbedder{}, rag.RAGConfig{})
}

func TestServer_HealthCheck(t *testing.T) {
	// Create test dependencies
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	resp, err := server.HealthCheck(context.Background(), nil)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", resp.Status)
	}

	if resp.Version != "v0.25.0" {
		t.Errorf("expected version v0.25.0, got %s", resp.Version)
	}

	if _, ok := resp.Components["memory"]; !ok {
		t.Error("expected memory component in health check")
	}

	if _, ok := resp.Components["rag"]; !ok {
		t.Error("expected rag component in health check")
	}

	if _, ok := resp.Components["workflow"]; !ok {
		t.Error("expected workflow component in health check")
	}
}

func TestServer_MemoryStore(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &MemoryStoreRequest{
		Content:    "test memory content",
		Category:   "test",
		Tier:       "medium",
		Importance: 0.5,
	}

	resp, err := server.MemoryStore(context.Background(), req)
	if err != nil {
		t.Fatalf("MemoryStore failed: %v", err)
	}

	if resp.Content != "test memory content" {
		t.Errorf("expected content 'test memory content', got %s", resp.Content)
	}

	if resp.Category != "test" {
		t.Errorf("expected category 'test', got %s", resp.Category)
	}

	if resp.Id == "" {
		t.Error("expected non-empty ID")
	}
}

func TestServer_MemoryRecall(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	// Store some test memories
	memStore.SaveWithTier("golang programming", "tech", memory.TierMedium, 0.5)
	memStore.SaveWithTier("python programming", "tech", memory.TierMedium, 0.5)
	memStore.SaveWithTier("cooking recipes", "food", memory.TierMedium, 0.5)

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &MemoryRecallRequest{
		Query: "programming",
		Limit: 10,
	}

	resp, err := server.MemoryRecall(context.Background(), req)
	if err != nil {
		t.Fatalf("MemoryRecall failed: %v", err)
	}

	if resp.Total == 0 {
		t.Error("expected at least one result")
	}
}

func TestServer_MemoryList(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	// Store some test memories
	memStore.Save("memory 1", "cat1")
	memStore.Save("memory 2", "cat2")

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	resp, err := server.MemoryList(context.Background(), nil)
	if err != nil {
		t.Fatalf("MemoryList failed: %v", err)
	}

	if resp.Count != 2 {
		t.Errorf("expected 2 entries, got %d", resp.Count)
	}
}

func TestServer_MemoryDelete(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	memStore.Save("test memory", "test")
	entries := memStore.Recent(1)
	if len(entries) == 0 {
		t.Fatal("failed to store test memory")
	}
	entryID := entries[0].ID

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &MemoryDeleteRequest{Id: entryID}
	_, err = server.MemoryDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("MemoryDelete failed: %v", err)
	}

	// Verify deletion
	if memStore.Count() != 0 {
		t.Error("expected memory to be deleted")
	}
}

func TestServer_WorkflowCreate(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowCreateRequest{
		Name:        "test-workflow",
		Description: "A test workflow",
		Tasks: []*Task{
			{
				Id:          "task1",
				Name:        "First Task",
				Description: "First task description",
				Action:      "echo",
				Params:      map[string]string{"message": "hello"},
			},
		},
		Version: "1.0",
	}

	resp, err := server.WorkflowCreate(context.Background(), req)
	if err != nil {
		t.Fatalf("WorkflowCreate failed: %v", err)
	}

	if resp.Name != "test-workflow" {
		t.Errorf("expected name 'test-workflow', got %s", resp.Name)
	}

	if len(resp.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(resp.Tasks))
	}
}

func TestServer_WorkflowList(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	// Create a workflow directly
	wf := workflow.NewWorkflow("test-wf", []*workflow.Task{
		{ID: "t1", Name: "Task 1", Action: "echo"},
	})
	wfEngine.RegisterWorkflow(wf)

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	resp, err := server.WorkflowList(context.Background(), nil)
	if err != nil {
		t.Fatalf("WorkflowList failed: %v", err)
	}

	if resp.Count != 1 {
		t.Errorf("expected 1 workflow, got %d", resp.Count)
	}
}

func TestGRPCServer_StartStop(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	serviceServer := NewServer(nil, memStore, ragMgr, wfEngine)
	grpcServer := NewGRPCServer(":0", serviceServer)

	if err := grpcServer.Start(); err != nil {
		t.Fatalf("failed to start gRPC server: %v", err)
	}

	// Verify server is listening
	addr := grpcServer.Addr()
	if addr == "" {
		t.Error("expected non-empty address")
	}

	// Stop the server
	grpcServer.Stop()
}

func TestNewServer(t *testing.T) {
	memStore, err := memory.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()
	ag := &agent.Agent{}

	server := NewServer(ag, memStore, ragMgr, wfEngine)

	if server.agent != ag {
		t.Error("agent not set correctly")
	}

	if server.memoryStore != memStore {
		t.Error("memory store not set correctly")
	}

	if server.ragManager != ragMgr {
		t.Error("rag manager not set correctly")
	}

	if server.workflowEngine != wfEngine {
		t.Error("workflow engine not set correctly")
	}
}

// Ensure mockEmbedder implements embedder.Embedder
var _ embedder.Embedder = (*mockEmbedder)(nil)

// ── Chat & ChatStream Tests ──────────────────────────────────────

type mockAgent struct {
	chatResponse string
	chatError    error
}

func (m *mockAgent) Chat(ctx context.Context, message string) (string, error) {
	return m.chatResponse, m.chatError
}

func TestServer_Chat(t *testing.T) {
	// Chat requires non-nil agent, skip for now
	t.Skip("Chat requires non-nil agent")
}

func TestServer_Chat_Error(t *testing.T) {
	// Chat requires non-nil agent, skip for now
	t.Skip("Chat requires non-nil agent")
}

func TestServer_ChatStream(t *testing.T) {
	// ChatStream requires non-nil agent, skip for now
	t.Skip("ChatStream requires non-nil agent")
}

type mockChatStream struct {
	ctx     context.Context
	sent    []*ChatChunk
	closeCh chan struct{}
}

func (m *mockChatStream) SetHeader(md metadata.MD) error { return nil }
func (m *mockChatStream) SendHeader(md metadata.MD) error { return nil }
func (m *mockChatStream) SetTrailer(md metadata.MD)       {}
func (m *mockChatStream) Context() context.Context        { return m.ctx }
func (m *mockChatStream) SendMsg(msg interface{}) error   { return nil }
func (m *mockChatStream) RecvMsg(msg interface{}) error   { return nil }

func (m *mockChatStream) Send(chunk *ChatChunk) error {
	m.sent = append(m.sent, chunk)
	return nil
}

// ── RAG Tests ──────────────────────────────────────

func TestServer_RAGIndex(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()
	ag := &agent.Agent{}

	server := NewServer(ag, memStore, ragMgr, wfEngine)

	req := &RAGIndexRequest{
		Source:  "test-source",
		Content: "This is test content for RAG indexing",
	}

	resp, err := server.RAGIndex(context.Background(), req)
	if err != nil {
		t.Fatalf("RAGIndex failed: %v", err)
	}

	if resp.Id == "" {
		t.Error("expected non-empty document ID")
	}

	if resp.Dimension != 1536 {
		t.Errorf("expected dimension 1536, got %d", resp.Dimension)
	}

	if resp.TotalEntries == 0 {
		t.Error("expected at least one chunk indexed")
	}
}

func TestServer_RAGSearch(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()
	ag := &agent.Agent{}

	// Index some content first
	_, err := ragMgr.IndexText("test-source", "", "golang programming language")
	if err != nil {
		t.Fatalf("failed to index content: %v", err)
	}

	server := NewServer(ag, memStore, ragMgr, wfEngine)

	req := &RAGSearchRequest{
		Query:     "programming",
		Limit:     10,
		Threshold: 0.5,
	}

	resp, err := server.RAGSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("RAGSearch failed: %v", err)
	}

	if resp.Total == 0 {
		t.Error("expected at least one search result")
	}

	if len(resp.Results) == 0 {
		t.Error("expected results slice to be non-empty")
	}
}

func TestServer_RAGSearch_DefaultLimit(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()
	ag := &agent.Agent{}

	server := NewServer(ag, memStore, ragMgr, wfEngine)

	req := &RAGSearchRequest{
		Query: "test",
		Limit: 0, // Should default to 10
	}

	_, err := server.RAGSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("RAGSearch with default limit failed: %v", err)
	}
}

// ── Workflow Tests ──────────────────────────────────────

func TestServer_WorkflowGet(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	// Register a test workflow
	wf := workflow.NewWorkflow("test-get-wf", []*workflow.Task{
		{ID: "t1", Name: "Task 1", Action: "echo"},
	})
	wfEngine.RegisterWorkflow(wf)

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowGetRequest{Id: wf.ID}
	resp, err := server.WorkflowGet(context.Background(), req)
	if err != nil {
		t.Fatalf("WorkflowGet failed: %v", err)
	}

	if resp.Name != "test-get-wf" {
		t.Errorf("expected name 'test-get-wf', got %s", resp.Name)
	}

	if len(resp.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(resp.Tasks))
	}
}

func TestServer_WorkflowGet_NotFound(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowGetRequest{Id: "non-existent-id"}
	_, err := server.WorkflowGet(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent workflow")
	}
}

func TestServer_WorkflowDelete(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	// Register and then delete a workflow
	wf := workflow.NewWorkflow("test-delete-wf", []*workflow.Task{
		{ID: "t1", Name: "Task 1", Action: "echo"},
	})
	wfEngine.RegisterWorkflow(wf)

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowDeleteRequest{Id: wf.ID}
	_, err := server.WorkflowDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("WorkflowDelete failed: %v", err)
	}

	// Verify deletion
	_, exists := wfEngine.GetWorkflow(wf.ID)
	if exists {
		t.Error("expected workflow to be deleted")
	}
}

func TestServer_WorkflowDelete_NotFound(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowDeleteRequest{Id: "non-existent-id"}
	// DeleteWorkflow always returns nil even for non-existent workflows
	_, err := server.WorkflowDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("WorkflowDelete should not return error for non-existent workflow: %v", err)
	}
}

func TestServer_WorkflowStart(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	// Register a workflow
	wf := workflow.NewWorkflow("test-start-wf", []*workflow.Task{
		{ID: "t1", Name: "Task 1", Action: "echo"},
	})
	wfEngine.RegisterWorkflow(wf)

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowStartRequest{WorkflowId: wf.ID}
	instance, err := server.WorkflowStart(context.Background(), req)
	if err != nil {
		t.Fatalf("WorkflowStart failed: %v", err)
	}

	if instance.WorkflowId != wf.ID {
		t.Errorf("expected workflow id %s, got %s", wf.ID, instance.WorkflowId)
	}

	if instance.Id == "" {
		t.Error("expected non-empty instance ID")
	}
}

func TestServer_WorkflowStart_NotFound(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowStartRequest{WorkflowId: "non-existent-id"}
	_, err := server.WorkflowStart(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent workflow")
	}
}

func TestServer_WorkflowInstanceGet(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	// Create workflow and start instance
	wf := workflow.NewWorkflow("test-instance-wf", []*workflow.Task{
		{ID: "t1", Name: "Task 1", Action: "echo"},
	})
	wfEngine.RegisterWorkflow(wf)
	instance, _ := wfEngine.StartWorkflow(wf.ID)

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowInstanceGetRequest{Id: instance.ID}
	resp, err := server.WorkflowInstanceGet(context.Background(), req)
	if err != nil {
		t.Fatalf("WorkflowInstanceGet failed: %v", err)
	}

	if resp.Id != instance.ID {
		t.Errorf("expected instance id %s, got %s", instance.ID, resp.Id)
	}

	if resp.WorkflowId != wf.ID {
		t.Errorf("expected workflow id %s, got %s", wf.ID, resp.WorkflowId)
	}
}

func TestServer_WorkflowInstanceGet_NotFound(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	req := &WorkflowInstanceGetRequest{Id: "non-existent-instance"}
	_, err := server.WorkflowInstanceGet(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent instance")
	}
}

// ── Helper Function Tests ──────────────────────────────────────

func TestServer_workflowToProto(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	wf := workflow.NewWorkflow("proto-test-wf", []*workflow.Task{
		{
			ID:          "t1",
			Name:        "Task 1",
			Description: "First task",
			Action:      "echo",
			Params:      map[string]interface{}{"msg": "hello"},
			DependsOn:   []string{},
			Timeout:     30 * time.Second,
			RetryCount:  3,
			RetryDelay:  5 * time.Second,
		},
	})
	wf.Description = "Test workflow"
	wf.Version = "1.0"

	proto := server.workflowToProto(wf)

	if proto.Name != "proto-test-wf" {
		t.Errorf("expected name 'proto-test-wf', got %s", proto.Name)
	}

	if proto.Description != "Test workflow" {
		t.Errorf("expected description 'Test workflow', got %s", proto.Description)
	}

	if proto.Version != "1.0" {
		t.Errorf("expected version '1.0', got %s", proto.Version)
	}

	if len(proto.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(proto.Tasks))
	}

	task := proto.Tasks[0]
	if task.Name != "Task 1" {
		t.Errorf("expected task name 'Task 1', got %s", task.Name)
	}

	if task.Params["msg"] != "hello" {
		t.Errorf("expected param msg='hello', got %s", task.Params["msg"])
	}

	if task.TimeoutMs != 30000 {
		t.Errorf("expected timeout 30000ms, got %d", task.TimeoutMs)
	}

	if task.RetryCount != 3 {
		t.Errorf("expected retry count 3, got %d", task.RetryCount)
	}

	if task.RetryDelayMs != 5000 {
		t.Errorf("expected retry delay 5000ms, got %d", task.RetryDelayMs)
	}
}

func TestServer_instanceToProto(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	server := NewServer(nil, memStore, ragMgr, wfEngine)

	wf := workflow.NewWorkflow("instance-proto-wf", []*workflow.Task{
		{ID: "t1", Name: "Task 1", Action: "echo"},
	})
	wfEngine.RegisterWorkflow(wf)
	instance, _ := wfEngine.StartWorkflow(wf.ID)

	// Manually set a result for testing
	instance.Results["t1"] = &workflow.TaskResult{
		TaskID:    "t1",
		Status:    "success",
		Output:    "test output",
		Error:     "",
		StartTime: time.Now().Add(-1 * time.Minute),
		EndTime:   time.Now(),
		Duration:  time.Minute,
	}

	proto := server.instanceToProto(instance)

	if proto.Id != instance.ID {
		t.Errorf("expected instance id %s, got %s", instance.ID, proto.Id)
	}

	if proto.WorkflowId != wf.ID {
		t.Errorf("expected workflow id %s, got %s", wf.ID, proto.WorkflowId)
	}

	if _, ok := proto.Results["t1"]; !ok {
		t.Error("expected result for task t1")
	}
}

// ── GRPCServer Tests ──────────────────────────────────────

func TestGRPCServer_Addr(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	serviceServer := NewServer(nil, memStore, ragMgr, wfEngine)
	grpcServer := NewGRPCServer(":0", serviceServer)

	// Before start, addr should be the configured address
	if grpcServer.Addr() != ":0" {
		t.Errorf("expected addr ':0' before start, got %s", grpcServer.Addr())
	}

	// Start the server
	if err := grpcServer.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// After start, addr should be the actual listening address
	addr := grpcServer.Addr()
	if addr == ":0" {
		t.Error("expected actual listening address, got ':0'")
	}

	grpcServer.Stop()
}

func TestGRPCServer_Stop(t *testing.T) {
	memStore, _ := memory.NewStore(t.TempDir())
	ragMgr := newTestRAGManager()
	wfEngine := newTestWorkflowEngine()

	serviceServer := NewServer(nil, memStore, ragMgr, wfEngine)
	grpcServer := NewGRPCServer(":0", serviceServer)

	if err := grpcServer.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop should not panic
	grpcServer.Stop()

	// Give server time to stop gracefully
	time.Sleep(100 * time.Millisecond)
}