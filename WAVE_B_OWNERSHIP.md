# Wave B Ownership Map — prATC v1.8/v1.9 Foundation

**Phase:** Iteration B — Guarded Executor Foundation  
**Prerequisite:** Wave A complete (commit `0f2ebf0`, proof `96fbd4e`, STATE.yaml `wave-a-complete`)  
**Max concurrent workers:** 6 (first wave), expand to 8 after B2-B3 green  
**Controller:** Hermes (integration, state, verification, commits)

---

## Worker Assignments

### Worker B1 — TUI Dashboard Integration (COMPLETED ✓)

**Status:** Implementation done; pending integration test and smoke verification.  
**Owner:** pre-implemented (untracked files)  
**Files:**
- `internal/monitor/tui/audit.go` — AuditLedgerPanel
- `internal/monitor/tui/corpus.go` — CorpusOverviewPanel
- `internal/monitor/tui/executor.go` — ExecutorConsolePanel
- `internal/monitor/tui/corpus_test.go`, `executor_test.go`

**Integration tasks remaining:**
- connect data.Store live updates through monitor broadcaster
- verify `./bin/pratc monitor --once` renders all panels without error
- add tests for panel state transitions

---

### Worker B2 — Live Preflight Checker

**Goal:** All 9 preflight gates enforced before any GitHub mutation.  
**Files:**
- `internal/executor/preflight.go` (new) — gate functions and aggregation
- `internal/github/pr.go`, `internal/github/check.go` (extend) — SHA, CI, mergeability, branch protection
- `internal/executor/executor.go` (modify) — call preflight before ExecuteIntent
- Tests: `internal/executor/preflight_test.go`, extend `executor_test.go`

**Test targets:**
- SHA revalidation pass/fail
- CI green/red/failing
- mergeability clean/blocked
- branch protection satisfied/denied
- token permission check
- rate-limit budget gate
- idempotency already-executed rejection

**Barrier:** preflight checks deterministic, 100% branch coverage, audit check added.

---

### Worker B3 — Guarded Comment/Label Executor (Fake Backend)

**Goal:** Safe non-destructive mutations behind executor ledger; no real GitHub writes.  
**Files:**
- `internal/executor/mutator.go` (new) — comment/label actions with fake GitHub client
- `internal/github/fake_client.go` (extend) — record proposed mutations in memory
- `internal/executor/ledger.go` (extend) — append mutation attempts
- Tests: `internal/executor/mutator_test.go`, `internal/cmd/execute_test.go`

**Constraints:**
- `--dry-run` mode defaults true; guarded mode opt-in via `--policy=guarded`
- No merge/close/push in guarded mode
- Every attempted mutation must be logged with intent ID, preflight hash, timestamp

**Barrier:** fake backend tests pass, audit verifies zero live mutations in dry-run.

---

### Worker B4 — Ledger Persistence (SQLite)

**Goal:** Replace `MemoryLedger` with crash-resilient append-only SQLite ledger.  
**Files:**
- `internal/cache/ledger.go` (new) — `Ledger` interface, SQLite impl with migrations
- `internal/executor/ledger.go` (adapt) — use persistent ledger
- Schema migration in `internal/cache/sqlite.go` — add `executor_ledger` table
- Tests: `internal/cache/ledger_test.go`, `internal/executor/ledger_integration_test.go`

**Schema:**
```sql
CREATE TABLE executor_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    intent_id TEXT NOT NULL,
    transition TEXT NOT NULL,  -- proposed/preflighted/executed/verified/failed
    preflight_snapshot TEXT NOT NULL,
    mutation_snapshot TEXT,
    timestamp TEXT NOT NULL,
    UNIQUE(intent_id, transition)
);
```

**Barrier:** ledger survives process crash; idempotency enforced; exactly-once guarantees tested.

---

### Worker B5 — Post-Action Verification

**Goal:** Confirm GitHub state after mutation; return to safe state on failure.  
**Files:**
- `internal/executor/verifier.go` (new) — poll GitHub API to confirm comment/label applied
- `internal/executor/executor.go` (modify) — verification loop after mutation
- `internal/executor/state_machine.go` (modify) — `failed` → `escalated`/`blocked` transitions
- Tests: `internal/executor/verifier_test.go`

**Behavior:**
- After fake/real mutation, re-fetch PR state
- If expected state missing, mark `failed` and trigger return-to-safe-state
- Safe states: `proposed`, `claimable`, `blocked`, `escalated`

**Barrier:** verifier tests pass for both fake and real GitHub backends.

---

### Worker B6 — Fix-and-Merge Sandbox

**Goal:** Local proof bundle generation for `fix_and_merge` items (dry-run only).  
**Files:**
- `internal/repo/checkout.go` (new) — isolated worktree creation, branch checkout
- `internal/repo/patch.go` (new) — apply patch/rebase, capture proof
- `internal/repo/test.go` (new) — run test command, capture stdout/stderr/exit
- `internal/executor/fixmerge.go` (new) — orchestrate checkout→patch→test→bundle
- `internal/workqueue/proof.go` (extend) — store proof bundle refs
- Tests: `internal/repo/checkout_test.go`, `internal/executor/fixmerge_test.go`

**Proof bundle shape:**
```go
type ProofBundle struct {
    WorkItemID   string
    WorkerID     string
    PatchDiff    string
    TestCommand  string
    TestOutput   string
    TestExitCode int
    Artifacts    []string
    Status       string // ok/failed
}
```

**Barrier:** dry-run only, no GitHub writes; proof bundles attached to queue items and pass validation.

---

### Worker B7 — E2E Harness and Audit Expansion

**Goal:** End-to-end guarded-mode proof with fake GitHub; audit coverage for v2 gates.  
**Files:**
- `tests/test_executor_e2e.py` or `internal/executor/e2e_test.go`
- `scripts/audit_guideline.py` (extend) — v2 guard checks (preflight, ledger, verification, proof)
- `tests/test_audit_v2_guarded.py` (new)

**Harness flow:**
1. Build ActionPlan from current corpus (advisory)
2. Claim 3–5 work items across lanes
3. Run preflight → mutate (fake) → verify → ledger append
4. Attach proof bundles
5. Run full audit; expect green

**Barrier:** e2e passes locally with fake backend; audit reports 0 required failures.

---

### Worker B8 — Docs and Runbook Sync

**Goal:** Keep operational docs aligned with guarded-mode implementation.  
**Files:**
- `autonomous/RUNBOOK.md` — Wave B command sequence, health probes, resume paths
- `docs/PLAN.md` (prATC) — mark Wave B active or complete
- `VERSION2.0.md` — update implementation progress table
- `GUIDELINE.md` — clarify guarded/autonomous policy boundaries if needed

**Deliverables:**
- RUNBOOK Wave B chapter: `pratc serve`, `pratc actions`, `pratc monitor`, `controller audit-state`
- Update `autonomous/GAP_LIST.md` from any new audit failures
- Refresh `autonomous/STATE.yaml` schema if phase names extend

**Barrier:** doc-check tests green; no stale cross-references.

---

## Sequencing and Barriers

| Step | Worker set | Barrier check |
|------|------------|---------------|
| B0 | controller | Wave A proof committed, STATE.yaml closed |
| B1 | pre-done | TUI panels compile, monitor renders (non-fatal if data missing) |
| B2 | B2 | `go test ./internal/executor ./internal/github` with preflight coverage ≥ 90% |
| B3 | B3 | executor mutator tests pass, fake ledger records all transitions |
| B4 | B4 | ledger migration works fresh & upgrade, crash-recovery test passes |
| B5 | B5 | verifier confirms fake mutations, failure fallback tested |
| B6 | B6 | fix-merge sandbox produces proof bundle; tests deterministic |
| B7 | B7 | e2e guarded harness completes, audit green |
| B8 | B8 | docs synced, diff-clean, doc-check tests green |

Start **B2–B4 in parallel** after B1 integration (max 3–4 workers). Add B5–B6 after B3–B4 green. B7 after B6 green. B8 in parallel with B5–B7.

---

## Verification Gate Before Wave C

```bash
git diff --check
go test ./internal/executor ./internal/github ./internal/cache ./internal/monitor/... -v
./bin/pratc monitor --once || true
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
python3 scripts/autonomous_controller.py audit-state  # must be clean
```

Wave C (autonomous mutation policy) requires Wave B gates fully green.

---

**Controller:** Hermes  
**Last updated:** 2026-04-24 Central (after Wave A closeout commit `3c613e3`)  
**Current HEAD:** `3c613e3` (proof: Wave A closeout — implementation boundary 0f2ebf0)
