# 已接入模块

LuckyHarness v0.36.0 当前已接入并正常工作的模块清单。

## ✅ 已接入

### Agent Loop + Function Calling
- 核心 RunLoop 实现，支持多轮对话
- OpenAI 兼容的 Function Calling 协议
- 工具调用结果写回 Session 上下文
- `max_tokens=131072`，支持长上下文

### 内置工具
- `shell` — 执行 shell 命令
- `read_file` / `write_file` — 文件读写
- `web_search` / `web_fetch` — 联网搜索与抓取
- `skill_read` — 按需读取完整 SKILL.md（v0.36.0 新增）
- 其他基础工具

### Skill 系统（88 个 Skill）
- **SkillLoader**：解析 frontmatter + `## Tools` section + 自动生成 `run` tool
- **脚本执行**：通过 `SKILL_ARG_*` 环境变量传递参数
- **两层设计**（v0.36.0）：
  - 88 个 Skill 摘要（~8.7K tokens）直接注入 `buildMessages` system prompt
  - `skill_read(name)` 工具按需读取完整 SKILL.md
  - `SkillInfo.Summary` 字段，`extractSummary` 解析 Trigger/Workflow/Steps 等 section
  - 84/88 skills 有摘要

### RAG 上下文检索
- `SearchWithContext` 5s 超时
- `[Retrieved Knowledge]` 注入到消息上下文
- `indexConversationTurn` 每轮对话后自动索引

### 多轮会话
- Session per-chat 隔离
- 持久化 `/root/.luckyharness/sessions/{id}.json`
- `ContextWindow` + `TrimLowPriority` 裁剪策略

### Shell 上下文持久化
- `ShellContext` with `Cwd` + `Env`
- `ShellAware` tool flag
- `CallWithShellContext` 自动检测 `cd`/`export`/`unset`

### 协作代理
- Delegate 机制，支持委派任务给子代理

### Embedder + 向量存储
- 文本嵌入与向量存储，支撑 RAG 检索

### 记忆系统
- 长期记忆存储与检索

### FallbackChain
- 多 Provider 降级链，主 Provider 失败自动切换

### Telegram 网关
- Bot Token 认证
- 消息收发（纯文本）
- **多媒体处理**（v0.36.0）：图片/语音/视频/文件附件
- `gateway.Message` 新增 `Attachments` 字段

### Cron 引擎（v0.36.0）
- Bot 命令：`/cron add`、`/cron remove`、`/cron pause`、`/cron resume`
- 定时任务调度与执行

### Metrics 追踪（v0.36.0）
- Chat 追踪：消息计数、响应时间
- Tool 追踪：调用次数、成功率
- Bot 命令：`/metrics`

### HTTP API + WebSocket（v0.36.0）
- `:9090` 端口并行启动
- REST API 端点
- WebSocket 实时通信
- Bot 命令：`/health`

### Bot 命令扩展（v0.36.0）
- `/skills` — 列出已加载 Skill
- `/cron` — 定时任务管理
- `/metrics` — 查看追踪数据
- `/health` — 健康检查

## ⚠️ 已知问题

| 问题 | 影响 | 优先级 |
|------|------|--------|
| Telegram adapter 静默丢弃非文本消息 | 图片/语音/视频无法处理 | 高 |
| `gateway.Message` 无 Attachments/Media 字段 | 多媒体数据丢失 | 高 |
| `provider.Message.Content` 是 string | 不支持 OpenAI content parts 格式 | 中 |
| `DetectModality` panic bug | mimeType[:5] 短字符串崩溃 | 高 |
| TokenEstimator 简单字符估算 | 不精确 | 低 |
| trimSummarize 简单截断 | 摘要质量低 | 低 |
| Session tool_calls 缺结构化信息 | 只有文本摘要 | 中 |

## ❌ 未接入

| 模块 | 说明 |
|------|------|
| workflow | 任务编排 |
| mq | 消息队列 |
| telemetry | OpenTelemetry |
| configcenter | 配置中心 |
| multimodal Provider | 需要 vision 模型 API |

## ❌ 缺失功能

| 功能 | 说明 |
|------|------|
| 流式响应 | RunLoopStream 代码有但 Telegram handler 用同步 |
| Skill AI 编排 | 当前是脚本执行，不是 AI 编排 |