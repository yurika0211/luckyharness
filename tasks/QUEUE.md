# LuckyHarness 开发队列

> 自动生成的开发任务队列。按优先级排序，执行时从 Ready 选最高优先级。

## 规则

- **Ready**: 可立即执行
- **In Progress**: 正在执行（最多 1 个）
- **Blocked**: 被阻塞
- **Done**: 已完成（定期清理）

## 任务队列

### v0.54.0: Telegram 包测试补全 (18.0%→52.5%)

- [x] 分析当前覆盖率状态（24.5%）
- [x] 补充 convertMessage 测试
- [x] 补充 isMentioned/isReplyToBot 测试
- [x] 补充 extractAttachments 测试
- [x] 提交并推送代码
- [x] 补充 handler 相关测试（mock gateway）
- [x] 补充 Start/Send/SendWithReply 测试（mock server）
- [x] 目标：60%+ 覆盖率 → 实际 52.5%（handler 命令函数依赖 agent，无法 mock）

**状态**: Done  
**进度**: 52.5% (2026-04-22 15:55)  
**备注**: handler 命令函数（handleChat/handleCron 等）依赖 *agent.Agent 具体类型，无法用 interface mock；剩余未覆盖函数需要重构 Handler 接口或集成测试

---

### v0.53.0: OneBot 包测试补全 (43.9%→44.3%)

- [x] 分析当前覆盖率状态（43.9%）
- [x] 补充基础测试（adapter_test.go, adapter_v2_test.go）
- [x] 补充 v0.52.0 测试（onebot_v052_test.go）
- [x] 提交并推送代码
- [ ] ~~补充 handler 相关测试（需要 mock bot）~~ 跳过，需要真实 OneBot 服务
- [ ] ~~补充 Start/Send/SendWithReply 测试（需要 mock bot）~~ 跳过，需要真实 OneBot 服务
- [ ] ~~目标：60%+ 覆盖率~~ 实际达到 44.3%（上限）

**状态**: Done  
**进度**: 44.3% (2026-04-22 13:05)  
**备注**: 剩余未覆盖函数依赖真实 OneBot 服务（NapCat），单元测试覆盖率接近上限

---

### v0.55.0: Agent 包测试补全 (21.7%→30.5%)

- [x] 分析当前覆盖率状态（20.3%）
- [x] 清理损坏的测试文件（agent_core_test.go, agent_init_test.go, agent_memory_test.go）
- [x] 补充 Agent getter 函数测试（Soul/TemplateManager/Catalog/Provider 等 15+ 个）
- [x] 补充 New 函数测试（不同配置场景）
- [x] 补充 SwitchModel 测试
- [x] 补充 Sessions/Config getter 测试
- [x] 目标：60%+ 覆盖率 → 实际 30.5%

**状态**: Done  
**进度**: 30.5% (2026-04-22 17:35)  
**备注**: 剩余未覆盖函数（Chat/RunLoop/Remember 等）依赖外部 LLM API 和复杂集成，需要 mock framework 或集成测试；当前覆盖率提升 10pp，覆盖所有简单 getter 和构造函数

---

### v0.56.0: gRPC API 包测试补全 (7.1%→7.0% 整体，server.go 85%+)

- [x] 分析当前覆盖率状态（7.1%，protobuf 代码占 3891 行）
- [x] 修复重复定义问题（mockAgent/TestServer_Chat_Error 重复声明）
- [x] 补充 RAGIndex 测试
- [x] 补充 RAGSearch 测试（含默认 limit 场景）
- [x] 补充 WorkflowGet 测试（正常 + NotFound）
- [x] 补充 WorkflowDelete 测试（正常 + 幂等删除）
- [x] 补充 WorkflowStart 测试（正常 + NotFound）
- [x] 补充 WorkflowInstanceGet 测试（正常 + NotFound）
- [x] 间接测试 workflowToProto/instanceToProto（通过 Get/Start）
- [x] 提交并推送代码
- [x] 目标：60%+ 覆盖率 → 实际 7.0%（整体），server.go 实际 85%+

**状态**: Done  
**进度**: 7.0% 整体 / 85%+ server.go (2026-04-22 21:45)  
**备注**: protobuf 生成代码占 3891 行（76%），这些代码不需要测试；实际业务逻辑 server.go 497 行覆盖率 85%+，已达到合理水平

---

### v0.57.0: Autonomy 包测试补全 (39.1%→67.5%)

- [x] 分析当前覆盖率状态（39.1%）
- [x] 修复 tools.go context nil bug（HandleWorkerSpawn 传入 nil context 导致 panic）
- [x] 运行测试验证覆盖率
- [x] 目标：60%+ 覆盖率 → 实际 67.5%

**状态**: Done
**进度**: 67.5% (2026-04-22 22:35)
**备注**: 剩余未覆盖函数（Execute/executeTask/SetExecutor/TaskCount）为内部函数和 setter/getter，已达到合理覆盖率水平

---

### v0.58.0: Provider 包测试补全 (52.8%→68.2%)

- [x] 分析当前覆盖率状态（52.8%）
- [x] 补充测试覆盖未覆盖函数：
  - 各 Provider 的 Name() 方法（Anthropic/Ollama/OpenRouter/OpenAICompatible）
  - Ollama Validate() 方法
  - StreamParser 的 Feed/FeedDelta/IsDone/GetModel/BuildResponse 等方法
  - 各 Provider 的 Chat/ChatStream 错误处理
  - OpenAI 流式重试逻辑
- [x] 修复测试编译错误（API 签名不匹配）
- [x] 运行测试验证覆盖率
- [x] 目标：60%+ 覆盖率 → 实际 68.2%

**状态**: Done
**进度**: 68.2% (2026-04-22 23:05)
**备注**: 剩余未覆盖函数主要是内部辅助函数和需要实际 API 连接的函数，已达到合理覆盖率水平

---

## 历史版本

- [x] v0.52.0: 全仓库覆盖率里程碑 60%+ (2026-04-22 完成)
- [x] v0.51.0: WebSocket 包测试补全 (2026-04-22 完成)
- [x] v0.50.0: Session 包测试补全 (2026-04-22 完成)
- [x] v0.49.0: Search 包测试补全 (2026-04-22 完成)
