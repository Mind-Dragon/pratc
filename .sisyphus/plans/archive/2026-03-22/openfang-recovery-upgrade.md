# OpenFang Recovery and Throughput Upgrade Plan

## TL;DR
> **Summary**: The system is not primarily down; it is deadlocked by control-loop design. Work is being produced in workspaces, but promotion to dependency-satisfying states is unreliable.
> **Deliverables**:
> - Canonical task-state parsing and normalization
> - Deterministic progression loop (dispatch -> work_done -> merged -> verified)
> - Coordinator-timeout bypass + task-executor recovery dispatch
> - Approval queue throughput safeguards
> - Anti-regression checks for deadlock prevention
> **Effort**: Medium
> **Parallel**: YES - 4 waves
> **Critical Path**: Normalize state -> enforce transition guard -> promotion/integration drain -> deadlock detector -> full-run verification

## Context
### Original Request
Perform RCA/plan/upgrade/comprehensive analysis for why OpenFang remains stuck and how to make it progress across all tasks.

### Verified Runtime Findings
- Agents are healthy and active, but coordinator message calls frequently exceed timeout windows.
- Work accumulates in multiple workspaces (`coder`, `orchestrator`, etc.) with substantial dirty trees.
- Status artifacts were inconsistent (`task id`/`status` vs `task_id`/`state`, `completed` alias), causing undercounted progress.
- Parser normalization increased visible progress from `done_like=2` to `done_like=5`.
- Dependency gating still requires `merged/verified`, so tasks labeled `done/completed` do not unblock downstream tasks.
- Manual approval churn can delay progress if sweeper cadence misses active burst windows.

### Metis Gap Review (Applied)
- Missing canonical write boundary for task status transitions.
- Missing deterministic promotion owner (`done -> merged -> verified`).
- Missing deadlock detector and failover policy when coordinators timeout repeatedly.

## Work Objectives
### Core Objective
Convert current pseudo-progress into deterministic end-to-end completion by making status semantics canonical and making merge/verification promotion a first-class loop operation.

### Definition of Done
- [x] `openfang-status.sh` reflects canonical task state counts with no schema drift.
- [x] At least one stalled dependency-blocking task is promoted to `merged` or `verified` via deterministic loop.
- [x] Coordinator timeout no longer halts progression (recovery dispatch path active).
- [x] Approval queue does not accumulate duplicate low-risk shell commands for >2 sweep intervals.
- [x] System can run 3 consecutive overseer ticks without losing `last_tick` freshness or regressing task counts.

## Verification Strategy
- Test decision: tests-after (script-level + runtime assertions)
- Required runtime checks:
  - `bash -n scripts/openfang-overseer.sh scripts/openfang-status.sh scripts/openfang-approval-sweeper.sh`
  - `./scripts/openfang-overseer.sh` (full tick, no forced kill)
  - `./scripts/openfang-status.sh` (confirm task count, severity reason, workspace activity)
  - `openfang approvals list` (queue depth and stale/manual behavior)
- Evidence output:
  - `.sisyphus/evidence/recovery-overseer-tick.txt`
  - `.sisyphus/evidence/recovery-status-snapshot.txt`
  - `.sisyphus/evidence/recovery-approvals.txt`

## Execution Strategy

### Wave 1 - State Canonicalization (Foundation)
- [x] W1.1 Normalize task status parsing across all readers.
  - **What to do**: Accept `task id|task_id` and `status|state`; map aliases (`completed -> done`, `inprogress -> in_progress`).
  - **Acceptance**: status counts increase when only schema variant changes; no false invalid-state warnings.
- [x] W1.2 Enforce one canonical internal state map for gate checks.
  - **What to do**: Internal gate should evaluate normalized states only.
  - **Acceptance**: same status artifact interpreted identically in `overseer` and `status` paths.

### Wave 2 - Liveness to Throughput (Recovery Dispatch)
- [x] W2.1 Add recovery-mode coordinator ordering.
  - **What to do**: On stagnation threshold, prioritize `task-executor` before planner/orchestrator.
  - **Acceptance**: log shows recovery-mode coordinator order and attempt sequence.
- [x] W2.2 Add timeout fallback policy.
  - **What to do**: In recovery mode, timeout on one coordinator must continue to next coordinator in same tick.
  - **Acceptance**: single tick logs multiple coordinator attempts before declaring failure.

### Wave 3 - Promotion Drain (Deadlock Breaker)
- [x] W3.1 Add deterministic integration-drain operation.
  - **What to do**: If earliest blocking task is `done` and not `merged/verified`, run a dedicated promotion path (verify -> merge -> verify-on-main).
  - **Acceptance**: at least one task transitions from `done` to `merged` or explicitly `blocked` with structured failure reason.
- [x] W3.2 Add artifact reconciliation from workspaces.
  - **What to do**: Continue reverse-sync of task/evidence artifacts and record source workspace in logs.
  - **Acceptance**: host `.sisyphus/status` updates from workspace-origin files without manual copy.

### Wave 4 - Approval Throughput Hardening
- [x] W4.1 Add request dedup safeguards for recurring shell commands.
  - **What to do**: prevent repeated identical low-risk requests from queue growth.
  - **Acceptance**: repeated `ls/mkdir` bursts do not increase pending queue linearly.
- [x] W4.2 Keep strict reject list for truly unsafe operations.
  - **What to do**: continue reject for `git config`, `reset --hard`, and stale pending operations.
  - **Acceptance**: unsafe approvals are rejected automatically with explicit reason tags in logs.

## Failure Budget and Escalation Policy
- Coordinator timeout budget: max 2 sequential timeouts per coordinator per tick before fallback.
- Stagnation threshold: 3 ticks without remaining-count change triggers recovery mode.
- Hard escalation: 6+ stagnation ticks AND no promotion success -> require integrator-focused recovery task dispatch (not broad orchestrator prompt).
- Human-required escalation only when:
  - merge conflict cannot be auto-resolved,
  - verification gate on `main` fails twice consecutively for same task,
  - approval queue includes high-risk command outside policy.

## Anti-Regression Checks
- [x] AR1: status parser accepts both legacy and frontmatter schema variants.
- [x] AR2: `done_like` count does not regress when only field naming changes.
- [x] AR3: recovery mode attempts at least two coordinators before fail.
- [x] AR4: recovery mode does not classify every timeout as success.
- [x] AR5: workspace activity and task status remain visible in one status snapshot.
- [x] AR6: pending approval queue remains bounded under repeated safe shell requests.

## Immediate Next 3 Actions
1. Execute one full-length overseer tick (no external timeout kill) to allow complete recovery attempt chain and state write.
2. Run a single-task integration recovery for Task 1 (`verify -> merge -> post-merge verify`), then update `.sisyphus/status/task-1.md` to canonical state.
3. Re-run status + approvals checks and confirm progression signal changed from stagnation-only to promotion progress.

## Success Criteria
- `done_like` is accurate and stable across schema variants.
- `next_task` advances after dependency blockers are promoted, not just after coordinator chatter.
- CRITICAL reflects real blocked promotion failures, not parser drift or stale coordinator semantics.
- Throughput is measurable by task state transitions (`done -> merged -> verified`), not only agent heartbeat activity.
