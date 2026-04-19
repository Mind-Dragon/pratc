# prATC Architecture

## Purpose

This document describes the system shape for the v1.5.0 full-corpus triage engine.

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
   - score substance (security, reliability, performance, roadmap alignment)
   - route now versus future
   - apply deeper judgment layers (confidence through signal quality)
   - existing components: internal/filter/ (pre-filter pipeline), internal/planning/ (pool selector, hierarchy, pairwise, time decay), internal/review/ (security, reliability, quality analyzers), internal/analysis/ (bot detection)

3. Reason ledger
   - store the reason trail for every PR
   - record bucket changes over time
   - keep confidence, evidence, and explanation together
   - implementation: extends the existing audit log (internal/audit/) with per-PR bucket and reason tracking

4. Output composer
   - build the full report (internal/report/ — PDF generation)
   - separate now, future, duplicate, junk, blocked, and risk buckets
   - surface both summary and detail
   - full appendix for complete corpus coverage

5. Operator interface
   - web dashboard (web/ — Next.js 15 + React 19)
   - let humans inspect the corpus through layers
   - make priority changes visible
   - preserve auditability

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
Layer 2: duplicate collapse (internal/ml/ → Python ML service)
    │
    ▼
Layer 3: obvious badness (spam/malware/junk classification)
    │
    ▼
Layer 4: substance scoring (internal/review/ analyzers)
    │
    ▼
Layer 5: now vs future routing
    │
    ▼
Layers 6–16: deep judgment (confidence, dependency, blast radius,
             leverage, ownership, stability, mergeability,
             strategic weight, attention cost, reversibility,
             signal quality)
    │
    ▼
Pool selection + planning (internal/planning/ + internal/formula/)
    │
    ▼
Report composition (internal/report/)
    │
    ▼
Operator view (web/ dashboard, CLI output, PDF)
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
- security (extends internal/review/ security analyzer)
- reliability (extends internal/review/ reliability analyzer)
- performance (extends internal/review/ quality analyzer)
- roadmap alignment (new)
- current vs future priority (new)

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

The report must make the full corpus understandable in layers:
- executive summary
- now queue
- future queue
- duplicate chains
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
- `maxPRs` cap of 5,000 applied to the overnight openclaw/openclaw run — full corpus is ~6,632 PRs; need to remove or raise the cap
- Conflict pairs at 38,884 after noise filtering — still above the 5,000 target; needs further noise file expansion or higher shared-file minimum
- Duplicate groups at 9 — may be genuine corpus behavior or a signal that file-overlap weighting needs tuning
- Legacy pool-cap constants still exist in `internal/types/`, but `BuildCandidatePool()` does not enforce them in the active runtime path
- `ListPRs()` in `internal/cache/sqlite.go` now exposes caller-visible paging/streaming via `PRPage` and `ListPRsIter()`; callers no longer need to materialize the full slice
- Bootstrap sync in `internal/sync/worker.go` now streams into the cache store

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
Plan(ctx context.Context, repo string, target int, mode formula.Mode) (*PlanResponse, error)
Health() *HealthResponse
```

### Shared thresholds and defaults

- Duplicate threshold: 0.85 (lowered from 0.90 in v1.5 — scoring formula maxes at 0.85)
- Overlap threshold: 0.70
- Default target: 20
- Default candidate pool cap: 100
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

### Production run results (openclaw/openclaw, 2026-04-19)

- 4,992 PRs analyzed (capped at 5,000; full corpus is ~6,632)
- 69 clusters, 9 duplicate groups, 741 overlap groups
- 38,884 conflict pairs (reduced from 92,911 by v1.5 noise filtering; still above target of 5,000)
- 215 stale PRs, 8 garbage PRs
- Full sync + analyze pipeline: 28.5 min (first run, no cache)
- Intermediate cache now stores duplicate groups, conflicts, and substance scores; second run skips O(n^2) recomputation

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
- **version1.4.2.md** is the v1.4.2 milestone summary (shipped). See CHANGELOG.md for v1.5.0 changes.
