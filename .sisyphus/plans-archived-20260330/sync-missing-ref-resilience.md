# Sync Missing-Ref Resilience Fix

## TL;DR

> **Quick Summary**: Make `FetchAllBatched` resilient to missing git refs so PRs with deleted/force-pushed head branches don't crash the entire sync.
>
> **Deliverables**:
> - `internal/repo/mirror.go`: Individual ref fallback on batch failure + missing-ref skip
> - `internal/repo/mirror_test.go`: Test for missing-ref scenario
> - `internal/sync/worker.go`: Propagate skipped refs in result
>
> **Estimated Effort**: Short (1-2 files, focused change)
> **Parallel Execution**: NO (sequential, single concern)
> **Critical Path**: mirror.go fix → test → verify

---

## Context

### Original Issue
Sync for `openclaw/openclaw` (~140 open PRs) fails on PR #50757 with:
```
fatal: couldn't find remote ref refs/pull/50757/head
```
The PR exists in GitHub's API but its git ref is missing (force-pushed/deleted head branch).

### Impact
- Entire sync aborts, no PRs synced
- User cannot analyze/plan/cluster any PRs from the repo
- Production use blocked for repos with "ghost PRs"

---

## Work Objectives

### Core Objective
Make `FetchAllBatched` skip missing refs instead of failing, so one bad PR doesn't crash the entire sync.

### Concrete Deliverables
- `internal/repo/mirror.go`: On batch fetch failure, fall back to individual ref fetch and skip missing ones
- `internal/repo/mirror_test.go`: Test coverage for missing-ref skip behavior
- `internal/sync/worker.go`: Surface skipped PRs in `SyncResult` for observability

### Definition of Done
- [x] `go test ./internal/repo/... -v` passes with new test
- [x] `go test ./internal/sync/... -v` passes
- [x] Manual test: sync a repo with a known-missing ref completes without error

### Must Have
- Sync completes even if some PR refs are missing
- Skipped PRs are logged/warned so user knows which were skipped
- No regression: successfully-fetched PRs still work correctly

### Must NOT Have
- Don't skip all failures blindly - only skip "ref not found" type errors
- Don't change the Mirror interface significantly
- Don't remove the batch optimization for well-behaved repos

---

## Technical Approach

### Root Cause Location
`internal/repo/mirror.go:185-210` — `FetchAllBatched` does one giant `git fetch` with all refspecs. One missing ref = entire batch fails.

### Fix Strategy

**Phase 1: Per-ref fallback on batch failure**

When `FetchAllBatched` encounters an error:
1. Log the batch failure
2. For each ref in the failed batch, try fetching individually
3. On "remote ref not found" error, log warning and skip
4. On other errors, propagate normally

**Phase 2: Surface skipped refs**

In `SyncResult`, add a `SkippedPRs []int` field that `Worker.SyncJob` populates from the mirror's `FetchAll` result.

### Key Functions to Modify

1. **`FetchAllBatched`** (`mirror.go:185-210`)
   - Add individual-ref fallback on batch failure
   - Detect "not found" vs "real error"
   - Track and return skipped PRs

2. **`Mirror.FetchAll`** (`mirror.go:181-183`)
   - Return `SkippedPRs []int` from the fetch
   - Update interface if needed

3. **`Worker.SyncJob`** (`worker.go:68-74`)
   - Handle skipped PRs from `FetchAll`
   - Include in `SyncResult`

4. **`SyncResult`** (`worker.go:28-34`)
   - Add `SkippedPRs []int` field

### Error Detection
Git error pattern for missing ref:
```
fatal: couldn't find remote ref refs/pull/50757/head
```
Check for `strings.Contains(err.Error(), "couldn't find remote ref")`.

---

## Execution Strategy

```
Wave 1 (Sequential — single concern):
├── Task 1: Modify FetchAllBatched to handle missing refs
├── Task 2: Add test for missing-ref skip behavior
└── Task 3: Surface skipped PRs in SyncResult
    └── Blocked by: Task 1
```

---

## TODOs

- [x] 1. **Modify FetchAllBatched for missing-ref resilience**

  **What to do**:
  - In `FetchAllBatched` (`mirror.go:185-210`): when batch fetch fails, retry each ref individually
  - On "couldn't find remote ref" error, log warning and skip (don't fail)
  - On other errors, return immediately (don't mask real failures)
  - Track skipped PRs in a `skipped map[int]bool` and return them
  - Update `FetchAll` signature to return `skippedPRs []int` alongside error

  **Must NOT do**:
  - Don't change the happy path (batch fetch succeeds → no individual fetches)
  - Don't skip non-"not found" errors
  - Don't change the public `Mirror` interface

  **Recommended Agent Profile**:
  - **Category**: `quick` (single-file, targeted change)
  - **Skills**: none needed
  - **Reason**: Mechanical refactor, clear patterns already exist in this file

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 2
  - **Blocked By**: None

  **References**:
  - `internal/repo/mirror.go:185-210` — Current FetchAllBatched implementation
  - `internal/repo/mirror.go:201` — Where batch failure currently propagates
  - `internal/repo/mirror.go:212-222` — PruneClosedPRs has skip logic to follow

  **Acceptance Criteria**:
  - [ ] `go test ./internal/repo/... -v -run TestFetchAll` passes

  **QA Scenarios**:

  Scenario: Batch fetch succeeds (happy path)
    Tool: Bash
    Preconditions: All PR refs exist
    Steps:
      1. Run existing tests
      2. Verify batch path is still used (no individual fetches)
    Expected Result: All refs fetched, no skipped
    Evidence: test output

  Scenario: Batch fails, one ref missing
    Tool: Bash
    Preconditions: One PR ref missing from remote
    Steps:
      1. Run modified FetchAllBatched
      2. Verify that ref is skipped with warning
      3. Verify remaining refs are fetched
    Expected Result: Skipped ref logged, others fetched
    Evidence: test output

- [x] 2. **Add test for missing-ref skip behavior**

  **What to do**:
  - Add test in `internal/repo/mirror_test.go` for `FetchAll` with a mock that returns "not found" for specific refs
  - Verify skipped refs are returned and other refs are fetched
  - Cover: happy path, partial batch failure, all-fail

  **Must NOT do**:
  - Don't add integration tests (that requires real git remote)
  - Don't change other tests

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: none needed

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 3
  - **Blocked By**: Task 1

  **References**:
  - `internal/repo/mirror_test.go` — Existing test patterns

  **Acceptance Criteria**:
  - [ ] `go test ./internal/repo/... -v -run TestFetchAll` passes
  - [ ] New test covers missing-ref scenario

- [x] 3. **Surface skipped PRs in SyncResult**

  **What to do**:
  - Add `SkippedPRs []int` field to `SyncResult` struct
  - Modify `Mirror.FetchAll` signature to return skipped PRs
  - Update `Worker.SyncJob` to populate `SkippedPRs` from `FetchAll`
  - Add test for skipped PRs in worker test

  **Must NOT do**:
  - Don't break existing callers of `SyncResult` fields
  - Don't change the JSON output contract without version bump

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 1

  **References**:
  - `internal/sync/worker.go:28-34` — SyncResult struct
  - `internal/sync/worker.go:68-74` — Where FetchAll is called

  **Acceptance Criteria**:
  - [ ] `go test ./internal/sync/... -v` passes

---

## Final Verification Wave

- [x] F1: `go build ./...` passes
- [x] F2: `go vet ./...` passes  
- [x] F3: `go test ./internal/repo/... -v` passes
- [x] F4: `go test ./internal/sync/... -v` passes
- [x] F5: Manual test with openclaw/openclaw completes (skips #50757, syncs others)

---

## Success Criteria

```bash
go build ./...     # exit 0
go vet ./...       # exit 0
go test ./internal/repo/... ./internal/sync/...  # all pass
```

And: `pratc sync --repo=openclaw/openclaw` completes without error, with warning about skipped PR #50757.
