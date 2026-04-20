# prATC TODO — Autonomous Mode Buildout

## Goal

Make prATC operate in a true autonomous mode where Hermes can resume from repo-local state, run an audit-driven controller loop, delegate subagents, and iteratively close GUIDELINE gaps against a real corpus until the required audit surface is green.

## Source of truth

- `GUIDELINE.md` — compliance rules and non-negotiables
- `ARCHITECTURE.md` — code ownership and system shape
- `AUTONOMOUS.md` — normative controller-loop contract
- `autonomous/STATE.yaml` — current checkpoint and resume state
- `autonomous/GAP_LIST.md` — current failure surface
- `autonomous/RUNBOOK.md` — exact execution commands

## 100% means

Autonomous mode is not "done enough" when the output looks better. It is done when all of these are true:

- [ ] `go test ./...` passes
- [ ] `python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py` passes
- [ ] a fresh workflow rerun completes and produces the required artifacts
- [ ] `scripts/audit_guideline.py <run-dir>` has zero required failures
- [ ] the report surface is audit-green because `analyze.json.prs[]` is self-describing enough for appendix/report use
- [ ] `AUTONOMOUS.md`, `TODO.md`, `autonomous/*`, and the live code describe the same system truthfully

## Promotion rule

Every new finding must be promoted immediately into exactly one of these buckets:

- a failing audit check
- an open gap in `autonomous/GAP_LIST.md`
- a live todo item with an explicit verification command
- a documented non-goal or blocker

If a finding lives only in chat, it is not part of the system.

## Current status

- Autonomous control-plane scaffold exists in-repo.
- `/autonomous` skill exists in Hermes skills.
- Deterministic scaffold scripts exist:
  - `scripts/audit_guideline.py`
  - `scripts/gap_list_from_audit.py`
  - `scripts/autonomous_controller.py`
- The latest audit against `projects/OpenClaw_OpenClaw/runs/20260419-065654` is working and produces `AUDIT_RESULTS.json` plus `autonomous/GAP_LIST.md`.
- The repo is not yet autonomous-complete because the controller loop has not yet closed the product gaps in the decision engine or the graph/report signal-quality gaps.

---

## Workstream 1 — Control-plane hardening

- [x] Create stable `AUTONOMOUS.md` with phase loop, state contract, artifact ownership, and stop conditions
- [x] Create `autonomous/STATE.yaml`
- [x] Create `autonomous/GAP_LIST.md`
- [x] Create `autonomous/RUNBOOK.md`
- [x] Create `autonomous/prompts/` prompt templates
- [x] Create `scripts/audit_guideline.py` scaffold and verify it emits `AUDIT_RESULTS.json`
- [x] Create `scripts/gap_list_from_audit.py` scaffold and verify it regenerates `autonomous/GAP_LIST.md`
- [x] Create `scripts/autonomous_controller.py` scaffold and verify basic state transitions (`reconcile`, `resume`, `pause`, `next-wave`, `complete`)
- [ ] Harden `autonomous_controller.py` from scaffold into a reliable checkpoint manager; verify repeated resume cycles preserve state truthfully with `python -m pytest -q scripts/test_autonomous_controller.py`
- [ ] Expand audit coverage from core checks to the full required GUIDELINE/report-readiness matrix; verify every required rule has either a machine check or an explicit `manual` annotation via `python -m pytest -q tests/test_audit_guideline.py`

## Workstream 2 — Session orchestration

- [x] Create Hermes `/autonomous` skill
- [ ] Prove `/autonomous` can rebuild the Hermes session todo entirely from repo-local state and latest audit output; verify by reconciling from `autonomous/STATE.yaml` + `autonomous/GAP_LIST.md` with no hidden chat context
- [ ] Define wave-generation logic from `autonomous/GAP_LIST.md` into session todo items; verify independent gaps become parallelizable wave items with attached verification commands
- [ ] Add closeout discipline so `/autonomous` patches `STATE.yaml`, `GAP_LIST.md`, and `TODO.md` truthfully after each verified wave

## Workstream 3 — Product gap closure

### Wave 1 — make `analyze.json` truthful

- [ ] Wire per-PR bucket/category visibility into `AnalysisResponse.PRs`; verify audit `bucket_coverage` passes after a fresh rerun
- [ ] Wire structured reason trails into `AnalysisResponse.PRs`; verify audit `reason_coverage` passes after a fresh rerun
- [ ] Wire confidence scoring into `AnalysisResponse.PRs`; verify audit `confidence_coverage` passes after a fresh rerun
- [ ] Expose temporal routing (`now` / `future` / `blocked`) directly on `AnalysisResponse.PRs`; verify audits `temporal_routing` and `future_work_visible` pass after a fresh rerun
- [ ] Make each PR row self-describing for report/appendix use; verify audit `report_self_describing_prs` passes after a fresh rerun
- [ ] Make future-work visibility explicit as a first-class auditable outcome rather than an implied side effect; verify audit `future_work_visible` passes after a fresh rerun

### Wave 2 — make graph signal usable

- [ ] Remove trivial same-branch dependency edges; verify audit `dependency_edge_quality` passes after a fresh rerun
- [ ] Reduce conflict noise toward the target threshold with truthful repo-specific filtering; verify audit `conflict_pairs_threshold` improves to pass after a fresh rerun

### Wave 3 — make the report surface truthfully useful

- [ ] Keep report usefulness encoded in audit checks rather than prose; verify report-related checks pass from artifact inspection rather than PDF hand-waving
- [ ] Remove or replace placeholder-only report sections before calling the report production-ready; verify report generation still passes and the report sections are backed by real artifact data

## Workstream 4 — Autonomous proof cycle

- [ ] Run one full autonomous cycle end-to-end: audit → gap list → subagent fix wave → build/test → rerun audit
- [ ] Prove interruption recovery by stopping mid-cycle and resuming from `autonomous/STATE.yaml`
- [ ] Record per-run outputs under `autonomous/runs/<timestamp>/`
- [ ] Update roadmap/docs after the first truthful autonomous green wave

---

## Current open gaps from the latest known run

- G-001 bucket coverage missing
- G-002 reason coverage missing
- G-003 confidence coverage missing
- G-004 trivial dependency-edge explosion
- G-005 conflict noise still too high
- G-006 temporal routing not visible
- G-007 report surface not yet self-describing enough for audit-green appendix/report use
- G-008 future work visibility missing

## Exit criteria

Autonomous mode is considered real when all of the following are true:

- [ ] A new controller session can resume from repo-local state without hidden chat context
- [ ] The session todo can be reconstructed from `STATE.yaml` + `GAP_LIST.md`
- [ ] `scripts/audit_guideline.py` and `scripts/gap_list_from_audit.py` form a deterministic audit surface
- [ ] `scripts/autonomous_controller.py` truthfully tracks phase/wave/checkpoint state through pause/resume
- [ ] Hermes `/autonomous` can run at least one full verified loop using delegated subagents
- [ ] Build and test verification remain green after control-plane changes
- [ ] The required audit surface is fully green on a fresh rerun
- [ ] `AUTONOMOUS.md`, `TODO.md`, and `autonomous/*` describe the same system
