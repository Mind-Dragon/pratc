# Fix Plan: Sync Job Tracking + Large Repo Strategy

## TL;DR

> **Quick Summary**: Two issues remain after the cache wiring fix: (1) CLI sync command doesn't create/track sync jobs, so "analyze" sees stale status; (2) Need proper strategy for 5k-6k PR repos where git mirror fetch can fail on individual refs.
> 
> **Deliverables**:
> - CLI sync command creates sync job, marks complete/failed
> - Graceful handling of missing PR refs during mirror fetch
> - Verified end-to-end sync → analyze → cluster → plan pipeline
> 
> **Estimated Effort**: Short
> **Parallel Execution**: NO - sequential fixes
> **Critical Path**: Fix sync job tracking → Fix mirror error handling → Verify pipeline

---

## Context

### Current State
- ✅ PR metadata saves to SQLite during sync (42 PRs for opencode-ai/opencode)
- ✅ Git mirror initializes and fetches main branch
- ❌ CLI sync command passes `("", "")` to `newRepoSyncManager` - no job tracking
- ❌ Stale `in_progress` job blocks subsequent analyze commands
- ⚠️ Mirror fetch fails if individual PR refs don't exist (merged/closed PRs)

### Root Cause: No Sync Job Tracking
The CLI `sync` command at line 630 calls `newRepoSyncManager("", "")` which:
1. Creates no sync job record
2. Passes empty jobID → `jobRecorder` never marks complete
3. Analyze command sees stale `in_progress` job from previous attempt
4. User gets "sync in progress" message even after sync completed

### Mirror Fetch Issue
The `FetchAllWithSkipped` method handles individual ref failures but the batch mode can fail on the first batch if the mirror wasn't properly initialized. Now that the mirror exists, this should work for subsequent syncs.

---

## Work Objectives

### Core Objective
Fix CLI sync command to properly track sync jobs so downstream commands see correct status.

### Concrete Deliverables
- `internal/cmd/root.go` - RegisterSyncCommand creates and tracks sync jobs
- Verified: sync → analyze pipeline works end-to-end

### Definition of Done
- [ ] `./bin/pratc sync --repo=opencode-ai/opencode` → completes with job_id in output
- [ ] `sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs ORDER BY created_at DESC LIMIT 1;"` → "completed"
- [ ] `./bin/pratc analyze --repo=opencode-ai/opencode --format=json` → returns real data (not "sync in progress")

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: Tests-after (existing sync tests cover the sync logic)
- **Framework**: Go's `testing` package

### QA Policy
Agent-executed verification after each change.

---

## Execution Strategy

```
Wave 1 (Fix sync job tracking — 1 task):
└── Task 1: Update RegisterSyncCommand to create and track sync jobs

Wave 2 (Verify pipeline — 1 task):
└── Task 2: Run sync → analyze → verify data flows end-to-end

Final Wave (1 review):
└── F1: Code quality and integration review
```

---

## TODOs

- [ ] 1. Update RegisterSyncCommand to create and track sync jobs

  **What to do**:
  - In `RegisterSyncCommand()` in `internal/cmd/root.go`:
    - Open cache store using `PRATC_DB_PATH` (with default fallback)
    - Check for existing in-progress job via `ResumeSyncJob`
    - Create new sync job via `CreateSyncJob(repo)`
    - Pass `dbPath` and `job.ID` to `newRepoSyncManager(dbPath, job.ID)`
    - On success: job is already marked complete by `DefaultRunner.Run()`
    - On failure: mark job failed via `MarkSyncJobFailed`
  - Update output to include `job_id` field

  **Must NOT do**:
  - Don't change the sync logic itself (already working)
  - Don't break `--no-wait` or `--watch` modes

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: NO (first step)
  - **Blocks**: Task 2
  - **Blocked By**: None

  **References**:
  - `internal/cmd/root.go:620-676` - Current RegisterSyncCommand
  - `internal/cmd/root.go:200-222` - startAnalyzeBackgroundSync (pattern to follow)
  - `internal/cache/sqlite.go` - CreateSyncJob, ResumeSyncJob, MarkSyncJobComplete, MarkSyncJobFailed

  **Acceptance Criteria**:
  - [ ] Sync command creates sync job record
  - [ ] Job status updates to "completed" after sync
  - [ ] Job status updates to "failed" on error
  - [ ] Output includes `job_id` field
  - [ ] `go build ./...` → PASS

  **QA Scenarios**:
  ```
  Scenario: Sync creates and completes job
    Tool: Bash
    Steps:
      1. ./bin/pratc sync --repo=opencode-ai/opencode
      2. sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs WHERE repo='opencode-ai/opencode' ORDER BY created_at DESC LIMIT 1;"
      3. Verify output contains "job_id" and status is "completed"
    Expected Result: Job created, completed, output includes job_id
    Evidence: .sisyphus/evidence/sync-fix/task-1-job-tracking.txt

  Scenario: Analyze sees completed sync
    Tool: Bash
    Steps:
      1. ./bin/pratc analyze --repo=opencode-ai/opencode --format=json
      2. Verify output contains real data (not "sync in progress")
    Expected Result: Analyze returns data from cache
    Evidence: .sisyphus/evidence/sync-fix/task-1-analyze-works.txt
  ```

  **Commit**: YES
  - Message: `fix(cmd): create and track sync jobs in CLI sync command`
  - Files: `internal/cmd/root.go`

  ---

- [ ] 2. Verify end-to-end pipeline

  **What to do**:
  - Run full pipeline: sync → analyze → cluster → graph → plan
  - Verify each command returns real data
  - Test with opencode-ai/opencode (42 PRs)

  **Must NOT do**:
  - Don't modify code in this task

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential validation)
  - **Blocks**: F1
  - **Blocked By**: Task 1

  **Acceptance Criteria**:
  - [ ] sync completes with job_id
  - [ ] analyze returns real PR data
  - [ ] cluster returns clusters
  - [ ] graph returns DOT output
  - [ ] plan returns merge plan

  **QA Scenarios**:
  ```
  Scenario: Full pipeline works end-to-end
    Tool: Bash
    Steps:
      1. ./bin/pratc sync --repo=opencode-ai/opencode
      2. ./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts'
      3. ./bin/pratc cluster --repo=opencode-ai/opencode --format=json | jq '.clusters | length'
      4. ./bin/pratc graph --repo=opencode-ai/opencode --format=dot | head -5
      5. ./bin/pratc plan --repo=opencode-ai/opencode --target=10 --format=json | jq '.selected | length'
    Expected Result: All commands return real data
    Evidence: .sisyphus/evidence/sync-fix/task-2-pipeline.txt
  ```

  **Commit**: NO

  ---

## Final Verification Wave

- [ ] F1. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go test ./...`. Review changes for: error handling, no regressions, clean code.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | VERDICT: APPROVE/REJECT`

---

## Commit Strategy

- **Wave 1**: `fix(cmd): create and track sync jobs in CLI sync command` — `internal/cmd/root.go`

---

## Success Criteria

### Verification Commands
```bash
# Sync creates job
./bin/pratc sync --repo=opencode-ai/opencode  # Output includes job_id

# Job is completed
sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs ORDER BY created_at DESC LIMIT 1;"  # → "completed"

# Analyze works
./bin/pratc analyze --repo=opencode-ai/opencode --format=json  # → Real data, not "sync in progress"

# All tests pass
go test ./...
```

### Final Checklist
- [ ] Sync job created and tracked
- [ ] Analyze sees completed sync
- [ ] Full pipeline works
- [ ] All tests pass
