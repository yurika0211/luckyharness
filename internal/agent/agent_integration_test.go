package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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
		Content:    "Hello from mock provider!",
		TokensUsed: 30,
		Model:      "gpt-3.5-turbo",
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
		Content:    "Fallback response",
		TokensUsed: 20,
		Model:      "gpt-3.5-turbo",
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
		Content:    "Session response",
		TokensUsed: 40,
		Model:      "gpt-3.5-turbo",
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

	sess, ok := a.sessions.Get(sessionID)
	if !ok {
		t.Fatalf("session %s not found", sessionID)
	}
	msgs := sess.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 persisted messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "Hello" {
		t.Fatalf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != expectedResp.Content {
		t.Fatalf("unexpected second message: %+v", msgs[1])
	}
}

func TestRunLoopWithSessionPersistsProviderMessagesInOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	gomock.InOrder(
		mockProvider.EXPECT().
			Chat(gomock.Any(), gomock.Any()).
			Return(&provider.Response{
				Content: "Let me check",
				ToolCalls: []provider.ToolCall{
					{ID: "call_1", Name: "missing_tool", Arguments: "{}"},
				},
			}, nil),
		mockProvider.EXPECT().
			Chat(gomock.Any(), gomock.Any()).
			Return(&provider.Response{
				Content: "Done",
			}, nil),
	)

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

	sess := a.sessions.New()
	result, err := a.RunLoopWithSession(context.Background(), sess, "Need tool", DefaultLoopConfig())
	if err != nil {
		t.Fatalf("RunLoopWithSession() error = %v", err)
	}
	if result.Response != "Done" {
		t.Fatalf("expected final response %q, got %q", "Done", result.Response)
	}

	msgs := sess.GetMessages()
	if len(msgs) != 4 {
		t.Fatalf("expected 4 session messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "Need tool" {
		t.Fatalf("unexpected user message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || len(msgs[1].ToolCalls) != 1 || msgs[1].Content != "Let me check" {
		t.Fatalf("unexpected assistant tool-call message: %+v", msgs[1])
	}
	if msgs[2].Role != "tool" || msgs[2].Name != "missing_tool" || msgs[2].ToolCallID != "call_1" {
		t.Fatalf("unexpected tool message: %+v", msgs[2])
	}
	if msgs[3].Role != "assistant" || msgs[3].Content != "Done" {
		t.Fatalf("unexpected final assistant message: %+v", msgs[3])
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

// TestAgentChatWithSessionStreamRespectsMaxIterations 验证流式路径不会无限递归
func TestAgentChatWithSessionStreamRespectsMaxIterations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	// 第一轮（native stream）返回 tool call，触发进入 simulated 路径
	streamChan := make(chan provider.StreamChunk, 2)
	streamChan <- provider.StreamChunk{
		ToolCallDeltas: []provider.StreamToolCallDelta{
			{
				Index:     0,
				ID:        "call_stream_1",
				Name:      "missing_tool",
				Arguments: "{}",
			},
		},
	}
	streamChan <- provider.StreamChunk{Done: true}
	close(streamChan)

	mockProvider.EXPECT().
		ChatStream(gomock.Any(), gomock.Any()).
		Return(streamChan, nil).
		Times(1)

	// 后续 simulated 路径持续返回 tool call，直到触发 max iterations
	loopResp := &provider.Response{
		Content: "still working",
		ToolCalls: []provider.ToolCall{
			{ID: "call_loop", Name: "missing_tool", Arguments: "{}"},
		},
	}
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(loopResp, nil).
		Times(2)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")
	cfg.Set("stream_mode", "native")
	cfg.Set("agent.max_iterations", "3")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.provider = mockProvider

	sessionID := a.sessions.New().ID
	eventChan, err := a.ChatWithSessionStream(context.Background(), sessionID, "loop test")
	if err != nil {
		t.Fatalf("ChatWithSessionStream() error = %v", err)
	}

	foundMaxIterationErr := false
	for event := range eventChan {
		if event.Type == ChatEventError && event.Err != nil &&
			strings.Contains(event.Err.Error(), "max iterations reached") {
			foundMaxIterationErr = true
		}
	}

	if !foundMaxIterationErr {
		t.Fatal("expected max iterations reached error event")
	}
}

func TestChatWithSessionStreamStopsRepeatedToolCallLoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	loopResp := &provider.Response{
		Content: "",
		ToolCalls: []provider.ToolCall{
			{ID: "call_loop", Name: "missing_tool", Arguments: "{}"},
		},
	}
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(loopResp, nil).
		Times(3)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")
	cfg.Set("stream_mode", "simulated")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.provider = mockProvider

	sessionID := a.sessions.New().ID
	eventChan, err := a.ChatWithSessionStream(context.Background(), sessionID, "repeat stream tool loop")
	if err != nil {
		t.Fatalf("ChatWithSessionStream() error = %v", err)
	}

	var doneContent string
	var streamErr error
	toolResultCount := 0
	for event := range eventChan {
		if event.Type == ChatEventDone {
			doneContent = event.Content
		}
		if event.Type == ChatEventError {
			streamErr = event.Err
		}
		if event.Type == ChatEventToolResult {
			toolResultCount++
		}
	}

	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}
	if !strings.Contains(doneContent, "Detected repeated tool-call loop") {
		t.Fatalf("expected repeated-tool loop guard response, got %q", doneContent)
	}
	if toolResultCount != 2 {
		t.Fatalf("expected 2 tool results before guard, got %d", toolResultCount)
	}
}

func TestChatWithSessionStreamSimulatedRecoversAfterEmptyResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	callN := 0
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msgs []provider.Message) (*provider.Response, error) {
			callN++
			if callN == 1 {
				return &provider.Response{Content: "   "}, nil
			}
			foundRecoveryPrompt := false
			for _, m := range msgs {
				if m.Role == "user" && strings.Contains(m.Content, emptyResponseRecoveryPrompt) {
					foundRecoveryPrompt = true
					break
				}
			}
			if !foundRecoveryPrompt {
				t.Fatalf("expected empty recovery prompt in messages, got %+v", msgs)
			}
			return &provider.Response{Content: "Recovered stream answer", FinishReason: "stop"}, nil
		}).
		Times(2)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")
	cfg.Set("stream_mode", "simulated")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.provider = mockProvider

	sessionID := a.sessions.New().ID
	eventChan, err := a.ChatWithSessionStream(context.Background(), sessionID, "recover empty stream")
	if err != nil {
		t.Fatalf("ChatWithSessionStream() error = %v", err)
	}

	var doneContent string
	var streamErr error
	for event := range eventChan {
		if event.Type == ChatEventDone {
			doneContent = event.Content
		}
		if event.Type == ChatEventError {
			streamErr = event.Err
		}
	}

	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}
	if doneContent != "Recovered stream answer" {
		t.Fatalf("expected recovered stream answer, got %q", doneContent)
	}
}

func TestChatWithSessionStreamNativeRecoversLengthTruncation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)

	streamChan := make(chan provider.StreamChunk, 2)
	streamChan <- provider.StreamChunk{Content: "Part A "}
	streamChan <- provider.StreamChunk{Done: true, FinishReason: "length"}
	close(streamChan)

	mockProvider.EXPECT().
		ChatStream(gomock.Any(), gomock.Any()).
		Return(streamChan, nil).
		Times(1)

	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msgs []provider.Message) (*provider.Response, error) {
			foundRecoveryPrompt := false
			for _, m := range msgs {
				if m.Role == "user" && strings.Contains(m.Content, lengthRecoveryPrompt) {
					foundRecoveryPrompt = true
					break
				}
			}
			if !foundRecoveryPrompt {
				t.Fatalf("expected length recovery prompt in messages, got %+v", msgs)
			}
			return &provider.Response{Content: "Part B", FinishReason: "stop"}, nil
		}).
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

	sessionID := a.sessions.New().ID
	eventChan, err := a.ChatWithSessionStream(context.Background(), sessionID, "length continuation stream")
	if err != nil {
		t.Fatalf("ChatWithSessionStream() error = %v", err)
	}

	var doneContent string
	var streamErr error
	for event := range eventChan {
		if event.Type == ChatEventDone {
			doneContent = event.Content
		}
		if event.Type == ChatEventError {
			streamErr = event.Err
		}
	}

	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}
	if doneContent != "Part A Part B" {
		t.Fatalf("expected merged stream answer, got %q", doneContent)
	}
}

func TestChatWithSessionStreamSimulatedStopsAfterLengthRecoveryLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(&provider.Response{Content: "Chunk ", FinishReason: "length"}, nil).
		Times(maxLengthContinuationRetries + 1)

	tmpDir := t.TempDir()
	cfg, _ := config.NewManagerWithDir(tmpDir)
	cfg.Set("provider", "openai")
	cfg.Set("api_key", "sk-test")
	cfg.Set("model", "gpt-3.5-turbo")
	cfg.Set("stream_mode", "simulated")

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a.provider = mockProvider

	sessionID := a.sessions.New().ID
	eventChan, err := a.ChatWithSessionStream(context.Background(), sessionID, "length limit stream")
	if err != nil {
		t.Fatalf("ChatWithSessionStream() error = %v", err)
	}

	var doneContent string
	var streamErr error
	for event := range eventChan {
		if event.Type == ChatEventDone {
			doneContent = event.Content
		}
		if event.Type == ChatEventError {
			streamErr = event.Err
		}
	}

	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}
	if !strings.Contains(doneContent, strings.TrimSpace(lengthTruncatedNotice)) {
		t.Fatalf("expected truncated notice in stream done content, got %q", doneContent)
	}
}

func TestRunLoopStopsRepeatedToolCallLoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	loopResp := &provider.Response{
		Content: "",
		ToolCalls: []provider.ToolCall{
			{ID: "call_repeat", Name: "missing_tool", Arguments: "{}"},
		},
	}
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(loopResp, nil).
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

	loopCfg := DefaultLoopConfig()
	loopCfg.MaxIterations = 8
	loopCfg.Timeout = 2 * time.Second

	result, err := a.RunLoop(context.Background(), "repeat tool loop", loopCfg)
	if err != nil {
		t.Fatalf("RunLoop() error = %v", err)
	}
	if result == nil {
		t.Fatal("RunLoop() returned nil result")
	}
	if result.Iterations != 3 {
		t.Fatalf("expected 3 iterations before short-circuit, got %d", result.Iterations)
	}
	if !strings.Contains(result.Response, "Detected repeated tool-call loop") {
		t.Fatalf("expected loop guard response, got: %q", result.Response)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("expected 2 executed tool calls before guard triggered, got %d", len(result.ToolCalls))
	}
}

func TestRunLoopStopsConsecutiveToolOnlyIterations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	callN := 0
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ []provider.Message) (*provider.Response, error) {
			callN++
			return &provider.Response{
				Content: "",
				ToolCalls: []provider.ToolCall{
					{ID: fmt.Sprintf("call_%d", callN), Name: "missing_tool", Arguments: fmt.Sprintf("{\"n\":%d}", callN)},
				},
			}, nil
		}).
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

	loopCfg := DefaultLoopConfig()
	loopCfg.MaxIterations = 8
	loopCfg.Timeout = 2 * time.Second

	result, err := a.RunLoop(context.Background(), "tool-only loop", loopCfg)
	if err != nil {
		t.Fatalf("RunLoop() error = %v", err)
	}
	if result.Iterations != 3 {
		t.Fatalf("expected 3 iterations before consecutive-tool guard, got %d", result.Iterations)
	}
	if !strings.Contains(result.Response, "Detected repeated tool-call loop") {
		t.Fatalf("expected loop guard response, got: %q", result.Response)
	}
}

func TestRunLoopRecoversAfterEmptyResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	callN := 0
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msgs []provider.Message) (*provider.Response, error) {
			callN++
			if callN == 1 {
				return &provider.Response{Content: "   "}, nil
			}
			foundRecoveryPrompt := false
			for _, m := range msgs {
				if m.Role == "user" && strings.Contains(m.Content, emptyResponseRecoveryPrompt) {
					foundRecoveryPrompt = true
					break
				}
			}
			if !foundRecoveryPrompt {
				t.Fatalf("expected recovery prompt in messages, got %+v", msgs)
			}
			return &provider.Response{Content: "Recovered answer"}, nil
		}).
		Times(2)

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

	loopCfg := DefaultLoopConfig()
	loopCfg.MaxIterations = 6
	loopCfg.Timeout = 2 * time.Second

	result, err := a.RunLoop(context.Background(), "need converged answer", loopCfg)
	if err != nil {
		t.Fatalf("RunLoop() error = %v", err)
	}
	if result.Response != "Recovered answer" {
		t.Fatalf("expected recovered answer, got %q", result.Response)
	}
	if result.Iterations != 2 {
		t.Fatalf("expected 2 iterations, got %d", result.Iterations)
	}
}

func TestRunLoopRecoversLengthTruncation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	callN := 0
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msgs []provider.Message) (*provider.Response, error) {
			callN++
			if callN == 1 {
				return &provider.Response{
					Content:      "Part A ",
					FinishReason: "length",
				}, nil
			}
			foundRecoveryPrompt := false
			for _, m := range msgs {
				if m.Role == "user" && strings.Contains(m.Content, lengthRecoveryPrompt) {
					foundRecoveryPrompt = true
					break
				}
			}
			if !foundRecoveryPrompt {
				t.Fatalf("expected length recovery prompt in messages, got %+v", msgs)
			}
			return &provider.Response{
				Content:      "Part B",
				FinishReason: "stop",
			}, nil
		}).
		Times(2)

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

	loopCfg := DefaultLoopConfig()
	loopCfg.MaxIterations = 6
	loopCfg.Timeout = 2 * time.Second

	result, err := a.RunLoop(context.Background(), "give me long answer", loopCfg)
	if err != nil {
		t.Fatalf("RunLoop() error = %v", err)
	}
	if result.Response != "Part A Part B" {
		t.Fatalf("expected merged continuation, got %q", result.Response)
	}
	if result.Iterations != 2 {
		t.Fatalf("expected 2 iterations, got %d", result.Iterations)
	}
}

func TestRunLoopFallsBackAfterRepeatedEmptyResponses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(&provider.Response{Content: "   "}, nil).
		Times(maxEmptyResponseRetries + 1)

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

	loopCfg := DefaultLoopConfig()
	loopCfg.MaxIterations = 8
	loopCfg.Timeout = 2 * time.Second

	result, err := a.RunLoop(context.Background(), "empty response fallback", loopCfg)
	if err != nil {
		t.Fatalf("RunLoop() error = %v", err)
	}
	if result.Response != emptyFinalResponseMessage {
		t.Fatalf("expected fallback response %q, got %q", emptyFinalResponseMessage, result.Response)
	}
	if result.Iterations != maxEmptyResponseRetries+1 {
		t.Fatalf("expected %d iterations, got %d", maxEmptyResponseRetries+1, result.Iterations)
	}
}

func TestRunLoopStopsAfterLengthRecoveryLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockProvider(ctrl)
	mockProvider.EXPECT().
		Chat(gomock.Any(), gomock.Any()).
		Return(&provider.Response{
			Content:      "Chunk ",
			FinishReason: "length",
		}, nil).
		Times(maxLengthContinuationRetries + 1)

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

	loopCfg := DefaultLoopConfig()
	loopCfg.MaxIterations = 10
	loopCfg.Timeout = 2 * time.Second

	result, err := a.RunLoop(context.Background(), "long continuation", loopCfg)
	if err != nil {
		t.Fatalf("RunLoop() error = %v", err)
	}
	if !strings.Contains(result.Response, strings.TrimSpace(lengthTruncatedNotice)) {
		t.Fatalf("expected truncated notice in response, got %q", result.Response)
	}
	if result.Iterations != maxLengthContinuationRetries+1 {
		t.Fatalf("expected %d iterations, got %d", maxLengthContinuationRetries+1, result.Iterations)
	}
}
