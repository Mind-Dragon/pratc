# prATC TODO — Current Iteration Toward 2.0

## Focus

Current iteration: move from the verified `1.7.1` advisory baseline toward the `2.0` swarm action engine by closing ActionPlan completeness and building the first swarm API/proof surfaces.

This TODO is intentionally narrow. The full 1.7.1 -> 2.0 roadmap lives in `PLAN.md` and `VERSION2.0.md`.

## Source of truth

- `PLAN.md` — current phased implementation plan from 1.7.1 to 2.0
- `VERSION2.0.md` — product/release plan and 16-lane ownership map
- `GUIDELINE.md` — action policy, lanes, bucket rules, non-negotiables
- `ARCHITECTURE.md` — system shape and component ownership
- `AUTONOMOUS.md` — autonomous loop contract
- `autonomous/RUNBOOK.md` — exact operator/controller commands
- `scripts/audit_guideline.py` — deterministic audit gate

## Verified baseline

- [x] Current HEAD: `8d80f7580c74`
- [x] Runtime binary: `prATC v1.7.1 commit=8d80f7580c74 dirty=true` for Wave A tmux smoke; final proof deferred until commit boundary
- [x] Runtime: tmux session `pratc-autonomous`, port `7400`, restarted with `--force-cache`
- [x] Health: loopback and VPN OK
- [x] Current-HEAD run: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z`
- [x] Corpus: `6,632` PRs
- [x] Audit: `23 passed`, `0 failed`, `0 manual`
- [x] PDF: `report.pdf`, 29 pages
- [x] ActionPlan: schema `2.0`, advisory, `6,632` work items, `182` intents

## Current dirty-state note

- [x] Defer refreshed `autonomous/runtime/runtime-proof.json` until the next final runtime proof boundary; current Wave A source tree is intentionally dirty.
- [x] Do not mix proof-only changes with implementation commits.

## Guardrails for this iteration

- Advisory/dry-run only. No live GitHub writes.
- Controller owns docs, TODO, proof, integration, and commits.
- Use controlled swarm: max 4 implementation workers for the first wave.
- No two workers edit the same file group in the same wave.
- Worker output is advisory until controller tests pass.
- Every code task starts with tests or fixture/audit failure first.
- Do not mark completion from TUI visuals alone; use tests/artifacts/audit.

## Iteration A — v1.8 closeout + v1.9 swarm foundation

### A0 — Baseline and design lock

- [x] Reconcile `PLAN.md`, `VERSION2.0.md`, `GUIDELINE.md`, and `ARCHITECTURE.md` for policy/lanes/mutation rules.
- [x] Record baseline before implementation:
  ```bash
  git status --short
  make build
  go test ./internal/types ./internal/app ./internal/cmd ./internal/actions ./internal/workqueue ./internal/executor ./internal/monitor/...
  python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
  git diff --check
  ```
- [x] Write file-ownership map for Wave A workers.

### A1 — ActionIntent completeness

Goal: every generated ActionIntent is complete enough for swarm/executor consumption.

- [x] Add/verify fields on every intent:
  - reasons
  - evidence refs
  - confidence
  - policy profile
  - preconditions
  - idempotency key
  - dry-run/write classification
- [x] Add hard audit coverage for intent completeness.
- [x] Regenerate/update current `action-plan.json` artifact; no checked-in fixture change required.
- [x] Preserve Go/Python/TypeScript JSON parity where applicable.

Verification:

```bash
go test ./internal/types ./internal/actions ./internal/app
python -m pytest -q tests/test_audit_guideline.py
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > /tmp/pratc-action-plan.json
```

### A2 — Swarm queue API foundation

Goal: workers can claim, release, heartbeat, and inspect queue state without GitHub writes.

- [x] Add claim endpoint.
- [x] Add release endpoint.
- [x] Add heartbeat endpoint.
- [x] Add queue status endpoint.
- [x] Support filters for lane, priority, state, and expired leases.
- [x] Add route tests and race-safe workqueue tests.

Verification:

```bash
go test ./internal/workqueue ./internal/cache ./internal/cmd
```

### A3 — Proof bundle attach path

Goal: workers can attach proof to work items before executor approval.

- [x] Define persisted proof bundle attachment shape if current type is insufficient.
- [x] Add proof attach/store API.
- [x] Validate work item id, worker id, lease ownership, artifact refs, command result, and proof status.
- [x] Add tests for attach success, invalid item, stale lease, wrong owner, and duplicate/idempotent attach.

Verification:

```bash
go test ./internal/workqueue ./internal/cache ./internal/cmd ./internal/executor
```

### A4 — TUI PR detail inspector

Goal: TUI shows the live per-PR decision truth, not only lane counts.

- [x] Add monitor data model for PR detail.
- [x] Include title, author, age, status, lane, bucket, confidence, reasons, decision layers, evidence refs, duplicate/synthesis refs, risk flags, and allowed actions.
- [x] Add testable render/snapshot path if interactive monitor cannot be tested directly.
- [x] Add/extend TUI tests.

Verification:

```bash
go test ./internal/monitor/...
./bin/pratc monitor --once || true
```

### A5 — Controller integration and OpenClaw proof

- [x] Integrate Wave A changes.
- [x] Run targeted test bundle:
  ```bash
  go test ./internal/types ./internal/actions ./internal/app ./internal/cache ./internal/workqueue ./internal/cmd ./internal/executor ./internal/monitor/...
  python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
  git diff --check
  ```
- [x] Generate current ActionPlan artifact:
  ```bash
  RUN_DIR=autonomous/runs/<run-id>
  mkdir -p "$RUN_DIR"
  ./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > "$RUN_DIR/action-plan.json"
  python3 scripts/audit_guideline.py "$RUN_DIR"
  ```
- [x] Update `autonomous/GAP_LIST.md` and `autonomous/STATE.yaml` only from audit output, not chat summaries.
- [x] Defer runtime proof refresh until implementation commit/final runtime probe boundary; Wave A dirty tmux smoke recorded in `autonomous/STATE.yaml`.

## Stretch only after Iteration A is green

- [ ] `fix_and_merge` sandbox workflow scaffold.
- [ ] TUI queue/proof/executor/rate-limit/audit panels.
- [ ] Live preflight checker scaffold.
- [ ] Guarded comment/label executor in fake backend only.

## Not this iteration

- [ ] Real GitHub mutation.
- [ ] Autonomous merge/close.
- [ ] Direct swarm-worker GitHub access.
- [ ] Browser dashboard revival.
- [ ] Multi-repo orchestration.

## Completion gate for this iteration

- [x] ActionIntent completeness is machine-audited.
- [x] Queue API can claim/release/heartbeat/status safely.
- [x] Proof bundle attach path works and is tested.
- [x] TUI PR detail inspector has testable output.
- [x] OpenClaw ActionPlan run remains audit-green.
- [x] No GitHub writes occurred.
- [x] Runtime proof deferred until implementation commit/final runtime probe boundary; no proof-only refresh mixed into Wave A source edits.
