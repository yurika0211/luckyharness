# LuckyHarness Development Queue

## Version Progress

| Version | Feature | Status | Notes |
|---------|---------|--------|-------|
| v0.1.0 | 基础骨架 | ✅ Done | CLI + Config + SOUL + Provider + Memory + Session + Tool |
| v0.2.0 | Agent Loop | ✅ Done | SSE streaming + tool call parsing + REPL |
| v0.3.0 | Provider 路由 | ✅ Done | 降级链 + Anthropic/Ollama/OpenRouter + Token 生命周期 |
| v0.4.0 | 持久记忆 | ✅ Done | 三层架构 + 衰减 + 摘要 + 提升 |
| v0.5.0 | 工具系统 | ✅ Done | Skill 插件 + MCP + 子代理委派 + 权限审批 |
| v0.6.0 | 消息网关 | 🔴 Blocked | 需要 Bot Token (Telegram/Discord/Slack) |
| v0.7.0 | 定时与自动化 | ✅ Done | Cron 引擎 + 自然语言解析 + Watcher |
| v0.8.0 | 沙箱与安全 | 🔴 Blocked | 需要 Docker 环境 |
| v0.9.0 | 多实例 Profile | ✅ Done | Profile 隔离 + Dashboard + Backup + Debug |
| v0.10.0 | Tool Gateway | ✅ Done | 统一网关 + 路由 + 订阅 + 计量配额 |
| v0.11.0 | Session & Stream | ✅ Done | 会话持久化 + 流式工具调用 + 配置热重载 |
| v0.12.0 | API Server | ✅ Done | HTTP RESTful + SSE + 认证限流 |
| v0.13.0 | Context Window | ✅ Done | Token 估算 + 4 种裁剪策略 + 优先级 |
| v0.14.0 | RAG 知识库 | ✅ Done | 向量索引 + 语义检索 + 持久化 + API 端点 |
| v0.15.0 | Plugin Marketplace | ✅ Done | Manifest + Registry + Installer + Sandbox + CLI + API + 测试 |
| v0.16.0 | Multi-turn RAG | ✅ Done | ConversationContext + QueryRewriter + FollowUpDetector + ContextAwareRetriever + FeedbackStore |
| v0.32.0 | Evaluation & Benchmark | ✅ Done | 评估框架 + 指标采集 + 基准测试 + 报告生成 |
| v0.33.0 | Prompt Template Engine | ✅ Done | 模板引擎 + 变量插值 + 条件/循环 + 继承 + CLI |
| v0.34.0 | Cost Tracker | ✅ Done | 成本追踪 + 预算告警 + 报表 + CLI |
| v0.35.0 | Retry & Circuit Breaker | ✅ Done | 指数退避重试 + 熔断器 + 可组合中间件 |
| v0.36.0 | Middleware System | ✅ Done | Provider 调用拦截器链 + 5 内置中间件 + MiddlewareProvider |
| v0.37.0 | Search & Fetch Rewrite | ✅ Done | 独立 search 包 + Exa 源 + 缓存 + 并发 + 配置管理 |
| v0.43.0 | Agent 包测试 + v0.43 功能整合 | ✅ Done | Agent 测试 37 新 (4.9%→23.9%) + 短期/中期记忆 + remember/recall + OneBot网关 + Telegram增强 + 文件沙箱 + Cron反馈 |
| v0.44.0 | Server 包测试补全 | ✅ Done | Server 测试 55 新 (41.0%→65.8%) — Health/Metrics/Context/FC/RAGStore/Workflow/Gateway/Plugin/Collab |
| v0.45.0 | 并发工具执行 | ✅ Done | ParallelSafe 标记 + 并发调度 + 串行保序 + 2 新测试 |
| v0.46.0 | Tool 包测试补全 | ✅ Done | 40 新测试 (64.0%→69.2%) — CallWithShellContext/UsageTracker/纯函数/边界 |
| v0.47.0 | Gateway 包测试补全 | ✅ Done | 35 新测试 — telegram 8.5%→18.0%, onebot 11.8%→28.6% |
| v0.48.0 | Provider 包测试补全 | ✅ Done | 45 新测试 (44.1%→56.1%) — Registry/FallbackChain/TokenStore/ModelCatalog/StreamParser |
| v0.49.0 | Search 包测试补全 | ✅ Done | 76 新测试 (52.5%→88.1%) — 纯函数/Cache边界/Config边界/Manager边界/URL验证/格式化/DeepSearch/Engine构造/DDGLite解析/集成测试 |

---

## ✅ Done — v0.15.0 Plugin Marketplace

### 子任务

- [x] **PM-1**: Plugin Manifest 规范 — 定义 plugin.yaml 格式（name, version, author, entry, permissions, dependencies）
- [x] **PM-2**: Plugin Registry — 插件注册中心，支持本地 + 远程仓库
- [x] **PM-3**: Plugin Installer — 下载/安装/卸载/更新插件
- [x] **PM-4**: Plugin Sandbox — 插件运行时隔离（权限控制 + 资源限制）
- [x] **PM-5**: Plugin CLI — `lh plugin install/list/update/remove/search`
- [x] **PM-6**: Plugin API — `/api/v1/plugins` 端点
- [x] **PM-7**: 测试 — 每个子模块单元测试

---

## ✅ Done — v0.16.0 Multi-turn RAG

### 子任务

- [x] **MR-1**: ConversationContext — 跟踪对话历史用于检索优化
- [x] **MR-2**: QueryRewriter — 基于上下文重写用户查询
- [x] **MR-3**: FollowUpDetector — 检测追问/澄清需求
- [x] **MR-4**: ContextAwareRetriever — 结合对话上下文的检索器
- [x] **MR-5**: RAG Feedback Loop — 检索结果反馈到对话策略
- [x] **MR-6**: 测试 — 每个子模块单元测试

---

## ✅ Done — v0.32.0 Evaluation & Benchmark

### 子任务

- [x] **EB-1**: Evaluator 接口 — 定义评估器抽象（输入/输出/期望/评分）
- [x] **EB-2**: 指标采集 — Accuracy / Relevance / Latency / TokenUsage / ToolCallAccuracy
- [x] **EB-3**: BenchmarkRunner — 批量运行评估用例，收集指标，生成报告
- [x] **EB-4**: 评估用例格式 — YAML 定义测试用例（input/expected_output/tools/context）
- [x] **EB-5**: CLI — `lh eval run/list/report`
- [x] **EB-6**: 测试 — 每个子模块单元测试

---

## ✅ Done — v0.33.0 Prompt Template Engine

### 子任务

- [x] **PT-1**: Template 定义与解析 — `{{variable}}` 插值 + `{{#if}}`/`{{#each}}` 控制 + `{{>partial}}` 引用
- [x] **PT-2**: TemplateStore — 模板存储（内存 + 文件系统），支持热加载
- [x] **PT-3**: Render 引擎 — 递归渲染 + 继承（layout/block）+ 内置函数（upper/lower/truncate/date/join/default）
- [x] **PT-4**: CLI — `lh template render/list/validate`
- [x] **PT-5**: 测试 — 每个子模块单元测试

---

## ✅ Done — v0.34.0 Cost Tracker

### 子任务

- [x] **CT-1**: CostRecord + PriceTable — 调用记录模型 + 模型定价表（10 个默认模型定价）
- [x] **CT-2**: CostStore — 成本存储（内存 + JSON 持久化），按 provider/model/session/period 聚合
- [x] **CT-3**: BudgetManager — 预算设置 + 告警阈值 + 回调通知 + provider 过滤
- [x] **CT-4**: CLI — `lh cost summary/detail/budget/set-budget`
- [x] **CT-5**: 测试 — 每个子模块单元测试

---

## ✅ Done — v0.35.0 Retry & Circuit Breaker

### 子任务

- [x] **RC-1**: Retry — 指数退避 + 抖动 + IsRetryable 分类 + 泛型 RetryWithResult[T]
- [x] **RC-2**: CircuitBreaker — 三态熔断（Closed/Open/HalfOpen）+ 自适应恢复 + 状态回调
- [x] **RC-3**: ResilientProvider — 组合 Retry + CB 的 Provider 装饰器，支持 Chat/ChatStream
- [x] **RC-4**: 测试 — 26 个单元测试（含 race detection）

---

## ✅ Done — v0.36.0 Middleware System

### 子任务

- [x] **MW-1**: Middleware 接口 + Chain — ChatHandler/StreamHandler + 反序包装 + Use/List/Len
- [x] **MW-2**: 5 内置中间件 — Logging / CostTracking / Retry / CircuitBreaker / RateLimit
- [x] **MW-3**: MiddlewareProvider — 包装任意 Provider + Chain 可访问
- [x] **MW-4**: 测试 — 18 个单元测试（含全栈集成）

---

## ✅ Done — v0.37.0 Search & Fetch Rewrite

### 子任务

- [x] **SF-1**: SearchEngine 接口 + 5 实现 — Brave / DDGS / DDG Lite / SearXNG / Exa
- [x] **SF-2**: FetchEngine 接口 + 3 实现 — Defuddle / Jina / curl+strip
- [x] **SF-3**: SearchCache + 并发搜索 — TTL 缓存 + 并发多源 + 合并去重 + 多源标注
- [x] **SF-4**: SearchConfig + 环境变量覆盖 — LH_SEARCH_* + BuildEngines/BuildFetchEngines + Manager
- [x] **SF-5**: 测试 — 35 个单元测试（含 race detection + 并发安全）

---

## ✅ Done — v0.43.0 Agent 包测试 + v0.43 功能整合

### 子任务

- [x] **AT-1**: Agent 包测试补全 — 37 新测试，覆盖率 4.9%→23.9%
  - truncate, splitIntoChunks (文本处理)
  - inferCategory, inferImportance (记忆分类)
  - sanitizeLoopConfig (Loop 配置校验)
  - toContextMessages, fromContextMessages (消息转换)
  - applyWebSearchEnv (环境变量覆盖)
  - handleMemoryTool (remember/recall 工具)
  - updateShellContext (shell cd/export/unset)
  - saveConversationMemory, autoSummarize (记忆持久化)
  - MemoryStats, DecayMemory, ExpireMidTermMemory
  - buildMessages, getStreamMode
  - LoopState/EventType 边界情况
- [x] **AT-2**: v0.43 功能整合（已在之前 commit 完成）
  - 短期记忆 ShortTermBuffer（滑动窗口 + 摘要压缩）
  - 中期记忆 MidTermStore（会话摘要 + 时间衰减检索）
  - remember/recall 工具（LLM 自主记忆）
  - OneBot (QQ) 网关适配器
  - Telegram 网关增强（typing + auto-like + group chat + chatID 持久化）
  - 文件系统沙箱
  - Cron 任务执行反馈

---

## ✅ Done — v0.45.0 并发工具执行

### 子任务

- [x] **PE-1**: Tool.ParallelSafe 标记 — 无状态工具(web_search/web_fetch/current_time/recall)标记为可并发，有状态工具(shell/remember)保持串行
- [x] **PE-2**: streamNative 并发调度 — 分类 parallel/serial 组，并发执行无状态工具，收集结果按原始顺序排列
- [x] **PE-3**: streamSimulated 并发调度 — 同 PE-2 逻辑，适配 simulated 流式模式
- [x] **PE-4**: RunLoopWithSession 并发调度 — 同 PE-2 逻辑，适配 REPL 循环模式
- [x] **PE-5**: isToolParallelSafe() — Agent 辅助方法，查询工具注册表
- [x] **PE-6**: 测试 — TestParallelToolExecution + TestParallelExecutionTiming (2 新测试)

---

## ✅ Done — v0.46.0 Tool 包测试补全

### 子任务

- [x] **TT-1**: Registry 边界测试 — 重复注册/空名称/nil Handler
- [x] **TT-2**: Permission 测试 — PermAuto/PermApprove/PermDeny 全路径
- [x] **TT-3**: UsageTracker 并发测试 — race condition + quota 并发扣减
- [x] **TT-4**: ToOpenAIFormat 完整测试 — 嵌套对象/数组参数 + required 字段
- [x] **TT-5**: 内置工具参数校验 — Shell/WebSearch/WebFetch/Remember/Recall 参数缺失/类型错误

---

## ✅ Done — v0.47.0 Gateway 包测试补全

### 子任务

- [x] **GW-1**: Telegram adapter 测试 — escapeMarkdownV2/streamSender/renderContent
- [x] **GW-2**: OneBot adapter 测试 — handleEvent/parseGroupID/splitMessage
- [x] **GW-3**: Gateway 接口测试 — Config defaults/Adapter lifecycle

---

## ✅ Done — v0.48.0 Provider 包测试补全

### 子任务

- [x] **PR-1**: Registry 测试 — Create/Get/Resolve/Close/RegisterFactory 边界
- [x] **PR-2**: Provider 构造函数 — 5 个 Provider 自定义 Config + 默认值
- [x] **PR-3**: FallbackChain 高级 — ChatWithOptions/ChatStreamWithOptions/ActiveProvider/ActiveIndex/ChainNames/ResetAllCooldowns/recordSuccess 切换/全不可用/越界/并发
- [x] **PR-4**: TokenStore 高级 — RefreshIfNeeded 5 种场景/持久化重载/短 token 脱敏/IsExpired 边界
- [x] **PR-5**: ModelCatalog 高级 — ResolveProvider o1-/o3-/Register 覆盖/排序/空结果
- [x] **PR-6**: 辅助函数 — toOpenAIMessages 边界/StreamParser 边界/结构体字段

---

## ✅ Done — v0.49.0 Search 包测试补全

### 子任务

- [x] **SR-1**: Search 包测试补全 — 76 新测试 (52.5%→88.1%)
  - 纯函数测试（URL 验证/格式化/规范化）
  - Cache 边界测试
  - Config 边界测试
  - Manager 边界测试
  - DDG Lite 解析测试
  - DeepSearch 测试
  - Engine 构造测试
  - 集成测试（DDGS/DDGLite/Jina/Curl）

---

## ✅ Done — v0.50.0 Session 包测试补全

### 子任务

- [x] **SS-1**: Session 包测试补全 — 71.9%→86.7%
  - ShellContext 测试（GetCwd/SetCwd/GetEnv/SetEnv/UnsetEnv）
  - 懒加载测试（GetMessages 懒加载逻辑）
  - Save/SaveAll 边界测试
  - NewManager 边界测试
  - Delete 边界测试
  - 并发安全测试（Session/Env 并发读写）
  - Message 序列化测试
  - Session 元数据测试（时间戳/标题）
  - Manager 搜索功能增强测试
  - 边界情况测试（特殊字符/Unicode/空查询）

---

## ✅ Done — v0.51.0 WebSocket 包测试补全

### 子任务

- [x] **WS-1**: WebSocket 包测试补全 — 覆盖率 56.7%→60.3% (+3.6pp)
  - ✅ Hub SendToClient/SendToSession 边界测试
  - ✅ Message 解析错误处理 (invalid JSON/missing type/nil data)
  - ✅ 并发广播压力测试 (10 客户端)
  - ✅ 并发连接/断开压力测试 (20 客户端)
  - ✅ Hub Stats 并发安全访问
  - ✅ 所有消息类型序列化/反序列化 (Chat/StreamChunk/StreamEnd/ToolCall/ToolResult/Status/Error/Reconnect)
  - ✅ Hub 生命周期管理
  - ✅ Client channel 并发写入
  - ⚠️ handler.go 测试 (HandleMessage/handleChat/syncChat/streamChat) 因依赖真实 agent.Agent 实例，待后续补充集成测试环境

---

## ✅ Done — v0.52.0 全仓库覆盖率里程碑

### 子任务

- [x] **CV-1**: OneBot 包测试补全 — 覆盖率 28.6%→43.9% (+15.3pp)
  - ✅ 新增 24 个测试 (onebot_v052_test.go)
  - ✅ Adapter 生命周期测试 (Start/Stop/IsRunning)
  - ✅ API 方法测试 (Send/SendWithReply/checkAPI/sendTyping/sendLike/callAPI)
  - ✅ 工具函数测试 (splitMessage/parseGroupID/truncateStr/waitRateLimit)
  - ✅ 并发安全测试 (SetHandler/Send/waitRateLimit)
  - ✅ Config 验证与边界测试
- [x] **CV-2**: Telegram 包测试补全 — 覆盖率 18.0%→22.0% (+4pp)
  - ✅ 新增 33 个测试 (telegram_v052_test.go)
  - ✅ Handler 基础功能测试 (SetDataDir/session 管理等)
  - ✅ Adapter 基础功能测试 (Send/SendWithReply/SendStream 等)
  - ✅ 工具函数测试 (truncateString/splitMessage/escapeMarkdownV2)
  - ✅ 并发安全测试 (并发发送/并发 session 操作)
- [x] **CV-3**: 全仓库覆盖率里程碑 — 59.7%→60.5% (+0.8pp) ✅ 目标达成

---

## 🔴 Blocked

### v0.6.0 消息网关
- 需要：Bot Token (Telegram/Discord/Slack)
- 用户需提供至少一个平台的 Bot Token

### v0.8.0 沙箱与安全
- 需要：Docker 环境
- 当前运行环境无 Docker

---

*Last updated: 2026-04-22 (v0.52.0 done — 全仓库覆盖率 60.5% 里程碑达成)*
