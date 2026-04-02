# Batch-Remediation Completion Plan

## TL;DR
> **Summary**: Complete batch-remediation work in three phases: push current stable changes, implement web dashboard integration, then performance validation and polish.
> **Deliverables**: Pushed origin/main, functional web dashboard, validated performance benchmarks, clean CLI UX
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: Push → Dashboard → Performance

## Context
### Original Request
Complete remaining 15/36 batch-remediation criteria with focus on web dashboard integration and performance validation.

### Current State
- 21/36 criteria completed ✅
- Core infrastructure working (PRATC_CACHE_DIR, mirror commands, cache-first analysis, batch fetch, parallel extraction)
- Local main has 8 commits ahead of origin/main
- All tests passing

### Interview Summary
User wants three-phase completion:
1. Push current stable changes to origin/main  
2. Continue with web dashboard implementation
3. Complete performance validation and CLI polish

## Work Objectives
### Core Objective
Complete all remaining batch-remediation criteria while maintaining backward compatibility and performance targets.

### Deliverables
- Origin/main updated with current stable changes
- Web dashboard with sync status display and interaction
- Performance benchmarks validating <5min for 6k PR analysis  
- Complete CLI UX with workflow guidance and API call estimates
- Backward compatibility for old mirror locations

### Definition of Done (verifiable conditions with commands)
- `git push origin main` succeeds with no conflicts
- Web dashboard shows sync status and allows sync triggering
- `make test` passes all tests including new frontend tests
- 6k PR analysis completes in <5 minutes (verified by timing)
- CLI provides clear workflow guidance with accurate API estimates
- Old mirrors in .pratc/repos/ are migrated or handled gracefully

### Must Have
- No breaking changes to existing CLI APIs
- Web dashboard matches existing UI patterns
- Performance targets met or exceeded
- Full test coverage for new functionality

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- No new external dependencies beyond existing stack
- No changes to core analysis logic (only caching/performance)
- No v0.1 scope creep (GitHub App/OAuth/webhooks remain out of scope)
- No duplicate code (follow existing patterns)

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: tests-after + TDD for new components
- QA policy: Every task has agent-executed scenarios
- Evidence: .sisyphus/evidence/task-{N}-{slug}.{ext}

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. Extract shared dependencies as Wave-1 tasks for max parallelism.

Wave 1: Push and Setup
- Push current changes to origin/main
- Create web dashboard feature branch  
- Set up frontend development environment

Wave 2: Web Dashboard Implementation  
- Implement sync status API endpoint enhancements
- Build sync status UI component
- Add sync triggering functionality
- Wire analysis requirements to sync state

Wave 3: Performance and Polish
- Implement file list caching in SQLite  
- Add performance benchmarking infrastructure
- Complete CLI workflow guidance
- Implement old mirror migration
- Run comprehensive validation

### Dependency Matrix (full, all tasks)
Task 1 → Task 2 → Tasks 3-7 → Tasks 8-12 → Tasks 13-15

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1: 3 tasks → git, frontend-setup
- Wave 2: 4 tasks → frontend-ui-ux, writing  
- Wave 3: 5 tasks → quick, unspecified-high, writing

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [ ] 1. Push Current Changes to Origin Main

  **What to do**: Push the current local main branch (8 commits ahead) to origin/main safely.
  **Must NOT do**: Force push, create merge conflicts, skip verification.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Simple git operation
  - Skills: [`git-master`] — Git expertise for safe pushes
  - Omitted: [`frontend-ui-ux`] — Not needed for git operations

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: [2, 3] | Blocked By: []

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Standard git push workflow
  - External: Git documentation for safe pushing

  **Acceptance Criteria** (agent-executable only):
  - [ ] `git push origin main` executes successfully
  - [ ] Remote origin/main matches local main exactly
  - [ ] No merge conflicts or force-push required

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Successful push to origin
    Tool: Bash
    Steps: git push origin main
    Expected: Push completes with "Everything up-to-date" or successful commit notification
    Evidence: .sisyphus/evidence/task-1-push-main.txt

  Scenario: Verify remote matches local
    Tool: Bash  
    Steps: git fetch origin && git diff origin/main..main
    Expected: No differences (empty output)
    Evidence: .sisyphus/evidence/task-1-verify-remote.txt
  ```

  **Commit**: NO | Message: N/A | Files: []

- [ ] 2. Create Web Dashboard Feature Branch

  **What to do**: Create a new feature branch for web dashboard work to isolate changes.
  **Must NOT do**: Work directly on main, create naming conflicts.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Standard git branching
  - Skills: [`git-master`] — Proper branching strategy
  - Omitted: [`writing`] — Simple branch creation

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [4, 5, 6, 7] | Blocked By: [1]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Standard feature branch naming
  - External: Git branching best practices

  **Acceptance Criteria** (agent-executable only):
  - [ ] Feature branch `feat/web-dashboard-sync` created from current main
  - [ ] Branch pushed to remote origin
  - [ ] No uncommitted changes in working directory

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create and push feature branch
    Tool: Bash
    Steps: git checkout -b feat/web-dashboard-sync && git push -u origin feat/web-dashboard-sync
    Expected: Branch created and pushed successfully
    Evidence: .sisyphus/evidence/task-2-feature-branch.txt

  Scenario: Verify branch exists remotely
    Tool: Bash
    Steps: git ls-remote --heads origin feat/web-dashboard-sync
    Expected: Branch reference returned (non-empty)
    Evidence: .sisyphus/evidence/task-2-verify-remote-branch.txt
  ```

  **Commit**: NO | Message: N/A | Files: []

- [ ] 3. Setup Frontend Development Environment

  **What to do**: Verify and configure frontend development setup for web dashboard work.
  **Must NOT do**: Modify existing working configuration, skip dependency verification.

  **Recommended Agent Profile**:
  - Category: `frontend-ui-ux` — Reason: Frontend environment setup
  - Skills: [] — Basic environment verification
  - Omitted: [`quick`] — Requires frontend knowledge

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [4, 5, 6, 7] | Blocked By: [2]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `web/AGENTS.md` — Web development conventions
  - Config: `web/package.json` — Dependencies and scripts

  **Acceptance Criteria** (agent-executable only):
  - [ ] `cd web && bun install` completes successfully
  - [ ] `bun run dev` starts development server without errors
  - [ ] Existing tests pass with `bun run test`

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Install frontend dependencies
    Tool: Bash
    Steps: cd web && bun install
    Expected: Installation completes with exit code 0
    Evidence: .sisyphus/evidence/task-3-bun-install.txt

  Scenario: Verify development server starts
    Tool: interactive_bash
    Steps: cd web && timeout 30s bun run dev
    Expected: Server starts and binds to port (output contains "localhost:" or similar)
    Evidence: .sisyphus/evidence/task-3-dev-server.txt
  ```

  **Commit**: NO | Message: N/A | Files: []

- [ ] 4. Enhance Sync Status API Endpoint

  **What to do**: Update the `/api/repos/{owner}/{name}/sync/status` endpoint to return complete status information as specified in the plan.
  **Must NOT do**: Break existing API contract, remove existing fields.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: API contract modification requiring careful validation
  - Skills: [`superpowers/test-driven-development`] — Ensure backward compatibility
  - Omitted: [`quick`] — Complex API changes require thorough testing

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [5] | Blocked By: [3]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/cmd/root.go:403` — Current getSyncStatus implementation
  - Contract: `.sisyphus/plans/batch-remediation.md:165` — Required response format
  - Type: `internal/cache/sqlite.go` — sync_jobs table schema for job status

  **Acceptance Criteria** (agent-executable only):
  - [ ] API returns `{status, last_sync, pr_count, in_progress, progress_percent}` 
  - [ ] `status` is one of: "completed", "in_progress", "never"
  - [ ] `in_progress` boolean indicates active sync job
  - [ ] `progress_percent` calculated from sync_progress table
  - [ ] Existing fields (`repo`, `last_sync`, `pr_count`) preserved

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Sync status returns complete object
    Tool: Bash
    Steps: curl -s http://localhost:8080/api/repos/owner/repo/sync/status | jq '.'
    Expected: JSON object contains all required fields with correct types
    Evidence: .sisyphus/evidence/task-4-api-response.json

  Scenario: Status reflects actual sync state
    Tool: Bash
    Steps: Trigger sync job, immediately call sync/status endpoint
    Expected: status="in_progress", in_progress=true during sync
    Evidence: .sisyphus/evidence/task-4-in-progress.json
  ```

  **Commit**: YES | Message: `feat(api): enhance sync status endpoint with full status info` | Files: [`internal/cmd/root.go`]

- [ ] 5. Implement Sync Status UI Component

  **What to do**: Create React component that displays sync status and allows user interaction.
  **Must NOT do**: Deviate from existing UI patterns, create accessibility issues.

  **Recommended Agent Profile**:
  - Category: `frontend-ui-ux` — Reason: UI component development
  - Skills: [] — Follow existing frontend patterns
  - Omitted: [`writing`] — Implementation-focused task

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [6] | Blocked By: [4]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `web/src/components/` — Existing component structure
  - Style: `web/src/styles/` — CSS styling conventions  
  - API: `web/src/lib/api.ts` — API client patterns

  **Acceptance Criteria** (agent-executable only):
  - [ ] Component displays sync timestamp and PR count when available
  - [ ] Shows "Never synced" message when no sync data exists
  - [ ] Includes "Sync Now" button that triggers background sync
  - [ ] Shows sync progress during active sync jobs
  - [ ] Matches existing UI design and accessibility standards

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Component renders sync status correctly
    Tool: /playwright
    Steps: Load dashboard page, check sync status display
    Expected: Shows appropriate message based on sync state (timestamp or "Never synced")
    Evidence: .sisyphus/evidence/task-5-status-display.png

  Scenario: Sync Now button is functional
    Tool: /playwright  
    Steps: Click "Sync Now" button, verify API call made
    Expected: POST request to /api/repos/{owner}/{repo}/sync initiated
    Evidence: .sisyphus/evidence/task-5-sync-trigger.png
  ```

  **Commit**: YES | Message: `feat(web): add sync status UI component` | Files: [`web/src/components/SyncStatus.tsx`, `web/src/components/SyncStatus.test.tsx`]

- [ ] 6. Wire Analysis to Require Sync State

  **What to do**: Update analysis commands to require recent sync or explicit --live flag before proceeding.
  **Must NOT do**: Break existing --force-live functionality, make analysis unusable.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: User workflow modification
  - Skills: [`superpowers/test-driven-development`] — Preserve existing behavior
  - Omitted: [`quick`] — Affects core user experience

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [7] | Blocked By: [5]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/app/service.go` — Current loadPRs logic
  - Flag: `--force-live` flag handling in CLI commands
  - Validation: Existing error handling patterns

  **Acceptance Criteria** (agent-executable only):
  - [ ] Analysis fails with helpful error if no recent sync and no --live flag
  - [ ] --live flag bypasses sync requirement (preserves existing behavior)
  - [ ] Error message includes clear instructions for workflow
  - [ ] Recent sync (within TTL) allows analysis without --live flag

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Analysis requires sync or --live flag
    Tool: Bash
    Steps: pratc analyze --repo=owner/repo (no sync, no --live)
    Expected: Error message with clear instructions for proper workflow
    Evidence: .sisyphus/evidence/task-6-analysis-blocked.txt

  Scenario: --live flag bypasses requirement
    Tool: Bash
    Steps: pratc analyze --repo=owner/repo --live  
    Expected: Analysis proceeds normally (existing behavior preserved)
    Evidence: .sisyphus/evidence/task-6-live-allowed.txt
  ```

  **Commit**: YES | Message: `feat(cli): require sync state for analysis with clear workflow guidance` | Files: [`internal/app/service.go`, `internal/cmd/root.go`]

- [ ] 7. Add Sync Triggering from Dashboard

  **What to do**: Connect the "Sync Now" button to actually trigger background sync jobs.
  **Must NOT do**: Make sync blocking, fail silently on errors.

  **Recommended Agent Profile**:
  - Category: `frontend-ui-ux` — Reason: Frontend-backend integration
  - Skills: [] — API integration patterns
  - Omitted: [`writing`] — Implementation task

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [] | Blocked By: [6]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `web/src/lib/api.ts` — Existing API client
  - Endpoint: `POST /api/repos/{owner}/{repo}/sync` — Sync trigger API  
  - Response: HTTP 202 Accepted for async operations

  **Acceptance Criteria** (agent-executable only):
  - [ ] "Sync Now" button makes POST request to sync endpoint
  - [ ] Button shows loading state during request
  - [ ] Success shows confirmation message
  - [ ] Error shows helpful error message
  - [ ] Component polls sync status after triggering sync

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Sync Now triggers background job
    Tool: /playwright
    Steps: Click "Sync Now", monitor network requests
    Expected: POST to /sync endpoint returns 202, component shows loading state
    Evidence: .sisyphus/evidence/task-7-sync-triggered.png

  Scenario: Component updates after sync completion
    Tool: /playwright
    Steps: Trigger sync, wait for completion, verify status updates
    Expected: Status changes from "in progress" to "completed" with timestamp
    Evidence: .sisyphus/evidence/task-7-sync-completed.png
  ```

  **Commit**: YES | Message: `feat(web): connect sync button to backend sync endpoint` | Files: [`web/src/components/SyncStatus.tsx`]

- [ ] 8. Implement File List Caching in SQLite  

  **What to do**: Cache file lists from git diff operations in SQLite to avoid repeated git operations.
  **Must NOT do**: Store redundant data, break existing file extraction logic.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Data caching optimization
  - Skills: [`superpowers/test-driven-development`] — Ensure cache correctness
  - Omitted: [`frontend-ui-ux`] — Backend-only change

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [9] | Blocked By: [7]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/cache/sqlite.go` — Existing cache schema and methods
  - Table: `pr_files` table already exists for file storage
  - Method: `GetChangedFiles()` in `internal/repo/mirror.go`

  **Acceptance Criteria** (agent-executable only):
  - [ ] File lists cached with PR number and repo as keys
  - [ ] Cache checked before running git diff in GetChangedFiles()
  - [ ] Cache invalidated when mirror syncs new refs for that PR
  - [ ] Target <100ms per PR file list retrieval from cache

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: File lists cached after first access
    Tool: Bash
    Steps: Call GetChangedFiles twice for same PR, measure timing
    Expected: Second call significantly faster (<100ms vs >500ms)
    Evidence: .sisyphus/evidence/task-8-cache-performance.txt

  Scenario: Cache invalidated on mirror sync
    Tool: Bash
    Steps: Sync new refs for PR, call GetChangedFiles
    Expected: Fresh git diff executed, not stale cached data
    Evidence: .sisyphus/evidence/task-8-cache-invalidation.txt
  ```

  **Commit**: YES | Message: `feat(cache): cache file lists in SQLite to avoid repeated git operations` | Files: [`internal/cache/sqlite.go`, `internal/repo/mirror.go`]

- [ ] 9. Add Performance Benchmarking Infrastructure

  **What to do**: Create automated performance benchmarks to validate the <5 minute target for 6k PR analysis.
  **Must NOT do**: Run benchmarks on every test, create flaky tests.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: Performance testing infrastructure
  - Skills: [] — Benchmarking expertise
  - Omitted: [`quick`] — Requires careful test design

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [10, 11] | Blocked By: [8]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/app/service_test.go` — Existing test structure
  - Target: `.sisyphus/plans/batch-remediation.md:151` — Performance targets
  - Scale: 5.5k PR fixture data in `fixtures/` directory

  **Acceptance Criteria** (agent-executable only):
  - [ ] Benchmark test for 6k PR analysis time <5 minutes
  - [ ] Benchmark test for 1000 PR sync time <2 minutes  
  - [ ] Benchmark test for 1000 PR file extraction <1 minute
  - [ ] Benchmarks marked as slow/integration tests (not run by default)
  - [ ] Clear pass/fail criteria with timing thresholds

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Performance benchmarks execute and pass
    Tool: Bash
    Steps: go test -tags=integration -run=TestPerformance ./...
    Expected: All performance benchmarks pass within time limits
    Evidence: .sisyphus/evidence/task-9-performance-benchmarks.txt

  Scenario: Benchmarks skip by default
    Tool: Bash
    Steps: go test ./... (without -tags=integration)
    Expected: Performance benchmarks skipped, regular tests pass quickly
    Evidence: .sisyphus/evidence/task-9-benchmarks-skipped.txt
  ```

  **Commit**: YES | Message: `feat(test): add performance benchmarking infrastructure for batch processing` | Files: [`internal/app/performance_test.go`]

- [ ] 10. Complete CLI Workflow Guidance

  **What to do**: Enhance CLI warnings to provide clear recommended workflow steps and estimated API call counts.
  **Must NOT do**: Make CLI too verbose, break existing --force behavior.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: User communication and messaging
  - Skills: [] — Clear instruction writing
  - Omitted: [`frontend-ui-ux`] — CLI-only changes

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [12] | Blocked By: [9]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/app/service.go` — Current warning messages
  - Requirement: `.sisyphus/plans/batch-remediation.md:190-192` — Specific content requirements
  - Style: Existing CLI output formatting

  **Acceptance Criteria** (agent-executable only):
  - [ ] Warning includes clear 2-step workflow instructions
  - [ ] Shows estimated API call count based on open PR count
  - [ ] Respects --force flag to skip warning entirely
  - [ ] Uses consistent formatting with existing CLI output
  - [ ] Provides actionable next steps for users

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: CLI shows enhanced workflow guidance
    Tool: Bash
    Steps: pratc analyze --repo=owner/repo (no sync)
    Expected: Output includes numbered workflow steps and API estimate
    Evidence: .sisyphus/evidence/task-10-cli-guidance.txt

  Scenario: --force flag skips enhanced warning
    Tool: Bash
    Steps: pratc analyze --repo=owner/repo --force
    Expected: Minimal warning (existing behavior) or no warning
    Evidence: .sisyphus/evidence/task-10-force-skips.txt
  ```

  **Commit**: YES | Message: `feat(cli): enhance workflow guidance with clear steps and API estimates` | Files: [`internal/app/service.go`]

- [ ] 11. Implement Old Mirror Migration

  **What to do**: Add backward compatibility to handle existing mirrors in the old location (.pratc/repos/).
  **Must NOT do**: Delete user data without confirmation, break existing setups.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Migration utility
  - Skills: [`superpowers/test-driven-development`] — Safe data handling
  - Omitted: [`writing`] — Implementation-focused

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [12] | Blocked By: [9]

  **References** (executor has NO interview context — be exhaustive):
  - Old location: `./.pratc/repos/` in project directory
  - New location: `PRATC_CACHE_DIR` or `~/.cache/pratc/repos/`
  - Command: `pratc mirror migrate` subcommand (from earlier work)

  **Acceptance Criteria** (agent-executable only):
  - [ ] Detects mirrors in old location (.pratc/repos/)
  - [ ] Provides `pratc mirror migrate` command to move old mirrors
  - [ ] Handles migration errors gracefully (don't fail if old mirrors corrupt)
  - [ ] Option to ignore old mirrors with warning instead of migrating
  - [ ] Preserves all mirror data during migration

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Migration command moves old mirrors
    Tool: Bash
    Steps: Create test mirror in .pratc/repos/, run pratc mirror migrate
    Expected: Mirror moved to new location, old location cleaned up
    Evidence: .sisyphus/evidence/task-11-migration-success.txt

  Scenario: Missing old mirrors handled gracefully
    Tool: Bash
    Steps: Run pratc mirror migrate with no old mirrors present
    Expected: Clean exit with "No old mirrors found" message
    Evidence: .sisyphus/evidence/task-11-no-old-mirrors.txt
  ```

  **Commit**: YES | Message: `feat(mirror): add backward compatibility for old mirror locations with migrate command` | Files: [`internal/cmd/root.go`, `internal/repo/mirror.go`]

- [ ] 12. Implement SSE Progress Streaming

  **What to do**: Enhance the SSE endpoint to stream sync progress to web clients in real-time.
  **Must NOT do**: Block the main thread, create memory leaks.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: Real-time streaming implementation
  - Skills: [] — SSE and streaming expertise  
  - Omitted: [`quick`] — Complex real-time features

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [13] | Blocked By: [10, 11]

  **References** (executor has NO interview context — be exhaustive):
  - Endpoint: `GET /api/repos/{owner}/{repo}/sync/stream` — Existing SSE route
  - Handler: `internal/cmd/root.go:457` — Current Stream implementation
  - Progress: `sync_progress` table for progress tracking

  **Acceptance Criteria** (agent-executable only):
  - [ ] SSE endpoint sends progress events during sync operations
  - [ ] Events include current/total PRs processed and percentage
  - [ ] Connection closes cleanly when sync completes
  - [ ] Multiple clients can connect simultaneously
  - [ ] Error states communicated via SSE events

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: SSE streams sync progress events
    Tool: Bash
    Steps: Start sync, connect to /sync/stream endpoint with curl
    Expected: Event stream shows progress updates during sync
    Evidence: .sisyphus/evidence/task-12-sse-progress.txt

  Scenario: Multiple SSE clients work simultaneously  
    Tool: Bash
    Steps: Connect two curl sessions to /sync/stream, start sync
    Expected: Both clients receive identical progress events
    Evidence: .sisyphus/evidence/task-12-multiple-clients.txt
  ```

  **Commit**: YES | Message: `feat(api): implement SSE progress streaming for sync operations` | Files: [`internal/cmd/root.go`, `internal/sync/sse.go`]

- [ ] 13. Implement Subsequent Request Blocking

  **What to do**: Make subsequent analysis requests block until background sync completes (with timeout).
  **Must NOT do**: Create infinite blocking, deadlock the system.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: Concurrency and coordination logic
  - Skills: [`superpowers/systematic-debugging`] — Avoid race conditions
  - Omitted: [`quick`] — Complex synchronization required

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [14] | Blocked By: [12]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/app/service.go` — Current loadPRs logic
  - State: `sync_jobs` table for tracking active sync jobs  
  - Timeout: Reasonable timeout (e.g., 30 seconds) for blocking

  **Acceptance Criteria** (agent-executable only):
  - [ ] Analysis request detects active sync job for same repo
  - [ ] Request blocks and waits for sync completion (max 30s timeout)
  - [ ] Returns analysis results once sync completes
  - [ ] Times out gracefully if sync takes too long
  - [ ] Concurrent requests for different repos don't block each other

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Analysis blocks during active sync
    Tool: Bash
    Steps: Start sync in background, immediately request analysis
    Expected: Analysis request waits, then returns results after sync completes
    Evidence: .sisyphus/evidence/task-13-blocking-analysis.txt

  Scenario: Timeout prevents infinite waiting
    Tool: Bash
    Steps: Mock long-running sync, request analysis
    Expected: Analysis times out after reasonable period with helpful error
    Evidence: .sisyphus/evidence/task-13-timeout.txt
  ```

  **Commit**: YES | Message: `feat(api): implement blocking analysis requests during active sync with timeout` | Files: [`internal/app/service.go`]

- [ ] 14. Run Comprehensive Validation

  **What to do**: Execute end-to-end validation of all implemented features to ensure they work together correctly.
  **Must NOT do**: Skip edge case testing, assume individual tests are sufficient.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: Integration validation
  - Skills: [`superpowers/verification-before-completion`] — Thorough validation
  - Omitted: [`quick`] — Comprehensive testing required

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: [15] | Blocked By: [13]

  **References** (executor has NO interview context — be exhaustive):
  - Plan: `.sisyphus/plans/batch-remediation.md` — All acceptance criteria
  - Tests: All implemented test files
  - Performance: Benchmarks from Task 9

  **Acceptance Criteria** (agent-executable only):
  - [ ] End-to-end workflow: sync → analyze completes successfully
  - [ ] Web dashboard shows correct sync status throughout workflow
  - [ ] CLI provides appropriate guidance at each step
  - [ ] Performance benchmarks pass consistently
  - [ ] All 36/36 batch-remediation criteria verified complete

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Complete end-to-end workflow validation
    Tool: Bash
    Steps: Clean environment, pratc sync, pratc analyze, check web dashboard
    Expected: Entire workflow works smoothly with proper status updates
    Evidence: .sisyphus/evidence/task-14-e2e-workflow.txt

  Scenario: All batch-remediation criteria verified
    Tool: Bash
    Steps: Count completed checkboxes in batch-remediation.md
    Expected: All 36 criteria marked as [x] complete
    Evidence: .sisyphus/evidence/task-14-all-complete.txt
  ```

  **Commit**: NO | Message: N/A | Files: []

- [ ] 15. Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay. Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep
## Commit Strategy
## Success Criteria