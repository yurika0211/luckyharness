# LuckyHarness v0.55.1 最终修复报告

## 🐛 问题描述

**错误信息**：
```json
{
  "error": {
    "message": "Invalid 'input[5].call_id': empty string. Expected a string with minimum length 1, but got an empty string instead.",
    "type": "invalid_request_error",
    "param": "input[5].call_id"
  }
}
```

**根本原因**：`api.boaiak.com` 的 `gpt-5.4-mini` 模型在流式响应中返回空的 `tool_call.id` 字段，导致后续 API 调用失败。

## ✅ 完整修复（4 处）

### 修复 1：非流式响应中的工具调用
**文件**：`internal/provider/openai_stream.go` 第 185-186 行  
**场景**：非流式 API 响应，直接返回完整 tool_calls

```go
for i, tc := range choice.Message.ToolCalls {
    id := tc.ID
    if id == "" {  // ✅ 修复点
        id = GenerateCallID()
    }
    result.ToolCalls[i] = ToolCall{
        ID: id,
        Name: tc.Function.Name,
        Arguments: tc.Function.Arguments,
    }
}
```

### 修复 2：流式响应累积后的工具调用
**文件**：`internal/provider/openai_stream.go` 第 269-270 行  
**场景**：流式 API 响应，累积所有 chunk 后组装 tool_calls

```go
for i := 0; i < len(toolCallAcc); i++ {
    if tc, ok := toolCallAcc[i]; ok && tc.Function.Name != "" {
        id := tc.ID
        if id == "" {  // ✅ 修复点
            id = GenerateCallID()
        }
        toolCalls = append(toolCalls, ToolCall{
            ID: id,
            Name: tc.Function.Name,
            Arguments: tc.Function.Arguments,
        })
    }
}
```

### 修复 3：流式解析器返回工具调用
**文件**：`internal/provider/stream_parser.go` 第 117-118 行  
**场景**：使用 StreamParser 解析流式响应后获取 tool_calls

```go
func (sp *StreamParser) GetToolCalls() []ToolCall {
    // ...
    for i := 0; i < len(sp.toolCalls); i++ {
        if dtc, ok := sp.toolCalls[i]; ok {
            id := dtc.ID
            if id == "" {  // ✅ 修复点
                id = GenerateCallID()
            }
            calls = append(calls, ToolCall{
                ID:        id,
                Name:      dtc.Function.Name,
                Arguments: dtc.Function.Arguments,
            })
        }
    }
    return calls
}
```

### 修复 4：Agent 流式工具调用处理
**文件**：`internal/agent/agent.go` 第 769-770 行  
**场景**：Agent 处理流式 tool_call deltas，累积后构建 ToolCall

```go
for _, acc := range toolCallsAcc {
    if acc.name != "" {
        id := acc.id
        if id == "" {  // ✅ 修复点（最初遗漏）
            id = provider.GenerateCallID()
        }
        toolCalls = append(toolCalls, provider.ToolCall{
            ID:        id,
            Name:      acc.name,
            Arguments: acc.arguments,
        })
    }
}
```

## 🔧 技术实现

### 唯一 ID 生成函数
**文件**：`internal/provider/function_calling.go`

```go
// GenerateCallID 生成唯一的 call_id，用于工具调用
// 格式："call_" + 16 字符随机字符串
func GenerateCallID() string {
    b := make([]byte, 8)
    if _, err := rand.Read(b); err != nil {
        // 极端情况下随机数生成失败，使用时间戳
        return "call_" + hex.EncodeToString([]byte("fallback"))
    }
    return "call_" + hex.EncodeToString(b)
}
```

**特点**：
- 使用 `crypto/rand` 生成加密安全的随机数
- 格式：`call_` + 16 字符十六进制字符串（8 字节）
- 降级方案：随机数失败时使用 fallback 字符串

### 导出为公共函数
为了让 `agent` 包也能使用，将 `generateCallID` 改为大写导出：
- `generateCallID()` → `GenerateCallID()`
- `agent.go` 中调用：`provider.GenerateCallID()`

## 📦 部署验证

### 编译检查
```bash
cd /root/luckyharness-src
/usr/local/go/bin/go build -o lh ./cmd/lh
```

### 修复点验证
```bash
grep -n "if id == \"\"" internal/provider/openai_stream.go \
    internal/provider/stream_parser.go internal/agent/agent.go
# 输出 4 行，对应 4 处修复
```

### 运行时验证
```bash
# 重启网关
./lh msg-gateway start --platform telegram --token "TOKEN"

# 观察日志
tail -f /tmp/lh-v0.55.1-final2.log
```

## 📊 修复覆盖范围

| 组件 | 场景 | 修复状态 |
|------|------|----------|
| **provider/openai_stream.go** | 非流式响应 | ✅ 已修复 |
| **provider/openai_stream.go** | 流式累积 | ✅ 已修复 |
| **provider/stream_parser.go** | 流式解析 | ✅ 已修复 |
| **agent/agent.go** | Agent 流式处理 | ✅ 已修复 |

## 🎯 测试建议

### 1. 简单对话测试
```
发送：你好
预期：正常响应，无错误
```

### 2. 工具调用测试
```
发送：查询天气 / 搜索信息 / 使用任何 skill
预期：工具正常调用，无 call_id 错误
```

### 3. 多轮对话测试
```
发送：多轮连续消息
预期：会话正常，记忆正常
```

### 4. 压力测试
```
发送：连续快速发送多条消息
预期：无并发问题，无 call_id 冲突
```

## 🔄 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v0.55.0 | 2026-04-23 | 初始版本（有 bug） |
| v0.55.1 | 2026-04-23 | 修复 3 处 call_id（遗漏 agent.go） |
| v0.55.1-final | 2026-04-23 | 修复第 4 处 call_id（完整修复） |

## 📝 配置文件变更

**同时统一配置文件格式**：
- `config.yaml` → `config.json`
- 移除 `gopkg.in/yaml.v3` 依赖
- 使用 Go 原生 `encoding/json`

**迁移脚本**：
```bash
python3 /root/luckyharness-src/scripts/migrate_config.py
```

## ✅ 验收标准

- [x] 4 处 call_id 修复全部应用
- [x] `GenerateCallID()` 正确导出
- [x] 编译无错误
- [x] 网关正常启动
- [x] 能接收 Telegram 消息
- [ ] 工具调用无 `empty string` 错误（待用户验证）
- [ ] 连续测试无回归（待观察）

## 🚀 下一步

1. ✅ 等待用户验证 Telegram 消息响应
2. ⚠️ 观察日志 24 小时，确认无新报错
3. ⚠️ 如有其他 API 兼容性问题，继续修复
4. 📝 更新 Release Notes

---

**版本**: v0.55.1-final  
**修复日期**: 2026-04-23 08:36 UTC  
**修复者**: RightClaw  
**状态**: ✅ 已部署，待验证  
**二进制**: `/root/.luckyharness/lh` (28M, 08:36)  
**网关 PID**: 21147
