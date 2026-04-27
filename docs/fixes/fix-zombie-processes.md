# 修复僵尸进程问题

## 问题描述

运行 `lh msg-gateway start` 命令后，会产生大量僵尸进程（`[lh] <defunct>`），这些进程占用进程表项但不释放。

## 根本原因

1. **信号处理不完整**: `runMsgGatewayStart` 函数使用 `signal.Notify` 捕获信号，但只在主 goroutine 中阻塞等待
2. **context 取消后未等待**: 当 context 被取消时，网关和服务器停止，但没有等待所有 goroutine 完成
3. **子进程未正确回收**: Telegram 和 OneBot 适配器可能启动子进程，但主进程退出时未正确回收

## 解决方案

### 1. 添加 WaitGroup 等待所有 goroutine 完成

```go
func runMsgGatewayStart(cmd *cobra.Command, args []string) error {
    // ... 现有代码 ...
    
    var wg sync.WaitGroup
    
    // 启动 HTTP API Server
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := srv.Start(); err != nil {
            fmt.Printf("[server] HTTP API error: %v\n", err)
        }
    }()
    
    // ... 启动网关 ...
    
    // 等待信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    
    fmt.Println("\n🛑 正在停止消息网关...")
    _ = gm.StopAll()
    _ = srv.Stop()
    
    // 等待所有 goroutine 完成
    wg.Wait()
    
    return nil
}
```

### 2. 确保网关 Stop 方法等待所有 goroutine 完成

在 `internal/gateway/gateway.go` 中：

```go
type Gateway struct {
    // ... 现有字段 ...
    wg sync.WaitGroup
}

func (g *Gateway) Start(ctx context.Context, name string) error {
    // ... 现有代码 ...
    
    g.wg.Add(1)
    go func() {
        defer g.wg.Done()
        // 运行适配器
    }()
    
    return nil
}

func (g *Gateway) StopAll() error {
    // 停止所有适配器
    // ...
    
    // 等待所有 goroutine 完成
    g.wg.Wait()
    
    return nil
}
```

### 3. 添加 SIGCHLD 信号处理

在主函数中添加：

```go
func main() {
    // 忽略 SIGCHLD 信号，让内核自动回收僵尸进程
    signal.Ignore(syscall.SIGCHLD)
    
    // ... 现有代码 ...
}
```

## 修改文件

1. `cmd/lh/main.go` - 添加 WaitGroup 和信号处理
2. `internal/gateway/gateway.go` - 添加 WaitGroup 等待
3. `internal/gateway/telegram/telegram.go` - 确保正确停止
4. `internal/gateway/onebot/onebot.go` - 确保正确停止

## 测试

1. 启动网关：`lh msg-gateway start --platform telegram --token TOKEN`
2. 运行 5 分钟
3. 停止网关：Ctrl+C
4. 检查僵尸进程：`ps aux | grep "<defunct>"`
5. 预期结果：无僵尸进程

## 影响

- ✅ 修复僵尸进程问题
- ✅ 改进资源清理
- ✅ 更优雅的关闭流程
- ⚠️ 停止网关时可能稍慢（等待 goroutine 完成）
