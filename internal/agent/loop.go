package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yurika0211/luckyharness/internal/function"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/session"
	"github.com/yurika0211/luckyharness/internal/tool"
)

// LoopState 代表 Agent Loop 的状态
type LoopState int

const (
	StateReason LoopState = iota // 推理：分析用户意图，决定下一步
	StateAct                     // 行动：调用工具或生成回复
	StateObserve                  // 观察：处理工具结果，决定是否继续
	StateDone                     // 完成：输出最终结果
)

func (s LoopState) String() string {
	switch s {
	case StateReason:
		return "Reason"
	case StateAct:
		return "Act"
	case StateObserve:
		return "Observe"
	case StateDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// LoopConfig 是 Agent Loop 的配置
type LoopConfig struct {
	MaxIterations int           // 最大循环次数
	Timeout       time.Duration // 单次循环超时
	AutoApprove   bool          // 自动批准工具调用 (--yolo)
}

// DefaultLoopConfig 返回默认 Loop 配置
func DefaultLoopConfig() LoopConfig {
	return LoopConfig{
		MaxIterations: 10,
		Timeout:       60 * time.Second,
		AutoApprove:   false,
	}
}

// LoopResult 是 Agent Loop 的执行结果
type LoopResult struct {
	Response    string        // 最终回复
	Iterations  int           // 实际循环次数
	ToolCalls   []toolCallLog // 工具调用记录
	State       LoopState     // 结束状态
	TokensUsed  int           // 总 token 消耗
}

type toolCallLog struct {
	Name      string
	Arguments string
	Result    string
	Duration  time.Duration
}

// RunLoop 执行 Agent Loop
func (a *Agent) RunLoop(ctx context.Context, userInput string, loopCfg LoopConfig) (*LoopResult, error) {
	return a.RunLoopWithSession(ctx, nil, userInput, loopCfg)
}

// RunLoopWithSession 执行 Agent Loop（带会话上下文）
func (a *Agent) RunLoopWithSession(ctx context.Context, sess *session.Session, userInput string, loopCfg LoopConfig) (*LoopResult, error) {
	result := &LoopResult{
		State: StateReason,
	}

	// 构建初始消息
	messages := a.buildMessages(userInput)

	// 注入会话历史（多轮上下文）
	if sess != nil {
		existingMsgs := sess.GetMessages()
		if len(existingMsgs) > 0 {
			// 插入到用户消息之前
			base := messages[:len(messages)-1] // 去掉最后的 user message
			messages = append(base, existingMsgs...)
			messages = append(messages, provider.Message{Role: "user", Content: userInput})
		}
	}

	// 上下文窗口裁剪
	messages = a.fitContextWindow(messages)

	// v0.16.0: 构建 function calling 工具定义
	fcMgr := function.NewManager(a.tools)
	callOpts := provider.CallOptions{
		Tools:      fcMgr.BuildTools(),
		ToolChoice: "auto",
	}

	for i := 0; i < loopCfg.MaxIterations; i++ {
		result.Iterations = i + 1
		result.State = StateReason

		// Reason: 调用 LLM（带 function calling 支持）
		loopCtx, cancel := context.WithTimeout(ctx, loopCfg.Timeout)
		var resp *provider.Response
		var err error

		// 尝试使用 FunctionCallingProvider 接口
		if fcProvider, ok := a.provider.(provider.FunctionCallingProvider); ok && len(callOpts.Tools) > 0 {
			resp, err = fcProvider.ChatWithOptions(loopCtx, messages, callOpts)
		} else {
			resp, err = a.provider.Chat(loopCtx, messages)
		}
		cancel()

		if err != nil {
			return result, fmt.Errorf("loop iteration %d: %w", i+1, err)
		}

		result.TokensUsed += resp.TokensUsed

		// 检查是否有工具调用
		if len(resp.ToolCalls) > 0 {
			result.State = StateAct

			// 将 assistant 消息（含 tool_calls）加入历史
			messages = append(messages, provider.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			// 写入 session：assistant 的 tool_calls
			if sess != nil {
				sess.AddMessage("assistant", resp.Content)
			}

			// Act: 执行每个工具调用
			for _, tc := range resp.ToolCalls {
				start := time.Now()

				toolResult, err := a.executeTool(tc.Name, tc.Arguments, loopCfg.AutoApprove)
				duration := time.Since(start)

				tcLog := toolCallLog{
					Name:      tc.Name,
					Arguments: tc.Arguments,
					Duration:  duration,
				}

				if err != nil {
					toolResult = fmt.Sprintf("Error: %v", err)
					tcLog.Result = toolResult
				} else {
					tcLog.Result = toolResult
				}

				result.ToolCalls = append(result.ToolCalls, tcLog)

				// 将工具结果加入消息（含 tool_call_id）
				messages = append(messages, provider.Message{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: tc.ID,
					Name:       tc.Name,
				})

				// 写入 session：工具调用结果
				if sess != nil {
					sess.AddToolMessage(tc.Name, toolResult)
				}
			}

			// 每轮工具调用后裁剪上下文窗口
			messages = a.fitContextWindow(messages)

			result.State = StateObserve
			continue // 继续循环，让 LLM 处理工具结果
		}

		// 没有工具调用，LLM 直接给出最终回复
		result.Response = resp.Content
		result.State = StateDone
		return result, nil
	}

	// 达到最大循环次数
	result.Response = "Max iterations reached, last response may be incomplete"
	result.State = StateDone
	return result, fmt.Errorf("max iterations (%d) reached", loopCfg.MaxIterations)
}

// fitContextWindow 裁剪消息列表到上下文窗口内
func (a *Agent) fitContextWindow(messages []provider.Message) []provider.Message {
	contextMessages := a.toContextMessages(messages)
	fitted, trimResult := a.contextWin.Fit(contextMessages)
	if trimResult.Trimmed {
		messages = a.fromContextMessages(fitted)
	}
	return messages
}

// RunLoopStream 执行流式 Agent Loop
func (a *Agent) RunLoopStream(ctx context.Context, userInput string, loopCfg LoopConfig) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent, 128)

	go func() {
		defer close(events)

		messages := a.buildMessages(userInput)

		// v0.16.0: 构建 function calling 工具定义
		fcMgr := function.NewManager(a.tools)
		callOpts := provider.CallOptions{
			Tools:      fcMgr.BuildTools(),
			ToolChoice: "auto",
		}

		for i := 0; i < loopCfg.MaxIterations; i++ {
			events <- StreamEvent{Type: EventReason, Iteration: i + 1}

			// 流式调用（带 function calling 支持）
			var ch <-chan provider.StreamChunk
			var err error
			if fcProvider, ok := a.provider.(provider.FunctionCallingProvider); ok && len(callOpts.Tools) > 0 {
				ch, err = fcProvider.ChatStreamWithOptions(ctx, messages, callOpts)
			} else {
				ch, err = a.provider.ChatStream(ctx, messages)
			}
			if err != nil {
				events <- StreamEvent{Type: EventError, Error: err}
				return
			}

			var content strings.Builder
			for chunk := range ch {
				if chunk.Content != "" {
					content.WriteString(chunk.Content)
					events <- StreamEvent{Type: EventContent, Content: chunk.Content}
				}
				if chunk.Done {
					break
				}
			}

			events <- StreamEvent{Type: EventDone, Content: content.String()}
			return
		}
	}()

	return events, nil
}

// StreamEvent 是流式事件
type StreamEvent struct {
	Type      EventType
	Content   string
	Iteration int
	Error     error
}

type EventType int

const (
	EventReason  EventType = iota // 推理阶段
	EventAct                      // 行动阶段
	EventObserve                   // 观察阶段
	EventContent                   // 内容片段
	EventDone                      // 完成
	EventError                     // 错误
)

// executeTool 执行工具调用（通过 Gateway）
func (a *Agent) executeTool(name, arguments string, autoApprove bool) (string, error) {
	// 解析参数
	var args map[string]any
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			args = map[string]any{"raw": arguments}
		}
	}

	// 通过 Gateway 执行（统一入口：路由 → 权限 → 配额 → 计量 → 执行）
	result, err := a.gateway.Execute(name, args, "")
	if err != nil {
		return "", err
	}

	return result.Output, nil
}

// buildMessages 构建消息列表
func (a *Agent) buildMessages(userInput string) []provider.Message {
	messages := []provider.Message{
		{Role: "system", Content: a.soul.SystemPrompt()},
	}

	// 加入记忆上下文
	recent := a.memory.Recent(5)
	if len(recent) > 0 {
		var memCtx strings.Builder
		memCtx.WriteString("[Recent Memory]\n")
		for _, e := range recent {
			memCtx.WriteString("- " + e.Content + "\n")
		}
		messages = append(messages, provider.Message{Role: "system", Content: memCtx.String()})
	}

	// v0.16.0: 工具描述不再放在 system prompt 中
	// 改为通过 OpenAI function calling 的 tools 参数传递
	// 如果 provider 不支持 FunctionCallingProvider，则回退到 system prompt 方式
	if _, ok := a.provider.(provider.FunctionCallingProvider); !ok {
		tools := a.Tools().ListEnabled()
		if len(tools) > 0 {
			var toolCtx strings.Builder
			toolCtx.WriteString("[Available Tools]\n")
			for _, t := range tools {
				permLabel := "🟢"
				if t.Permission == tool.PermApprove {
					permLabel = "🟡"
				}
				toolCtx.WriteString(fmt.Sprintf("- %s %s: %s\n", permLabel, t.Name, t.Description))
			}
			messages = append(messages, provider.Message{Role: "system", Content: toolCtx.String()})
		}
	}

	// 用户消息
	messages = append(messages, provider.Message{Role: "user", Content: userInput})

	return messages
}
