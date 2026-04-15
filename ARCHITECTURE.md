# prATC Architecture

## Purpose

This document describes the system shape for the v1.4 full-corpus triage engine.

The point is not to make the system clever in one shot. The point is to make it honest, layered, and complete: every PR enters, every decision is explainable, and the system keeps separating the obvious from the subtle until what remains is worth human attention.

For detailed component breakdowns, API routes, data model, SLOs, and technology stack, see [docs/techref.md](docs/techref.md).

## System shape

The system has five broad parts:

1. Corpus ingestion
   - load every open PR
   - normalize metadata
   - persist the full corpus in SQLite (internal/cache/)
   - keep the corpus available for repeated passes

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
- `maxPRs` default of 1000 in `internal/cmd/analyze.go` — must be removed or made explicit
- `DefaultPoolCap = 100` in `internal/types/constants.go` — must become configurable or be removed as a hard gate
- `ListPRs()` in `internal/cache/sqlite.go` has no pagination — needs cursor-based pagination for 6k+ corpora
- Bootstrap sync in `internal/sync/worker.go` loads all PRs into memory — needs streaming insert

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
- **docs/techref.md** has the detailed component breakdowns, API routes, data model, and SLOs.
- **This document** defines the system shape, data flow, and design philosophy.
