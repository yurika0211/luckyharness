package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yurika0211/luckyharness/internal/provider"
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
	result := &LoopResult{
		State: StateReason,
	}

	// 构建初始消息
	messages := a.buildMessages(userInput)

	for i := 0; i < loopCfg.MaxIterations; i++ {
		result.Iterations = i + 1
		result.State = StateReason

		// Reason: 调用 LLM
		loopCtx, cancel := context.WithTimeout(ctx, loopCfg.Timeout)
		resp, err := a.provider.Chat(loopCtx, messages)
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
				Role:    "assistant",
				Content: resp.Content,
			})

			// Act: 执行每个工具调用
			for _, tc := range resp.ToolCalls {
				start := time.Now()

				toolResult, err := a.executeTool(tc.Name, tc.Arguments, loopCfg.AutoApprove)
				duration := time.Since(start)

				log := toolCallLog{
					Name:      tc.Name,
					Arguments: tc.Arguments,
					Duration:  duration,
				}

				if err != nil {
					toolResult = fmt.Sprintf("Error: %v", err)
					log.Result = toolResult
				} else {
					log.Result = toolResult
				}

				result.ToolCalls = append(result.ToolCalls, log)

				// Observe: 将工具结果加入消息
				messages = append(messages, provider.Message{
					Role:    "tool",
					Content: toolResult,
				})
			}

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

// RunLoopStream 执行流式 Agent Loop
func (a *Agent) RunLoopStream(ctx context.Context, userInput string, loopCfg LoopConfig) (<-chan StreamEvent, error) {
	events := make(chan StreamEvent, 128)

	go func() {
		defer close(events)

		messages := a.buildMessages(userInput)

		for i := 0; i < loopCfg.MaxIterations; i++ {
			events <- StreamEvent{Type: EventReason, Iteration: i + 1}

			// 流式调用
			ch, err := a.provider.ChatStream(ctx, messages)
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

			// v0.2.0: 流式模式暂不支持工具调用检测
			// v0.3.0 将实现流式工具调用
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

// executeTool 执行工具调用
func (a *Agent) executeTool(name, arguments string, autoApprove bool) (string, error) {
	// 解析参数
	var args map[string]any
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			args = map[string]any{"raw": arguments}
		}
	}

	// 检查工具是否存在
	t, ok := a.Tools().Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	// 权限检查
	if !autoApprove {
		// v0.2.0: 简单的权限提示
		// v0.8.0 将实现完整的审批机制
		_ = t // placeholder
	}

	result, err := a.Tools().Call(name, args)
	if err != nil {
		return "", err
	}

	return result, nil
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

	// 加入工具描述
	tools := a.Tools().List()
	if len(tools) > 0 {
		var toolCtx strings.Builder
		toolCtx.WriteString("[Available Tools]\n")
		for _, t := range tools {
			toolCtx.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
		}
		messages = append(messages, provider.Message{Role: "system", Content: toolCtx.String()})
	}

	// 用户消息
	messages = append(messages, provider.Message{Role: "user", Content: userInput})

	return messages
}
