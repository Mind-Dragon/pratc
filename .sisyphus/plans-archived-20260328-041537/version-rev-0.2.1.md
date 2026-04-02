# Version Rev 0.2.1 — VPN Entry, Runtime Unification, PDF Download Fix

## TL;DR
> **Summary**: Ship `0.2.1` as a stabilization patch that fixes VPN-only dashboard access, removes runtime port/bind drift, and replaces the broken `/reports` flow with a direct PDF download endpoint.
> **Deliverables**:
> - Single user entrypoint over VPN at `http://100.112.201.95:7788`
> - Unified runtime contract across Go/web/docker/docs
> - Same-origin `/api/*` bridge on the dashboard host
> - Direct report PDF endpoint + wired dashboard CTA
> - Version bump and release notes for `0.2.1`
> **Effort**: Medium
> **Parallel**: YES - 3 waves
> **Critical Path**: 1 → 2 → 4 → 6 → 7 → 11 → 12 → 13

## Context
### Original Request
User requested a version-rev update plan after repeated runtime/access issues: remote VPN dashboard access, strict no-public-exposure requirement, broken PDF button (404), and usability confusion.

### Interview Summary
- Access model selected: **single VPN entrypoint** (recommended path accepted).
- Report fix selected: **direct PDF endpoint** (no `/reports` page in this rev).
- Release target selected: **patch rev `0.2.1`** (“do all fixes in-scope”).

### Metis Review (gaps addressed)
- Topology must be decided first to avoid rework.
- Guard against config drift across Go/web/docker/docs.
- Add explicit regression gates for bind policy, CORS behavior, and PDF download headers.
- Keep scope to hardening + report download wiring; avoid auth/TLS/platform redesign.

## Work Objectives
### Core Objective
Deliver a safe, deterministic `0.2.1` patch where the dashboard is reachable via VPN-only entrypoint, API integration is consistent, and the report download action works end-to-end.

### Deliverables
- Binding/CORS/runtime contract hardened and tested.
- Port/base-URL consistency across code and deployment assets.
- Backend PDF endpoint implemented and consumable from dashboard button.
- User-facing messaging clarified for sync/report states.
- Version and release docs updated to `0.2.1`.

### Definition of Done (verifiable conditions with commands)
- `go test ./internal/cmd/...` passes.
- `bun run test` (in `web/`) passes.
- `bun run build` (in `web/`) passes.
- `curl -sSf http://100.112.201.95:7788/ | grep -q "prATC Dashboard"` succeeds.
- `curl -sSf http://100.112.201.95:7788/api/health` succeeds through the dashboard-host bridge.
- `curl -sSI http://100.112.201.95:7788/api/repos/opencode-ai/opencode/report.pdf | grep -qi "Content-Type: application/pdf"` succeeds.
- `curl -sSI -H "Origin: http://evil.com" http://100.112.201.95:7788/api/health | grep -qi "Access-Control-Allow-Origin"` returns no match.
- `grep -n "0.2.1" CHANGELOG.md` and all version surfaces expected for rev are present.

### Must Have
- VPN-only operational behavior for user entrypoint.
- No accidental public-facing bind expansion.
- Dashboard host serves `/api/*` through a repo-local rewrite bridge (no external proxy redesign).
- No `/reports` 404 from dashboard CTA.
- Deterministic PDF response contract (`200`, `application/pdf`, filename disposition).
- No stale port guidance in runtime UX/docs for this rev.
- No release-diff scope drift from planning bookkeeping unless explicitly excluded.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- No OAuth/auth-system rollout in this patch.
- No TLS/proxy/platform redesign (nginx/caddy/systemd) in this patch.
- No unrelated settings API redesign.
- No broad CORS wildcard policy.
- No splitting into multiple plans.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: tests-after (hardening and endpoint verification after implementation tasks)
- QA policy: Every task includes happy + failure/edge scenarios with exact commands
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
Wave 1: Contract and topology foundation (tasks 1-3)
Wave 2: Backend/Frontend implementation wiring (tasks 4-7)
Wave 3: Docs/version/release validation (tasks 8-10)
Wave 4: Remediation closure after reviewer rejects (tasks 11-13)

### Dependency Matrix (full, all tasks)
- 1 blocks 2,3,4,5.
- 2 and 3 block 6 and 7.
- 4 and 5 block 6.
- 6 and 7 block 8 and 9.
- 8 and 9 block 10.
- 10 blocks 11, 12, and 13.
- 11 blocks 13.
- 12 blocks 13.

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1: 3 tasks → deep, unspecified-high
- Wave 2: 4 tasks → unspecified-high, visual-engineering
- Wave 3: 3 tasks → writing, unspecified-high
- Wave 4: 3 tasks → unspecified-high, writing, deep

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [x] 1. Lock 0.2.1 runtime topology contract (single VPN entrypoint)

  **What to do**: Define and codify the authoritative runtime contract: user entrypoint `100.112.201.95:7788`, internal API behavior, and config precedence (flags > env > defaults).
  **Must NOT do**: Do not introduce TLS/auth/proxy stack changes in this task.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: cross-cutting contract decision that drives all downstream tasks.
  - Skills: []
  - Omitted: [`subagent-driven-development`] — this is contract codification, not broad execution.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 2,3,4,5 | Blocked By: none

  **References**:
  - Pattern: `internal/cmd/root.go:417-430` — `serve` command port defaults/flags.
  - Pattern: `internal/cmd/root.go:559-567` — server bind location.
  - Pattern: `web/package.json:5-9` — dashboard runtime ports.
  - Pattern: `docker-compose.yml` — current container networking contract.

  **Acceptance Criteria**:
  - [ ] Contract section added to release docs/evidence with explicit host/port rules.
  - [ ] Config precedence is stated and reflected in implementation tasks.
  - [ ] No contradictory port/bind assumptions remain in plan scope files.

  **QA Scenarios**:
  ```
  Scenario: Happy path contract validation
    Tool: Bash
    Steps: grep for canonical contract text and expected host/port in updated release docs
    Expected: exactly one canonical entrypoint definition found
    Evidence: .sisyphus/evidence/task-1-topology-contract.txt

  Scenario: Drift detection
    Tool: Bash
    Steps: search for conflicting host/port statements (3000/8080/7788 mismatches)
    Expected: no conflicting active guidance in scoped files
    Evidence: .sisyphus/evidence/task-1-topology-drift.txt
  ```

  **Commit**: YES | Message: `docs(runtime): define 0.2.1 topology contract` | Files: scoped runtime contract docs/evidence

- [x] 2. Harden Go bind policy for VPN-safe serving

  **What to do**: Update server bind behavior in `serve` flow to satisfy the chosen topology and prevent accidental public exposure.
  **Must NOT do**: Do not bind to `0.0.0.0` without explicit policy validation checks.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: security-sensitive runtime behavior with regression risk.
  - Skills: []
  - Omitted: [`writing-skills`] — code/task hardening, not skill authoring.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 6,9 | Blocked By: 1

  **References**:
  - Pattern: `internal/cmd/root.go:559-567` — current `http.Server` bind and ListenAndServe.
  - Pattern: `internal/cmd/root.go:605-609` — env-backed defaults pattern in file.
  - Test: `internal/cmd/*_test.go` — command runtime behavior tests.

  **Acceptance Criteria**:
  - [ ] Bind behavior matches topology contract in task 1.
  - [ ] Boot fails with explicit error for disallowed bind targets (if policy requires).
  - [ ] `go test ./internal/cmd/...` passes after changes.

  **QA Scenarios**:
  ```
  Scenario: Happy path bind
    Tool: Bash
    Steps: start server with approved bind settings; query /healthz
    Expected: health endpoint returns 200 JSON status
    Evidence: .sisyphus/evidence/task-2-bind-happy.txt

  Scenario: Disallowed bind target
    Tool: Bash
    Steps: start server with forbidden bind target
    Expected: process exits non-zero with explicit policy error
    Evidence: .sisyphus/evidence/task-2-bind-reject.txt
  ```

  **Commit**: YES | Message: `feat(serve): enforce VPN-safe bind policy` | Files: `internal/cmd/root.go`, related tests

- [x] 3. Replace brittle CORS heuristic with strict origin validation

  **What to do**: Refactor CORS logic to use explicit parsed origin allowlist matching the topology contract; update tests accordingly.
  **Must NOT do**: Do not use wildcard ACAO with credentials enabled.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: security correctness + test parity.
  - Skills: []
  - Omitted: [`artistry`] — conventional hardening problem.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 9 | Blocked By: 1

  **References**:
  - Pattern: `internal/cmd/root.go:1109-1126` — current CORS middleware.
  - Test: `internal/cmd/cors_test.go:27-170` — stale fixed-origin expectations.

  **Acceptance Criteria**:
  - [ ] Allowed origins receive correct ACAO.
  - [ ] Disallowed origins receive no ACAO.
  - [ ] Requests without Origin do not produce malformed CORS headers.
  - [ ] CORS tests updated and passing.

  **QA Scenarios**:
  ```
  Scenario: Allowed origin
    Tool: Bash
    Steps: curl with Origin=http://100.112.201.95:7788 against /api/health
    Expected: response contains matching Access-Control-Allow-Origin
    Evidence: .sisyphus/evidence/task-3-cors-allow.txt

  Scenario: Evil origin
    Tool: Bash
    Steps: curl with Origin=http://evil.com against /api/health
    Expected: no Access-Control-Allow-Origin header present
    Evidence: .sisyphus/evidence/task-3-cors-deny.txt
  ```

  **Commit**: YES | Message: `test(cors): codify strict allowlist behavior` | Files: `internal/cmd/root.go`, `internal/cmd/cors_test.go`

- [x] 4. Unify web API base contract across dashboard code paths

  **What to do**: Centralize URL-building in `web/src/lib/api.ts` as the single source of truth, then make `web/src/components/SyncStatusPanel.tsx` import/reuse it. Refactor `fetchOmniPlan()` to use the same shared repo-path helper (no inline repo URL construction).
  **Must NOT do**: Do not leave duplicated hardcoded host/port literals with drift risk.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: frontend runtime behavior and UX consistency.
  - Skills: []
  - Omitted: [`test-driven-development`] — tests still required, but task is integration-focused.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 7,9 | Blocked By: 1,2

  **References**:
  - Pattern: `web/src/lib/api.ts:39-45` — API base fallback.
  - Pattern: `web/src/components/SyncStatusPanel.tsx:45-51` — duplicate API base fallback.
  - Pattern: `web/src/lib/api.ts:83-96` — current inline Omni path construction that must be de-duplicated.
  - Pattern: `web/src/pages/index.tsx` — user-facing API guidance string.
  - Test: `web/src/__tests__/lib/api.test.ts` — add exact URL composition assertions.
  - Test: `web/src/components/SyncStatusPanel.test.tsx` — add exact fetch/SSE URL assertions and no-duplicate-path guard.

  **Acceptance Criteria**:
  - [ ] `web/src/lib/api.ts` exports canonical URL helpers; SyncStatusPanel no longer declares local `apiBaseUrl()`/`repoPath()`.
  - [ ] `fetchOmniPlan()` uses shared repo-path helper (no inline `/api/repos/${...}` construction).
  - [ ] Exact URL regression tests exist for API client + Sync panel paths (not substring-only checks).
  - [ ] Regression guard verifies no generated request URL includes duplicated `/api/api/` segment.
  - [ ] `bun run test -- src/__tests__/lib/api.test.ts src/components/SyncStatusPanel.test.tsx` passes.

  **QA Scenarios**:
  ```
  Scenario: Happy path exact URL contract
    Tool: Bash
    Steps: run `bun run test -- src/__tests__/lib/api.test.ts src/components/SyncStatusPanel.test.tsx`
    Expected: tests assert exact composed URLs for sync/status/stats/sync-stream and API helper endpoints
    Evidence: .sisyphus/evidence/task-4-web-api-base.txt

  Scenario: Duplicate-path regression guard
    Tool: Bash
    Steps: run targeted web tests that explicitly fail on `/api/api/` URL composition
    Expected: no generated URL contains `/api/api/`; failure path assertion remains covered
    Evidence: .sisyphus/evidence/task-4-web-api-failure.txt
  ```

  **Commit**: YES | Message: `feat(web): unify API base resolution` | Files: `web/src/lib/api.ts`, `web/src/components/SyncStatusPanel.tsx`, related tests

- [x] 5. Align container/runtime wiring with canonical ports and entrypoint model

  **What to do**: Update `web/package.json`, `Dockerfile.web`, and `docker-compose.yml` so runtime wiring matches the chosen single-entrypoint model.
  **Must NOT do**: Do not leave conflicting dev/prod container port assumptions.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: deployment/runtime consistency and regression risk.
  - Skills: []
  - Omitted: [`openclaw-config`] — repository-local runtime files only.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 9 | Blocked By: 1,2

  **References**:
  - Pattern: `web/package.json:5-9` — script port defaults.
  - Pattern: `Dockerfile.web` — container command/port assumptions.
  - Pattern: `docker-compose.yml` — service port maps + env wiring.

  **Acceptance Criteria**:
  - [ ] Dev/prod scripts and container ports are consistent.
  - [ ] Compose wiring matches effective application runtime.
  - [ ] `docker compose config` validates without warnings relevant to changed services.

  **QA Scenarios**:
  ```
  Scenario: Happy path compose config
    Tool: Bash
    Steps: run docker compose config validation
    Expected: config parses successfully and reflects canonical ports
    Evidence: .sisyphus/evidence/task-5-compose-config.txt

  Scenario: Port drift regression check
    Tool: Bash
    Steps: grep runtime files for contradictory 3000/8080/7788 guidance
    Expected: no contradiction in scoped runtime files
    Evidence: .sisyphus/evidence/task-5-port-drift.txt
  ```

  **Commit**: YES | Message: `chore(runtime): align docker and script port wiring` | Files: `web/package.json`, `Dockerfile.web`, `docker-compose.yml`

- [x] 6. Add direct backend PDF export endpoint

  **What to do**: Implement HTTP endpoint for report PDF download using `internal/report/pdf.go` generation utilities and register route in serve handlers.
  **Must NOT do**: Do not add a web `/reports` page in this rev.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: backend API contract + binary response handling.
  - Skills: []
  - Omitted: [`artistry`] — conventional endpoint implementation.

  **Parallelization**: Can Parallel: NO | Wave 2 | Blocks: 7,9 | Blocked By: 2,4

  **References**:
  - Pattern: `internal/cmd/root.go:521-557` — route registration pattern.
  - Pattern: `internal/cmd/root.go` helpers (`writeHTTPJSON`, `writeHTTPError`) — response conventions.
  - Pattern: `internal/report/pdf.go:374-389` — `NewPDFExporter`/`Export`.
  - Test: `internal/report/pdf_test.go` — PDF generation behavior baseline.

  **Acceptance Criteria**:
  - [ ] Endpoint exists at agreed route (e.g., `/api/repos/{owner}/{repo}/report.pdf`).
  - [ ] Returns `200` with `Content-Type: application/pdf` and deterministic filename disposition.
  - [ ] Failure cases return controlled errors (missing data, invalid repo).

  **QA Scenarios**:
  ```
  Scenario: Happy path PDF download
    Tool: Bash
    Steps: curl endpoint and inspect status/headers/body signature
    Expected: 200 + application/pdf + non-empty payload
    Evidence: .sisyphus/evidence/task-6-pdf-download.txt

  Scenario: Invalid repo request
    Tool: Bash
    Steps: call PDF endpoint with invalid/missing repo path
    Expected: structured error response with non-200 status
    Evidence: .sisyphus/evidence/task-6-pdf-invalid.txt
  ```

  **Commit**: YES | Message: `feat(report): add direct PDF export endpoint` | Files: `internal/cmd/root.go`, report handler files, tests

- [x] 7. Rewire dashboard "Download Report" CTA to direct PDF endpoint

  **What to do**: Replace `/reports` navigation with direct endpoint download action using current repo context.
  **Must NOT do**: Do not keep dead-link fallback to nonexistent `/reports` route.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: user-visible UX correctness and route behavior.
  - Skills: []
  - Omitted: [`apple-hig`] — not platform-native design work.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 9 | Blocked By: 4,6

  **References**:
  - Pattern: `web/src/components/SyncStatusPanel.tsx:280-296` — current broken button behavior.
  - Pattern: `web/src/lib/api.ts` — repo path construction conventions.
  - Pattern: `web/src/pages/index.tsx` — user-facing messaging behavior.

  **Acceptance Criteria**:
  - [ ] Clicking Download Report triggers PDF download (no 404 page navigation).
  - [ ] URL includes correct repo context.
  - [ ] Error state shows clear message when endpoint unavailable.

  **QA Scenarios**:
  ```
  Scenario: Happy path CTA
    Tool: Playwright
    Steps: open dashboard, click Download Report, capture response
    Expected: file download or PDF response with 200 status
    Evidence: .sisyphus/evidence/task-7-cta-download.png

  Scenario: Endpoint unavailable
    Tool: Playwright
    Steps: stop API, click Download Report
    Expected: user sees explicit actionable failure message
    Evidence: .sisyphus/evidence/task-7-cta-failure.png
  ```

  **Commit**: YES | Message: `fix(web): wire report CTA to direct PDF endpoint` | Files: `web/src/components/SyncStatusPanel.tsx`, related tests

- [x] 8. Normalize user-facing operational guidance in dashboard copy

  **What to do**: Update stale messages that currently instruct wrong ports/routes and improve “as a human” clarity for sync/report states.
  **Must NOT do**: Do not redesign the dashboard IA or add new product features.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: concise UX messaging clarity and consistency.
  - Skills: []
  - Omitted: [`brainstorming`] — scope is bounded copy normalization.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 10 | Blocked By: 6,7

  **References**:
  - Pattern: `web/src/pages/index.tsx` — API unavailable guidance text.
  - Pattern: `web/src/components/TriageView.tsx:252-257` — triage unavailable guidance.
  - Pattern: `web/src/components/SyncStatusPanel.tsx` — sync/report state text.

  **Acceptance Criteria**:
  - [ ] No stale guidance references invalid route/port.
  - [ ] States are understandable to non-technical user (syncing vs unavailable vs error).
  - [ ] Web tests/snapshots updated and passing.

  **QA Scenarios**:
  ```
  Scenario: Happy path messaging
    Tool: Playwright
    Steps: load dashboard during sync-in-progress state
    Expected: clear non-technical status copy shown
    Evidence: .sisyphus/evidence/task-8-ux-sync.png

  Scenario: API unavailable messaging
    Tool: Playwright
    Steps: stop API and load dashboard
    Expected: clear actionable fallback message with correct command/port
    Evidence: .sisyphus/evidence/task-8-ux-unavailable.png
  ```

  **Commit**: YES | Message: `docs(web): clarify dashboard operational states` | Files: affected web copy files/tests

- [x] 9. Add cross-surface regression checks and evidence bundle

  **What to do**: Execute a deterministic verification suite covering bind policy, CORS behavior, dashboard reachability, PDF endpoint headers, and Task-4 URL-composition regression checks.
  **Must NOT do**: Do not mark complete without saved command outputs.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: final integration-level validation.
  - Skills: []
  - Omitted: [`verification-before-completion`] — process is embedded explicitly here.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: 10 | Blocked By: 2,3,4,5,6,7

  **References**:
  - Pattern: `internal/cmd/root.go` — bind/CORS behavior.
  - Pattern: `web/src/lib/api.ts` + `SyncStatusPanel.tsx` — API base + report CTA wiring.
  - Pattern: `docker-compose.yml`, `Dockerfile.web`, `README.md` — runtime/documentation consistency.

  **Acceptance Criteria**:
  - [ ] Verification script outputs saved under `.sisyphus/evidence/task-9-*`.
  - [ ] All required checks pass: health, CORS allow/deny, dashboard 200, PDF headers.
  - [ ] Targeted URL-contract tests pass and prove no `/api/api/` duplication in web API composition paths.
  - [ ] No unresolved contradictions in runtime-facing docs.

  **QA Scenarios**:
  ```
  Scenario: Happy path full verification
    Tool: Bash
    Steps: run full check suite command set including `go test ./internal/cmd/...`, `bun run test`, `bun run build`, dashboard curl checks, and PDF header checks
    Expected: all checks PASS with explicit status lines and saved command output
    Evidence: .sisyphus/evidence/task-9-full-suite.txt

  Scenario: Negative CORS check
    Tool: Bash
    Steps: send disallowed origin request
    Expected: denied CORS behavior verified
    Evidence: .sisyphus/evidence/task-9-cors-negative.txt

  Scenario: URL-duplication regression check
    Tool: Bash
    Steps: run `bun run test -- src/__tests__/lib/api.test.ts src/components/SyncStatusPanel.test.tsx` and inspect for duplicate `/api/api/` guard failures
    Expected: targeted tests pass and no duplicate-path failure is emitted
    Evidence: .sisyphus/evidence/task-9-url-regression.txt
  ```

  **Commit**: YES | Message: `test(release): add 0.2.1 regression evidence bundle` | Files: `.sisyphus/evidence/task-9-*`

- [x] 10. Bump and reconcile version surfaces to `0.2.1`

  **What to do**: Update all version-bearing surfaces and release notes to `0.2.1` consistent with shipped behavior.
  **Must NOT do**: Do not bump beyond patch level or leave mixed versions.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: release metadata/documentation precision.
  - Skills: []
  - Omitted: [`finishing-a-development-branch`] — this is release-surface update, not merge strategy.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: Final Verification Wave | Blocked By: 8,9

  **References**:
  - Pattern: `CHANGELOG.md` — release entry.
  - Pattern: `internal/app/service.go:25-29` — version constant surface.
  - Pattern: `web/package.json:4` — web version surface.
  - Pattern: `README.md` — runtime command docs.

  **Acceptance Criteria**:
  - [ ] All planned version surfaces read `0.2.1`.
  - [ ] Changelog entry describes this rev scope (VPN entrypoint + PDF fix + runtime unification).
  - [ ] No stale `0.1.0`/`0.2.0` remnants in scoped release surfaces.

  **QA Scenarios**:
  ```
  Scenario: Happy path version audit
    Tool: Bash
    Steps: grep scoped files for 0.2.1 and disallowed older values
    Expected: expected 0.2.1 hits; no stale value hits in scoped files
    Evidence: .sisyphus/evidence/task-10-version-audit.txt

  Scenario: Changelog completeness
    Tool: Read
    Steps: inspect changelog 0.2.1 section
    Expected: includes all in-scope fixes and no out-of-scope claims
    Evidence: .sisyphus/evidence/task-10-changelog-check.md
  ```

  **Commit**: YES | Message: `docs(release): finalize 0.2.1 metadata and notes` | Files: `CHANGELOG.md`, version surfaces, docs

- [x] 11. Bridge the dashboard host to the Go API with same-origin `/api/*` routing

  **What to do**: Add a repo-local bridge so requests to `http://100.112.201.95:7788/api/*` resolve on the dashboard host by rewriting them to the Go service, and update browser-facing API calls to use same-origin `/api` paths so the rewrite is exercised.
  **Must NOT do**: Do not introduce nginx/caddy/systemd or any external reverse proxy stack.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: routing bridge plus browser/API contract alignment.
  - Skills: []
  - Omitted: [`artistry`] — minimal repo-local bridge, not novel architecture.

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: 13, Final Verification Wave | Blocked By: 10

  **References**:
  - Pattern: `web/next.config.js:1-16` — current Next config surface where rewrites belong.
  - Pattern: `web/src/lib/api.ts` — browser API base helper currently driving request URLs.
  - Pattern: `web/src/components/SyncStatusPanel.tsx` — browser-origin sync/report request paths.
  - Pattern: `docker-compose.yml:33-36` — existing internal API origin default for compose.

  **Acceptance Criteria**:
  - [ ] `curl -sSf http://100.112.201.95:7788/api/health` returns a non-404 JSON response via the dashboard host.
  - [ ] Browser-visible API calls use same-origin `/api/...` URLs instead of hard-coded backend hostnames.
  - [ ] Next rewrite bridge is covered by tests or explicit verification evidence.
  - [ ] No external proxy or ingress configuration is added.

  **QA Scenarios**:
  ```
  Scenario: Dashboard-host API bridge
    Tool: Bash
    Steps: curl `http://100.112.201.95:7788/api/health` and `http://100.112.201.95:7788/api/repos/opencode-ai/opencode/report.pdf`
    Expected: both paths resolve on the dashboard host without 404
    Evidence: .sisyphus/evidence/task-11-api-bridge.txt

  Scenario: Same-origin browser requests
    Tool: Bun test
    Steps: run web tests that assert browser URLs are `/api/...` and not absolute backend hostnames
    Expected: tests pass and no absolute API host remains in browser request paths
    Evidence: .sisyphus/evidence/task-11-browser-api.txt
  ```

  **Commit**: YES | Message: `feat(web): bridge dashboard api paths through next rewrites` | Files: `web/next.config.js`, `web/src/lib/api.ts`, `web/src/components/SyncStatusPanel.tsx`, related tests

- [x] 12. Reconcile scope-drift and bookkeeping changes from the release diff

  **What to do**: Remove unintended release-diff changes to `.gitignore` and `.sisyphus/boulder.json`; keep `web/AGENTS.md` only if it is explicitly justified as docs-only alignment, otherwise revert it too.
  **Must NOT do**: Do not leave operational bookkeeping files in the implementation diff.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: release-diff hygiene and doc/bookkeeping reconciliation.
  - Skills: []
  - Omitted: [`subagent-driven-development`] — narrow diff cleanup, not a broad feature task.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: 13 | Blocked By: 10

  **References**:
  - Pattern: `.gitignore` — docs/ ignore rule currently flagged as out of scope.
  - Pattern: `.sisyphus/boulder.json` — session/plan bookkeeping currently flagged as out of scope.
  - Pattern: `web/AGENTS.md` — docs-only runtime guidance update.

  **Acceptance Criteria**:
  - [ ] `.gitignore` no longer carries the `docs/` ignore rule unless explicitly justified.
  - [ ] `.sisyphus/boulder.json` is not part of the implementation diff.
  - [ ] `web/AGENTS.md` is either justified as docs-only alignment or reverted.
  - [ ] Final diff contains only plan-scope implementation/release surfaces.

  **QA Scenarios**:
  ```
  Scenario: Diff hygiene
    Tool: Bash
    Steps: run `git diff --name-only` and inspect the release diff
    Expected: no unintended bookkeeping files remain in the final implementation diff
    Evidence: .sisyphus/evidence/task-12-diff-hygiene.txt

  Scenario: Docs-only classification
    Tool: Bash
    Steps: verify any remaining `web/AGENTS.md` change is documentation-only
    Expected: docs-only change is explicitly justified or removed
    Evidence: .sisyphus/evidence/task-12-docs-classification.txt
  ```

  **Commit**: YES | Message: `chore(release): remove scope-drift bookkeeping changes` | Files: `.gitignore`, `.sisyphus/boulder.json`, `web/AGENTS.md` as applicable

- [x] 13. Regenerate missing evidence and rerun the final verification wave

  **What to do**: Produce the missing evidence artifacts identified by the verification audits and rerun the release checks after tasks 11-12 complete.
  **Must NOT do**: Do not mark the release complete until every missing evidence artifact exists and F1-F4 approve.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: final evidence closure and validation bundle.
  - Skills: [`superpowers:verification-before-completion`] — required for fresh evidence discipline.
  - Omitted: [`brainstorming`] — execution/verification only.

  **Parallelization**: Can Parallel: NO | Wave 4 | Blocks: Final Verification Wave | Blocked By: 11, 12

  **References**:
  - Pattern: `.sisyphus/evidence/task-6-pdf-download.txt` and `task-6-pdf-invalid.txt` — backend PDF coverage.
  - Pattern: `.sisyphus/evidence/task-7-cta-download.png` and `task-7-cta-failure.png` — CTA success/failure evidence.
  - Pattern: `.sisyphus/evidence/task-8-ux-sync.png` and `task-8-ux-unavailable.png` — UX copy evidence.
  - Pattern: `.sisyphus/evidence/task-9-cors-negative.txt` and `task-10-changelog-check.md` — final audit evidence.

  **Acceptance Criteria**:
  - [ ] All missing evidence files reported by F1 exist and contain fresh outputs.
  - [ ] `go test ./internal/cmd/...`, `bun run test`, and `bun run build` all pass after remediation.
  - [ ] `curl -sSf http://100.112.201.95:7788/api/health` and PDF header checks pass.
  - [ ] F1-F4 all approve on the remediated state.

  **QA Scenarios**:
  ```
  Scenario: Evidence bundle completeness
    Tool: Bash
    Steps: verify all required evidence files exist and contain current command output
    Expected: every missing artifact is present with fresh timestamps/outputs
    Evidence: .sisyphus/evidence/task-13-evidence-bundle.txt

  Scenario: Final verification rerun
    Tool: Bash
    Steps: rerun the full verification suite and reviewer wave after tasks 11-12
    Expected: all checks PASS and reviewers approve
    Evidence: .sisyphus/evidence/task-13-final-wave.txt
  ```

  **Commit**: YES | Message: `test(release): regenerate evidence and rerun final verification` | Files: `.sisyphus/evidence/*`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Commit 1: `test(serve): codify 0.2.1 bind and CORS contract`
- Commit 2: `feat(runtime): unify API/web entrypoint and config contract`
- Commit 3: `feat(report): add direct PDF endpoint and wire dashboard CTA`
- Commit 4: `docs(release): align docs and bump to 0.2.1`

## Success Criteria
- Dashboard reachable through VPN entrypoint with stable runtime behavior.
- Dashboard host serves `/api/*` through same-origin rewrite bridge.
- Report download path no longer 404 and returns valid PDF headers/body.
- Port/bind/CORS behavior is consistent and regression-tested.
- User-facing operational guidance no longer conflicts with actual runtime.
- Final diff contains no unintended bookkeeping drift.
- `0.2.1` release surfaces are updated consistently.
