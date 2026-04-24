# prATC 1.7.1 -> 2.0 Implementation Plan

> **For Hermes:** Use `subagent-driven-development` for scoped implementation tasks and controlled `swarm-orchestrator` waves only after the controller writes a file-ownership map. Do not start a wide swarm without a green barrier check.

**Goal:** Move prATC from the verified 1.7.1 advisory triage/runtime baseline to a 2.0 guarded autonomous action engine.

**Architecture:** Keep prATC read-only by default. Swarm workers claim work and attach proof; only the central executor can perform GitHub mutations after policy, live preflight, idempotency, audit ledger write, and post-action verification. The TUI becomes the live operator surface; PDF remains a snapshot artifact.

**Tech Stack:** Go CLI/API/TUI, SQLite cache/settings/ledger, Python audit/controller tests, GitHub GraphQL/REST clients, tmux runtime proof on port 7400.

---

## Current Verified Baseline

Latest current-HEAD proof run:

- Commit: `8d80f7580c74`
- Binary: `prATC v1.7.1`, `dirty=false`
- Runtime: `tmux` session `pratc-autonomous`, port `7400`
- Health: loopback and VPN OK
- Run dir: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z`
- Corpus: `6,632` PRs
- Audit: `22 passed`, `0 failed`, `0 manual`
- PDF: `report.pdf`, 29 pages
- ActionPlan: schema `2.0`, policy `advisory`, `6,632` work items, `182` action intents
- Lanes: `duplicate_close=91`, `human_escalate=6541`

Important state:

- Current runtime and HEAD match.
- `autonomous/runtime/runtime-proof.json` was refreshed and is currently the only tracked dirty file before this plan/TODO update.
- The system is still advisory. No GitHub write path is allowed as complete until guarded/autonomous preflight and ledger gates are implemented and audited.

---

## Operating Model

Use controlled swarm, not uncontrolled 16-way parallelism.

### Controller owns

- `PLAN.md`, `TODO.md`, `VERSION2.0.md`, `GUIDELINE.md`, `ARCHITECTURE.md`, `AUTONOMOUS.md`
- wave decomposition and file ownership
- integration of worker changes
- final verification and status updates
- runtime proof refresh
- commits, unless explicitly delegated to isolated workers

### Worker rules

- Workers get exact file ownership.
- No two workers edit the same file group in the same wave.
- Workers must use TDD for code changes.
- Workers do not mutate GitHub.
- Worker output is advisory until the controller runs local verification.
- If a worker stalls or writes outside scope, controller kills/reconciles before continuing.

### Parallelism caps

- First implementation wave: max 4 workers.
- After a green barrier: max 6-8 workers.
- Use 16 lanes as a map of ownership, not as an immediate concurrency target.

---

## Release Ladder

### Phase 0 — Design Lock and Baseline Hygiene

**Objective:** Make the next build unambiguous and prevent dirty-state confusion.

**Files:**

- Modify: `PLAN.md`
- Modify: `TODO.md`
- Optional modify: `autonomous/runtime/runtime-proof.json`
- Read/check: `VERSION2.0.md`, `GUIDELINE.md`, `ARCHITECTURE.md`, `AUTONOMOUS.md`, `autonomous/RUNBOOK.md`

**Tasks:**

1. Decide whether to commit the refreshed `autonomous/runtime/runtime-proof.json` as a proof-only commit or keep it dirty until the next runtime proof boundary.
2. Confirm docs agree on:
   - policy profiles: `advisory`, `guarded`, `autonomous`
   - action lanes
   - no direct swarm-to-GitHub mutation
   - no merge/close without live preflight and audit
3. Record baseline test status.
4. Write a worker ownership map before spawning implementation workers.

**Verification:**

```bash
git status --short
make build
go test ./internal/types ./internal/app ./internal/cmd ./internal/actions ./internal/workqueue ./internal/executor ./internal/monitor/...
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
git diff --check
```

**Exit gate:** baseline green, docs aligned, current iteration tasks in `TODO.md`.

---

### Phase 1 — v1.8 Closeout: ActionPlan Completeness

**Objective:** Finish the advisory ActionPlan contract so every intent is safe for downstream swarm consumption.

**Files:**

- Modify: `internal/types/models.go`
- Modify: `fixtures/action-plan.json`
- Modify: `ml-service/src/pratc_ml/models.py` if JSON parity changes
- Modify: `internal/actions/*`
- Modify: `internal/app/*`
- Modify: `scripts/audit_guideline.py`
- Modify: `tests/test_audit_guideline.py`
- Test: `internal/types/*_test.go`, `internal/actions/*_test.go`, `internal/app/*_test.go`

**Task 1: Enforce ActionIntent completeness**

Add or tighten generated ActionIntent fields so every intent carries:

- reason trail
- evidence refs
- confidence
- policy profile
- preconditions
- idempotency key
- dry-run/write classification

**Verification:**

```bash
go test ./internal/types ./internal/actions ./internal/app
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > /tmp/action-plan.json
python3 scripts/audit_guideline.py projects/openclaw_openclaw/runs/v171-head-20260424T153126Z
```

**Task 2: Add hard audit check for intent completeness**

Audit should fail if any generated intent lacks reasons, evidence, confidence, policy, preconditions, or idempotency.

**Verification:**

```bash
python -m pytest -q tests/test_audit_guideline.py
python3 scripts/audit_guideline.py <run-dir-with-action-plan>
```

**Exit gate:** advisory ActionPlan audit remains `0 failed`, with intent completeness machine-checked.

---

### Phase 2 — v1.8 Closeout: TUI PR Detail and Dashboard Data Bridge

**Objective:** Let the TUI expose the same per-PR truth currently buried in JSON/PDF artifacts.

**Files:**

- Modify: `internal/monitor/data/*`
- Modify: `internal/monitor/tui/*`
- Modify: `internal/report/*` only if shared data adapters are required
- Test: `internal/monitor/...`

**Tasks:**

1. Add PR detail model with title, author, age, state, lane, bucket, confidence, reason trail, decision layers, evidence refs, duplicate/synthesis refs, risk flags, and allowed actions.
2. Add a testable non-interactive render/snapshot function if `monitor --once` is not present.
3. Add TUI PR detail inspector over the generated ActionPlan/analyze data.
4. Add report/dashboard data bridge helpers so TUI and PDF consume normalized data shapes rather than duplicating interpretation.

**Verification:**

```bash
go test ./internal/monitor/...
./bin/pratc monitor --once || true
```

**Exit gate:** TUI can render lane overview plus selected PR detail without relying on the PDF.

---

### Phase 3 — v1.9 Swarm API and Proof Loop

**Objective:** Let swarm workers claim work, heartbeat, release, attach proof bundles, and exercise dry-run executor transitions without GitHub writes.

**Files:**

- Modify: `internal/workqueue/*`
- Modify: `internal/cache/*`
- Modify: `internal/cmd/*`
- Modify: `internal/executor/*`
- Modify: `internal/types/models.go`
- Test: `internal/workqueue/*_test.go`, `internal/cmd/*_test.go`, `internal/executor/*_test.go`, cache migration tests

**Task 1: Queue API endpoints**

Add authenticated API endpoints for:

- claim work item
- release work item
- heartbeat lease
- query queue status
- filter by lane, priority, state, and lease expiry

**Task 2: Proof bundle attach path**

Add endpoint and store support for proof bundle refs:

- work item id
- worker id
- patch/rebase proof refs
- test command and result
- artifact paths
- validation status

**Task 3: Representative dry-run swarm harness**

Create fixture-backed or cache-backed e2e that claims representative items across lanes and feeds them through dry-run executor.

**Verification:**

```bash
go test ./internal/cache ./internal/workqueue ./internal/cmd ./internal/executor
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > autonomous/runs/<run-id>/action-plan.json
python3 scripts/audit_guideline.py autonomous/runs/<run-id>
```

**Exit gate:** multiple workers can claim disjoint work, expired leases return safely, proof attaches to the right work item, and dry-run executor records what would happen.

---

### Phase 4 — Fix-and-Merge Sandbox

**Objective:** Build the local proof path for `fix_and_merge` items before any executor can consider merge.

**Files:**

- Modify: `internal/repo/*`
- Modify: `internal/executor/*`
- Modify: `internal/workqueue/*`
- Modify: `internal/types/models.go`
- Test: `internal/repo/*_test.go`, `internal/executor/*_test.go`, `internal/workqueue/*_test.go`

**Tasks:**

1. Create isolated worktree/checkout prep for a work item.
2. Capture patch/rebase proof.
3. Capture test command, output, and exit code.
4. Attach proof bundle to queue item.
5. Keep this dry-run/local-only until v2 guarded/autonomous gates are green.

**Verification:**

```bash
go test ./internal/repo ./internal/executor ./internal/workqueue
```

**Exit gate:** a `fix_and_merge` item can produce an auditable proof bundle without mutating GitHub.

---

### Phase 5 — v2.0 Guarded Executor

**Objective:** Enable non-destructive guarded GitHub actions only through central executor.

**Files:**

- Modify: `internal/github/*`
- Modify: `internal/executor/*`
- Modify: `internal/cache/*`
- Modify: `internal/cmd/*`
- Modify: `internal/actions/*`
- Modify: `scripts/audit_guideline.py`
- Test: `internal/github/*_test.go`, `internal/executor/*_test.go`, `internal/cache/*_test.go`, `internal/cmd/*_test.go`

**Tasks:**

1. Add live preflight checks:
   - PR open
   - head SHA unchanged or revalidated
   - base branch allowed
   - CI/checks green for merge
   - mergeability clean for merge
   - branch protection/review requirements satisfied
   - token permission sufficient
   - rate-limit budget sufficient
   - policy allows action
   - idempotency key not already executed
2. Add append-only executor ledger.
3. Add guarded comment/label executor path.
4. Ensure guarded mode cannot merge, close, push, or update branches.
5. Add post-action verification for comment/label actions.

**Verification:**

```bash
go test ./internal/github ./internal/executor ./internal/cache ./internal/cmd ./internal/actions
python -m pytest -q tests/test_audit_guideline.py
```

**Exit gate:** guarded mode can perform comment/label actions in a test/fake/live-approved path, logs every transition, and cannot merge or close.

---

### Phase 6 — v2.0 Autonomous Mutation Policy

**Objective:** Add merge/close paths only after guarded mode, preflight, proof bundles, and ledger are green.

**Files:**

- Modify: `internal/executor/*`
- Modify: `internal/actions/*`
- Modify: `internal/workqueue/*`
- Modify: `internal/cmd/*`
- Modify: `internal/monitor/tui/*`
- Modify: `scripts/audit_guideline.py`

**Tasks:**

1. Add autonomous policy gates for:
   - `fast_merge`
   - `duplicate_close`
   - `reject_or_close`
   - `fix_and_merge` after proof validation
2. Add operator controls:
   - hold
   - resume
   - panic stop / no-new-actions
   - policy visibility
3. Add post-action verification and failure return-to-safe-state behavior.
4. Add audit checks proving autonomous mode cannot bypass preflight.

**Verification:**

```bash
go test ./...
make test-python
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
git diff --check
```

**Exit gate:** autonomous mode is impossible to bypass, idempotent, auditable, and safe under fake/e2e harness before any real mutation run.

---

### Phase 7 — Final OpenClaw Proof and Release Closeout

**Objective:** Prove the full 2.0 release surface on OpenClaw artifacts and runtime.

**Tasks:**

1. Build current HEAD.
2. Restart `pratc-autonomous` under tmux.
3. Run health and preflight.
4. Generate full-corpus workflow artifacts and PDF.
5. Generate ActionPlan.
6. Run dry-run/guarded/autonomous e2e according to enabled policy.
7. Run audit.
8. Refresh runtime proof.
9. Update docs and TODO.

**Verification:**

```bash
make build
go test ./...
make test-python
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
./bin/pratc workflow --repo openclaw/openclaw --max-prs=0 --sync-max-prs=0 --progress=false
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > autonomous/runs/<run-id>/action-plan.json
python3 scripts/audit_guideline.py autonomous/runs/<run-id>
curl -sf http://127.0.0.1:7400/healthz
curl -sf --max-time 5 http://100.112.201.95:7400/healthz
git diff --check
```

**Exit gate:** full-corpus v2 run audit-green, no direct swarm mutation path, guarded/autonomous gates proven, docs aligned.

---

## Worker Ownership Map

Use these lanes for assignment; do not run all 16 concurrently by default.

1. Governance/contracts — root docs and terminology audit
2. Go type surface — `internal/types/*`, `contracts/*`, fixtures
3. Python/TypeScript parity — `ml-service/src/pratc_ml/*`, retained TS types
4. Lane classifier — `internal/actions/classifier.go`
5. Policy profiles — `internal/actions/policy.go`
6. ActionPlan service — `internal/app/*`
7. CLI command — `cmd/pratc/*`, `internal/cmd/*`
8. HTTP API — `internal/cmd/*`, route tests
9. Persistence/migrations — `internal/cache/*`
10. Queue/leases — `internal/workqueue/*`
11. Dry-run executor — `internal/executor/*`
12. Live preflight — `internal/github/*`, `internal/executor/preflight.go`
13. Fix-and-merge sandbox — `internal/repo/*`, proof helpers
14. TUI dashboard — `internal/monitor/tui/*`, monitor data adapters
15. Audit checks — `scripts/audit_guideline.py`, audit tests
16. Integration/e2e — `tests/*`, `scripts/*`, runbook, proof smoke

---

## Current Recommended Swarm Shape

### Wave A — max 4 workers

1. ActionIntent completeness + audit
2. Queue API foundation
3. Proof bundle attach path
4. TUI PR detail inspector

Controller integrates and runs:

```bash
go test ./internal/types ./internal/actions ./internal/app ./internal/cache ./internal/workqueue ./internal/cmd ./internal/monitor/...
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
git diff --check
```

### Wave B — max 6-8 workers after Wave A is green

1. Live preflight checker
2. Guarded comment/label executor
3. Ledger persistence
4. Post-action verification
5. Fix-and-merge sandbox
6. TUI queue/proof/executor/rate-limit/audit panels
7. E2E harness and audit expansion
8. Docs/runbook sync

### Wave C — max 4-6 workers after Wave B is green

1. Autonomous merge/close policy path
2. Operator hold/resume/panic controls
3. Failure return-to-safe-state behavior
4. Full OpenClaw proof run and runtime proof refresh

---

## Definition of Done for 2.0

- Every PR is accounted for in ActionPlan.
- Every PR has exactly one primary action lane.
- Every ActionIntent has reasons, evidence refs, confidence, risk flags, preconditions, policy, and idempotency key.
- TUI can navigate corpus overview, action lanes, PR detail, queue, proof, executor, rate-limit/auth, and audit ledger state.
- Advisory mode performs zero writes.
- Guarded mode can only comment/label.
- Autonomous mode cannot bypass live preflight.
- Swarm workers can claim work and attach proof without direct GitHub mutation.
- Executor ledger is append-only and captures every transition.
- Failed preflight/execution/verification returns items to safe blocked/escalated states.
- OpenClaw full-corpus v2 run is audit-green.
