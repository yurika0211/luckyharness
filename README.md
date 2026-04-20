# LuckyHarness 🍀

> Go 版自主 AI Agent 框架 — 仿 [Hermes Agent](https://github.com/NousResearch/hermes-agent) 架构，迭代式开发。

## 项目定位

LuckyHarness 是一个用 Go 重写的 AI Agent 框架，参考 Hermes Agent 的核心架构设计，逐步实现其关键特性：

- 🧠 **SOUL 系统** — 可定制的 Agent 人格与行为指令
- 🔌 **Provider 路由** — 多 LLM 提供商自动解析与切换
- 💾 **持久记忆** — 跨会话记忆与自动学习
- 🛠️ **工具系统** — 可扩展的 Skill/Tool 插件架构
- 🔄 **Agent Loop** — 自主推理-行动循环
- 📱 **多平台网关** — Telegram/Discord/Slack/微信等消息平台接入
- ⏰ **定时任务** — 自然语言 cron 调度

## 版本路线

| 版本 | 主题 | 核心特性 |
|------|------|----------|
| v0.1.0 | 基础骨架 | CLI 入口 + 配置系统 + SOUL + 单轮对话 |
| v0.2.0 | Agent Loop | 多轮推理-行动循环 + 工具调用 |
| v0.3.0 | Provider 路由 | 多提供商解析 + 自动降级 + OAuth |
| v0.4.0 | 持久记忆 | 跨会话记忆 + 自动摘要 + 上下文压缩 |
| v0.5.0 | 工具系统 | Skill 插件 + MCP 集成 + 子代理委派 |
| v0.6.0 | 消息网关 | Telegram/Discord/Slack 适配器 |
| v0.7.0 | 定时与自动化 | Cron 调度 + 后台任务监控 |
| v0.8.0 | 沙箱与安全 | Docker/SSH 沙箱 + 权限控制 |
| v0.9.0 | 多实例 Profile | 隔离配置 + Web Dashboard |
| v0.10.0 | Tool Gateway | 统一工具网关 + 订阅制集成 |
| v0.11.0 | Session & Stream | 会话持久化 + 流式工具调用 + 配置热重载 |
| v0.12.0 | API Server | HTTP RESTful API + SSE 流式 + 认证限流 |
| v0.13.0 | Context Window | Token 估算 + 4 种裁剪策略 + 优先级管理 |
| v0.14.0 | RAG 知识库 | 向量索引 + 语义检索 + 持久化 + API 端点 |
| v0.15.0 | Plugin Marketplace | 插件清单 + 注册中心 + 安装器 + 沙箱 + CLI/API |
| v0.16.0 | Function Calling | OpenAI 原生 FC + 多轮调用 + 流式 + API 端点 |
| v0.17.0 | Observability & Metrics | 结构化日志 + Prometheus 指标 + 三级健康检查 + CLI metrics 命令 |
| v0.18.0 | WebSocket 实时通信 | 双向实时通信 + 会话绑定 + 心跳保活 + 断线重连 + 流式推送 |
| v0.19.0 | 多语言 SOUL 模板 | TemplateManager + 6 内置模板 + 变量插值 + 语言检测 + API + CLI |
| v0.20.0 | RAG SQLite 持久化 | SQLite 向量存储 + WAL 模式 + 增量索引 + 持久化 API + REPL 命令 |
| v0.21.0 | 嵌入模型管理 | Embedder 接口 + Registry + LRU 缓存 + OpenAI/Ollama Provider + API + REPL |
| v0.22.0 | 多 Agent 协作 | Agent Registry + 任务委派 + 结果聚合 + Pipeline/Parallel/Debate 模式 + API + CLI |

## v0.22.0 新特性

### 多 Agent 协作系统

支持多个 Agent 协同完成复杂任务，提供三种协作模式：

```bash
# 列出注册的 Agent
lh agent list

# 创建并行协作任务
lh agent delegate parallel "分析这段代码" agent-1 agent-2

# 查看任务状态
lh agent task collab-1

# 取消任务
lh agent cancel collab-1
```

#### 特性

- **Agent Registry** — 注册、发现、健康检查、能力匹配
- **任务委派** — 任务拆分、超时管理、取消、重试
- **结果聚合** — 5 种聚合策略（concat/best/vote/merge/summary）
- **协作模式** — Pipeline（串行）、Parallel（并行）、Debate（辩论）
- **API 端点** — 8 个 RESTful API 端点
- **REPL 命令** — lh agent list/delegate/task/tasks/cancel

#### 协作模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| Pipeline | 串行执行，前一个输出作为后一个输入 | 多步骤流水线 |
| Parallel | 并行执行，结果聚合 | 多 Agent 同时处理 |
| Debate | 辩论模式，多轮讨论后达成共识 | 决策、评审 |

#### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/api/v1/agents` | 列出所有 Agent |
| GET  | `/api/v1/agents/get?id=` | 获取单个 Agent |
| POST | `/api/v1/agents/register` | 注册新 Agent |
| DELETE | `/api/v1/agents/deregister?id=` | 注销 Agent |
| POST | `/api/v1/agents/delegate` | 创建协作任务 |
| GET  | `/api/v1/agents/task?id=` | 获取任务状态 |
| GET  | `/api/v1/agents/tasks` | 列出所有任务 |
| POST | `/api/v1/agents/cancel?id=` | 取消任务 |

## v0.21.0 新特性

### 嵌入模型管理系统

统一的嵌入模型管理，支持多 Provider 注册、切换和缓存：

```bash
# 列出嵌入模型
/embedder

# 切换嵌入模型
/embedder switch openai-default

# 测试嵌入
/embedder test "Hello, world!"
```

#### 特性

- **Embedder 接口** — 统一的 Embed/EmbedBatch/Dimension/Name/Model 接口
- **Embedder Registry** — 注册、切换、列表管理多个嵌入模型
- **LRU 缓存** — 相同输入自动缓存向量结果，避免重复 API 调用
- **OpenAI Provider** — 支持 text-embedding-3-small/large, ada-002 及兼容端点
- **Ollama Provider** — 支持 nomic-embed-text, mxbai-embed-large 等本地模型
- **RAG 集成** — RAG 管理器使用 Embedder Registry 的 active embedder（带缓存）

#### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/api/v1/embedders` | 列出所有嵌入模型 |
| GET  | `/api/v1/embedders/{id}` | 获取嵌入模型详情 |
| POST | `/api/v1/embedders/register` | 注册新嵌入模型 |
| POST | `/api/v1/embedders/switch` | 切换活跃嵌入模型 |
| POST | `/api/v1/embedders/{id}/test` | 测试嵌入模型 |

## v0.20.0 新特性

### RAG SQLite 持久化存储

RAG 知识库支持 SQLite 后端持久化，替代纯内存/JSON 方案，支持增量更新和高效查询：

```bash
# 使用 SQLite 后端（默认启用）
/rag store sqlite --db ./data/rag.db

# 查看存储状态
/rag store status

# 切换回内存存储
/rag store memory
```

#### 特性

- **SQLite 向量存储** — 向量和元数据持久化到 SQLite 数据库
- **WAL 模式** — 启用 Write-Ahead Logging，支持并发读写
- **增量更新** — Upsert 语义，支持插入和更新
- **内存缓存** — 懒加载缓存，搜索时自动从 DB 加载
- **并发安全** — RWMutex 保护，支持多 goroutine 并发访问
- **自动持久化** — Agent 关闭时自动保存，启动时自动加载

#### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/api/v1/rag/store` | 查看存储后端状态 |
| POST | `/api/v1/rag/store` | 切换存储后端 (sqlite/memory) |

## v0.19.0 新特性

### 多语言 SOUL 模板系统

内置 SOUL 模板管理器，支持多语言人格模板的加载、变量插值和语言检测：

```bash
# 列出可用模板
lh soul templates

# 使用模板创建 SOUL
lh soul apply --template coder --lang zh

# 查看模板详情
lh soul template-info coder
```

#### 6 个内置模板

| 模板 | 说明 | 适用场景 |
|------|------|----------|
| `coder` | 编程助手 | 代码生成、调试、重构 |
| `writer` | 写作助手 | 文案、文章、翻译 |
| `analyst` | 数据分析师 | 数据分析、报告生成 |
| `tutor` | 教学助手 | 知识讲解、学习指导 |
| `creative` | 创意助手 | 头脑风暴、创意生成 |
| `minimal` | 极简模板 | 自定义起点 |

#### 变量插值

模板支持 `{{.Variable}}` 格式的变量插值，自动检测语言并填充默认值。

#### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/api/v1/soul/templates` | 模板列表 |
| GET  | `/api/v1/soul/templates/{name}` | 模板详情 |
| POST | `/api/v1/soul/apply` | 应用模板 |

## v0.18.0 新特性

### WebSocket 实时通信

内置 WebSocket 支持，实现双向实时通信、会话绑定、心跳保活和断线重连：

#### 连接与通信

```bash
# 启动 API Server（自动启用 WebSocket）
lh serve

# WebSocket 端点
ws://localhost:9090/api/v1/ws?session=my-session

# 查看 WebSocket 统计
lh ws stats
```

#### 消息协议

所有消息使用 JSON 格式：

```json
// 客户端 → 服务端
{"type": "chat", "session_id": "my-session", "data": {"message": "hello", "stream": true}}
{"type": "ping", "session_id": "my-session", "data": null}
{"type": "reconnect", "session_id": "my-session", "data": {"last_message_id": "ws-xxx"}}

// 服务端 → 客户端
{"type": "stream_chunk", "session_id": "my-session", "data": {"content": "Hello", "done": false}}
{"type": "stream_end", "session_id": "my-session", "data": {"full_response": "...", "iterations": 1}}
{"type": "status", "session_id": "my-session", "data": {"state": "thinking", "message": "processing"}}
{"type": "pong", "session_id": "my-session", "data": null}
{"type": "error", "session_id": "my-session", "data": {"code": "ERR001", "message": "..."}}
```

#### 特性

- **会话绑定** — 多客户端可连接同一 session，消息广播到同 session 所有客户端
- **心跳保活** — 自动 ping/pong，54s 间隔，60s 超时
- **断线重连** — 客户端发送 `reconnect` 消息携带 `last_message_id`
- **流式推送** — Agent 流式输出通过 WebSocket 实时推送
- **工具调用通知** — 工具调用状态通过 `tool_call` / `tool_result` 实时推送

#### API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/ws` | WebSocket | WebSocket 连接（`?session=<id>`） |
| `/api/v1/ws/stats` | GET | WebSocket 统计信息 |

## v0.17.0 新特性

### Observability & Metrics

内置可观测性系统，支持结构化日志、Prometheus 指标和三级健康检查：

#### 结构化日志

```bash
# 启动时配置日志级别和格式
lh serve --log-level debug --log-format json

# 日志级别: debug, info, warn, error
# 日志格式: json, text (默认)
```

#### Prometheus 指标

```bash
# 获取 Prometheus 格式指标
curl http://localhost:9090/api/v1/metrics

# CLI 查看指标
lh metrics
```

指标包括：
- `lh_requests_total` — 总请求数
- `lh_chat_requests_total` — 聊天请求数
- `lh_error_requests_total` — 错误请求数
- `lh_tool_calls_total` — 工具调用数
- `lh_function_calls_total` — Function Call 数
- `lh_active_sessions` — 活跃会话数
- `lh_provider_calls_total{provider=}` — Provider 调用数
- `lh_provider_errors_total{provider=}` — Provider 错误数
- `lh_provider_latency_ms{provider=}` — Provider 平均延迟

#### 三级健康检查

| 端点 | 用途 | 说明 |
|------|------|------|
| `GET /api/v1/health` | 兼容旧版 | 简单状态检查 |
| `GET /api/v1/health/live` | Liveness | 进程是否存活 |
| `GET /api/v1/health/ready` | Readiness | 是否可以接受流量 |
| `GET /api/v1/health/detail` | Detail | 详细健康状态 |

Readiness 检查包含：
- **memory** — 记忆系统是否初始化
- **provider** — Provider 是否配置

返回状态：
- `healthy` — 一切正常
- `degraded` — 部分功能降级（仍可服务）
- `unhealthy` — 关键组件不可用（返回 503）

#### Serve 命令新增参数

```bash
lh serve --metrics-addr :9091    # 独立 metrics 端口
lh serve --log-level debug       # 日志级别
lh serve --log-format json       # JSON 格式日志
```

## v0.16.0 新特性

### Function Calling (OpenAI 原生)

内置 OpenAI Function Calling 协议适配，支持多轮工具调用：

```bash
# 列出 Function Calling 工具
lh fc tools

# 查看调用历史
lh fc history

# 清除历史
lh fc clear
```

### API 端点

```bash
# 执行 function calling
curl -X POST http://localhost:9090/api/v1/fc \
  -H "Content-Type: application/json" \
  -d '{"message": "What is the weather in Tokyo?", "auto_approve": true}'

# 列出可用工具
curl http://localhost:9090/api/v1/fc/tools

# 查看调用历史
curl http://localhost:9090/api/v1/fc/history
```

### FunctionCallingProvider 接口

```go
// 支持 Function Calling 的 Provider 接口
type FunctionCallingProvider interface {
    Provider
    ChatWithOptions(ctx, messages, opts) (*Response, error)
    ChatStreamWithOptions(ctx, messages, opts) (<-chan StreamChunk, error)
}
```

## v0.15.0 新特性

### Plugin Marketplace

内置插件市场系统，支持插件的安装、卸载、更新、搜索和权限管理：

```bash
# 安装插件（本地路径）
lh plugin install /path/to/my-plugin

# 列出已安装插件
lh plugin list

# 查看插件详情
lh plugin info my-plugin

# 搜索插件
lh plugin search "web search"

# 更新插件
lh plugin update my-plugin /path/to/new-version

# 启用/禁用插件
lh plugin enable my-plugin
lh plugin disable my-plugin

# 卸载插件
lh plugin remove my-plugin
```

### plugin.yaml 清单格式

```yaml
name: my-plugin
version: 1.0.0
author: author-name
description: A cool plugin
license: MIT
homepage: https://example.com
entry: main.go
type: skill          # skill | tool | provider | hook
min_version: 0.14.0
tags:
  - search
  - web
dependencies:
  - base-plugin@1.0.0
permissions:
  - filesystem
  - network
```

### 权限系统

8 种权限级别，默认受限模式：

| 权限 | 说明 |
|------|------|
| filesystem | 文件系统访问 |
| network | 网络访问 |
| memory | 记忆系统访问 |
| tool | 工具注册 |
| rag | RAG 知识库访问 |
| session | 会话访问 |
| config | 配置修改 |
| admin | 管理员操作 |

### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/plugins` | 插件列表（支持 ?type=&status= 过滤） |
| GET | `/api/v1/plugins/search?q=` | 搜索插件 |
| POST | `/api/v1/plugins/install` | 安装插件 |
| DELETE | `/api/v1/plugins/{name}` | 卸载插件 |
| POST | `/api/v1/plugins/{name}/enable` | 启用插件 |
| POST | `/api/v1/plugins/{name}/disable` | 禁用插件 |
| GET | `/api/v1/plugins/{name}/permissions` | 查看权限 |
| POST | `/api/v1/plugins/{name}/permissions` | 授予/撤销权限 |

### 资源限制

默认沙箱限制：

| 限制项 | 默认值 |
|--------|--------|
| 最大内存 | 256 MB |
| 最大 CPU | 50% |
| 最大 Goroutine | 10 |
| 执行超时 | 30s |
| 最大输出 | 1 MB |
| 调用频率 | 60/min |

## v0.14.0 新特性

### RAG 知识库

内置 RAG（Retrieval-Augmented Generation）知识库，支持文档索引、语义检索和上下文注入：

```bash
# 索引文件到知识库
/rag index /path/to/document.md

# 索引文本内容
/rag index --text "source" "title" "content"

# 搜索知识库
/rag search "programming language"

# 查看知识库统计
/rag stats

# 列出所有文档
/rag list

# 删除文档
/rag remove <doc_id>
```

### API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/rag/index` | POST | 索引文件/文本/目录 |
| `/api/v1/rag/index` | DELETE | 删除文档 |
| `/api/v1/rag/search` | POST | 语义搜索 |
| `/api/v1/rag/stats` | GET | 知识库统计 |

### 特性

- **向量索引**：内存向量存储 + 余弦相似度搜索
- **智能分块**：按段落/句子分割，支持重叠窗口
- **MMR 重排**：Maximal Marginal Relevance 多样性重排
- **持久化**：JSON 序列化，启动自动加载，关闭自动保存
- **可扩展 Embedder**：MockEmbedder（测试）/ OpenAI Embedder（生产）/ Ollama Embedder（本地）
- **Embedder Registry**：多模型注册、切换、LRU 缓存（v0.21.0）
- **自动上下文注入**：对话时自动检索相关知识注入 system prompt

## v0.13.0 新特性

### Context Window 管理

自动管理上下文窗口，防止超出模型 token 限制：

```bash
# 查看上下文窗口状态
/context

# 手动触发裁剪
/context fit

# 查看裁剪策略
/context strategy
```

### 4 种裁剪策略

| 策略 | 说明 |
|------|------|
| TrimOldest | 优先裁剪最旧消息 |
| TrimLowPriority | 优先裁剪低优先级消息 |
| TrimSlidingWindow | 滑动窗口保留最近 N 条 |
| TrimSummarize | 摘要压缩低优先级消息 |

### Token 估算

启发式 token 估算，支持英文/中文/代码/混合文本：

```go
estimator := contextx.NewTokenEstimator()
tokens := estimator.Estimate("你好世界 Hello World")
```

### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/api/v1/context` | 上下文窗口配置查询 |
| POST | `/api/v1/context/fit` | 手动触发上下文裁剪 |

## v0.12.0 新特性

### API Server

启动 HTTP API Server，暴露 RESTful API 供外部程序调用：

```bash
# 启动 API Server
lh serve

# 自定义地址和认证
lh serve --addr :8080 --api-keys key1,key2 --rate-limit 120

# 禁用 CORS
lh serve --no-cors
```

### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/chat` | 流式聊天 (SSE) |
| POST | `/api/v1/chat/sync` | 同步聊天 |
| GET  | `/api/v1/sessions` | 会话列表 |
| GET  | `/api/v1/memory` | 记忆统计 |
| POST | `/api/v1/memory` | 保存记忆 |
| GET  | `/api/v1/memory/recall?q=` | 搜索记忆 |
| GET  | `/api/v1/memory/stats` | 记忆统计 |
| GET  | `/api/v1/tools` | 工具列表 |
| GET  | `/api/v1/stats` | 服务器统计 |
| GET  | `/api/v1/soul` | SOUL 信息 |
| GET  | `/api/v1/health` | 健康检查 |

### 认证

支持三种 API Key 传递方式：

```bash
# Header
curl -H "X-API-Key: your-key" http://localhost:9090/api/v1/stats

# Bearer Token
curl -H "Authorization: Bearer your-key" http://localhost:9090/api/v1/stats

# Query Parameter
curl "http://localhost:9090/api/v1/stats?api_key=your-key"
```

### SSE 流式聊天

```bash
curl -N -X POST http://localhost:9090/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!", "stream": true}'
```

## 快速开始

```bash
# 安装
go install github.com/yurika0211/luckyharness/cmd/lh@latest

# 初始化
lh init

# 配置 Provider
lh config set provider openai
lh config set api_key sk-xxx

# 开始对话
lh chat

# 指定 SOUL
lh chat --soul ./SOUL.md

# 查看可用模型
lh models

# 交互模式切换模型
/model gpt-4o
/models
```

## v0.3.0 新特性

### Provider 自动降级链

在 `config.yaml` 中配置降级链，当主 Provider 失败时自动切换：

```yaml
provider: openai
api_key: sk-xxx
model: gpt-4o
fallbacks:
  - provider: anthropic
    api_key: sk-ant-xxx
    model: claude-sonnet-4-20250514
  - provider: ollama
    model: llama3
```

降级链行为：
- 连续 3 次失败后自动降级到下一个 Provider
- 冷却期 5 分钟后自动恢复
- 成功调用后自动切回更高优先级的 Provider
- 支持自定义切换回调

### 新 Provider 支持

| Provider | 说明 | 认证方式 |
|----------|------|----------|
| OpenAI | GPT-4o / GPT-4o Mini / GPT-3.5 | API Key |
| Anthropic | Claude Sonnet 4 / Claude 3.5 Sonnet / Claude 3 Haiku | API Key (x-api-key header) |
| Ollama | Llama 3 / Mistral / Qwen 2 (本地) | 无需认证 |
| OpenRouter | 聚合多模型 (OpenAI 格式) | API Key |

### Model Catalog

内置 17 个模型信息，支持按 Provider / 能力筛选：

```go
catalog := provider.NewModelCatalog()
catalog.ListByProvider("anthropic")
catalog.FindByCapability("vision")
catalog.ResolveProvider("gpt-4o") // → "openai"
```

### OAuth Token 生命周期

Token 自动管理：存储、过期检测、刷新、脱敏列表。

```go
ts, _ := provider.NewTokenStore("~/.luckyharness/tokens")
ts.Set(&provider.TokenEntry{Provider: "openai", AccessToken: "sk-xxx", ExpiresAt: ...})
ts.RefreshIfNeeded("openai", refreshTokenFn)
```

## 架构

```
┌─────────────────────────────────────────────┐
│                  CLI (Cobra)                 │
├─────────┬──────────┬──────────┬─────────────┤
│  Config │  SOUL    │ Profile  │   Memory    │
├─────────┴──────────┴──────────┴─────────────┤
│              Agent Loop                      │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐ │
│  │ Reason  │→ │ Act      │→ │ Observe    │ │
│  └─────────┘  └──────────┘  └────────────┘ │
├─────────────────────────────────────────────┤
│           Provider Resolution               │
│  OpenAI │ Anthropic │ Ollama │ OpenRouter   │
├─────────────────────────────────────────────┤
│           Tool / Skill System               │
│  Built-in │ MCP │ Plugins │ Sub-agents     │
├─────────────────────────────────────────────┤
│           Messaging Gateway                 │
│  Telegram │ Discord │ Slack │ WeChat        │
└─────────────────────────────────────────────┘
```

## 开发

```bash
go test ./...
go build -o lh ./cmd/lh
```

## License

MIT
