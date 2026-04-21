# 开发路线图

## 已完成版本

### v0.36.0（当前）

**全模块接入** — Telegram 多媒体、Cron 引擎、Metrics、HTTP API、Bot 命令扩展

- ✅ Telegram 多媒体处理（图片/语音/视频/文件附件）
- ✅ `gateway.Message` Attachments 字段
- ✅ Cron 引擎（bot 命令 /cron add/remove/pause/resume）
- ✅ Metrics 追踪（chat/tool 追踪）
- ✅ HTTP API :9090 并行启动
- ✅ Bot 命令扩展（/skills /cron /metrics /health）
- ✅ Skill 两层设计（摘要注入 + 按需读取）
- ✅ SkillInfo.Summary 字段 + extractSummary
- ✅ 84/88 skills 有摘要

### v0.35.2

**增强 SkillLoader**

- ✅ Frontmatter 解析
- ✅ 无 `## Tools` 的 Skill 自动生成 run tool
- ✅ 脚本执行 handler（SKILL_ARG_* 环境变量）
- ✅ sanitizeName

### v0.35.1

**自动加载 Skill**

- ✅ Agent 启动时扫描 `~/.luckyharness/skills/`
- ✅ 88 skills 从 nanobot 加载

### v0.35.0

**RAG 上下文检索**

- ✅ `SearchWithContext` 5s 超时
- ✅ `[Retrieved Knowledge]` 注入
- ✅ `indexConversationTurn` 每轮索引

### v0.34.0

**Shell 上下文持久化**

- ✅ `ShellContext` with Cwd + Env
- ✅ `ShellAware` tool flag
- ✅ `CallWithShellContext`
- ✅ 自动检测 cd/export/unset

### v0.33.1

**Bug 修复**

- ✅ Tool calling `required` nil bug（Go []string zero value → null）
- ✅ ContextWindow trimming
- ✅ Tool 中间结果写入 session
- ✅ max_tokens 4096→131072

### v0.33.0

**消息网关工具调用**

- ✅ `chatWithSession` → `RunLoopWithSession`
- ✅ FallbackChain 实现 FunctionCallingProvider
- ✅ msg-gateway start signal blocking

### v0.31.0

**数据竞争修复 + 插件市场**

- ✅ DelegateToSkill/DelegateToMCP 数据竞争修复
- ✅ Plugin Marketplace（v0.15.0）
- ✅ Multi-turn RAG（v0.16.0）

## 待开发功能

### 高优先级

| 功能 | 说明 | 状态 |
|------|------|------|
| Telegram 多媒体完整支持 | 修复 adapter 静默丢弃 + Message Attachments | 🔴 阻塞 |
| DetectModality panic 修复 | mimeType[:5] 短字符串崩溃 | 🔴 阻塞 |
| provider.Message content parts | 支持 OpenAI content parts 格式 | 🟡 进行中 |
| 流式响应 | RunLoopStream + Telegram handler | 🟡 代码已有 |

### 中优先级

| 功能 | 说明 | 状态 |
|------|------|------|
| Skill AI 编排 | 从脚本执行升级为 AI 编排 | 📋 规划中 |
| Session tool_calls 结构化 | 添加 ID、参数等结构化信息 | 📋 规划中 |
| trimSummarize 升级 | 接入 LLM 做摘要压缩 | 📋 规划中 |
| TokenEstimator 升级 | 使用真正的 tokenizer | 📋 规划中 |

### 低优先级

| 功能 | 说明 | 状态 |
|------|------|------|
| Workflow 任务编排 | DAG 式任务编排 | 📋 规划中 |
| MQ 消息队列 | 异步消息处理 | 📋 规划中 |
| OpenTelemetry | 分布式追踪 | 📋 规划中 |
| ConfigCenter | 动态配置中心 | 📋 规划中 |
| Multimodal Provider | Vision 模型 API 接入 | 📋 规划中 |

## 版本规划

```
v0.37.0 — Telegram 多媒体完整支持 + 流式响应
v0.38.0 — Skill AI 编排 + Session 结构化
v0.39.0 — Workflow 任务编排
v0.40.0 — OpenTelemetry + ConfigCenter
v1.0.0  — 稳定版发布
```