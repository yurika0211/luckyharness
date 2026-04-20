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
- **可扩展 Embedder**：MockEmbedder（测试）/ OpenAI Embedder（生产）
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
