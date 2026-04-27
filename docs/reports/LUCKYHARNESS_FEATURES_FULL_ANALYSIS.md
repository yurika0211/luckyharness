# LuckyHarness 全量特性解析（代码实装版）

> 文档目标：基于当前仓库代码（而非 README 路线图）给出 LuckyHarness 的“已实现能力 + 接入状态 + 关键边界”。
>
> 分析基线：`cmd/lh/main.go` + `internal/cli/lhcmd/*` + `internal/server/*` + `api/grpc/*` + `internal/*` 核心模块。

---

## 1. 项目定位与总体架构

LuckyHarness 是一个以 Go 实现的 Agent 框架，主架构可分为四层：

1. **交互层**
- CLI（`lh ...` 命令）
- REPL（`lh chat` 无参数进入）
- HTTP API（`internal/server`）
- WebSocket（实时消息）
- 消息网关（Telegram / OneBot）

2. **Agent 运行层**
- Agent Loop（Reason → Act → Observe）
- Function Calling 调度
- Tool Gateway 统一执行入口
- 会话（Session）与上下文窗口管理

3. **能力层**
- Provider（OpenAI / Anthropic / Ollama / OpenRouter + fallback）
- Memory（短/中/长期）
- RAG（索引、检索、SQLite 持久化、流式增量索引）
- Skill / MCP / Delegate / Collab / Autonomy / Workflow

4. **工程与运维层**
- 配置管理与热重载
- Health / Metrics / Debug / Backup / Dashboard / Eval / Cost / Prompt Template

核心入口链路：
- `cmd/lh/main.go` -> `internal/cli/lhcmd/Execute()`
- `agent.New()` 在初始化时组装 Provider、Tools、Memory、RAG、Session、Cron、Autonomy、Gateway 等。

---

## 2. 核心能力矩阵（按“是否接入主链路”）

| 能力域 | 已实现特性 | 主入口 | 接入状态 |
|---|---|---|---|
| Agent Loop | 多轮迭代、工具调用、并发安全工具执行、空响应/截断恢复、防循环保护 | `internal/agent/loop.go` | **主链路已接入** |
| 流式对话 | native/simulated 双模式，含 tool call 事件流 | `internal/agent/agent.go` | **主链路已接入** |
| Provider | openai/openai-compatible/anthropic/ollama/openrouter + fallback chain | `internal/provider/*` | **主链路已接入** |
| Tool 系统 | registry、权限级别、路由/别名、shell context、输出压缩 | `internal/tool/tool.go`, `router.go` | **主链路已接入** |
| 内置工具 | shell/file_read/file_write/file_list/web_search/web_fetch/current_time/remember/recall | `internal/tool/builtin.go` | **主链路已接入** |
| 会话 | 多会话持久化、搜索、shell 环境记忆（cwd/env） | `internal/session/session.go` | **主链路已接入** |
| Memory | 三层记忆 + 衰减/提升/去重 + 中期摘要检索 | `internal/memory/*` | **主链路已接入** |
| Context Window | token 估算、裁剪策略（oldest/priority/sliding/summarize） | `internal/contextx/*` | **主链路已接入** |
| RAG | 文档索引、检索、MMR、对话增强、SQLite/JSON 持久化 | `internal/rag/*` | **主链路已接入** |
| Stream RAG | watch/scan/queue/process/start/stop | `internal/rag/stream.go` | **主链路已接入** |
| SOUL | 系统提示词加载、模板管理、多语言模板 | `internal/soul/*` | **主链路已接入** |
| Skill | 从目录加载 SKILL.md 与脚本、自动注册工具、`skill_read` | `internal/tool/skill_*.go`, `agent.go` | **主链路已接入** |
| MCP | MCP server 管理、工具发现与注册调用 | `internal/tool/mcp_client.go` | **主链路已接入** |
| Tool Gateway | 统一执行链：权限/配额/订阅/计量 | `internal/tool/gateway.go` | **主链路已接入** |
| Plugin 管理 | manifest/registry/sandbox/install/uninstall/update | `internal/plugin/*` | **管理链路已接入，运行时深度集成有限** |
| HTTP API | 聊天/记忆/RAG/FC/WS/SOUL/Embedder/Agent/Workflow/Gateway | `internal/server/server.go` | **主链路已接入** |
| gRPC 服务 | Chat/Memory/RAG/Workflow/Health proto+实现 | `api/proto`, `api/grpc/server.go` | **实现存在，默认启动链路未接入** |
| Collab（多 Agent） | registry + pipeline/parallel/debate + 聚合策略 | `internal/collab/*` | **API 侧可用；CLI 侧存在执行边界（见限制）** |
| Autonomy | queue/pool/heartbeat + autonomy_* tools | `internal/autonomy/*` | **库能力存在，默认未自动启动** |
| Workflow | DAG 校验、依赖执行、实例管理 | `internal/workflow/*` | **API 链路接入；默认 action handler 未注册** |
| 消息网关 | Telegram / OneBot 适配、会话映射、状态统计 | `internal/gateway/*` | **可运行（通过 `msg-gateway`）** |
| 观测与运维 | health/metrics/debug/backup/dashboard/eval/cost/template/profile | `internal/*` 对应模块 | **大部分已接入 CLI** |
| ConfigCenter / MQ / Telemetry / Middleware / Resilience / Search / Multimodal | 独立能力包 | 对应模块 | **多数为能力库，默认主启动链路未接入** |

---

## 3. CLI 全景（`lh`）

### 3.1 根命令与分组

根命令：`lh`

一级命令（当前代码注册）：
- `init`
- `chat [message]`
- `config`
- `soul`
- `models`
- `version`
- `profile`
- `backup`
- `dashboard`
- `debug`
- `gateway`
- `msg-gateway`
- `sub`
- `usage`
- `serve`
- `rag`
- `plugin`
- `metrics`
- `ws`
- `agent`
- `eval`
- `template`
- `cost`

### 3.2 关键命令族

`config`
- `get [key]`
- `set [key] [value]`
- `list`

`soul`
- `show`
- `list`（支持语言过滤）
- `switch <template-id>`

`profile`
- `list/show/create/delete/switch`
- `env set/unset`

`gateway`（工具路由）
- `info`
- `route list/add/remove`
- `alias list/add/remove`

`msg-gateway`
- `start`（telegram/onebot/all）
- `stop [platform]`
- `status`

`usage`
- `stats [user_id]`
- `quota set/list/remove`

`rag`
- `index/search/stats`
- `watch/unwatch/scan/start/stop/status/queue/process`

`plugin`
- `install/list/remove/update/search/info/enable/disable`

`agent`
- `list`
- `delegate <mode> <input> <agent_ids...>`
- `task/tasks/cancel`

`eval`
- `run/list/report`

`template`
- `render/list/validate`

`cost`
- `summary/detail/budget/set-budget`

### 3.3 REPL `/` 命令（`lh chat`）

已实现命令（来自 `handleCommand`）：
- `/quit /exit /q`
- `/help`
- `/yolo`
- `/model`
- `/models`
- `/soul`
- `/tools`
- `/remember`
- `/remember-long`
- `/recall`
- `/memstats`
- `/memdecay`
- `/promote`
- `/clear`
- `/sessions`
- `/session ...`
- `/reload`
- `/skills [dir]`
- `/mcp <name> <url> [api_key]`
- `/approve <tool>`
- `/deny <tool>`
- `/cron ...`
- `/watch ...`
- `/profile ...`
- `/dashboard start`
- `/serve [addr]`
- `/context ...`
- `/rag ...`
- `/fc ...`
- `/embedder ...`

---

## 4. HTTP API 矩阵（`internal/server.Start()` 已注册）

### 4.1 基础与会话
- `GET /api/v1/health`
- `GET /api/v1/health/live`
- `GET /api/v1/health/ready`
- `GET /api/v1/health/detail`
- `GET /api/v1/metrics`
- `GET /api/v1/stats`
- `GET /api/v1/sessions`
- `GET /api/v1/tools`
- `GET /api/v1/soul`
- `GET /`

### 4.2 聊天
- `POST /api/v1/chat`（SSE）
- `POST /api/v1/chat/sync`

### 4.3 记忆
- `GET /api/v1/memory`
- `POST /api/v1/memory`
- `GET /api/v1/memory/recall?q=...`
- `GET /api/v1/memory/stats`

### 4.4 上下文窗口
- `GET /api/v1/context`
- `POST /api/v1/context/fit`

### 4.5 RAG
- `POST/DELETE /api/v1/rag/index`
- `POST /api/v1/rag/search`
- `GET /api/v1/rag/stats`
- `GET/POST /api/v1/rag/store`

### 4.6 流式 RAG
- `POST/DELETE /api/v1/rag/stream/watch`
- `POST /api/v1/rag/stream/scan`
- `POST /api/v1/rag/stream/start`
- `POST /api/v1/rag/stream/stop`
- `GET /api/v1/rag/stream/status`
- `POST/DELETE /api/v1/rag/stream/index`
- `GET /api/v1/rag/stream/queue`
- `POST /api/v1/rag/stream/process`

### 4.7 插件
- `GET /api/v1/plugins`
- `GET /api/v1/plugins/search`
- `POST /api/v1/plugins/install`

### 4.8 Function Calling
- `GET/POST /api/v1/fc`
- `GET /api/v1/fc/tools`
- `GET /api/v1/fc/history`

### 4.9 WebSocket
- `GET /api/v1/ws`
- `GET /api/v1/ws/stats`

### 4.10 SOUL 模板
- `GET/POST /api/v1/soul/templates`
- `GET/DELETE /api/v1/soul/templates/{id}`

### 4.11 Embedders
- `GET /api/v1/embedders`
- `POST /api/v1/embedders/register`
- `POST /api/v1/embedders/switch`
- `GET /api/v1/embedders/{id}`
- `POST /api/v1/embedders/{id}/test`

### 4.12 Agent 协作
- `GET /api/v1/agents`
- `POST /api/v1/agents/register`
- `DELETE /api/v1/agents/deregister`
- `POST /api/v1/agents/delegate`
- `GET /api/v1/agents/task`
- `GET /api/v1/agents/tasks`
- `POST /api/v1/agents/cancel`

### 4.13 Workflow
- `GET/POST /api/v1/workflows`
- `GET/DELETE /api/v1/workflows/{id}`
- `GET/POST /api/v1/workflow-instances`
- `GET/DELETE /api/v1/workflow-instances/{id}`
- `GET /api/v1/workflow-instances/{id}/results`

### 4.14 消息网关
- `GET /api/v1/gateways`
- `POST /api/v1/gateways/telegram/start`
- `POST /api/v1/gateways/{name}/stop`
- `GET /api/v1/gateways/{name}/status`

---

## 5. gRPC 能力矩阵（`api/proto/luckyharness.proto`）

服务：`LuckyHarnessService`

方法：
- Chat：`Chat(ChatRequest) -> ChatResponse`
- ChatStream：`ChatStream(ChatRequest) -> stream ChatChunk`
- Memory：`MemoryStore/MemoryRecall/MemoryList/MemoryDelete`
- RAG：`RAGIndex/RAGSearch`
- Workflow：`WorkflowCreate/Get/List/Delete/Start/WorkflowInstanceGet`
- Health：`HealthCheck`

说明：
- `.proto` 带 HTTP 注解（grpc-gateway 风格映射）。
- 代码实现在 `api/grpc/server.go`，包含 health 与 reflection 注册。
- 当前默认 CLI/API 启动路径未直接启动 gRPC 监听（需额外接入）。

---

## 6. 实时通信与消息网关

### 6.1 WebSocket 协议

消息类型（双向）：
- 客户端 -> 服务端：`chat`, `stream_ack`, `ping`, `reconnect`
- 服务端 -> 客户端：`stream_chunk`, `stream_end`, `tool_call`, `tool_result`, `status`, `error`, `pong`

核心结构：`internal/websocket/message.go`，连接管理：`internal/websocket/hub.go`，Agent 适配：`internal/websocket/handler.go`。

### 6.2 Telegram 网关

实现点：
- 支持私聊/群聊触发（@提及或回复）
- 支持流式输出 sender（thinking/tool-call/result）
- 支持附件抽取（image/audio/video/document）
- 支持 chatID -> sessionID 持久化
- 支持命令：`/start /help /chat /model /soul /tools /reset /history /session /skills /cron /metrics /health /new /stop /status /restart`

### 6.3 OneBot 网关

实现点：
- HTTP API + WS 事件监听 + 可选 webhook
- 发送/回复、typing、自动点赞、分片发送、限速
- 基础命令支持（reset/model/soul/tools/skills/health 等）

---

## 7. 运维与工程能力

### 7.1 配置与热重载
- 配置结构覆盖 Provider/Server/Gateway/Memory/ModelRouter/Limits/Retry/CircuitBreaker/RateLimit/Context/Agent 等。
- 支持 `ConfigWatcher` 轮询热重载和 diff 输出。

### 7.2 Profile（多实例）
- 多 profile YAML、活跃 profile 切换、独立数据目录、环境变量注入。

### 7.3 Backup
- `tar.gz` 全目录备份、恢复、列表、元信息查看。

### 7.4 Dashboard
- `/api/status`, `/api/data`, `/api/health` + 内嵌 SPA 回退页面。

### 7.5 Debug
- 导出运行环境/配置概况/日志；敏感变量掩码。

### 7.6 Health & Metrics
- Liveness/Readiness/Detail 三类健康检查。
- 指标快照与 Prometheus 文本导出。

### 7.7 Eval
- 多 evaluator（accuracy/relevance/latency/token/tool_call_accuracy），支持批量 case、报告输出。

### 7.8 Cost
- 按 provider/model/session 聚合，预算阈值预警（warning/critical）。

### 7.9 Prompt Template
- 模板语法支持：变量、if/else、each、partial、layout、helper。

---

## 8. 实现边界与已知限制（必须关注）

1. **Embedder 实现边界**
- `internal/embedder/providers.go` 中 OpenAI/Ollama `EmbedBatch` 仍是 TODO，当前回退到 mock 向量。

2. **Delegate Enhanced 是简化实现**
- `internal/tool/delegate_enhanced.go` 的 skill/mcp 执行路径为占位式完成，不是完整真实执行链。

3. **Multimodal OpenAI Vision 为占位结果**
- `internal/multimodal/openai_provider.go` 明确注释“real implementation would call OpenAI API”，当前返回 placeholder 文本。

4. **Plugin HTTP 路由仅挂了部分端点**
- 虽实现了 `handlePluginGet/handlePluginUninstall/handlePluginToggle/handlePluginPermissions`，但 `Start()` 只注册了：
  - `/api/v1/plugins`
  - `/api/v1/plugins/search`
  - `/api/v1/plugins/install`

5. **Agent 单体查询 handler 未挂路由**
- `handleAgentsGet` 已实现，但 `Start()` 未注册对应独立路径；当前主要通过 `/api/v1/agents` 列表接口。

6. **gRPC ChatStream 为伪流式**
- `api/grpc/server.go` 中 `ChatStream` 先执行完整 `Chat`，再按固定 chunk 切分发送。

7. **CLI 的 `serve`/`dashboard start` 缺少优雅退出**
- `internal/cli/lhcmd/commands.go` 中启动后为 `select {}` 阻塞，未做 signal -> graceful stop 逻辑。

8. **Workflow 默认执行器没有注册 action handler**
- `workflow.NewDefaultExecutor()` 创建后未在主流程注册任何 `RegisterActionHandler`，任务动作会因“无 handler”失败。

9. **CLI Agent 协作路径有执行边界**
- `agent.New()` 里的 `collab.NewDelegateManager(collabReg, nil)` handler 为空，`lh agent delegate ...` 任务可能因 `no task handler configured` 失败。
- Server 侧协作使用的是 `server.New()` 内部另一个带 handler 的 delegateManager。

10. **Plugin URL 安装未完成**
- `internal/plugin/installer.go` 的 `InstallFromURL` 明确 TODO（仅本地路径安装可用）。

11. **版本号字符串存在不一致**
- CLI/Server/Debug/Dashboard/gRPC 等模块内嵌版本号不统一（例如 `v0.38.2`、`v0.21.0`、`v0.9.0`、`v0.25.0`）。

12. **多个模块是“能力库”状态，未接主启动链路**
- `internal/configcenter`, `internal/mq`, `internal/search`, `internal/telemetry`, `internal/middleware`, `internal/resilience`, `internal/multimodal`。

13. **内置命令存在未实现反馈**
- `/stop` 与 `/restart` 在 HTTP 内置命令中返回“not implemented / 手动重启”。

---

## 9. 模块导航索引（模块 -> 关键文件）

### 9.1 入口与 CLI
- `cmd/lh/main.go`
- `internal/cli/lhcmd/execute.go`
- `internal/cli/lhcmd/root_cmd.go`
- `internal/cli/lhcmd/commands.go`
- `internal/cli/lhcmd/chat_repl.go`
- `internal/cli/lhcmd/plugin_cmd.go`

### 9.2 核心 Agent 与运行时
- `internal/agent/agent.go`
- `internal/agent/loop.go`
- `internal/agent/tool_call.go`

### 9.3 Provider
- `internal/provider/provider.go`
- `internal/provider/openai_stream.go`
- `internal/provider/anthropic.go`
- `internal/provider/ollama.go`
- `internal/provider/openrouter.go`
- `internal/provider/fallback.go`
- `internal/provider/catalog.go`
- `internal/provider/function_calling.go`

### 9.4 Tool / Gateway / Skill / MCP / Delegate
- `internal/tool/tool.go`
- `internal/tool/builtin.go`
- `internal/tool/gateway.go`
- `internal/tool/router.go`
- `internal/tool/subscription.go`
- `internal/tool/usage_tracker.go`
- `internal/tool/skill_loader.go`
- `internal/tool/skill_registry.go`
- `internal/tool/skill_sandbox.go`
- `internal/tool/mcp_client.go`
- `internal/tool/delegate.go`
- `internal/tool/delegate_enhanced.go`

### 9.5 Memory / Session / Context / RAG
- `internal/memory/memory.go`
- `internal/memory/short_term.go`
- `internal/memory/mid_term.go`
- `internal/session/session.go`
- `internal/contextx/window.go`
- `internal/contextx/estimator.go`
- `internal/rag/rag.go`
- `internal/rag/indexer.go`
- `internal/rag/retriever.go`
- `internal/rag/vector.go`
- `internal/rag/sqlite_store.go`
- `internal/rag/persist.go`
- `internal/rag/stream.go`
- `internal/rag/multiturn.go`

### 9.6 API 与通信
- `internal/server/server.go`
- `internal/server/plugin_handlers.go`
- `internal/server/stream_handlers.go`
- `internal/server/soul_handlers.go`
- `internal/server/embedder_handlers.go`
- `internal/server/collab_handlers.go`
- `api/proto/luckyharness.proto`
- `api/grpc/server.go`
- `internal/websocket/message.go`
- `internal/websocket/hub.go`
- `internal/websocket/handler.go`
- `internal/gateway/manager.go`
- `internal/gateway/telegram/*`
- `internal/gateway/onebot/*`

### 9.7 扩展能力
- `internal/plugin/*`
- `internal/embedder/*`
- `internal/soul/*`
- `internal/workflow/*`
- `internal/collab/*`
- `internal/autonomy/*`
- `internal/function/manager.go`

### 9.8 运维与支撑
- `internal/config/*`
- `internal/profile/profile.go`
- `internal/backup/backup.go`
- `internal/dashboard/dashboard.go`
- `internal/debug/debug.go`
- `internal/health/health.go`
- `internal/metrics/metrics.go`
- `internal/eval/eval.go`
- `internal/cost/cost.go`
- `internal/prompt/prompt.go`
- `internal/configcenter/*`
- `internal/search/*`
- `internal/mq/*`
- `internal/telemetry/telemetry.go`
- `internal/middleware/middleware.go`
- `internal/resilience/resilience.go`
- `internal/multimodal/*`
- `internal/logger/logger.go`

---

## 10. 结论（面向当前代码快照）

LuckyHarness 在当前代码中已经具备完整的 Agent 主干（对话、工具、记忆、上下文、RAG、API、消息网关、运维命令），并且模块化程度较高。

同时，存在一批“已实现但未完全接线”或“占位实现”的能力点，主要集中在：
- gRPC 启动接入
- workflow action handler 注册
- collab CLI 执行链一致性
- plugin API 路由完整挂载
- embedder / multimodal 的真实后端实现
- 部分工程模块（telemetry/middleware/resilience/configcenter/mq/search/multimodal）与主启动路径对齐

这意味着项目当前更接近“可用主干 + 丰富扩展能力库”的形态，适合继续做接线收敛与边界补全。
