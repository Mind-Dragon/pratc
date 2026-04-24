# prATC Architecture

## Purpose

This document describes the system shape for the v1.x full-corpus triage engine and the v2.0 action engine.

The point is not to make the system clever in one shot. The point is to make it honest, layered, complete, and safe: every PR enters, every decision is explainable, every action is policy-bound, and the system keeps separating the obvious from the subtle until what remains is ready for a human, a swarm worker, or the executor.

For the v2.0 implementation plan, see `VERSION2.0.md`. For detailed component breakdowns, API routes, data model, SLOs, and technology stack, see the technical reference section in this document.

## System shape

The system has seven broad parts:

1. Corpus ingestion
   - load every open PR
   - normalize metadata
   - persist the full corpus in SQLite (internal/cache/)
   - keep the corpus available for repeated passes
   - treat the first sync as the source-of-truth snapshot and reuse it locally for later phases

2. Layered decision engine
   - run cheap outer layers first (garbage, duplicates, obvious badness)
   - record an explicit 16-gate journey for every reviewed PR
   - score substance, route now versus future, and apply deeper judgment only after the outer peel
   - consume diff-grounded evidence where available without making the outer peel expensive
   - emit duplicate-synthesis plans that nominate a best candidate for future merge-by-bot work
   - existing components: internal/filter/ (pre-filter pipeline), internal/planning/ (pool selector, hierarchy, pairwise, time decay), internal/review/ (security, reliability, quality analyzers), internal/analysis/ (bot detection)
   - duplicate detection now defaults to a Go cache-first path with MinHash/LSH candidate generation plus exact rescoring; the Python ML service remains optional for enrichment/alternate backends

3. Action engine
   - convert full-corpus review output into action lanes and ActionIntents
   - assign every PR exactly one primary action lane
   - enforce policy profiles (`advisory`, `guarded`, `autonomous`)
   - produce `action-plan.json` for swarm workers and external agents
   - planned ownership: new internal/actions/ package plus service/API/CLI wiring

4. Reason and action ledger
   - store the reason trail for every PR
   - record gate journey, bucket changes, action lane, confidence, evidence, and explanation together
   - keep diff-grounded findings and duplicate-synthesis recommendations visible in machine-readable form
   - record every action state transition, preflight, denial, executor action, and verification result
   - implementation: extends internal/audit/ and internal/cache/ with action work items, leases, proof bundles, and ledger tables

5. Swarm work queue
   - expose claimable work items by lane
   - lease work to swarm agents with heartbeat/expiry semantics
   - accept proof bundles from workers
   - keep swarm workers away from direct GitHub mutation
   - planned ownership: internal/workqueue/ or internal/actions queue helpers

6. Output composer and dashboard surfaces
   - build the full snapshot report (internal/report/ — PDF generation)
   - expose live terminal dashboard state (internal/monitor/tui/)
   - separate now, future, duplicate, junk, blocked, risk, and action lanes
   - surface both summary and detail
   - full appendix / navigable PR detail for complete corpus coverage

7. Operator and executor interfaces
   - CLI for humans running the tool directly
   - API for AI systems, swarm workers, and executor clients
   - TUI for live action-lane dashboard, work queue, preflight/executor stream, and audit ledger
   - PDF report for point-in-time human decision packets
   - centralized executor for all GitHub mutations after live preflight

## Data flow

```
GitHub API
    │
    ▼
Corpus ingestion (internal/sync/ + internal/github/)
    │
    ▼
Normalize + store (internal/cache/ — SQLite)
    │
    ▼
Layer 1: garbage detection (internal/analysis/ + new classifiers)
    │
    ▼
Layer 2: duplicate collapse (Go MinHash/LSH candidate generation + exact rescoring; optional ML bridge)
    │
    ▼
Layer 3: obvious badness (spam/malware/junk classification)
    │
    ▼
Layer 4: substance scoring + diff-grounded evidence (internal/review/ analyzers)
    │
    ▼
Layer 5: now vs future routing
    │
    ▼
Layers 6–16: explicit gate journey + deep judgment (confidence, dependency, blast radius,
             leverage, ownership, stability, mergeability,
             strategic weight, attention cost, reversibility,
             signal quality)
    │
    ▼
Pool selection + planning (internal/planning/ + internal/formula/)
    │
    ▼
Action lane classification + policy gating (internal/actions/)
    │
    ▼
ActionPlan + swarm work queue (action-plan.json, leases, proof bundles)
    │
    ├──► TUI dashboard (live lanes, queue, executor, audit stream)
    │
    ├──► PDF snapshot report (point-in-time packet)
    │
    └──► Central executor (policy + live preflight + audit ledger)
             │
             ▼
        GitHub mutation only when allowed
```

That flow is intentional.

- Nothing bypasses ingestion.
- Nothing gets removed without a reason.
- Nothing is treated as final until it has passed the relevant layers.
- Heavy judgment is reserved for work that has already survived the early peel.

## Layer groups

### 1. Outer peel
These layers remove obvious noise before deeper computation:
- garbage
- duplicates
- obvious badness

### 2. Substance assessment
These layers judge whether the PR is good in a meaningful way:
- security (includes diff-grounded risky file and auth-change evidence)
- reliability (metadata-first today, with room for deeper diff evidence later)
- performance (metadata-first today, with room for deeper diff evidence later)
- quality (includes test-gap evidence tied to changed files)
- current vs future priority

### 3. Deep judgment
These layers refine priority and readiness:
- confidence
- dependency
- blast radius
- leverage
- ownership
- stability
- mergeability
- strategic weight
- attention cost
- reversibility
- signal quality

## Output contract

The output surfaces must make the full corpus understandable in layers:
- API responses expose gate journey, evidence, duplicate groups, duplicate-synthesis plans, action lanes, ActionIntents, and queue state for AI consumers.
- TUI is the live dashboard: action lanes, PR detail, queue leases, proof bundles, executor state, rate-limit/auth, and audit stream.
- PDF remains a point-in-time human-facing packet and should summarize the same underlying truth.

The report/dashboard data must make the full corpus understandable in layers:
- executive summary
- action lane board (`fast_merge`, `fix_and_merge`, `duplicate_close`, `reject_or_close`, `focused_review`, `future_or_reengage`, `human_escalate`)
- now queue
- future queue
- duplicate chains
- nominated canonical / synthesis candidates
- junk/noise bucket
- blocked items
- risk-focused items (security, reliability, performance)
- proof bundle and executor status where applicable
- full appendix or navigable PR detail for the rest

No PR may vanish without an explanation. No action may execute without policy, preflight, and audit.

## Scaling rules

The architecture accepts that the corpus may be large.

- All PRs are included.
- Cost can be high.
- Time can be longer.
- The system may batch or shard work internally.
- The system may cache intermediate results.
- The system may prioritize cheap passes before expensive ones.

What it may not do is silently narrow the corpus.

### Known scaling constraints to address
- `ListPRs()` in `internal/cache/sqlite.go` now exposes caller-visible paging/streaming via `PRPage` and `ListPRsIter()`; callers no longer need to materialize the full slice
- Bootstrap sync in `internal/sync/worker.go` now streams into the cache store
- `maxPRs` is an operator-facing cap, not a hidden default; full-corpus autonomous runs must not pass a cap unless truncation is intentionally recorded in run metadata

## v2.0 action architecture

The 2.0 engine adds a second decision product after analysis: `ActionPlan`.

`ActionPlan` is not a top-N merge list. It is the full-corpus action map consumed by the TUI, API, swarm workers, and executor.

Required concepts:

- policy profile: `advisory`, `guarded`, `autonomous`
- action lane: one primary lane per PR
- action intent: typed possible operation with preconditions and evidence
- work item: claimable unit for swarm workers
- proof bundle: worker-produced evidence for `fix_and_merge` or deeper review
- executor ledger: append-only record of preflight, denial, mutation, and verification

Planned package ownership:

- `internal/actions/` — lane classifier, policy gates, ActionPlan builder
- `internal/workqueue/` — durable claims, leases, heartbeat, proof bundle association
- `internal/executor/` — dry-run executor, live preflight, guarded/autonomous mutation path
- `internal/monitor/tui/` — live action dashboard panels
- `internal/cache/` — migrations for action plans, work items, leases, proof bundles, and ledger
- `internal/cmd/` — `actions` CLI and HTTP routes

Swarm workers are clients of the queue and proof API. They do not hold GitHub mutation authority.

## Autonomous runtime architecture

Autonomous mode is the control loop around the product runtime. It does not replace the CLI/API/TUI/PDF surfaces; it drives them, audits their artifacts, converts failures into stable gaps, and dispatches implementation waves.

### Control plane

- `AUTONOMOUS.md` defines loop policy and stop conditions.
- `autonomous/STATE.yaml` is the resume checkpoint. It records repo, branch, baseline commit, current run, current phase, open gaps, blocked gaps, last audit, and last green commit.
- `autonomous/GAP_LIST.md` is the current failure surface generated from audit output.
- `autonomous/RUNBOOK.md` contains exact bootstrap, run, audit, gap, resume, pause, and closeout commands.
- `scripts/autonomous_controller.py` manages state, wave synthesis, resume, closeout, and consistency checks.
- `scripts/audit_guideline.py` evaluates run artifacts against `GUIDELINE.md`.
- `scripts/gap_list_from_audit.py` promotes required audit failures into stable gap entries.

### Run artifact layout

Controller-owned runs live under `autonomous/runs/<run-id>/` and should contain:

```text
controller-log.md
wave-summary.md
run-metadata.yaml
subagent-results/
analyze.json
step-3-cluster.json
step-4-graph.json
step-5-plan.json
report.pdf
AUDIT_RESULTS.json
```

`run-metadata.yaml` records the commit, binary path, binary banner/version, service health probe or explicit service-skip reason, corpus source, cache path, settings path, and whether any PR cap was applied.

### State invariants

- `corpus_dir` must exist before audit or gap generation.
- `last_audit_path` must exist before gap generation, fix waves, or complete.
- `last_green_commit` may equal `baseline_commit` only after tests and a current-HEAD audit pass, unless an explicit artifact compatibility note is recorded.
- Full-corpus autonomous runs must not pass `--max-prs` unless truncation is intentional and recorded in run metadata.
- Manual audit checks cannot silently satisfy `SUCCESS`; they must either be converted to machine checks or accepted explicitly in `wave-summary.md`.

### Wave model

Open gaps are synthesized into ordered implementation waves:

1. data model / type surface
2. core decision logic
3. wiring / artifact flow / report population
4. verification and doc sync

The controller remains responsible for state and verification. Subagents implement or review isolated gaps, but the controller reruns tests, product commands, and audits before changing durable state.

### Runtime proof

Autonomous runtime readiness requires a fresh binary and, when API readiness is claimed, a live health probe. CLI-only autonomous runs may skip `serve`, but the skip reason must be recorded in run metadata.

## Technical reference

This section keeps the concrete routes, command surface, and service contracts close to the architecture they support.

### CLI surface

| Command | Purpose | Key flags |
|---------|---------|-----------|
| `analyze` | Full PR analysis | `--repo`, `--format`, `--use-cache-first` |
| `cluster` | ML clustering only | `--repo`, `--format` |
| `graph` | Dependency graph | `--repo`, `--format` (dot/json) |
| `plan` | Merge planning | `--repo`, `--target`, `--mode`, `--dry-run` |
| `actions` | Full-corpus action lane plan | `--repo`, `--policy`, `--lane`, `--format`, `--dry-run` |
| `report` | Generate PDF report | `--repo`, `--input-dir`, `--output`, `--format` |
| `monitor` | TUI dashboard | action lanes, queue, executor, audit stream |
| `serve` | Start API server | `--port`, `--repo` |
| `sync` | GitHub sync | `--repo`, `--watch`, `--interval` |
| `audit` | Query audit log | `--limit`, `--format` |
| `mirror` | Git mirror management | `list`, `info`, `prune`, `clean` |

### HTTP routes

RESTful routes:

GET  /api/repos/{owner}/{repo}/analyze
GET  /api/repos/{owner}/{repo}/cluster
GET  /api/repos/{owner}/{repo}/graph
GET  /api/repos/{owner}/{repo}/plan
GET  /api/repos/{owner}/{repo}/actions
POST /api/repos/{owner}/{repo}/actions/claim
POST /api/repos/{owner}/{repo}/actions/{work_item_id}/proof
POST /api/repos/{owner}/{repo}/actions/{work_item_id}/execute
POST /api/repos/{owner}/{repo}/sync
GET  /api/repos/{owner}/{repo}/sync/stream

Legacy routes:

GET /analyze?repo=owner/repo
GET /cluster?repo=owner/repo
GET /graph?repo=owner/repo&format=dot
GET /plan?repo=owner/repo&target=20

### Service contract

The service facade exposes the primary app operations:

```go
Analyze(ctx context.Context, repo string) (*AnalysisResponse, error)
Cluster(ctx context.Context, repo string) (*ClusterResponse, error)
Graph(ctx context.Context, repo string) (*GraphResponse, error)
PlanWithOptions(ctx context.Context, repo string, opts PlanOptions) (*PlanResponse, error)
Actions(ctx context.Context, repo string, opts ActionOptions) (*ActionPlanResponse, error) // v2.0 target
Health() *HealthResponse

// PlanOptions controls the planning behavior.
type PlanOptions struct {
    Target               int
    Mode                 formula.Mode
    IncludeBots          bool
    ScoreMin             float64
    StaleDays            int
    StaleScoreThreshold  float64
    CandidatePoolCap     int
    ConflictFilterMode   string
}
```

### Shared thresholds and defaults

- Duplicate threshold: 0.85 (lowered from 0.90 in v1.5 — scoring formula maxes at 0.85)
- Overlap threshold: 0.70
- Default target: 20
- Default candidate pool cap: legacy constant only; not enforced by `BuildCandidatePool()`
- Max target: 1000
- Plan dry-run default: true
- Default deep candidate subset size: 64

### Core packages

- `internal/filter/` — pre-filter pipeline, scoring, and rejection logic
- `internal/graph/` — dependency and conflict graph construction
- `internal/planner/` — functional-options planner implementation
- `internal/formula/` — combinatorial counting and generation
- `internal/github/` — rate-limited GitHub GraphQL client
- `internal/ml/` — JSON stdin/stdout bridge to the Python ML service
- `internal/sync/` — incremental sync, bootstrap, and drift handling

### Data layer

- SQLite with forward-only migrations
- Schema version 7 (adds duplicate_groups, conflict_cache, substance_cache tables; repo normalization migration)
- Pragmas on open: WAL, busy_timeout, foreign_keys

### Sync and rate limiting

- Sync jobs track open PRs, closed PRs, cursor position, and snapshot ceiling
- Bootstrap can stream directly into the cache store
- Later phases should read from local SQLite and artifacts first, not refetch the same corpus from GitHub
- GitHub rate limiting keeps a reserve budget and backs off on secondary limits
- Rate-limit status checks should be throttled so they do not become a hidden per-request tax

### Efficiency rules

- Sync once, then reuse the local snapshot for repeated analysis passes
- Treat unchanged PRs as cache hits, not fresh fetches
- Keep downstream phases artifact-driven whenever possible
- Prefer delta refreshes over full corpus re-downloads
- Avoid overlapping workflow and analyze runs for the same repository

### Performance SLOs

- Analyze: 300s
- Cluster: 180s
- Graph: 120s
- Plan: 90s

### Current full-corpus validation status (openclaw/openclaw, 2026-04-23)

- v1.7.1 cache-first workflow: audit-green (`19` pass, `0` fail, `0` manual)
- Full corpus analyzed: `6,632` PRs
- Cluster count: `81`
- Duplicate groups: `95`
- Collapsed duplicate groups: `85`
- Conflict pairs after current filtering: `91`
- Garbage PRs: `14`
- Stale PRs: `4,248`
- Snapshot PDF: `projects/openclaw_openclaw/runs/v171-analysis-20260423T234148Z/report.pdf`
- Runtime proof: `prATC 1.7.1`, API health green on port `7400`

The v1.7.1 PDF is a point-in-time packet. The 2.0 dashboard should expose those same concepts as live navigable state backed by `ActionPlan`, queue, executor, and audit data.

## Key design principle

prATC is not a single ranking function.
It is a layered decision and action-routing system.

That matters because different questions live at different depths:
- Is this junk?
- Is this duplicate?
- Is this worth trust?
- Is this worth time now?
- Is this worth time later?
- Is this worth a human at all?
- Is this safe for a swarm to fix?
- Is this safe for the executor to merge, close, comment, or reject?

The architecture exists to answer those questions without flattening them into one number.

## Relationship to other documents
- **GUIDELINE.md** is the authority on bucket definitions, layer ordering, and non-negotiables.
- **ROADMAP.md** defines what gets built and when.
- **VERSION2.0.md** defines the active action-engine plan and 16-developer swarm execution map.
- **This document** defines the system shape, data flow, technical reference details, and design philosophy.
- **version1.4.2.md** is the v1.4.2 milestone summary (shipped). See CHANGELOG.md for v1.6.0 and later changes.
