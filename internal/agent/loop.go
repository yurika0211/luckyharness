package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yurika0211/luckyharness/internal/function"
	"github.com/yurika0211/luckyharness/internal/provider"
	"github.com/yurika0211/luckyharness/internal/session"
	"github.com/yurika0211/luckyharness/internal/tool"
)

// shell 上下文解析正则
var (
	cdPattern     = regexp.MustCompile(`(?i)(?:^|;|\|&&|\s)cd\s+(.+?)(?:\s*;|\s*&&|\s*\|\||\s*$)`)
	exportPattern = regexp.MustCompile(`(?i)(?:^|;|\|&&|\s)export\s+([A-Za-z_][A-Za-z0-9_]*)=(.+?)(?:\s*;|\s*&&|\s*\|\||\s*$)`)
	unsetPattern  = regexp.MustCompile(`(?i)(?:^|;|\|&&|\s)unset\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s*;|\s*&&|\s*\|\||\s*$)`)
)

// LoopState 代表 Agent Loop 的状态
type LoopState int

const (
	StateReason  LoopState = iota // 推理：分析用户意图，决定下一步
	StateAct                      // 行动：调用工具或生成回复
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

// maxAllowedIterations 是 MaxIterations 的硬上限
const maxAllowedIterations = 100

const (
	maxEmptyResponseRetries      = 2
	maxLengthContinuationRetries = 3
	emptyResponseRecoveryPrompt  = "Your last response was empty. Please provide a direct, complete answer to my previous request. Avoid tool calls unless required."
	lengthRecoveryPrompt         = "Continue exactly from where you stopped. Do not repeat previous content."
	emptyFinalResponseMessage    = "I couldn't produce a complete answer this round. Please retry."
	lengthTruncatedNotice        = "\n\n[Output may be truncated after multiple continuation attempts.]"
)

// sanitizeLoopConfig 校验并修正 LoopConfig 的安全边界
func sanitizeLoopConfig(cfg *LoopConfig) {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}
	if cfg.MaxIterations > maxAllowedIterations {
		cfg.MaxIterations = maxAllowedIterations
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.Timeout > 10*time.Minute {
		cfg.Timeout = 10 * time.Minute
	}
}

func appendContinuation(dst *strings.Builder, part string) {
	if strings.TrimSpace(part) == "" {
		return
	}
	dst.WriteString(part)
}

// LoopResult 是 Agent Loop 的执行结果
type LoopResult struct {
	Response   string        // 最终回复
	Iterations int           // 实际循环次数
	ToolCalls  []toolCallLog // 工具调用记录
	State      LoopState     // 结束状态
	TokensUsed int           // 总 token 消耗
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
	// 安全边界校验
	sanitizeLoopConfig(&loopCfg)

	result := &LoopResult{
		State: StateReason,
	}
	finalize := func(response string) {
		result.Response = response
		result.State = StateDone

		// v0.24.1: 将对话添加到会话
		if sess != nil {
			sess.AddMessage("user", userInput)
			sess.AddMessage("assistant", response)
		}

		// v0.35.0: 将本轮对话索引进 RAG（异步，不阻塞返回）
		if a.ragManager != nil {
			a.indexConversationTurn(userInput, response)
		}

		// v0.24.1: 保存会话到磁盘
		if sess != nil {
			if saveErr := sess.Save(); saveErr != nil {
				fmt.Printf("[agent] warning: failed to save session: %v\n", saveErr)
			}
		}
	}
	toolCallRepeatCount := make(map[string]int)
	toolCallLastResult := make(map[string]string)
	consecutiveToolOnlyIters := 0
	emptyResponseRetries := 0
	lengthRecoveryCount := 0
	var continuedResponse strings.Builder
	toolCallSig := func(name, arguments string) string {
		return name + "|" + arguments
	}

	// 构建初始消息
	messages := a.buildMessages(userInput)

	// 注入会话历史（多轮上下文，滑动窗口）
	if sess != nil {
		existingMsgs := sess.GetMessages(20) // v0.44.0: 只取最近 20 轮
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
	// 若用户显式在输入中点名工具（如“必须调用 file_read”），优先只暴露这些工具，提升可控性。
	if required := a.extractRequiredToolNames(userInput); len(required) > 0 {
		filtered := make([]map[string]any, 0, len(required))
		for _, name := range required {
			if t, ok := a.tools.Get(name); ok && t.Enabled {
				filtered = append(filtered, t.ToOpenAIFormat())
			}
		}
		if len(filtered) > 0 {
			callOpts.Tools = filtered
		}
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
			emptyResponseRetries = 0
			lengthRecoveryCount = 0
			result.State = StateAct
			if strings.TrimSpace(resp.Content) == "" {
				consecutiveToolOnlyIters++
			} else {
				consecutiveToolOnlyIters = 0
			}

			// 防止模型陷入重复工具调用死循环（尤其是 skill_read 反复触发）。
			repeatedSigs := make([]string, 0, len(resp.ToolCalls))
			allRepeated := true
			for _, tc := range resp.ToolCalls {
				sig := toolCallSig(tc.Name, tc.Arguments)
				repeatedSigs = append(repeatedSigs, sig)
				toolCallRepeatCount[sig]++
				if toolCallRepeatCount[sig] < 3 {
					allRepeated = false
				}
			}
			if (allRepeated && strings.TrimSpace(resp.Content) == "") || consecutiveToolOnlyIters >= 3 {
				var b strings.Builder
				b.WriteString("Detected repeated tool-call loop and stopped early to avoid timeout.\n")
				b.WriteString("Latest tool outputs:\n")
				for _, sig := range repeatedSigs {
					parts := strings.SplitN(sig, "|", 2)
					name := parts[0]
					out := strings.TrimSpace(toolCallLastResult[sig])
					if out == "" {
						out = "(no cached output)"
					}
					if len(out) > 240 {
						out = out[:240] + "...(truncated)"
					}
					b.WriteString(fmt.Sprintf("- %s: %s\n", name, out))
				}
				finalize(strings.TrimSpace(b.String()))
				return result, nil
			}

			// 将 assistant 消息（含 tool_calls）加入历史
			messages = append(messages, provider.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			// 写入 session：assistant 的 tool_calls
			if sess != nil {
				sess.AddProviderMessage(provider.Message{
					Role:      "assistant",
					Content:   resp.Content,
					ToolCalls: resp.ToolCalls,
				})
			}

			// Act: 执行工具调用（并发优化：无依赖的工具并行执行）
			type toolExecResult struct {
				Index       int
				ToolCall    toolCallLog
				ToolMessage provider.Message
			}

			// 分析依赖：shell/file_write/remember 等有状态工具必须串行
			// 无状态工具（web_search, web_fetch, file_read, current_time, recall）可并发
			var parallelGroup []int // 可并发的工具索引
			var serialGroup []int   // 必须串行的工具索引

			for i, tc := range resp.ToolCalls {
				if a.isToolParallelSafe(tc.Name) {
					parallelGroup = append(parallelGroup, i)
				} else {
					serialGroup = append(serialGroup, i)
				}
			}

			resultCh := make(chan toolExecResult, len(resp.ToolCalls))

			// 并发执行无状态工具
			if len(parallelGroup) > 0 {
				for _, idx := range parallelGroup {
					tc := resp.ToolCalls[idx]
					go func(idx int, tc provider.ToolCall) {
						start := time.Now()
						toolResult, err := a.executeToolWithSession(tc.Name, tc.Arguments, loopCfg.AutoApprove, sess)
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

						resultCh <- toolExecResult{
							Index:    idx,
							ToolCall: tcLog,
							ToolMessage: provider.Message{
								Role:       "tool",
								Content:    toolResult,
								ToolCallID: tc.ID,
								Name:       tc.Name,
							},
						}
					}(idx, tc)
				}
			}

			// 串行执行有状态工具
			for _, idx := range serialGroup {
				tc := resp.ToolCalls[idx]
				start := time.Now()
				toolResult, err := a.executeToolWithSession(tc.Name, tc.Arguments, loopCfg.AutoApprove, sess)
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

				resultCh <- toolExecResult{
					Index:    idx,
					ToolCall: tcLog,
					ToolMessage: provider.Message{
						Role:       "tool",
						Content:    toolResult,
						ToolCallID: tc.ID,
						Name:       tc.Name,
					},
				}
			}

			// 收集所有结果，按原始顺序排列
			allResults := make([]toolExecResult, 0, len(resp.ToolCalls))
			for i := 0; i < len(resp.ToolCalls); i++ {
				allResults = append(allResults, <-resultCh)
			}
			// 按 Index 排序，保证消息顺序与 tool_calls 一致
			sort.Slice(allResults, func(i, j int) bool {
				return allResults[i].Index < allResults[j].Index
			})

			for _, r := range allResults {
				result.ToolCalls = append(result.ToolCalls, r.ToolCall)
				messages = append(messages, r.ToolMessage)
				toolCallLastResult[toolCallSig(r.ToolCall.Name, r.ToolCall.Arguments)] = r.ToolCall.Result
				if sess != nil {
					sess.AddProviderMessage(r.ToolMessage)
				}
			}

			// 每轮工具调用后裁剪上下文窗口
			messages = a.fitContextWindow(messages)

			result.State = StateObserve

			// v0.24.1: 工具调用后保存会话
			if sess != nil {
				if saveErr := sess.Save(); saveErr != nil {
					fmt.Printf("[agent] warning: failed to save session: %v\n", saveErr)
				}
			}

			continue // 继续循环，让 LLM 处理工具结果
		}

		raw := resp.Content
		clean := strings.TrimSpace(raw)

		// 空回复恢复：给模型一次显式重试机会，避免直接返回空答案。
		if clean == "" {
			if emptyResponseRetries < maxEmptyResponseRetries {
				emptyResponseRetries++
				messages = append(messages, provider.Message{Role: "assistant", Content: raw})
				messages = append(messages, provider.Message{Role: "user", Content: emptyResponseRecoveryPrompt})
				continue
			}
			if strings.TrimSpace(continuedResponse.String()) != "" {
				finalize(strings.TrimSpace(continuedResponse.String()))
			} else {
				finalize(emptyFinalResponseMessage)
			}
			return result, nil
		}
		emptyResponseRetries = 0

		// 截断恢复：当模型因长度截断时，拼接已有内容并显式请求续写。
		if strings.EqualFold(resp.FinishReason, "length") {
			appendContinuation(&continuedResponse, raw)
			if lengthRecoveryCount < maxLengthContinuationRetries {
				lengthRecoveryCount++
				messages = append(messages, provider.Message{Role: "assistant", Content: raw})
				messages = append(messages, provider.Message{Role: "user", Content: lengthRecoveryPrompt})
				continue
			}
			partial := strings.TrimSpace(continuedResponse.String())
			if partial == "" {
				partial = clean
			}
			finalize(partial + lengthTruncatedNotice)
			return result, nil
		}
		lengthRecoveryCount = 0

		// 没有工具调用，LLM 直接给出最终回复
		finalResponse := raw
		if strings.TrimSpace(continuedResponse.String()) != "" {
			appendContinuation(&continuedResponse, raw)
			finalResponse = strings.TrimSpace(continuedResponse.String())
		}
		finalize(finalResponse)
		return result, nil
	}

	if strings.TrimSpace(continuedResponse.String()) != "" {
		finalize(strings.TrimSpace(continuedResponse.String()) + lengthTruncatedNotice)
		return result, nil
	}

	// 达到最大循环次数
	result.Response = "Max iterations reached, last response may be incomplete"
	result.State = StateDone

	// v0.24.1: 保存会话到磁盘
	if sess != nil {
		if saveErr := sess.Save(); saveErr != nil {
			fmt.Printf("[agent] warning: failed to save session: %v\n", saveErr)
		}
	}

	return result, fmt.Errorf("max iterations (%d) reached", loopCfg.MaxIterations)
}

// extractRequiredToolNames 从用户输入中提取显式点名的工具（按出现顺序）。
func (a *Agent) extractRequiredToolNames(input string) []string {
	type hit struct {
		name string
		pos  int
	}

	var hits []hit
	for _, t := range a.tools.ListEnabled() {
		if idx := strings.Index(input, t.Name); idx >= 0 {
			hits = append(hits, hit{name: t.Name, pos: idx})
		}
	}
	if len(hits) == 0 {
		return nil
	}

	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].pos < hits[j].pos
	})

	seen := make(map[string]struct{}, len(hits))
	result := make([]string, 0, len(hits))
	for _, h := range hits {
		if _, ok := seen[h.name]; ok {
			continue
		}
		seen[h.name] = struct{}{}
		result = append(result, h.name)
	}
	return result
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

// indexConversationTurn 将一轮对话索引进 RAG（异步执行）
func (a *Agent) indexConversationTurn(userInput, assistantResponse string) {
	go func() {
		// 组合对话内容作为索引文本
		content := "User: " + userInput + "\nAssistant: " + assistantResponse
		title := userInput
		if len(title) > 80 {
			title = title[:80] + "..."
		}
		source := "conversation"

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if _, err := a.ragManager.IndexText(source, title, content); err != nil {
			// 索引失败不影响主流程，静默忽略
			_ = ctx.Err()
		}
	}()
}

// RunLoopStream 执行流式 Agent Loop
func (a *Agent) RunLoopStream(ctx context.Context, userInput string, loopCfg LoopConfig) (<-chan StreamEvent, error) {
	// 安全边界校验
	sanitizeLoopConfig(&loopCfg)

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
	EventObserve                  // 观察阶段
	EventContent                  // 内容片段
	EventDone                     // 完成
	EventError                    // 错误
)

// executeTool 执行工具调用（通过 Gateway）
func (a *Agent) executeTool(name, arguments string, autoApprove bool) (string, error) {
	return a.executeToolWithSession(name, arguments, autoApprove, nil)
}

// executeToolWithSession 执行工具调用（带 session，支持 shell 上下文持久化）
func (a *Agent) executeToolWithSession(name, arguments string, autoApprove bool, sess *session.Session) (string, error) {
	// 记忆工具：直接由 agent 处理，不走 gateway
	if name == "remember" || name == "recall" {
		return a.handleMemoryTool(name, arguments)
	}

	// 解析参数
	var args map[string]any
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			args = map[string]any{"raw": arguments}
		}
	}

	// 构建 shell 上下文
	var sc *tool.ShellContext
	if sess != nil {
		cwd := sess.GetCwd()
		env := sess.GetEnv()
		if cwd != "" || len(env) > 0 {
			sc = &tool.ShellContext{
				Cwd: cwd,
				Env: env,
			}
		}
	}

	// 通过 Gateway 执行
	var result *tool.GatewayResult
	var err error
	if sc != nil {
		result, err = a.gateway.ExecuteWithShellContext(name, args, "", sc)
	} else {
		result, err = a.gateway.Execute(name, args, "")
	}
	if err != nil {
		return "", err
	}

	// shell 执行后更新 session 的 cwd/env
	if sess != nil && name == "shell" {
		a.updateShellContext(sess, arguments, result.Output)
	}

	return result.Output, nil
}

// updateShellContext 从 shell 执行结果中提取 cwd 和 env 变更
func (a *Agent) updateShellContext(sess *session.Session, command, output string) {
	// 检测 cd 命令：提取目标目录并验证
	cdMatch := cdPattern.FindStringSubmatch(command)
	if len(cdMatch) >= 2 {
		target := strings.TrimSpace(cdMatch[1])
		if target != "" {
			// 验证目录是否存在
			if abs, err := filepath.Abs(target); err == nil {
				if info, err := os.Stat(abs); err == nil && info.IsDir() {
					sess.SetCwd(abs)
				}
			}
		}
	}

	// 检测 export 命令
	exportMatches := exportPattern.FindAllStringSubmatch(command, -1)
	for _, m := range exportMatches {
		if len(m) >= 3 {
			key := strings.TrimSpace(m[1])
			val := strings.TrimSpace(m[2])
			if key != "" {
				sess.SetEnv(key, val)
			}
		}
	}

	// 检测 unset 命令
	unsetMatches := unsetPattern.FindAllStringSubmatch(command, -1)
	for _, m := range unsetMatches {
		if len(m) >= 2 {
			key := strings.TrimSpace(m[1])
			if key != "" {
				sess.UnsetEnv(key)
			}
		}
	}
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

	// v0.35.0: RAG 检索相关上下文
	if a.ragManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ragContext, results, err := a.ragManager.SearchWithContext(ctx, userInput)
		cancel()
		if err == nil && ragContext != "" && len(results) > 0 {
			messages = append(messages, provider.Message{Role: "system", Content: ragContext})
		}
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

	// v0.56.4: 移除全量 skill 列表从 system prompt
	// 原因：96 个 skill 摘要占 60KB，导致响应时间 15-65 秒
	// 改为：通过 RAG 检索相关 skill，或用户显式调用 skill_read
	// if len(a.skills) > 0 {
	// 	var skillCtx strings.Builder
	// 	skillCtx.WriteString("[Available Skills — use skill_read(name) to get full SKILL.md]\n")
	// 	for _, s := range a.skills {
	// 		if s.Summary != "" {
	// 			skillCtx.WriteString(fmt.Sprintf("- %s: %s | %s\n", s.Name, s.Description, s.Summary))
	// 		} else {
	// 			skillCtx.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
	// 		}
	// 	}
	// 	messages = append(messages, provider.Message{Role: "system", Content: skillCtx.String()})
	// }

	// 用户消息
	messages = append(messages, provider.Message{Role: "user", Content: userInput})

	// v0.56.4 DEBUG
	totalLen := 0
	for _, m := range messages {
		totalLen += len(m.Content)
	}
	_ = totalLen // 避免未使用变量警告

	return messages
}

// isToolParallelSafe 检查工具是否可安全并发执行
func (a *Agent) isToolParallelSafe(toolName string) bool {
	t, ok := a.tools.Get(toolName)
	if !ok {
		return false // 未知工具保守处理
	}
	return t.ParallelSafe
}

// ParallelSummarizeThreshold 触发并行摘要的对话条数阈值
const ParallelSummarizeThreshold = 20

// ParallelSummarize 并行摘要对话历史
// 当对话超过阈值时，将对话分成两半，用 goroutine 并行调用 LLM 摘要
// 合并结果替换原对话
func (a *Agent) ParallelSummarize(messages []provider.Message) ([]provider.Message, error) {
	if len(messages) <= ParallelSummarizeThreshold {
		return messages, nil // 未达阈值，不处理
	}

	// 保留 system 消息
	var systemMsgs []provider.Message
	var conversationMsgs []provider.Message
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			conversationMsgs = append(conversationMsgs, msg)
		}
	}

	if len(conversationMsgs) <= ParallelSummarizeThreshold {
		return messages, nil
	}

	// 将对话分成两半
	mid := len(conversationMsgs) / 2
	firstHalf := conversationMsgs[:mid]
	secondHalf := conversationMsgs[mid:]

	// 使用 channel 收集摘要结果
	type summarizeResult struct {
		summary string
		err     error
	}
	resultCh := make(chan summarizeResult, 2)

	// 定义摘要 prompt
	summarizePrompt := func(msgs []provider.Message, part string) string {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Please summarize the following conversation %s concisely in 2-3 sentences. ", part))
		sb.WriteString("Preserve key information, decisions, and action items. ")
		sb.WriteString("Output only the summary, no additional commentary.\n\n")
		sb.WriteString("Conversation:\n")
		for _, m := range msgs {
			role := m.Role
			if role == "assistant" {
				role = "Assistant"
			} else {
				role = "User"
			}
			sb.WriteString(fmt.Sprintf("%s: %s\n", role, m.Content))
		}
		return sb.String()
	}

	// 并发摘要两半对话
	ctx := context.Background()

	// 第一部分
	go func() {
		prompt := summarizePrompt(firstHalf, "(first part)")
		messages := []provider.Message{
			{Role: "system", Content: "You are a helpful assistant that summarizes conversations."},
			{Role: "user", Content: prompt},
		}

		resp, err := a.provider.Chat(ctx, messages)
		if err != nil {
			resultCh <- summarizeResult{summary: "", err: err}
			return
		}
		resultCh <- summarizeResult{summary: resp.Content, err: nil}
	}()

	// 第二部分
	go func() {
		prompt := summarizePrompt(secondHalf, "(second part)")
		messages := []provider.Message{
			{Role: "system", Content: "You are a helpful assistant that summarizes conversations."},
			{Role: "user", Content: prompt},
		}

		resp, err := a.provider.Chat(ctx, messages)
		if err != nil {
			resultCh <- summarizeResult{summary: "", err: err}
			return
		}
		resultCh <- summarizeResult{summary: resp.Content, err: nil}
	}()

	// 收集两个摘要结果
	var firstSummary, secondSummary string
	for i := 0; i < 2; i++ {
		result := <-resultCh
		if result.err != nil {
			// 如果摘要失败，返回原始消息
			return messages, result.err
		}
		if firstSummary == "" {
			firstSummary = result.summary
		} else {
			secondSummary = result.summary
		}
	}

	// 合并摘要
	var summaryContent strings.Builder
	summaryContent.WriteString("[Conversation Summary - Parallel Summarization]\n")
	summaryContent.WriteString(fmt.Sprintf("First Part Summary: %s\n", firstSummary))
	summaryContent.WriteString(fmt.Sprintf("Second Part Summary: %s\n", secondSummary))
	summaryContent.WriteString("\n[End Summary]\n")

	// 构建新的消息列表：system + 摘要 + 最近少量原始消息
	newMessages := make([]provider.Message, 0, len(systemMsgs)+1+5)

	// 添加 system 消息
	newMessages = append(newMessages, systemMsgs...)

	// 添加摘要消息
	newMessages = append(newMessages, provider.Message{
		Role:    "system",
		Content: summaryContent.String(),
	})

	// 保留最后 5 条原始对话作为上下文
	keepCount := 5
	if keepCount > len(conversationMsgs) {
		keepCount = len(conversationMsgs)
	}
	startIdx := len(conversationMsgs) - keepCount
	newMessages = append(newMessages, conversationMsgs[startIdx:]...)

	return newMessages, nil
}
