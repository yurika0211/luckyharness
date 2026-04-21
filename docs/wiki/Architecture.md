# 架构总览

LuckyHarness 采用模块化架构，核心是一个 **Agent Loop**，围绕它构建了消息网关、工具系统、知识库、记忆系统等外围模块。

## 核心架构图

```
┌─────────────────────────────────────────────────────────┐
│                    Message Gateway                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Telegram │  │   HTTP   │  │   CLI    │              │
│  │ Adapter  │  │  Adapter │  │ Adapter  │              │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘              │
│       │              │              │                    │
│       └──────────────┼──────────────┘                    │
│                      ▼                                   │
│              ┌──────────────┐                            │
│              │  Agent Loop  │ ◄── Function Calling       │
│              │  (RunLoop)   │                            │
│              └──────┬───────┘                            │
│                     │                                    │
│       ┌─────────────┼─────────────┐                     │
│       ▼             ▼             ▼                     │
│  ┌─────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Provider│  │  Tools   │  │   RAG    │              │
│  │ (LLM)   │  │ System   │  │ Engine   │              │
│  └─────────┘  └──────────┘  └──────────┘              │
│                     │                                    │
│       ┌─────────────┼─────────────┐                     │
│       ▼             ▼             ▼                     │
│  ┌─────────┐  ┌──────────┐  ┌──────────┐              │
│  │  Skill  │  │  Memory  │  │  Shell   │              │
│  │ System  │  │  System  │  │ Context  │              │
│  └─────────┘  └──────────┘  └──────────┘              │
│                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │  Cron    │  │ Metrics  │  │ Embedder │              │
│  │  Engine  │  │ Tracker  │  │ + Vector │              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
```

## 模块依赖关系

| 模块 | 依赖 | 被依赖 |
|------|------|--------|
| Agent Loop | Provider, Tools | Gateway |
| Message Gateway | Agent Loop | - |
| Provider (LLM) | - | Agent Loop |
| Tools System | Skill, Shell | Agent Loop |
| Skill System | SkillLoader | Tools |
| RAG Engine | Embedder, VectorStore | Agent Loop |
| Memory System | - | Agent Loop |
| Shell Context | - | Tools |
| Cron Engine | Agent Loop | - |
| Metrics | - | Gateway |
| HTTP API | Gateway | - |

## 数据流

1. **消息接收**：Gateway Adapter 接收平台消息 → 转换为统一 `gateway.Message`
2. **会话管理**：按 chat ID 查找/创建 Session，加载历史上下文
3. **上下文构建**：`buildMessages` 组装 system prompt + skill 摘要 + RAG 检索结果 + 历史消息
4. **LLM 调用**：Provider 发送请求，支持 Function Calling
5. **工具执行**：解析 tool_calls → 路由到对应 handler → 返回结果
6. **响应发送**：最终回复通过 Gateway Adapter 发回平台

## 关键设计决策

- **Session 隔离**：每个 chat 独立 Session，持久化到 `~/.luckyharness/sessions/{id}.json`
- **Context Window 管理**：`TrimLowPriority` 策略，超出时优先裁剪旧消息
- **Fallback Chain**：多 Provider 降级链，主 Provider 失败自动切换
- **Skill 两层设计**：摘要注入 system prompt（~8.7K tokens）+ 按需读取完整 SKILL.md