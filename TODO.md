# prATC TODO — Autonomous Runtime Readiness

## Goal

Make prATC autonomous mode trustworthy enough to run the closed loop from repo-local state:

1. establish current repo/runtime truth
2. run or reuse a full-corpus workflow
3. audit artifacts against `GUIDELINE.md`
4. generate stable gaps
5. synthesize implementation waves
6. dispatch subagents for non-trivial fixes
7. verify and rerun the audit
8. update durable state and continue, stop, or close out

This replaces the completed ML Reliability + Honesty Slice, which landed locally in commit `3b70307` and is recorded in `CHANGELOG.md`.

## Source of truth

- `GUIDELINE.md` — rules, buckets, non-negotiables
- `ARCHITECTURE.md` — product/system shape
- `AUTONOMOUS.md` — autonomous loop contract
- `autonomous/STATE.yaml` — current resume checkpoint
- `autonomous/GAP_LIST.md` — current failure surface
- `autonomous/RUNBOOK.md` — exact operator/controller commands
- `scripts/audit_guideline.py` — deterministic guideline audit
- `scripts/gap_list_from_audit.py` — audit-to-gap promotion
- `scripts/autonomous_controller.py` — state, wave, closeout controller
- `docs/plans/autonomous-runtime-architecture.md` — current architecture plan and swarm audit basis

## Current status

- [x] v1.7 ML honesty slice committed locally: `3b70307`
- [x] Control-plane tests pass: `python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py` -> `97 passed`
- [x] Latest historical full-corpus audit exists: `projects/openclaw_openclaw/runs/release-audit-20260421T023224Z/AUDIT_RESULTS.json`
- [x] Historical audit is green: `17 passed`, `0 failed`, `2 manual`
- [x] Four-model architecture audit swarm completed:
  - `Kimi K2P6` control plane
  - `Kimi K2P6` artifacts/runtime
  - `Z.AI 5.1` doc consistency
  - `Z.AI 5.1` gap/wave architecture
- [ ] Fresh current-HEAD autonomous run exists under `autonomous/runs/<run-id>/`
- [ ] Fresh audit from current HEAD has zero required failures
- [ ] `./bin/pratc` runtime branding/version surface matches current release/commit
- [ ] `pratc serve` health proof is recorded or explicitly skipped for CLI-only autonomous run

## Phase A — Reconcile docs and repo-local state

### A1. State and gap truth

- [x] Point `autonomous/STATE.yaml` at a real existing historical audit path while marking the phase as bootstrap/fresh-run-required
- [x] Regenerate or patch `autonomous/GAP_LIST.md` so it references the real audit path, not the missing `final-wave`
- [ ] Add stricter controller checks so missing `last_audit_path` cannot coexist with `phase: complete`
- [ ] Add stricter controller checks so `last_green_commit` older than `baseline_commit` requires explicit compatibility notes

Verification:

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py audit-state
python3 scripts/autonomous_controller.py synthesize-wave
```

### A2. Runbook truth

- [x] Replace `./pratc-bin` with `./bin/pratc`
- [x] Replace missing `final-wave` path with `release-audit-20260421T023224Z` as historical bootstrap artifact
- [x] Remove default `--max-prs 5000` from full-corpus rerun examples
- [x] Add binary/version proof and real-interface health proof steps
- [ ] Add a generated `run-metadata.yaml` command/template to the runbook

Verification:

```bash
cd /home/agent/pratc
! grep -q 'pratc-bin' autonomous/RUNBOOK.md
! grep -q 'final-wave' autonomous/RUNBOOK.md
! grep -q -- '--max-prs 5000' autonomous/RUNBOOK.md
```

### A3. Current-surface docs

- [x] Add autonomous runtime architecture section to `ARCHITECTURE.md`
- [x] Clarify maxPRs as operator-facing cap, not hidden default
- [x] Update `ROADMAP.md` so v1.7 is shipped and autonomous runtime readiness is current before v1.8
- [x] Update `CHANGELOG.md` v1.6 status from in-progress to dated shipped state
- [x] Keep `docs/plans/autonomous-runtime-architecture.md` as design/audit basis

Verification:

```bash
cd /home/agent/pratc
git diff --check
```

## Phase B — Runtime proof

- [ ] Fix version/build branding so the binary identifies the current prATC release and commit
- [ ] Add `pratc version` or `--version` support for deterministic run metadata
- [ ] Rebuild current binary with `make build`
- [ ] Start or verify `pratc serve` on port `7400`
- [ ] Health probe real interface when available: `curl -sf http://100.112.201.95:7400/healthz`
- [ ] Record PID, binary path, banner/version output, health URL, and health response in run metadata

Verification:

```bash
cd /home/agent/pratc
make build
./bin/pratc --help | head -5
./bin/pratc version
curl -sf http://100.112.201.95:7400/healthz
```

If service is intentionally skipped for a CLI-only run, record the skip reason in `autonomous/runs/<run-id>/run-metadata.yaml`.

## Phase C — Fresh current-HEAD autonomous run

- [ ] Create `autonomous/runs/<run-id>/`
- [ ] Write `controller-log.md`
- [ ] Write `wave-summary.md`
- [ ] Write `run-metadata.yaml`
- [ ] Run cache-backed full-corpus analyze without `--max-prs` cap
- [ ] Run cluster, graph, plan, report
- [ ] Run `scripts/audit_guideline.py <run-dir>`
- [ ] Run `scripts/gap_list_from_audit.py --audit <run-dir>/AUDIT_RESULTS.json --gap-list autonomous/GAP_LIST.md --state autonomous/STATE.yaml`
- [ ] Update `autonomous/STATE.yaml` so `last_green_commit` equals current HEAD only after this audit passes

Verification:

```bash
cd /home/agent/pratc
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
python3 scripts/autonomous_controller.py audit-state
```

## Phase D — Close autonomy gaps in the controller/audit layer

### D1. Manual audit checks

- [ ] Convert `deeper_judgment_layers` from manual to machine-checkable, likely by auditing ordered gate journey / exit trail artifacts
- [ ] Convert `disposal_bucket_persistence` from manual to machine-checkable, likely by comparing prior snapshot state or a dedicated fixture
- [ ] Until converted, require explicit operator acceptance for remaining manual checks before `SUCCESS`

Verification:

```bash
cd /home/agent/pratc
python -m pytest -q tests/test_audit_guideline.py
python3 scripts/audit_guideline.py <run-dir>
```

Target: `0 fail`; manual checks either `0` or explicitly accepted in `wave-summary.md`.

### D2. Gap registry

- [ ] Deduplicate `GAP_MAP` between `scripts/gap_list_from_audit.py` and `scripts/autonomous_controller.py`
- [ ] Add one shared registry source for stable gap IDs
- [ ] Preserve fixed/deferred/blocked gap history instead of rewriting only open gaps
- [ ] Add tests for unknown audit checks and stable generated IDs

Verification:

```bash
cd /home/agent/pratc
python -m pytest -q scripts/test_autonomous_controller.py tests/test_audit_guideline.py
```

### D3. Wave/subagent readiness

- [ ] Expand `autonomous/prompts/audit-gap.md`
- [ ] Expand `autonomous/prompts/fix-gap.md`
- [ ] Expand `autonomous/prompts/wave-closeout.md`
- [ ] Add file ownership, TDD, verification, and no-scope-creep rules to prompts
- [ ] Make `synthesize-wave` emit machine-readable todo JSON for Hermes session todo reconstruction
- [ ] Write pre-wave and post-wave summaries into `autonomous/runs/<run-id>/wave-summary.md`

Verification:

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py synthesize-wave
python -m pytest -q scripts/test_autonomous_controller.py
```

## Phase E — Autonomous closeout gate

Autonomous mode is not done until all of these pass from a clean tree:

```bash
cd /home/agent/pratc
git status --short
go test ./...
make test-python
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
python3 scripts/autonomous_controller.py audit-state
python3 scripts/autonomous_controller.py synthesize-wave
python3 scripts/audit_guideline.py <current-run-dir>
```

And the current run has:

- [ ] `controller-log.md`
- [ ] `wave-summary.md`
- [ ] `run-metadata.yaml`
- [ ] `subagent-results/`
- [ ] `analyze.json`
- [ ] `step-3-cluster.json`
- [ ] `step-4-graph.json`
- [ ] `step-5-plan.json`
- [ ] `report.pdf`
- [ ] `AUDIT_RESULTS.json`
- [ ] `GAP_LIST.md` and `STATE.yaml` consistent with the audit

## Out of scope until autonomous runtime is green

- [ ] v1.8 multi-repo implementation
- [ ] ML feedback loop implementation
- [ ] GitHub App/OAuth/webhook implementation
- [ ] Automatic PR actions or merge actions
- [ ] Dashboard/UI work
