# Upstream Response Capture (2026-04-24)

## 目标

复现并抓取上游 OpenAI-compatible 接口在 `gpt-5.4-mini` 下的完整原始响应（非流式 + 流式），用于排查：

- `non-stream empty content with ... completion_tokens`
- `stream retry ...`

## 抓取开关

已在代码中增加环境变量开关：

- `LH_UPSTREAM_CAPTURE_DIR=/path/to/dir`

开启后每次上游请求会在该目录生成：

- `*.meta.json`：请求上下文（provider/model/api_base/时间）
- `*.request.json`：完整请求体
- `*.response.meta.json`：HTTP 状态码 + 响应头
- `*.response.body.txt`：非流式完整响应体
- `*.response.sse.txt`：流式完整原始 SSE（逐字节镜像）
- `*.error.txt`：请求/读取/扫描错误（如有）

## 本次复现命令

```bash
export HOME="$PWD/.lh-home"
export LH_UPSTREAM_CAPTURE_DIR="$PWD/docs/upstream-captures"
export HTTPS_PROXY="http://127.0.0.1:7897"
export HTTP_PROXY="http://127.0.0.1:7897"
export NO_PROXY="127.0.0.1,localhost"
go run ./cmd/lh chat "帮我深度调研计算机的就业行情"
```

## 关键结论

1. 非流式响应返回 `200`，但 `message` 只有 `role`，没有 `content`，同时 `completion_tokens` 很高。  
   示例文件：
- `docs/upstream-captures/20260424_194140.502_000001_chat_completions_non_stream.response.body.txt`
- `docs/upstream-captures/20260424_194201.436_000003_chat_completions_non_stream.response.body.txt`
- `docs/upstream-captures/20260424_194215.552_000005_chat_completions_non_stream.response.body.txt`

2. 流式重试能拿到完整 SSE；部分轮次仅返回 `tool_calls`，`finish_reason=tool_calls`，仍无最终文本。  
   示例文件：
- `docs/upstream-captures/20260424_194219.461_000006_chat_completions_stream.response.sse.txt`
- `docs/upstream-captures/20260424_194208.355_000004_chat_completions_stream.response.sse.txt`
- `docs/upstream-captures/20260424_194152.345_000002_chat_completions_stream.response.sse.txt`

## 完整文件清单（本次抓取）

- `docs/upstream-captures/20260424_194125.388_000001_chat_completions_non_stream.error.txt`
- `docs/upstream-captures/20260424_194125.388_000001_chat_completions_non_stream.meta.json`
- `docs/upstream-captures/20260424_194125.388_000001_chat_completions_non_stream.request.json`
- `docs/upstream-captures/20260424_194140.502_000001_chat_completions_non_stream.meta.json`
- `docs/upstream-captures/20260424_194140.502_000001_chat_completions_non_stream.request.json`
- `docs/upstream-captures/20260424_194140.502_000001_chat_completions_non_stream.response.body.txt`
- `docs/upstream-captures/20260424_194140.502_000001_chat_completions_non_stream.response.meta.json`
- `docs/upstream-captures/20260424_194152.345_000002_chat_completions_stream.meta.json`
- `docs/upstream-captures/20260424_194152.345_000002_chat_completions_stream.request.json`
- `docs/upstream-captures/20260424_194152.345_000002_chat_completions_stream.response.meta.json`
- `docs/upstream-captures/20260424_194152.345_000002_chat_completions_stream.response.sse.txt`
- `docs/upstream-captures/20260424_194201.436_000003_chat_completions_non_stream.meta.json`
- `docs/upstream-captures/20260424_194201.436_000003_chat_completions_non_stream.request.json`
- `docs/upstream-captures/20260424_194201.436_000003_chat_completions_non_stream.response.body.txt`
- `docs/upstream-captures/20260424_194201.436_000003_chat_completions_non_stream.response.meta.json`
- `docs/upstream-captures/20260424_194208.355_000004_chat_completions_stream.meta.json`
- `docs/upstream-captures/20260424_194208.355_000004_chat_completions_stream.request.json`
- `docs/upstream-captures/20260424_194208.355_000004_chat_completions_stream.response.meta.json`
- `docs/upstream-captures/20260424_194208.355_000004_chat_completions_stream.response.sse.txt`
- `docs/upstream-captures/20260424_194215.552_000005_chat_completions_non_stream.meta.json`
- `docs/upstream-captures/20260424_194215.552_000005_chat_completions_non_stream.request.json`
- `docs/upstream-captures/20260424_194215.552_000005_chat_completions_non_stream.response.body.txt`
- `docs/upstream-captures/20260424_194215.552_000005_chat_completions_non_stream.response.meta.json`
- `docs/upstream-captures/20260424_194219.461_000006_chat_completions_stream.meta.json`
- `docs/upstream-captures/20260424_194219.461_000006_chat_completions_stream.request.json`
- `docs/upstream-captures/20260424_194219.461_000006_chat_completions_stream.response.meta.json`
- `docs/upstream-captures/20260424_194219.461_000006_chat_completions_stream.response.sse.txt`

## 备注

- 抓取文件保留原始业务输入与上游返回，请按需管理访问权限。  
- 抓取仅在设置 `LH_UPSTREAM_CAPTURE_DIR` 时启用，不影响默认运行。

