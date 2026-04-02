# Audit Plan Correction and Artifact Archival

## TL;DR
> **Summary**: Correct `.sisyphus` planning state by archiving factually incorrect planning artifacts and activating a dedicated audit reconciliation plan as the sole active plan pointer.
> **Deliverables**:
> - Archived incorrect planning artifact(s) with move manifest
> - New audit reconciliation plan in `.sisyphus/plans/`
> - Updated `.sisyphus/boulder.json` pointing to the audit plan
> - Verification evidence proving pointer/file-system consistency
> **Effort**: Short
> **Parallel**: YES - 3 waves
> **Critical Path**: T1 → T3 → T4 → T5 → T6

## Context
### Original Request
Archive any incorrect plan artifacts and set up the correct plan for audit.

### Interview Summary
- User explicitly requested archival of incorrect planning artifacts.
- User explicitly requested setup of the correct plan for audit.
- Current active pointer references OpenClaw experiment plan, which is off-target for audit-focused active planning.

### Metis Review (gaps addressed)
- Added explicit policy separating “incorrect/stale” artifacts from “completed but valid” artifacts.
- Added guardrail to prevent scope creep into product remediation implementation.
- Added acceptance checks for active pointer integrity and non-archive active-plan invariant.
- Added failure-path QA for stale/nonexistent active plan pointers.

## Work Objectives
### Core Objective
Establish a decision-complete audit-focused active plan state in `.sisyphus` with traceable archival of incorrect planning artifacts.

### Deliverables
- Incorrect artifact classification manifest (evidence-backed).
- Archived stale draft(s) with preserved traceability index.
- New active audit reconciliation plan file.
- Updated active-plan metadata in `.sisyphus/boulder.json`.
- Verification bundle with pass/fail evidence.

### Definition of Done (verifiable conditions with commands)
- `python - <<'PY'\nimport json;d=json.load(open('.sisyphus/boulder.json'));print(d['active_plan']);print(d['plan_name'])\nPY` prints audit plan path/name.
- `python - <<'PY'\nimport json,os;d=json.load(open('.sisyphus/boulder.json'));p=d['active_plan'];assert os.path.isfile(p),p;assert '/archive/' not in p,p;print('OK')\nPY` prints `OK`.
- `test -f .sisyphus/plans/audit-reconciliation.md` exits 0.
- `test -f .sisyphus/drafts/archive/2026-03-26/pratc-v0.2-alignment.md` exits 0.
- `test ! -f .sisyphus/drafts/pratc-v0.2-alignment.md` exits 0.

### Must Have
- Archive only evidence-backed incorrect/stale planning artifacts.
- Keep audit correction work constrained to `.sisyphus/*.md` and `.sisyphus/boulder.json`.
- Maintain traceability with explicit source→destination archival manifest.
- Activate exactly one non-archive audit plan in `.sisyphus/plans/`.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- No source-code edits outside `.sisyphus` planning/state artifacts.
- No reopening of v0.1/v0.2 implementation work.
- No ambiguous “looks correct” checks; all checks must be command-verifiable.
- No active pointer to archived or nonexistent plan file.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: tests-after (artifact/state verification after edits)
- QA policy: Every task includes happy + failure scenarios with concrete commands
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
Wave 1: Classification and foundational setup (T1)
Wave 2: Artifact archival + audit plan creation (T2, T3)
Wave 3: Activation, verification, reconciliation (T4, T5, T6)

### Dependency Matrix (full, all tasks)
- T1 blocks T2 and T3.
- T2 and T3 block T4.
- T4 blocks T5.
- T5 blocks T6.

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1: 1 task → deep
- Wave 2: 2 tasks → unspecified-low, writing
- Wave 3: 3 tasks → unspecified-low, unspecified-high, writing

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Build incorrect-artifact classification manifest (RED phase)

  **What to do**: Produce an evidence-backed classification list of planning artifacts as `incorrect/stale`, `completed-valid`, or `active-target`.
  **Must NOT do**: Do not archive anything in this task.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: requires evidence reconciliation across plan/draft/state files.
  - Skills: []
  - Omitted: [`test-driven-development`] — no source-code behavior change.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T2,T3 | Blocked By: none

  **References**:
  - Pattern: `.sisyphus/boulder.json:2,12` — current active pointer and plan name
  - Pattern: `.sisyphus/drafts/pratc-v0.2-alignment.md:6,15-17` — stale active-plan assumptions
  - Pattern: `.sisyphus/plans/openclaw-5000pr-minimax-voyage.md:455-478` — completed final verification state
  - External: `AGENTS.md` — scope guardrails for v0.1/v0.2 boundaries

  **Acceptance Criteria**:
  - [ ] Evidence file lists every candidate with classification and rationale.
  - [ ] Manifest explicitly identifies `.sisyphus/drafts/pratc-v0.2-alignment.md` as stale.
  - [ ] Manifest explicitly states whether OpenClaw plan is archived now or retained as completed reference.

  **QA Scenarios**:
  ```
  Scenario: Happy path classification
    Tool: Bash
    Steps: read pointer + listed artifacts and generate markdown manifest
    Expected: manifest contains path, class, rationale, and action for each candidate
    Evidence: .sisyphus/evidence/task-1-artifact-classification.md

  Scenario: Missing evidence guard
    Tool: Bash
    Steps: attempt to classify a file without path-based proof
    Expected: classification rejected with "evidence missing" marker
    Evidence: .sisyphus/evidence/task-1-missing-evidence-guard.txt
  ```

  **Commit**: YES | Message: `docs(sisyphus): classify incorrect planning artifacts` | Files: `.sisyphus/evidence/task-1-artifact-classification.md`

- [x] 2. Archive stale draft artifact(s) with traceability

  **What to do**: Move stale draft(s) approved by T1 to `.sisyphus/drafts/archive/2026-03-26/` and create a move manifest.
  **Must NOT do**: Do not archive active or merely completed-valid plans unless T1 explicitly marks them stale/incorrect.

  **Recommended Agent Profile**:
  - Category: `unspecified-low` — Reason: deterministic file operations in `.sisyphus`.
  - Skills: []
  - Omitted: [`systematic-debugging`] — no runtime debugging required.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T4 | Blocked By: T1

  **References**:
  - Pattern: `.sisyphus/drafts/pratc-v0.2-alignment.md:6,15-17` — stale assumptions to retire
  - Pattern: `.sisyphus/plans/archive/2026-03-25/comprehensive-cleanup-documentation.md` — archive precedent

  **Acceptance Criteria**:
  - [ ] `.sisyphus/drafts/archive/2026-03-26/pratc-v0.2-alignment.md` exists.
  - [ ] `.sisyphus/drafts/pratc-v0.2-alignment.md` no longer exists.
  - [ ] Move manifest records source, destination, timestamp, and reason.

  **QA Scenarios**:
  ```
  Scenario: Happy path archival
    Tool: Bash
    Steps: move stale draft to dated archive path and write manifest
    Expected: archived file exists and source path is absent
    Evidence: .sisyphus/evidence/task-2-draft-archive.txt

  Scenario: Wrong-file protection
    Tool: Bash
    Steps: attempt to archive a file not listed as stale in T1 manifest
    Expected: operation blocked with explicit "not approved for archive" message
    Evidence: .sisyphus/evidence/task-2-archive-guard.txt
  ```

  **Commit**: YES | Message: `chore(sisyphus): archive stale draft artifacts` | Files: `.sisyphus/drafts/archive/2026-03-26/*`, `.sisyphus/evidence/task-2-*.txt`

- [x] 3. Create canonical active audit reconciliation plan (GREEN phase)

  **What to do**: Create `.sisyphus/plans/audit-reconciliation.md` using `.sisyphus/plans/archive/2026-03-25/v0-1-remediation.md` as baseline, trimmed to audit reconciliation only.
  **Must NOT do**: Do not include product feature implementation tasks.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: structured decision-complete planning artifact creation.
  - Skills: []
  - Omitted: [`subagent-driven-development`] — this task creates plan, not executes implementation.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T4 | Blocked By: T1

  **References**:
  - Pattern: `.sisyphus/plans/archive/2026-03-25/v0-1-remediation.md` — audit closure baseline
  - Pattern: `.sisyphus/evidence/v0.1-contract-audit-report.md` — gap inventory source
  - Pattern: `.sisyphus/evidence/final-f3-e2e-qa.md` — QA closure evidence
  - Pattern: `.sisyphus/evidence/final-f4-scope-fidelity.md:7-40` — scope-fidelity closure evidence

  **Acceptance Criteria**:
  - [ ] New plan exists at `.sisyphus/plans/audit-reconciliation.md`.
  - [ ] Plan scope includes audit reconciliation only (IN/OUT explicit).
  - [ ] Every task in new plan has executable acceptance criteria and QA scenarios.

  **QA Scenarios**:
  ```
  Scenario: Happy path plan creation
    Tool: Read
    Steps: inspect .sisyphus/plans/audit-reconciliation.md for full template compliance
    Expected: all required sections present and task details are decision-complete
    Evidence: .sisyphus/evidence/task-3-plan-structure-check.md

  Scenario: Scope creep guard
    Tool: Grep
    Steps: search new plan for forbidden expansion keywords (OAuth/webhooks/gRPC/auto PR actions)
    Expected: no new implementation expansion outside audit reconciliation intent
    Evidence: .sisyphus/evidence/task-3-scope-guard.txt
  ```

  **Commit**: YES | Message: `docs(plan): add canonical audit reconciliation plan` | Files: `.sisyphus/plans/audit-reconciliation.md`


- [x] 4. Switch active plan pointer to audit reconciliation plan

  **What to do**: Update `.sisyphus/boulder.json` to set `active_plan` and `plan_name` to the new audit reconciliation plan.
  **Must NOT do**: Do not point to any file under `/archive/`.

  **Recommended Agent Profile**:
  - Category: `unspecified-low` — Reason: small, deterministic JSON state edit.
  - Skills: []
  - Omitted: [`verification-before-completion`] — verification handled in T5.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T5 | Blocked By: T2,T3

  **References**:
  - Pattern: `.sisyphus/boulder.json:2,12` — current fields to replace
  - Pattern: `.sisyphus/plans/audit-reconciliation.md` — required active target

  **Acceptance Criteria**:
  - [ ] `active_plan` equals `/home/agent/pratc/.sisyphus/plans/audit-reconciliation.md`.
  - [ ] `plan_name` equals `audit-reconciliation`.
  - [ ] JSON remains valid.

  **QA Scenarios**:
  ```
  Scenario: Happy path pointer activation
    Tool: Bash
    Steps: parse boulder.json and print active_plan + plan_name
    Expected: values match audit-reconciliation target exactly
    Evidence: .sisyphus/evidence/task-4-pointer-activation.txt

  Scenario: Archive pointer rejection
    Tool: Bash
    Steps: run pointer invariant check that fails if '/archive/' appears in active_plan
    Expected: invariant passes only for non-archive active path
    Evidence: .sisyphus/evidence/task-4-pointer-invariant.txt
  ```

  **Commit**: YES | Message: `chore(sisyphus): activate audit reconciliation plan pointer` | Files: `.sisyphus/boulder.json`

- [x] 5. Run pointer and inventory integrity verification suite

  **What to do**: Execute deterministic checks proving active pointer validity, archive correctness, and single-active-plan invariant.
  **Must NOT do**: Do not mark complete if any check is skipped.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: audit-grade verification with binary pass/fail.
  - Skills: []
  - Omitted: [`test-driven-development`] — state verification, not behavior implementation.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T6 | Blocked By: T4

  **References**:
  - Pattern: `.sisyphus/boulder.json` — pointer source-of-truth
  - Pattern: `.sisyphus/plans/` + `.sisyphus/plans/archive/` — active vs archive inventories
  - Pattern: `.sisyphus/drafts/archive/2026-03-26/` — stale draft archival destination

  **Acceptance Criteria**:
  - [ ] Nonexistent-pointer check passes.
  - [ ] Non-archive-pointer check passes.
  - [ ] Archived stale draft present + source removed check passes.
  - [ ] Evidence bundle contains all command outputs.

  **QA Scenarios**:
  ```
  Scenario: Happy path integrity suite
    Tool: Bash
    Steps: run scripted checks for file existence, archive exclusion, and inventory consistency
    Expected: all checks exit 0 with explicit PASS lines
    Evidence: .sisyphus/evidence/task-5-integrity-suite.txt

  Scenario: Failure-path stale pointer
    Tool: Bash
    Steps: run check against intentionally wrong pointer value in temp copy of boulder.json
    Expected: check fails with explicit "active plan missing or archived" message
    Evidence: .sisyphus/evidence/task-5-failure-path.txt
  ```

  **Commit**: YES | Message: `test(sisyphus): add active-plan integrity verification evidence` | Files: `.sisyphus/evidence/task-5-*.txt`

- [x] 6. Publish audit state reconciliation note (REFACTOR phase)

  **What to do**: Write a concise reconciliation note mapping old state → corrected state, including why archived artifacts were retired.
  **Must NOT do**: Do not add new implementation requirements in this note.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: final traceability memo for future agents.
  - Skills: []
  - Omitted: [`systematic-debugging`] — no debugging context.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: Final Verification Wave | Blocked By: T5

  **References**:
  - Pattern: `.sisyphus/evidence/task-1-artifact-classification.md`
  - Pattern: `.sisyphus/evidence/task-5-integrity-suite.txt`
  - Pattern: `.sisyphus/boulder.json`

  **Acceptance Criteria**:
  - [ ] Reconciliation note exists and includes before/after pointer values.
  - [ ] Note lists archived artifacts with reasons.
  - [ ] Note confirms audit plan file now active and non-archive.

  **QA Scenarios**:
  ```
  Scenario: Happy path reconciliation publication
    Tool: Read
    Steps: inspect reconciliation note for required sections and exact path references
    Expected: all required sections present with concrete file paths
    Evidence: .sisyphus/evidence/task-6-reconciliation-note.md

  Scenario: Drift detection
    Tool: Bash
    Steps: compare reconciliation note pointer value to current boulder.json pointer
    Expected: mismatch triggers explicit drift failure
    Evidence: .sisyphus/evidence/task-6-drift-detection.txt
  ```

  **Commit**: YES | Message: `docs(sisyphus): publish audit state reconciliation note` | Files: `.sisyphus/evidence/task-6-reconciliation-note.md`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Commit 1: `chore(sisyphus): archive stale planning artifacts`
- Commit 2: `docs(plan): add audit reconciliation active plan`
- Commit 3: `chore(sisyphus): update boulder active audit pointer and verification evidence`

## Success Criteria
- Incorrect/stale planning artifact(s) are archived with traceability.
- Active plan pointer targets a non-archive audit reconciliation plan.
- Verification evidence proves pointer integrity and scope fidelity.
- Final Verification Wave F1-F4 pass and user provides explicit final approval.
