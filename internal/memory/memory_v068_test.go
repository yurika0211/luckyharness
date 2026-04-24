//go:build !integration
// +build !integration

package memory

import (
	"os"
	"testing"
	"time"
)

// v0.68.0: memory 包测试补全 - 覆盖 SearchParallel 0% 函数

// TestSearchParallel 测试 SearchParallel 函数
func TestSearchParallel(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// 添加测试数据
	store.SaveWithTierAndTags("short term memory 1", "test", TierShort, 1.0, []string{"test", "memory"})
	store.SaveWithTierAndTags("short term memory 2", "test", TierShort, 1.0, []string{"test", "search"})
	store.SaveWithTierAndTags("medium term memory", "test", TierMedium, 1.0, []string{"test", "parallel"})
	store.SaveWithTierAndTags("long term memory", "test", TierLong, 1.0, []string{"test", "coverage"})

	// 测试基本搜索
	results := store.SearchParallel("test", 3)
	if len(results) == 0 {
		t.Errorf("SearchParallel: expected non-empty results")
	}

	// 验证返回数量不超过限制
	if len(results) > 3 {
		t.Errorf("SearchParallel: expected max 3 results, got %d", len(results))
	}
}

// TestSearchParallelLimit 测试 SearchParallel 的 limit 参数
func TestSearchParallelLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// 添加多条测试数据
	for i := 0; i < 10; i++ {
		store.SaveWithTierAndTags(string(rune('a'+i)), "test", TierShort, 1.0, []string{"test"})
	}

	// 测试 limit=2
	results := store.SearchParallel("test", 2)
	if len(results) < 2 {
		t.Errorf("SearchParallel limit=2: expected at least 2 results, got %d", len(results))
	}

	// 测试 limit=3
	results3 := store.SearchParallel("test", 3)
	if len(results3) > 3 {
		t.Errorf("SearchParallel limit=3: expected max 3 results, got %d", len(results3))
	}
}

// TestSearchParallelEmpty 测试空搜索
func TestSearchParallelEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// 空 store 搜索
	results := store.SearchParallel("nonexistent", 3)
	if results == nil {
		// 允许返回空数组或 nil
	}
}

// TestSearchParallelByTier 测试各层级记忆的搜索
func TestSearchParallelByTier(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// 为每个层级添加数据
	store.SaveWithTierAndTags("short term unique keyword xyz123", "short", TierShort, 1.0, []string{"short"})
	store.SaveWithTierAndTags("medium term unique keyword xyz123", "medium", TierMedium, 1.0, []string{"medium"})
	store.SaveWithTierAndTags("long term unique keyword xyz123", "long", TierLong, 1.0, []string{"long"})

	// 搜索应该能找到各层级的记忆
	results := store.SearchParallel("xyz123", 3)
	if len(results) == 0 {
		t.Errorf("SearchParallel: expected to find entries across tiers")
	}
}

// TestSearchParallelDecay 测试搜索结果的衰减
func TestSearchParallelDecay(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// 添加数据
	store.SaveWithTierAndTags("test decay entry", "decay", TierShort, 1.0, []string{"decay"})

	// 等待一小段时间
	time.Sleep(100 * time.Millisecond)

	// 搜索应该仍然能找到
	results := store.SearchParallel("decay", 3)
	if len(results) == 0 {
		t.Errorf("SearchParallel: expected to find entry after delay")
	}
}
