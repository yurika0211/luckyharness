# 配置指南

## 启动命令

### Telegram Bot

```bash
# 前台运行（调试用）
./lh msg-gateway start --platform telegram --token <BOT_TOKEN>

# 后台运行
nohup ./lh msg-gateway start --platform telegram --token <BOT_TOKEN> &

# 同时启动 HTTP API :9090（默认行为）
nohup ./lh msg-gateway start --platform telegram --token <BOT_TOKEN> &
```

### 交互式聊天

```bash
# YOLO 模式（自动执行工具，不确认）
./lh chat --yolo

# 普通模式（工具执行前确认）
./lh chat
```

### 测试

```bash
# 单元测试
go test ./...

# 评估框架
./lh eval run ./testcases/
```

## 配置文件

### 主配置

配置文件位于 `~/.luckyharness/config.yaml`（或通过命令行参数指定）。

```yaml
# LLM Provider 配置
provider:
  name: openai-compat
  api_base: http://maas.icompify.com:32788/v1
  api_key: sk-xxx
  model: glm-5.1
  extra_headers:
    X-Custom-Header: value

# Telegram 配置
telegram:
  token: "8675623968:AAEaxF3Fm4wVyDWYR6mVFIqPoI5G7R90Mlc"
  # 可选：自定义 Bot API 服务器
  # base_url: "https://custom-bot-api.example.com"
  # base_file_url: "https://custom-bot-api.example.com"

# HTTP API 配置
http:
  port: 9090

# Session 配置
session:
  dir: ~/.luckyharness/sessions
  max_context_messages: 100
  max_tokens: 131072

# Skill 配置
skills:
  dir: ~/.luckyharness/skills
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `LUCKYHARNESS_CONFIG` | 配置文件路径 | `~/.luckyharness/config.yaml` |
| `LUCKYHARNESS_SESSION_DIR` | Session 存储目录 | `~/.luckyharness/sessions` |
| `LUCKYHARNESS_SKILLS_DIR` | Skill 目录 | `~/.luckyharness/skills` |

## LLM Provider

### icompify（当前使用）

- **API 地址**: `http://maas.icompify.com:32788/v1`
- **模型**: glm-5.1
- **格式**: 兼容 OpenAI

### FallbackChain

支持配置多个 Provider，按优先级降级：

```yaml
providers:
  - name: primary
    api_base: http://primary-api/v1
    api_key: sk-primary
    model: glm-5.1
  - name: fallback
    api_base: http://fallback-api/v1
    api_key: sk-fallback
    model: gpt-3.5-turbo
```

## Session 管理

- 每个 chat 独立 Session
- 持久化到 `~/.luckyharness/sessions/{id}.json`
- `ContextWindow` 自动裁剪旧消息
- `TrimLowPriority` 策略：优先保留 system prompt 和最近消息

## Skill 加载

启动时自动扫描 `~/.luckyharness/skills/` 目录：

1. 读取每个 Skill 的 `SKILL.md`
2. 解析 frontmatter 元数据
3. 提取摘要（Trigger/Workflow/Steps）
4. 生成工具定义（自动或从 `## Tools` section）
5. 注入摘要到 system prompt