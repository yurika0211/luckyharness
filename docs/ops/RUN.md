 在项目根目录直接这样启动（推荐）：

  cd /media/shiokou/DevRepo43/DevHub/Projects/2026-myapp/luckyharness

  # 1) 初始化
  go run ./cmd/lh init

  # 2) 配置模型提供商（示例：OpenAI）
  go run ./cmd/lh config set provider openai
  go run ./cmd/lh config set api_key sk-xxx

  # 3) 启动交互聊天
  go run ./cmd/lh chat

  如果要启动 API 服务：

  go run ./cmd/lh serve --addr :9090
  
  go run ./cmd/lh msg-gateway start --platform telegram

  如果你更想用编译后的二进制：

  go build -o lh ./cmd/lh
  ./lh serve

  注意：仓库里现有 ./lh 是旧构建（v0.38.2），源码当前跑出来是 v0.20.0，建议先
  go build 再用 ./lh。
  如果想把配置写在项目内而不是 ~/.luckyharness，先执行：

  export HOME="$PWD/.lh-home"
  cd /media/shiokou/DevRepo43/DevHub/Projects/2026-myapp/luckyharness
 
   # 先配好大模型（否则机器人收得到消息但可能回复失败）
   ./lh init
   ./lh config set provider openai
   ./lh config set api_key sk-xxx
 
   # 启动 Telegram 网关（token 从 @BotFather 获取）
   ./lh msg-gateway start --platform telegram --token "<YOUR_TG_BOT_TOKEN>"
   --api-addr 127.0.0.1:9090
 
   然后去 Telegram 给你的 bot 发消息即可。
 
   常用管理命令：
 
   ./lh msg-gateway status --api-addr 127.0.0.1:9090
   ./lh msg-gateway stop telegram --api-addr 127.0.0.1:9090
