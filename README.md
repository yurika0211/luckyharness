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
