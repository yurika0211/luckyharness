# Cron Tool 与共享持久化打通记录

日期: 2026-04-28

## 1. 背景

在前一轮修复中，已经完成了两件基础工作：

1. `lh` REPL 的 `/cron add ...` 不再只是打印占位消息，而是可以真正执行任务
2. REPL 层的 cron 任务已经具备本地持久化和自动恢复能力

但随后暴露出一个新的结构性问题：

- **bot 自己仍然不会设置定时任务**
- 就算用户通过对话提出“每天 9:00 提醒我”，模型也不会主动使用 cron
- 即使我们后来把 cron 暴露成 Agent tool，如果持久化仍只在 REPL 层，就会形成两套状态：
  - REPL 一套
  - Agent 一套

因此，这次问题不再是“cron 能不能跑”，而是：

1. **bot 能不能调用 cron**
2. **bot 设的 cron 能不能跨重启保留**
3. **REPL 和 Agent 是否共享同一套任务状态**

## 2. 初始状态分析

### 2.1 bot 为什么“不知道”能这样设置定时任务

根因并不是简单的“知识库没写”，而是这条能力最初根本不在 Agent 可调用工具集合里。

LuckyHarness 的模型工具调用能力依赖于：

- `a.tools`
- `Function Calling` 工具定义
- `system prompt` 中暴露给模型的可用能力

而 `/cron` 原本只是 REPL 外层命令：

- 在 `internal/cli/lhcmd/chat_repl.go` 里由 `/cron` 分支拦截
- 不属于 Agent tool
- 不会进入 `a.tools`

结论：

- 用户在终端里能 `/cron add ...`
- 但 bot 在对话里没有对应能力入口
- 它就算“知道要用定时任务”，也没有办法真正调用

### 2.2 持久化为什么也不统一

上一轮做的 `cron_jobs.json` 持久化最初是挂在 REPL 层的：

- REPL 启动时恢复
- `/cron add/remove/pause/resume/...` 时保存

这能解决 REPL 可用性问题，但会留下更深的问题：

- Agent 内部有自己的 `cronEngine`
- REPL 也曾自己创建一套 `cronEngine`
- 如果 bot 新增了 cron tool，但还沿用另一套引擎或另一套持久化层
- 就会出现：
  - bot 添加的任务不一定出现在 REPL
  - REPL 恢复的任务不一定出现在 bot
  - 同一进程可能重复恢复两遍

所以这次必须把“工具能力”和“持久化能力”同时统一。

## 3. 修复目标

本次目标明确分成三层：

### 目标一：把 cron 提升成 Agent tool

至少要支持：

- `cron_add`
- `cron_list`
- `cron_remove`
- `cron_pause`
- `cron_resume`
- `cron_status`

### 目标二：bot 设的任务自动持久化

即：

- 通过 `cron_add` 新增任务
- 自动写入 `cron_jobs.json`
- 进程重启后恢复

### 目标三：REPL 和 Agent 共享同一个 cron 引擎与同一个持久化文件

这一步的目标是消除“双系统”。

## 4. 设计决策

### 4.1 不复用 REPL 的字符串命令解析作为 Agent tool 协议

REPL 里的 `/cron add ...` 是面向人类输入的字符串命令。

但 Agent tool 更适合结构化参数：

- `id`
- `schedule`
- `mode`
- `command`

因此没有直接让 Agent 去拼 `/cron add ...`，而是增加了独立的结构化 cron tool。

这样做的好处：

- 更适合函数调用
- 更容易验证参数合法性
- 更容易做类型约束

### 4.2 共享持久化层下沉到 `internal/cron`

上一轮的持久化是放在 REPL 包里的，这次不够用了。

因为现在持久化既要给 REPL 用，也要给 Agent 用。

因此把共享持久化层下沉到：

- `internal/cron/store.go`

让它成为一个通用的 cron store。

这样可以做到：

- REPL 可用
- Agent 可用
- 以后 server/gateway 如果也要接 cron，也能直接复用

### 4.3 REPL 必须复用 Agent 的 `cronEngine`

如果 REPL 继续自己 new 一份引擎，就会产生两个问题：

1. 状态不一致
2. 同一持久化文件在同一进程里可能被恢复两遍

所以最终决策是：

- REPL 不再单独创建 cron engine
- 改为直接使用 `a.CronEngine()`

这样 REPL 命令和 bot 工具调用落在同一个运行时对象上。

## 5. 实施过程

## 5.1 新增 Agent cron tools

新增文件：

- `internal/agent/cron_tools.go`

定义并注册了以下工具：

- `cron_add`
- `cron_list`
- `cron_remove`
- `cron_pause`
- `cron_resume`
- `cron_status`

### `cron_add`

支持参数：

- `id`
- `schedule`
- `mode`
- `command`

其中：

- `schedule` 支持中文自然语言和 5 段 cron 表达式
- `mode` 支持 `shell` / `agent`
- `command` 在 `shell` 模式下是 shell 命令，在 `agent` 模式下是 prompt

### `cron_list`

返回当前引擎中的任务列表，包括：

- `id`
- `schedule`
- `status`
- `next_run`
- `last_run`
- `mode`
- `command`

### `cron_remove / cron_pause / cron_resume / cron_status`

用于做完整生命周期管理，而不是只支持添加。

## 5.2 在 `agent.New()` 里注册 cron tools

在 Agent 初始化完成后：

- 调用 `a.registerCronTools()`

这样 cron tools 会进入：

- `a.tools`

进而进入：

- function calling tool definitions

从这一刻开始，模型在对话里就具备了真正使用 cron 的能力，而不是只能“知道这个命令存在”。

## 5.3 给 Agent 增加 cron 持久化能力

在 `Agent` 结构中新增：

- `cronStore *cron.Store`

并在 `agent.New()` 时初始化为：

- `~/.luckyharness/cron_jobs.json`

这一步让 Agent 自己拥有了对 cron 持久化层的直接访问权。

## 5.4 在 Agent 启动时自动恢复 cron

在 `agent.New()` 中：

1. 创建好 `cronEngine`
2. 启动引擎
3. 调用 `restoreCronJobs()`

恢复逻辑基于共享的 `cron.Store.Load(...)`：

- 读取 `cron_jobs.json`
- 为每个任务重建 `Task func() error`
- 恢复 pause 状态
- 恢复引擎运行状态

这样做的结果是：

- 不依赖 REPL
- 只要 Agent 起来，bot 设的任务就会恢复

## 5.5 在 Agent cron tool 状态变更后自动保存

在这些 handler 里增加自动保存：

- `handleCronAdd`
- `handleCronRemove`
- `handleCronPause`
- `handleCronResume`

即：

- 对话里一旦通过 cron tool 改了任务状态
- 就立即写回 `cron_jobs.json`

这样不会出现：

- REPL 的任务会保存
- bot 的任务不保存

## 5.6 把共享持久化层迁移到 `internal/cron/store.go`

新增通用结构：

- `PersistedState`
- `PersistedJob`
- `Store`

并实现：

- `Save(engine)`
- `Load(engine, taskBuilder)`
- `ParsePersistedSchedule(...)`

注意这里的 `Load(...)` 设计成接收 `taskBuilder` 回调，而不是把 task 固定写死在 store 里。

原因：

- cron store 只负责保存/恢复元数据
- 真正的 `Task func() error` 由上层决定
- REPL 和 Agent 都能复用同一份持久化层

这是本次设计里比较关键的一步。

## 5.7 REPL 改为复用 Agent 的 cron engine

修改 `startREPL(...)`：

- 删掉独立的 `cron.NewEngine()`
- 改为 `cronEngine := a.CronEngine()`

同时：

- REPL 不再自己恢复 cron jobs
- 因为 Agent 初始化时已经恢复

这一步完成后：

- `/cron add ...`
- bot 的 `cron_add`

都操作同一套引擎、同一份任务、同一个持久化文件。

## 5.8 删除旧的 REPL 专用持久化实现

由于持久化层已经下沉到共享 cron 包，原先的：

- `internal/cli/lhcmd/cron_persist.go`

不再需要，已删除。

这一步避免了：

- 同一逻辑两份实现
- 后续维护时出现行为漂移

## 6. 修复后的行为

现在 cron 功能分成两种入口，但落在同一套系统上。

### 6.1 REPL 入口

用户可以继续用：

```text
/cron add morning 每天9点 echo hello
/cron add daily-plan 工作日9点 agent:总结昨日进展并生成今日计划
```

这些任务会：

- 落到 `Agent.CronEngine()`
- 自动保存到 `cron_jobs.json`

### 6.2 bot 对话入口

模型现在可以通过 tool 调用：

- `cron_add`
- `cron_list`
- `cron_pause`
- `cron_resume`
- `cron_remove`

也就是说，bot 自己已经具备“设置定时任务”的真实能力，而不是只能口头描述。

### 6.3 重启恢复

不管任务是通过：

- `/cron add`
- `cron_add`

创建的，最终都会写入同一个文件，并在下次 Agent 启动时恢复。

## 7. 测试与验证

本次新增/通过的关键测试包括：

### Agent 侧

```bash
go test ./internal/agent -run 'TestCronToolsLifecycle|TestCronAddAgentModeExecutesLoop|TestCronToolsPersistAcrossAgentRestart' -v
```

覆盖点：

- cron tool 完整生命周期
- `mode=agent` 的任务到点能真正进一次 Agent Loop
- bot 设的任务跨 Agent 重启后仍可恢复

### REPL / CLI 侧

```bash
go test ./internal/cli/lhcmd -run 'TestParseCronAddSpecSupportsFiveFieldCron|TestHandleCronCommandAddsExecutableShellJob|TestParseCronTaskCommandAgentPrefix|TestCronStoreSaveAndLoad' -v
```

覆盖点：

- 5 段 cron 表达式解析
- `/cron add` 的 shell 执行
- `agent:` 前缀解析
- 共享 cron store 的保存/恢复

## 8. 修改文件

本次关键修改/新增文件：

- `internal/agent/cron_tools.go`
- `internal/agent/agent.go`
- `internal/agent/agent_test.go`
- `internal/cron/store.go`
- `internal/cli/lhcmd/chat_repl.go`
- `internal/cli/lhcmd/chat_repl_test.go`

删除文件：

- `internal/cli/lhcmd/cron_persist.go`

## 9. 本次修复解决了什么

### 已解决

- bot 无法直接调用 cron
- bot 设的任务不持久化
- REPL 与 Agent 各自维护独立 cron 状态
- 同一进程潜在的双恢复 / 双状态问题
- cron 功能只能在 REPL 可用，不能在对话中可用

### 尚未解决

- cron 任务仍然是本地单机持久化，不是分布式存储
- watcher 持久化还未统一到同一层
- cron tool 还没有接入外部消息推送通道
- 尚未提供正式 CLI 子命令（目前是 REPL 命令 + Agent tool）

## 10. 后续建议

建议按以下顺序继续推进：

1. 给 watcher 增加共享持久化
2. 给 cron tool 接入通知通道，例如 Telegram / 邮件 / 日历
3. 把 cron 能力从 REPL 命令提升为正式 CLI 子命令
4. 如果未来需要多实例，考虑把 cron store 迁移到 SQLite 或服务端存储

## 11. 结论

这次修复的核心，不是单纯“加几个工具”，而是把 cron 彻底从：

- **REPL 层的人类命令能力**

提升成：

- **Agent 可调用能力**
- **可持久化能力**
- **REPL 与 bot 共享的一致能力**

修复完成后，系统达成了两个关键结果：

1. bot 现在真的“知道并会用”定时任务
2. bot 和 REPL 设定的任务会落在同一套共享持久化系统中

这意味着 cron 功能从“局部可用”进入了“系统级可用”的状态。  
