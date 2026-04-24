# LuckyHarness 配置指南 v0.64.0

## 配置文件位置

统一使用一个配置文件：

`~/.luckyharness/config.json`

程序启动时会加载这个文件，未显式传 CLI 参数时使用这里的值。

## 完整示例

请直接参考仓库根目录：

`config.example.json`

## 启动命令与配置映射

- `lh chat` 读取：`provider/api_key/api_base/model/soul_path/max_tokens/temperature` 和 `agent.*`
- `lh serve` 读取：`server.*`
- `lh dashboard start` 读取：`dashboard.addr`
- `lh msg-gateway start` 读取：`msg_gateway.*`

说明：

- CLI 参数优先级高于 `config.json`。
- 未传 CLI 参数时，启动命令会自动回落到 `config.json`。

## 常用字段

- LLM 主配置：`provider`, `api_key`, `api_base`, `model`, `max_tokens`, `temperature`
- Provider 额外请求头：`extra_headers`
- 重试/熔断/限流：`retry`, `circuit_breaker`, `rate_limit`
- Agent Loop：`agent.max_iterations`, `agent.timeout_seconds`, `agent.auto_approve`
- API Server：`server.addr`, `server.api_keys`, `server.enable_cors`, `server.rate_limit`
- 消息网关：`msg_gateway.platform`, `msg_gateway.telegram.token`, `msg_gateway.onebot.*`

## 生效方式

编辑 `config.json` 后重启对应进程即可生效。

示例：

```bash
pkill -9 lh
lh serve
```
