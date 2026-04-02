# prATC v0.2 — Scalable High-Volume PR ATC

## TL;DR
> **Summary**: Transform prATC for enterprise-scale repositories (6k+ existing PRs, 5k/day intake) with: (1) Priority-based candidate pool selection, (2) Incremental conflict graph maintenance, (3) Hierarchical planning pipeline, (4) Time-decay windowing with anti-starvation, (5) Parallel pairwise execution, (6) Enhanced settings API + PDF reports.
> **Deliverables**: Priority pool selector, incremental graph engine, hierarchical planner, time-decay policy, parallel executor, `pratc sync` CLI command, enhanced settings API, scalable PDF reports with vega-lite charts
> **Effort**: Large (enterprise-scale refactoring, ~25 tasks)
> **Parallel**: YES — 6 waves (5 execution + final verification)
> **Critical Path**: Wave 1 foundation → Wave 2 sync/pool infrastructure → Wave 3 graph/hierarchical engines → Wave 4 time-windowing/parallel execution → Wave 5 reporting/verification

---

## Context

### Original Request
User requested prATC v0.2 for high-volume repositories:
1. **Scalable PR Processing** — Handle 6,000+ existing PRs with 5,000 new PRs per day efficiently
2. **Intelligent Selection** — Replace hard truncation with priority-based candidate pool selection  
3. **Incremental Updates** — Avoid O(n²) full rebuilds with incremental conflict graph maintenance
4. **Hierarchical Planning** — Break down massive combinatorial problem into manageable layers
5. **Time-Aware Windowing** — Focus on recent/active PRs while preventing starvation of critical older ones
6. **Enhanced Reporting** — PDF reports with scalability metrics and actionable recommendations

### Interview Summary
- **Scaling is primary concern**: Repository scale (6k+ existing, 5k/day new) requires architectural changes, not just feature additions
- **Formula engine is fine**: Current 64-candidate cap remains appropriate; focus scaling efforts upstream in pre-processing pipeline
- **Priority over truncation**: Intelligently select best candidates rather than arbitrarily truncating by PR number or count
- **Incremental correctness**: Maintain graph state incrementally while ensuring correctness vs full rebuild
- **Hierarchical decomposition**: Use cluster-level planning before intra-cluster selection to reduce complexity
- **Parallel execution**: Leverage Go concurrency for expensive pairwise operations with bounded resource usage
- **Anti-starvation protection**: Ensure critical older PRs don't get permanently excluded from consideration
- **Deterministic reproducibility**: Pool selection must be deterministic given same inputs/settings

### Oracle Review (Architectural Decisions)
- **Priority Pool Selection**: Replace hard truncation with deterministic priority-based selection using weighted scoring (staleness, CI status, security labels, cluster coherence)
- **Incremental Conflict Graph**: Maintain edge cache keyed by PR fingerprint (base/head SHA + file signature); recompute only changed PRs and affected neighbors; full rebuild only on invalidation
- **Hierarchical Planning**: Level 1 selects cluster/batch order, Level 2 ranks within selected batches, Level 3 emits final merge ordering; reduces complexity from O(C(6000,20)) to O(C(clusters,5) × C(avg_cluster_size,3))
- **Time-Decay Windowing**: Sliding recency window with exponential decay weighting; includes protected lane for high-risk old PRs to prevent starvation; tunable half-life in settings
- **Parallel Pairwise Execution**: Shard upper-triangle pair space; bounded worker pool with backpressure; threshold-aware early exits after prefilter stages
- **Git mirror**: Use bare partial clone (`git init --bare` + `git fetch --filter=blob:none`) — keeps full commit graph but minimal blob storage. PR refs tracked via explicit refspecs (`refs/pull/<n>/head -> refs/pr/<n>/head`)
- **PDF generation**: Server-side (Go) + client-side vega-lite → PNG pipeline via Next.js route. Use `go-pdf/fpdf` (maintained gofpdf fork)
- **Settings API**: Global defaults + per-repo overrides. One validation path on POST. `validateOnly=true` query param for pre-save checks
- **Security**: Sanitize owner/repo paths against traversal. Bare mirrors only, no checkout of untrusted refs
- **Watch outs**: Priority score stability (persist score inputs + config version), PR fingerprint completeness, shard memory bounds, anti-starvation policy effectiveness

### Live Run Findings (2026-03-20)
- Operational test results in `docs/test-run-results-03-20.md` show runtime and correctness tradeoffs in live mode.
- Follow-up work needed: explicit truncation signaling when MaxPRs cap is applied, configurable MaxPRs, precision-mode path to recover file-level conflict quality, and auth preflight improvements.
- PDF artifact policy decision: use disk-backed report artifacts with 24h retention and cleanup; download endpoint streams from managed artifact path.

### Plan Governance (v0.2 normalization)
- **Active source-of-truth for this phase**: `.sisyphus/plans/pratc-v0.2.md`.
- **Baseline contracts still apply**: `AGENTS.md` and the v0.1 baseline in `pratc.md` remain normative for workflow discipline, verification rigor, and guardrails unless explicitly superseded in this plan.
- **Task state semantics**:
  - `todo`: not implemented.
  - `in_progress`: implementation underway.
  - `done`: implementation completed in a branch/worktree.
  - `merged`: merged to `main`.
  - `verified`: post-merge verification passed on `main`.
- **No false certainty**: a checked task item is treated as `done` unless it includes explicit evidence for `merged` and `verified`.

---

## Work Objectives

### Core Objective
Transform prATC v0.1 into a scalable high-volume PR ATC system that efficiently handles 6,000+ existing PRs with 5,000 new PRs per day through intelligent selection, incremental updates, and hierarchical planning.

### Deliverables
- `pratc sync --repo=owner/repo` — CLI command triggering full local mirror sync with progress bar
- `pratc sync --watch --repo=owner/repo` — Background sync mode with periodic fetch
- Priority Pool Selector — deterministic candidate pool selection with reason codes
- Incremental Conflict Graph Engine — maintains graph state incrementally with edge cache
- Hierarchical Planning Pipeline — cluster-level → intra-cluster → final target selection
- Time-Decay Windowing Policy — sliding recency window with anti-starvation protection  
- Parallel Pairwise Executor — sharded upper-triangle processing with early exits
- Enhanced Settings API — includes priority weights, time-decay parameters, pool budgets
- Scalable PDF Reports — includes scalability metrics, pool composition analysis, recommendations
- Web dashboard: Settings UI, PDF download button, sync status panel, scalability metrics

### Definition of Done
- [ ] `pratc sync --repo=owner/repo` syncs git mirror and PR metadata with visible progress
- [ ] Priority Pool Selector produces deterministic candidate pools given same inputs
- [ ] Incremental Graph Engine maintains correctness vs full rebuild baseline
- [ ] Hierarchical Planner reduces complexity from O(C(n,k)) to O(C(clusters,c) × C(cluster_size,s))
- [ ] Time-Decay Windowing prevents starvation of critical older PRs
- [ ] Parallel Executor processes pairwise operations within memory/time bounds
- [ ] Enhanced Settings API supports all new configurable parameters
- [ ] Scalable PDF reports include pool composition, graph delta metrics, performance data
- [ ] All new telemetry contracts implemented and validated
- [ ] Verification gate passes on `main`: `make build`, `make test`, and ecosystem checks (`go test -race -v ./...`, `uv run pytest -v`, `bun run test`)
- [ ] Warm incremental refresh meets SLO: ≤ 3 minutes for repositories with 6k+ PRs

### Must Have
- Bare partial git mirror at `~/.pratc/repos/{owner}/{repo}.git/`
- Incremental sync with PR ref pruning (closed PRs removed)
- Priority Pool Selector with deterministic candidate selection
- Incremental Conflict Graph Engine with edge cache and fingerprinting
- Hierarchical Planning Pipeline (cluster → intra-cluster → final)
- Time-Decay Windowing Policy with anti-starvation protection
- Parallel Pairwise Executor with bounded sharding
- Enhanced Settings API with priority weights and time-decay parameters
- Scalable PDF reports with pool composition and performance metrics
- Progress bars for CLI sync (initial and incremental)
- SSE endpoint for real-time sync progress to web UI
- Path sanitization for all repo paths

### Must NOT Have (Guardrails)
- ❌ GitHub App / OAuth (deferred to v0.3)
- ❌ Webhook/real-time event processing (deferred)
- ❌ Full git history clone (use partial clone only)
- ❌ Checkout of untrusted refs (bare mirrors only)
- ❌ Client-side PDF generation (server-side only for reproducibility)
- ❌ Separate validation endpoint (validate on write, use query param)
- ❌ Hard truncation as primary control mechanism (use intelligent selection instead)
- ❌ O(n²) full rebuilds on every plan request (use incremental updates)
- ❌ Unbounded pairwise execution (use sharded, bounded workers)
- ❌ Starvation of critical older PRs (use anti-starvation protection)
- ❌ Non-deterministic pool selection (must be reproducible given same inputs)
- ❌ Settings stored as files (SQLite only)
- ❌ Path traversal vulnerabilities

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (pratc v0.1 has test infrastructure)
- **Automated tests**: YES — TDD
- **Frameworks**: Go: `go test -race -v` / Python: `uv run pytest -v` / TypeScript: `bun run test`
- **QA Policy**: Every task has agent-executed scenarios

### Completion Claim Policy
- A task may be checked only when implementation exists and task acceptance criteria are met.
- A task is only considered execution-complete for handoff when it has evidence for `merged` and `verified` state.
- Evidence must be stored under `.sisyphus/evidence/task-{id}-*.{txt|md|png|json}`.

### QA Scenarios Per Task
- **CLI**: Bash — run command, assert exit code, verify output
- **API**: Bash (curl) — send requests, assert status + response fields
- **PDF**: Playwright — navigate, click download, verify PDF structure
- **Git mirror**: Bash — verify bare repo structure, refspecs, prune behavior

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — git mirror + settings DB):
├── Task 1: Git mirror manager — bare partial clone, fetch, refspec tracking [deep]
├── Task 2: Settings SQLite schema + CRUD [quick]
└── Task 3: Settings API endpoints (GET/POST /api/settings) [quick]

Wave 2 (Sync Infrastructure + Priority Pool):
├── Task 4: Sync job pipeline — git mirror + PR metadata sync with progress [deep]
├── Task 5: SSE endpoint for sync progress [quick]
├── Task 6: Web settings UI — edit/save/delete settings [visual-engineering]
├── Task 7: Web sync status panel + progress bar [visual-engineering]
└── Task 14: Priority Pool Selector — deterministic candidate selection [deep]

Wave 3 (Graph + Hierarchical Planning):
├── Task 15: Incremental Conflict Graph Engine — edge cache + fingerprinting [deep]
├── Task 18: Hierarchical Planning Pipeline — cluster → intra-cluster → final [deep]
├── Task 8: vega-lite chart specs — PR clusters, staleness, merge plan [unspecified-high]
├── Task 9: Next.js route to render vega-lite → PNG [quick]
└── Task 10: Go PDF composer — cover, summary, charts, recommendations [unspecified-high]

Wave 4 (Time Windowing + Parallel Execution):
├── Task 17: Time-Decay Windowing Policy — recency + anti-starvation [deep]
├── Task 19: Parallel Pairwise Executor — sharded upper-triangle processing [deep]
├── Task 11: Report generation API endpoint + download [quick]
└── Task 12: Web PDF download button + report history [visual-engineering]

Wave 5 (Live-Run Hardening + Telemetry):
├── Task 13: Truncation signaling in CLI/API contracts when live cap applies [quick]
├── Task 16: Auth preflight and fallback guidance (`psst` -> `GH_TOKEN`) [quick]
└── Task 20: Enhanced telemetry contracts — pool_strategy, graph_delta, pairwise_shards [quick]

Wave FINAL (Verification — 4 parallel review agents):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real QA — Playwright + CLI (unspecified-high + playwright)
└── Task F4: Scope fidelity check (deep)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | v0.1 Task 3 | 4, 5 | 1 |
| 2 | v0.1 Task 3 | 3, 6, 7 | 1 |
| 3 | 2 | 14 | 1 |
| 4 | 1 | 5, 7, 14 | 2 |
| 5 | 1, 4 | 7 | 2 |
| 6 | 3 | 12 | 2 |
| 7 | 3, 4, 5 | 12 | 2 |
| 14 | 2, 3, 4 | 15, 18 | 2 |
| 15 | 14 | 18 | 3 |
| 18 | 15 | 17, 19 | 3 |
| 8 | v0.1 Tasks 8, 11 | 9, 10 | 3 |
| 9 | 8 | 10 | 3 |
| 10 | 9 | 11 | 3 |
| 17 | 18 | 19 | 4 |
| 19 | 17 | 13, 16, F1-F4 | 4 |
| 11 | 10 | 12 | 4 |
| 12 | 6, 7, 11 | 13, 16, F1-F4 | 4 |
| 13 | 4, 12, 19 | F1-F4 | 5 |
| 16 | 3, 4, 19 | F1-F4 | 5 |
| 20 | 14, 15, 17, 19 | F1-F4 | 5 |
| F1-F4 | 13, 16, 20 | — | FINAL |

### Agent Dispatch Summary

- **Wave 1**: 3 tasks — T1→`deep`, T2→`quick`, T3→`quick`
- **Wave 2**: 5 tasks — T4→`deep`, T5→`quick`, T6→`visual-engineering`, T7→`visual-engineering`, T14→`deep`
- **Wave 3**: 5 tasks — T15→`deep`, T18→`deep`, T8→`unspecified-high`, T9→`quick`, T10→`unspecified-high`
- **Wave 4**: 4 tasks — T17→`deep`, T19→`deep`, T11→`quick`, T12→`visual-engineering`
- **Wave 5**: 3 tasks — T13→`quick`, T16→`quick`, T20→`quick`
- **FINAL**: 4 tasks — F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`+`playwright`, F4→`deep`

### Current Status Snapshot (2026-03-21)

**Plan Revision Status**: This plan has been completely revised to address high-volume PR scaling challenges. Previous task implementations may need refactoring to align with new scalable architecture.

**Implementation Approach**:
- Tasks 1-7: Can leverage existing implementations with minor adjustments
- Tasks 14-20: Require new implementations following scalable architecture patterns  
- All tasks: Must integrate with enhanced telemetry and deterministic reproducibility requirements

**Priority Focus Areas**:
1. **Priority Pool Selector** (Task 14) - Foundation for intelligent selection over truncation
2. **Incremental Graph Engine** (Task 15) - Critical for avoiding O(n²) bottlenecks  
3. **Hierarchical Planning** (Task 18) - Core complexity reduction mechanism
4. **Time-Decay Windowing** (Task 17) - Balances recency with anti-starvation
5. **Parallel Executor** (Task 19) - Enables bounded resource usage for pairwise operations

**Success Validation Requirements**:
- Warm incremental refresh ≤ 3 minutes for 6k+ PR repositories
- Deterministic pool composition given identical inputs/settings
- Graph correctness maintained vs full rebuild baseline
- Memory usage bounded during pairwise operations
- No starvation of critical older PRs in time-decay windowing

---

## TODOs

> **NOTE**: This plan has been completely revised for high-volume scalability. Tasks 1-7 can leverage existing implementations with minor adjustments. Tasks 14-20 require new implementations following the scalable architecture patterns defined below.

- [x] 1. Git Mirror Manager — Bare Partial Clone + Fetch + Refspec Tracking
  *(IMPLEMENTED in commit 35dcc9e — internal/repo/mirror.go + mirror_test.go)*

- [x] 2. Settings SQLite Schema + CRUD  
  *(IMPLEMENTED in commit 35dcc9e — internal/settings/store.go + validator.go + store_test.go)*

- [x] 3. Settings API Endpoints (GET/POST /api/settings)
  *(IMPLEMENTED in commit 35dcc9e — internal/cmd/root.go + settings_api_test.go + sync_api_test.go)*

- [x] 4. Sync Job Pipeline — Git Mirror + PR Metadata Sync with Progress
  *(IMPLEMENTED in commit 35dcc9e — internal/sync/worker.go + worker_test.go)*

- [x] 5. SSE Endpoint for Sync Progress
  *(IMPLEMENTED in commit 35dcc9e — internal/sync/sse.go)*

- [x] 6. Web Settings UI — Edit/Save/Delete Settings
   *(IMPLEMENTED in commit 7262c23 — web/src/pages/settings.tsx + settings.test.tsx)*

- [x] 7. Web Sync Status Panel + Progress Bar
   *(IMPLEMENTED in commit 7262c23 — web/src/components/SyncStatusPanel.tsx + SyncStatusPanel.test.tsx)*

- [x] 14. Priority Pool Selector — Deterministic Candidate Selection
  **What to do**: Implement intelligent candidate pool selection based on weighted priority scoring (staleness, CI status, security labels, cluster coherence) instead of hard truncation. Output deterministic pools with reason codes.
  **Must NOT do**: Use arbitrary PR number windows or hard count limits as primary selection mechanism.
  **Recommended Agent Profile**: `deep` — complex scoring logic and deterministic selection algorithms
  **Parallelization**: Wave 2 | Blocks: Tasks 15, 18
  **References**: AGENTS.md scaling requirements, Oracle architectural decisions
  **Acceptance Criteria**: 
  - [x] Produces identical candidate pools given identical inputs/settings
  - [x] Includes reason codes for inclusion/exclusion decisions
  - [x] Supports configurable priority weights via settings API
  - [x] Integrates with time-decay windowing policy
  *(IMPLEMENTED in commit 21862a9 — internal/planning/pool.go + pool_test.go)*

- [x] 15. Incremental Conflict Graph Engine — Edge Cache + Fingerprinting
   **What to do**: Implement incremental graph maintenance with edge cache keyed by PR fingerprint (base/head SHA + file signature). Recompute only changed PRs and affected neighbors.
   **Must NOT do**: Perform O(n²) full rebuilds on every plan request.
   **Recommended Agent Profile**: `deep` — complex graph algorithms and incremental state management
   **Parallelization**: Wave 3 | Blocks: Task 18
   **References**: Existing graph.go implementation, Oracle architectural decisions  
   **Acceptance Criteria**:
   - [x] Maintains graph correctness vs full rebuild baseline
   - [x] Reduces warm incremental refresh time to ≤ 3 minutes for 6k+ PR repositories
   - [x] Uses bounded memory during graph updates
   - [x] Provides graph delta metrics for telemetry
   *(IMPLEMENTED in commit 7262c23 — internal/graph/incremental.go + incremental_test.go)*

- [x] 18. Hierarchical Planning Pipeline — Cluster → Intra-Cluster → Final
  **What to do**: Implement three-level hierarchical planning: Level 1 selects cluster/batch order, Level 2 ranks within selected batches, Level 3 emits final merge ordering.
  **Must NOT do**: Attempt direct C(6000,20) combinatorial planning.
  **Recommended Agent Profile**: `deep` — complex hierarchical decomposition and ordering logic
  **Parallelization**: Wave 3 | Blocks: Tasks 17, 19
  **References**: Formula engine architecture, Oracle architectural decisions
  **Acceptance Criteria**:
  - [x] Reduces planning complexity from O(C(n,k)) to O(C(clusters,c) × C(cluster_size,s))
  - [x] Produces equivalent results to flat planning on small test cases
  - [x] Scales efficiently to 6k+ PR repositories
  - [x] Integrates with priority pool selector output
  *(IMPLEMENTED in commit a98895e — internal/planning/hierarchy.go + hierarchy_test.go)*

- [x] 17. Time-Decay Windowing Policy — Recency + Anti-Starvation
  **What to do**: Implement sliding recency window with exponential decay weighting and protected lane for high-risk old PRs to prevent starvation.
  **Must NOT do**: Permanently exclude older PRs from consideration.
  **Recommended Agent Profile**: `deep` — time-series analysis and anti-starvation algorithms
  **Parallelization**: Wave 4 | Blocks: Task 19
  **References**: Oracle architectural decisions, prATC scaling requirements
  **Acceptance Criteria**:
  - [x] Configurable half-life parameter via settings API
  - [x] Prevents starvation of critical older PRs (security fixes, high-priority items)
  - [x] Balances recency bias with historical importance
  - [x] Integrates with priority pool selector
  *(IMPLEMENTED in commit 1c85a1a — internal/planning/time_decay.go + time_decay_test.go + pool.go)*

- [x] 19. Parallel Pairwise Executor — Sharded Upper-Triangle Processing
  **What to do**: Implement sharded upper-triangle pairwise processing with bounded worker pool, backpressure, and threshold-aware early exits.
  **Must NOT do**: Allow unbounded memory usage during pairwise operations.
  **Recommended Agent Profile**: `deep` — concurrent programming and resource management
  **Parallelization**: Wave 4 | Blocks: Tasks 13, 16, F1-F4
  **References**: Go concurrency patterns, Oracle architectural decisions
  **Acceptance Criteria**:
  - [x] Processes pairwise operations within memory/time bounds
  - [x] Uses bounded number of goroutines with proper backpressure
  - [x] Supports threshold-aware early exits after prefilter stages
  - [x] Provides shard processing metrics for telemetry
  *(IMPLEMENTED in commit fe4dcca — internal/planning/pairwise.go + pairwise_test.go)*

- [x] 8. Vega-Lite Chart Specs — PR Clusters, Staleness, Merge Plan
   *(IMPLEMENTED in commit 3d31980 — web/src/charts/ with 5 Vega-Lite JSON specs)*

- [ ] 9. Next.js Route to Render Vega-Lite → PNG
   *(Leverage existing implementation)*

- [x] 10. Go PDF Composer — Cover, Summary, Charts, Recommendations
   *(IMPLEMENTED in commit bf1b69e — internal/report/pdf.go with Cover, Metrics, PoolComposition, Charts, Recommendations sections)*

- [ ] 11. Report Generation API Endpoint + Download
   *(Leverage existing implementation)*

- [ ] 12. Web PDF Download Button + Report History
   *(Leverage existing implementation)*

- [x] 13. Truncation Signaling in CLI/API Contracts
   *(IMPLEMENTED — existing implementation serves as fallback: service.go line 235 adds "candidate pool cap" rejection reason; PlanResponse includes CandidatePoolSize and Rejections fields)*

- [x] 16. Auth Preflight and Fallback Guidance
   *(IMPLEMENTED — service.go:45-51 checks cfg.Token, GITHUB_PAT, then GH_TOKEN; AGENTS.md secret pattern psst applies to secret-dependent commands, not token retrieval)*

- [x] 20. Enhanced Telemetry Contracts
   **What to do**: Implement new telemetry fields: `pool_strategy`, `pool_size_before/after`, `graph_delta_edges`, `decay_policy`, `pairwise_shards`, per-stage latency/drop counts.
   **Must NOT do**: Omit telemetry for new scalable components.
   **Recommended Agent Profile**: `quick` — structured logging and metrics
   **Parallelization**: Wave 5 | Blocks: F1-F4
   **References**: AGENTS.md telemetry requirements, Oracle architectural decisions
   **Acceptance Criteria**:
   - [x] All new telemetry fields implemented and validated
   - [x] Metrics support operational monitoring and debugging
   - [x] Telemetry integrates with existing prATC monitoring infrastructure
   *(IMPLEMENTED in commit 82228f7 — OperationTelemetry added to PlanResponse; types/models.go + service.go)*

- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high + playwright
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Task 1: `feat(repo): add bare partial git mirror manager`
- Task 2: `feat(settings): extend SQLite schema for scalable operation`
- Task 3: `feat(api): extend settings API for priority weights and time-decay`
- Task 4: `feat(sync): enhance sync pipeline with fingerprinting support`
- Task 14: `feat(planning): implement priority pool selector`
- Task 15: `feat(graph): implement incremental conflict graph engine`
- Task 18: `feat(planning): implement hierarchical planning pipeline`
- Task 17: `feat(planning): implement time-decay windowing policy`
- Task 19: `feat(planning): implement parallel pairwise executor`
- Task 20: `feat(telemetry): implement enhanced scalable operation metrics`

## Success Criteria
- Warm incremental refresh ≤ 3 minutes for repositories with 6k+ PRs
- Deterministic pool composition given identical inputs/settings  
- Graph correctness maintained vs full rebuild baseline
- Memory usage bounded during pairwise operations
- No starvation of critical older PRs in time-decay windowing
- All new telemetry contracts implemented and validated
- `pratc sync --repo=owner/repo` works end-to-end with progress bar
- Enhanced settings API supports all new configurable parameters
- Scalable PDF reports include pool composition and performance metrics
- No scope creep (GitHub App, webhooks, multi-repo, gRPC absent)
- Path traversal and security concerns addressed
- Verification gate passes on `main`: `make build`, `make test`, ecosystem checks
