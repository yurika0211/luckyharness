# LuckyHarness 本地源码启动与 Telegram 接入报告

日期：2026-04-26  
仓库：`luckyharness`

## 结论

当前仓库可以直接通过源码方式启动，不依赖预编译的 `./lh` 二进制。

推荐入口：

```bash
go run ./cmd/lh
```

Telegram 接入使用：

```bash
go run ./cmd/lh msg-gateway start --platform telegram --token "<YOUR_BOT_TOKEN>"
```

该命令会同时拉起 HTTP API Server 和 Telegram 网关，不需要你先单独执行 `serve`。

## 代码确认结果

我核对了当前源码中的实际入口与命令定义：

- `cmd/lh/main.go`
  - 主程序入口
  - 调用 `internal/cli/lhcmd.Execute()`
- `internal/cli/lhcmd/root_cmd.go`
  - 当前源码包含 `msg-gateway` 子命令
- `internal/cli/lhcmd/commands.go`
  - `runMsgGatewayStart()` 会：
    - 创建 Agent
    - 启动 HTTP API Server
    - 注册 Telegram Adapter
    - 启动 Telegram 网关
- `internal/config/config.go`
  - Telegram 支持以下配置项：
    - `msg_gateway.telegram.token`
    - `msg_gateway.telegram.chat_timeout_seconds`
    - `msg_gateway.telegram.progress_as_messages`
    - `msg_gateway.telegram.progress_as_natural_language`
    - `msg_gateway.telegram.show_tool_details_in_result`

## 前置条件

### 1. Go 版本

仓库 workflow 使用的是 Go `1.25`，本地也建议保持一致。

检查：

```bash
go version
```

### 2. LLM Provider 配置

Telegram 只是消息入口，真正回复消息仍然依赖 LLM provider。

至少需要配置：

- `provider`
- `api_key`
- 可选：`api_base`
- 可选：`model`

## 推荐的本地启动方式

### 方案 A：直接使用默认 HOME

配置会写到：

```bash
~/.luckyharness/config.json
```

初始化并配置：

```bash
cd /media/shiokou/DevRepo43/DevHub/Projects/2026-myapp/luckyharness

go run ./cmd/lh init
go run ./cmd/lh config set provider openai
go run ./cmd/lh config set api_key sk-xxx
go run ./cmd/lh config set model gpt-5.4-mini
```

如果你走兼容 OpenAI 的中转站，也要配：

```bash
go run ./cmd/lh config set api_base https://your-api-base/v1
```

### 方案 B：把配置隔离在项目目录里

如果你不想污染全局 `~/.luckyharness`，推荐先切换 `HOME`：

```bash
cd /media/shiokou/DevRepo43/DevHub/Projects/2026-myapp/luckyharness
export HOME="$PWD/.lh-home"
```

然后再执行初始化和配置：

```bash
go run ./cmd/lh init
go run ./cmd/lh config set provider openai
go run ./cmd/lh config set api_key sk-xxx
go run ./cmd/lh config set model gpt-5.4-mini
```

这样配置会落到：

```bash
.lh-home/.luckyharness/config.json
```

## 本地源码启动命令

### 1. 先验证 CLI 能跑

```bash
go run ./cmd/lh version
```

### 2. 启动交互模式

```bash
go run ./cmd/lh chat
```

### 3. 单独启动 API Server

如果你只想本地调 API：

```bash
go run ./cmd/lh serve --addr 127.0.0.1:9090
```

## Telegram 接入方式

### 1. 获取 Bot Token

在 Telegram 里找 `@BotFather`：

- `/newbot`
- 按提示创建 bot
- 复制得到的 token

### 2. 启动 Telegram 网关

最直接的源码启动命令：

```bash
go run ./cmd/lh msg-gateway start --platform telegram --token "<YOUR_BOT_TOKEN>" --api-addr 127.0.0.1:9090
```

说明：

- `--platform telegram`
  - 指定启动 Telegram 网关
- `--token`
  - BotFather 给你的 token
- `--api-addr`
  - 同时启动的本地 HTTP API 地址

### 3. 用配置文件方式启动

也可以先把 token 写进配置：

```bash
go run ./cmd/lh config set msg_gateway.platform telegram
go run ./cmd/lh config set msg_gateway.telegram.token "<YOUR_BOT_TOKEN>"
go run ./cmd/lh config set msg_gateway.api_addr 127.0.0.1:9090
```

然后直接启动：

```bash
go run ./cmd/lh msg-gateway start
```

## 推荐的 Telegram 配置项

当前源码支持的 Telegram 行为开关：

```bash
go run ./cmd/lh config set msg_gateway.telegram.chat_timeout_seconds 600
go run ./cmd/lh config set msg_gateway.telegram.progress_as_messages true
go run ./cmd/lh config set msg_gateway.telegram.progress_as_natural_language false
go run ./cmd/lh config set msg_gateway.telegram.show_tool_details_in_result false
```

建议起步值：

- `progress_as_messages=true`
  - 中间步骤单独发消息
- `progress_as_natural_language=false`
  - 先关闭，避免输出风格过于“播报化”
- `show_tool_details_in_result=false`
  - 先关闭，保持最终回答简洁

如果你想让 bot 像“过程汇报型助手”那样逐步播报，可以改成：

```bash
go run ./cmd/lh config set msg_gateway.telegram.progress_as_natural_language true
go run ./cmd/lh config set msg_gateway.telegram.show_tool_details_in_result true
```

## 启动后的验证方法

### 1. 查看帮助

```bash
go run ./cmd/lh msg-gateway start --help
```

### 2. Telegram 侧验证

启动后，在 Telegram 中：

- 打开你的 bot 私聊窗口
- 发送 `/start`
- 再发送一条普通消息，例如：

```text
你好，帮我介绍一下你自己
```

如果 LLM provider 正常，bot 会返回回答。

### 3. 查看网关状态

另开一个终端：

```bash
go run ./cmd/lh msg-gateway status --api-addr 127.0.0.1:9090
```

### 4. 停止网关

```bash
go run ./cmd/lh msg-gateway stop telegram --api-addr 127.0.0.1:9090
```

## 常见问题

### 1. `telegram 需要 --token 参数`

说明当前没有通过命令行或配置文件提供 token。

处理：

```bash
go run ./cmd/lh config set msg_gateway.telegram.token "<YOUR_BOT_TOKEN>"
```

或者直接启动时带：

```bash
go run ./cmd/lh msg-gateway start --platform telegram --token "<YOUR_BOT_TOKEN>"
```

### 2. bot 能收到消息，但回复失败

通常不是 Telegram 问题，而是上游 LLM 配置问题。

检查：

```bash
go run ./cmd/lh config list
```

重点确认：

- `provider`
- `api_key`
- `api_base`
- `model`

### 3. 本地配置和项目配置混淆

如果你没有显式设置：

```bash
export HOME="$PWD/.lh-home"
```

那么源码默认会读写：

```bash
~/.luckyharness/config.json
```

这通常是“为什么我明明改了项目里的配置，但启动结果不对”的根因。

### 4. 启动命令阻塞终端

这是正常行为。

`msg-gateway start` 会持续运行，直到你按：

```bash
Ctrl+C
```

## 推荐操作顺序

建议按下面顺序做：

1. 进入项目目录
2. 可选：`export HOME="$PWD/.lh-home"`
3. `go run ./cmd/lh init`
4. 配好 `provider/api_key/model/api_base`
5. 先用 `go run ./cmd/lh chat "hello"` 验证 LLM 是否可用
6. 再执行 `go run ./cmd/lh msg-gateway start --platform telegram --token ...`
7. 去 Telegram 私聊 bot 验证

## 最小可执行示例

```bash
cd /media/shiokou/DevRepo43/DevHub/Projects/2026-myapp/luckyharness

export HOME="$PWD/.lh-home"

go run ./cmd/lh init
go run ./cmd/lh config set provider openai
go run ./cmd/lh config set api_key sk-xxx
go run ./cmd/lh config set model gpt-5.4-mini

go run ./cmd/lh chat "hello"

go run ./cmd/lh msg-gateway start --platform telegram --token "<YOUR_BOT_TOKEN>" --api-addr 127.0.0.1:9090
```

## 当前判断

当前仓库在“源码本地启动 + Telegram 接入”这条链路上是通的，且源码命令定义与配置结构是一致的。

最需要注意的不是 Telegram 本身，而是：

- 你到底在用哪个 `HOME`
- 你的 `provider/api_key/api_base/model` 是否正确
- 你是否理解 `msg-gateway start` 会顺带启动 HTTP API Server
