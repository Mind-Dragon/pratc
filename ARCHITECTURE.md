# prATC Architecture

## Purpose

This document describes the system shape for the v1.7.0 full-corpus triage engine.

The point is not to make the system clever in one shot. The point is to make it honest, layered, and complete: every PR enters, every decision is explainable, and the system keeps separating the obvious from the subtle until what remains is worth human attention.

For the detailed component breakdowns, API routes, data model, SLOs, and technology stack, see the technical reference section in this document.

## System shape

The system has five broad parts:

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

3. Reason ledger
   - store the reason trail for every PR
   - record gate journey, bucket changes, confidence, evidence, and explanation together
   - keep diff-grounded findings and duplicate-synthesis recommendations visible in machine-readable form
   - implementation: extends the existing audit log (internal/audit/) with per-PR bucket and reason tracking

4. Output composer
   - build the full report (internal/report/ — PDF generation)
   - separate now, future, duplicate, junk, blocked, and risk buckets
   - surface both summary and detail
   - full appendix for complete corpus coverage

5. Operator interface
   - CLI for humans running the tool directly
   - API for AI systems and external integrations
   - PDF report for human decision-makers

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
Duplicate synthesis planning (best-of-group nomination, advisory-only)
    │
    ▼
Report composition (internal/report/)
    │
    ▼
Operator view (CLI output, API responses, PDF report)
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
- API responses expose gate journey, evidence, duplicate groups, and duplicate-synthesis plans for AI consumers.
- PDF remains the human-facing packet and should summarize the same underlying truth.

The report must make the full corpus understandable in layers:
- executive summary
- now queue
- future queue
- duplicate chains
- nominated canonical / synthesis candidates
- junk/noise bucket
- blocked items
- risk-focused items (security, reliability, performance)
- full appendix for the rest

No PR may vanish without an explanation.

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

## Autonomous runtime architecture

Autonomous mode is the control loop around the product runtime. It does not replace the CLI/API/PDF surfaces; it drives them, audits their artifacts, converts failures into stable gaps, and dispatches implementation waves.

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
| `report` | Generate PDF report | `--repo`, `--input-dir`, `--output`, `--format` |
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

### Current full-corpus validation status (openclaw/openclaw, 2026-04-21)

- Cache-first full workflow: audit-green (`17` pass, `0` fail, `2` manual)
- Explicit live validation (`--refresh-sync --force-live`): audit-green (`17` pass, `0` fail, `2` manual)
- Full corpus analyzed: `6,632` PRs
- Duplicate groups: `95`
- Overlap groups: `0`
- Conflict pairs after noise filtering: `0`
- Garbage PRs: `14`
- Workflow artifact contract complete: `sync.json`, `analyze.json`, step-numbered artifacts, and `report.pdf`
- Duplicate detection on large sparse corpora now uses MinHash/LSH candidate generation with exact rescoring; the 6k sparse synthetic benchmark sits around `~90ms/op` instead of `~21s/op` for exact pairwise comparison

## Key design principle

prATC is not a single ranking function.
It is a layered decision system.

That matters because different questions live at different depths:
- Is this junk?
- Is this duplicate?
- Is this worth trust?
- Is this worth time now?
- Is this worth time later?
- Is this worth a human at all?

The architecture exists to answer those questions without flattening them into one number.

## Relationship to other documents
- **GUIDELINE.md** is the authority on bucket definitions, layer ordering, and non-negotiables.
- **ROADMAP.md** defines what gets built and when.
- **This document** defines the system shape, data flow, technical reference details, and design philosophy.
- **version1.4.2.md** is the v1.4.2 milestone summary (shipped). See CHANGELOG.md for v1.6.0 changes.
