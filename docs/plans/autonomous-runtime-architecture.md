# prATC Autonomous Runtime Architecture Plan

Status: swarm-audited draft
Date: 2026-04-23
Repo: /home/agent/pratc
Baseline commit: 3b70307 feat: complete prATC v1.7 ML honesty slice

## Multi-model audit consensus

Four requested audit agents reviewed this plan:

- Kimi K2P6 control-plane/state-machine audit
- Kimi K2P6 artifact/runtime audit
- Z.AI 5.1 doc consistency audit
- Z.AI 5.1 gap/wave architecture audit

Consensus verdict: **NO-GO for full autonomous readiness yet; GO for Phase A doc/state reconciliation.**

Consensus blockers:

1. `autonomous/STATE.yaml` pointed at a missing `final-wave` path and mixed current baseline with old green commit.
2. `autonomous/RUNBOOK.md` referenced `./pratc-bin`, stale `final-wave`, and full-corpus `--max-prs 5000` examples.
3. `TODO.md` described completed ML honesty work, not the active autonomous readiness backlog.
4. `ROADMAP.md` and `CHANGELOG.md` disagreed about whether v1.7 had shipped.
5. `./bin/pratc` is stale-branded and lacks a deterministic `version` command.
6. No fresh current-HEAD run exists under `autonomous/runs/<run-id>/`.
7. Two audit checks remain manual without conversion or operator-acceptance rules.
8. `GAP_MAP` is duplicated between controller and gap generator.

Applied Phase A doc/state reconciliation updates are tracked in `TODO.md`. Remaining work starts at runtime proof and fresh current-HEAD run.

## Purpose

Make prATC capable of running an autonomous improvement loop against real corpus outputs without relying on chat memory, stale run paths, or manual interpretation of vague docs.

The target loop is:

1. establish live repo/runtime truth
2. run or reuse a full-corpus workflow
3. audit artifacts against GUIDELINE.md
4. generate stable gaps
5. synthesize implementation waves
6. dispatch subagents for non-trivial work
7. verify locally and rerun the audit
8. update durable state and continue or stop

## Current verified truth

Recent checks from this session:

- Git tree clean at `3b70307`
- Go full suite passed before commit: `go test ./...`
- Python ML suite passed before commit: `32 passed`
- Control-plane tests pass now: `python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py` -> `97 passed`
- Current `autonomous/STATE.yaml` points to stale path `projects/openclaw_openclaw/runs/final-wave`
- Real green audit artifact exists at `projects/openclaw_openclaw/runs/release-audit-20260421T023224Z/AUDIT_RESULTS.json`
- That audit is green: 17 passed, 0 failed, 2 manual
- Checked binary `./bin/pratc` is stale-branded: `Harness Optimizer v1.6.1 built on 2026-04-23...`
- `./bin/pratc version` is not a valid command
- No `pratc serve` process responded on `100.112.201.95:7400` or `127.0.0.1:7400`

## Governing docs and current drift

### AUTONOMOUS.md

Good:
- defines the correct closed loop
- requires state, gap list, audit results, and per-run artifacts
- defines clear stop conditions
- explicitly says findings must be promoted into audit/gap/todo/blocker

Drift:
- assumes stable artifacts are already wired into an active loop
- does not distinguish stale historical green audit from a fresh audit against current HEAD
- still allows 2 manual checks to coexist with success without a sharper rule

### autonomous/STATE.yaml

Current stale state:

```yaml
mode: complete
repo: openclaw/openclaw
baseline_commit: 61e49d6
current_run_id: final-wave
corpus_dir: projects/openclaw_openclaw/runs/final-wave
last_audit_path: projects/openclaw_openclaw/runs/final-wave/AUDIT_RESULTS.json
last_green_commit: 61e49d6
```

Required state shape for next cycle:

```yaml
mode: active | complete | paused | blocked
repo: openclaw/openclaw
branch: main
baseline_commit: 3b70307
current_run_id: <new autonomous run id or release-audit-20260421T023224Z if bootstrapping only>
corpus_dir: autonomous/runs/<run-id> or projects/openclaw_openclaw/runs/<run-id>
phase: bootstrap | run_pipeline | audit | gap_generation | fix_wave | rerun | closeout | complete
current_wave: 0
open_gaps: []
blocked_gaps: []
completed_gaps: []
last_audit_path: <real path>
last_green_commit: 3b70307 only after current-HEAD audit passes
paused: false
stop_reason: '' unless stopped
resume_command: python3 scripts/autonomous_controller.py resume
updated_at: <timestamp>
```

### autonomous/RUNBOOK.md

Good:
- captures bootstrap, audit, gap generation, resume, pause, complete
- tells controller to read docs/state/gaps first

Drift:
- references stale `projects/openclaw_openclaw/runs/final-wave`
- uses `./pratc-bin`, which does not exist in current tree
- recommends `--max-prs 5000` for analyze/graph/plan even though docs say this caused truncation risk
- health probe uses loopback; operator preference and live deployment should prefer real VPN IPv4 for service verification

### TODO.md

Current state:
- now describes the completed v1.7 ML honesty slice
- not suitable as the active autonomous backlog

Required next shape:
- rewrite or replace with `prATC TODO — Autonomous Runtime Readiness`
- keep completed ML honesty slice in changelog/release note, not as active TODO
- include explicit acceptance commands and phase order

### ARCHITECTURE.md

Good:
- describes layered decision engine, artifact-driven reuse, cache-first repeated passes
- documents current full-corpus validation and SLOs

Drift / next architecture gaps:
- no concrete autonomous runtime architecture section tying controller, audit, gap, run artifacts, and service process together
- still says maxPRs cap is a known constraint even though maxprs audit clarifies it is operator/runbook usage, not hidden default
- service health/version proof is under-specified

### ROADMAP.md / CHANGELOG.md

Drift:
- ROADMAP says v1.7 Evidence Enrichment is Q4 2026 current target, while CHANGELOG has v1.7.0 shipped 2026-04-23
- v1.8 work should not start until autonomous runtime can run, audit, gap, and resume reliably

## Desired architecture

### 1. Controller state model

`autonomous/STATE.yaml` must be the sole resume checkpoint.

State transitions:

```text
bootstrap
  -> run_pipeline
  -> audit
  -> gap_generation
  -> fix_wave
  -> rerun
  -> closeout
  -> complete
```

Stop overlays:

```text
paused, blocked, stalled, budget, human_hold
```

Invariant:
- `last_green_commit` is updated only after tests + audit pass against artifacts produced by that commit or explicitly marked as reused artifacts compatible with that commit.
- `corpus_dir` must exist.
- `last_audit_path` must exist unless phase is bootstrap/run_pipeline.
- if `open_gaps` is empty, `GAP_LIST.md` must say no open gaps and `audit-state` must pass.

### 2. Run artifact model

Use `autonomous/runs/<timestamp-or-run-id>/` for controller-owned runs.

Minimum layout:

```text
autonomous/runs/<run-id>/
├── controller-log.md
├── wave-summary.md
├── subagent-results/
├── run-metadata.yaml
├── sync.json or cache-reuse.md
├── analyze.json
├── step-3-cluster.json
├── step-4-graph.json
├── step-5-plan.json
├── report.pdf
└── AUDIT_RESULTS.json
```

`run-metadata.yaml` should include:
- repo
- branch
- commit
- binary path
- binary banner/version output
- service health URL and response
- cache path
- settings DB path
- corpus source
- whether sync was refreshed or cache reused
- whether any PR cap was applied

### 3. Audit model

`scripts/audit_guideline.py` remains the deterministic evaluator.

Short-term target:
- keep current checks green
- make current 2 manual checks explicit non-blocking manual checks in `AUDIT_RESULTS.json`

Medium-term target:
- convert manual checks into machine checks:
  - `deeper_judgment_layers`: require gate journey/order/exit trail in artifacts
  - `disposal_bucket_persistence`: add previous-run comparison fixture or snapshot state

### 4. Gap model

`scripts/gap_list_from_audit.py` turns required audit failures into stable gaps.

Needed improvement:
- preserve fixed/deferred/blocked gap history, not only open gaps
- include owner area, verification command, and source artifact path for every generated gap
- preserve stable IDs even when audit label text changes

### 5. Wave model

`scripts/autonomous_controller.py synthesize-wave` maps gaps to wave groups:

1. data model / type surface
2. core decision logic
3. wiring / artifact flow / report population
4. verification and doc sync

Needed improvement:
- generate machine-readable session todo JSON for Hermes controller consumption
- write `autonomous/runs/<run-id>/wave-summary.md` before and after the wave
- include exact file ownership and test commands in each synthesized task

### 6. Service/runtime model

Autonomous mode needs a live or explicitly skipped service proof.

Required checks:
- build fresh binary from current HEAD
- verify banner/version output
- start `serve` on port 7400 bound to a real interface or document why local-only is used
- health probe using real IPv4: `http://100.112.201.95:7400/healthz`
- record PID and health response in `run-metadata.yaml`

Avoid:
- `./pratc-bin` references unless that binary actually exists
- `--max-prs 5000` defaults for full-corpus runs
- claiming current runtime proof from an old binary

### 7. Subagent swarm model

Controller stays responsible for state and verification.

Subagents do:
- design audit
- implementation of isolated gaps
- code review / spec review

Swarm lane requested for this planning pass:
- Kimi K2P6 x2
- Z.AI 5.1 x2

Because child MCP filesystem scope may point outside `/home/agent/pratc`, prompts must instruct agents to use terminal commands from `/home/agent/pratc` and write outputs to explicit files under `.swarm/autonomous-architecture/`.

## Proposed implementation phases

### Phase A — Reconcile state and docs

Patch:
- `TODO.md` -> active autonomous readiness backlog
- `autonomous/STATE.yaml` -> current repo truth and real audit path
- `autonomous/RUNBOOK.md` -> remove stale final-wave, remove `pratc-bin`, remove default `--max-prs 5000`, prefer real health URL
- `ARCHITECTURE.md` -> add autonomous runtime architecture section and clarify maxPRs status
- `ROADMAP.md` -> mark v1.7 as shipped/currently local, put autonomous runtime readiness before v1.8 implementation

Verify:
- `python3 scripts/autonomous_controller.py audit-state`
- `python3 scripts/autonomous_controller.py synthesize-wave`
- `python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py`
- `git diff --check`

### Phase B — Runtime proof

Patch/fix:
- version/build metadata if needed
- service run commands in runbook

Run:
- `make build`
- `./bin/pratc --help`
- start `./bin/pratc serve --port 7400` under managed background process or documented tmux/systemd path
- health probe real IPv4

Verify:
- `curl -sf http://100.112.201.95:7400/healthz`
- record metadata

### Phase C — Fresh current-HEAD autonomous run

Run cache-first without cap:

```bash
RUN_ID=$(date -u +%Y%m%dT%H%M%SZ)
RUN_DIR=autonomous/runs/$RUN_ID
mkdir -p "$RUN_DIR/subagent-results"
./bin/pratc analyze --repo openclaw/openclaw --force-cache --format=json > "$RUN_DIR/analyze.json"
./bin/pratc cluster --repo openclaw/openclaw --force-cache --format=json > "$RUN_DIR/step-3-cluster.json"
./bin/pratc graph --repo openclaw/openclaw --force-cache --format=json > "$RUN_DIR/step-4-graph.json"
./bin/pratc plan --repo openclaw/openclaw --force-cache --target 20 --format=json > "$RUN_DIR/step-5-plan.json"
./bin/pratc report --repo openclaw/openclaw --input-dir "$RUN_DIR" --output "$RUN_DIR/report.pdf"
python3 scripts/audit_guideline.py "$RUN_DIR"
python3 scripts/gap_list_from_audit.py --audit "$RUN_DIR/AUDIT_RESULTS.json" --gap-list autonomous/GAP_LIST.md --state autonomous/STATE.yaml
```

Verify:
- audit has zero required failures
- `autonomous/STATE.yaml` points to the new run
- `GAP_LIST.md` reflects audit truth

### Phase D — Close manual audit gaps

Implement machine-checkable evidence for:
- gate/deeper judgment ordering
- disposal bucket persistence

Only after A-C are stable.

## First concrete next steps

1. Run 4-agent design audit swarm on this plan.
2. Merge consensus into a final `TODO.md` and doc patch plan.
3. Patch docs/state only after consensus.
4. Verify control-plane tests.
5. Commit local.
