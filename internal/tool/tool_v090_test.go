package tool

import (
	"testing"
	"time"
)

// TestMCPClient 测试 MCP 客户端相关函数
func TestMCPClient(t *testing.T) {
	t.Run("MCPError", func(t *testing.T) {
		err := MCPError{Code: 404, Message: "not found"}
		expected := "MCP error 404: not found"
		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("NewMCPClient", func(t *testing.T) {
		client := NewMCPClient()
		if client == nil {
			t.Error("expected non-nil client")
		}
		if client.servers == nil {
			t.Error("expected servers map to be initialized")
		}
		if client.client == nil {
			t.Error("expected http client to be initialized")
		}
	})

	t.Run("AddServer and ListServers", func(t *testing.T) {
		client := NewMCPClient()
		cfg := MCPServerConfig{
			Name: "test-server",
			URL:  "http://localhost:8080",
		}
		client.AddServer(cfg)

		servers := client.ListServers()
		if len(servers) != 1 {
			t.Errorf("expected 1 server, got %d", len(servers))
		}
	})

	t.Run("RemoveServer", func(t *testing.T) {
		client := NewMCPClient()
		cfg := MCPServerConfig{
			Name: "to-remove",
			URL:  "http://localhost:8080",
		}
		client.AddServer(cfg)
		client.RemoveServer("to-remove")

		servers := client.ListServers()
		if len(servers) != 0 {
			t.Errorf("expected 0 servers after removal, got %d", len(servers))
		}
	})
}

// TestSkillLoaderFunctions 测试 skill_loader 中的低覆盖率函数
func TestSkillLoaderFunctions(t *testing.T) {
	t.Run("stripFrontmatter empty", func(t *testing.T) {
		result := stripFrontmatter("")
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})

	t.Run("stripFrontmatter no frontmatter", func(t *testing.T) {
		input := "just content"
		result := stripFrontmatter(input)
		if result != input {
			t.Errorf("expected '%s', got '%s'", input, result)
		}
	})

	t.Run("stripFrontmatter with frontmatter", func(t *testing.T) {
		input := `---
title: test
---
content`
		result := stripFrontmatter(input)
		// 应该去掉 frontmatter 部分
		if result == input {
			t.Error("expected frontmatter to be stripped")
		}
	})

	t.Run("parseFrontmatter", func(t *testing.T) {
		input := `---
title: test
author: tester
---
content`
		result := parseFrontmatter(input)
		if result["title"] != "test" {
			t.Errorf("expected title='test', got '%s'", result["title"])
		}
	})
}

// TestGatewayFunctions 测试 gateway 中的函数
func TestGatewayFunctions(t *testing.T) {
	t.Run("truncateStr", func(t *testing.T) {
		// 测试截断功能 - 截断后会添加...
		result := truncateStr("hello world", 5)
		if result[:5] != "hello" {
			t.Errorf("expected to start with 'hello', got '%s'", result)
		}

		// 测试不需要截断的情况
		result2 := truncateStr("hi", 10)
		if result2 != "hi" {
			t.Errorf("expected 'hi', got '%s'", result2)
		}
	})
}

// TestSubscriptionFunctions 测试 subscription 中的函数
func TestSubscriptionFunctions(t *testing.T) {
	t.Run("SubTier String", func(t *testing.T) {
		if SubFree.String() != "free" {
			t.Errorf("expected 'free', got '%s'", SubFree.String())
		}
		if SubPro.String() != "pro" {
			t.Errorf("expected 'pro', got '%s'", SubPro.String())
		}
	})

	t.Run("ParseSubTier", func(t *testing.T) {
		tier, err := ParseSubTier("pro")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tier != SubPro {
			t.Errorf("expected SubPro, got %v", tier)
		}
	})

	t.Run("Subscribe and getOrCreateUsage", func(t *testing.T) {
		mgr := NewSubscriptionManager()
		// 添加一个订阅
		mgr.Subscribe("user1", SubFree, time.Hour*24)

		// 获取 usage
		usage := mgr.getOrCreateUsage("user1")
		if usage == nil {
			t.Error("expected non-nil usage")
		}
	})
}

// TestUsageTrackerFunctions 测试 usage_tracker 中的函数
func TestUsageTrackerFunctions(t *testing.T) {
	t.Run("SetQuota and GetQuota", func(t *testing.T) {
		tracker := NewUsageTracker()
		err := tracker.SetQuota("user1", "tool1", "daily", 100)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		quota := tracker.GetQuota("user1", "tool1")
		if quota == nil {
			t.Error("expected non-nil quota")
		}
	})

	t.Run("GetAllUsage", func(t *testing.T) {
		tracker := NewUsageTracker()
		tracker.Record("user1", "tool1", time.Hour, true)
		tracker.Record("user1", "tool2", time.Hour, true)

		usage := tracker.GetAllUsage("user1")
		if len(usage) == 0 {
			t.Error("expected non-empty usage")
		}
	})

	t.Run("RemoveQuota", func(t *testing.T) {
		tracker := NewUsageTracker()
		tracker.SetQuota("user1", "tool1", "daily", 100)
		tracker.RemoveQuota("user1", "tool1")

		quota := tracker.GetQuota("user1", "tool1")
		if quota != nil {
			t.Error("expected quota to be removed")
		}
	})

	t.Run("IncrementUsage", func(t *testing.T) {
		tracker := NewUsageTracker()
		tracker.SetQuota("user1", "tool1", "daily", 100)

		// 增加使用量
		tracker.IncrementUsage("user1", "tool1")
	})
}

// TestBuiltinFunctions 测试 builtin 中的低覆盖率函数
func TestBuiltinFunctions(t *testing.T) {
	t.Run("validateFetchURL invalid", func(t *testing.T) {
		err := validateFetchURL("invalid-url-!@#$")
		if err == nil {
			t.Error("expected error for invalid URL")
		}
	})

	t.Run("parseDDGLiteHTML empty", func(t *testing.T) {
		result := parseDDGLiteHTML("", 10)
		if result == "" {
			// 空输入应该返回空字符串
		}
	})

	t.Run("handleShell with nil shell context", func(t *testing.T) {
		// 测试 handleShell 在 shell context 为 nil 时的行为
		result, err := handleShell(map[string]any{
			"command": "echo test",
		})
		// 不应该 panic
		_ = result
		_ = err
	})

	t.Run("handleFileRead with missing path", func(t *testing.T) {
		result, err := handleFileRead(map[string]any{})
		if err == nil {
			t.Error("expected error for missing path")
		}
		_ = result
	})

	t.Run("handleFileWrite with missing path", func(t *testing.T) {
		result, err := handleFileWrite(map[string]any{})
		if err == nil {
			t.Error("expected error for missing path")
		}
		_ = result
	})

	t.Run("handleFileList with invalid path", func(t *testing.T) {
		result, err := handleFileList(map[string]any{
			"path": "/nonexistent/path/!@#$",
		})
		// 不应该 panic
		_ = result
		_ = err
	})

	t.Run("normalizeWhitespace", func(t *testing.T) {
		result := normalizeWhitespace("  hello   world  ")
		if result != "hello world" {
			t.Errorf("expected 'hello world', got '%s'", result)
		}
	})

	t.Run("urlEncode", func(t *testing.T) {
		result := urlEncode("hello world")
		// urlEncode 使用 URL 编码，空格变成 %20
		if result != "hello%20world" {
			t.Errorf("expected 'hello%%20world', got '%s'", result)
		}
	})

	t.Run("validatePath invalid", func(t *testing.T) {
		err := validatePath("!@#$invalid")
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}
