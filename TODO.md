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

- [x] `go test ./...` passes
- [x] `python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py` passes
- [ ] a fresh workflow rerun completes and produces the required artifacts
- [x] `scripts/audit_guideline.py <run-dir>` has zero required failures
- [x] the report surface is audit-green because `analyze.json.prs[]` is self-describing enough for appendix/report use
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
- The latest audit against `projects/openclaw_openclaw/runs/final-wave` is green and produces `AUDIT_RESULTS.json` plus a no-open-gap `autonomous/GAP_LIST.md`.
- Product/output gaps are closed on the current canonical cache-backed rerun.
- The repo is not yet autonomous-complete because the remaining work is controller-proof / hardening: repo-local resume, todo reconstruction, wave synthesis, closeout discipline, interruption recovery, and final report polish.

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
- [x] Expand audit coverage from core checks to the full required GUIDELINE/report-readiness matrix; verified by `python -m pytest -q tests/test_audit_guideline.py` and final-wave audit-green run

## Workstream 2 — Session orchestration

- [x] Create Hermes `/autonomous` skill
- [ ] Prove `/autonomous` can rebuild the Hermes session todo entirely from repo-local state and latest audit output; verify by reconciling from `autonomous/STATE.yaml` + `autonomous/GAP_LIST.md` with no hidden chat context
- [ ] Define wave-generation logic from `autonomous/GAP_LIST.md` into session todo items; verify independent gaps become parallelizable wave items with attached verification commands
- [ ] Add closeout discipline so `/autonomous` patches `STATE.yaml`, `GAP_LIST.md`, and `TODO.md` truthfully after each verified wave

## Workstream 3 — Product gap closure

### Wave 1 — make `analyze.json` truthful

- [x] Wire per-PR bucket/category visibility into `AnalysisResponse.PRs`; verified on `projects/openclaw_openclaw/runs/20260420-193647-wave1` via audit `bucket_coverage`
- [x] Wire structured reason trails into `AnalysisResponse.PRs`; verified on `projects/openclaw_openclaw/runs/20260420-193647-wave1` via audit `reason_coverage`
- [x] Wire confidence scoring into `AnalysisResponse.PRs`; verified on `projects/openclaw_openclaw/runs/20260420-193647-wave1` via audit `confidence_coverage`
- [x] Expose temporal routing (`now` / `future` / `blocked`) directly on `AnalysisResponse.PRs`; verified on `projects/openclaw_openclaw/runs/20260420-193647-wave1` via audits `temporal_routing` and `future_work_visible`
- [x] Make each PR row self-describing for report/appendix use; verified on `projects/openclaw_openclaw/runs/20260420-193647-wave1` via audit `report_self_describing_prs`
- [x] Make future-work visibility explicit as a first-class auditable outcome rather than an implied side effect; verified on `projects/openclaw_openclaw/runs/20260420-193647-wave1` via audit `future_work_visible`
- [x] Restore duplicate detection on the current cache-backed analyze path; verified by `duplicate_presence` passing on `projects/openclaw_openclaw/runs/final-wave`

### Wave 2 — make graph signal usable

- [x] Remove trivial same-branch dependency edges; verified by audit `dependency_edge_quality` passing on `projects/openclaw_openclaw/runs/final-wave`
- [x] Reduce conflict noise toward the target threshold with truthful repo-specific filtering; verified by audit `conflict_pairs_threshold` passing on `projects/openclaw_openclaw/runs/final-wave`

### Wave 3 — make the report surface truthfully useful

- [x] Keep report usefulness encoded in audit checks rather than prose; verified on `projects/openclaw_openclaw/runs/final-wave` with audit-green report surface and generated PDF
- [ ] Remove or replace placeholder-only report sections before calling the report production-ready; verify report generation still passes and the report sections are backed by real artifact data

## Workstream 4 — Autonomous proof cycle

- [x] Run one full autonomous cycle end-to-end: audit → gap list → subagent fix wave → build/test → rerun audit; verified on `projects/openclaw_openclaw/runs/final-wave`
- [ ] Prove interruption recovery by stopping mid-cycle and resuming from `autonomous/STATE.yaml`
- [x] Record per-run outputs under `autonomous/runs/<timestamp>/`; verified by `autonomous/runs/20260420-final-wave/{controller-log.md,wave-summary.md}`
- [x] Update roadmap/docs after the first truthful autonomous green wave

---

## Current open gaps from the latest known run

No required open gaps remain on `projects/openclaw_openclaw/runs/final-wave`; controller-proof items below remain open.

## Exit criteria

Autonomous mode is considered real when all of the following are true:

- [ ] A new controller session can resume from repo-local state without hidden chat context
- [ ] The session todo can be reconstructed from `STATE.yaml` + `GAP_LIST.md`
- [ ] `scripts/audit_guideline.py` and `scripts/gap_list_from_audit.py` form a deterministic audit surface
- [ ] `scripts/autonomous_controller.py` truthfully tracks phase/wave/checkpoint state through pause/resume
- [ ] Hermes `/autonomous` can run at least one full verified loop using delegated subagents
- [ ] Build and test verification remain green after control-plane changes
- [x] The required audit surface is fully green on `projects/openclaw_openclaw/runs/final-wave`
- [ ] `AUTONOMOUS.md`, `TODO.md`, and `autonomous/*` describe the same system
