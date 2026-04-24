# Changelog

## v0.38.3 — Gateway Control & Telegram Command Runtime (2026-04-24)

### 🐛 Fixes

- `lh msg-gateway status/stop` now controls the running gateway process via HTTP API instead of local in-memory state.
  - Added `--api-addr` for both commands (defaults to `msg_gateway.api_addr`, then `127.0.0.1:9090`).
  - Improved API error propagation with structured fallback parsing.
- Implemented Telegram `/stop` real task cancellation:
  - Added per-chat cancellable task tracking.
  - `/stop` now cancels active request and returns immediate feedback.
- Implemented Telegram `/restart` runtime reconnect:
  - Added adapter stop/start reconnect flow with anti-reentry guard.
  - Bot reports reconnect result in-chat after restart attempt.
- Decoupled Telegram message handling from update loop by dispatching chat processing asynchronously.
  - Prevents long-running chat from blocking command handling.

## v0.38.2 — Provider Resilience & Config Wiring (2026-04-24)

### 🐛 Fixes

- Wired runtime provider safety configs from app config to provider layer:
  - limits / retry / circuit_breaker / rate_limit / context are now propagated when creating provider config.
- Hardened OpenAI-compatible HTTP request path:
  - Added dedicated HTTP client/transport for OpenAI calls.
  - Disabled HTTP/2 attempt on this path to reduce flaky proxy behavior.
  - Added transport-level retry with exponential backoff for retryable network/TLS failures.
  - Retries force fresh connection on subsequent attempts (`req.Close = true` + close idle conns).
- Added retry classification for common transient failures:
  - `tls: bad record mac`, timeout-like errors, connection reset/lost/unexpected EOF variants.

### 🧪 Tests

- Added `openai_stream_retry_test.go` to verify:
  - retryable TLS error detection
  - request retries are attempted and second attempt forces new connection

## v0.38.1 — Stability & Gateway Reliability (2026-04-24)

### 🐛 Fixes

- Fixed Telegram `msg-gateway` startup concurrency bug in CLI path
  - Removed invalid `WaitGroup` usage (`Done` without matching `Add`)
  - API server startup now returns explicit startup errors
- Unified Telegram chat-session persistence path between CLI and HTTP API
  - Both now use `Config().HomeDir()/data/telegram`
- Fixed legacy tool-history compatibility for OpenAI-compatible gateways
  - Prevent `Invalid input[*].call_id: empty string` by downgrading invalid legacy `tool` messages before request encoding
- Persisted structured tool-call metadata in session history
  - Preserve `assistant.tool_calls` and `tool.tool_call_id` instead of flattening to plain text
- Enforced max-iteration boundary in streaming conversation path
  - Native/simulated streaming recursion now decrements remaining iterations and exits with `max iterations reached`

### 🧪 Tests

- Added regression test for stream path max-iteration enforcement
- Added provider/session tests for tool-call metadata and legacy compatibility

## v0.38.0 — Agent Autonomy Kit (2026-04-21)

### 🧠 New: `internal/autonomy` — Native Agent Autonomy Kit

The biggest feature since v0.36.0. Agents can now work proactively without human prompting.

**Architecture:**
```
┌─────────────────────────────────────────────┐
│              AutonomyKit                     │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐ │
│  │TaskQueue │──│WorkerPool│──│Heartbeat   │ │
│  │          │  │          │  │Engine      │ │
│  │ Ready ──→│  │ W1 ──→  │  │ (proactive)│ │
│  │ InProg   │  │ W2 ──→  │  │            │ │
│  │ Blocked  │  │ W3 ──→  │  │ 15m cycle  │ │
│  │ Done     │  │ ...     │  │            │ │
│  └──────────┘  └──────────┘  └───────────┘ │
│       ↑              │                      │
│       │    ┌─────────┘                      │
│       │    ↓                                │
│  ┌──────────────────────┐                   │
│  │  AgentExecutor       │                   │
│  │  (interface)         │                   │
│  │  (isolated session)  │                   │
│  └──────────────────────┘                   │
└─────────────────────────────────────────────┘
```

**Components:**

1. **TaskQueue** (`queue.go`)
   - Priority-based task queue: Ready → InProgress → Done/Blocked
   - Concurrent-safe with `sync.RWMutex`
   - Channel-based ready notification for non-blocking dispatch
   - Operations: Add, Pull (highest priority first), Complete, Fail, Block/Unblock
   - Stats, CleanDone, ListByState

2. **WorkerPool** (`worker.go`)
   - Goroutine-based worker pool (not OS threads — Go's concurrency advantage)
   - Each Worker has isolated session for context isolation
   - Workers execute tasks through `AgentExecutor` interface (breaks import cycle)
   - Auto-dispatch: idle workers pull from queue automatically
   - ScaleUp/ScaleDown for dynamic pool sizing
   - Backpressure via buffered results channel
   - Graceful shutdown with context cancellation

3. **HeartbeatEngine** (`heartbeat.go`)
   - Proactive heartbeat that **does work**, not just checks
   - Two modes: `passive` (check only) and `proactive` (check + dispatch)
   - Active hours configuration (supports midnight wrap, e.g. 22:00-06:00)
   - Max tasks per beat to prevent overload
   - Manual trigger support (`autonomy_heartbeat_trigger` tool)
   - Event history for observability

4. **AutonomyKit** (`heartbeat.go`)
   - Top-level orchestrator combining Queue + Pool + Heartbeat
   - Single `Start()`/`Stop()` lifecycle
   - Convenience methods: `AddTask()`, `Status()`

5. **Built-in Tools** (`tools.go`)
   - `autonomy_queue_add` — Add task to queue
   - `autonomy_queue_list` — List tasks (filter by state)
   - `autonomy_queue_update` — Complete/fail/block/unblock tasks
   - `autonomy_worker_spawn` — Spawn worker for specific task
   - `autonomy_worker_list` — List active workers
   - `autonomy_heartbeat_trigger` — Manual heartbeat trigger
   - `autonomy_status` — Overall autonomy system status

### 🔧 Enhanced: `tool/delegate.go` — Real Agent Execution

- Added `AgentExecutorFunc` type and `SetAgentExecutor()` method
- `executeTask()` now calls Agent Loop when executor is set (no more placeholder)
- Falls back to placeholder only when no executor is configured

### 🔧 Enhanced: `agent/agent.go` — Autonomy Integration

- Added `autonomy *autonomy.AutonomyKit` field
- `New()` registers 7 autonomy tools with tool registry
- `StartAutonomy(ctx)` method for explicit startup
- `agentExecutorAdapter` bridges `Agent` → `AgentExecutor` interface
- Delegate manager gets `AgentExecutorFunc` for real sub-agent execution
- Added `Autonomy()` accessor method

### 🧪 Tests

- 22 tests in `internal/autonomy/autonomy_test.go` — all passing
- Covers: queue CRUD, priority ordering, concurrent access, heartbeat hours, tool handlers
- Full project test suite: **all green** (0 failures)

### 📐 Design Decisions

- **Interface over import**: `AgentExecutor` interface breaks the `autonomy ↔ agent` import cycle
- **Goroutines over threads**: Worker pool uses lightweight goroutines, not OS threads
- **Channels over locks**: Task dispatch uses channel-based communication
- **Proactive over passive**: Heartbeat defaults to proactive mode (dispatches work, not just checks)
- **Isolated sessions**: Each worker gets its own session for context isolation

---

## v0.37.0 — Web Search Rewrite (2026-04-21)

- web_search multi-source fallback: Brave → ddgs → DDG Lite → SearXNG
- web_search deep mode: multi-source cross-validation + URL dedup
- web_fetch fallback: Defuddle CLI → Jina Reader → curl+stripHTML
- WebSearchConfig struct, RegisterBuiltinToolsWithConfig
- applyWebSearchEnv() from LH_WEB_SEARCH_* environment variables

## v0.36.0 — Full Module Integration (2026-04-21)

- Telegram multimedia (image/voice/video/file attachments)
- Cron engine (bot commands / cron add/remove/pause/resume)
- Metrics (chat/tool tracking)
- HTTP API (:9090 parallel startup)
- Bot command expansion (/skills /cron /metrics /health)
- Skill system upgrade: two-layer design (summaries in prompt + skill_read tool)
