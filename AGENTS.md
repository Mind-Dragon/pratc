# AGENTS.md — prATC Execution Guide

## Mission
Build **prATC (PR Air Traffic Control)**: a self-hostable, repo-agnostic system for large-scale PR triage and merge planning.

Deliverables:
- Go CLI: `pratc` with `analyze`, `cluster`, `graph`, `plan`, `serve`
- Python ML service: clustering, duplicate detection, overlap heuristics
- TypeScript web dashboard: ATC view + Outlook-style triage inbox
- Docker Compose stack for self-hosting
- SQLite cache + incremental GitHub sync
- Formula engine: `P(n,k)`, `C(n,k)`, `n^k`
- Dependency graph engine: topo sort + DOT output

## Scope Guardrails
Must have:
- Rate-limit-aware GitHub client
- In-process background sync with persisted progress/resume
- Pre-filter pipeline before combinatorial planning: cluster -> CI -> conflict -> score
- Branch-aware analysis universes
- Dry-run default for all actions + audit logging

Must not have in v0.1:
- GitHub App/OAuth/webhooks
- ML feedback loops
- Multi-repo aggregate UI
- gRPC
- Automatic PR action execution
- Nx/Turborepo or JS monorepo tooling

## Stack
- Go 1.23+
- Python 3.11+ with `uv`
- Node 20+ + `bun`
- Docker + Compose
- `psst` for secrets

Secrets rule:
- Never read raw secret values.
- Always execute secret-dependent commands as:
  - `psst SECRET_NAME -- <command>`

## Build and Test Contract
- TDD required: tests fail first, then implementation.
- Primary test commands:
  - `go test -race -v ./...`
  - `uv run pytest -v`
  - `bun run test` (vitest)
- Unified checks:
  - `make verify-env`
  - `make build`
  - `make test`

## Command Behavior Targets
- `pratc analyze --repo=owner/repo`: categories, clusters, duplicates, conflicts
- `pratc cluster --repo=owner/repo`: cluster assignments + similarity
- `pratc graph --repo=owner/repo`: dependency/conflict DOT graph
- `pratc plan --repo=owner/repo --target=20`: ranked merge plan
- `pratc serve --port=8080`: API server mode

## UI Targets
- Dashboard runs at `localhost:3000`
- API runs at `localhost:8080`
- Include:
  - ATC overview
  - Outlook-style sequential triage
  - Interactive dependency graph
  - Merge plan panel

## Execution Order (Critical Path)
0 -> 1 -> 3 -> 13 -> 14 -> 18 -> 19 -> 20 -> Final verification

Parallel waves:
- Wave 0: toolchain/bootstrap
- Wave 1: scaffolding/foundations
- Wave 2: formula/graph/ML/filter engines
- Wave 3: CLI integration commands
- Wave 4: dashboard implementation
- Wave 5: docker/polish/edge cases/docs
- Final: compliance, quality, QA, scope-fidelity audits

## Agent Operating Model
- `.sisyphus/plans/pratc.md` is the source-of-truth execution plan. Agents may summarize it, but must not silently diverge from its task graph, acceptance criteria, or scope guardrails.
- `AGENTS.md` is the execution contract. When the plan and implementation workflow are in tension, agents must follow `AGENTS.md` for safety, testing, merge discipline, and reporting.
- Use a coordinator -> worker -> integrator model.
- Coordinator responsibilities:
  - read the current plan in `.sisyphus/plans/`
  - dispatch only dependency-ready tasks
  - assign one owner per task
  - track task state under `.sisyphus/` artifacts
  - block out-of-order work that violates the dependency graph
- Worker responsibilities:
  - own exactly one task or one tightly-coupled task bundle
  - work in an isolated worktree or feature branch
  - modify only the files required for that task
  - add or update tests before implementation where applicable
  - produce evidence under `.sisyphus/evidence/`
- Integrator responsibilities:
  - merge completed work into `main`
  - run required post-merge verification on `main`
  - fix forward immediately if `main` fails
  - publish merge/test status back into `.sisyphus/` artifacts
- Review/QA agent responsibilities:
  - verify plan compliance
  - verify regression risk and test coverage
  - verify evidence files match the claimed outcomes

### Task Dispatch Rules
- One worktree per task by default. Only bundle tasks when they share a tight dependency and overlapping file ownership is unavoidable.
- Every dispatched task must include:
  - task id and exact title from the plan
  - dependencies already satisfied
  - owned files or subsystem boundaries
  - acceptance criteria copied from the plan
  - required tests and evidence artifacts
  - merge-to-`main` and test-on-`main` completion rule
- Workers must not self-expand scope to adjacent tasks without explicit coordinator approval.
- If a task reveals a plan defect, the agent must record it explicitly and either patch the plan or escalate before continuing.

### State and Artifact Layout
- Store plan files under `.sisyphus/plans/`.
- Store QA logs, screenshots, command outputs, and validation artifacts under `.sisyphus/evidence/`.
- Store task state snapshots under `.sisyphus/status/`.
- Store reusable worker prompts under `.sisyphus/prompts/`.
- Minimum task state values: `todo`, `in_progress`, `blocked`, `done`, `merged`, `verified`.

### Definition of Task Completion
- A worker task is complete only when:
  - code and tests are implemented
  - evidence artifacts are written
  - the branch/worktree is merged to `main`
  - verification passes on `main`
- Returning "implemented" without merge and post-merge verification is incomplete work.

## Data and Model Policy
- Never run combinatorial planning on full raw PR universe.
- Always reduce candidate set first.
- Duplicate thresholds:
  - `> 0.90` similarity => duplicate
  - `0.70 - 0.90` => overlapping

## Done Criteria
Project is done only when all are true:
- CLI commands above return expected structured outputs
- Compose profiles both work:
  - `local-ml`
  - `minimax-light`
- Web + API are reachable and integrated
- `make test` passes across Go/Python/Web
- Formula engine values validated against MAG40 calculations

## Execution v1.1 (Normative Contracts)

### CLI Output Contracts
- `pratc analyze --repo=owner/repo --format=json` must return exit code `0` and valid JSON containing keys: `repo`, `generatedAt`, `counts`, `clusters`, `duplicates`, `overlaps`, `conflicts`, `stalenessSignals`.
- `pratc cluster --repo=owner/repo --format=json` must return exit code `0` and keys: `repo`, `generatedAt`, `model`, `thresholds`, `clusters`.
- `pratc graph --repo=owner/repo --format=dot` must return exit code `0`, non-empty DOT text, and include at least one `digraph` declaration.
- `pratc plan --repo=owner/repo --target=N --format=json` must return exit code `0` and keys: `repo`, `generatedAt`, `target`, `candidatePoolSize`, `strategy`, `selected`, `ordering`, `rejections`.
- `pratc serve --port=8080` must return exit code `0` on healthy shutdown and expose `/healthz` returning HTTP `200` with JSON keys `status` and `version`.
- All CLI commands must return exit code `2` for invalid args and exit code `1` for runtime failures.
- Maximum runtime SLOs on fixture dataset (`~5,500` open PRs): `analyze <= 300s`, `cluster <= 180s`, `graph <= 120s`, `plan <= 90s` after cache warmup.
- Duplicate classification thresholds are fixed for v0.1: `similarity > 0.90 => duplicate`, `0.70 <= similarity <= 0.90 => overlapping`.

### SQLite Schema and Migration Policy
- Database must include a `schema_migrations` table with columns: `version INTEGER PRIMARY KEY`, `name TEXT`, `applied_at TEXT`.
- Every migration must be forward-only and idempotent when re-run in a no-op context.
- Required baseline tables: `pull_requests`, `pr_files`, `pr_reviews`, `ci_status`, `sync_jobs`, `sync_progress`, `merged_pr_index`.
- `user_version` pragma must match latest applied migration version.
- Startup behavior: fail fast if DB version is newer than binary-supported version.
- Rollback policy: no destructive down-migrations in production; recovery is restore-from-backup plus forward reapply.
- Test policy: fixture-based migration tests must verify upgrade path from `N-2`, `N-1`, and fresh DB to `N`.

### GitHub Rate-Limit and Retry Policy
- Request budgeting must keep a reserve of `>= 200` requests per hour; background sync pauses when reserve is crossed.
- Primary GraphQL path with REST fallback for unsupported fields or GraphQL transient failures.
- On `403` secondary-rate-limit responses: backoff with exponential delay and jitter, starting at `2s`, capped at `60s`, max `8` retries.
- On network/5xx errors: retry with exponential backoff and jitter, starting at `1s`, capped at `30s`, max `6` retries.
- On primary rate limit exhaustion: persist cursor/progress, sleep until reset plus `15s` safety margin, then resume.
- All retries must be logged with reason, attempt, wait duration, and request correlation id.

### Performance SLOs for 5.5k PR Scale
- Initial incremental sync (cold cache) target: `<= 20 min` for one repository with `~5,500` open PRs.
- Incremental refresh (warm cache, <5% changed PRs) target: `<= 3 min`.
- Planner candidate generation and ordering target: `<= 90s` for `target=20`.
- API p95 latency targets on warm cache: `/analyze <= 5s`, `/cluster <= 3s`, `/graph <= 2s`, `/plan <= 2s`.
- Memory ceiling for CLI analyze run on fixture scale: `<= 2.5 GB RSS`.

### Minimum Telemetry Contract
- Sync job metrics: `sync_jobs_started_total`, `sync_jobs_completed_total`, `sync_jobs_failed_total`, `sync_job_duration_seconds`.
- API reliability metrics: `api_requests_total`, `api_errors_total`, `api_request_duration_seconds`, with route and status labels.
- Rate-limit metrics: `github_rate_remaining`, `github_rate_reset_epoch`, `github_secondary_limit_events_total`.
- Queue/progress metrics: `sync_queue_depth`, `sync_cursor_age_seconds`, `sync_staleness_seconds`.
- Planner trace log per run must include: pool size, filter drop counts by stage, chosen strategy, final ordering rationale.
- All structured logs must include `timestamp`, `level`, `component`, `repo`, `job_id` (if present), and `correlation_id`.

### Worktree Integration and Mainline Safety Policy
- Any work completed in a worktree/feature branch is not done until it is merged into `main`.
- After merging to `main`, agents must run the full verification gate on `main` before declaring success:
  - `make build`
  - `make test`
  - ecosystem checks as applicable (`go test -race -v ./...`, `uv run pytest -v`, `bun run test`)
- If post-merge tests fail, agents must immediately fix forward on `main` and re-run the full verification gate.
- Agents must not leave `main` red. Task completion requires a passing test state on `main`.
- Merge reports must include:
  - merged branch/worktree name
  - merge commit/hash
  - test commands executed
  - final pass/fail status

## Agent Working Rules
- Keep changes small, atomic, and test-backed.
- Preserve v0.1 scope; defer v0.2+ items explicitly.
- Prefer deterministic, inspectable outputs (JSON, DOT, logs).
- Record QA evidence under `.sisyphus/evidence/task-*-*.txt|md|png`.
