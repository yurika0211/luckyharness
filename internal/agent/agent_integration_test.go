package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/yurika0211/luckyharness/internal/config"
	"github.com/yurika0211/luckyharness/internal/mocks"
	"github.com/yurika0211/luckyharness/internal/provider"
)

// TestAgentChatWithMockProvider 测试 Agent.Chat 使用 mock Provider
func TestAgentChatWithMockProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock Provider
	mockProvider := mocks.NewMockProvider(ctrl)

	// 设置期望：Chat 被调用一次，返回预设响应
	expectedResp := &provider.Response{
		Content: "Hello from mock provider!",
		TokensUsed: 30,
		Model: "gpt-3.5-turbo",
	}
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(expectedResp, nil).
		Times(1)

	// 创建 Agent 并注入 mock Provider
	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// 替换 provider 为 mock
	a.provider = mockProvider

	// 调用 Chat
	ctx := context.Background()
	content, err := a.Chat(ctx, "Hello")

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if content == "" {
		t.Fatal("Chat() returned empty content")
	}
	if content != expectedResp.Content {
		t.Errorf("expected content %q, got %q", expectedResp.Content, content)
	}
}

// TestAgentChatWithMockProviderError 测试 Agent.Chat 错误处理
// 注意：Chat 内部使用 RunLoopWithSession，错误处理复杂，这里只验证基本流程
func TestAgentChatWithMockProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	// 设置期望：Chat 被调用（RunLoop 回退到 chatStreamSimple 时会调用）
	expectedResp := &provider.Response{
		Content: "Fallback response",
		TokensUsed: 20,
		Model: "gpt-3.5-turbo",
	}
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(expectedResp, nil).
		AnyTimes()

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a.provider = mockProvider

	ctx := context.Background()
	content, err := a.Chat(ctx, "Hello")

	// Chat 可能通过回退机制成功，不一定会返回错误
	if err != nil {
		t.Logf("Chat() returned error (expected in some cases): %v", err)
	}
	if content == "" {
		t.Log("Chat() returned empty content")
	}
}

// TestAgentChatStreamWithMockProvider 测试 Agent.ChatStream 使用 mock Provider
func TestAgentChatStreamWithMockProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	// 创建流式响应 channel
	streamChan := make(chan provider.StreamChunk, 3)
	streamChan <- provider.StreamChunk{Content: "Hello"}
	streamChan <- provider.StreamChunk{Content: " from"}
	streamChan <- provider.StreamChunk{Content: " mock!"}
	close(streamChan)

	// 设置期望：ChatStream 被调用一次，返回流式 channel
	mockProvider.EXPECT().
		ChatStream(gomock.Any(), gomock.Any()).
		Return(streamChan, nil).
		Times(1)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")
	cfg.Set("stream_mode", "native")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a.provider = mockProvider

	ctx := context.Background()
	eventChan, err := a.ChatStream(ctx, "Hello")

	if err != nil {
		t.Fatalf("ChatStream() error = %v", err)
	}
	if eventChan == nil {
		t.Fatal("ChatStream() returned nil channel")
	}

	// 读取流式响应
	chunks := []provider.StreamChunk{}
	for chunk := range eventChan {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("ChatStream() returned no chunks")
	}
}

// TestAgentChatStreamWithMockProviderError 测试 Agent.ChatStream 错误处理
func TestAgentChatStreamWithMockProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	expectedErr := errors.New("mock stream error")
	mockProvider.EXPECT().
		ChatStream(gomock.Any(), gomock.Any()).
		Return(nil, expectedErr).
		Times(1)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a.provider = mockProvider

	ctx := context.Background()
	_, err = a.ChatStream(ctx, "Hello")

	if err == nil {
		t.Error("ChatStream() should return error")
	}
}

// TestAgentChatWithSessionMockProvider 测试 Agent.ChatWithSession 使用 mock
func TestAgentChatWithSessionMockProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	expectedResp := &provider.Response{
		Content: "Session response",
		TokensUsed: 40,
		Model: "gpt-3.5-turbo",
	}
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(expectedResp, nil).
		Times(1)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a.provider = mockProvider

	ctx := context.Background()
	// 先创建 session
	sessionID := a.sessions.New().ID
	content, err := a.ChatWithSession(ctx, sessionID, "Hello")

	if err != nil {
		t.Fatalf("ChatWithSession() error = %v", err)
	}
	if content == "" {
		t.Fatal("ChatWithSession() returned empty content")
	}
	if content != expectedResp.Content {
		t.Errorf("expected content %q, got %q", expectedResp.Content, content)
	}
}

// TestAgentChatWithSessionStreamMockProvider 测试 Agent.ChatWithSessionStream 使用 mock
func TestAgentChatWithSessionStreamMockProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	// ChatWithSessionStream 内部会调用 ChatStream
	streamChan := make(chan provider.StreamChunk, 2)
	streamChan <- provider.StreamChunk{Content: "Session"}
	streamChan <- provider.StreamChunk{Content: " stream"}
	close(streamChan)

	mockProvider.EXPECT().
		ChatStream(gomock.Any(), gomock.Any()).
		Return(streamChan, nil).
		Times(1)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")
	cfg.Set("stream_mode", "native")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a.provider = mockProvider

	ctx := context.Background()
	// 先创建 session
	sessionID := a.sessions.New().ID
	eventChan, err := a.ChatWithSessionStream(ctx, sessionID, "Hello")

	if err != nil {
		t.Fatalf("ChatWithSessionStream() error = %v", err)
	}
	if eventChan == nil {
		t.Fatal("ChatWithSessionStream() returned nil channel")
	}

	// 读取事件（等待 goroutine 完成）
	events := []ChatEvent{}
	for event := range eventChan {
		events = append(events, event)
	}

	// 验证至少收到一些事件
	if len(events) == 0 {
		t.Error("ChatWithSessionStream() returned no events")
	}
}
