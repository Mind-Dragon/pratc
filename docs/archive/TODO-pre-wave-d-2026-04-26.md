# Archived TODO before Wave D active queue

This is the previous root `TODO.md` archived before the 2026-04-26 Wave D queue reset. It is historical context only; the live backlog is the root `TODO.md`.

---

# prATC TODO — Current Iteration Toward 2.0

## Focus

Current iteration: move from the verified `1.7.1` advisory baseline toward the `2.0` swarm action engine by closing ActionPlan completeness and building the first swarm API/proof surfaces.

**Status:** Iteration A (Wave A) complete. Wave B complete. Wave C in progress — live GitHub mutation (--live flag + worker pool).

This TODO is intentionally narrow. The full 1.7.1 → 2.0 roadmap lives in `PLANS.md` and `VERSION2.0.md`.

## Source of truth

- `PLANS.md` — current phased implementation plan from 1.7.1 to 2.0
- `VERSION2.0.md` — product/release plan and 16-lane ownership map
- `GUIDELINE.md` — action policy, lanes, bucket rules, non-negotiables
- `ARCHITECTURE.md` — system shape and component ownership
- `AUTONOMOUS.md` — autonomous loop contract
- `autonomous/RUNBOOK.md` — exact operator/controller commands
- `scripts/audit_guideline.py` — deterministic audit gate

## Verified baseline

- [x] Current HEAD: `8d80f7580c74`
- [x] Runtime binary: `prATC v1.7.1 commit=8d80f7580c74 dirty=true` for Wave A tmux smoke; final proof deferred until commit boundary
- [x] Runtime: tmux session `pratc-autonomous`, port `7400`, restarted with `--force-cache`
- [x] Health: loopback and VPN OK
- [x] Current-HEAD run: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z`
- [x] Corpus: `6,632` PRs
- [x] Audit: `23 passed`, `0 failed`, `0 manual`
- [x] PDF: `report.pdf`, 29 pages
- [x] ActionPlan: schema `2.0`, advisory, `6,632` work items, `182` intents

## Current dirty-state note

- [x] Defer refreshed `autonomous/runtime/runtime-proof.json` until the next final runtime proof boundary; current Wave A source tree is intentionally dirty.
- [x] Do not mix proof-only changes with implementation commits.

## Guardrails for this iteration

- Advisory/dry-run only. No live GitHub writes.
- Controller owns docs, TODO, proof, integration, and commits.
- Use controlled swarm: max 4 implementation workers for the first wave.
- No two workers edit the same file group in the same wave.
- Worker output is advisory until controller tests pass.
- Every code task starts with tests or fixture/audit failure first.
- Do not mark completion from TUI visuals alone; use tests/artifacts/audit.

## Iteration A — v1.8 closeout + v1.9 swarm foundation

### A0 — Baseline and design lock

- [x] Reconcile `PLANS.md`, `VERSION2.0.md`, `GUIDELINE.md`, and `ARCHITECTURE.md` for policy/lanes/mutation rules.
- [x] Record baseline before implementation:
  ```bash
  git status --short
  make build
  go test ./internal/types ./internal/app ./internal/cmd ./internal/actions ./internal/workqueue ./internal/executor ./internal/monitor/...
  python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
  git diff --check
  ```
- [x] Write file-ownership map for Wave A workers.

### A1 — ActionIntent completeness

Goal: every generated ActionIntent is complete enough for swarm/executor consumption.

- [x] Add/verify fields on every intent:
  - reasons
  - evidence refs
  - confidence
  - policy profile
  - preconditions
  - idempotency key
  - dry-run/write classification
- [x] Add hard audit coverage for intent completeness.
- [x] Regenerate/update current `action-plan.json` artifact; no checked-in fixture change required.
- [x] Preserve Go/Python/TypeScript JSON parity where applicable.

Verification:

```bash
go test ./internal/types ./internal/actions ./internal/app
python -m pytest -q tests/test_audit_guideline.py
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > /tmp/pratc-action-plan.json
```

### A2 — Swarm queue API foundation

Goal: workers can claim, release, heartbeat, and inspect queue state without GitHub writes.

- [x] Add claim endpoint.
- [x] Add release endpoint.
- [x] Add heartbeat endpoint.
- [x] Add queue status endpoint.
- [x] Support filters for lane, priority, state, and expired leases.
- [x] Add route tests and race-safe workqueue tests.

Verification:

```bash
go test ./internal/workqueue ./internal/cache ./internal/cmd
```

### A3 — Proof bundle attach path

Goal: workers can attach proof to work items before executor approval.

- [x] Define persisted proof bundle attachment shape if current type is insufficient.
- [x] Add proof attach/store API.
- [x] Validate work item id, worker id, lease ownership, artifact refs, command result, and proof status.
- [x] Add tests for attach success, invalid item, stale lease, wrong owner, and duplicate/idempotent attach.

Verification:

```bash
go test ./internal/workqueue ./internal/cache ./internal/cmd ./internal/executor
```

### A4 — TUI PR detail inspector

Goal: TUI shows the live per-PR decision truth, not only lane counts.

- [x] Add monitor data model for PR detail.
- [x] Include title, author, age, status, lane, bucket, confidence, reasons, decision layers, evidence refs, duplicate/synthesis refs, risk flags, and allowed actions.
- [x] Add testable render/snapshot path if interactive monitor cannot be tested directly.
- [x] Add/extend TUI tests.

Verification:

```bash
go test ./internal/monitor/...
./bin/pratc monitor --once || true
```

### A5 — Controller integration and OpenClaw proof

- [x] Integrate Wave A changes.
- [x] Run targeted test bundle:
  ```bash
  go test ./internal/types ./internal/actions ./internal/app ./internal/cache ./internal/workqueue ./internal/cmd ./internal/executor ./internal/monitor/...
  python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
  git diff --check
  ```
- [x] Generate current ActionPlan artifact:
  ```bash
  RUN_DIR=autonomous/runs/<run-id>
  mkdir -p "$RUN_DIR"
  ./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > "$RUN_DIR/action-plan.json"
  python3 scripts/audit_guideline.py "$RUN_DIR"
  ```
- [x] Update `autonomous/GAP_LIST.md` and `autonomous/STATE.yaml` only from audit output, not chat summaries.
- [x] Defer runtime proof refresh until implementation commit/final runtime probe boundary; Wave A dirty tmux smoke recorded in `autonomous/STATE.yaml`.

## Stretch only after Iteration A is green

- [ ] `fix_and_merge` sandbox workflow scaffold.
- [ ] Live preflight checker scaffold.
- [ ] Guarded comment/label executor in fake backend only.

## Iteration B — v2.0 Guarded Executor Foundation

### B0 — Wave B Design Lock

- [x] Wave A complete: proof committed (0f2ebf0), STATE.yaml closed, audit green
- [x] Wave B ownership map confirmed (`WAVE_B_OWNERSHIP.md`)
- [x] Implementation boundary clean; runtime healthy on port 7400

Status: **B0 done — Wave B implementation phase active.** B1 TUI panels already implemented; starting B2 preflight scaffold.

### B1 — TUI Queue/Proof/Executor/Audit Panels (COMPLETED ✓)

Worker output already present and reviewed by controller:
- `internal/monitor/tui/audit.go`, `corpus.go`, `executor.go` + tests
- Data layer: `ExecutorState`, `ProofBundleRef`, `AuditLedger`, `GetAuditLedger()` in `store.go`
- Integration pending: connect live broadcaster, verify `monitor --once` renders

Gate: B1 complete; proceed to B2–B4 parallel launch.

### B2 — Live Preflight Checker (COMPLETED ✓)

Goal: Enforce all 9 preflight gates before any mutation.

- [x] Implemented `Executor.ExecuteIntent()` with live preflight checks:
  - PR still open
  - head SHA unchanged or revalidated
  - base branch allowed
  - CI/checks green for merge
  - mergeability clean for merge
  - branch protection/review requirements satisfied
  - token permission sufficient
  - rate-limit budget sufficient
  - policy allows action
  - idempotency key not already executed
- [x] Added `preflight.go` with check functions
- [x] Added tests for each gate (pass/fail scenarios)
- [x] Preflight results recorded in ledger
- [x] All tests pass (42 tests)

Files:
- `/home/agent/pratc/internal/executor/preflight.go`
- `/home/agent/pratc/internal/executor/executor.go` (ExecuteIntent changes)
- `/home/agent/pratc/internal/executor/preflight_test.go`

Verification:

```bash
go test ./internal/executor ./internal/github
```

### B3 — Guarded Comment/Label Executor (COMPLETED ✓)

Goal: Safe mutations that don't touch code.

- [x] Add `comment` and `label` action implementations in executor
- [x] Use fake GitHub mutator for testing
- [x] Respect `--dry-run` flag
- [x] Record mutations in executor ledger
- [x] Add guarded verification for comment/label actions

Files:
- `/home/agent/pratc/internal/executor/guarded.go`
- `/home/agent/pratc/internal/executor/executor.go` (interface changes, ExecuteIntent updates)
- `/home/agent/pratc/internal/executor/fake_github.go` (GetComments/GetLabels implementations)
- `/home/agent/pratc/internal/executor/ledger_integration_test.go` (test mutator updates)

Verification:

```bash
go test ./internal/executor ./internal/cmd
```

### B4 — Ledger Persistence (COMPLETED ✓)

Goal: Replace MemoryLedger with SQLite for crash recovery.

- [x] Add `executor_ledger` table to cache schema
- [x] Implement `SQLiteLedger` with append-only writes
- [x] Migrate idempotency tracking from MemoryLedger
- [x] Ensure exactly-once execution guarantees

Files:
- `/home/agent/pratc/internal/cache/ledger_schema.sql`
- `/home/agent/pratc/internal/cache/sqlite_ledger.go`
- `/home/agent/pratc/internal/cache/sqlite.go` (initExecutorLedger)
- `/home/agent/pratc/internal/cache/sqlite_ledger_test.go`

Verification:

```bash
go test ./internal/cache ./internal/executor
```

### B5 — Post-Action Verification (COMPLETED ✓)

Goal: Confirm mutations succeeded after execution.

- [x] Add verification loop after each mutation
- [x] Check GitHub state matches expected result
- [x] Return work items to safe state on failure

Files:
- `/home/agent/pratc/internal/executor/verification.go`
- `/home/agent/pratc/internal/executor/executor.go` (verification after mutations)
- `/home/agent/pratc/internal/executor/guarded.go` (simplified to use verification.go)
- `/home/agent/pratc/internal/executor/verification_test.go`

Verification:

```bash
go test ./internal/executor
```

### B6 — Fix-and-Merge Sandbox (COMPLETED ✓)

Goal: Local proof path for `fix_and_merge` items.

- [x] Create isolated worktree/checkout per work item
- [x] Capture patch/rebase proof
- [x] Capture test command output and exit code
- [x] Attach proof bundle to queue item
- [x] Keep dry-run/local-only until v2 gates green

Files:
- `/home/agent/pratc/internal/sandbox/fix_merge.go`
- `/home/agent/pratc/internal/sandbox/sandbox_test.go`
- `/home/agent/pratc/internal/executor/executor.go` (ExecuteFixAndMerge method)

Verification:

```bash
go test ./internal/sandbox ./internal/executor
```

### B7 — E2E Harness and Audit Expansion (COMPLETED ✓)

Goal: End-to-end test coverage for guarded executor.

- [x] Create fake GitHub mutator with full API surface
- [x] Build e2e harness for comment/label actions
- [x] Expand audit checks for guarded mode
- [x] Verify zero GitHub writes in advisory mode

Files:
- `/home/agent/pratc/internal/e2e/harness.go`
- `/home/agent/pratc/internal/e2e/harness_test.go`

Verification:

```bash
go test ./internal/e2e
```

### B8 — Docs and Runbook Sync (COMPLETED ✓)

Goal: Keep docs aligned with implementation.

- [x] Update `autonomous/RUNBOOK.md` with Wave B commands
- [x] Update `VERSION2.0.md` with completion status
- [x] Refresh `autonomous/STATE.yaml` from audit output

Verification:

```bash
git diff --check
```
```

## Wave C — Real GitHub Mutation + Autonomous Merge/Close

### Prerequisites
  - [x] Wave B complete: TUI panels, preflight gates, guarded executor, ledger, verification
  - [x] Audit green for Wave B, no GitHub writes in advisory mode
  - [x] Import cycle resolved, all tests green
  - [x] Fake GitHub mutator validated

### Implementation
  - [ ] Configure live GitHub PAT (environment only, never commit)
  - [ ] Implement real GitHub mutator (replace fake_github.go with live API calls)
  - [ ] Run preflight checks against live GitHub state (PR open, CI, mergeable)
  - [ ] Action: `merge` — use GitHub merge API with proper method (merge/squash/rebase)
  - [ ] Action: `close` — close PR with comment explaining reason
  - [ ] Add `--live` flag to CLI (default dry-run)
  - [ ] Add safety circuit breaker: max concurrent mutations per repo
  - [ ] Retain all ledger records with real mutation outcomes
  - [ ] Expand audit: verify real GitHub state after mutation
  - [ ] Update TUI to show real mutation status (not fake)
  - [ ] E2E test against real repo (use test repo or sandbox org)
  - [ ] Update RUNBOOK.md with Wave C commands

### Gates before merge to main
  - [ ] All tests pass with real GitHub integration (use test double or sandbox)
  - [ ] Dry-run mode remains default and safe
  - [ ] Ledger persists all mutation attempts with timestamps
  - [ ] Verification confirms GitHub state matches intent
  - [ ] Audit confirms zero unexpected mutations
  - [ ] TUI executor panel updates real-time
  - [ ] Documentation updated (VERSION2.0.md, RUNBOOK.md)

### Not this iteration
  - [ ] Direct swarm-worker GitHub access (requires OAuth architecture)
  - [ ] Browser dashboard revival (Wave D+)
  - [ ] Multi-repo orchestration (Wave D+)


## Wave C — Live GitHub Mutation (IN PROGRESS)

### Implementation
  - [x] Add `--live` flag to `serve` command
  - [x] Extend `runServer` signature to accept `live` parameter
  - [x] Link `ActionIntent.WorkItemID` to persisted queue work items
  - [x] Persist `ActionIntent` records in `internal/workqueue` and expose `GetIntentsForWorkItem()`
  - [x] Make `LiveGitHubMutator` satisfy `executor.GitHubMutator` with dry-run-aware methods
  - [x] Add `executor.Worker` to claim work items and execute persisted intents through the central executor
  - [x] Wire `serve --live` to start the executor worker
  - [x] Verify focused tests, build, and `serve --help`

### Verification
```bash
go test ./internal/cmd ./internal/executor ./internal/workqueue ./internal/app ./internal/types/...
make build
./bin/pratc serve --help | grep -- --live
git diff --check
```

## Not this iteration

- [ ] Real GitHub mutation (requires Wave B completion)
- [ ] Autonomous merge/close (requires Wave C)
- [ ] Direct swarm-worker GitHub access
- [ ] Browser dashboard revival
- [ ] Multi-repo orchestration

## Completion gate for this iteration

- [x] ActionIntent completeness is machine-audited.
- [x] Queue API can claim/release/heartbeat/status safely.
- [x] Proof bundle attach path works and is tested.
- [x] TUI PR detail inspector has testable output.
- [x] OpenClaw ActionPlan run remains audit-green.
- [x] No GitHub writes occurred.
- [x] Runtime proof deferred until implementation commit/final runtime probe boundary.

## Wave B completion gate

- [x] TUI panels render with live data (verified)
- [x] Live preflight checker enforces all 9 gates
- [x] Guarded comment/label executor works with fake GitHub
- [x] Ledger persistence survives restart
- [x] Post-action verification confirms mutations
- [x] Fix-and-merge sandbox produces valid proof bundles
- [x] E2E harness passes all audit checks
- [x] Docs aligned with implementation
