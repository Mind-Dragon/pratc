# Consolidated Fix Plan: Sync Job Tracking + Cache Wiring + Production

## TL;DR

> **Quick Summary**: Fix the critical sync job tracking bug blocking all production work, verify the cache wiring actually persists PR data end-to-end, then execute production on openclaw/openclaw.
> 
> **Deliverables**:
> - CLI sync creates and tracks jobs properly (fix RegisterSyncCommand)
> - Cache wiring verified: PRs persist to SQLite during sync
> - Full pipeline verified: sync → analyze → cluster → graph → plan → report
> - Production execution on openclaw/openclaw (5,500+ PRs)
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: NO - sequential critical path
> **Critical Path**: Fix job tracking → Verify cache → Run production

---

## Context

### Current Broken State

| Issue | Status | Evidence |
|-------|--------|----------|
| CLI sync passes `("", "")` to newRepoSyncManager | ❌ NOT FIXED | `root.go:630` |
| openclaw sync job stuck `in_progress` | ❌ BLOCKED | Job ID: `openclaw/openclaw-1774822575053413373` |
| opencode-ai sync job stuck `in_progress` | ❌ BLOCKED | Job ID: `opencode-ai/opencode-1774830908064383753` |
| Cache wiring exists but unverified | ⚠️ PARTIAL | Commit `93dc443` exists, Tasks 5-9 not done |
| Report produces minimal PDF | ⚠️ UNVERIFIED | 2.4KB vs expected 100KB+ |

### Root Cause Chain

```
1. RegisterSyncCommand (line 630) passes ("", "") → no job created
2. Job never marked complete/failed
3. Downstream commands see stale "in_progress" job
4. analyze/cluster/graph/report fail or return stale data
```

---

## Work Objectives

### Core Objective
Unblock production by fixing sync job tracking and verifying end-to-end pipeline works.

### Concrete Deliverables
- `internal/cmd/root.go` - RegisterSyncCommand creates and tracks sync jobs
- Verified: 42 PRs saved for opencode-ai/opencode (cache wiring works)
- Full pipeline: sync → analyze → cluster → graph → plan → report
- Production run on openclaw/openclaw (5,500+ PRs)

### Definition of Done
- [ ] `./bin/pratc sync --repo=opencode-ai/opencode` → job created, completed
- [ ] `./bin/pratc analyze --repo=opencode-ai/opencode --format=json` → returns real data
- [ ] `./bin/pratc cluster --repo=opencode-ai/opencode --format=json` → returns clusters
- [ ] `./bin/pratc report --repo=opencode-ai/opencode --format=pdf` → PDF > 100KB
- [ ] `./bin/pratc sync --repo=openclaw/openclaw` → completes successfully
- [ ] SQLite has ~5,500 PRs for openclaw/openclaw

### Must Have
- Sync job tracking fix applied to CLI (not just API)
- End-to-end verification on small repo first
- Full production run on openclaw

### Must NOT Have
- Breaking changes to CLI interface
- External ML API calls (use local)
- Memory leaks or crashes

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: Tests-after (verify existing tests pass)
- **QA Policy**: Every task includes agent-executed verification

---

## Execution Strategy

```
Wave 1 (CRITICAL: Fix sync job tracking — 2 tasks):
├── Task 1: Fix RegisterSyncCommand to create and track jobs
└── Task 2: Verify fix with small repo (opencode-ai/opencode)

Wave 2 (Verify cache wiring — 3 tasks):
├── Task 3: Run full pipeline on opencode-ai/opencode
├── Task 4: Verify report produces comprehensive PDF
└── Task 5: Run existing tests to ensure no regressions

Wave 3 (Production execution — 2 tasks):
├── Task 6: Execute cold sync on openclaw/openclaw
└── Task 7: Run full analysis pipeline on openclaw

Wave FINAL (Verification — 1 task):
└── F1: Final verification and success confirmation
```

Critical Path: T1 → T2 → T3 → T4 → T5 → T6 → T7 → F1

---

## TODOs

- [ ] 1. Fix RegisterSyncCommand to create and track sync jobs

  **What to do**:
  - In `RegisterSyncCommand()` in `internal/cmd/root.go`:
    - Open cache store using `PRATC_DB_PATH` (with default fallback `~/.pratc/pratc.db`)
    - Check for existing in-progress job via `ResumeSyncJob`
    - Create new sync job via `CreateSyncJob(repo)`
    - Pass `dbPath` and `job.ID` to `newRepoSyncManager(dbPath, job.ID)`
    - On success: job is already marked complete by `DefaultRunner.Run()`
    - On failure: mark job failed via `MarkSyncJobFailed`
  - Update output to include `job_id` field

  **Must NOT do**:
  - Don't change the sync logic itself (already working after 93dc443)
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
  - `internal/cmd/root.go:185-222` - startBackgroundSync (pattern to follow for job tracking)
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
    Preconditions: Fresh DB or clean state
    Steps:
      1. rm -f ~/.pratc/pratc.db  # Start fresh
      2. mkdir -p ~/.pratc
      3. ./bin/pratc sync --repo=opencode-ai/opencode
      4. sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs ORDER BY created_at DESC LIMIT 1;"
    Expected Result: Status is "completed", output contains job_id
    Evidence: .sisyphus/evidence/consolidated/task-1-job-tracking.txt

  Scenario: Analyze sees completed sync (not stale in_progress)
    Tool: Bash
    Preconditions: From previous scenario
    Steps:
      1. ./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts.total_prs'
    Expected Result: Returns 42 (not "sync in progress" error)
    Evidence: .sisyphus/evidence/consolidated/task-1-analyze-works.txt
  ```

  **Commit**: YES
  - Message: `fix(cmd): create and track sync jobs in CLI sync command`
  - Files: `internal/cmd/root.go`

  ---

- [ ] 2. Verify fix with small repo (opencode-ai/opencode)

  **What to do**:
  - Run sync on opencode-ai/opencode (42 PRs)
  - Verify job completes successfully
  - Verify PRs saved to SQLite
  - Run analyze to confirm data accessible

  **Must NOT do**:
  - Don't modify code in this task

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Tasks 3-5
  - **Blocked By**: Task 1

  **Acceptance Criteria**:
  - [ ] Sync completes with job_id in output
  - [ ] sqlite3 shows "completed" status
  - [ ] 42 PRs in pull_requests table

  **QA Scenarios**:
  ```
  Scenario: Small repo sync completes successfully
    Tool: Bash
    Steps:
      1. export GH_TOKEN=$(gh auth token)
      2. ./bin/pratc sync --repo=opencode-ai/opencode
      3. sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='opencode-ai/opencode';"
    Expected Result: Count is 42
    Evidence: .sisyphus/evidence/consolidated/task-2-pr-count.txt
  ```

  **Commit**: NO

  ---

- [ ] 3. Run full pipeline on opencode-ai/opencode

  **What to do**:
  - Run analyze → cluster → graph → plan → report
  - Verify each command returns real data
  - Time each operation against SLOs

  **Must NOT do**:
  - Don't modify code

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 4
  - **Blocked By**: Task 2

  **Acceptance Criteria**:
  - [ ] analyze returns valid JSON with counts
  - [ ] cluster returns clusters array
  - [ ] graph returns valid DOT
  - [ ] plan returns ranked selections

  **QA Scenarios**:
  ```
  Scenario: Full pipeline works end-to-end
    Tool: Bash
    Steps:
      1. time ./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts'
      2. time ./bin/pratc cluster --repo=opencode-ai/opencode --format=json | jq '.clusters | length'
      3. time ./bin/pratc graph --repo=opencode-ai/opencode --format=dot | head -3
      4. time ./bin/pratc plan --repo=opencode-ai/opencode --target=10 --format=json | jq '.selected | length'
    Expected Result: All return real data, times within SLOs
    Evidence: .sisyphus/evidence/consolidated/task-3-pipeline.txt
  ```

  **Commit**: NO

  ---

- [ ] 4. Verify report produces comprehensive PDF

  **What to do**:
  - Run report command
  - Check PDF size (> 100KB expected)
  - Check page count (8-12 expected)
  - If minimal, debug wiring to PDFComposer

  **Must NOT do**:
  - Don't skip this verification

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 5
  - **Blocked By**: Task 3

  **Acceptance Criteria**:
  - [ ] Report command exits 0
  - [ ] PDF file > 100KB
  - [ ] PDF has 8-12 pages

  **QA Scenarios**:
  ```
  Scenario: Report generates comprehensive PDF
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/report-test.pdf
      2. ls -la /tmp/report-test.pdf
      3. file /tmp/report-test.pdf
    Expected Result: File size > 100KB, 8-12 pages
    Evidence: .sisyphus/evidence/consolidated/task-4-report.txt
  ```

  **Commit**: NO

  ---

- [ ] 5. Run existing tests to ensure no regressions

  **What to do**:
  - Run `go test ./...`
  - Run `go build ./...`
  - Verify all tests pass

  **Must NOT do**:
  - Don't skip test verification

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 6
  - **Blocked By**: Task 4

  **Acceptance Criteria**:
  - [ ] `go test ./...` passes
  - [ ] `go build ./...` succeeds

  **QA Scenarios**:
  ```
  Scenario: All tests pass
    Tool: Bash
    Steps:
      1. go test ./... 2>&1 | tail -20
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/consolidated/task-5-tests.txt
  ```

  **Commit**: NO

  ---

- [ ] 6. Execute cold sync on openclaw/openclaw

  **What to do**:
  - Run sync on openclaw/openclaw (5,500+ PRs)
  - Monitor progress
  - Expect 15-20 minutes

  **Must NOT do**:
  - Don't interrupt sync

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 7
  - **Blocked By**: Task 5

  **Acceptance Criteria**:
  - [ ] Sync completes
  - [ ] ~5,500 PRs in SQLite

  **QA Scenarios**:
  ```
  Scenario: Production sync completes
    Tool: Bash
    Steps:
      1. export GH_TOKEN=$(gh auth token) && export GITHUB_TOKEN=$GH_TOKEN
      2. time ./bin/pratc sync --repo=openclaw/openclaw
      3. sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"
    Expected Result: ~5500 PRs
    Evidence: .sisyphus/evidence/consolidated/task-6-openclaw-sync.txt
  ```

  **Commit**: NO

  ---

- [ ] 7. Run full analysis pipeline on openclaw

  **What to do**:
  - Run analyze → cluster → graph → plan
  - Measure against SLOs
  - Generate report

  **Must NOT do**:
  - Don't modify code

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: F1
  - **Blocked By**: Task 6

  **Acceptance Criteria**:
  - [ ] All commands complete within SLOs
  - [ ] Report PDF generated

  **QA Scenarios**:
  ```
  Scenario: Full production pipeline
    Tool: Bash
    Steps:
      1. time ./bin/pratc analyze --repo=openclaw/openclaw --format=json > /tmp/openclaw-analyze.json
      2. time ./bin/pratc cluster --repo=openclaw/openclaw --format=json > /tmp/openclaw-cluster.json
      3. time ./bin/pratc graph --repo=openclaw/openclaw --format=dot > /tmp/openclaw-graph.dot
      4. time ./bin/pratc plan --repo=openclaw/openclaw --target=50 --format=json > /tmp/openclaw-plan.json
      5. ./bin/pratc report --repo=openclaw/openclaw --format=pdf --output=openclaw-report.pdf
    Expected Result: All complete, times within SLOs
    Evidence: .sisyphus/evidence/consolidated/task-7-openclaw-pipeline.txt
  ```

  **Commit**: NO

  ---

## Final Verification Wave

- [ ] F1. **Final Success Confirmation** — `oracle`

  Verify all Definition of Done criteria met:
  - Sync jobs complete properly (not stuck in_progress)
  - opencode-ai/analyze returns 42 PRs
  - openclaw sync completes with ~5500 PRs
  - Report PDF > 100KB
  - All SLOs met

  Output: `DoD [N/N] | SLOs [N/N] | VERDICT: SUCCESS/FAILURE`

---

## Commit Strategy

- **Wave 1**: `fix(cmd): create and track sync jobs in CLI sync command` — `internal/cmd/root.go`

---

## Success Criteria

### Verification Commands
```bash
# Sync job tracking
./bin/pratc sync --repo=opencode-ai/opencode  # Output includes job_id
sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs ORDER BY created_at DESC LIMIT 1;"  # → "completed"

# Small repo pipeline
./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts.total_prs'  # → 42

# Report
./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=report.pdf
stat -c%s report.pdf  # → > 100000

# Production
./bin/pratc sync --repo=openclaw/openclaw
sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"  # → ~5500

# Tests
go test ./...
```

### Final Checklist
- [ ] Sync job tracking fixed
- [ ] opencode-ai pipeline verified (42 PRs)
- [ ] Report produces comprehensive PDF
- [ ] openclaw production run succeeds (~5500 PRs)
- [ ] All tests pass
- [ ] SLOs met
