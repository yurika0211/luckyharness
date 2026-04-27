# LuckyHarness Bot 对话问题复盘（2026-04-25）

## 1. 背景

在多轮对话、复杂任务和工具调用链路中，Bot 暴露出一批收敛与稳定性问题。  
本文记录对话中出现的实际问题、根因分析、已完成修复和待办项。

## 2. 主要问题清单

1. 工具调用循环  
典型现象：重复调用相同工具，长时间无最终回答。  
影响：响应超时、迭代浪费。  
当前状态：已修复（同步 + 流式）。

2. 空回复收敛失败  
典型现象：模型返回空内容，用户看不到有效结果。  
影响：对话“卡死感”强。  
当前状态：已修复（同步 + 流式）。

3. 长回答被截断  
典型现象：回答中途结束，信息不完整。  
影响：任务执行结果不完整。  
当前状态：已修复（同步 + 流式）。

4. 流式缺少 finish_reason  
典型现象：原生流式无法识别 `length` 截断。  
影响：无法触发续写策略。  
当前状态：已修复（provider 层）。

5. API `/stop` 占位  
典型现象：HTTP 聊天接口收到 `/stop` 仅提示未实现。  
影响：无法真正取消任务。  
当前状态：待修复。

6. Telegram 流关闭回退不一致  
典型现象：事件通道提前关闭时，结果落入超时分支。  
影响：期望“返回 partial”，实际“请求超时”。  
当前状态：待修复。

## 3. 根因分析

### 3.1 收敛状态未统一管理

早期实现里，循环终止主要依赖：

- 是否还有 tool calls
- 最大迭代次数

缺少对以下状态的主动管理：

- 空回复连续次数
- 截断续写次数
- 截断内容累积缓冲

导致“可恢复但未恢复”的失败较多。

### 3.2 流式链路信息不完整

原生流式 `StreamChunk` 未统一传递 `FinishReason`，导致上层无法识别 `length`，只能把流结束当“正常完成”或“未知中断”处理，续写策略无法触发。

### 3.3 取消语义在不同入口不一致

- Telegram Handler 已实现 per-chat 取消任务
- HTTP API 的 `/stop` 仍是提示语，不做真实 cancel

同一个 Bot 在不同入口表现不一致，增加用户困惑和排障成本。

## 4. 已完成修复（截至 2026-04-25）

### 4.1 同步对话链路（RunLoop）强化收敛

已实现：

- 空回复重试（上限）
- `finish_reason=length` 自动续写（上限）
- 续写达到上限后返回“部分结果 + 截断提示”
- 最终落盘逻辑统一，避免分支遗漏

主要文件：

- `internal/agent/loop.go`
- `internal/agent/agent_integration_test.go`

### 4.2 流式链路（native + simulated）强化收敛

已实现：

- 引入流式收敛状态对象（空回复计数、续写计数、累计内容）
- native/simulated 两条链路统一应用空回复恢复和长度续写策略
- 超限时返回可用部分内容而不是直接报错

主要文件：

- `internal/agent/agent.go`
- `internal/agent/agent_integration_test.go`

### 4.3 Provider 层补齐流式结束原因

已实现：

- `StreamChunk` 新增 `FinishReason`
- OpenAI/OpenRouter(复用 OpenAI)、Anthropic、Ollama 流式结束事件回传 finish reason

主要文件：

- `internal/provider/provider.go`
- `internal/provider/openai_stream.go`
- `internal/provider/anthropic.go`
- `internal/provider/ollama.go`

### 4.4 测试结果

已通过：

- `go test ./internal/agent -count=1`
- `go test ./internal/provider -count=1`
- `go test ./internal/server -count=1`

## 5. 待修复问题

### 5.1 API `/stop` 真取消能力

现状：HTTP 聊天接口中的 `/stop` 仅返回提示文本，未执行真实任务取消。  
建议：对齐 Telegram 入口，引入 session/task cancel token 并统一取消语义。

### 5.2 Telegram 流异常关闭回退策略

现状：`TestV054HandleChatStreamUnexpectedClose` 期望“返回 partial content”，当前结果可能落入“请求超时”。  
建议：当已收到 `ChatEventContent` 且通道提前关闭时，优先回退到最后有效内容。

## 6. 后续建议

1. 统一 `stop_reason` 观测字段（completed/tool_loop/empty_recovered/length_recovered/max_iterations/cancelled/error）。
2. 增加跨入口 E2E 回归：CLI / API / Telegram 使用同一套收敛断言。
3. 在指标中新增：
   - `empty_response_retries_total`
   - `length_recovery_attempts_total`
   - `conversation_partial_finalize_total`

## 7. 结论

本轮改造后，Bot 在“多轮工具调用 + 长文本输出 + 流式回答”场景下的收敛能力已明显提升。  
当前剩余风险主要集中在“入口取消语义一致性”和“异常流关闭的最终结果策略”，建议在下一迭代优先处理。
