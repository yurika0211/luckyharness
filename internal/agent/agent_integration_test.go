package agent

import (
	"context"
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
		Usage: &provider.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
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
	resp, err := a.Chat(ctx, "Hello")

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp == nil {
		t.Fatal("Chat() returned nil response")
	}
	if resp.Content != expectedResp.Content {
		t.Errorf("expected content %q, got %q", expectedResp.Content, resp.Content)
	}
}

// TestAgentChatWithMockProviderError 测试 Agent.Chat 错误处理
func TestAgentChatWithMockProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	// 设置期望：Chat 返回错误
	expectedErr := provider.ErrEmptyContent
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
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
	_, err = a.Chat(ctx, "Hello")

	if err == nil {
		t.Error("Chat() should return error")
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

	// 读取事件
	events := []ChatEvent{}
	for event := range eventChan {
		events = append(events, event)
	}

	if len(events) == 0 {
		t.Error("ChatStream() returned no events")
	}
}

// TestAgentChatStreamWithMockProviderError 测试 Agent.ChatStream 错误处理
func TestAgentChatStreamWithMockProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	expectedErr := provider.ErrEmptyContent
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
		Usage: &provider.TokenUsage{
			PromptTokens:     15,
			CompletionTokens: 25,
			TotalTokens:      40,
		},
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
	resp, err := a.ChatWithSession(ctx, "test-session", "Hello")

	if err != nil {
		t.Fatalf("ChatWithSession() error = %v", err)
	}
	if resp == nil {
		t.Fatal("ChatWithSession() returned nil response")
	}
	if resp.Content != expectedResp.Content {
		t.Errorf("expected content %q, got %q", expectedResp.Content, resp.Content)
	}
}

// TestAgentChatWithSessionStreamMockProvider 测试 Agent.ChatWithSessionStream 使用 mock
func TestAgentChatWithSessionStreamMockProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

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
	eventChan, err := a.ChatWithSessionStream(ctx, "test-session", "Hello")

	if err != nil {
		t.Fatalf("ChatWithSessionStream() error = %v", err)
	}
	if eventChan == nil {
		t.Fatal("ChatWithSessionStream() returned nil channel")
	}

	// 读取事件
	events := []ChatEvent{}
	for event := range eventChan {
		events = append(events, event)
	}

	if len(events) == 0 {
		t.Error("ChatWithSessionStream() returned no events")
	}
}
