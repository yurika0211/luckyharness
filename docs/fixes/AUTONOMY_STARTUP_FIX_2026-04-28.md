# Autonomy 启动链路修复记录

日期: 2026-04-28

## 1. 背景

本次问题的核心不是“任务内容无法处理”，而是 `autonomy` 这条任务执行链没有真正启动，导致：

- 任务没有进入可执行队列
- 调度器没有开始工作
- worker 没有被拉起
- heartbeat 没有产生有效时间戳

用户侧观测到的典型状态是：

- `started = false`
- `ready / in_progress / blocked / done = 0`
- `worker_count = 0`
- `last_heartbeat = 0001-01-01T00:00:00Z`

这组状态说明的不是“任务失败”，而是“任务系统未启动，因此没有任何任务被执行”。

## 2. 排查目标

本次排查围绕三件事展开：

1. 任务是否真的被放进了 `autonomy` 队列
2. 调度器是否真正启动
3. worker 是否真实存在并处于运行状态

## 3. 排查过程

### 3.1 先定位状态来源

首先在代码中全局搜索以下关键字：

- `last_heartbeat`
- `worker_count`
- `started`
- `ready`
- `in_progress`
- `blocked`
- `done`
- `scheduler`
- `worker`

定位结果表明，相关实现集中在 `internal/autonomy`：

- `internal/autonomy/queue.go`
- `internal/autonomy/worker.go`
- `internal/autonomy/heartbeat.go`

这说明用户看到的状态并不是 `cron` 的状态，而是 `autonomy` 自主任务系统的状态。

### 3.2 确认队列和 worker 的启动条件

继续检查 `TaskQueue`、`WorkerPool` 和 `HeartbeatEngine` 的实现：

- `TaskQueue` 是内存队列，任务存在于进程内 map 中，不具备持久化能力
- `WorkerPool` 只有在 `Start(ctx)` 被调用后才会创建最小 worker 集合
- `HeartbeatEngine` 只有在 `Start(ctx)` 后才会进入 heartbeat loop

结论：

- `worker_count = 0` 的直接原因是 `WorkerPool.Start()` 没有被调用
- `last_heartbeat = 零值时间` 的直接原因是 `HeartbeatEngine.Start()` 没有被调用，或者启动后从未触发首拍

### 3.3 检查 AutonomyKit 是否真的被启动

在 `internal/autonomy/heartbeat.go` 中可以看到：

- `AutonomyKit.Start(ctx)` 会启动 `WorkerPool`
- `AutonomyKit.Start(ctx)` 会启动 `HeartbeatEngine`
- `AutonomyKit.Status()` 返回用户看到的整体状态

随后全局搜索 `StartAutonomy(` 的调用点。

结果：

- 存在 `Agent.StartAutonomy(ctx)` 方法
- 但在 CLI、server、gateway 等真实入口中，没有找到自动调用它的路径

这意味着：

- `autonomy` 工具虽然已经注册到 agent
- 但执行基础设施默认没有启动

### 3.4 对比 cron 和 autonomy 的差异

检查 `internal/agent/agent.go` 后发现：

- `cronEngine.Start()` 会在 `agent.New()` 中自动执行
- `autonomy` 仅在 `agent.New()` 中被创建和注册工具
- `autonomy` 默认不会在 `agent.New()` 中启动

所以系统呈现出了一个非常容易误导人的状态：

- `cron` 是运行的
- `autonomy` 是存在但未启动的

如果仅看“系统有调度能力”，容易误判为调度器异常；实际上是两条不同的执行链。

### 3.5 发现第二个根因：实例错位

更深入看 `Agent.StartAutonomy(ctx)` 的实现，发现存在更隐蔽的问题：

- `agent.New()` 时先创建了一份 `autonomyKit`
- `autonomy` 工具注册时绑定的是这第一份实例
- `StartAutonomy(ctx)` 调用时又重新 `NewAutonomyKit(...)`
- `a.autonomy` 被替换成了新实例

结果是：

- 新实例被启动
- 老实例仍然被工具引用
- `autonomy_status`、`autonomy_queue_add`、`autonomy_worker_list` 这类工具可能继续访问旧实例

这会导致一种更糟糕的状态：

- 启动代码看起来执行了
- 但工具查询仍然返回“未启动、无 worker、无任务”

也就是说，用户观察到的现象不只是“没有启动”，还有“即使启动也可能查错对象”。

## 4. 根因总结

本次问题最终确认有三个根因。

### 根因一：Autonomy 默认不启动

`autonomy` 只完成了初始化和工具注册，没有在实际运行入口自动拉起。

影响：

- `started = false`
- `worker_count = 0`
- 队列永远不消费

### 根因二：StartAutonomy 会替换实例

`StartAutonomy()` 不是启动现有实例，而是重新创建一份新的 `AutonomyKit`。

影响：

- 启动实例和工具引用实例不一致
- 状态查询会落在旧对象上
- 用户看到“系统仍未启动”

### 根因三：heartbeat 启动后没有首拍

即使 heartbeat loop 已经启动，如果还没等到第一个周期，`last_heartbeat` 仍然是零值。

影响：

- 状态会继续看起来像“完全未启动”
- 运维判断容易被误导

## 5. 修复目标

修复不是简单把某个布尔值改成 `true`，而是要让整条执行链真的可用。

本次修复目标包括：

1. 保证工具和运行时使用同一份 `AutonomyKit`
2. 保证 `RunLoop` 首次进入时能够自动拉起 autonomy
3. 保证已有 worker 在后续注入 executor 后也能正常工作
4. 保证 `last_heartbeat` 在启动后立即可观测
5. 保证测试或短生命周期场景关闭时能回收后台 goroutine

## 6. 实施方案

### 6.1 不再重建 AutonomyKit

修改 `Agent.StartAutonomy(ctx)`：

- 不再调用 `autonomy.NewAutonomyKit(...)` 重建实例
- 改为对现有 `a.autonomy` 注入 executor
- 如果已经启动，则直接返回
- 如果未启动，则启动现有实例

这一步修复了“工具绑定旧实例，启动发生在新实例”的对象错位问题。

### 6.2 在 agent 构造完成后注入 executor

在 `agent.New()` 的尾部增加 executor 注入：

- `a.autonomy.SetExecutor(&agentExecutorAdapter{agent: a})`

这样可以保证：

- 工具绑定的就是后续真正要使用的那一份 `AutonomyKit`
- 不需要靠重建实例来补 executor

### 6.3 WorkerPool 支持热更新 executor

为 `AutonomyKit` 增加 `SetExecutor()`，向下透传到 `WorkerPool.SetExecutor()`。

同时增强 `WorkerPool.SetExecutor()`：

- 更新 pool 级别的 `executor`
- 同步更新已存在 worker 的 `Executor`
- 如果 worker 之前还没有 session，则补建 `SessionID`

这一步解决了“worker 先创建、executor 后注入”时的失配问题。

### 6.4 在 RunLoop 入口做 lazy start

在 `internal/agent/loop.go` 的 `RunLoopWithSession(...)` 开头增加：

- 首次进入 loop 时调用 `a.StartAutonomy(ctx)`

这样做的原因是：

- 不需要在 `agent.New()` 阶段无条件常驻启动所有后台协程
- 避免测试里每创建一个 agent 就长期挂着 worker 和 heartbeat
- 在真正进入可执行阶段时再启动 autonomy，更符合实际使用路径

### 6.5 让 autonomy 作为进程级后台组件运行

虽然 `RunLoop` 会传入一个请求级 `ctx`，但 autonomy 本质上是后台基础设施，而不是单次请求生命周期内的临时对象。

因此在 `StartAutonomy()` 中：

- 不把单次请求的 `ctx` 直接传给 `autonomy.Start(...)`
- 改为使用进程级 `context.Background()`

原因：

- 如果把 HTTP 请求、Telegram 消息或一次 CLI 调用的 `ctx` 直接传进去
- 请求一结束，worker pool 和 heartbeat loop 就会被自动取消
- 但状态未必立刻反映为“已停止”

这会形成新的“看似启动，实际已死”的假象。

### 6.6 heartbeat 启动即首拍

修改 `HeartbeatEngine.Start(ctx)`：

- 在进入 `go h.loop(ctx)` 之前，先执行一次 `h.beat(ctx)`

这样做的效果是：

- `last_heartbeat` 在启动后立即有效
- `autonomy_status` 不再用零值时间误导判断

### 6.7 Close 时回收后台资源

增强 `Agent.Close()`：

- 如果 autonomy 已启动，则停止 autonomy
- 停止 cron engine
- 保留原有的 RAG/SQLite 收尾逻辑

这一步的目标不是修复用户线上问题，而是避免：

- 测试中创建多个 agent 后后台 goroutine 累积
- 局部场景下资源无法及时释放

## 7. 代码变更清单

本次实际修改了以下文件：

- `internal/agent/agent.go`
- `internal/agent/loop.go`
- `internal/agent/agent_test.go`
- `internal/autonomy/heartbeat.go`
- `internal/autonomy/worker.go`

变更类别如下：

- 启动链路修复
- 实例一致性修复
- worker executor 注入修复
- heartbeat 可观测性修复
- 生命周期回收修复
- 回归测试补充

## 8. 测试与验证

本次定向验证通过的命令：

```bash
go test ./internal/agent -run 'TestAgentStartAutonomy|TestRunLoopWithSessionLazyStartsAutonomy|TestAgent_StartAutonomy_Nil' -v
go test ./internal/autonomy -run 'TestAutonomyKitStartStop|TestAutonomyKitStatus|TestWorkerPoolSetExecutor' -v
```

验证覆盖点：

- `StartAutonomy()` 幂等
- `StartAutonomy()` 不替换原实例
- 启动后至少有一个 worker
- 启动后 `last_heartbeat` 不再是零值
- `RunLoop()` 首次进入会 lazy start autonomy
- `WorkerPool.SetExecutor()` 能更新已有 worker

### 额外观察

完整执行 `go test ./internal/agent` 时没有作为本次修复完成条件，因为该包内仍有已有测试会走真实 provider 网络路径，可能被网络或超时拖住。这不是本次 autonomy 启动链修复引入的新问题。

换句话说：

- 本次修复已经通过定向回归验证
- 但仓库里仍存在与外部网络耦合的旧测试问题，需单独治理

## 9. 修复后的行为预期

修复完成后，预期行为如下：

1. agent 初始化后，autonomy 工具与运行时引用同一份实例
2. 首次进入 `RunLoop` 时，autonomy 会被自动拉起
3. worker pool 会创建最小 worker 数量
4. heartbeat 会在启动瞬间产生首次 beat
5. `autonomy_status` 能返回真实、即时、可解释的状态

理想情况下，修复后首次实际使用时应看到类似状态：

- `started = true`
- `worker_count >= 1`
- `last_heartbeat != zero`

如果任务已入队，还应看到：

- `ready > 0` 或
- `in_progress > 0`

而不是长期停留在全零态。

## 10. 本次修复解决了什么，没解决什么

### 已解决

- autonomy 默认不启动
- 启动实例与工具实例错位
- worker 在后注入 executor 时不可用
- heartbeat 启动后长时间显示零值
- 局部场景下后台组件不回收

### 未解决

- `TaskQueue` 仍然是内存队列，不具备持久化能力
- 进程重启后 autonomy 队列状态仍会丢失
- 全量测试中的网络依赖问题仍在

## 11. 后续建议

如果要把这套链路从“能启动”继续提升到“稳定可运营”，建议按顺序做下面三件事：

1. 给 `TaskQueue` 增加持久化层，至少支持重启恢复
2. 增加显式的 autonomy 管理入口，例如状态查看、启动、停止、重载
3. 清理会访问真实网络的测试，将其改为 mock 或 integration test 隔离执行

## 12. 结论

这次问题的本质不是任务执行失败，而是 autonomy 基础设施没有真正接线。

更准确地说，问题分成两层：

- 第一层：默认没启动
- 第二层：即使启动，原实现也可能查错实例

本次修复已经把这两层问题同时处理掉，并补上了 heartbeat 首拍和生命周期回收，使得：

- `autonomy_status` 能反映真实状态
- `RunLoop` 会自动拉起 autonomy
- worker 和 heartbeat 会真正进入运行态

因此，这次修复属于“执行基础设施接通”，不是“单个任务修复”。
