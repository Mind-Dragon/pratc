# OpenFang Status Activity and Liveness Upgrade

## TL;DR
> **Summary**: Upgrade `scripts/openfang-status.sh` so operators can reliably tell whether the system is making progress, how long to wait before escalating, and which agents are healthy, stale, or likely crashed.
> **Deliverables**:
> - Agent liveness table with health classification
> - Tick-age and no-progress detectors with explicit wait guidance
> - Crash/stall signal section (daemon, dispatch, approvals)
> - Configurable thresholds via environment variables
> - Regression-safe test suite for status logic
> **Effort**: Short
> **Parallel**: YES - 2 waves
> **Critical Path**: Task 1 -> Task 2 -> Task 4 -> Task 6 -> Task 8

## Context
### Original Request
User asked for a status experience that answers:
- Is it actually doing something?
- How long should I wait?
- Are agents active, stalled, or crashed?

### Interview Summary
- Existing `openfang-status.sh` currently shows task counts, last tick age, dispatch result, approvals, and recent events.
- Existing script does not classify agent freshness/liveness from `openfang agent list --json` fields (`last_active`, `state`, `ready`, `auth_status`).
- Current scheduler cadence: overseer every 10 minutes, approval sweeper every 1 minute.

### Metis Review (gaps addressed)
- Add explicit derived health states and escalation thresholds.
- Add no-progress detection without changing OpenFang core behavior.
- Harden error handling so status script never crashes orchestrator loop callers.
- Add deterministic tests for healthy/idle/stale/failure scenarios.

## Work Objectives
### Core Objective
Make `scripts/openfang-status.sh` a trustworthy operator panel that reports real activity, flags stalled/crashed conditions early, and gives concrete wait-time guidance.

### Deliverables
- Agent health section based on live agent metadata.
- System health summary with severity (`OK`, `WARN`, `CRITICAL`).
- No-progress monitor tied to task remaining counts and tick history.
- Guidance block with "wait vs intervene" instructions.
- Tests and fixtures for classification and fallback behavior.

### Definition of Done (verifiable conditions with commands)
- `scripts/openfang-status.sh --once` exits `0` and prints: `Agent Health`, `System Health`, and `Operator Guidance` sections.
- `scripts/openfang-status.sh --watch --interval 5` continuously refreshes and never exits on transient OpenFang API errors.
- `uv run pytest -v scripts/tests/test_openfang_status_activity.py` passes.
- `OPENFANG_IDLE_SECONDS=1 OPENFANG_STALE_SECONDS=2 scripts/openfang-status.sh --once` reflects aggressive thresholds in output.
- With simulated unavailable agent list, script still exits `0` and prints degraded-mode warning.

### Must Have
- Read-only status behavior (no agent mutation operations).
- Agent classification from `state`, `ready`, `last_active`, `auth_status`.
- Wait guidance tied to overseer cadence (10m default) and stale thresholds.
- Persistent no-progress memory using a dedicated status snapshot file.
- Configurable thresholds documented in script header.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- No changes to `scripts/openfang-overseer.sh` dispatch logic for this task.
- No restarts/kills/model changes from status script.
- No UI/web dashboard additions; CLI output only.
- No estimated project completion time claims.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: tests-after (unit + command smoke tests).
- QA policy: every task has happy + failure scenario with evidence artifacts.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.txt`

## Execution Strategy
### Parallel Execution Waves
Wave 1: classification + resilience foundation
- Tasks 1-4

Wave 2: guidance, no-progress UX, and validation closure
- Tasks 5-8

### Dependency Matrix (full, all tasks)
- 1 blocks 2,3
- 2 blocks 4,5
- 3 blocks 6
- 4 blocks 7
- 5 blocks 7
- 6 blocks 8
- 7 blocks 8

### Agent Dispatch Summary (wave -> task count -> categories)
- Wave 1 -> 4 tasks -> quick, unspecified-high
- Wave 2 -> 4 tasks -> quick, writing, unspecified-high

## TODOs
> Implementation + Test = ONE task. Never separate.

- [ ] 1. Define Liveness Model and Threshold Defaults

  **What to do**: Define deterministic health states and thresholds used by status script:
  - `HEALTHY`: agent `state=Running`, `ready=true`, `last_active_age <= OPENFANG_IDLE_SECONDS`
  - `IDLE`: running+ready but `last_active_age` between idle and stale thresholds
  - `STALE`: running+ready but `last_active_age > OPENFANG_STALE_SECONDS`
  - `UNREADY`: `ready=false` or `state != Running`
  - `AUTH_ISSUE`: `auth_status` missing/unknown/missing-auth while stale
  Defaults:
  - `OPENFANG_IDLE_SECONDS=900`
  - `OPENFANG_STALE_SECONDS=3600`
  - `OPENFANG_OVERSEER_WARN_SECONDS=900`
  - `OPENFANG_NO_PROGRESS_TICKS=3`
  **Must NOT do**: Do not infer health only from `state=Running`.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: bounded logic contract.
  - Skills: `[]`
  - Omitted: `frontend-ui-ux` — Reason: CLI status only.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 2,3 | Blocked By: none

  **References**:
  - Pattern: `scripts/openfang-status.sh` — current render pipeline.
  - API/Type: `openfang agent list --json` output fields.
  - Pattern: `/home/agent/.openfang/overseer-state.json` — tick metadata.

  **Acceptance Criteria**:
  - [ ] Health-state definitions and defaults are encoded and documented in script comments.
  - [ ] Classification logic handles missing/invalid timestamps safely.

  **QA Scenarios**:
  ```
  Scenario: Happy path healthy and stale mix
    Tool: Bash
    Steps: Feed fixture agent JSON with mixed last_active ages to classifier path
    Expected: At least one HEALTHY and one STALE classification appears
    Evidence: .sisyphus/evidence/task-1-health-classification.txt

  Scenario: Failure/edge case missing last_active
    Tool: Bash
    Steps: Feed fixture with null/missing last_active
    Expected: Script does not crash; classifies as UNKNOWN/UNREADY safely
    Evidence: .sisyphus/evidence/task-1-missing-timestamp.txt
  ```

  **Commit**: YES | Message: `feat(status): define liveness model and defaults` | Files: `scripts/openfang-status.sh`

- [ ] 2. Add Agent Health Section to Status Output

  **What to do**: Extend output with `Agent Health` table columns:
  `name`, `state`, `ready`, `auth_status`, `last_active_age`, `health_class`.
  Include summary counts per class.
  **Must NOT do**: Do not remove existing top-level summary lines.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: single-script output enhancement.
  - Skills: `[]`
  - Omitted: `writing` — Reason: implementation-focused task.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 4,5 | Blocked By: 1

  **References**:
  - Pattern: `scripts/openfang-status.sh:103` — existing output sections.
  - API/Type: `openfang agent list --json` — per-agent telemetry.

  **Acceptance Criteria**:
  - [ ] `scripts/openfang-status.sh --once` prints `Agent Health` section.
  - [ ] Section still renders if agent list call fails.

  **QA Scenarios**:
  ```
  Scenario: Happy path agent table render
    Tool: Bash
    Steps: Run scripts/openfang-status.sh --once with live daemon
    Expected: Agent Health table includes all active agents with health classes
    Evidence: .sisyphus/evidence/task-2-agent-table.txt

  Scenario: Failure path agent list unavailable
    Tool: Bash
    Steps: Simulate failed agent list command in test harness
    Expected: Degraded warning shown; script exits 0
    Evidence: .sisyphus/evidence/task-2-agent-list-failure.txt
  ```

  **Commit**: YES | Message: `feat(status): add agent health table` | Files: `scripts/openfang-status.sh`

- [ ] 3. Add No-Progress Tracking Snapshot

  **What to do**: Add lightweight snapshot file `/home/agent/.openfang/status-progress.json` storing:
  - `last_remaining`
  - `last_next_task`
  - `last_remaining_change_at`
  - `ticks_without_progress`
  Update on each status render and compute stagnation warning.
  **Must NOT do**: Do not modify `overseer-state.json` schema.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: stateful logic + edge handling.
  - Skills: `[]`
  - Omitted: `systematic-debugging` — Reason: feature enhancement, not incident repair.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 6 | Blocked By: 1

  **References**:
  - Pattern: `/home/agent/.openfang/overseer-state.json` — source remaining/next_task.
  - Pattern: `scripts/openfang-status.sh` — render loop and watch behavior.

  **Acceptance Criteria**:
  - [ ] Progress snapshot file is created/updated safely.
  - [ ] `ticks_without_progress` increases only when `remaining` is unchanged.

  **QA Scenarios**:
  ```
  Scenario: Happy path progress reset
    Tool: Bash
    Steps: Simulate remaining decrease across two renders
    Expected: ticks_without_progress resets to 0
    Evidence: .sisyphus/evidence/task-3-progress-reset.txt

  Scenario: Failure/edge case corrupted snapshot file
    Tool: Bash
    Steps: Preload malformed status-progress.json then run status
    Expected: Script recovers with fresh snapshot and exits 0
    Evidence: .sisyphus/evidence/task-3-progress-corruption.txt
  ```

  **Commit**: YES | Message: `feat(status): add no-progress snapshot tracking` | Files: `scripts/openfang-status.sh`

- [ ] 4. Add System Health Severity and Wait Guidance

  **What to do**: Add `System Health` and `Operator Guidance` sections with deterministic messages:
  - `OK`: tick age <= warn threshold, dispatch sent recently, no critical stale required agents
  - `WARN`: tick overdue OR multiple stale agents OR manual approvals pending
  - `CRITICAL`: repeated timeout/no send + no-progress ticks exceed threshold
  Print explicit wait instruction:
  - normal: "wait up to 12m before escalation"
  - warn: "if unchanged for 15m, run one manual tick"
  - critical: "intervene now"
  **Must NOT do**: Do not print ambiguous phrases like "maybe stuck".

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: deterministic output logic.
  - Skills: `[]`
  - Omitted: `writing` — Reason: functional output changes.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 7 | Blocked By: 2

  **References**:
  - Pattern: `scripts/openfang-status.sh:87-110` — existing summary and events.
  - Pattern: `/home/agent/.openfang/overseer.log` — dispatch timeout/sent signals.

  **Acceptance Criteria**:
  - [ ] Status output includes one severity label (`OK|WARN|CRITICAL`).
  - [ ] Wait-time recommendation appears in all severities.

  **QA Scenarios**:
  ```
  Scenario: Happy path healthy cadence
    Tool: Bash
    Steps: Use fresh last_tick and sent dispatch state fixture
    Expected: System Health=OK with "wait up to 12m"
    Evidence: .sisyphus/evidence/task-4-healthy-guidance.txt

  Scenario: Failure path repeated timeout + stagnation
    Tool: Bash
    Steps: Use stale tick + timeout dispatch + high no-progress ticks fixture
    Expected: System Health=CRITICAL with "intervene now"
    Evidence: .sisyphus/evidence/task-4-critical-guidance.txt
  ```

  **Commit**: YES | Message: `feat(status): add system severity and wait guidance` | Files: `scripts/openfang-status.sh`

- [ ] 5. Add Non-Interactive Smoke Tests

  **What to do**: Add command-level smoke tests for:
  - normal render
  - watch loop first cycle
  - threshold overrides
  - degraded API mode
  **Must NOT do**: Do not rely on manual inspection as pass criterion.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: straightforward verification scripts.
  - Skills: `[]`
  - Omitted: `deep` — Reason: no architecture complexity.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 7 | Blocked By: 2

  **References**:
  - Pattern: `scripts/openfang-status.sh --once|--watch --interval`.

  **Acceptance Criteria**:
  - [ ] Smoke suite exits 0 in healthy mode.
  - [ ] Smoke suite verifies degraded-mode resilience.

  **QA Scenarios**:
  ```
  Scenario: Happy path smoke suite
    Tool: Bash
    Steps: Run scripted smoke checks against live daemon
    Expected: All checks pass and produce output artifacts
    Evidence: .sisyphus/evidence/task-5-smoke-pass.txt

  Scenario: Failure path forced API timeout
    Tool: Bash
    Steps: Mock/force openfang call timeout in test harness
    Expected: Script prints degraded state and exits 0
    Evidence: .sisyphus/evidence/task-5-smoke-degraded.txt
  ```

  **Commit**: YES | Message: `test(status): add non-interactive smoke coverage` | Files: `scripts/tests/*`

- [ ] 6. Add Unit Tests for Classification and Progress Logic

  **What to do**: Add `uv`/pytest tests with fixtures covering:
  - health class decisions
  - future timestamp clock skew
  - stale non-required agents
  - no-progress increment/reset
  - corrupted snapshot recovery
  **Must NOT do**: Do not leave thresholds untested.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: richer logic coverage.
  - Skills: `[]`
  - Omitted: `test-driven-development` — Reason: task already defined as test-first execution in plan wave.

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: 8 | Blocked By: 3

  **References**:
  - Pattern: existing `uv run pytest -v` contract in `AGENTS.md`.

  **Acceptance Criteria**:
  - [ ] `uv run pytest -v scripts/tests/test_openfang_status_activity.py` passes.
  - [ ] All edge-case fixtures are covered.

  **QA Scenarios**:
  ```
  Scenario: Happy path unit suite
    Tool: Bash
    Steps: Run uv run pytest -v scripts/tests/test_openfang_status_activity.py
    Expected: All tests pass
    Evidence: .sisyphus/evidence/task-6-pytest-pass.txt

  Scenario: Failure path regression guard
    Tool: Bash
    Steps: Temporarily mutate threshold expectation fixture and run tests
    Expected: Test fails (guard works), then restored and passes
    Evidence: .sisyphus/evidence/task-6-regression-guard.txt
  ```

  **Commit**: YES | Message: `test(status): add unit coverage for liveness and progress` | Files: `scripts/tests/*`

- [ ] 7. Wire Threshold Configuration and Inline Help

  **What to do**: Add env var reads and print active threshold values in status output footer (`idle`, `stale`, `warn`, `no_progress_ticks`).
  **Must NOT do**: Do not hardcode hidden constants.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: compact config surface.
  - Skills: `[]`
  - Omitted: `writing-skills` — Reason: code-level config change.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 8 | Blocked By: 4,5

  **References**:
  - Pattern: existing env-independent constants in `scripts/openfang-status.sh`.

  **Acceptance Criteria**:
  - [ ] Changing env vars changes classification behavior in output.
  - [ ] Footer prints active values for operator clarity.

  **QA Scenarios**:
  ```
  Scenario: Happy path custom thresholds
    Tool: Bash
    Steps: Set OPENFANG_IDLE_SECONDS/OPENFANG_STALE_SECONDS and run status once
    Expected: Output reflects overridden thresholds and changed classifications
    Evidence: .sisyphus/evidence/task-7-threshold-override.txt

  Scenario: Failure path invalid env values
    Tool: Bash
    Steps: Set non-numeric threshold value and run status once
    Expected: Script falls back to defaults and reports warning without crash
    Evidence: .sisyphus/evidence/task-7-invalid-threshold.txt
  ```

  **Commit**: YES | Message: `feat(status): add configurable thresholds and footer` | Files: `scripts/openfang-status.sh`

- [ ] 8. Final Operator Validation and Docs Update

  **What to do**: Update operational docs with interpretation guide:
  - what each health class means
  - how long to wait at each severity
  - exact intervention command sequence
  Validate against live daemon once and archive output.
  **Must NOT do**: Do not leave undocumented severity meanings.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: operator docs + handoff clarity.
  - Skills: `[]`
  - Omitted: `artistry` — Reason: deterministic runbook text.

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: none | Blocked By: 6,7

  **References**:
  - Pattern: `docs/runbooks/openfang-opencode-operations.md` (if present).
  - Pattern: `scripts/openfang-status.sh` final output format.

  **Acceptance Criteria**:
  - [ ] Runbook includes severity-to-action matrix.
  - [ ] Live status sample captured after update.

  **QA Scenarios**:
  ```
  Scenario: Happy path operator interpretation
    Tool: Bash
    Steps: Run status once and map output to runbook actions
    Expected: Every section has clear next action
    Evidence: .sisyphus/evidence/task-8-runbook-alignment.txt

  Scenario: Failure path stale system response
    Tool: Bash
    Steps: Use stale fixture output and follow runbook commands
    Expected: Escalation path is explicit and reproducible
    Evidence: .sisyphus/evidence/task-8-stale-escalation.txt
  ```

  **Commit**: YES | Message: `docs(status): add liveness interpretation and escalation guide` | Files: `docs/*`, `scripts/openfang-status.sh`

## Final Verification Wave (4 parallel agents, ALL must APPROVE)
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Commit each task atomically with evidence artifacts.
- Keep script changes small and test-backed.
- No refactor beyond status-script scope.

## Success Criteria
- Operators can tell within one screen whether work is progressing or stalled.
- Status output explicitly answers "how long should I wait" per severity.
- Agent freshness/crash signals are visible and actionable.
- Script remains resilient and non-crashing under partial API failure.
