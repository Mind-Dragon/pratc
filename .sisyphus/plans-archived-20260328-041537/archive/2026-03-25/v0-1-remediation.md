# prATC v0.1 Remediation Closure Plan

## TL;DR
> **Summary**: Close every unresolved item from the v0.1 contract audit by extracting missing backend packages, restoring CLI/API safety contracts, completing web parity requirements, and finishing objective verification evidence.
> **Deliverables**:
> - Missing backend packages: `internal/filter`, `internal/planner`, `internal/audit`
> - CLI/API contract closure: dry-run defaults, audit command, `/plans` validation coverage, CORS verification, migration-path tests
> - Web closure: settings API compile fixes, `/inbox` route parity, interactive D3 graph, table/action UX alignment
> - Verification closure: SLO/benchmark evidence, docker runtime evidence, final F1-F4 approval wave
> **Effort**: Large
> **Parallel**: YES - 5 waves
> **Critical Path**: 1 -> 4 -> 5 -> 11 -> 14 -> F1-F4

## Context
### Original Request
User requested: review `v0.1-contract-audit-report.md` and make a new plan that takes care of all remaining items.

### Interview Summary
- No additional preference questions were required because gap inventory and priorities were explicit in the audit findings summarized in this plan.
- Plan scope is strictly v0.1 closure (no v0.2 expansion work).

### Metis Review (gaps addressed)
- Required explicit decision on planner naming resolved: keep `internal/planning` algorithms, add `internal/planner` orchestration.
- Required explicit safety guardrails resolved: dry-run defaults and intent-only action logging remain mandatory in v0.1.
- Required explicit verification guardrail resolved: no task completion claims without agent-executed evidence artifacts.

## Work Objectives
### Core Objective
Bring prATC v0.1 to contract-complete status against archived v0.1 requirements (`.sisyphus/plans/archive/2026-03-22/pratc.md`) and the unresolved findings summarized in this plan by addressing all P0-P3 items with reproducible, agent-executed verification.

### Deliverables
- Contract-compliant backend package boundaries for filter/planner/audit responsibilities.
- Contract-compliant CLI safety behavior (`--dry-run` default semantics and `pratc audit`).
- Contract-compliant web functionality for settings API usage, inbox route parity, and interactive dependency graph rendering.
- Contract-compliant verification proof for migrations, CORS, SLOs, docker runtime health, and final review wave.

### Definition of Done (verifiable conditions with commands)
- [ ] `go test -race -v ./internal/filter/... ./internal/planner/... ./internal/audit/...` passes.
- [ ] `./bin/pratc plan --repo=opencode-ai/opencode --target=20 --dry-run --format=json` exits `0` and emits dry-run semantics.
- [ ] `./bin/pratc audit --format=json` exits `0` with contract-valid audit entries.
- [ ] `cd web && bun run build` exits `0` (no settings import/type errors).
- [ ] `cd web && bun run test` passes including inbox/graph/settings behavior tests.
- [ ] `go test -race -v ./internal/cache/...` includes migration-path tests (fresh, N-1, N-2).
- [ ] `curl -s -H "Origin: http://localhost:3000" -I http://localhost:8080/api/health` includes expected CORS header.
- [ ] SLO evidence file exists with measured timings for analyze/cluster/graph/plan commands.
- [ ] Final F1-F4 verification tasks each produce explicit approval evidence artifacts.

### Must Have
- Resolve every outstanding item from the unresolved findings matrix defined in this remediation plan.
- Preserve v0.1 scope guardrails from `AGENTS.md` and archived `pratc.md`.
- Keep action execution disabled; log intents only.
- Keep API/backward compatibility where currently used (`/triage` compatibility, `/healthz` and `/api/health`).

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- Must not introduce v0.2-only features (OAuth/webhooks/multi-repo aggregation/auto execution).
- Must not duplicate planning algorithms between `internal/planning` and `internal/planner`.
- Must not mark any task complete without command output and evidence artifact path.
- Must not rely on human/manual verification statements.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: TDD (RED-GREEN-REFACTOR) for all newly introduced packages and contract tests.
- QA policy: Every task includes explicit happy-path + failure/edge-case scenarios.
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}` for every task, plus final consolidated audit artifacts.

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave; pull shared contracts into early waves.

Wave 1: Backend contract extraction
- Task 1 filter package extraction
- Task 2 planner orchestration package
- Task 3 audit package + CLI command

Wave 2: Safety and API contract closure
- Task 4 dry-run default semantics across CLI/API
- Task 5 `/plans` query param validation + deterministic defaults coverage
- Task 6 CORS contract tests
- Task 7 SQLite migration-path tests + schema fail-fast tests

Wave 3: Web contract closure
- Task 8 settings API/type export alignment
- Task 9 inbox route parity + triage compatibility
- Task 10 inbox table/action workflow compliance
- Task 11 interactive dependency graph with D3

Wave 4: Reliability/performance evidence
- Task 12 rate-limit/backoff contract regression tests
- Task 13 SLO benchmark harness + evidence capture
- Task 14 docker runtime/health profile verification evidence
- Task 17 bot PR detection (parallel with 12, 13, 14)
- Task 18 minimax 2.7 provider integration (replaces openrouter-light profile)

Wave 5: Process closure
- Task 15 evidence inventory reconciliation
- Task 16 docs/contract alignment update for v0.1 closure

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 2,4,5 | 1 |
| 2 | 1 | 4,5 | 1 |
| 3 | — | 4,15 | 1 |
| 4 | 2,3 | 10,15 | 2 |
| 5 | 2 | 16 | 2 |
| 6 | — | 15 | 2 |
| 7 | — | 12,15 | 2 |
| 8 | — | 9,10,11 | 3 |
| 9 | 8 | 10 | 3 |
| 10 | 3,4,8,9 | 15 | 3 |
| 11 | 8 | 15 | 3 |
| 12 | 7 | 13 | 4 |
| 13 | 12 | 15 | 4 |
| 14 | 18 | 15 | 4 |
| 15 | 3,4,6,7,10,11,13,14,17,18 | 16,F1-F4 | 5 |
| 16 | 5,15 | F1-F4 | 5 |
| 17 | — | 15 | 4 |
| 18 | — | 14,15 | 4 |
| F1-F4 | 16 | — | FINAL |

### Agent Dispatch Summary (wave -> task count -> categories)
- Wave 1 -> 3 tasks -> `deep`, `unspecified-high`
- Wave 2 -> 4 tasks -> `unspecified-high`, `quick`
- Wave 3 -> 3 tasks -> `visual-engineering`, `unspecified-high`
- Wave 4 -> 5 tasks -> `deep`, `quick`
- Wave 5 -> 2 tasks -> `writing`, `unspecified-high`
- FINAL -> 4 tasks -> `oracle`, `unspecified-high`, `deep`

## TODOs
> Implementation + Test = ONE task. Every task includes agent profile, references, acceptance criteria, and QA scenarios.

- [x] 1. Extract pre-filter pipeline into `internal/filter`

  **What to do**: Move filter stages currently in `internal/app/service.go` into `internal/filter/pipeline.go`, `internal/filter/filters.go`, `internal/filter/scorer.go`, `internal/filter/pool.go`; add package tests that characterize existing behavior first and then assert package behavior.
  **Must NOT do**: Do not change scoring semantics or candidate ordering relative to current `service.go` behavior.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: behavior-preserving extraction on critical planning path
  - Skills: [`superpowers/test-driven-development`] — enforce RED-GREEN-REFACTOR
  - Omitted: [`frontend-ui-ux`] — no UI impact

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 2,4,5 | Blocked By: none

  **References**:
  - Pattern: `internal/app/service.go` — current inline filtering/scoring behavior to preserve
  - API/Type: `internal/types/models.go` — response structures consuming filtered pool
  - Test: `internal/planning/pool_test.go` — expected deterministic pool behavior patterns

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/filter/...` passes.
  - [ ] Existing `./bin/pratc plan --repo=opencode-ai/opencode --target=20 --format=json` remains contract-valid.

  **QA Scenarios**:
  ```
  Scenario: Happy path filter extraction preserves behavior
    Tool: Bash
    Steps: Run package tests and compare plan JSON key set before/after extraction on same fixture repo.
    Expected: Tests pass and output contract keys unchanged.
    Evidence: .sisyphus/evidence/task-1-filter-happy.txt

  Scenario: Edge case empty candidate pool handling
    Tool: Bash
    Steps: Run targeted unit test with all PRs rejected by filters.
    Expected: Empty pool returned with deterministic rejection metadata (no panic).
    Evidence: .sisyphus/evidence/task-1-filter-empty.txt
  ```

  **Commit**: YES | Message: `refactor(filter): extract service prefilter pipeline` | Files: `internal/filter/*`, `internal/app/service.go`

- [x] 2. Add `internal/planner` orchestration package while retaining `internal/planning` algorithms

  **What to do**: Create `internal/planner/planner.go`, `plan.go`, `optimizer.go`, `validator.go` as orchestration boundary calling `internal/filter`, `internal/planning`, `internal/formula`, and `internal/graph`; migrate orchestration out of `internal/app/service.go`.
  **Must NOT do**: Do not duplicate algorithms already in `internal/planning`; no large rename from `planning` to `planner`.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: cross-package orchestration and contract stability
  - Skills: [`superpowers/test-driven-development`] — preserve deterministic outcomes
  - Omitted: [`superpowers/subagent-driven-development`] — single focused orchestration boundary task

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 4,5 | Blocked By: 1

  **References**:
  - Pattern: `internal/planning/pool.go` — existing candidate-selection behavior
  - Pattern: `internal/planning/hierarchy.go` — existing hierarchy algorithm usage
  - Pattern: `internal/app/service.go` — current orchestration to extract
  - Test: `internal/planning/*_test.go` — deterministic expectations

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/planner/...` passes.
  - [ ] `go test -race -v ./internal/planning/...` remains green.
  - [ ] `./bin/pratc plan --repo=opencode-ai/opencode --target=20 --format=json` exits `0` with required keys.

  **QA Scenarios**:
  ```
  Scenario: Happy path planner orchestration
    Tool: Bash
    Steps: Execute planner package tests, then CLI plan command with fixture repo.
    Expected: Package tests pass and CLI returns ordered/selected/rejections fields.
    Evidence: .sisyphus/evidence/task-2-planner-happy.txt

  Scenario: Failure path invalid planner input
    Tool: Bash
    Steps: Run unit test for invalid target/pool constraints.
    Expected: Validation error returned without panic.
    Evidence: .sisyphus/evidence/task-2-planner-invalid.txt
  ```

  **Commit**: YES | Message: `feat(planner): add orchestration layer over planning engines` | Files: `internal/planner/*`, `internal/app/service.go`

- [x] 3. Implement audit subsystem and `pratc audit` command

  **What to do**: Add `internal/audit/log.go` + storage wiring, create `audit_log` schema/migration in cache init path, add CLI command in `internal/cmd/root.go` and `internal/cmd/audit.go` that returns paginated JSON/table history.
  **Must NOT do**: No GitHub-side action execution; no auth layer additions.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: CLI + persistence + API-safe contract work
  - Skills: [`superpowers/test-driven-development`] — schema and command correctness
  - Omitted: [`frontend-ui-ux`] — backend/CLI task

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 4,15 | Blocked By: none

  **References**:
  - Pattern: `internal/cache/sqlite.go` — migration and table initialization patterns
  - Pattern: `internal/cmd/root.go` — command registration conventions
  - API/Type: `internal/types/models.go` — add/align audit response models

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/audit/...` passes.
  - [ ] `./bin/pratc audit --format=json` exits `0` with valid JSON entries.

  **QA Scenarios**:
  ```
  Scenario: Happy path audit read command
    Tool: Bash
    Steps: Seed audit entries via test helper, run `pratc audit --format=json`.
    Expected: Ordered entries with timestamp/action/repo fields.
    Evidence: .sisyphus/evidence/task-3-audit-happy.txt

  Scenario: Failure path malformed pagination flag
    Tool: Bash
    Steps: Run `pratc audit --limit=-1`.
    Expected: Exit code 2 with flag validation error.
    Evidence: .sisyphus/evidence/task-3-audit-invalid-flag.txt
  ```

  **Commit**: YES | Message: `feat(audit): add audit log storage and CLI query command` | Files: `internal/audit/*`, `internal/cache/*`, `internal/cmd/audit.go`, `internal/cmd/root.go`

- [x] 4. Enforce dry-run default semantics across action-capable flows

  **What to do**: Add/normalize `--dry-run` defaults for relevant CLI/API action-intent paths; ensure output explicitly states dry-run mode and routes through audit intent logging semantics.
  **Must NOT do**: No real GitHub mutations; no silent non-dry-run default.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: safety-critical contract behavior across interfaces
  - Skills: [`superpowers/test-driven-development`] — prevent regressions
  - Omitted: [`frontend-ui-ux`] — primarily backend/CLI

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: 10,15 | Blocked By: 2,3

  **References**:
  - Pattern: `internal/types/models.go` — existing `dry_run` model fields
  - Pattern: `internal/cmd/root.go` — command flag wiring conventions
  - Pattern: `AGENTS.md` — v0.1 safety and no-mutation guardrails

  **Acceptance Criteria**:
  - [ ] `./bin/pratc plan --repo=opencode-ai/opencode --target=20 --dry-run --format=json` exits `0` with explicit dry-run indication.
  - [ ] Action-capable flows default to dry-run unless explicitly overridden where allowed by v0.1 policy.

  **QA Scenarios**:
  ```
  Scenario: Happy path dry-run output behavior
    Tool: Bash
    Steps: Run plan/action-intent flows with defaults and explicit `--dry-run`.
    Expected: Responses indicate non-mutating behavior and log intent only.
    Evidence: .sisyphus/evidence/task-4-dryrun-happy.txt

  Scenario: Failure path forbidden execution mode in v0.1
    Tool: Bash
    Steps: Trigger execution-style flag/path disallowed by v0.1.
    Expected: Deterministic error explaining execution disabled in v0.1.
    Evidence: .sisyphus/evidence/task-4-dryrun-forbidden.txt
  ```

  **Commit**: YES | Message: `feat(safety): enforce dry-run defaults for v0.1 action flows` | Files: `internal/cmd/*`, `internal/app/*`, `internal/types/*`

- [x] 5. Harden `/plans` query-param contract and validation

  **What to do**: Ensure `/api/repos/:owner/:repo/plans` explicitly accepts, validates, and applies `cluster_id`, `exclude_conflicts`, `stale_score_threshold`, `candidate_pool_cap`, `score_min` with deterministic defaults and field-level 400 errors.
  **Must NOT do**: Do not silently coerce invalid params; do not bypass prefilter config construction.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: API validation and handler contract hardening
  - Skills: [] — direct handler test-driven work
  - Omitted: [`frontend-ui-ux`] — non-UI task

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 16 | Blocked By: 1,2

  **References**:
  - Pattern: `internal/cmd/root.go` — current `handlePlan` parameter parsing
  - Pattern: `.sisyphus/plans/archive/2026-03-22/pratc.md` — `/plans` query contract requirements
  - Test: `internal/cmd/*_test.go` — endpoint testing style

  **Acceptance Criteria**:
  - [ ] API accepts full query surface and returns deterministic defaults when omitted.
  - [ ] Invalid ranges produce `400` with parameter-specific error messages.

  **QA Scenarios**:
  ```
  Scenario: Happy path extended params accepted and applied
    Tool: Bash
    Steps: Curl `/plans` with all supported params and inspect pool/rejections fields.
    Expected: 200 response; candidate pool and rejection metadata reflect supplied params.
    Evidence: .sisyphus/evidence/task-5-plans-params-happy.json

  Scenario: Failure path invalid params rejected
    Tool: Bash
    Steps: Curl `/plans` with out-of-range threshold/cap/score values.
    Expected: 400 with per-field validation errors.
    Evidence: .sisyphus/evidence/task-5-plans-params-invalid.json
  ```

  **Commit**: YES | Message: `fix(api): enforce /plans query contract and validation` | Files: `internal/cmd/root.go`, `internal/cmd/*_test.go`

- [x] 6. Add CORS contract tests for dashboard origin

  **What to do**: Add tests validating origin/method/header behavior for `corsMiddleware` and endpoint responses, including preflight and negative-origin behavior.
  **Must NOT do**: Do not weaken CORS policy by broad wildcards.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: focused middleware contract test coverage
  - Skills: []
  - Omitted: [`superpowers/test-driven-development`] — simple incremental test addition

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 15 | Blocked By: none

  **References**:
  - Pattern: `internal/cmd/root.go` — `corsMiddleware` implementation
  - Test: `internal/cmd/*_test.go` — HTTP handler test patterns

  **Acceptance Criteria**:
  - [ ] Preflight test confirms expected CORS headers for `http://localhost:3000`.
  - [ ] Non-allowed origin case is handled as designed (documented and tested).

  **QA Scenarios**:
  ```
  Scenario: Happy path allowed origin headers
    Tool: Bash
    Steps: Run server tests and curl HEAD/OPTIONS with Origin localhost:3000.
    Expected: Access-Control-Allow-Origin present and correct.
    Evidence: .sisyphus/evidence/task-6-cors-happy.txt

  Scenario: Failure path disallowed origin
    Tool: Bash
    Steps: Send request with unexpected origin and assert policy behavior.
    Expected: Origin not allowed per middleware contract.
    Evidence: .sisyphus/evidence/task-6-cors-invalid-origin.txt
  ```

  **Commit**: YES | Message: `test(api): add CORS contract coverage` | Files: `internal/cmd/*_test.go`

- [x] 7. Add SQLite migration-path and schema-version fail-fast tests

  **What to do**: Add tests proving forward upgrade behavior from fresh, N-1, and N-2 schemas, and startup fail-fast behavior when on-disk schema is newer than binary-supported version.
  **Must NOT do**: No destructive down-migration logic.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: migration compatibility and startup safety correctness
  - Skills: [`superpowers/test-driven-development`]
  - Omitted: [`frontend-ui-ux`] — backend persistence task

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 12,15 | Blocked By: none

  **References**:
  - Pattern: `internal/cache/sqlite.go` — schema initialization and migration state
  - Test: `internal/cache/sqlite_test.go` — current test harness style
  - Contract: archived `pratc.md` migration policy section (now under `archive/2026-03-22/pratc.md`)

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/cache/...` includes and passes migration upgrade-path tests.
  - [ ] Future-schema startup test fails fast with explicit error.

  **QA Scenarios**:
  ```
  Scenario: Happy path migration upgrades
    Tool: Bash
    Steps: Run migration tests covering fresh, N-1, N-2 fixtures.
    Expected: All upgrade paths pass and user_version aligns to latest.
    Evidence: .sisyphus/evidence/task-7-migrations-happy.txt

  Scenario: Failure path unsupported future schema
    Tool: Bash
    Steps: Run test with DB user_version newer than supported binary version.
    Expected: Startup fails fast with clear compatibility error.
    Evidence: .sisyphus/evidence/task-7-migrations-future-version.txt
  ```

  **Commit**: YES | Message: `test(cache): cover migration paths and future-schema fail-fast` | Files: `internal/cache/sqlite_test.go`, migration fixtures

- [x] 8. Fix settings API/type exports and compile-path gaps

  **What to do**: Implement/export missing settings client functions and types in `web/src/lib/api.ts`, align `web/src/pages/settings.tsx` imports/usages, and add regression tests for settings API interactions.
  **Must NOT do**: Do not change settings UX scope beyond compile/runtime correctness.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: frontend API-client + page integration correctness
  - Skills: [`frontend-ui-ux`] — preserve existing interface behavior
  - Omitted: [`playwright`] — implementation task, not QA-only agent mode

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 9,10,11 | Blocked By: none

  **References**:
  - Pattern: `web/src/pages/settings.tsx` — current import and usage points
  - Pattern: `web/src/lib/api.ts` — existing fetch client patterns
  - Test: `web/src/pages/settings.test.tsx` — settings page test style

  **Acceptance Criteria**:
  - [ ] `cd web && bun run build` passes with zero settings import/type errors.
  - [ ] `cd web && bun run test` passes including settings tests.

  **QA Scenarios**:
  ```
  Scenario: Happy path settings compile and tests
    Tool: Bash
    Steps: Run web build and unit test commands.
    Expected: Build succeeds and settings-related tests pass.
    Evidence: .sisyphus/evidence/task-8-settings-happy.txt

  Scenario: Failure path invalid settings payload handling
    Tool: Bash
    Steps: Run unit test for malformed settings import/post path.
    Expected: Deterministic UI/client error handling without crash.
    Evidence: .sisyphus/evidence/task-8-settings-invalid.txt
  ```

  **Commit**: YES | Message: `fix(web): align settings api exports and compile contracts` | Files: `web/src/lib/api.ts`, `web/src/pages/settings.tsx`, tests

- [x] 9. Implement inbox route parity with `/triage` compatibility

  **What to do**: Add canonical `/inbox` page route and preserve existing `/triage` compatibility (redirect or shared component) so plan contract is met without breaking existing navigation paths.
  **Must NOT do**: Do not remove `/triage` without compatibility path and tests.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: route-level UX contract correction
  - Skills: [`frontend-ui-ux`]
  - Omitted: [`superpowers/test-driven-development`] — route parity is straightforward

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 10 | Blocked By: 8

  **References**:
  - Pattern: `web/src/pages/triage.tsx` — existing triage implementation
  - Pattern: `web/src/components/Navigation.tsx` — route navigation definitions
  - Test: `web/src/components/Navigation.test.tsx`

  **Acceptance Criteria**:
  - [ ] `/inbox` route renders triage experience.
  - [ ] `/triage` still resolves to equivalent content or deterministic redirect.

  **QA Scenarios**:
  ```
  Scenario: Happy path inbox route available
    Tool: Playwright
    Steps: Navigate to `/inbox`, verify triage table/pane content is visible.
    Expected: Inbox page loads with expected primary triage UI.
    Evidence: .sisyphus/evidence/task-9-inbox-happy.png

  Scenario: Compatibility path `/triage`
    Tool: Playwright
    Steps: Navigate to `/triage` and assert redirect/shared-content behavior.
    Expected: No 404; user reaches same triage experience.
    Evidence: .sisyphus/evidence/task-9-triage-compat.png
  ```

  **Commit**: YES | Message: `feat(web): add /inbox route with /triage compatibility` | Files: `web/src/pages/*`, navigation/tests

- [x] 10. Complete inbox table/action workflow contract

  **What to do**: Align inbox UX with required table + action workflow expectations, including deterministic action-intent wiring to audit path, scalable row rendering strategy, TanStack Table column definitions, and TanStack Virtual for 500+ row handling. Add `@tanstack/react-table` and `@tanstack/react-virtual` to `web/package.json`.
  **Must NOT do**: No real PR mutations; no scope expansion to saved views/auth/keyboard shortcuts.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: data-dense table workflow and action flow
  - Skills: [`frontend-ui-ux`]
  - Omitted: [`playwright`] — use for QA validation, not implementation behavior

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: 15 | Blocked By: 3,4,8,9

  **References**:
  - Pattern: `web/src/pages/triage.tsx` — current baseline table/actions
  - Pattern: `internal/audit/*` and CLI/API action-intent model
  - Test: `web/src/pages/settings.test.tsx`, navigation tests as style reference

  **Acceptance Criteria**:
  - [ ] `cd web && bun add @tanstack/react-table @tanstack/react-virtual` succeeds; `bun run build` passes.
  - [ ] Inbox table uses TanStack Table with 6 defined columns (number, title, author, cluster, CI status, age).
  - [ ] TanStack Virtual handles 500+ rows without visible lag in Playwright scroll test.
  - [ ] Inbox action controls emit intent-only events and surface deterministic feedback.
  - [ ] Web tests cover action path success and failure handling.

  **QA Scenarios**:
  ```
  Scenario: Happy path action intent flow
    Tool: Playwright
    Steps: Trigger approve/close/skip actions from inbox row.
    Expected: UI indicates logged intent; no mutation side effects.
    Evidence: .sisyphus/evidence/task-10-inbox-actions-happy.png

  Scenario: Failure path audit endpoint unavailable
    Tool: Playwright
    Steps: Simulate API failure for action intent endpoint.
    Expected: User sees deterministic error state and row state remains stable.
    Evidence: .sisyphus/evidence/task-10-inbox-actions-failure.png
  ```

  **Commit**: YES | Message: `feat(web): finalize inbox action workflow with intent logging` | Files: inbox page/components/tests

- [x] 11. Implement interactive dependency graph in web with D3

  **What to do**: Add D3 dependency in `web/package.json`, replace DOT-text rendering in `web/src/pages/graph.tsx` with interactive graph component (zoom/pan, node/edge legend, filter controls) backed by existing graph API payload.
  **Must NOT do**: Do not introduce 3D graph tooling or unrelated visualization frameworks.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: interactive data visualization implementation
  - Skills: [`frontend-ui-ux`]
  - Omitted: [`superpowers/test-driven-development`] — UI interaction focus with component/e2e tests

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 15 | Blocked By: 8

  **References**:
  - Pattern: `web/src/pages/graph.tsx` — current DOT rendering baseline
  - Pattern: `internal/graph/graph.go` — backend graph model semantics
  - External: `https://d3js.org/` — interaction and rendering APIs

  **Acceptance Criteria**:
  - [ ] `cd web && bun run build` passes with D3 dependency installed.
  - [ ] Graph page renders interactive nodes/edges from API payload.

  **QA Scenarios**:
  ```
  Scenario: Happy path graph interaction
    Tool: Playwright
    Steps: Open `/graph`, verify nodes/edges render, perform zoom and pan.
    Expected: Interaction works without runtime errors; graph remains visible.
    Evidence: .sisyphus/evidence/task-11-graph-happy.png

  Scenario: Failure path malformed graph payload
    Tool: Playwright
    Steps: Mock malformed payload response.
    Expected: Graph page shows controlled error/empty state, no crash.
    Evidence: .sisyphus/evidence/task-11-graph-invalid-data.png
  ```

  **Commit**: YES | Message: `feat(web): replace DOT view with interactive D3 dependency graph` | Files: `web/package.json`, graph page/components/tests

- [x] 12. Expand rate-limit/backoff regression coverage

  **What to do**: Extend GitHub client tests to explicitly validate retry ceilings, jitter/backoff behavior, reserve-budget pause behavior, and non-hanging failure semantics.
  **Must NOT do**: No production token/API calls in unit tests.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: reliability behavior under retry/rate limits
  - Skills: [`superpowers/test-driven-development`]
  - Omitted: [`frontend-ui-ux`] — backend reliability tests only

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: 13 | Blocked By: 7

  **References**:
  - Pattern: `internal/github/client_test.go` — existing rate-limit test scaffold
  - Pattern: `internal/github/client.go` — retry/backoff logic under test
  - Contract: archived `pratc.md` output/runtime policy

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/github/...` covers all rate-limit policy branches.
  - [ ] Tests assert no infinite retry/hang behavior.

  **QA Scenarios**:
  ```
  Scenario: Happy path backoff + eventual success
    Tool: Bash
    Steps: Run targeted retry tests with mocked transient failures.
    Expected: Retries occur within configured bounds and request succeeds.
    Evidence: .sisyphus/evidence/task-12-ratelimit-happy.txt

  Scenario: Failure path max retries exceeded
    Tool: Bash
    Steps: Run mocked persistent failure test.
    Expected: Deterministic terminal error after configured retry ceiling.
    Evidence: .sisyphus/evidence/task-12-ratelimit-failure.txt
  ```

  **Commit**: YES | Message: `test(github): harden rate-limit and retry contract coverage` | Files: `internal/github/client_test.go`

- [x] 13. Create SLO benchmark harness and evidence capture

  **What to do**: Add reproducible benchmark script/tests for analyze/cluster/graph/plan command runtimes and capture evidence against v0.1 SLO targets using frozen `fixtures/prs/` dataset with warm cache preconditions (`pratc analyze --repo=fixture/test` run first to populate cache).
  **Must NOT do**: Do not claim SLO PASS without measured command timings saved to evidence.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: cross-command performance verification and evidence discipline
  - Skills: []
  - Omitted: [`frontend-ui-ux`] — no UI changes

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: 15 | Blocked By: 12

  **References**:
  - Pattern: `Makefile` — command orchestration entrypoints
  - Contract: archived `pratc.md` runtime target section
  - Fixture: `fixtures/prs/*`, `fixtures/manifest.json`

  **Acceptance Criteria**:
  - [ ] Evidence file contains measured durations for analyze/cluster/graph/plan.
  - [ ] PASS/FAIL against each target threshold is explicitly recorded.

  **QA Scenarios**:
  ```
  Scenario: Happy path benchmark run
    Tool: Bash
    Steps: Execute benchmark harness against fixture repo with warm cache precondition.
    Expected: Timings captured for all four commands and persisted in evidence.
    Evidence: .sisyphus/evidence/task-13-slo-benchmarks.txt

  Scenario: Failure path benchmark timeout/overflow
    Tool: Bash
    Steps: Run harness with enforced timeout guard.
    Expected: Timeout failure is reported explicitly with command context.
    Evidence: .sisyphus/evidence/task-13-slo-timeout.txt
  ```

  **Commit**: YES | Message: `test(perf): add reproducible v0.1 slo benchmark harness` | Files: benchmark script/tests, evidence capture helper

- [x] 14. Verify docker runtime health for both documented profiles

  **What to do**: Execute compose lifecycle validation for `local-ml` and `minimax-light` profiles, validate service health endpoints and clean teardown behavior, store command outputs.
  **Must NOT do**: Do not replace profile definitions; only validate and patch misconfigurations required for health checks.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: infrastructure validation and evidence capture
  - Skills: []
  - Omitted: [`superpowers/test-driven-development`] — command-level verification task

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: 15 | Blocked By: 18

  **References**:
  - Pattern: `docker-compose.yml` — profile and healthcheck definitions
  - Pattern: `Dockerfile.cli`, `Dockerfile.web` — service runtime contracts

  **Acceptance Criteria**:
  - [ ] Both compose profiles start, report healthy services, and shut down cleanly.
  - [ ] Health endpoint checks are recorded for API and web.

  **QA Scenarios**:
  ```
  Scenario: Happy path local-ml profile
    Tool: Bash
    Steps: `docker compose --profile local-ml up -d`, check health, curl endpoints, `docker compose down`.
    Expected: Services healthy and reachable; clean shutdown.
    Evidence: .sisyphus/evidence/task-14-docker-local-ml.txt

  Scenario: Failure path minimax-light misconfig
    Tool: Bash
    Steps: Start profile with `MINIMAX_API_KEY=""` (empty string) to simulate missing/invalid credentials.
    Expected: Clear startup/configuration error without hanging partial state.
    Evidence: .sisyphus/evidence/task-14-docker-minimax-failure.txt
  ```

  **Commit**: YES | Message: `test(docker): verify compose profile runtime health contracts` | Files: optional compose/test scripts + evidence artifacts

- [x] 15. Reconcile evidence inventory and close task-level proof gaps

  **What to do**: Build evidence index for Tasks 1-18 and ensure every task has happy/failure artifacts with deterministic naming; fill missing evidence artifacts by re-running agent-executable checks.
  **Must NOT do**: Do not mark tasks complete based on narrative-only claims.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: artifact curation + verification bookkeeping
  - Skills: []
  - Omitted: [`frontend-ui-ux`] — non-UI task

  **Parallelization**: Can Parallel: NO | Wave 5 | Blocks: 16,F1-F4 | Blocked By: 3,4,6,7,10,11,13,14,17,18

  **References**:
  - Pattern: `.sisyphus/evidence/` — existing artifact structure
  - Pattern: this plan's Task 1-18 matrix — unresolved items checklist

  **Acceptance Criteria**:
  - [ ] Every task (1-18) in this plan has both happy and failure evidence files.
  - [ ] Evidence index document maps task -> commands -> artifact paths (Tasks 1-18).

  **QA Scenarios**:
  ```
  Scenario: Happy path evidence completeness
    Tool: Bash
    Steps: Run evidence index validation script against expected task artifact matrix.
    Expected: Zero missing mandatory artifacts.
    Evidence: .sisyphus/evidence/task-15-evidence-complete.txt

  Scenario: Failure path missing artifact detection
    Tool: Bash
    Steps: Simulate missing artifact entry and run validator.
    Expected: Validator flags exact missing task artifact path.
    Evidence: .sisyphus/evidence/task-15-evidence-missing-detected.txt
  ```

  **Commit**: YES | Message: `chore(qa): reconcile task evidence inventory for v0.1 closure` | Files: evidence index + validator script/artifacts

- [x] 16. Align v0.1 docs/contracts with completed behavior and routes

  **What to do**: Update v0.1-facing docs and contract references (health endpoint usage, inbox route parity note, `/plans` parameter surface, dry-run/audit behavior) to match implemented behavior after remediation.
  **Must NOT do**: Do not introduce new feature promises beyond implemented v0.1 scope.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: contract/doc consistency finalization
  - Skills: []
  - Omitted: [`superpowers/test-driven-development`] — documentation alignment task

  **Parallelization**: Can Parallel: NO | Wave 5 | Blocks: F1-F4 | Blocked By: 5,15

  **References**:
  - Pattern: `README.md` — current user-facing behavior docs
  - Pattern: this plan's Definition of Done + Success Criteria — source checklist
  - Pattern: `.sisyphus/plans/archive/2026-03-22/pratc.md` — original v0.1 contract text

  **Acceptance Criteria**:
  - [ ] Route/endpoint/flag docs match implemented behavior exactly.
  - [ ] No unresolved contradictions remain between docs and executable behavior.

  **QA Scenarios**:
  ```
  Scenario: Happy path docs-command consistency
    Tool: Bash
    Steps: Execute documented command examples and verify expected outputs.
    Expected: Commands run as documented without contradiction.
    Evidence: .sisyphus/evidence/task-16-docs-consistency.txt

  Scenario: Failure path stale doc reference detection
    Tool: Bash
    Steps: Run link/path checker over referenced internal paths and commands.
    Expected: Stale/missing references are surfaced with exact doc location.
    Evidence: .sisyphus/evidence/task-16-docs-stale-reference.txt
  ```

  **Commit**: YES | Message: `docs(v0.1): align contracts and usage with remediation outcomes` | Files: `README.md`, relevant v0.1 contract docs

- [x] 17. Implement bot PR detection and auto-clustering

  **What to do**: Add `internal/analysis/bots.go` with author pattern matching for `dependabot[bot]`, `renovate[bot]`, `github-actions[bot]`, `snyk-bot`, and title pattern matching for `^Bump `, `^chore\(deps\)`, `^Update dependency`; tag bot PRs with `is_bot: true` and cluster them into a "Dependency Updates" group separate from feature PRs; exclude bot PRs from merge plans by default with `--include-bots` flag to override.
  **Must NOT do**: No custom bot detection rules beyond hardcoded v0.1 patterns.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: analysis pipeline stage feeding into filter
  - Skills: []
  - Omitted: [`frontend-ui-ux`] — no UI changes

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: 15 | Blocked By: none

  **References**:
  - Pattern: `internal/app/service.go` — where bot tagging would be inserted into analysis pipeline
  - Pattern: `internal/filter/filters.go` — where bot exclusion filter would be applied
  - API/Type: `internal/types/models.go` — `IsBot` field on PR model
  - Contract: archived `pratc.md` Task 27 bot detection requirements

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/analysis/...` passes with bot detection test cases.
  - [ ] Bot PRs (dependabot, renovate) are tagged `is_bot: true` during analysis phase.
  - [ ] `./bin/pratc plan --repo=fixture/test --target=20 --format=json | jq '[.rejections[] | select(.reason == "bot pr")] | length'` returns >0 (bots rejected by default).
  - [ ] `./bin/pratc plan --repo=fixture/test --target=20 --include-bots --format=json | jq '[.rejections[] | select(.reason == "bot pr")] | length'` returns 0 when bots are present.

  **QA Scenarios**:
  ```
  Scenario: Happy path bot PRs auto-clustered and excluded from plan
    Tool: Bash
    Steps: Run analysis on fixture repo with known bot PRs, inspect cluster output and plan output.
    Expected: Bot PRs in separate cluster; plan rejects them with "bot pr" reason by default.
    Evidence: .sisyphus/evidence/task-17-bot-excluded.txt

  Scenario: Failure path --include-bots flag includes bots in plan
    Tool: Bash
    Steps: Run plan with --include-bots flag and assert bot PRs appear in selected/ordering.
    Expected: Bot PRs included when flag is set; no "bot pr" rejections.
    Evidence: .sisyphus/evidence/task-17-bot-included.txt
  ```

  **Commit**: YES | Message: `feat(analysis): add bot PR detection and auto-clustering` | Files: `internal/analysis/bots.go`, `internal/analysis/bots_test.go`, `internal/app/service.go`, `internal/filter/filters.go`

- [x] 18. Integrate Minimax 2.7 provider and replace openrouter-light profile

  **What to do**: Add Minimax 2.7 support to ML service by updating `ml-service/src/pratc_ml/providers/__init__.py` to include `minimax` backend option with `MINIMAX_API_KEY`, `MINIMAX_EMBED_MODEL` ("abab6.5t"), and `MINIMAX_REASON_MODEL` ("abab6.5t"); update Docker Compose to rename `openrouter-light` profile to `minimax-light`; update environment variable references throughout codebase from OpenRouter to Minimax.
  **Must NOT do**: Do not break existing local-ml or voyage backends; maintain backward compatibility for local development.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: cross-cutting ML provider integration affecting multiple subsystems
  - Skills: []
  - Omitted: [`frontend-ui-ux`] — backend-only task

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: 14,15 | Blocked By: none

  **References**:
  - Pattern: `ml-service/src/pratc_ml/providers/__init__.py` — current OpenRouter/Voyage provider structure
  - Pattern: `docker-compose.yml` — current profile definitions
  - Pattern: `AGENTS.md` — execution contract requirements
  - External: Minimax 2.7 API documentation (abab6.5t model)

  **Acceptance Criteria**:
  - [ ] `ML_BACKEND=minimax MINIMAX_API_KEY="valid-key" uv run pytest -v` passes all ML tests
  - [ ] `docker compose --profile minimax-light config` validates successfully
  - [ ] All environment variable references updated from OpenRouter to Minimax
  - [ ] Existing local-ml and voyage backends remain functional

  **QA Scenarios**:
  ```
  Scenario: Happy path minimax provider configuration
    Tool: Bash
    Steps: Set MINIMAX_API_KEY env var, run ML service health check
    Expected: Service starts and reports healthy with minimax backend
    Evidence: .sisyphus/evidence/task-18-minimax-happy.txt

  Scenario: Failure path missing minimax API key
    Tool: Bash
    Steps: Set ML_BACKEND=minimax without MINIMAX_API_KEY, run ML service
    Expected: Clear error message about missing API key, service exits cleanly
    Evidence: .sisyphus/evidence/task-18-minimax-missing-key.txt
  ```

  **Commit**: YES | Message: `feat(ml): add minimax 2.7 provider and minimax-light docker profile` | Files: `ml-service/src/pratc_ml/providers/__init__.py`, `docker-compose.yml`, env references

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> Do NOT auto-proceed after verification.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

  **QA Scenarios**:
  ```
  Scenario: F1 plan compliance audit
    Tool: task(subagent_type="oracle")
    Steps: Review the finalized implementation and evidence against every TODO requirement in this plan.
    Expected: Oracle returns APPROVE with no missing mandatory deliverables.
    Evidence: .sisyphus/evidence/final-f1-plan-compliance.md

  Scenario: F2 code quality review
    Tool: Bash + task(category="unspecified-high")
    Steps: Run `go vet ./...`, `go test -race ./...`, `uv run pytest -v`, `bun run test`, and static-pattern checks for forbidden anti-patterns.
    Expected: All checks pass; reviewer returns APPROVE with zero blocking quality issues.
    Evidence: .sisyphus/evidence/final-f2-code-quality.txt

  Scenario: F3 end-to-end QA sweep
    Tool: Playwright + Bash + task(category="unspecified-high")
    Steps: Execute representative CLI/API/web happy and failure flows from Tasks 1-16, capture screenshots/logs/output files.
    Expected: Scenario matrix passes with explicit pass/fail counts and no unresolved blockers.
    Evidence: .sisyphus/evidence/final-f3-e2e-qa.md

  Scenario: F4 scope fidelity check
    Tool: task(subagent_type="deep")
    Steps: Diff implemented behavior against this plan's scope boundaries and must-not-have guardrails.
    Expected: Deep reviewer returns APPROVE and reports no scope creep.
    Evidence: .sisyphus/evidence/final-f4-scope-fidelity.md
  ```

## Commit Strategy
- One task bundle per commit message in conventional format: `type(scope): desc`.
- No `--no-verify`; no amend unless explicitly required by hook behavior and local-head rules.

## Success Criteria
- All unresolved audit items are closed with evidence-backed PASS status.
- `make build`, `make test`, `go test -race -v ./...`, `uv run pytest -v`, `bun run test` all pass on `main` after merge.
- v0.1 scope guardrails remain intact.
