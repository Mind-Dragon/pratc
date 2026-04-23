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
- [x] Fresh current-HEAD autonomous run exists under `autonomous/runs/20260423T203433Z/`
- [x] Fresh audit from current HEAD has zero required failures: `19 passed`, `0 failed`, `0 manual`
- [x] `./bin/pratc` runtime branding/version surface matches current release/commit
- [x] `pratc serve` health proof is recorded in `autonomous/runtime/runtime-proof.json`

## Phase A — Reconcile docs and repo-local state

### A1. State and gap truth

- [x] Point `autonomous/STATE.yaml` at a real existing historical audit path while marking the phase as bootstrap/fresh-run-required
- [x] Regenerate or patch `autonomous/GAP_LIST.md` so it references the real audit path, not the missing `final-wave`
- [x] Add stricter controller checks so missing `last_audit_path` cannot coexist with `phase: complete`
- [x] Add stricter controller checks so `last_green_commit` older than `baseline_commit` requires explicit compatibility notes

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
- [x] Add a generated `run-metadata.yaml` command/template to the runbook

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

- [x] Fix version/build branding so the binary identifies the current prATC release and commit
- [x] Add `pratc version` or `--version` support for deterministic run metadata
- [x] Rebuild current binary with `make build`
- [x] Start or verify `pratc serve` on port `7400`
- [x] Health probe real interface when available: `curl -sf http://100.112.201.95:7400/healthz`
- [x] Record PID, binary path, banner/version output, health URL, and health response in run metadata

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

- [x] Create `autonomous/runs/<run-id>/`
- [x] Write `controller-log.md`
- [x] Write `wave-summary.md`
- [x] Write `run-metadata.yaml`
- [x] Run cache-backed full-corpus analyze without `--max-prs` cap
- [x] Run cluster, graph, plan, report
- [x] Run `scripts/audit_guideline.py <run-dir>`
- [x] Run `scripts/gap_list_from_audit.py --audit <run-dir>/AUDIT_RESULTS.json --gap-list autonomous/GAP_LIST.md --state autonomous/STATE.yaml`
- [x] Update `autonomous/STATE.yaml` so `last_green_commit` equals current HEAD only after this audit passes

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

- [x] Convert `deeper_judgment_layers` from manual to machine-checkable by auditing ordered decision-layer artifacts
- [x] Convert `disposal_bucket_persistence` from manual to machine-checkable by requiring disposal buckets to have matching terminal decision layers
- [x] Until converted, require explicit operator acceptance for remaining manual checks before `SUCCESS`

Verification:

```bash
cd /home/agent/pratc
python -m pytest -q tests/test_audit_guideline.py
python3 scripts/audit_guideline.py <run-dir>
```

Target: `0 fail`; manual checks either `0` or explicitly accepted in `wave-summary.md`.

### D2. Gap registry

- [x] Deduplicate `GAP_MAP` between `scripts/gap_list_from_audit.py` and `scripts/autonomous_controller.py`
- [x] Add one shared registry source for stable gap IDs
- [x] Preserve fixed/deferred/blocked gap history instead of rewriting only open gaps
- [x] Add tests for unknown audit checks and stable generated IDs

Verification:

```bash
cd /home/agent/pratc
python -m pytest -q scripts/test_autonomous_controller.py tests/test_audit_guideline.py
```

### D3. Wave/subagent readiness

- [x] Expand `autonomous/prompts/audit-gap.md`
- [x] Expand `autonomous/prompts/fix-gap.md`
- [x] Expand `autonomous/prompts/wave-closeout.md`
- [x] Add file ownership, TDD, verification, and no-scope-creep rules to prompts
- [x] Make `synthesize-wave` emit machine-readable todo JSON for Hermes session todo reconstruction
- [x] Write pre-wave and post-wave summaries into `autonomous/runs/<run-id>/wave-summary.md`

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

- [x] `controller-log.md`
- [x] `wave-summary.md`
- [x] `run-metadata.yaml`
- [x] `subagent-results/`
- [x] `analyze.json`
- [x] `step-3-cluster.json`
- [x] `step-4-graph.json`
- [x] `step-5-plan.json`
- [x] `report.pdf`
- [x] `AUDIT_RESULTS.json`
- [x] `GAP_LIST.md` and `STATE.yaml` consistent with the audit

## v1.7.1 — GitHub Credential Pooling Patch Release

Primary release goal: make prATC automatically discover, pool, and rotate every available GitHub credential so large OpenClaw runs do not silently depend on one active `gh` account.

- [ ] Automatic multi-source GitHub token discovery
  - Read tokens from `PRATC_GITHUB_TOKENS`, `GITHUB_TOKEN`, `GH_TOKEN`, `GITHUB_PAT`, supported `.env` files, prATC config/settings, and cached `gh` account credentials.
  - Discover all usable cached `gh` logins on the machine, not only the active `gh` account.
  - Deduplicate tokens by fingerprint; never log or persist raw token values.
  - Preserve explicit source/provenance metadata for diagnostics: env/config/cli/gh-login name.
- [ ] CLI/config surface for token sources
  - Add/verify a safe CLI/config path for passing multiple token sources without shell-exporting secrets.
  - Show redacted token-source inventory in diagnostics/preflight.
  - Keep `PRATC_GITHUB_TOKENS` as a compatibility path, but do not require it for normal multi-account use.
- [ ] Unified token fallback across live GitHub clients
  - Wire the token pool into GraphQL, REST, sync, preflight, workflow, analyze live-refresh, and serve paths.
  - Rotate/fallback on auth failures and rate-limit/secondary-limit failures where another token can help.
  - Do not rotate on non-retryable permission errors that indicate the operation is invalid for all tokens.
  - Log which redacted source index is active, exhausted, or skipped.
- [ ] OpenClaw release verification
  - Test on this machine with both cached `gh` accounts: `Mind-Dragon` and `avirweb`.
  - Verify preflight reports multiple token sources without exposing secrets.
  - Run an all-open `openclaw/openclaw` workflow with `--max-prs=0 --sync-max-prs=0` and confirm it can use fallback tokens.
  - Add regression tests for token discovery, dedupe, active/inactive `gh` account handling, fallback rotation, and redaction.

## v1.8 — ML Automation Release Prep

Primary release goal: add more automation through ML while keeping prATC advisory, auditable, and operator-controlled.

- [ ] ML feedback loop implementation
  - Capture operator bucket/category overrides, recommendation rejections, duplicate corrections, and plan-order changes as structured feedback.
  - Store feedback append-only with audit metadata and replay/idempotency guards.
  - Export privacy-safe training/evaluation batches for the Python ML bridge.
  - Keep online behavior deterministic by default; ML feedback may tune recommendations only through explicit batch/evaluation gates.
- [ ] ML automation evaluation harness
  - Measure recommendation quality before/after feedback on saved OpenClaw runs.
  - Track duplicate/canonicalization accuracy, bucket override rate, selected-plan acceptance, and false-positive disposal decisions.
  - Emit an analyst-readable ML automation report section.
- [ ] TUI operator feedback surface
  - Use the terminal UI as the release-facing operator surface for accepting/rejecting recommendations and entering feedback.
  - Show model/heuristic provenance, confidence, reason trail, and safe feedback actions.
- [ ] GitHub App/OAuth/webhook design stays prepared but implementation is deferred unless needed for ML feedback capture.
- [ ] Automatic PR actions or merge actions remain out of scope; prATC stays advisory/read-only unless explicitly enabled later.
- [ ] Web-based dashboard/UI is out of scope for v1.8.
- [ ] Multi-repo implementation is postponed to a later release.
