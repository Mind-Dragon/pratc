# Structured Logging Specification

**Package:** `internal/logger`
**Status:** Specification (not yet implemented)
**Created:** 2026-04-02

---

## 1. Overview

This spec defines the unified structured log format for prATC. All Go backend components emit JSON log lines to stderr. The format is consistent across every package, making logs machine-parseable without external monitoring tools.

Python ML service stdout remains reserved for IPC. Any Python logging goes to stderr in the same JSON format.

CLI commands that write `--format=json` to stdout are unaffected. Structured logging is orthogonal to CLI output contracts.

## 2. Log Format

### 2.1 Line Format

Each log entry is a single JSON object on one line, written to stderr:

```json
{
  "ts": "2026-04-02T14:23:01.234Z",
  "level": "INFO",
  "component": "github",
  "request_id": "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
  "repo": "owner/repo",
  "job_id": "sync-20260402-142301",
  "msg": "rate limit status checked",
  "remaining": 1834,
  "reset_epoch": 1712064181
}
```

### 2.2 Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ts` | string (RFC 3339) | yes | UTC timestamp with millisecond precision |
| `level` | string | yes | `INFO` or `ERROR` |
| `component` | string | yes | Package or subsystem name |
| `request_id` | string (UUID v4) | yes* | Correlates all log lines for one request/job. Omitted only in startup/shutdown entries. |
| `repo` | string | when available | Repository in `owner/repo` form. Omitted when no repo context exists. |
| `job_id` | string | when available | Sync or batch job identifier. Omitted outside sync/batch context. |
| `msg` | string | yes | Human-readable message describing the event |
| `*` | any | no | Additional key-value pairs for context. Keys are `snake_case`. Values are strings, numbers, or bools. |

*`request_id` is required for all API request handling and CLI operations. It may be omitted for process-level events like startup, shutdown, and background worker lifecycle.

### 2.3 Rules

1. **One JSON object per line.** No pretty-printing, no multi-line messages.
2. **Keys are snake_case.** No CamelCase, no hyphens in keys.
3. **Values are primitives only.** Strings, numbers (int or float), booleans. No nested objects or arrays as log values. Flatten structured data into top-level keys.
4. **No message interpolation.** The `msg` field is a static template string. Dynamic values go into separate key-value fields.
5. **Deterministic field order.** Core fields (`ts`, `level`, `component`, `request_id`) appear first, followed by context fields alphabetically.

## 3. Log Levels

Only two levels: `INFO` and `ERROR`. No DEBUG, no WARN.

### 3.1 INFO

Use INFO for normal, expected operations. If something is working as designed, it gets logged at INFO.

| Category | Examples |
|----------|----------|
| Request lifecycle | "request received", "request completed", "response sent" |
| Sync operations | "sync started", "sync completed", "PRs fetched: 47" |
| ML bridge | "ML subprocess started", "ML response received" |
| Cache | "cache hit", "cache miss", "migration applied" |
| GitHub client | "GraphQL query executed", "rate limit remaining: 1834" |
| Graph operations | "graph built: 120 nodes, 85 edges" |
| Filter pipeline | "filter stage completed: conflict_detection, dropped 12" |
| Planning | "pool selected: 87 candidates", "plan generated: 20 selected" |
| Service lifecycle | "server started on :7400", "server shutting down" |

### 3.2 ERROR

Use ERROR when an operation fails or encounters an unexpected condition that requires attention.

| Category | Examples |
|----------|----------|
| Request failures | "request failed: database connection timeout" |
| GitHub API errors | "GraphQL query failed: rate limit exhausted", "HTTP 502 from upstream" |
| ML bridge failures | "ML subprocess crashed: exit code 1", "ML timeout after 30s" |
| Cache failures | "migration failed: table already exists", "database locked" |
| Sync failures | "sync failed: cursor invalid", "mirror fetch failed" |
| Internal errors | "unhandled error in handler", "panic recovered" |

### 3.3 Level Decision Guide

```
Did the operation succeed or behave as expected?
  yes -> INFO
  no  -> Is this a transient/recoverable condition that was handled?
           yes -> INFO (log that it was retried/recovered)
           no  -> ERROR
```

For example, a rate limit pause that succeeds after backoff is INFO ("rate limit pause completed, resumed after 12s"). A rate limit exhaustion that causes the operation to abort is ERROR ("sync aborted: rate limit exhausted, no retries remaining").

## 4. Request ID

### 4.1 Format

UUID v4, lowercased, no braces, no dashes... actually, with dashes. Standard UUID v4 format:

```
a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d
```

### 4.2 Generation

- Use `crypto/rand` via `google/uuid` or the Go 1.21+ `log/slog` context integration.
- In practice: `github.com/google/uuid` is the standard. Generate with `uuid.New().String()`.

### 4.3 Propagation

The request ID flows through the system via Go's `context.Context`:

```
HTTP request arrives
    |
    v
Middleware generates UUID, stores in context
    |
    v
Handler extracts request_id from context
    |
    v
Service methods receive context, pass to logger
    |
    v
Logger reads request_id from context, includes in every log line
```

#### 4.3.1 Context Key

```go
type contextKey struct{}

func RequestIDFromContext(ctx context.Context) string {
    if id, ok := ctx.Value(contextKey{}).(string); ok {
        return id
    }
    return ""
}

func ContextWithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, contextKey{}, id)
}
```

The key is an unexported struct type to prevent collisions with other context values.

#### 4.3.2 HTTP Middleware

A middleware wraps the `http.ServeMux`. For each incoming request:

1. Check the `X-Request-ID` header. If present and a valid UUID v4, use it. This allows external callers to pass their own correlation ID.
2. If absent or invalid, generate a new UUID v4.
3. Store in request context via `ContextWithRequestID`.
4. Set the `X-Request-ID` response header so callers can correlate responses.

For background jobs (sync, batch operations), the job runner generates the request ID at job start and passes it via context to all downstream calls.

#### 4.3.3 CLI Operations

Each CLI command invocation gets a request ID generated at startup. It propagates through the `context.Context` passed to service methods. This lets operators correlate CLI log output with specific commands.

#### 4.3.4 Go-to-Python Bridge

The ML bridge does not receive the request ID directly. Instead, the Go caller logs the request ID in its own log lines around the ML call. The Python subprocess only logs to stderr (never to stdout, which is reserved for IPC). Python log lines include the request ID if passed as a field in the IPC payload, but this is optional for v0.1.

## 5. Component Names

Component identifies the originating package or subsystem. Use these exact values:

| Component | Source Package |
|-----------|---------------|
| `serve` | `internal/cmd` (HTTP server) |
| `app` | `internal/app` (service facade) |
| `github` | `internal/github` (GraphQL client) |
| `ml` | `internal/ml` (bridge to Python) |
| `cache` | `internal/cache` (SQLite) |
| `filter` | `internal/filter` (pipeline) |
| `graph` | `internal/graph` |
| `planner` | `internal/planner` |
| `formula` | `internal/formula` |
| `planning` | `internal/planning` |
| `sync` | `internal/sync` |
| `settings` | `internal/settings` |
| `audit` | `internal/audit` |
| `analysis` | `internal/analysis` |
| `repo` | `internal/repo` (git mirror) |
| `report` | `internal/report` |
| `cli` | `cmd/pratc` (CLI entrypoints) |

Component names match the directory name, not the Go package import path. This keeps them short and predictable.

## 6. Output Destination

| Process | Log Destination | Notes |
|---------|----------------|-------|
| Go CLI (`pratc analyze`, etc.) | stderr | JSON lines. stdout remains reserved for `--format=json` CLI output. |
| Go HTTP server (`pratc serve`) | stderr | JSON lines. HTTP responses are separate. |
| Python ML service | stderr | JSON lines. stdout is reserved for IPC with Go bridge. |

No log files are written by the application itself. Operators redirect stderr to files or pipe to log collectors as needed:

```bash
pratc serve 2>> /var/log/pratc/server.log
pratc analyze --repo=owner/repo 2>> /var/log/pratc/analyze.log
```

## 7. Integration with Go log/slog

### 7.1 Handler Setup

The `internal/logger` package provides a constructor that returns a configured `*slog.Logger`:

```go
func New(component string) *slog.Logger
```

This creates a logger pre-configured with:
- JSON handler writing to `os.Stderr`
- The `component` field set as a permanent attribute
- The `request_id`, `repo`, and `job_id` fields extracted from context on each call

### 7.2 Context-Aware Logging

Since `request_id` lives in context, every log call must accept a context parameter. The logger provides two signatures:

```go
// Standard: extracts request_id, repo, job_id from context
logger.InfoContext(ctx, "message", "key", value)

// Explicit: override context-derived values (rare)
logger.InfoContext(ctx, "message",
    "request_id", explicitID,
    "key", value,
)
```

Always prefer `InfoContext`/`ErrorContext` over `Info`/`Error`. The bare `Info`/`Error` methods should only be used in process lifecycle code where no context exists.

### 7.3 slog.Handler Customization

The default `slog.JSONHandler` handles most needs. The custom handler only adds context extraction:

```go
type contextHandler struct {
    handler slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
    // Extract request_id, repo, job_id from ctx
    // Add as attrs to r
    return h.handler.Handle(ctx, r)
}
```

This wrapping handler is the only custom code needed. No custom `slog.LogValuer` implementations for v0.1.

### 7.4 Performance

`slog` with the JSON handler adds roughly 1-3 microseconds per log call at INFO level. With an estimated 500-2000 log lines per typical operation, this is well under the 15% overhead budget.

For hot paths (e.g., per-PR iterations in the filter pipeline), use `slog.Enabled(ctx, slog.LevelInfo)` to skip logging when disabled, though INFO is always enabled in the current two-level design.

## 8. Example Log Sequences

### 8.1 API Request

```json
{"ts":"2026-04-02T14:23:01.234Z","level":"INFO","component":"serve","request_id":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","repo":"acme/webapp","msg":"request received","method":"GET","path":"/api/repos/acme/webapp/analyze"}
{"ts":"2026-04-02T14:23:01.456Z","level":"INFO","component":"cache","request_id":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","repo":"acme/webapp","msg":"cache hit","table":"pull_requests","rows":5423}
{"ts":"2026-04-02T14:23:01.789Z","level":"INFO","component":"github","request_id":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","repo":"acme/webapp","msg":"GraphQL query executed","query":"fetchPRs","remaining":2841}
{"ts":"2026-04-02T14:23:03.012Z","level":"INFO","component":"ml","request_id":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","repo":"acme/webapp","msg":"ML response received","action":"cluster","duration_ms":890}
{"ts":"2026-04-02T14:23:03.100Z","level":"INFO","component":"app","request_id":"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d","repo":"acme/webapp","msg":"request completed","method":"GET","path":"/api/repos/acme/webapp/analyze","status":200,"duration_ms":1866}
```

### 8.2 Sync Job

```json
{"ts":"2026-04-02T15:00:00.001Z","level":"INFO","component":"sync","request_id":"b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e","repo":"acme/webapp","job_id":"sync-20260402-150000","msg":"sync started","cursor":""}
{"ts":"2026-04-02T15:00:02.500Z","level":"INFO","component":"github","request_id":"b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e","repo":"acme/webapp","job_id":"sync-20260402-150000","msg":"rate limit pause completed","duration_ms":2000}
{"ts":"2026-04-02T15:00:45.200Z","level":"INFO","component":"sync","request_id":"b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e","repo":"acme/webapp","job_id":"sync-20260402-150000","msg":"sync completed","prs_fetched":87,"duration_ms":45199}
```

### 8.3 Error Recovery

```json
{"ts":"2026-04-02T15:05:10.100Z","level":"ERROR","component":"github","request_id":"c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f","repo":"acme/webapp","msg":"GraphQL query failed","query":"fetchPRs","status":502,"error":"upstream service unavailable","retry":1,"max_retries":6}
{"ts":"2026-04-02T15:05:12.400Z","level":"INFO","component":"github","request_id":"c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f","repo":"acme/webapp","msg":"retry succeeded","query":"fetchPRs","attempt":2,"duration_ms":2300}
```

### 8.4 Process Lifecycle (no request_id)

```json
{"ts":"2026-04-02T14:00:00.000Z","level":"INFO","component":"serve","msg":"server started","port":7400,"version":"1.7.1"}
{"ts":"2026-04-02T18:30:00.000Z","level":"INFO","component":"serve","msg":"server shutting down","signal":"SIGTERM"}
```

## 9. Relationship to Existing Systems

### 9.1 Audit Log (`internal/audit`)

The audit log and structured log serve different purposes:

- **Structured log** (`internal/logger`): operational, ephemeral, per-event, for debugging and monitoring
- **Audit log** (`internal/audit`): persistent, business-level, for compliance and history

Both systems remain independent. The audit log stores `AuditEntry` records in SQLite. The structured log writes JSON to stderr. They are not merged or synchronized.

### 9.2 OperationTelemetry (`internal/types/models.go`)

`OperationTelemetry` carries operation-level metrics (pool strategy, latencies, drop counts) in API responses. This is response payload data, not log data.

Structured logging complements telemetry by providing a timeline of events. A single API request might produce dozens of log lines but one telemetry block in the response.

### 9.3 Telemetry Contract (AGENTS.md)

The telemetry contract in AGENTS.md defines metric names (counters, histograms) that will eventually map to OpenTelemetry or similar. This spec covers the logging format, not the metrics format. They are complementary:
- Log format: human+machine readable event stream
- Telemetry metrics: aggregated numeric data for dashboards and alerting

Both share the `component`, `repo`, and `job_id` fields for correlation.

## 10. What This Spec Does NOT Cover

- Log rotation or retention (handled by OS/file redirect)
- Log shipping to external systems (out of scope for v0.1)
- Structured error types with stack traces (future enhancement)
- Log sampling or rate limiting (not needed at current scale)
- Python-side logging implementation (separate spec for `ml-service`)
- Per-component log level configuration (two levels, always enabled)

## 11. Implementation Checklist

When implementing this spec:

1. Create `internal/logger/logger.go` with `New(component string) *slog.Logger`
2. Create `internal/logger/context.go` with `ContextWithRequestID` and `RequestIDFromContext`
3. Create `internal/logger/middleware.go` with HTTP request ID middleware
4. Add request ID generation to CLI command entrypoints in `cmd/pratc/`
5. Add request ID propagation to sync job runner in `internal/sync/`
6. Wire logger into `internal/cmd/root.go` handler functions
7. Replace any `fmt.Fprintf(os.Stderr, ...)` calls with logger calls across packages
8. Add tests for context propagation, field ordering, and omit-when-empty behavior
9. Update AGENTS.md telemetry contract section to reference this spec
