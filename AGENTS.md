# AGENTS.md — prATC Knowledge Base

**Generated:** 2026-04-24 | **Commit:** 8be25e9 | **Branch:** main

## Overview
prATC (PR Air Traffic Control) — self-hostable, repo-agnostic system for large-scale PR triage, merge planning, and v2.0 action-lane orchestration. Go CLI + Python ML service + HTTP API + TUI dashboard. The active v2.0 plan is `VERSION2.0.md`.

## Structure
```
pratc/
├── cmd/pratc/          # CLI entrypoints (init() → RegisterXCommand)
├── internal/           # Go backend (see internal/AGENTS.md)
│   ├── cmd/            # HTTP server + API routes + settings (see internal/cmd/AGENTS.md)
│   ├── app/            # Service facade (Analyze/Cluster/Graph/Plan)
│   ├── planning/       # Pool selector, hierarchy, pairwise, time decay (see internal/planning/AGENTS.md)
│   ├── planner/        # Core planner with functional options
│   ├── formula/        # Combinatorial engine P/C/n^k (see internal/formula/AGENTS.md)
│   ├── filter/         # Pre-filter pipeline + scoring (see internal/filter/AGENTS.md)
│   ├── cache/          # SQLite + migrations (see internal/cache/AGENTS.md)
│   ├── graph/          # Dependency/conflict graph + DOT + incremental
│   ├── github/         # GraphQL client + rate limiting
│   ├── ml/             # Go→Python bridge (stdin/stdout JSON)
│   ├── types/          # Shared type definitions (PR, responses, telemetry)
│   ├── settings/       # Settings store (global/repo scope, YAML import/export)
│   ├── sync/           # Background sync worker + mirror
│   ├── analysis/       # Bot detection
│   ├── audit/          # Audit log (memory + SQLite)
│   ├── repo/           # Git mirror management
│   ├── report/         # PDF snapshot generation
│   ├── actions/        # v2.0 target: ActionPlan, action lanes, policy gates
│   ├── workqueue/      # v2.0 target: swarm claims, leases, proof bundles
│   ├── executor/       # v2.0 target: dry-run/live preflight and GitHub mutation
│   ├── monitor/        # TUI dashboard and monitoring surfaces
│   └── testutil/       # Fixture loading helpers
├── ml-service/         # Python ML service (see ml-service/AGENTS.md)
│   └── src/pratc_ml/   # Clustering, duplicates, overlap, providers
├── web/                # Archived/deferred browser dashboard; v2.0 dashboard is TUI-first
│   └── src/            # Pages, components, lib, types, styles
├── fixtures/           # Test data (~5,500 PR snapshot)
├── contracts/          # API contract definitions
└── scripts/            # Development scripts
```

## Where To Look
| Task | Location | Notes |
|------|----------|-------|
| CLI command wiring | `cmd/pratc/*.go` | `init()` → `cmd.RegisterXCommand()` |
| HTTP API routes | `internal/cmd/root.go` | All routes registered via `RegisterServeCommand` |
| Core business logic | `internal/app/service.go` | Service methods: Analyze, Cluster, Graph, Plan, v2.0 Actions |
| PR type definitions | `internal/types/models.go` | Domain types + response/request structs; add ActionPlan types here first |
| Action lanes/policy | `internal/actions/` | v2.0 target: classifier, policy gates, ActionPlan builder |
| Work queue | `internal/workqueue/` | v2.0 target: claim leases, heartbeat, proof bundle association |
| Executor | `internal/executor/` | v2.0 target: dry-run/live preflight and centralized GitHub mutation |
| Pre-filter pipeline | `internal/filter/pipeline.go` | Draft → conflict → CI → bot filtering |
| Combinatorial engine | `internal/formula/modes.go` | `Count()` + `GenerateByIndex()` with math/big |
| Pool selection | `internal/planning/pool.go` | Priority weights, time decay, cluster coherence |
| Dependency graph | `internal/graph/graph.go` | `Build()` + `TopologicalOrder()` + `DOT()` |
| SQLite + migrations | `internal/cache/sqlite.go` | Forward-only, `schema_migrations` table |
| ML bridge | `internal/ml/bridge.go` | JSON stdin/stdout to Python subprocess |
| Settings CRUD | `internal/settings/store.go` | Global/repo scope, YAML import/export |
| GitHub GraphQL client | `internal/github/client.go` | Rate limiting + retry with jitter |
| Fixture loader | `internal/testutil/fixture_loader.go` | JSON fixture files in `fixtures/` |

## Cross-Cutting Patterns

### Type Consistency
Go, Python, and TypeScript share **identical** type definitions with `snake_case` JSON keys for active cross-language surfaces. v2.0 ActionPlan fields must be added to all retained consumers before release.
- Go: `internal/types/models.go`
- Python: `ml-service/src/pratc_ml/models.py` (Pydantic with bootstrap fallback)
- TypeScript: `web/src/types/api.ts`

### API Contracts
All responses include `repo` + `generatedAt` + operation-specific payload.
Error format: `{"error": "...", "message": "...", "status": "..."}`

### Configuration Flow
`env vars → Go (root.go) → Go app (service.go/actions) → Python ML (via stdin JSON, optional) → TUI/API/PDF consumers`

### Go↔Python IPC
JSON over stdin/stdout via `exec.CommandContext` (`internal/ml/bridge.go`). Actions: `health`, `cluster`, `duplicates`, `overlap`.

### Shared Thresholds
`duplicateThreshold = 0.85`, `overlapThreshold = 0.70` — defined in `internal/types/constants.go` / app defaults, mirrored in Python defaults, documented in AGENTS.md.

## Commands
```bash
make verify-env    # Check toolchain (go, python3.11+, uv, node, bun, docker)
make build         # Compile Go binary to ./bin/pratc
make test          # Run all tests (go + python + web)
make test-go       # go test -race -v ./...
make test-python   # uv run pytest -v (in ml-service/)
make test-web      # bun run test (vitest, in web/)
make lint          # go vet ./... (only — no golangci-lint yet)
make docker-up     # docker-compose --profile local-ml up --build -d
make docker-down   # docker-compose down --remove-orphans
```

## Anti-Patterns (This Project)
- Never read raw secret values; use `psst SECRET_NAME -- <command>`
- Never run combinatorial planning on raw PR universe; always pre-filter first
- Never store GitHub PAT in SQLite or config files; only runtime env
- Never let swarm workers mutate GitHub directly; only the central executor may write after policy + preflight
- Never treat a v1.x `plan` or PDF report as an execution manifest
- Never commit GITHUB_PAT, OPENROUTER_API_KEY, or other secrets
- Never leave `main` red; post-merge verification is mandatory
- Never self-expand task scope without coordinator approval
- Never use port 8080 for prATC — reserved port range is **7400–7500** (default: 7400)
- Historical v0.1 scope excluded GitHub App/OAuth/webhooks, ML feedback loops, multi-repo UI, gRPC, and auto PR actions. v2.0 explicitly reopens GitHub actions only through `VERSION2.0.md`, `GUIDELINE.md`, policy profiles, and centralized executor gates.

## Go Conventions (All internal/ packages)
- Error wrapping: `fmt.Errorf("context: %w", err)` — never bare `err`
- Interfaces: small (1-3 methods), defined at consumption point
- Constructors: `New()` + functional options (`WithX()`) for configurable types
- Tests: table-driven with `t.Run` subtests, no testify/assert
- `init()`: only in `cmd/pratc/` for cobra registration, never in `internal/`
- Sorting: stable + deterministic (PR number tiebreaker everywhere)
- Ports: default API port 7400, reserved range 7400–7500, never 8080

## Scope Guardrails
Must have: rate-limit-aware GitHub client, pre-filter pipeline, dry-run default, audit logging, action-policy enforcement, live preflight before mutation.
Must not have: hidden corpus caps, direct swarm GitHub mutation, un-audited merge/close actions, browser dashboard as required v2.0 surface.

---

# Normative Contracts (v1.1)

## CLI Output Contracts
- `analyze --format=json` → exit `0`, keys: `repo`, `generatedAt`, `counts`, `clusters`, `duplicates`, `overlaps`, `conflicts`, `stalenessSignals`
- `cluster --format=json` → exit `0`, keys: `repo`, `generatedAt`, `model`, `thresholds`, `clusters`
- `graph --format=dot` → exit `0`, non-empty DOT with `digraph`
- `plan --target=N --format=json` → exit `0`, keys: `repo`, `generatedAt`, `target`, `candidatePoolSize`, `strategy`, `selected`, `ordering`, `rejections`
- `actions --format=json` → v2.0 target exit `0`, keys: `repo`, `generatedAt`, `policy_profile`, `lanes`, `work_items`, `action_intents`, `audit`
- `serve` → exit `0` on shutdown, `/healthz` returns `200` with `{"status":"...", "version":"..."}`
- Exit codes: `2` = invalid args, `1` = runtime failure
- Runtime SLOs (5.5k PRs, warm cache): analyze ≤300s, cluster ≤180s, graph ≤120s, plan ≤90s

## SQLite Migration Policy
- `schema_migrations` table: `version INTEGER PRIMARY KEY`, `name TEXT`, `applied_at TEXT`
- Forward-only, idempotent migrations. No destructive down-migrations.
- `user_version` pragma must match latest migration. Fail fast if DB is newer than binary.
- Required tables: `pull_requests`, `pr_files`, `pr_reviews`, `ci_status`, `sync_jobs`, `sync_progress`, `merged_pr_index`
- Test: verify upgrade path from N-2, N-1, and fresh DB to N

## GitHub Rate-Limit Policy
- Reserve ≥200 requests/hour; pause when crossed
- Primary GraphQL with REST fallback (REST not yet implemented)
- 403 secondary-rate-limit: exponential backoff + jitter, 2s→60s, max 8 retries
- 5xx/network: exponential backoff + jitter, 1s→30s, max 6 retries
- Rate limit exhaustion: persist cursor, sleep until reset +15s, resume

## Performance SLOs (5.5k PR Scale)
- Cold sync ≤20min, warm refresh ≤3min, plan ≤90s
- API p95: /analyze ≤5s, /cluster ≤3s, /graph ≤2s, /plan ≤2s
- Memory ceiling: ≤2.5 GB RSS for CLI analyze

## Telemetry Contract (not yet implemented — `internal/telemetry/` is empty)
- Sync: `sync_jobs_started_total`, `sync_jobs_completed_total`, `sync_jobs_failed_total`, `sync_job_duration_seconds`
- API: `api_requests_total`, `api_errors_total`, `api_request_duration_seconds` (route+status labels)
- Rate-limit: `github_rate_remaining`, `github_rate_reset_epoch`, `github_secondary_limit_events_total`
- All logs: `timestamp`, `level`, `component`, `repo`, `job_id`, `correlation_id`

## Worktree & Mainline Safety
- Work not merged to `main` is incomplete. Post-merge: run `make build && make test` on `main`.
- Merge reports: branch name, commit hash, test commands, pass/fail status.

## Agent Operating Model
- `.sisyphus/plans/` = source-of-truth plan. `AGENTS.md` = execution contract. Tension → follow `AGENTS.md`.
- Coordinator → worker → integrator model. One worktree per task.
- Task completion: code + tests + evidence + merge to main + passing verification.
- Evidence under `.sisyphus/evidence/task-*-*.txt|md|png`. Status under `.sisyphus/status/`.

### Subagent delegation policy
- Prefer Hermes-native delegation first. Do not switch to Codex or another external ACP lane unless the user explicitly asks for that lane.
- Default path: use Hermes subagent logic / inherited Hermes transport first; keep the controller on Hermes and the children on Hermes.
- Before any parallel wave, run a single trivial Hermes child as a delegation preflight.
- Treat these as delegation-lane failures, not task failures: child `api_calls: 0`, immediate interrupt, model mismatch, or no repo-side evidence of execution.
- If Hermes ACP is needed, use Hermes ACP (`hermes acp`) and Hermes command flags; do not substitute a different ACP binary by default.
- If Hermes ACP is unavailable or unhealthy, fall back in this order:
  1. sequential Hermes-native child dispatch
  2. shell-driven Hermes CLI child execution
  3. pure controller-local execution
- After a delegation-lane failure, stop the batch, record the RCA, and avoid retrying the same broken lane blindly.
- Do not update local delegation doctrine after a failure until the operator/user has confirmed the intended Hermes-first policy.
