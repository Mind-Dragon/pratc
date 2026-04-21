# Roadmap

prATC development roadmap.

## Version 1.4.2 — Full-Corpus Triage Engine (COMPLETED)

**Shipped 2026-04-16.** Full-corpus PR triage with layered decision reasoning, analyst-packet reporting, and managed background sync.

- Local-first sync with explicit sync ceilings and resumable checkpoints
- Rate-limit pause/restart with auto-resume
- Planning integration (PoolSelector, HierarchicalPlanner, PairwiseExecutor)
- Review pipeline with security/reliability/performance/quality analyzers
- PDF analyst report with PR table, bucket counts, recommendations
- 16-layer decision ladder (documented, partially implemented)
- See `version1.4.2.md` for full details

## Version 1.5 — Triage Engine Fixes + Performance (CODE COMPLETE, CACHE-BACKED + LIVE VALIDATION GREEN)

**Code complete: 2026-04-19. Validation complete: 2026-04-21.** The v1.5 line is now green on both the default cache-first full workflow and one explicit live validation run against `openclaw/openclaw`. The current local release-ready surface is: full artifact contract, audit-green analyze/report output, cache-first by default, and live-path verification without required failures.

### Completed

- [x] Auth passthrough — resolved token now reaches sync worker
- [x] Repo name normalization — case-insensitive, migration v7
- [x] `pratc preflight` command — delta estimate, time, rate limit, lock status
- [x] Singleton lock — per-repo file lock with stale cleanup
- [x] Duplicate threshold — lowered from 0.90 to 0.85
- [x] Conflict filtering — 2+ shared files required, expanded noise list
- [x] Intermediate caching — duplicate groups, conflicts, substance scores cached with corpus fingerprint
- [x] Near-duplicate section in PDF report
- [x] Garbage classifier, conflict noise, deep judgment, pipeline cache tests (50+ tests added)

### Completed cache-backed verification

Cache-backed verification run `projects/openclaw_openclaw/runs/final-wave` now passes the required audit surface:

- [x] bucket coverage
- [x] reason coverage
- [x] confidence coverage
- [x] future work visibility
- [x] temporal routing visibility
- [x] self-describing report/appendix PR rows
- [x] duplicate presence on cache-backed reruns
- [x] dependency edge quality
- [x] conflict pairs below threshold

### Next improvements (if needed after verification)

- [x] Profile duplicate detection hot path with `go test -cpuprofile` / `go tool pprof` and capture the current hotspot surface
- [x] Add MinHash/LSH candidate generation to bound duplicate comparisons on large corpora; `BenchmarkClassifyDuplicatesSparseSimilarity` now runs at ~90ms/op vs `BenchmarkExactDuplicatePairsSparseSimilarity` at ~21s/op on the 6k sparse synthetic benchmark
- [x] Expand noise file list further based on live run results; added OpenClaw-derived generated docs/schema filters (`docs/.generated/*`, `docs/docs.json`, `schema.base.generated.ts`, `schema.help.ts`, `schema.labels.ts`)
- [x] Tune substance scoring weights based on operator feedback / observed output spread; widened the score using source-file impact and diff-footprint weighting with regression tests in `internal/review/deep_judgment_test.go`

## Version 1.6 — Dashboard Enhancements (Q3 2026)

**Goal:** Transform the web dashboard from a monitoring tool into a full PR triage interface.

### Triage Inbox

- Filterable PR list with analyzer findings displayed inline
- Bulk actions: approve for merge, request changes, mark as duplicate
- Keyboard shortcuts for rapid triage (vim-style navigation)

### Merge Execution

- **Read-only merge planning** (no auto-merge)
- Generate merge instructions with conflict warnings
- Copy-to-clipboard merge commands for operators
- Integration with GitHub CLI for one-click merge execution

### Real-Time Collaboration

- WebSocket-based live updates for multi-operator workflows
- Operator presence indicators
- Comment threads on PRs (stored locally, not pushed to GitHub)

### Mobile Responsiveness

- Full-featured mobile interface for on-call operators
- Push notifications for high-priority PRs (via Telegram integration)
- Offline mode with local cache

### Guardrails

- No auto-merge or auto-approve behavior (read-only planning only)
- All merge execution requires explicit operator confirmation

## Version 1.7 — Evidence Enrichment (Q4 2026)

**Goal:** Enhance analyzer evidence beyond metadata to include diff analysis, subsystem detection, and test coverage impact.

### Diff Analysis

- Parse unified diff to detect subsystem changes (security/, auth/, api/)
- Risky pattern detection (SQL queries, auth checks, crypto operations)
- Test file changes (coverage impact estimation)

### Dependency Impact

- Public API breaking change detection
- Shared library downstream impact
- Configuration schema migration requirements

### Test Evidence

- Identify test files changed alongside source files
- Estimate coverage impact (lines changed in tested vs untested code)
- Flag PRs that modify production code without test changes

## Version 1.8 — Multi-Repo + ML Feedback (Q1 2027)

**Goal:** Extend beyond single-repo operations.

### Multi-Repo Support

- Aggregate analysis across multiple repositories
- Cross-repo dependency detection
- Unified merge planning for monorepo-style workflows

### ML Feedback Loop

- Operator decisions as training signals
- Improve duplicate detection accuracy over time
- Personalized scoring based on team preferences

### GitHub App Integration

- OAuth-based authentication (no PAT management)
- Webhook-triggered analysis (real-time PR updates)
- Status check integration (block merge on high-risk findings)

---

## Guardrails (All Versions)

1. **No auto-merge or auto-approve** — prATC is advisory only
2. **No silent exclusion** — every PR accounted for with reason codes
3. **No hidden caps** — corpus coverage is explicit and configurable
4. **Read-only by default** — all destructive operations require explicit opt-in
5. **Non-commercial use** — FSL-1.1-Apache-2.0 license
