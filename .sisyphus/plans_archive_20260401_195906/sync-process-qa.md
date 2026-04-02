# QA Plan: Sync Process Issues

> **Generated**: 2026-03-31
> **Session**: ses_2bf0d2f9bffe11KfoYGblmzQ4g
> **Status**: CRITICAL ISSUES FOUND - Production execution blocked

---

## TL;DR

**Critical Issues Found**: 3 (2 HIGH, 1 CRITICAL)
**Production Execution**: BLOCKED
**Root Cause**: Sync process creates mirror but never fetches PR data

---

## Issues Summary

| ID | Severity | Title | Task |
|----|----------|-------|------|
| SYNC-001 | CRITICAL | Sync returns completed:true but pull_requests table empty | T4 |
| SYNC-002 | HIGH | Mirror created but PR refs never fetched | T4 |
| SYNC-003 | HIGH | Sync job stuck in 'in_progress' state | T4 |

---

## Issue Details

### SYNC-001: Sync Returns Completed But No Data

**Severity**: CRITICAL

**Symptoms**:
```
$ ./bin/pratc sync --repo=openclaw/openclaw
{"completed":true,"repo":"openclaw/openclaw"}

$ sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"
0
```

**Expected**: After sync completes, `pull_requests` table should have ~5,500 rows

**Actual**: `pull_requests` table is empty

**Impact**: Cannot run any downstream analysis commands

---

### SYNC-002: Mirror Created But PR Refs Never Fetched

**Severity**: HIGH

**Symptoms**:
```
$ ./bin/pratc mirror list
REPO                    PATH                                                    SIZE    LAST_SYNC
openclaw/openclaw       /home/agent/.cache/pratc/repos/openclaw/openclaw.git  77703342        -

$ cd /home/agent/.cache/pratc/repos/openclaw/openclaw.git && git for-each-ref refs/pull/ | wc -l
0
```

**Expected**: Mirror should contain PR refs (refs/pull/*)

**Actual**: Mirror has 48,285 commits but 0 PR refs

**Impact**: Sync never fetched PR data into the mirror

---

### SYNC-003: Sync Job Stuck In 'in_progress' State

**Severity**: HIGH

**Symptoms**:
```
$ sqlite3 ~/.pratc/pratc.db "SELECT id, status, created_at FROM sync_jobs WHERE repo='openclaw/openclaw' ORDER BY created_at DESC LIMIT 1;"
openclaw/openclaw-1774822575053413373|in_progress|2026-03-29T22:16:15Z
```

**Expected**: Completed jobs should have status='completed' or 'failed'

**Actual**: Job has been 'in_progress' since 2026-03-29 (2+ days)

**Impact**: New sync operations may be blocked by stale in_progress job

---

## Verification Commands

```bash
# Check sync job status
sqlite3 ~/.pratc/pratc.db "SELECT * FROM sync_jobs WHERE repo='openclaw/openclaw' ORDER BY created_at DESC LIMIT 1;"

# Check PR count
sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"

# Check mirror PR refs
cd /home/agent/.cache/pratc/repos/openclaw/openclaw.git && git for-each-ref refs/pull/ | wc -l

# Check sync progress
sqlite3 ~/.pratc/pratc.db "SELECT * FROM sync_progress WHERE repo='openclaw/openclaw';"
```

---

## Root Cause Hypothesis

The sync process appears to:
1. ✅ Create git mirror directory
2. ✅ Clone base repository (48k commits)
3. ❌ Fetch PR refs (refs/pull/*) - never happens
4. ❌ Parse PR data from GitHub GraphQL
5. ❌ Insert PRs into SQLite
6. ❌ Mark job as completed
7. ❌ Update sync_jobs status

The sync likely fails silently during the PR ref fetch phase, but the CLI returns success anyway.

---

## Recommended Investigation Steps

1. **Add Debug Logging**: Instrument sync process to log each step
2. **Check GitHub Rate Limits**: `gh api rate_limit`
3. **Test GraphQL Query**: Manually run the PR query to verify it works
4. **Verify PR Ref Format**: GitHub PR refs are `refs/pull/{number}/head`
5. **Add Error Handling**: Ensure CLI returns non-zero exit code on sync failure
6. **Add Progress Reporting**: Real-time updates during sync

---

## Files for Reference

- Evidence: `.sisyphus/evidence/production-openclaw/task-4-sync.txt`
- QA Issues: `.sisyphus/drafts/production-openclaw-qa-issues.md`
- Database: `~/.pratc/pratc.db`
- Mirror: `/home/agent/.cache/pratc/repos/openclaw/openclaw.git`

---

## Next Steps

1. **Fix Sync Process** - Investigate why PR refs are never fetched
2. **Add Validation** - Verify PR count after sync claims completion
3. **Add Error Handling** - Return non-zero exit code on failure
4. **Retry Execution** - After fix, re-run production-openclaw-omni plan

---

*End of QA plan.*
