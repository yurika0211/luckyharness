# LuckyHarness 性能评测报告（2026-04-25）

## 1. 报告目的

建立 LuckyHarness 在 `msg-gateway + API Server` 场景下的可复现性能评测流程，统一采集口径，输出可比较的性能数据（延迟、成功率、错误率、token 消耗、工具调用、网关稳定性）。

本报告覆盖：

- 在线请求性能（`POST /api/v1/chat/sync`）
- 服务侧运行指标（`GET /api/v1/stats`、`GET /api/v1/metrics`）
- 网关稳定性（`GET /api/v1/gateways`）
- 质量与延迟基线（`lh eval run`）

---

## 2. 评测范围与对象

### 2.1 接口范围

- `POST /api/v1/chat/sync`：同步聊天主评测接口
- `GET /api/v1/stats`：服务请求统计
- `GET /api/v1/metrics`：Prometheus 指标
- `GET /api/v1/gateways`：网关状态与收发计数

### 2.2 执行方式

- 本地运行：`go run ./cmd/lh ...`
- 默认评测地址：`http://127.0.0.1:19090`

> 注意：如果不是 `19090`，请统一替换为实际地址。

---

## 3. 关键指标定义（统一口径）

### 3.1 请求层指标

- `success_rate`：HTTP 200 数 / 总请求数
- `error_rate`：非 200 数 / 总请求数
- `avg_latency_s`：全部请求平均耗时（秒）
- `p95_latency_s`：95 分位耗时（秒）

### 3.2 模型执行指标（来自 `/chat/sync` 响应）

- `tokens_used`：单请求 token 总消耗
- `iterations`：单请求 loop 迭代次数
- `tool_calls_count`：单请求工具调用次数（`tool_calls` 长度）

### 3.3 服务级指标（来自 `/stats`、`/metrics`）

- 总请求、聊天请求、错误请求
- Provider 调用次数、错误次数、平均延迟
- 活跃会话数

### 3.4 网关稳定性指标（来自 `/gateways`）

- `messages_sent`
- `messages_received`
- `errors`
- `running` 状态连续性

---

## 4. 数据采集流程（可直接执行）

以下命令会将本次评测数据写入 `bench/<timestamp>/`。

```bash
API="http://127.0.0.1:19090"
RUN="bench/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RUN"

git rev-parse --short HEAD > "$RUN/commit.txt"
go version > "$RUN/go_version.txt"

# 起始快照
curl -s "$API/api/v1/health"  > "$RUN/health_start.json"
curl -s "$API/api/v1/stats"   > "$RUN/stats_start.json"
curl -s "$API/api/v1/metrics" > "$RUN/metrics_start.prom"

# 评测问题集
cat > "$RUN/prompts.txt" << 'QEOF'
请用三点总结 Go 并发模型
解释一下 TCP 三次握手
帮我写一个二分查找的 Go 示例
QEOF

# 发请求并记录 http_code + time_total
i=0
while IFS= read -r q; do
  [ -z "$q" ] && continue
  i=$((i+1))
  payload=$(jq -nc --arg m "$q" '{message:$m,max_iterations:8,auto_approve:true}')
  curl -sS -o "$RUN/resp_${i}.json" \
    -w "%{http_code} %{time_total}\n" \
    -X POST "$API/api/v1/chat/sync" \
    -H "Content-Type: application/json" \
    -d "$payload" >> "$RUN/http_times.log"
done < "$RUN/prompts.txt"

# 汇总请求层指标
TOTAL=$(wc -l < "$RUN/http_times.log")
OK=$(awk '$1==200{c++} END{print c+0}' "$RUN/http_times.log")
AVG=$(awk '{s+=$2} END{if(NR) printf "%.3f", s/NR; else print "0"}' "$RUN/http_times.log")
P95=$(awk '{print $2}' "$RUN/http_times.log" | sort -n | awk '{a[NR]=$1} END{if(NR==0){print 0}else{idx=int((95*NR+99)/100); if(idx<1) idx=1; print a[idx]}}')

printf "total=%s ok=%s success_rate=%s avg_s=%s p95_s=%s\n" \
  "$TOTAL" "$OK" \
  "$(awk -v ok=$OK -v t=$TOTAL 'BEGIN{if(t==0)print 0; else printf "%.4f", ok/t}')" \
  "$AVG" "$P95" | tee "$RUN/latency_summary.txt"

# 汇总模型执行指标
jq -s '{
  avg_tokens: (map(.tokens_used // 0) | (if length==0 then 0 else add/length end)),
  avg_iterations: (map(.iterations // 0) | (if length==0 then 0 else add/length end)),
  avg_tool_calls: (map((.tool_calls // []) | length) | (if length==0 then 0 else add/length end))
}' "$RUN"/resp_*.json | tee "$RUN/model_summary.json"

# 结束快照
curl -s "$API/api/v1/stats"   > "$RUN/stats_end.json"
curl -s "$API/api/v1/metrics" > "$RUN/metrics_end.prom"
```

---

## 5. 网关稳定性采集（Telegram/OneBot）

每 5 秒采样一次网关状态，持续 5 分钟：

```bash
API="http://127.0.0.1:19090"
RUN="bench/$(date +%Y%m%d_%H%M%S)_gateway"
mkdir -p "$RUN"

for _ in $(seq 1 60); do
  printf "%s " "$(date '+%F %T')" >> "$RUN/gateway_status.ndjson"
  curl -s "$API/api/v1/gateways" >> "$RUN/gateway_status.ndjson"
  echo >> "$RUN/gateway_status.ndjson"
  sleep 5
done
```

建议重点观察：

- `running` 是否始终为 `true`
- `errors` 是否突增
- `messages_sent/received` 是否符合业务负载

---

## 6. 质量基线（Eval）

用于回归对比（文本质量 + 延迟约束），命令如下：

```bash
lh eval list ./eval-cases
lh eval run ./eval-cases -f text -t 0.7 -o bench/eval_report.txt
```

测试用例 YAML 示例：

```yaml
- id: qa_go_concurrency_001
  name: Go 并发基础问答
  input:
    query: "请解释 goroutine 和 channel 的关系"
  expected:
    responseContains: ["goroutine", "channel"]
    maxLatency: 8s
```

限制说明：

- 当前 CLI `eval` runner 主要稳定产出 `response + latency`。
- 细粒度 `token/tool` 指标建议以 `/api/v1/chat/sync` 响应采集为准。

---

## 7. 可选：上游响应抓包（定位空响应/超时）

如需定位 provider 侧返回问题，可启用：

```bash
export LH_UPSTREAM_CAPTURE_DIR="$PWD/docs/upstream-captures"
```

参考说明文档：

- `docs/UPSTREAM_RESPONSE_CAPTURE_2026-04-24.md`

---

## 8. 本次报告结论（当前状态）

截至 2026-04-25，本报告已完成：

- 统一性能评测口径
- 提供可复现采集命令
- 提供网关稳定性采样方法
- 提供 Eval 质量基线入口
- 提供上游抓包定位入口

尚未包含真实跑数结果（P50/P95/错误率趋势）。建议按第 4、5 节执行后，将 `bench/<timestamp>/` 产物追加归档到下一版报告。

---

## 9. 下一版建议补充

- 引入固定 20/50/100 并发压测场景（持续 5~15 分钟）
- 增加不同模型（如 `gpt-5.4-mini` vs 其他）横向对比
- 增加 Telegram 真实对话流量窗口下的稳定性对比
- 对 `timeout` 与 `empty content` 建立错误码分布看板
