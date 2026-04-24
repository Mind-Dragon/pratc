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

## Version 1.6 — Pipeline-First Reset (COMPLETED)

**Shipped 2026-04-21.** Remove dashboard as product surface, strengthen the 16-gate funnel, and make CLI/API/PDF the only promoted interfaces.

### Product Surface Reset

- [x] Web dashboard removed from active product contract
- [x] CLI + API + PDF are the only first-class surfaces
- [x] `serve` is an AI-facing API server, not a backing server for browser dashboard
- [x] Verify repo can be understood and operated without `web/` assumptions

### Funnel Contract

- [x] Every non-garbage PR passes through all 16 gates in order
- [x] Gate outputs recorded explicitly (gate entered, outcome, reason, cost tier)
- [x] Outer peel removes junk fast; inner gates spend more CPU only on survivors
- [x] Duplicates advance from detection to synthesis planning

### Evidence and Synthesis

- [x] First diff-grounded evidence slice lands in review/analyze output
- [x] Duplicate groups emit synthesis-ready advisory artifacts with nominated canonical candidates

### API and Output

- [x] API responses expanded with explicit gate-journey, diff-evidence, and duplicate-synthesis fields
- [x] PDF report reads like a decision packet, not a dashboard export
- [x] Remove browser/dashboard assumptions from endpoint naming and docs

## Autonomous Runtime Readiness — COMPLETED LOCALLY

**Completed locally 2026-04-23.** The autonomous controller loop is green under the current contract: current-HEAD run, 19/0/0 audit, live runtime proof, reconciled state, and 4-provider PASS audit.

### Completed before v1.8 implementation

- [x] Reconcile `autonomous/STATE.yaml` and `autonomous/GAP_LIST.md` against real existing artifacts
- [x] Replace stale runbook paths and remove full-corpus `--max-prs 5000` defaults
- [x] Rebuild binary with truthful version/commit surface
- [x] Produce a fresh current-HEAD run under `autonomous/runs/20260423T203433Z/`
- [x] Audit that run with zero required failures
- [x] Convert or explicitly accept remaining manual audit checks
- [x] Keep `TODO.md`, `AUTONOMOUS.md`, `ARCHITECTURE.md`, and `RUNBOOK.md` aligned

## Version 1.7 — Evidence Enrichment (COMPLETED)

**Shipped locally 2026-04-23.** Enhanced analyzer evidence beyond metadata to include diff analysis, subsystem detection, test coverage impact, P1 reliability fixes, and ML reliability/honesty improvements.

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

### P1 Reliability + API Contract Repair

- Plan API: honor documented query params (`exclude_conflicts`, `stale_score_threshold`, `candidate_pool_cap`, `score_min`)
- Plan/Analyze parity: make `dry_run` explicit and consistent across HTTP and CLI paths
- Review propagation: stop swallowing review errors; return or surface them clearly
- CORS + auth headers: preserve response headers on 401/429 paths
- Settings API correctness: validate `scope`, align GET/POST/DELETE behavior, and keep import/export scoped
- GitHub auth/retry: fix `ResolveTokenForLogin`, `IsRetryableError`, and token-rotation fallback behavior
- Backoff/rate-limit handling: keep transient retry windows within documented caps and report `unlimited` correctly
- Cache/scheduler consistency: atomic sync-job updates and resume-state bookkeeping must remain transaction-safe
- SQL/YAML import safety: fail atomically on import and avoid partial writes

### P1 Verification Gates

- Add contract tests before each fix
- Verify HTTP handlers against the route contract in `internal/cmd/AGENTS.md`
- Verify cache and sync paths under `-race`
- Keep `go test ./...`, `go test ./... -race`, and Python tests green after every merge

## Version 1.8 — Action-Readiness Dry Run

**Goal:** generate a full-corpus `ActionPlan` and live TUI action dashboard in advisory mode. v1.8 is still read-only: it prepares prATC to become the engine a swarm can consume, but it performs no GitHub mutations.

See `VERSION2.0.md` for the full 16-developer swarm plan.

### ActionPlan Contract

- [ ] Add `ActionLane`, `ActionIntent`, `ActionWorkItem`, `ActionPlan`, `PolicyProfile`, `ActionPreflight`, and `ProofBundle` types
- [ ] Add JSON/schema fixtures for `action-plan.json`
- [ ] Keep Go/Python/TypeScript contract parity where those surfaces remain active
- [ ] Make every PR land in exactly one primary action lane

### Lane Classifier and Policy Gates

- [ ] Add deterministic classifier for `fast_merge`, `fix_and_merge`, `duplicate_close`, `reject_or_close`, `focused_review`, `future_or_reengage`, and `human_escalate`
- [ ] Prevent contradictions such as `blocked` plus merge intent
- [ ] Add policy profiles: `advisory`, `guarded`, `autonomous`
- [ ] Keep `advisory` as the default and prove it performs zero writes

### Product Surfaces

- [ ] CLI: `pratc actions --repo=owner/repo --format=json`
- [ ] API: `GET /api/repos/{owner}/{repo}/actions`
- [ ] TUI: action-lane board and PR detail inspector
- [ ] PDF: remains point-in-time snapshot; useful concepts move into TUI as live state

### Audit and OpenClaw Dry Run

- [ ] Add v2 audit checks for lane coverage, unsafe merge intent, action reason/evidence coverage, policy profile visibility, and advisory-mode zero writes
- [ ] Produce an OpenClaw full-corpus ActionPlan artifact from the 1.7.1 cache-first baseline
- [ ] Use the 1.7.1 report as the snapshot baseline, not as an execution manifest

## Version 1.9 — Swarm Dry-Run + Proof Loop

**Goal:** allow a 16-agent swarm to claim work from prATC, produce proof bundles, and exercise a dry-run executor without mutating GitHub.

- [ ] Durable work-item queue with claim/release/heartbeat/expiry
- [ ] Queue leases stored in SQLite with race-safe transitions
- [ ] Swarm APIs for claim, release, heartbeat, proof attach, and status
- [ ] Dry-run GitHub executor with fake backend and idempotency checks
- [ ] `fix_and_merge` sandbox workflow with patch/test/proof bundle capture
- [ ] TUI panels for queue leases, proof bundles, executor dry-run stream, rate limits, and audit ledger
- [ ] OpenClaw representative dry-run across all action lanes

## Version 2.0 — Guarded Autonomous Mutation

**Goal:** central executor can perform policy-approved GitHub actions only after live preflight, audit, idempotency check, and post-action verification.

- [ ] Live preflight for open state, head SHA, CI, mergeability, branch protection, review requirements, token permission, rate-limit budget, and policy profile
- [ ] Guarded mode: comments and labels only; no merge or close
- [ ] Autonomous mode: merge `fast_merge`, close/comment duplicates and rejects, merge `fix_and_merge` after proof validation
- [ ] Append-only executor ledger for preflight, denial, mutation, and verification
- [ ] TUI operator controls for hold/resume and policy visibility
- [ ] No direct swarm-worker-to-GitHub mutation path
- [ ] OpenClaw full-corpus v2 run audit-green

---

## Guardrails (All Versions)

1. **No unaudited GitHub mutation** — every mutation requires typed intent, live preflight, idempotency, audit ledger write, and post-action verification
2. **No direct swarm mutation** — swarm workers claim work and attach proof; only the central executor mutates GitHub
3. **No silent exclusion** — every PR accounted for with reason codes
4. **No hidden caps** — corpus coverage is explicit and configurable
5. **Read-only by default** — `advisory` remains the default policy profile
6. **Non-commercial use** — FSL-1.1-Apache-2.0 license
