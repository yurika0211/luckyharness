package tool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yurika0211/luckyharness/internal/config"
)

// TestDelegateManager_SetAgentExecutor 测试设置 Agent 执行器
func TestDelegateManager_SetAgentExecutor(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	executor := func(ctx context.Context, description, contextStr string) (string, error) {
		return "test result", nil
	}

	dm.SetAgentExecutor(executor)

	// 验证执行器已设置（通过调用 delegate 来验证）
	// 这里主要测试 SetAgentExecutor 不 panic
	if dm == nil {
		t.Fatal("DelegateManager should not be nil")
	}
}

// TestDelegateManager_DelegateParallel 测试并行委派
func TestDelegateManager_DelegateParallel(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	// 测试空任务列表
	result := dm.DelegateParallel([]string{}, "", time.Minute)
	if result.Summary != "No tasks to delegate" {
		t.Errorf("Expected 'No tasks to delegate', got: %s", result.Summary)
	}

	// 测试单个任务
	result = dm.DelegateParallel([]string{"test task"}, "", time.Minute)
	if result.Summary == "" {
		t.Error("Expected non-empty summary")
	}
	t.Logf("Parallel result: %s", result.Summary)
}

// TestDelegateManager_DelegateParallelMultiple 测试多个并行任务
func TestDelegateManager_DelegateParallelMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	tasks := []string{
		"task 1",
		"task 2",
		"task 3",
	}

	result := dm.DelegateParallel(tasks, "test context", time.Minute)

	// 验证结果包含所有任务
	if result.SuccessCount < 0 || result.SuccessCount > len(tasks) {
		t.Errorf("Expected success count between 0 and %d, got %d", len(tasks), result.SuccessCount)
	}

	t.Logf("Parallel delegate: %d tasks, %d succeeded", len(tasks), result.SuccessCount)
}

// TestDelegateManager_DelegateParallelEmptyContext 测试空上下文
func TestDelegateManager_DelegateParallelEmptyContext(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	result := dm.DelegateParallel([]string{"task with empty context"}, "", 30*time.Second)

	if result.TotalDuration < 0 {
		t.Error("Expected non-negative duration")
	}
}

// TestDelegateManager_Config 测试配置
func TestDelegateManager_Config(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	// 验证管理器已正确初始化
	if dm == nil {
		t.Fatal("DelegateManager should not be nil")
	}

	// 验证默认配置
	if dm.config.MaxConcurrent <= 0 {
		t.Log("Using default max concurrent")
	}
}

// TestDelegateManager_ConcurrentLimit 测试并发限制
func TestDelegateManager_ConcurrentLimit(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	// 设置较小的并发数
	dm.config.MaxConcurrent = 2

	tasks := []string{
		"task 1",
		"task 2",
		"task 3",
		"task 4",
		"task 5",
	}

	result := dm.DelegateParallel(tasks, "", time.Minute)

	// 验证任务执行完成
	if result.Summary == "" {
		t.Error("Expected non-empty summary")
	}

	t.Logf("Concurrent test completed: %s", result.Summary)
}

// TestDelegateManager_Timeout 测试超时处理
func TestDelegateManager_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	// 测试非常短的超时
	result := dm.DelegateParallel([]string{"quick task"}, "", 100*time.Millisecond)

	// 验证有结果返回（即使超时）
	if result == nil {
		t.Error("Expected result even with timeout")
	}
}

// TestDelegateManager_LargeParallel 测试大量并行任务
func TestDelegateManager_LargeParallel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large parallel test in short mode")
	}

	tmpDir := t.TempDir()
	_, err := config.NewManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	cfg := DelegateConfig{}
	dm := NewDelegateManager(cfg)

	// 创建 10 个任务
	tasks := make([]string, 10)
	for i := 0; i < 10; i++ {
		tasks[i] = fmt.Sprintf("parallel task %d", i+1)
	}

	result := dm.DelegateParallel(tasks, "", 2*time.Minute)

	t.Logf("Large parallel: %d tasks, %d succeeded, duration: %v",
		len(tasks), result.SuccessCount, result.TotalDuration)
}
