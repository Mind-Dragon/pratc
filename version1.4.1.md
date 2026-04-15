# prATC v1.4.1 — Resumable Worker and Explicit Sync States

This minor release tightens the production execution model around rate limits and long-running sync jobs.

## What changed

- Workflow and sync paths now preserve paused job state in SQLite.
- Rate-limit pauses use explicit job states instead of an in-process sleep-only model.
- Job lifecycle states are explicit: `queued`, `running`, `paused_rate_limit`, `resuming`, `completed`, `failed`, `canceled`.
- Status endpoints and CLI surfaces report explicit state and resume metadata.
- Persistent workflow artifacts live under `projects/<repo>/runs/<timestamp>/` with a README manifest.

## Why it matters

The worker can now survive rate-limit pauses and process restarts without losing its checkpointed state, which makes the production workflow less dependent on a single foreground shell session.

## Verification

- `go test ./internal/cache ./internal/cmd ./internal/sync ./internal/monitor/data`
- `go test ./...`
