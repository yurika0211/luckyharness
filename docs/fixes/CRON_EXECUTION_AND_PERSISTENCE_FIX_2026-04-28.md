# LH 定时任务执行与持久化修复记录

日期: 2026-04-28

## 1. 问题背景

在排查 `lh` 的任务系统时，发现用户对“定时任务”的预期和当前实现之间存在明显落差。

用户期待的是：

- `/cron add ...` 后，任务能按计划真正执行
- 任务重启后仍然存在
- 支持中文时间表达和 cron 表达式
- 能用于 shell 命令，或者直接触发一次 agent 任务

但当时系统的实际状态是：

1. `/cron add` 只是把任务注册进内存里的 `cron.Engine`
2. 到点后任务函数只是 `fmt.Printf(...)`
3. REPL 重启后所有任务丢失
4. REPL 里的 `cronEngine` 默认未启动，除非用户手动 `/cron start`
5. 带空格的 5 段 cron 表达式解析不稳定

这意味着“调度框架存在”，但“用户可用性不成立”。

## 2. 现象确认

首先确认定时任务相关代码入口。

REPL 初始化位置在：

- `internal/cli/lhcmd/chat_repl.go`

可以看到：

- REPL 自己创建了一个 `cron.NewEngine()`
- 同时还创建了 `cron.NewWatcher(...)`
- 但没有在初始化时恢复任何任务

`/cron` 命令处理逻辑位于：

- `handleCronCommand(...)`

初始实现中，`add` 子命令对应的任务函数只是：

```go
task := func() error {
    fmt.Printf("\n⏰ [cron:%s] %s\n", id, command)
    return nil
}
```

这说明：

- 定时任务到点只打印命令字符串
- 不执行 shell
- 不调用 agent
- 不持久化

## 3. 原始架构梳理

在决定怎么修之前，先把 `cron` 这套原生逻辑梳理清楚。

### 3.1 Cron Engine

`internal/cron/engine.go` 提供了核心调度引擎：

- `AddJob(...)`
- `RemoveJob(...)`
- `PauseJob(...)`
- `ResumeJob(...)`
- `Start()`
- `Stop()`
- `ListJobs()`

每个 `Job` 包含：

- `ID`
- `Name`
- `Description`
- `Schedule`
- `Task func() error`
- `Status`
- `LastRun`
- `NextRun`
- `RunCount`
- `Metadata`

调度逻辑是：

1. 引擎启动后进入循环
2. 每分钟 tick 一次
3. 遍历所有 job
4. 如果 `now >= job.NextRun` 且状态允许执行
5. 运行 `job.Task()`
6. 根据 `Schedule.Next(...)` 计算下一次运行时间

结论：

- `cron.Engine` 本身是能工作的
- 问题不在调度器，而在 REPL 上层没有把它接成“可执行产品能力”

### 3.2 调度表达式

当前支持的调度类型有四类：

- `IntervalSchedule`
- `DailySchedule`
- `CronSchedule`
- `OnceSchedule`

此外还支持自然语言解析：

- `每天9点`
- `每小时`
- `每30分钟`
- `每周一9点`
- `工作日9点`
- `明天10点`
- `2026-06-01 12:00`

结论：

- 底层 schedule 能力已具备
- 真正缺的是 REPL 层如何把用户命令转换为长期可运行的任务定义

## 4. 发现的具体问题

这次实际确认了 5 个问题。

### 问题一：任务只打印，不执行

这是最直观的问题。

`/cron add` 表面上接受了 `<command>`，但内部任务函数只会打印：

- `⏰ [cron:id] command`

根本没有调用：

- shell
- agent loop
- 任何业务执行器

所以它更像“提醒器”而不是“定时任务执行器”。

### 问题二：任务不持久化

REPL 的 cron 任务完全保存在内存：

- 退出 REPL 后丢失
- 重启后不会恢复

这使得 cron 在真实使用场景中几乎不可用。

### 问题三：引擎默认不启动

REPL 会创建 `cronEngine`，但不会自动启动。

结果是：

- 用户 add 了任务
- 任务确实进入了引擎
- 但如果没有额外执行 `/cron start`
- 它永远不会跑

这属于典型的“功能存在，但默认路径不可用”。

### 问题四：5 段 cron 表达式解析容易吞命令

用户输入例如：

```text
/cron add raw-cron 0 9 * * * echo hello
```

如果 REPL 只靠简单 `strings.Fields()` + 固定下标处理，会碰到：

- schedule 和 command 都是空格分隔
- 解析器可能把 `echo` 吞进 schedule
- 最终 command 被截断或错位

这会让合法的 cron 表达式无法稳定使用。

### 问题五：调度模式单一

如果只支持 shell，cron 能力仍然不够贴合 LuckyHarness 的定位。

因为一个 AI agent 系统里常见的定时任务其实有两类：

1. 定时执行 shell/脚本/命令
2. 定时触发一次 agent 思考任务

例如：

- 每天早上汇总日志
- 工作日生成日报草稿
- 每晚扫描某个目录并出结论

所以只补 shell 还不够，至少要给出 agent 触发入口。

## 5. 修复目标

这次修复目标明确分成两层：

### 第一层：让 `/cron add` 真正可执行

- 默认执行 shell
- 支持触发 agent 任务
- 添加后调度器自动启动

### 第二层：让任务可持续存在

- 本地持久化
- REPL 启动时自动恢复
- 保存暂停状态
- 保存引擎运行状态

## 6. 设计决策

### 6.1 不把 cron 数据塞进主配置文件

可以把 cron 任务塞进 `config.json`，但这不是最合适的选择。

原因：

- config 属于静态配置
- cron jobs 属于动态运行时对象
- 二者生命周期不同
- 任务列表频繁变化，不应该污染主配置

因此最终选择：

- 新增独立文件 `~/.luckyharness/cron_jobs.json`

优点：

- 更干净
- 更容易演进
- 不影响已有配置结构

### 6.2 REPL 层做持久化，而不是改动 Cron Engine

`cron.Engine` 当前只关心运行，不关心存储。

这层分离是合理的：

- Engine = 调度与执行
- REPL = 用户输入与状态恢复

因此没有把持久化硬塞进 `internal/cron`，而是在 `internal/cli/lhcmd` 加了一个薄层：

- `cronStore`

这样可以保持底层 cron 包足够纯净。

### 6.3 任务模式最小可用先做两种

最终支持两类任务模式：

1. `shell`
2. `agent`

规则：

- 默认无前缀时按 `shell`
- `shell:...` 显式指定 shell
- `agent:...` 或 `prompt:...` 触发 agent 任务

这样对用户来说：

- 默认体验简单
- 又能覆盖 AI agent 场景

## 7. 实施过程

### 7.1 修复 `/cron add` 的解析逻辑

新增了 `cronAddSpec`，用于统一描述一条待添加的任务：

- `ID`
- `Schedule`
- `ScheduleText`
- `Command`
- `Mode`
- `Payload`

并实现了：

- `parseCronAddSpec(...)`
- `parseCronTaskCommand(...)`

解析逻辑现在支持：

- 单 token 自然语言 schedule
- 双 token 自然语言/日期时间 schedule
- 5 段 cron 表达式

同时修掉了：

- `每小时 echo hello` 被误判成双 token schedule
- `0 9 * * * echo hello` 吞掉 command 的问题

### 7.2 把占位任务改成真实执行任务

新增：

- `buildCronTask(...)`

根据 mode 生成不同执行器。

#### shell 模式

通过 `a.Gateway().Execute("shell", ...)` 执行 shell 工具：

- 复用现有 shell 工具能力
- 不重新造执行器
- 保持 LuckyHarness 内部行为一致

执行结果会直接输出到 REPL。

#### agent 模式

通过：

- `a.Sessions().NewWithTitle("cron-"+id)`
- `a.RunLoopWithSession(...)`

触发一次独立 agent 任务。

这样可以直接支持类似：

```text
/cron add report 每天9点 agent:总结昨天日志并输出今日计划
```

### 7.3 添加任务后自动启动引擎

此前用户必须显式执行：

```text
/cron start
```

现在改成：

- `/cron add` 成功后，如果引擎未运行，则自动 `engine.Start()`

这样可以避免：

- “add 成功但任务不跑”的误导状态

### 7.4 增加本地持久化层

新增文件：

- `internal/cli/lhcmd/cron_persist.go`

实现：

- `newCronStore(homeDir)`
- `Save(engine)`
- `Load(engine, a, loopCfg)`

持久化文件格式为：

- `~/.luckyharness/cron_jobs.json`

保存内容包括：

- 版本号
- 引擎是否运行
- 任务列表
- 每个任务的：
  - `id`
  - `schedule_text`
  - `command`
  - `mode`
  - `paused`

### 7.5 REPL 启动时自动恢复任务

在 `startREPL(...)` 里增加：

1. 构建 `cronStore`
2. 读取当前 `loopCfg`
3. 调用 `cronStore.Load(...)`
4. 如果存在持久化任务，自动恢复并打印恢复数量

这样 REPL 每次启动时都会尝试恢复历史任务。

### 7.6 在状态变更后自动保存

为了保证状态一致性，在以下操作后增加自动保存：

- `/cron add`
- `/cron remove`
- `/cron pause`
- `/cron resume`
- `/cron start`
- `/cron stop`

这样用户不需要再手动执行任何保存动作。

## 8. 最终行为

修复完成后，REPL 里的 cron 行为变为：

### 添加 shell 任务

```text
/cron add cleanup 每天2点 shell:find tmp -type f -mtime +3 -delete
```

或省略前缀：

```text
/cron add cleanup 每天2点 find tmp -type f -mtime +3 -delete
```

### 添加 agent 任务

```text
/cron add daily-plan 工作日9点 agent:总结昨日进展并生成今天计划
```

### 添加原始 cron 表达式

```text
/cron add raw 0 9 * * * echo hello
```

### 持久化行为

- add 后自动保存
- stop/start 状态会保存
- pause/resume 状态会保存
- REPL 重启后自动恢复

## 9. 测试与验证

本次新增并通过了以下定向测试：

```bash
go test ./internal/cli/lhcmd -run 'TestParseCronAddSpecSupportsFiveFieldCron|TestHandleCronCommandAddsExecutableShellJob|TestParseCronTaskCommandAgentPrefix|TestCronStoreSaveAndLoad' -v
```

覆盖内容：

### `TestParseCronAddSpecSupportsFiveFieldCron`

验证：

- `0 9 * * * echo hello` 可以正确解析
- 不会把 command 吞掉

### `TestHandleCronCommandAddsExecutableShellJob`

验证：

- `/cron add` 会自动启动引擎
- shell 任务会真正执行

### `TestParseCronTaskCommandAgentPrefix`

验证：

- `agent:` 前缀能正确解析为 agent 模式

### `TestCronStoreSaveAndLoad`

验证：

- cron 状态可保存到本地文件
- 任务可从持久化文件恢复
- pause 状态可恢复
- engine running 状态可恢复

## 10. 修改文件

本次实际修改/新增文件：

- `internal/cli/lhcmd/chat_repl.go`
- `internal/cli/lhcmd/chat_repl_test.go`
- `internal/cli/lhcmd/cron_persist.go`

## 11. 当前仍然存在的边界

虽然现在已经进入“可实际使用”的状态，但还有几个边界没有做：

### 11.1 只在 REPL 层持久化

当前持久化逻辑只接入了 `startREPL(...)`。

这意味着：

- REPL 使用 cron 有持久化
- 其他入口如果以后也暴露 cron 功能，还需要单独接入

### 11.2 没有失败重试策略

当前 job 失败后：

- 状态会记为 failed
- 下次调度时间仍按 schedule.Next 继续推进

没有做：

- 指数退避
- 最大重试次数
- 告警升级

### 11.3 没有并发隔离策略

目前 job 到点后直接执行：

- shell 模式会跑命令
- agent 模式会进 RunLoop

如果未来 job 数量很多，可能需要：

- worker 池
- 并发限制
- 执行队列

### 11.4 Watcher 仍然是独立组件

本次只修了时间调度 cron，不包括 watcher 的持久化。

如果需要完整“自动化中心”，后续可以把：

- cron jobs
- watch patterns

统一纳入一个持久化层。

## 12. 后续建议

如果继续往下做，优先级建议如下：

1. 给 watcher 也加持久化
2. 给 agent 任务增加结果落盘/通知机制
3. 给 cron 增加失败重试和告警策略
4. 考虑把 cron 功能提升为正式 CLI 子命令，而不是仅限 REPL 的 `/cron`

## 13. 结论

这次问题的本质不是 cron 引擎本身坏了，而是：

- REPL 层只接了一个演示级占位实现
- 没有执行器
- 没有持久化
- 默认还不自动启动

本次修复把它补到了“真实可用”的程度：

- 能执行
- 能恢复
- 能持续存在
- 能支持 shell 和 agent 两种任务模式

因此，这次修复属于：

- 从“框架能力存在”补到“产品行为成立”

而不是简单的 bugfix。  
