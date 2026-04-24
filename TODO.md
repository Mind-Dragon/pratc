# prATC TODO — v2.0 Action Engine

## Goal

Build prATC 2.0 as the action engine for a swarm that can safely triage the entire OpenClaw PR corpus, group PRs into multiple action lanes, dispatch autonomous workers, and execute only policy-approved GitHub actions through an audited central executor.

## Source of truth

- `VERSION2.0.md` — v2.0 execution plan and 16-developer swarm map
- `GUIDELINE.md` — action policy, lanes, bucket rules, non-negotiables
- `ARCHITECTURE.md` — product/system shape and data flow
- `AUTONOMOUS.md` — autonomous loop contract
- `autonomous/RUNBOOK.md` — exact operator/controller commands
- `scripts/audit_guideline.py` — deterministic guideline audit
- `projects/openclaw_openclaw/runs/v171-analysis-20260423T234148Z/` — current 1.7.1 OpenClaw snapshot baseline

## Current baseline

- [x] prATC `1.7.1` builds and runs locally
- [x] OpenClaw full-corpus run exists: `projects/openclaw_openclaw/runs/v171-analysis-20260423T234148Z`
- [x] Corpus size: `6,632` PRs
- [x] Audit: `19 passed`, `0 failed`, `0 manual`
- [x] Snapshot PDF exists: `report.pdf`
- [x] Key gap identified: current report/plan are advisory and not safe execution manifests

## Active release path

### v1.8 — Action-readiness dry run

Primary release goal: emit a full-corpus `ActionPlan` and TUI action dashboard in advisory mode with no GitHub writes.

- [x] Define action contracts
  - Added `ActionLane`, `ActionIntent`, `ActionWorkItem`, `ActionPlan`, `PolicyProfile`, `ActionPreflight`, and `ProofBundle` types.
  - Kept JSON tags stable and snake_case where existing contracts require it.
  - Added Go/Python/TypeScript parity coverage and schema fixtures for `action-plan.json`.
- [x] Build deterministic lane classifier
  - Added `internal/actions.ClassifyLane` over review results plus explicit evidence hooks for duplicate, risk, ownership, and mergeability facts.
  - Assigns exactly one primary lane: `fast_merge`, `fix_and_merge`, `duplicate_close`, `reject_or_close`, `focused_review`, `future_or_reengage`, `human_escalate`.
  - Prevents contradictions such as blocked/high-risk PRs receiving merge actions.
- [x] Add policy profiles
  - `advisory`: no executable actions / zero writes.
  - `guarded`: comments/labels only.
  - `autonomous`: merge/close allowed only after required live preflight checks pass; executor ledger remains a v2.0 implementation item.
  - Default remains `advisory`.
- [ ] Add product surfaces
  - CLI: `pratc actions --repo=owner/repo --format=json`.
  - API: `GET /api/repos/{owner}/{repo}/actions`.
  - TUI: read-only action-lane board and PR detail inspector.
  - Report bridge: reuse PDF concepts as dashboard data, not only one-off PDF pages.
- [x] Extend audit checks (Wave 1 local ActionPlan checks)
  - [x] Every PR has one primary action lane in an ActionPlan work-item set.
  - [ ] Every action intent has reasons, evidence refs, confidence, policy, and preconditions.
  - [x] Blocked/high-risk PRs cannot land in `fast_merge` without routing to review/escalation.
  - [x] Advisory mode cannot emit executable mutation side effects.

Verification:

```bash
cd /home/agent/pratc
make build
go test ./internal/types ./internal/app ./internal/cmd
python -m pytest -q tests/test_audit_guideline.py
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > /tmp/pratc-action-plan.json
```

### v1.9 — Swarm dry-run and proof loop

Primary release goal: let a 16-agent swarm claim action work, produce proof bundles, and exercise a dry-run executor without mutating GitHub.

- [ ] Add durable work queue and leases
  - Store work items, states, leases, claim owner, lease expiry, idempotency key, and proof refs.
  - Expired leases return to `claimable` safely.
  - Claims must be race-safe.
- [ ] Add swarm APIs
  - Claim/release/heartbeat endpoints.
  - Lane filters and priority filters.
  - Proof bundle upload/attach path.
  - Queue status surface for TUI.
- [ ] Add dry-run executor
  - Fake GitHub backend for merge/comment/label/close actions.
  - Dry-run mutation records expected action without touching GitHub.
  - Idempotency tests for repeated attempts.
- [ ] Add `fix_and_merge` proof workflow
  - Worktree/checkout preparation.
  - Patch or rebase proof capture.
  - Test command capture.
  - Result attached to work item before executor approval.
- [ ] Add TUI operational panels
  - queue leases
  - proof bundle status
  - executor dry-run stream
  - rate-limit/auth view
  - audit ledger stream

Verification:

```bash
cd /home/agent/pratc
make build
go test ./internal/cache ./internal/actions ./internal/workqueue ./internal/executor ./internal/monitor/...
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > autonomous/runs/<run-id>/action-plan.json
python3 scripts/audit_guideline.py autonomous/runs/<run-id>
```

### v2.0 — Guarded autonomous mutation

Primary release goal: central executor can perform guarded/autonomous GitHub actions only after live preflight, audit, and post-action verification.

- [ ] Add live preflight executor
  - PR still open.
  - analyzed head SHA matches live head SHA or item is revalidated.
  - CI/check suite green when merge is requested.
  - mergeability clean when merge is requested.
  - branch protection and required reviews satisfied.
  - token permission sufficient.
  - action policy allows requested mutation.
- [ ] Add guarded actions
  - comment duplicate/rejection recommendations.
  - label action lanes where configured.
  - never merge or close in guarded mode.
- [ ] Add autonomous actions
  - merge `fast_merge` only after all gates pass.
  - close/comment `duplicate_close` and `reject_or_close` only with high-confidence disposal reasons.
  - merge `fix_and_merge` only after proof bundle validation and fresh preflight.
- [ ] Add audit ledger and verification
  - append-only transition ledger.
  - mutation result capture.
  - post-action verification.
  - failure returns item to blocked/escalated state.
- [ ] Add operator controls
  - TUI hold/resume.
  - policy profile switch visibility.
  - panic stop that prevents new executor actions.

Verification:

```bash
cd /home/agent/pratc
make build
go test ./...
make test-python
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > autonomous/runs/<run-id>/action-plan.json
python3 scripts/audit_guideline.py autonomous/runs/<run-id>
git diff --check
```

## 16-developer swarm assignment

Controller owns integration, status, docs, and verification. Workers do not share mutable files inside the same wave.

1. Governance/contracts — root docs and terminology audit
2. Go type surface — `internal/types/*`, `contracts/*`
3. Python/TypeScript parity — `ml-service/src/pratc_ml/*`, retained TS types
4. Lane classifier — `internal/actions/classifier.go`
5. Policy profiles — `internal/actions/policy.go`
6. ActionPlan service — `internal/app/*`
7. CLI command — `cmd/pratc/*`, `internal/cmd/*`
8. HTTP API — `internal/cmd/root.go`, route tests
9. Persistence/migrations — `internal/cache/*`
10. Queue/leases — `internal/workqueue/*` or action queue package
11. Dry-run executor — `internal/executor/*`, fake GitHub backend
12. Live preflight — `internal/github/*`, executor preflight
13. Fix-and-merge sandbox — `internal/repo/*`, proof helpers
14. TUI dashboard — `internal/monitor/tui/*`
15. Audit checks — `scripts/audit_guideline.py`, audit tests
16. Integration/e2e — end-to-end fixtures, runbook, smoke checks

## Barriered execution

### Barrier 0 — design lock

- [ ] `VERSION2.0.md`, `GUIDELINE.md`, and `ARCHITECTURE.md` agree on lanes, policy profiles, and mutation rules.
- [ ] Controller records baseline `git status --short` and test status.

### Wave 1 — contracts and classifier foundation

Lanes: 1-5.

- [x] action types
- [x] schema fixtures
- [x] lane classifier
- [x] policy profile gates
- [x] doc terminology synced

### Wave 2 — product surfaces and persistence

Lanes: 6-10.

- [ ] service method
- [ ] CLI command
- [ ] HTTP route
- [ ] migrations
- [ ] queue leases

### Wave 3 — executor, proof, TUI

Lanes: 11-14.

- [ ] dry-run executor
- [ ] preflight checker
- [ ] proof bundle flow
- [ ] TUI action dashboard

### Wave 4 — audit and OpenClaw dry run

Lanes: 15-16 plus fix lanes.

- [ ] v2 audit checks
- [ ] OpenClaw ActionPlan artifact
- [ ] dry-run executor proof
- [ ] TUI smoke proof
- [ ] final docs/runbook sync

## Done means

- [ ] every OpenClaw PR has one action lane
- [ ] every action intent has reasons, evidence, confidence, preconditions, and idempotency key
- [ ] advisory mode proves no writes
- [ ] guarded mode cannot merge or close
- [ ] autonomous mode cannot bypass live preflight
- [ ] swarm workers never mutate GitHub directly
- [ ] TUI exposes the living version of the report concepts
- [ ] OpenClaw full-corpus v2 run is audit-green
