# Bot 命令手册

LuckyHarness Telegram Bot 支持的命令列表。

## 通用命令

| 命令 | 说明 |
|------|------|
| `/start` | 启动 Bot，显示欢迎消息 |
| `/help` | 显示帮助信息 |
| `/skills` | 列出已加载的 Skill（v0.36.0） |
| `/health` | 健康检查，显示 Bot 和 HTTP API 状态（v0.36.0） |

## 定时任务（v0.36.0）

| 命令 | 说明 | 示例 |
|------|------|------|
| `/cron add` | 添加定时任务 | `/cron add 0 9 * * * 每天早上9点提醒我站会` |
| `/cron remove` | 删除定时任务 | `/cron remove <task_id>` |
| `/cron pause` | 暂停定时任务 | `/cron pause <task_id>` |
| `/cron resume` | 恢复定时任务 | `/cron resume <task_id>` |

## 追踪与监控（v0.36.0）

| 命令 | 说明 |
|------|------|
| `/metrics` | 查看 Chat 和 Tool 追踪数据 |

## 自然语言交互

除了命令，Bot 还支持自然语言对话：

- 直接发送消息即可与 AI 对话
- Bot 会自动识别意图并调用对应 Skill
- 支持多轮对话，上下文自动保持
- 工具调用对用户透明，Bot 会自动执行并返回结果

## 使用示例

```
用户: 帮我搜索一下 Go 1.24 的新特性
Bot: [调用 web_search 工具] → 返回搜索结果摘要

用户: 现在北京天气怎么样？
Bot: [调用 weather skill] → 返回天气信息

用户: /cron add 0 9 * * 1-5 工作日早9点提醒我写日报
Bot: ✅ 定时任务已创建 (ID: cron_abc123)
     ⏰ Cron: 0 9 * * 1-5
     📝 内容: 工作日早9点提醒我写日报
```

## 配置

Bot 使用 icompify API 作为 LLM 后端：

- **模型**: glm-5.1
- **API**: 兼容 OpenAI 格式
- **启动命令**: `nohup ./lh msg-gateway start --platform telegram --token <TOKEN> &`