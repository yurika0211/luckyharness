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
