# Fix Plan: Sync Process Integration

## TL;DR

> **Quick Summary**: Fix the critical architectural flaw in prATC's sync process by integrating PR metadata saving with the existing cache/database layer. The current implementation fetches PR data via GraphQL but discards it instead of saving to SQLite.
> 
> **Deliverables**:
> - Modified `githubMetadataSource.SyncRepo()` to save PR metadata to cache
> - Updated `Worker.SyncJob()` to pass cache store to metadata source  
> - Integration tests verifying PR data persistence
> - End-to-end sync verification with real data
> 
> **Estimated Effort**: Medium
> **Critical Path**: Fix metadata saving → Verify integration → Test end-to-end → Re-run production

---

## Context

### Root Cause Analysis

**Current Broken Flow**:
1. `Worker.SyncJob()` calls `w.Metadata.SyncRepo()`  
2. `githubMetadataSource.SyncRepo()` fetches full PR objects via GraphQL
3. **PR objects are discarded** - only PR numbers are returned
4. PR numbers used to fetch git refs into mirror
5. **SQLite remains empty** because PR metadata was never saved
6. Sync returns success (git operations worked)
7. Downstream commands fail (no data in SQLite)

**Missing Integration**:
- Database layer exists (`cache.UpsertPR()`) 
- But sync process never calls it
- PR metadata fetched but immediately garbage collected

### Evidence of Failure

| Symptom | Evidence |
|---------|----------|
| Empty SQLite | `SELECT COUNT(*) FROM pull_requests` → 0 |
| Stuck jobs | `sync_jobs.status = 'in_progress'` since 2026-03-29 |
| Mirror without PRs | 48k commits but 0 `refs/pull/*` |
| False success | CLI returns `{"completed": true}` |

---

## Work Objectives

### Core Objective
Modify the sync process to properly persist PR metadata to SQLite while maintaining existing git mirror functionality.

### Concrete Deliverables
- `internal/sync/default_runner.go` - Updated to pass cache store to worker
- `internal/sync/worker.go` - Modified Worker to accept and use cache store  
- `internal/github/client.go` - Enhanced githubMetadataSource to save PRs
- Integration tests verifying end-to-end sync
- Verified working sync on test repository

### Definition of Done
- [ ] `./bin/pratc sync --repo=test/repo` → PRs saved to SQLite
- [ ] `sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests"` → > 0
- [ ] `./bin/pratc analyze --repo=test/repo` → returns valid JSON with data
- [ ] All existing tests continue to pass
- [ ] End-to-end production execution works on openclaw/openclaw

### Must Have
- PR metadata saved to SQLite during sync
- Backward compatibility maintained
- Error handling for database operations
- Progress reporting during metadata saving

### Must NOT Have
- Breaking changes to existing CLI interface
- Performance regression on sync operations
- Data loss or corruption

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go tests, SQLite)
- **Automated tests**: TDD - write failing tests first, then implement fix
- **Framework**: Go's `testing` package

### QA Policy
Every task includes agent-executed verification. Evidence saved to `.sisyphus/evidence/sync-fix/`.

---

## Execution Strategy

```
Wave 1 (Fix Implementation — 4 tasks):
├── Task 1: Create integration test showing current failure
├── Task 2: Modify githubMetadataSource to accept cache store
├── Task 3: Update Worker and DefaultRunner to pass cache store
└── Task 4: Implement PR metadata saving in SyncRepo

Wave 2 (Testing & Validation — 3 tasks):
├── Task 5: Run integration tests, verify fix works
├── Task 6: Test end-to-end with small repository
└── Task 7: Verify all existing tests still pass

Wave 3 (Production Retry — 2 tasks):
├── Task 8: Re-run production execution on openclaw/openclaw
└── Task 9: Final verification and success confirmation

Final Wave (2 parallel reviews):
├── F1: Code quality and integration review
└── F2: Production execution audit
```

Critical Path: T1 → T2 → T3 → T4 → T5 → T6 → T8 → F1-F2

---

## TODOs

- [x] 1. Create integration test showing current failure

  **What to do**:
  - Create `internal/sync/sync_integration_test.go`
  - Write test that: creates temp DB, runs sync on test repo, checks PR count
  - Test should FAIL initially (demonstrating the bug)
  - Use small test repository (e.g., `jeffersonnunn/test-repo` with 1-2 PRs)

  **Must NOT do**:
  - Don't modify production code yet
  - Don't use large repositories in tests

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: NO (first step)
  - **Blocks**: Tasks 2-4
  - **Blocked By**: None

  **References**:
  - `internal/cache/sqlite_test.go` - Existing cache tests
  - `internal/sync/default_runner_test.go` - Existing sync tests

  **Acceptance Criteria**:
  - [ ] Integration test file created
  - [ ] Test fails initially (proving bug exists)
  - [ ] Test uses real GitHub API (not mocked)

  **QA Scenarios**:
  ```
  Scenario: Integration test demonstrates sync bug
    Tool: Bash
    Steps:
      1. go test -run TestSyncIntegration -v ./internal/sync/
      2. Verify test fails with "expected > 0 PRs, got 0"
    Expected Result: Test fails, proving bug exists
    Evidence: .sisyphus/evidence/sync-fix/task-1-test-failure.txt
  ```

  **Commit**: YES
  - Message: `test(sync): add integration test demonstrating metadata bug`
  - Files: `internal/sync/sync_integration_test.go`

  ---

- [x] 2. Modify githubMetadataSource to accept cache store

  **What to do**:
  - Add `cacheStore *cache.Store` field to `githubMetadataSource` struct
  - Update constructor to accept cache store parameter
  - Add validation to ensure cache store is not nil when saving enabled
  - Maintain backward compatibility (nil cache store = no saving)

  **Must NOT do**:
  - Don't break existing usage patterns
  - Don't make cache store mandatory (preserve existing behavior)

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 3 after T1)
  - **Blocks**: Task 4
  - **Blocked By**: Task 1

  **References**:
  - `internal/sync/default_runner.go:114-139` - Current githubMetadataSource
  - `internal/cache/sqlite.go:54-93` - UpsertPR implementation

  **Acceptance Criteria**:
  - [ ] githubMetadataSource accepts optional cache store
  - [ ] Backward compatibility maintained
  - [ ] `go build ./...` → PASS

  **QA Scenarios**:
  ```
  Scenario: githubMetadataSource accepts cache store
    Tool: Bash
    Steps:
      1. go build ./...
      2. Verify no compilation errors
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/sync-fix/task-2-build.txt
  ```

  **Commit**: YES
  - Message: `feat(sync): add cache store support to githubMetadataSource`
  - Files: `internal/sync/default_runner.go`

  ---

- [x] 3. Update Worker and DefaultRunner to pass cache store

  **What to do**:
  - Add `CacheStore *cache.Store` field to `Worker` struct
  - Update `defaultWorker()` to create `githubMetadataSource` with cache store
  - Update `DefaultRunner` constructor to accept cache store parameter
  - Pass cache store through from CLI to worker

  **Must NOT do**:
  - Don't break existing CLI interface
  - Don't make cache store mandatory at CLI level

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2 after T1)
  - **Blocks**: Task 4
  - **Blocked By**: Task 1

  **References**:
  - `internal/sync/worker.go:38-42` - Worker struct
  - `internal/sync/default_runner.go:20-23` - DefaultRunner struct
  - `cmd/pratc/sync.go` - CLI sync command

  **Acceptance Criteria**:
  - [ ] Worker accepts optional cache store
  - [ ] DefaultRunner passes cache store to worker
  - [ ] CLI sync command passes cache store from config
  - [ ] `go build ./...` → PASS

  **QA Scenarios**:
  ```
  Scenario: Worker and DefaultRunner updated for cache integration
    Tool: Bash
    Steps:
      1. go build ./...
      2. ./bin/pratc sync --help (verify no breaking changes)
    Expected Result: Build succeeds, CLI unchanged
    Evidence: .sisyphus/evidence/sync-fix/task-3-cli-unchanged.txt
  ```

  **Commit**: YES
  - Message: `feat(sync): integrate cache store through worker pipeline`
  - Files: `internal/sync/worker.go`, `internal/sync/default_runner.go`

  ---

- [x] 4. Implement PR metadata saving in SyncRepo

  **What to do**:
  - In `githubMetadataSource.SyncRepo()`, after fetching PRs:
    - If cache store is not nil, iterate through PRs and call `cacheStore.UpsertPR(pr)`
    - Add error handling for database operations
    - Add progress reporting for metadata saving
  - Ensure PR objects are not discarded before saving
  - Handle partial failures gracefully

  **Must NOT do**:
  - Don't block git operations if database fails
  - Don't lose PR data on partial failures

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 2, 3)
  - **Blocks**: Tasks 5-9
  - **Blocked By**: Tasks 2, 3

  **References**:
  - `internal/sync/default_runner.go:118-139` - Current SyncRepo implementation
  - `internal/cache/sqlite.go:54-93` - UpsertPR signature and error handling

  **Acceptance Criteria**:
  - [ ] PR metadata saved to SQLite during sync
  - [ ] Error handling for database operations
  - [ ] Progress reporting during metadata saving
  - [ ] `go build ./...` → PASS

  **QA Scenarios**:
  ```
  Scenario: PR metadata saved during sync
    Tool: Bash
    Steps:
      1. Create temp DB: sqlite3 /tmp/test.db ".schema"
      2. Run sync with temp DB
      3. Check PR count: sqlite3 /tmp/test.db "SELECT COUNT(*) FROM pull_requests;"
    Expected Result: PR count > 0
    Evidence: .sisyphus/evidence/sync-fix/task-4-metadata-saved.txt
  ```

  **Commit**: YES
  - Message: `fix(sync): save PR metadata to cache during sync`
  - Files: `internal/sync/default_runner.go`

  ---

- [ ] 5. Run integration tests, verify fix works

  **What to do**:
  - Run the integration test created in Task 1
  - Should now PASS (was previously failing)
  - Add additional test cases for edge cases
  - Verify both git mirror and SQLite are populated

  **Must NOT do**:
  - Don't skip test verification
  - Don't claim fix works without evidence

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential validation)
  - **Blocks**: Tasks 6-9
  - **Blocked By**: Task 4

  **Acceptance Criteria**:
  - [ ] Integration test passes
  - [ ] Both git mirror and SQLite populated
  - [ ] All existing tests still pass

  **QA Scenarios**:
  ```
  Scenario: Integration test passes after fix
    Tool: Bash
    Steps:
      1. go test -run TestSyncIntegration -v ./internal/sync/
      2. Verify test passes
    Expected Result: Test passes, PRs saved to DB
    Evidence: .sisyphus/evidence/sync-fix/task-5-test-pass.txt
  ```

  **Commit**: NO

  ---

- [ ] 6. Test end-to-end with small repository

  **What to do**:
  - Choose small test repository (1-5 PRs)
  - Run full sync → analyze → cluster → graph → plan → report
  - Verify all commands return valid data
  - Measure performance vs expectations

  **Must NOT do**:
  - Don't use large repositories yet (save for production)
  - Don't skip any pipeline steps

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential validation)
  - **Blocks**: Tasks 7-9
  - **Blocked By**: Task 5

  **Acceptance Criteria**:
  - [ ] Full pipeline works end-to-end
  - [ ] All commands return valid data (not placeholders)
  - [ ] Performance reasonable for small repo

  **QA Scenarios**:
  ```
  Scenario: End-to-end pipeline works on small repo
    Tool: Bash
    Steps:
      1. ./bin/pratc sync --repo=jeffersonnunn/test-repo-small
      2. ./bin/pratc analyze --repo=jeffersonnunn/test-repo-small --format=json | jq '.counts.total_prs'
      3. Verify PR count > 0
    Expected Result: All commands work, return real data
    Evidence: .sisyphus/evidence/sync-fix/task-6-e2e-small.txt
  ```

  **Commit**: NO

  ---

- [ ] 7. Verify all existing tests still pass

  **What to do**:
  - Run full test suite: `make test-go`
  - Ensure no regressions introduced
  - Verify CLI interface unchanged
  - Check for any new warnings or errors

  **Must NOT do**:
  - Don't ignore test failures
  - Don't proceed to production with broken tests

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 6)
  - **Blocks**: Tasks 8-9
  - **Blocked By**: Task 4

  **Acceptance Criteria**:
  - [ ] `make test-go` → all tests pass
  - [ ] No new compiler warnings
  - [ ] CLI help unchanged

  **QA Scenarios**:
  ```
  Scenario: All existing tests pass
    Tool: Bash
    Steps:
      1. make test-go
      2. Verify all tests pass
    Expected Result: No test failures, no regressions
    Evidence: .sisyphus/evidence/sync-fix/task-7-tests-pass.txt
  ```

  **Commit**: NO

  ---

- [ ] 8. Re-run production execution on openclaw/openclaw

  **What to do**:
  - Execute full production plan on openclaw/openclaw
  - Monitor sync progress and completion
  - Verify PR count matches expected (~5,500)
  - Run full omni pipeline (analyze, cluster, graph, plan, report)

  **Must NOT do**:
  - Don't skip any production steps
  - Don't assume success without verification

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential production run)
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 5, 6, 7

  **Acceptance Criteria**:
  - [ ] Sync completes successfully
  - [ ] PR count ~5,500 in SQLite
  - [ ] All omni commands work end-to-end
  - [ ] Performance within SLOs

  **QA Scenarios**:
  ```
  Scenario: Production execution works on openclaw/openclaw
    Tool: Bash
    Steps:
      1. ./bin/pratc sync --repo=openclaw/openclaw
      2. sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"
      3. ./bin/pratc analyze --repo=openclaw/openclaw --format=json | jq '.counts.total_prs'
    Expected Result: Sync completes, ~5500 PRs, analyze returns data
    Evidence: .sisyphus/evidence/sync-fix/task-8-production-success.txt
  ```

  **Commit**: NO

  ---

- [ ] 9. Final verification and success confirmation

  **What to do**:
  - Verify all Definition of Done criteria met
  - Generate comprehensive PDF report
  - Start web dashboard and verify data loading
  - Archive evidence and mark production complete

  **Must NOT do**:
  - Don't skip final verification
  - Don't claim success prematurely

  **Recommended Agent Profile**:
  - **Category**: `oracle`

  **Parallelization**:
  - **Can Run In Parallel**: NO (final step)
  - **Blocks**: Final Wave
  - **Blocked By**: Task 8

  **Acceptance Criteria**:
  - [ ] All DoD criteria verified
  - [ ] PDF report generated (> 100KB)
  - [ ] Web dashboard loads real data
  - [ ] Evidence archived

  **QA Scenarios**:
  ```
  Scenario: Final production success verification
    Tool: Bash + Playwright
    Steps:
      1. ./bin/pratc report --repo=openclaw/openclaw --format=pdf --output=final-report.pdf
      2. stat -c%s final-report.pdf
      3. Start web dashboard, verify data loads
    Expected Result: Report generated, dashboard shows real data
    Evidence: .sisyphus/evidence/sync-fix/task-9-final-success.txt
  ```

  **Commit**: NO

  ---

## Final Verification Wave (MANDATORY)

- [ ] F1. **Code Quality and Integration Review** — `unspecified-high`
  
  Review code changes for quality, maintainability, and proper integration. Verify error handling, logging, and performance.
  
  Output: `Code Quality [PASS/FAIL] | Integration [PASS/FAIL] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Production Execution Audit** — `oracle`
  
  Verify the complete production execution against original requirements. Confirm all omni features delivered and working.
  
  Output: `Features Delivered [N/N] | SLOs Met [N/N] | VERDICT: SUCCESS/FAILURE`

---

## Commit Strategy

- **Wave 1**: Feature commits with clear scope (`feat(sync): ...`, `fix(sync): ...`)
- **Wave 2**: No commits (verification only)
- **Wave 3**: No commits (production execution)

---

## Success Criteria

### Verification Commands
```bash
# Integration test
go test -run TestSyncIntegration -v ./internal/sync/

# Small repo end-to-end  
./bin/pratc sync --repo=jeffersonnunn/test-repo-small
./bin/pratc analyze --repo=jeffersonnunn/test-repo-small --format=json | jq '.'

# Production verification
./bin/pratc sync --repo=openclaw/openclaw
sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"
./bin/pratc analyze --repo=openclaw/openclaw --format=json | jq '.counts.total_prs'

# Full test suite
make test-go
```

### Final Checklist
- [ ] Integration test demonstrates and fixes the bug
- [ ] PR metadata saved to SQLite during sync
- [ ] Git mirror functionality preserved
- [ ] All existing tests pass
- [ ] Production execution works end-to-end
- [ ] Performance within SLOs for 5.5k PR scale