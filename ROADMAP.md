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

## Version 1.5 — Triage Engine Fixes + Performance (CODE COMPLETE, VERIFICATION PENDING)

**Code complete: 2026-04-19.** BUG-1 through BUG-6 all fixed and on main. Overnight production run completed. Remaining: conflict/duplicate tuning and final metric verification.

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

### Remaining (live run verification)

Overnight run `20260419-065654` results: 4,992 PRs, 9 duplicate groups, 38,884 conflict pairs, 8 garbage PRs.

- [ ] Raise cap to 7,000+ or remove cap — verify duplicate groups > 10
- [ ] Expand noise file list or raise shared-file minimum — reduce conflicts below 5,000
- [ ] Run cached second pass — verify analysis < 15 min

### Next improvements (if needed after verification)

- [ ] Profile duplicate detection hot path with `go tool pprof`
- [ ] LSH/MinHash for O(n) approximate duplicate detection (replaces O(n^2) brute force)
- [ ] Expand noise file list further based on live run results
- [ ] Tune substance scoring weights based on operator feedback

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
