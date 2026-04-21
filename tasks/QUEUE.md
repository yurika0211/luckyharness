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

## 🔴 Blocked

### v0.6.0 消息网关
- 需要: Bot Token (Telegram/Discord/Slack)
- 用户需提供至少一个平台的 Bot Token

### v0.8.0 沙箱与安全
- 需要: Docker 环境
- 当前运行环境无 Docker

---

*Last updated: 2026-04-21*