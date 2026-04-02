# Remediation Plan: Fix Batch Processing and Mirror Integration

## Problem Summary

The current live analysis path (`fetchLivePRs`) fetches PR files sequentially via 6,000+ individual GraphQL calls instead of using the v0.2 git mirror infrastructure. The mirror exists but:
- Is stored in project directory (pollutes workspace)  
- Isn't used by analysis commands
- Fetches refs sequentially instead of in batches
- UI doesn't guide users to sync-first workflow

## Core Objectives

1. **Use git mirror for file content** - Eliminate sequential GraphQL file calls
2. **Proper mirror isolation** - Store mirrors outside project directory  
3. **Wire up v0.2 components** - Connect sync pipeline to analysis flow
4. **Improve UX flow** - Guide users to sync-first, analyze-second workflow

## Phase 1: Mirror Storage and Directory Structure

### Task 1.1: Relocate Mirror Base Directory
**Problem**: Current mirror uses project directory (`./.pratc/repos/`)
**Solution**: Use standard XDG cache directory

```go
// internal/repo/mirror.go - DefaultBaseDir()
func DefaultBaseDir() (string, error) {
    // Check PRATC_CACHE_DIR env var first
    if cacheDir := os.Getenv("PRATC_CACHE_DIR"); cacheDir != "" {
        return filepath.Join(cacheDir, "repos"), nil
    }
    // Fallback to XDG cache
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("resolve home directory: %w", err)
    }
    return filepath.Join(home, ".cache", "pratc", "repos"), nil
}
```

**Acceptance Criteria**:
- [x] `PRATC_CACHE_DIR` environment variable controls mirror location
- [x] Default location is `~/.cache/pratc/repos/` on Unix systems
- [x] Existing mirrors in old location are migrated or ignored (backward compatibility)

### Task 1.2: Add Mirror Lifecycle Management
**Problem**: No cleanup or size management for mirrors
**Solution**: Add `pratc mirror` subcommands

```bash
pratc mirror list          # List all synced repos
pratc mirror info owner/repo  # Show mirror stats (size, last sync, PR count)
pratc mirror prune         # Remove mirrors for repos no longer tracked
pratc mirror clean         # Remove all mirrors (nuclear option)
```

**Acceptance Criteria**:
- [x] `mirror list` shows repo, path, size, last sync timestamp
- [x] `mirror info` shows detailed stats including disk usage
- [x] `mirror prune` removes mirrors for repos not in recent analysis history
- [x] All commands respect `PRATC_CACHE_DIR` location

## Phase 2: Wire Up Sync Pipeline to Analysis

### Task 2.1: Modify Service Layer to Use Cached Data First
**Problem**: `fetchLivePRs` ignores cached sync data
**Solution**: Check mirror + SQLite cache before live fetch

```go
// internal/app/service.go - loadPRs()
func (s Service) loadPRs(ctx context.Context, repo string) ([]types.PR, string, truncationMeta, error) {
    // NEW: Check if we have recent sync data
    if s.allowLive && s.useCacheFirst {
        if cachedPRs, ok := s.tryLoadFromCache(repo); ok && len(cachedPRs) > 0 {
            return s.processCachedPRs(cachedPRs, repo)
        }
    }
    
    // Existing live fetch logic as fallback
    if s.allowLive && s.token != "" {
        // ... existing fetchLivePRs logic
    }
}
```

**Acceptance Criteria**:
- [x] New `--use-cache-first` flag on all CLI commands (default: true)
- [x] Cache lookup checks both SQLite metadata AND git mirror file contents
- [x] Falls back to live fetch only when cache missing/expired
- [x] Cache TTL is configurable via `PRATC_CACHE_TTL` (default: 1 hour)

### Task 2.2: Extract File Content from Git Mirror
**Problem**: Mirror stores refs but analysis needs actual file lists
**Solution**: Implement `GetChangedFiles` method on Mirror

```go
// internal/repo/mirror.go
func (m *Mirror) GetChangedFiles(ctx context.Context, prNumber int) ([]string, error) {
    // Get merge base between PR head and base branch
    baseRef := "refs/heads/main" // TODO: make configurable
    prRef := fmt.Sprintf("refs/pr/%d/head", prNumber)
    
    // Get diff --name-only between merge base and PR head
    out, err := m.runner.Run(ctx, "diff", "--name-only", baseRef+"..."+prRef)
    if err != nil {
        return nil, fmt.Errorf("git diff: %w", err)
    }
    
    files := strings.Split(strings.TrimSpace(string(out)), "\n")
    var result []string
    for _, file := range files {
        if file != "" {
            result = append(result, file)
        }
    }
    return result, nil
}
```

**Acceptance Criteria**:
- [x] `GetChangedFiles` returns list of files changed in PR vs main branch
- [x] Handles PRs against non-main branches (configurable base branch)
- [x] Caches file lists in SQLite to avoid repeated git operations
- [x] Performance: <100ms per PR file list retrieval

### Task 2.3: Replace Sequential File Fetching with Mirror Access
**Problem**: `fetchLivePRs` does 6k+ sequential GraphQL calls
**Solution**: Use mirror for file content when available

```go
// internal/app/service.go - fetchLivePRs() replacement
func (s Service) fetchPRsWithFiles(ctx context.Context, repo string, prNumbers []int) ([]types.PR, error) {
    // Try to get mirror first
    var mirror *repo.Mirror
    if s.mirrorAvailable {
        var err error
        mirror, err = s.openMirror(ctx, repo)
        if err == nil {
            // Use mirror for file content
            return s.fetchFromMirror(ctx, mirror, prNumbers)
        }
    }
    
    // Fallback to sequential GraphQL (existing behavior)
    return s.fetchSequentialGraphQL(ctx, repo, prNumbers)
}
```

**Acceptance Criteria**:
- [x] When mirror exists, file content comes from git diff, not GraphQL
- [x] Fallback preserves existing behavior when mirror unavailable
- [x] Performance improvement: 6k PR analysis completes in <5 minutes vs >60 minutes

## Phase 3: UI/UX Flow Improvements

### Task 3.1: Add Sync Status to Dashboard
**Problem**: Users don't know sync state before analyzing
**Solution**: Show sync status prominently in web dashboard

**Web Changes**:
- Dashboard shows "Last sync: 2 hours ago" or "Never synced"
- Analysis button disabled until sync completed or force-live option selected  
- Sync progress visible during background sync

**Acceptance Criteria**:
- [x] Web API exposes `/api/repos/{owner}/{name}/sync/status`
- [x] Dashboard shows sync timestamp and PR count from last sync
- [x] "Sync Now" button triggers background sync job
- [x] Analysis requires either recent sync or explicit `--live` flag

### Task 3.2: Improve CLI Command Flow
**Problem**: CLI doesn't guide users to proper workflow
**Solution**: Add warnings and suggestions

```bash
# Current: pratc analyze --repo=owner/repo
# Improved:
$ pratc analyze --repo=owner/repo
⚠️  No recent sync data found for owner/repo
   This will make ~6000 API calls to GitHub (slow + rate limit risk)

💡 Recommended workflow:
   1. pratc sync --repo=owner/repo    # One-time setup (5-10 min)
   2. pratc analyze --repo=owner/repo  # Fast analysis using local mirror

❓ Continue with live fetch? [y/N]: 
```

**Acceptance Criteria**:
- [x] CLI warns when no recent sync data exists
- [x] Provides clear recommended workflow steps
- [x] Respects `--force` flag to skip warning
- [x] Shows estimated API call count based on open PR count

### Task 3.3: Add Background Sync Auto-Trigger
**Problem**: Users forget to sync before analysis
**Solution**: Auto-trigger sync for new repos

**Implementation**:
- When `analyze` detects first-time repo with no sync data
- Automatically starts background sync job
- Returns cached empty response immediately  
- Subsequent requests get actual data once sync completes

**Acceptance Criteria**:
- [x] First-time `analyze` on new repo starts background sync
- [x] Immediate response indicates "sync in progress"
- [x] SSE endpoint streams sync progress to web clients
- [x] Subsequent requests block until sync completes (with timeout)

## Phase 4: Performance Optimizations

### Task 4.1: Batch Git Fetch Operations
**Problem**: Mirror fetches refs one-by-one (1000 calls for 1000 PRs)
**Solution**: Batch multiple refspecs per git fetch

```go
// internal/repo/mirror.go - optimized FetchAll
const maxRefsPerFetch = 100

func (m *Mirror) FetchAll(ctx context.Context, openPRs []int, progress func(done, total int)) error {
    refspecs := BuildRefspecs(openPRs)
    total := len(refspecs)
    
    // Process in batches of maxRefsPerFetch
    for i := 0; i < total; i += maxRefsPerFetch {
        end := min(i+maxRefsPerFetch, total)
        batch := refspecs[i:end]
        
        // Single git fetch command with multiple refspecs
        args := []string{"fetch", "--prune", "--filter=blob:none", "origin"}
        for _, refspec := range batch {
            args = append(args, refspec)
        }
        
        if _, err := m.runner.Run(ctx, args...); err != nil {
            return err
        }
        
        if progress != nil {
            progress(end, total)
        }
    }
    return nil
}
```

**Acceptance Criteria**:
- [x] Git fetch operations reduced from N to ceil(N/100)
- [x] Progress reporting still accurate (shows correct completion percentage)
- [x] Performance: 1000 PR sync completes in <2 minutes vs >10 minutes

### Task 4.2: Parallel File Content Extraction
**Problem**: File extraction from mirror is sequential
**Solution**: Use worker pool for parallel git diff operations

```go
// internal/repo/mirror.go - parallel GetChangedFiles
func (m *Mirror) GetChangedFilesBatch(ctx context.Context, prNumbers []int) (map[int][]string, error) {
    sem := make(chan struct{}, 10) // 10 concurrent operations
    results := make(chan fileResult, len(prNumbers))
    var wg sync.WaitGroup
    
    for _, prNum := range prNumbers {
        wg.Add(1)
        go func(num int) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            
            files, err := m.GetChangedFiles(ctx, num)
            results <- fileResult{prNum: num, files: files, err: err}
        }(prNum)
    }
    
    go func() { wg.Wait(); close(results) }()
    
    resultMap := make(map[int][]string)
    for result := range results {
        if result.err != nil {
            return nil, result.err
        }
        resultMap[result.prNum] = result.files
    }
    return resultMap, nil
}
```

**Acceptance Criteria**:
- [x] File extraction uses up to 10 concurrent git operations
- [x] Error handling preserves partial results where possible
- [x] Performance: 1000 PR file extraction completes in <1 minute vs >10 minutes

## Testing Strategy

### Unit Tests
- Mirror directory relocation with various environment configurations
- Batch git fetch with different refspec counts (0, 1, 100, 150, 1000)
- File extraction from git mirror vs GraphQL fallback
- Cache hit/miss scenarios in service layer

### Integration Tests  
- End-to-end sync → analyze workflow with real GitHub repo
- Performance benchmarks: sequential vs batch vs parallel
- CLI UX flow validation with mock stdin/stdout

### E2E Tests
- Web dashboard sync status display and interaction
- Background sync auto-trigger on first-time analysis
- Large repo handling (5000+ PRs) with memory and time limits

## Success Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| 6k PR analysis time | >60 min | <5 min | CLI timing |
| GitHub API calls | ~6130 | ~100 | Rate limit headers |
| Mirror disk usage | N/A | <500MB/1000 PRs | du -sh |
| Sync completion time | N/A | <10 min/1000 PRs | Sync job timing |
| Memory usage | ~2.5GB | <1.5GB | RSS monitoring |

## Rollout Strategy

### Phase 1 (Week 1)
- Tasks 1.1, 1.2: Mirror directory and lifecycle
- Task 4.1: Batch git fetch optimization

### Phase 2 (Week 2)  
- Tasks 2.1, 2.2, 2.3: Wire up sync to analysis
- Task 4.2: Parallel file extraction

### Phase 3 (Week 3)
- Tasks 3.1, 3.2, 3.3: UI/UX improvements
- Comprehensive testing and performance validation

### Backward Compatibility
- All existing CLI flags and APIs preserved
- Old mirror locations automatically migrated
- Live fetch remains available as fallback option
- No breaking changes to existing workflows
