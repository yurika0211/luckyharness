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

### v0.55.0: Agent 包测试补全 (21.7%→60%+)

**状态**: Ready  
**优先级**: Medium

---

### v0.56.0: gRPC API 包测试补全 (7.1%→60%+)

**状态**: Ready  
**优先级**: Low

---

## 历史版本

- [x] v0.52.0: 全仓库覆盖率里程碑 60%+ (2026-04-22 完成)
- [x] v0.51.0: WebSocket 包测试补全 (2026-04-22 完成)
- [x] v0.50.0: Session 包测试补全 (2026-04-22 完成)
- [x] v0.49.0: Search 包测试补全 (2026-04-22 完成)
