# LuckyHarness v0.55.1 修复说明

## 🐛 问题描述

API 报错：
```json
{
  "error": {
    "message": "Invalid 'input[5].call_id': empty string. Expected a string with minimum length 1, but got an empty string instead.",
    "type": "invalid_request_error",
    "param": "input[5].call_id"
  }
}
```

**根本原因**：`api.boaiak.com` 的 `gpt-5.4-mini` 模型在流式响应中返回空的 `tool_call.id` 字段。

## ✅ 修复内容

### 1. 配置文件统一为 JSON

**修改**：`internal/config/config.go`
- 从 `config.yaml` 改为 `config.json`
- 移除 `gopkg.in/yaml.v3` 依赖
- 使用 Go 原生 `encoding/json`

**修改**：`internal/debug/debug.go`
- 调试信息收集读取 `config.json`

### 2. 空 call_id 自动修复

**新增函数**：`internal/provider/function_calling.go`
```go
// generateCallID 生成唯一的 call_id，用于工具调用
// 格式："call_" + 16 字符随机字符串
func generateCallID() string {
    b := make([]byte, 8)
    if _, err := rand.Read(b); err != nil {
        return "call_" + hex.EncodeToString([]byte("fallback"))
    }
    return "call_" + hex.EncodeToString(b)
}
```

**修复位置 1**：`internal/provider/openai_stream.go` 第 185 行
```go
// 非流式响应中的工具调用
for i, tc := range choice.Message.ToolCalls {
    id := tc.ID
    if id == "" {
        id = generateCallID()
    }
    result.ToolCalls[i] = ToolCall{
        ID: id,
        Name: tc.Function.Name,
        Arguments: tc.Function.Arguments,
    }
}
```

**修复位置 2**：`internal/provider/openai_stream.go` 第 269 行
```go
// 流式响应累积后的工具调用
for i := 0; i < len(toolCallAcc); i++ {
    if tc, ok := toolCallAcc[i]; ok && tc.Function.Name != "" {
        id := tc.ID
        if id == "" {
            id = generateCallID()
        }
        toolCalls = append(toolCalls, ToolCall{
            ID: id,
            Name: tc.Function.Name,
            Arguments: tc.Function.Arguments,
        })
    }
}
```

**修复位置 3**：`internal/provider/stream_parser.go` 第 117 行
```go
// 流式解析器返回工具调用时
func (sp *StreamParser) GetToolCalls() []ToolCall {
    // ...
    for i := 0; i < len(sp.toolCalls); i++ {
        if dtc, ok := sp.toolCalls[i]; ok {
            id := dtc.ID
            if id == "" {
                id = generateCallID()
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

## 📦 部署步骤

### 1. 编译新版本
```bash
cd /root/luckyharness-src
/usr/local/go/bin/go build -o lh ./cmd/lh
```

### 2. 替换二进制
```bash
cd /root/.luckyharness
cp lh lh.bak.v0.55.0  # 备份
cp /root/luckyharness-src/lh lh
chmod +x lh
```

### 3. 迁移配置（首次）
```bash
python3 /root/luckyharness-src/scripts/migrate_config.py
```

### 4. 重启网关
```bash
pkill -9 lh
./lh msg-gateway start --platform telegram --token "YOUR_TOKEN"
```

## 🧪 验证方法

### 1. 检查编译结果
```bash
# 验证三处修复已编译
grep -n "if id ==" internal/provider/openai_stream.go internal/provider/stream_parser.go
# 应输出：
# internal/provider/openai_stream.go:185: if id == "" {
# internal/provider/openai_stream.go:269: if id == "" {
# internal/provider/stream_parser.go:117: if id == "" {
```

### 2. 测试工具调用
在 Telegram 发送需要调用工具的消息，观察：
- ✅ 正常响应
- ✅ 无 `empty string` 错误
- ✅ 工具调用正常执行

### 3. 检查日志
```bash
cat /tmp/lh-v0.55.1.log | grep -i "error\|call_id"
```

## 📊 影响范围

| 组件 | 影响 | 修复 |
|------|------|------|
| **配置文件** | `config.yaml` → `config.json` | ✅ 已迁移 |
| **非流式响应** | 工具调用 ID 为空 | ✅ 已修复 |
| **流式响应** | 工具调用 ID 为空 | ✅ 已修复 |
| **流式解析器** | 工具调用 ID 为空 | ✅ 已修复 |

## 🔄 回滚方案

如需回滚到 v0.55.0：
```bash
cd /root/.luckyharness
cp lh.bak.v0.55.0 lh
mv config.json config.json.bak
mv config.yaml.bak config.yaml
pkill -9 lh
./lh msg-gateway start --platform telegram --token "YOUR_TOKEN"
```

## 📚 相关文档

- [配置迁移指南](docs/CONFIG_MIGRATION_v0.55.1.md)
- [配置项说明](docs/config-reference.md)
- [故障排查](docs/troubleshooting.md)

## 🎯 下一步

1. ✅ 监控 Telegram 网关日志，确认无新报错
2. ✅ 观察工具调用成功率
3. ⚠️ 如有其他 API 兼容性问题，继续修复

---

**版本**: v0.55.1  
**修复日期**: 2026-04-23  
**修复者**: RightClaw  
**状态**: ✅ 已部署
