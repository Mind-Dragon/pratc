# prATC Autonomous Mode

## Purpose

This document defines the stable operating contract for autonomous improvement of prATC against real corpus outputs.

Autonomous mode is a closed loop:
1. run the pipeline on a real corpus
2. audit the outputs against GUIDELINE.md
3. convert failures into explicit gaps
4. dispatch subagents to fix those gaps
5. re-run the pipeline and audit again
6. stop only on success, true blocker, or stall budget

This document is the normative spec. It should stay stable. Volatile run-specific findings live under `autonomous/`.

## Definition of success

"100%" means audit-green, not "looks better".

Required success conditions:
- `go test ./...` passes
- controller/audit tests pass
- a fresh workflow rerun completes and produces the required artifacts
- `scripts/audit_guideline.py <run-dir>` has zero required failures
- any remaining manual audit checks are either converted to machine checks or explicitly accepted in `wave-summary.md`
- `last_green_commit` equals the current `baseline_commit` before `phase: complete`
- report usefulness is represented by passing audit checks, not prose optimism
- durable docs and repo-local state describe the same system truthfully

## Finding promotion rule

Every new finding must be promoted immediately into exactly one of these buckets:
- a failing audit check
- an open gap in `autonomous/GAP_LIST.md`
- a live todo item with an explicit verification command
- a documented non-goal or blocker

Findings that remain only in chat are not part of the autonomous system.

## Control model

### Roles

- `AUTONOMOUS.md` — policy and loop contract
- `TODO.md` — durable project backlog for autonomous-mode buildout and remaining milestone work
- `autonomous/STATE.yaml` — controller checkpoint and resume state
- `autonomous/GAP_LIST.md` — latest failing audit surface
- `autonomous/RUNBOOK.md` — exact bootstrap / pause / resume / recovery commands
- Hermes `/autonomous` skill — execution behavior for the controller session
- Hermes session todo — live wave queue for the current execution pass
- TUI monitor — observation surface only, not the source of truth
- subagents — all non-trivial implementation and review work

### Authority order

On conflicts:
1. `GUIDELINE.md` wins on rules, buckets, and non-negotiables
2. `ARCHITECTURE.md` wins on system shape and file ownership
3. `AUTONOMOUS.md` wins on autonomous loop behavior
4. `TODO.md` wins on current durable backlog ordering
5. `autonomous/STATE.yaml` wins on resume point for the active loop

## Observation vs control

The TUI monitor is not a control surface. It exists to expose live truth:
- job progress
- request activity
- rate-limit status
- structured logs

The controller acts through code, scripts, and delegated subagents. It does not treat visual TUI state as canonical completion proof.

## Required artifacts

Each autonomous cycle must create or update these artifacts:

### Stable artifacts

- `AUTONOMOUS.md`
- `TODO.md`
- `autonomous/RUNBOOK.md`
- `autonomous/STATE.yaml`
- `autonomous/GAP_LIST.md`
- `scripts/audit_guideline.py`
- `scripts/gap_list_from_audit.py`
- `scripts/autonomous_controller.py`

### Per-run artifacts

Under `autonomous/runs/<timestamp>/`:
- `AUDIT_RESULTS.json`
- `controller-log.md`
- `wave-summary.md`
- `subagent-results/` directory
- optional copies or links to run inputs used for the audit

## Controller contract

### What the controller must do

- read governing docs before acting
- reconcile repo state, latest run artifacts, and controller checkpoint
- keep the session todo aligned with the current wave
- generate or refresh the audit result before changing gap status
- delegate all non-trivial implementation work to subagents
- verify locally after each subagent wave
- update durable artifacts after every phase boundary
- preserve resumability if interrupted

### What the controller must not do

- implement non-trivial fixes directly
- mark a gap fixed without rerunning the audit
- overwrite durable state with chat-only summaries
- skip test/build verification after code changes
- claim completion from the TUI alone
- silently drop gaps, blockers, or failed waves

## Phase loop

### Phase 0 — bootstrap

Required checks:
- repo clean enough to reason about (`git status`)
- active branch and commit recorded
- `go test ./...` baseline recorded
- corpus path chosen and recorded in `autonomous/STATE.yaml`
- `./bin/pratc` builds and exposes current binary provenance through banner, `version`, or equivalent metadata
- `pratc serve` available, already running, or explicitly skipped with a recorded CLI-only reason
- TUI monitor available for observation when needed

Outputs:
- `autonomous/STATE.yaml` initialized or refreshed
- session todo seeded from current open wave

### Phase 1 — run pipeline

Run the full corpus workflow using the selected cached or fresh corpus.

Minimum expected outputs:
- sync artifact or explicit cache reuse note
- `analyze.json`
- cluster output
- graph output
- plan output
- PDF report output

### Phase 2 — audit

Run `scripts/audit_guideline.py` against the selected run directory.

The audit must:
- check every GUIDELINE rule that is machine-checkable
- record actual vs expected values
- exit non-zero if any required rule fails
- write `AUDIT_RESULTS.json`

### Phase 3 — gap generation

Run `scripts/gap_list_from_audit.py`.

The gap generator must:
- read `AUDIT_RESULTS.json`
- update `autonomous/GAP_LIST.md`
- preserve stable gap IDs where possible
- classify severity and likely owner area
- separate open, fixed, blocked, and deferred gaps

### Phase 4 — fix waves

The controller converts open gaps into implementation waves.

Default wave ordering:
1. data model / type surface
2. core decision logic
3. wiring / report population / artifact flow
4. verification and doc sync

Rules:
- independent gaps may run in parallel
- tightly coupled gaps sharing files may be grouped
- every non-trivial change goes through a subagent with TDD instructions
- controller verifies locally after the wave returns

### Phase 5 — re-run and re-audit

After each wave:
- `go build ./...`
- `go test ./...`
- rerun the necessary pipeline steps
- rerun the audit
- regenerate the gap list
- update `autonomous/STATE.yaml`

No wave is complete until the refreshed audit proves progress.

### Phase 6 — closeout

Success requires all of:
- all required audit checks passing
- build green
- test suite green
- durable docs updated truthfully
- controller state marked complete
- final commit made or explicit human hold noted

## Audit contract

The audit layer is deterministic code, not prose.

At minimum it must evaluate:
- PR accounting coverage
- bucket coverage
- reason coverage
- confidence coverage
- duplicate handling presence
- garbage routing presence
- temporal routing visibility
- report-readiness / self-describing PR rows
- no silent exclusion
- no opaque top-N reasoning
- graph noise thresholds
- performance and artifact presence checks

If a rule cannot yet be machine-checked, it must be listed explicitly in `AUTONOMOUS.md` or `RUNBOOK.md` as a manual audit item rather than implied.

## Gap contract

`autonomous/GAP_LIST.md` is the current failure surface.

Each gap entry must include:
- gap ID
- governing rule
- severity
- expected behavior
- actual behavior
- likely code ownership area
- verification command or artifact check
- current status
- notes / blockers

Run-specific findings belong here, not in `AUTONOMOUS.md`.

## State / resume contract

`autonomous/STATE.yaml` is required for true autonomous mode.

Minimum fields:
- mode
- repo
- branch
- baseline_commit
- current_run_id
- corpus_dir
- phase
- current_wave
- open_gaps
- blocked_gaps
- completed_gaps
- last_audit_path
- last_green_commit
- paused
- stop_reason
- resume_command
- updated_at

Resume semantics:
- if interrupted, the next controller session reads `STATE.yaml` first
- the controller reconstructs the live session todo from the current phase and open gaps
- no phase restarts from scratch unless artifacts are missing or invalid
- if artifacts are stale relative to HEAD, the controller records that explicitly and reruns the necessary steps
- if `last_audit_path` does not exist, the controller must not proceed to gap generation, fix waves, or complete; it must downgrade to bootstrap or run-pipeline
- if `last_green_commit` is older than `baseline_commit`, completion requires a fresh current-HEAD audit or an explicit artifact-compatibility note

## Stop conditions

Autonomous mode stops only when one of these is true:
- `SUCCESS` — all required checks pass
- `BLOCKED` — a real external blocker prevents further progress
- `STALLED` — no measurable progress after the configured retry budget
- `BUDGET` — explicit time/token ceiling reached
- `HUMAN_HOLD` — operator requested stop

Every non-success stop must update `autonomous/STATE.yaml` and `autonomous/GAP_LIST.md` with the reason.

## Definition of done

Autonomous-mode buildout is done when:
- the repo has the control-plane structure under `autonomous/`
- the deterministic audit and controller scripts exist
- the `/autonomous` skill can resume from repo-local state
- the session todo can be regenerated from repo-local state
- the controller can drive at least one full audit → gap → fix-wave → rerun cycle
- the durable docs describe the same system

## File layout

```text
pratc/
├── AUTONOMOUS.md
├── TODO.md
├── autonomous/
│   ├── GAP_LIST.md
│   ├── RUNBOOK.md
│   ├── STATE.yaml
│   ├── prompts/
│   │   ├── audit-gap.md
│   │   ├── fix-gap.md
│   │   └── wave-closeout.md
│   └── runs/
├── scripts/
│   ├── audit_guideline.py
│   ├── gap_list_from_audit.py
│   └── autonomous_controller.py
└── internal/
    └── ... product code and tests
```

## Relationship to other documents

- `GUIDELINE.md` defines what a compliant output means.
- `ARCHITECTURE.md` defines where logic and data should live.
- `TODO.md` tracks the durable backlog and verification targets.
- `ROADMAP.md` defines milestone framing and release sequencing.
- `AUTONOMOUS.md` defines how the controller loop operates.
- `autonomous/*` stores the current execution state for the loop.

If these documents do not describe the same system, autonomous mode is not trustworthy yet.
