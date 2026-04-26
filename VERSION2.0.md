# prATC Version 2.0 Plan — Action Engine + Swarm Control Plane

> For Hermes: use `subagent-driven-development` and `swarm-orchestrator` to execute this plan wave-by-wave. Use 16 parallel developer lanes only where file ownership is non-overlapping. Controller owns docs, integration, verification, and final status updates.

## Goal

Turn prATC from a full-corpus advisory triage engine into the engine a swarm can point at to classify, queue, fix, merge, close, reject, and escalate GitHub PRs with auditable safety gates.

## Current baseline

Validated current-HEAD snapshot: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z`

- prATC binary line: `1.7.1`, commit `8d80f7580c74`, `dirty=false`
- repo: `openclaw/openclaw`
- corpus: `6,632` PRs
- clusters: `81`
- duplicate groups: `95`
- collapsed duplicate groups: `85`
- garbage PRs: `14`
- stale PRs: `4,363`
- conflict pairs: `91`
- audit: `22 passed`, `0 failed`, `0 manual`
- action plan: schema `2.0`, policy `advisory`, `6,632` work items, `182` intents
- lanes: `duplicate_close=91`, `human_escalate=6541`
- report: `report.pdf`, one-off snapshot artifact

Important gap from this run:

- `plan` still returns one chosen set, historically `target=20`.
- `review_payload.next_action=merge` is too broad for autonomous action.
- many `merge_now` records are also `priority_tier=blocked`.
- confidence and risk buckets are not calibrated enough for mutation decisions.
- current `GUIDELINE.md` still forbids auto-merge under the v1.x advisory contract.

2.0 must not treat the 1.7.1 report as an execution manifest. It must convert the same underlying concepts into a live action dashboard, a machine-readable ActionPlan, and a guarded executor path.

## Product model

prATC 2.0 has three layers:

1. Evidence engine
   - ingest the full PR corpus
   - run the 16-gate ladder
   - preserve reasons, evidence, duplicate synthesis, risks, and confidence

2. Action engine
   - assign every PR to an action lane
   - emit typed ActionIntents with preconditions and evidence refs
   - expose durable work queues for swarms
   - keep advisory mode as the default

3. Execution engine
   - perform only policy-allowed mutations
   - centralize GitHub writes behind live preflight
   - verify each action after execution
   - record every transition in an audit ledger

The swarm does not directly mutate GitHub. The swarm claims work, produces proof bundles, and asks the prATC executor to act.

## Policy profiles

### `advisory`

Default. No GitHub mutations. Produces ActionPlan, dashboard lanes, and PDF/snapshot reports only.

### `guarded`

Allows non-destructive GitHub actions only:

- comments
- labels
- status/check annotations if implemented
- duplicate/rejection recommendation comments

No merge, close, branch update, or push.

### `autonomous`

Allows mutation only through typed ActionIntents that pass all required gates:

- merge
- close
- comment
- label
- request changes / reject when supported
- apply swarm-generated fix branch when explicitly enabled

Every mutation requires live preflight, idempotency key, audit ledger write, and post-action verification.

## Action lanes

Every PR must land in exactly one primary lane and may hold additive risk flags.

### `fast_merge`

Clean, low-risk PRs that can be merged after live preflight.

Required signals:

- CI passing
- mergeable clean
- not draft
- head SHA unchanged or revalidated
- no duplicate/superseded relationship
- low blast radius
- no unresolved security/reliability/performance risk
- high confidence with calibrated reason trail

### `fix_and_merge`

Valid PRs that are close enough for a swarm to fix.

Common reasons:

- test gap
- lint or CI failure likely caused by small drift
- rebase conflict with limited footprint
- minor code quality issue
- missing generated artifact
- review comment repair

Swarm output must include a proof bundle before merge can be considered.

### `duplicate_close`

Duplicate or superseded PRs that should be closed or commented with a canonical link.

Required signals:

- duplicate group ID
- canonical PR number
- duplicate confidence
- closure/comment text preview
- no unresolved conflict over canonical selection

### `reject_or_close`

Invalid PRs that should be closed, rejected, or quarantined.

Allowed reasons:

- junk/spam/malformed
- abandoned low-signal PR
- unsafe/malicious pattern
- invalid target branch or structurally impossible change
- generated noise with no durable value

Disposal requires stronger confidence than ordinary routing.

### `focused_review`

Meaningful PRs that need deeper agent/human review before action.

These can become `fast_merge`, `fix_and_merge`, `future_or_reengage`, or `human_escalate` after review.

### `future_or_reengage`

Valid work that does not belong in the current merge cycle.

Examples:

- good idea, wrong priority window
- stale but worth reviving
- blocked on product decision
- needs author response

### `human_escalate`

PRs that must not be autonomously acted on.

Triggers:

- low confidence
- high blast radius
- security-sensitive code
- unclear ownership
- legal/licensing concerns
- conflicting evidence
- policy gap

## ActionPlan contract

`action-plan.json` is the 2.0 swarm interface.

Required top-level fields:

```json
{
  "schema_version": "2.0",
  "run_id": "...",
  "repo": "owner/repo",
  "policy_profile": "advisory|guarded|autonomous",
  "generated_at": "...",
  "corpus_snapshot": {
    "total_prs": 0,
    "head_sha_indexed": true,
    "analysis_truncated": false,
    "max_prs_applied": 0
  },
  "lanes": [],
  "work_items": [],
  "action_intents": [],
  "audit": {}
}
```

Each work item must include:

- work item ID
- PR number
- lane
- state
- priority score
- confidence
- risk flags
- reason trail
- evidence refs
- required preflight checks
- idempotency key
- lease state
- allowed actions
- blocked reasons
- proof bundle refs, if any

## State machine

Action work item states:

1. `proposed`
2. `claimable`
3. `claimed`
4. `preflighted`
5. `patched`
6. `tested`
7. `approved_for_execution`
8. `executed`
9. `verified`
10. `failed`
11. `escalated`
12. `canceled`

No state may be skipped unless the skip is recorded as an explicit transition with a reason.

## Live preflight gates

Before any GitHub mutation:

- PR still exists and is open
- analyzed head SHA equals live head SHA or item is revalidated
- base branch still matches policy
- mergeability is clean for merge actions
- CI/check suite is green for merge actions
- required reviews and branch protection are satisfied
- token has required permission
- rate-limit budget is sufficient
- no active conflicting work item owns the same PR or branch
- action is allowed by current policy profile
- idempotency key has not already executed

## TUI dashboard

The PDF is a snapshot packet. In 2.0, its useful concepts move into a live terminal dashboard.

First-class TUI panels:

1. Corpus overview
   - total PRs
   - sync freshness
   - truncation/cap status
   - audit status

2. Action lane board
   - counts by lane
   - queue depth
   - blocked/escalated counts
   - drift since last run

3. PR detail inspector
   - title, author, age, status
   - decision layer journey
   - reason trail
   - evidence refs
   - duplicate/synthesis data
   - risk flags

4. Swarm work queue
   - claim leases
   - assigned worker
   - state machine status
   - proof bundle status
   - stale lease detection

5. Executor console
   - pending ActionIntents
   - live preflight results
   - executed mutations
   - post-action verification
   - rollback/escalation notes

6. Rate-limit and auth panel
   - token-source inventory, redacted
   - active token index
   - remaining budget
   - retry/backoff state

7. Audit ledger stream
   - action transitions
   - policy denials
   - failed preflights
   - human overrides

The TUI is a control and observation surface for prATC 2.0. The browser dashboard remains deferred unless explicitly revived later.

## Release path

### v1.8 — Action-readiness dry run

Primary outcome: prATC can generate trustworthy full-corpus ActionPlans without mutating GitHub.

Deliverables:

- `ActionLane`, `ActionIntent`, `ActionWorkItem`, `ActionPlan` types
- deterministic lane classifier
- policy profiles with `advisory` as default
- `pratc actions --repo=... --format=json`
- `/api/repos/{owner}/{repo}/actions`
- TUI action-lane dashboard read-only view
- report/dashboard data bridge
- v2 audit checks for lane coverage and unsafe action contradictions
- OpenClaw full-corpus dry-run ActionPlan artifact

Exit gate:

- every PR in the OpenClaw corpus is assigned to exactly one primary action lane
- no blocked PR emits merge intent
- no high-risk PR lands in `fast_merge`
- every ActionIntent has preconditions, reasons, evidence refs, and policy profile
- `advisory` mode performs zero GitHub writes

### v1.9 — Swarm dry-run and proof bundles

Primary outcome: 16-agent swarms can claim work from prATC, produce proof bundles, and drive dry-run executor transitions.

Deliverables:

- durable queue/lease store
- claim/release/heartbeat APIs
- proof bundle schema
- dry-run GitHub executor
- fake GitHub mutation harness
- `fix_and_merge` sandbox workflow
- duplicate/reject comment preview generator
- TUI lease/proof/executor panels
- swarm runbook and prompt set

Exit gate:

- multiple workers can safely claim disjoint work without races
- expired leases return to claimable state
- proof bundles are attached to the right work item
- dry-run executor proves what would happen without mutating GitHub
- OpenClaw dry-run swarm processes a representative queue across all lanes

### v2.0 — Guarded autonomous mutation

Primary outcome: prATC can execute policy-approved GitHub actions through a centralized, audited executor.

Deliverables:

- live preflight executor
- guarded comment/label actions
- duplicate close/comment actions
- reject/close actions
- merge actions for `fast_merge`
- swarm-fixed merge path for `fix_and_merge`
- post-action verification loop
- rollback/escalation policy
- mutation audit ledger
- operator hold/resume controls in TUI

Exit gate:

- guarded mode succeeds on comment/label actions only
- autonomous mode executes only actions that pass preflight
- all mutations are idempotent and auditable
- failed preflights return items to blocked/escalated lanes
- no direct swarm-to-GitHub mutation path exists

## 16-developer swarm plan

Use exactly one controller/integrator and up to 16 implementation lanes. Do not let two developers modify the same file group in the same wave.

### Controller responsibilities

- own `VERSION2.0.md`, `TODO.md`, status docs, and final integration
- run baseline checks before each wave
- enforce file ownership
- run tests and audits after each wave
- keep `main` green before declaring completion
- do not let workers commit unless explicitly isolated

### Developer lanes

1. Governance/contracts
   - Files: `GUIDELINE.md`, `ARCHITECTURE.md`, `README.md`, `ROADMAP.md`, `TODO.md`
   - Output: doc consistency patch and terminology audit

2. Go type surface
   - Files: `internal/types/*`, `contracts/*`
   - Output: ActionPlan and action-lane structs with JSON contract tests

3. Python/TypeScript model parity
   - Files: `ml-service/src/pratc_ml/*`, `web/src/types/*` if retained, contract fixtures
   - Output: parity models and schema validation tests

4. Lane classifier core
   - Files: new `internal/actions/classifier.go`, tests
   - Output: deterministic lane assignment from existing analysis/review artifacts

5. Policy profiles and gates
   - Files: new `internal/actions/policy.go`, tests
   - Output: advisory/guarded/autonomous allow/deny rules

6. ActionPlan service wiring
   - Files: `internal/app/service.go`, new service helpers
   - Output: `Actions(ctx, repo, opts)` returns full-corpus ActionPlan

7. CLI command wiring
   - Files: `cmd/pratc/*`, `internal/cmd/*`
   - Output: `pratc actions` command and JSON output contract tests

8. HTTP API wiring
   - Files: `internal/cmd/root.go`, route tests, contracts
   - Output: `/api/repos/{owner}/{repo}/actions`, pagination/filter params

9. Persistence and migrations
   - Files: `internal/cache/*`
   - Output: action plans, work items, leases, proof bundles, ledger tables

10. Queue/lease engine
    - Files: new `internal/workqueue/*` or `internal/actions/queue.go`
    - Output: claim/release/heartbeat/expiry behavior with race-safe tests

11. GitHub executor dry-run
    - Files: `internal/github/*`, new `internal/executor/*`
    - Output: dry-run executor with idempotency and fake GitHub backend

12. Live preflight
    - Files: `internal/github/*`, `internal/executor/preflight.go`
    - Output: SHA, CI, mergeability, branch protection, permission checks

13. Fix-and-merge sandbox
    - Files: `internal/repo/*`, new worker proof helpers
    - Output: local checkout/worktree patch/test/proof bundle flow

14. TUI dashboard
    - Files: `internal/monitor/tui/*`, monitor data adapters
    - Output: live action-lane board, PR detail, queue, executor, and ledger panels

15. Audit and verification
    - Files: `scripts/audit_guideline.py`, `tests/test_audit_guideline.py`, fixtures
    - Output: v2 action safety checks and OpenClaw artifact audit coverage

16. Integration/e2e harness
    - Files: `tests/*`, `scripts/*`, docs runbook
    - Output: fixture-backed end-to-end ActionPlan, queue, dry-run executor, and TUI smoke checks

## Barriered execution waves

### Barrier 0 — design lock

Controller verifies:

```bash
git status --short
go test ./...
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
```

No implementation starts until `VERSION2.0.md`, `GUIDELINE.md`, and `ARCHITECTURE.md` agree on policy profiles, lanes, and mutation rules.

### Wave 1 — contracts and classifier foundation

Lanes: 1, 2, 3, 4, 5

Expected outputs:

- ActionPlan schema
- Go/Python/TypeScript type parity where applicable
- lane classifier tests
- policy profile tests
- docs synced

Barrier check:

```bash
go test ./internal/types ./internal/actions ./internal/app
python -m pytest -q tests/test_audit_guideline.py
```

### Wave 2 — product surfaces and persistence

Lanes: 6, 7, 8, 9, 10

Expected outputs:

- service method
- CLI command
- HTTP route
- SQLite migrations
- queue leases

Barrier check:

```bash
go test ./internal/app ./internal/cmd ./internal/cache ./internal/actions ./internal/workqueue
./bin/pratc actions --repo openclaw/openclaw --force-cache --format=json > /tmp/action-plan.json
python3 scripts/audit_guideline.py projects/openclaw_openclaw/runs/v171-head-20260424T153126Z
```

### Wave 3 — executor, proof, and TUI

Lanes: 11, 12, 13, 14

Expected outputs:
- dry-run executor
- live preflight checker
- proof bundle path
- TUI action dashboard

Barrier check:
```bash
go test ./internal/github ./internal/executor ./internal/repo ./internal/monitor/...
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json
./bin/pratc monitor --once || true
```

`monitor --once` may not exist yet; if not, add a testable render/snapshot function instead of relying on interactive UI.

**Wave B Completion Status (COMPLETED ✓)**
- TUI panels render with live data (verified)
- Live preflight checker enforces all 9 gates
- Guarded comment/label executor works with fake GitHub
- Ledger persistence survives restart (SQLite)
- Post-action verification confirms mutations
- Fix-and-merge sandbox produces valid proof bundles
- E2E harness passes all audit checks
- Docs aligned with implementation

### Wave 4 — audit, e2e, and OpenClaw dry run

Lanes: 15, 16, plus targeted fix lanes from prior wave failures

Expected outputs:

- v2 audit checks
- dry-run end-to-end harness
- OpenClaw ActionPlan artifact
- TUI smoke proof
- updated docs and runbook

Barrier check:

```bash
make build
go test ./...
make test-python
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > autonomous/runs/<run-id>/action-plan.json
python3 scripts/audit_guideline.py autonomous/runs/<run-id>
git diff --check
```

## Acceptance criteria

2.0 is not complete until:

- every PR is accounted for in ActionPlan
- every PR has exactly one primary action lane
- every ActionIntent has preconditions, evidence refs, confidence, risk flags, and idempotency key
- the TUI can navigate the same concepts the PDF summarized
- advisory mode proves zero writes
- guarded mode can perform comment/label actions with audit proof
- autonomous mode cannot bypass preflight
- swarm workers can claim work and attach proof without direct GitHub mutation
- failed/exhausted/expired work items return to safe states
- OpenClaw full-corpus v2 run is audit-green

## Non-goals for 2.0

- browser dashboard revival
- multi-repo orchestration as the primary release target
- online self-training from operator feedback without explicit batch/evaluation gates
- direct swarm worker GitHub mutation
- hidden PR caps
- silent model/provider fallback that changes judgment strength

## Document hierarchy

- `GUIDELINE.md` owns action policy, lanes, bucket rules, and non-negotiables.
- `ARCHITECTURE.md` owns system shape, data flow, and component ownership.
- `VERSION2.0.md` owns this execution plan.
- `TODO.md` mirrors the active implementation queue.
- `AUTONOMOUS.md` and `autonomous/RUNBOOK.md` own controller mechanics.
- `README.md` summarizes public/current status.

If documents conflict on action safety, `GUIDELINE.md` wins.
