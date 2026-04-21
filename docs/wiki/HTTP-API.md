# HTTP API 文档

LuckyHarness v0.36.0 在启动 Telegram Bot 时会并行启动 HTTP API 服务，默认监听 `:9090`。

## 基础信息

- **协议**: HTTP/1.1 + WebSocket
- **默认端口**: 9090
- **Content-Type**: `application/json`

## 端点

### 健康检查

```
GET /health
```

**响应**：
```json
{
  "status": "ok",
  "uptime": "2h30m",
  "version": "0.36.0"
}
```

### 发送消息

```
POST /chat
```

**请求体**：
```json
{
  "session_id": "optional-session-id",
  "message": "你好",
  "stream": false
}
```

**响应**：
```json
{
  "session_id": "abc123",
  "reply": "你好！有什么可以帮你的吗？",
  "tool_calls": []
}
```

### WebSocket

```
WS /ws
```

实时双向通信，支持流式响应。

### Metrics

```
GET /metrics
```

**响应**：
```json
{
  "chat": {
    "total_messages": 1234,
    "avg_response_time_ms": 1500
  },
  "tools": {
    "total_calls": 567,
    "success_rate": 0.95
  }
}
```

## 启动

HTTP API 与 Telegram Bot 同时启动：

```bash
nohup ./lh msg-gateway start --platform telegram --token <TOKEN> &
```

启动后日志会显示：
```
HTTP API listening on :9090
Telegram bot started
```