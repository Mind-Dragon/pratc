# prATC Stabilization + Omni-Batching + Upstream PR Test Protocol

## TL;DR
> **Summary**: Stabilize the current `batch-remediation` working tree to a clean, deterministic baseline, then add a new large-scale omni-batching capability (AND/OR selector algebra + staged batch recomposition) without breaking existing `/plan` behavior.
> **Deliverables**:
> - Stable baseline (sync/API/UI/test correctness)
> - New omni-batch API + UI flow for 5,000-6,400 PR processing
> - CI-stable full-confidence test protocol aligned to `openclaw/openclaw`
> **Effort**: XL
> **Parallel**: YES - 4 waves
> **Critical Path**: T1 → T3 → T6 → T8 → T9 → T10 → T11 → T12

## Context
### Original Request
- Do A: stabilization/cleanup plan for current git state.
- Prepare proper test-run plan for upstream PR workflow.
- Extend scope to include UI AND/OR selection + large-scale batching (5,000+ PRs, staged batches, omni recomposition).

### Interview Summary
- Scope locked to ONE combined plan with two tracks:
  - **A**: Stabilize current local branch state.
  - **B**: Build new large-scale omni-batching feature.
- Default verification gate is **Full confidence tier**.
- Default test stance is **CI-stable only** (no non-deterministic live/provider tests in merge gate).
- Upstream protocol target is **`openclaw/openclaw`**.

### Metis Review (gaps addressed)
- Freeze behavior contracts before coding (sync 200/202 semantics, selector grammar, staged recomposition contract).
- Keep existing `/plan` backward-compatible; add new omni path instead of replacing.
- Explicitly handle hard caps (64 planning pool, 100 fetch batching) and do recomposition above those constraints.
- Add deterministic tie-break rules and staged telemetry to prevent silent skew and non-reproducible results.

## Work Objectives
### Core Objective
Ship a verified, backward-compatible omni-batching workflow for large PR volumes, starting from a stabilized baseline and ending with deterministic, evidence-backed full-tier verification.

### Deliverables
- Stabilized codebase for existing modified/untracked files.
- New selector algebra (AND/OR) for explicit PR numbers + ranges.
- New staged batch orchestration + omni recomposition backend path.
- UI support for selector input, range processing, staged progress, and final proposed series.
- CI-stable full-tier test protocol/checklist aligned to `openclaw/openclaw` expectations.

### Definition of Done (verifiable conditions with commands)
- `make build` exits 0.
- `make test` exits 0.
- `make lint` exits 0.
- `cd web && bun run build` exits 0.
- New omni path processes fixture-scale 6.4k simulation in bounded staged batches and emits deterministic ordered output across repeated runs.
- Existing `/plan` response contract remains unchanged for legacy requests.

### Must Have
- Zero regressions on existing sync/analyze behavior contract.
- Deterministic selector semantics (explicit precedence + tie-breaking).
- Staged batching with recomposition that respects current formula/pipeline limits.
- Evidence files for each task under `.sisyphus/evidence/`.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- No breaking change to existing `/plan` parameters/shape.
- No rollout of non-deterministic live/provider tests in default merge gate.
- No scope drift into multi-repo UI, gRPC, GitHub App/OAuth, or auto PR actions.
- No “best effort” partial success without explicit incomplete/failure signaling.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: **tests-after** (stabilization first, then feature TDD-like incremental test additions).
- Frameworks: Go test, Vitest, pytest, repo Make targets, contract/perf scripts.
- QA policy: every task has happy + failure/edge scenario.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
Wave 1 (foundation/stabilization contracts): T1, T2, T3, T4
Wave 2 (stabilization hardening + upstream protocol codification): T5, T6, T7
Wave 3 (omni backend core): T8, T9, T10
Wave 4 (omni UI + scale verification): T11, T12

### Dependency Matrix (full)
- T1 blocks T3, T6
- T2 blocks T6
- T3 blocks T5, T6
- T4 blocks T7
- T5 blocks T6
- T6 blocks T8
- T7 informs T12 gate wording
- T8 blocks T9
- T9 blocks T10
- T10 blocks T11, T12
- T11 blocks T12

### Agent Dispatch Summary
- Wave 1: 4 tasks → unspecified-high (backend/api), visual-engineering (web), quick (tests)
- Wave 2: 3 tasks → unspecified-high, writing
- Wave 3: 3 tasks → deep (architecture-sensitive backend)
- Wave 4: 2 tasks → visual-engineering + unspecified-high

## TODOs

- [x] 1. Freeze analyze/sync behavior contract (200 vs 202 semantics)

  **What to do**: Define and implement explicit contract for analyze when sync is stale/in-progress across CLI + API + UI. Document expected payload fields and status codes.
  **Must NOT do**: Change unrelated route shapes or settings endpoints.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: cross-layer contract behavior.
  - Skills: [`superpowers/systematic-debugging`] — prevent semantic drift.
  - Omitted: [`superpowers/test-driven-development`] — contract-first patching with immediate verification is sufficient.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T3,T6 | Blocked By: none

  **References**:
  - Pattern: `internal/cmd/root.go` — current analyze + sync status flow
  - Pattern: `internal/cmd/sync_api_test.go` — existing 202/200 assertions
  - UI: `web/src/pages/index.tsx` — sync-in-progress render path
  - Test: `web/src/__tests__/pages/index.test.tsx` — sync-state expectations

  **Acceptance Criteria**:
  - [ ] `go test ./internal/cmd -run "TestHandleAnalyze|TestHandleRepoActionSyncStatus" -count=1` passes
  - [ ] `cd web && bun run test -- index.test.tsx` passes

  **QA Scenarios**:
  ```
  Scenario: stale cache triggers sync-in-progress response
    Tool: Bash
    Steps: start server; request /api/repos/<repo>/analyze with stale cache conditions
    Expected: HTTP 202 and payload includes sync_status=in_progress
    Evidence: .sisyphus/evidence/task-1-analyze-sync-contract.txt

  Scenario: fresh cache returns analysis payload
    Tool: Bash
    Steps: seed/complete sync; request /api/repos/<repo>/analyze
    Expected: HTTP 200 with counts/analysis fields
    Evidence: .sisyphus/evidence/task-1-analyze-ready.txt
  ```

  **Commit**: YES | Message: `fix(cmd): lock analyze sync response contract` | Files: `internal/cmd/root.go`, `internal/cmd/sync_api_test.go`, `web/src/pages/index.tsx`, `web/src/__tests__/pages/index.test.tsx`

- [x] 2. Resolve web build blocker and stabilize dashboard compile path

  **What to do**: Fix current TypeScript/export mismatch(es) and ensure dashboard build passes with current feature set.
  **Must NOT do**: Introduce new UI features in this task.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: TS/web compile pipeline.
  - Skills: [`superpowers/systematic-debugging`] — isolate root compile cause.
  - Omitted: [`superpowers/test-driven-development`] — compile-fix task.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T6 | Blocked By: none

  **References**:
  - Pattern: `web/src/lib/api.ts` — exported client surface
  - Pattern: `web/src/pages/*.tsx` — import usage
  - Test: `web/src/__tests__/` — existing API/page tests

  **Acceptance Criteria**:
  - [ ] `cd web && bun run build` exits 0
  - [ ] `cd web && bun run test` exits 0

  **QA Scenarios**:
  ```
  Scenario: dashboard compiles with no TS export mismatch
    Tool: Bash
    Steps: run bun run build in web/
    Expected: successful build, zero type export errors
    Evidence: .sisyphus/evidence/task-2-web-build.txt

  Scenario: regression check for primary pages
    Tool: Bash
    Steps: run bun run test in web/
    Expected: pages and lib tests pass
    Evidence: .sisyphus/evidence/task-2-web-tests.txt
  ```

  **Commit**: YES | Message: `fix(web): restore stable dashboard build` | Files: `web/src/**`

- [x] 3. Harden sync/SSE determinism and test safety constraints

  **What to do**: Ensure SSE replay and sync job lifecycle remain deterministic; remove/avoid `t.Parallel` where `t.Setenv` is used.
  **Must NOT do**: Reintroduce synthetic completion shortcuts.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: concurrency correctness.
  - Skills: [`superpowers/systematic-debugging`] — race/state issues.
  - Omitted: [`superpowers/test-driven-development`] — tests already exist and need hardening.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T5,T6 | Blocked By: T1

  **References**:
  - Pattern: `internal/sync/sse.go`, `internal/sync/sse_test.go`
  - Pattern: `internal/cmd/sync_api_test.go`
  - Notes: `.sisyphus/notepads/batch-remediation/issues.md`

  **Acceptance Criteria**:
  - [ ] `go test ./internal/sync ./internal/cmd -count=1` passes
  - [ ] `go test -race ./internal/sync ./internal/cmd` passes

  **QA Scenarios**:
  ```
  Scenario: late subscriber gets deterministic replay
    Tool: Bash
    Steps: run targeted sse replay test twice
    Expected: identical assertions pass both runs
    Evidence: .sisyphus/evidence/task-3-sse-determinism.txt

  Scenario: sync runner failure marks job failed
    Tool: Bash
    Steps: run sync API failure-path test
    Expected: sync_jobs.status becomes failed, not stuck in_progress
    Evidence: .sisyphus/evidence/task-3-sync-failure-path.txt
  ```

  **Commit**: YES | Message: `test(sync): enforce deterministic sse and job lifecycle` | Files: `internal/sync/*`, `internal/cmd/sync_api_test.go`

- [x] 4. Codify openclaw-aligned full-tier CI-stable test protocol for PR runs

  **What to do**: Add local testing protocol doc/checklist reflecting `openclaw/openclaw` expectations (build/check/test/docs, verification evidence fields).
  **Must NOT do**: Copy unrelated openclaw internals into prATC runtime.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: protocol/spec artifact quality.
  - Skills: []
  - Omitted: [`superpowers/systematic-debugging`] — documentation/protocol task.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T7 | Blocked By: none

  **References**:
  - External: `https://github.com/openclaw/openclaw/blob/main/CONTRIBUTING.md`
  - External: `https://github.com/openclaw/openclaw/blob/main/.github/pull_request_template.md`
  - Pattern: `AGENTS.md`, `CONTRIBUTING.md`, `.sisyphus/evidence/`

  **Acceptance Criteria**:
  - [ ] protocol doc includes mandatory command order, evidence format, and failure triage
  - [ ] protocol references CI-stable-only default and live-test exclusion by default

  **QA Scenarios**:
  ```
  Scenario: protocol checklist is executable
    Tool: Bash
    Steps: follow documented command sequence end-to-end
    Expected: all listed commands are valid in repo context
    Evidence: .sisyphus/evidence/task-4-protocol-dryrun.txt

  Scenario: protocol failure branch exists
    Tool: Read
    Steps: verify doc has explicit fail-fast and triage section
    Expected: contains deterministic next actions for failed gates
    Evidence: .sisyphus/evidence/task-4-protocol-triage.md
  ```

  **Commit**: YES | Message: `docs(testing): add openclaw-aligned pr validation protocol` | Files: `docs/**` or `AGENTS.md` additions

- [x] 5. Finalize stabilization regression pack and baseline evidence

  **What to do**: Run and record stabilization evidence across Go/Python/Web + build/lint.
  **Must NOT do**: Skip failing areas with partial claims.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: verification orchestration.
  - Skills: [`superpowers/verification-before-completion`]
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: T6 | Blocked By: T3

  **References**:
  - Pattern: `Makefile` test/build/lint targets
  - Evidence: `.sisyphus/evidence/`

  **Acceptance Criteria**:
  - [ ] `make build` passes
  - [ ] `make test` passes
  - [ ] `make lint` passes

  **QA Scenarios**:
  ```
  Scenario: full local verification baseline
    Tool: Bash
    Steps: run make build, make test, make lint sequentially
    Expected: all exit 0
    Evidence: .sisyphus/evidence/task-5-baseline-full.txt

  Scenario: targeted regression replay
    Tool: Bash
    Steps: rerun previously failing command subsets
    Expected: no recurring failures
    Evidence: .sisyphus/evidence/task-5-regression-replay.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 6. Introduce selector algebra contract (AND/OR over explicit PR IDs + ranges)

  **What to do**: Define selector AST + parser + deterministic precedence rules and errors.
  **Must NOT do**: Add ambiguous implicit precedence.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: parser/semantics correctness.
  - Skills: [`superpowers/systematic-debugging`]
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: T8,T9 | Blocked By: T1,T5

  **References**:
  - Pattern: `internal/types/models.go` for request/response contracts
  - Pattern: `internal/cmd/root.go` query parsing
  - Oracle guidance: selector algebra set semantics + deterministic ordering

  **Acceptance Criteria**:
  - [ ] parser/evaluator tests cover explicit IDs, ranges, AND, OR, mixed groups
  - [ ] invalid syntax returns deterministic 400 error payload

  **QA Scenarios**:
  ```
  Scenario: mixed selector resolves deterministically
    Tool: Bash
    Steps: call omni endpoint with selector '(pr:1..100 AND pr:5,7,9) OR pr:200'
    Expected: stable sorted set with deterministic tie-break rules
    Evidence: .sisyphus/evidence/task-6-selector-determinism.txt

  Scenario: invalid selector rejected cleanly
    Tool: Bash
    Steps: call endpoint with malformed selector 'pr:1.. AND'
    Expected: HTTP 400 with parse error code/message
    Evidence: .sisyphus/evidence/task-6-selector-invalid.txt
  ```

  **Commit**: YES | Message: `feat(planning): add deterministic selector algebra` | Files: `internal/planning/**`, `internal/cmd/**`, `internal/types/**`

- [x] 7. Add new omni-batch API path (backward-compatible with existing /plan)

  **What to do**: Introduce dedicated request path/handler for staged batch planning while preserving legacy `/plan` behavior.
  **Must NOT do**: Change existing `/plan` response shape for old clients.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: API compatibility-critical.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8,T11 | Blocked By: T4,T6

  **References**:
  - Pattern: `internal/cmd/root.go` existing `handlePlan`
  - Pattern: `docs/api-contracts.md`

  **Acceptance Criteria**:
  - [ ] existing `/plan` contract tests remain green
  - [ ] new omni endpoint returns selector + stage metadata

  **QA Scenarios**:
  ```
  Scenario: legacy /plan unchanged
    Tool: Bash
    Steps: call existing /api/plan with current params
    Expected: response keys unchanged from baseline
    Evidence: .sisyphus/evidence/task-7-legacy-compat.txt

  Scenario: omni endpoint returns staged envelope
    Tool: Bash
    Steps: call new /api/plan/omni endpoint with selector and stage config
    Expected: response includes stage summaries + final ordering payload
    Evidence: .sisyphus/evidence/task-7-omni-endpoint.txt
  ```

  **Commit**: YES | Message: `feat(api): add backward-compatible omni batching endpoint` | Files: `internal/cmd/root.go`, `internal/types/models.go`, tests

- [x] 8. Implement stage-1 batch processor for 5k-6.4k PR intake

  **What to do**: Process selector result set in fixed-size stages; run existing filter/scoring in each stage; collect per-stage summaries.
  **Must NOT do**: Feed >64 candidates directly into formula search.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: scaling + algorithmic constraints.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T9,T10 | Blocked By: T6,T7

  **References**:
  - Pattern: `internal/filter/pipeline.go` cap behavior
  - Pattern: `internal/formula/config.go` MaxPoolSize
  - Pattern: `internal/repo/mirror.go` 100-ref batching

  **Acceptance Criteria**:
  - [ ] stage processor respects configurable stage size and hard planning caps
  - [ ] stage telemetry includes pre/post counts and rejection reasons

  **QA Scenarios**:
  ```
  Scenario: 6400 PRs split into deterministic stages
    Tool: Bash
    Steps: run stage processor on fixture-scale input with stage_size=64 or configured value
    Expected: stable stage count/order and complete coverage of selected PR set
    Evidence: .sisyphus/evidence/task-8-stage-coverage.txt

  Scenario: stage overflow guarded by cap rules
    Tool: Bash
    Steps: run with oversized candidate set per stage
    Expected: capping and rejection reasons emitted; no formula panic
    Evidence: .sisyphus/evidence/task-8-stage-cap-guard.txt
  ```

  **Commit**: YES | Message: `feat(planning): add stage-1 large-scale batch processor` | Files: `internal/app/**`, `internal/planning/**`, tests

- [x] 9. Implement stage-2 omni recomposition and final proposal ordering

  **What to do**: Merge stage finalists, dedupe by PR number, enforce deterministic tie-breaks, run final global ordering.
  **Must NOT do**: silently drop duplicates without telemetry.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: recomposition correctness and determinism.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T10,T11 | Blocked By: T8

  **References**:
  - Pattern: `internal/planner/planner.go` ordering/rejections
  - Pattern: `internal/graph/graph.go` dependency ordering

  **Acceptance Criteria**:
  - [ ] recomposed output has no duplicate PR numbers
  - [ ] repeated identical runs produce identical ordering

  **QA Scenarios**:
  ```
  Scenario: duplicate finalists deduped deterministically
    Tool: Bash
    Steps: inject overlapping PR finalists across stages
    Expected: one entry per PR, stable ordering tie-break by PR number
    Evidence: .sisyphus/evidence/task-9-dedupe-order.txt

  Scenario: cross-stage dependency ordering preserved
    Tool: Bash
    Steps: include dependency edges across stages in input fixture
    Expected: final ordering respects dependency graph constraints
    Evidence: .sisyphus/evidence/task-9-cross-stage-deps.txt
  ```

  **Commit**: YES | Message: `feat(planning): add omni recomposition and deterministic ordering` | Files: `internal/app/**`, `internal/planner/**`, tests

- [x] 10. Add omni-batching UI controls and results workflow

  **What to do**: Add UI support for explicit PR IDs, range scan, AND/OR combination, stage progress summary, and final proposed series visualization.
  **Must NOT do**: remove legacy plan page behavior.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: UX/interaction-heavy implementation.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: T12 | Blocked By: T7,T9

  **References**:
  - Pattern: `web/src/pages/plan.tsx`
  - Pattern: `web/src/pages/settings.tsx` (range controls)
  - Pattern: `web/src/components/TriageView.tsx` (selection model)
  - API: `web/src/lib/api.ts`

  **Acceptance Criteria**:
  - [ ] UI supports explicit IDs, ranges, and mixed AND/OR request construction
  - [ ] UI renders stage summaries and final proposal list from omni endpoint

  **QA Scenarios**:
  ```
  Scenario: mixed selector request from UI
    Tool: Playwright
    Steps: enter explicit IDs + range; choose AND/OR mode; submit
    Expected: request payload matches selector algebra contract; results render
    Evidence: .sisyphus/evidence/task-10-ui-selector-flow.png

  Scenario: invalid selector surfaced in UI
    Tool: Playwright
    Steps: submit malformed selector input
    Expected: user-visible validation/error from API parse failure
    Evidence: .sisyphus/evidence/task-10-ui-invalid-selector.png
  ```

  **Commit**: YES | Message: `feat(web): add omni batching selector and staged results ui` | Files: `web/src/pages/**`, `web/src/components/**`, tests

- [x] 11. Scale/performance verification for omni mode at 5k-6.4k envelope

  **What to do**: Add/execute deterministic perf validation for staged omni flow under fixture-scale loads; record latency and memory observations.
  **Must NOT do**: claim perf pass without captured evidence artifacts.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: perf benchmarking + evidence.
  - Skills: [`superpowers/verification-before-completion`]
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: T12 | Blocked By: T8,T9,T10

  **References**:
  - SLO: `AGENTS.md`, `docs/api-contracts.md`
  - Existing perf tests: `internal/app/service_test.go`, `internal/repo/mirror_test.go`
  - Script: `scripts/slo_benchmark.sh`

  **Acceptance Criteria**:
  - [ ] staged omni test run completes within agreed envelope for fixture-scale input
  - [ ] evidence logs include duration + memory and stage metrics

  **QA Scenarios**:
  ```
  Scenario: fixture-scale staged throughput
    Tool: Bash
    Steps: run benchmark/tests for omni path with 5k-6.4k fixture simulation
    Expected: completes successfully within defined thresholds
    Evidence: .sisyphus/evidence/task-11-omni-scale.txt

  Scenario: stress failure mode behavior
    Tool: Bash
    Steps: force oversized or malformed stage configuration
    Expected: graceful error with explicit reason, no crash/partial silent success
    Evidence: .sisyphus/evidence/task-11-omni-stress-failure.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`, optional benchmark tests

- [x] 12. Execute full CI-stable gate and prepare upstream-style PR verification bundle

  **What to do**: Run the full gate sequence and produce PR evidence package aligned to openclaw-style expectations (summary, root cause, tests, evidence, risks).
  **Must NOT do**: include non-deterministic live/provider results in default pass criteria.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: final verification narrative + checklist quality.
  - Skills: [`superpowers/verification-before-completion`]
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: Final Wave | Blocked By: T5,T10,T11

  **References**:
  - External: `https://github.com/openclaw/openclaw/blob/main/CONTRIBUTING.md`
  - External: `https://github.com/openclaw/openclaw/blob/main/.github/pull_request_template.md`
  - Local: `.sisyphus/evidence/`, `AGENTS.md`, `CONTRIBUTING.md`

  **Acceptance Criteria**:
  - [ ] `make build && make test && make lint` all pass
  - [ ] `cd web && bun run build` passes
  - [ ] verification bundle includes command outputs + failure triage notes + risk summary

  **QA Scenarios**:
  ```
  Scenario: full gate pass
    Tool: Bash
    Steps: run make build, make test, make lint, web build
    Expected: all commands exit 0
    Evidence: .sisyphus/evidence/task-12-full-gate.txt

  Scenario: fail-fast triage protocol
    Tool: Read
    Steps: inspect verification bundle for explicit failed-gate handling section
    Expected: deterministic triage actions and blocker classification present
    Evidence: .sisyphus/evidence/task-12-triage-protocol.md
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`, `.sisyphus/status/**`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Commit per task (or per tightly-coupled micro-group) with conventional messages.
- Never mix stabilization-only fixes with new-feature work in same commit.
- Keep protocol/evidence commits separate from runtime logic commits.
- Suggested sequence:
  1. `fix(cmd|sync|web): stabilize batch-remediation regressions`
  2. `feat(planning): selector algebra + omni endpoint`
  3. `feat(planning): staged batch processor + recomposition`
  4. `feat(web): omni batching ui`
  5. `test(omni): add scale/perf verification`
  6. `docs(testing): ci-stable full-tier protocol`

## Success Criteria
- Existing stabilization diff is fully validated and no known blockers remain.
- Omni path supports explicit IDs/ranges with deterministic AND/OR behavior.
- 5k-6.4k staged processing produces reproducible final proposal ordering.
- Legacy `/plan` behavior remains backward-compatible.
- Full CI-stable verification gate passes with evidence artifacts.
- Final Wave F1-F4 all APPROVE and user gives explicit final okay.
