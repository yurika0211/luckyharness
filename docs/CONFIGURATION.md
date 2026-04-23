# LuckyHarness 配置指南 v0.56.0

## 📋 完整配置示例

```json
{
  "provider": "openai",
  "api_key": "sk-your-api-key",
  "api_base": "https://api.boaiak.com/v1",
  "model": "gpt-5.4-mini",
  "soul_path": "/root/.luckyharness/SOUL.md",
  
  "limits": {
    "max_tokens": 4096,
    "temperature": 0.7,
    "timeout_seconds": 60,
    "max_timeout_seconds": 600,
    "max_tool_calls": 5,
    "max_concurrent_tool_calls": 3
  },
  
  "retry": {
    "enabled": true,
    "max_attempts": 3,
    "initial_delay_ms": 1000,
    "max_delay_ms": 10000,
    "retry_on_rate_limit": true,
    "retry_on_timeout": true,
    "retry_on_server_error": true
  },
  
  "circuit_breaker": {
    "enabled": false,
    "error_threshold": 5,
    "window_seconds": 60,
    "timeout_seconds": 30,
    "half_open_max_requests": 1
  },
  
  "rate_limit": {
    "enabled": true,
    "requests_per_minute": 60,
    "tokens_per_minute": 100000,
    "burst_size": 10
  },
  
  "memory": {
    "short_term_max_turns": 10,
    "midterm_expire_days": 90,
    "midterm_max_summaries": 100
  },
  
  "context": {
    "max_history_turns": 50,
    "max_context_tokens": 8000,
    "compression_threshold": 0.8
  }
}
```

## 🔧 配置项说明

### 基础配置

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `provider` | Provider 类型 | `openai` |
| `api_key` | API 密钥 | 必填 |
| `api_base` | API 基础 URL | `https://api.openai.com/v1` |
| `model` | 模型名称 | `gpt-4o` |
| `soul_path` | 角色配置文件路径 | - |

### 限制配置 (`limits`)

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `max_tokens` | 最大生成 token 数 | 4096 |
| `temperature` | 温度参数 | 0.7 |
| `timeout_seconds` | 单次调用超时（秒） | 60 |
| `max_timeout_seconds` | 最大允许超时（秒） | 600 |
| `max_tool_calls` | 单次响应最大工具调用数 | 5 |
| `max_concurrent_tool_calls` | 最大并发工具调用数 | 3 |

### 重试配置 (`retry`)

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `enabled` | 是否启用重试 | `true` |
| `max_attempts` | 最大重试次数 | 3 |
| `initial_delay_ms` | 初始重试延迟（毫秒） | 1000 |
| `max_delay_ms` | 最大重试延迟（毫秒） | 10000 |
| `retry_on_rate_limit` | 429 错误重试 | `true` |
| `retry_on_timeout` | 超时重试 | `true` |
| `retry_on_server_error` | 5xx 错误重试 | `true` |

### 熔断器配置 (`circuit_breaker`)

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `enabled` | 是否启用熔断器 | `false` |
| `error_threshold` | 错误次数阈值 | 5 |
| `window_seconds` | 错误统计窗口（秒） | 60 |
| `timeout_seconds` | 熔断超时（秒） | 30 |
| `half_open_max_requests` | 半开状态最大请求数 | 1 |

### 限流配置 (`rate_limit`)

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `enabled` | 是否启用限流 | `true` |
| `requests_per_minute` | 每分钟最大请求数 | 60 |
| `tokens_per_minute` | 每分钟最大 token 数 | 100000 |
| `burst_size` | 突发请求数 | 10 |

### 内存配置 (`memory`)

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `short_term_max_turns` | 短期记忆最大轮数 | 10 |
| `midterm_expire_days` | 中期记忆过期天数 | 90 |
| `midterm_max_summaries` | 中期记忆最大摘要数 | 100 |

### 上下文配置 (`context`)

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `max_history_turns` | 最大历史对话轮数 | 50 |
| `max_context_tokens` | 最大上下文 token 数 | 8000 |
| `compression_threshold` | 压缩阈值 | 0.8 |

## 🚀 使用方式

### 1. 编辑配置文件

```bash
nano /root/.luckyharness/config.json
```

### 2. 重启 LuckyHarness

```bash
pkill -9 lh
cd /root/.luckyharness
./lh msg-gateway start --platform telegram --token YOUR_TOKEN
```

### 3. 验证配置

```bash
curl http://localhost:9090/api/v1/health
```

## 📝 配置优先级

1. **配置文件** (`config.json`) - 最高优先级
2. **环境变量** - 次优先级
3. **代码默认值** - 最低优先级

## ⚠️ 注意事项

- `max_tokens` 不能超过模型的最大上下文窗口
- `timeout_seconds` 应该小于 `max_timeout_seconds`
- `max_attempts` 设置过大会增加延迟
- 熔断器和限流可以同时启用
- 生产环境建议启用熔断器

## 🔍 故障排查

### 查看当前配置

```bash
cat /root/.luckyharness/config.json | python3 -m json.tool
```

### 测试配置是否生效

```bash
# 发送测试消息
curl -X POST http://localhost:9090/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "test"}'
```

### 查看日志

```bash
tail -f /tmp/lh-*.log
```

## 📊 推荐配置

### 开发环境

```json
{
  "limits": {
    "max_tokens": 2048,
    "timeout_seconds": 30
  },
  "retry": {
    "max_attempts": 2
  },
  "rate_limit": {
    "requests_per_minute": 30
  }
}
```

### 生产环境

```json
{
  "limits": {
    "max_tokens": 4096,
    "timeout_seconds": 60
  },
  "retry": {
    "max_attempts": 3
  },
  "circuit_breaker": {
    "enabled": true,
    "error_threshold": 5
  },
  "rate_limit": {
    "requests_per_minute": 60
  }
}
```

### 高并发环境

```json
{
  "limits": {
    "max_tokens": 8192,
    "timeout_seconds": 120,
    "max_tool_calls": 10
  },
  "retry": {
    "max_attempts": 5
  },
  "circuit_breaker": {
    "enabled": true,
    "error_threshold": 10
  },
  "rate_limit": {
    "requests_per_minute": 120,
    "burst_size": 20
  }
}
```
