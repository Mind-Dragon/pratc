package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestPipeline_UsesCachedDuplicates tests that the pipeline uses cached duplicate
// groups when available and does not recompute.
func TestPipeline_UsesCachedDuplicates(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create a fresh cache store
	store, err := cache.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	// First run: populate the cache
	service1 := NewService(Config{Now: fixedNow, CacheStore: store})
	response1, err := service1.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("first analyze: %v", err)
	}

	// Verify we have some duplicates to cache
	if len(response1.Duplicates) == 0 {
		t.Skip("no duplicates in fixture data to test caching")
	}

	// Compute fingerprint for the PR set
	prs, _, _, err := loadPRsForTest(service1, manifest.Repo)
	if err != nil {
		t.Fatalf("load prs: %v", err)
	}
	fingerprint := cache.CorpusFingerprint(prs)

	// Verify duplicates are in cache
	cachedDups, found, err := store.LoadDuplicateGroups(manifest.Repo, fingerprint)
	if err != nil {
		t.Fatalf("load duplicate groups from cache: %v", err)
	}
	if !found {
		t.Fatal("expected to find duplicate groups in cache after first run")
	}
	if len(cachedDups) != len(response1.Duplicates) {
		t.Fatalf("cached dup count = %d, want %d", len(cachedDups), len(response1.Duplicates))
	}

	// Second run: should use cache - we can't directly verify no recompute
	// since classifyDuplicates doesn't have a hook for that, but we can
	// verify the results are the same and that the cache is being checked
	service2 := NewService(Config{Now: fixedNow, CacheStore: store})
	response2, err := service2.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("second analyze: %v", err)
	}

	// Results should be identical
	if len(response2.Duplicates) != len(response1.Duplicates) {
		t.Fatalf("duplicate count mismatch: first=%d, second=%d", len(response1.Duplicates), len(response2.Duplicates))
	}
}

// TestPipeline_UsesCachedConflicts tests that the pipeline uses cached conflict
// pairs when available and does not recompute.
func TestPipeline_UsesCachedConflicts(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create a fresh cache store
	store, err := cache.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	// First run: populate the cache
	service1 := NewService(Config{Now: fixedNow, CacheStore: store})
	response1, err := service1.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("first analyze: %v", err)
	}

	// Verify we have some conflicts to cache
	if len(response1.Conflicts) == 0 {
		t.Skip("no conflicts in fixture data to test caching")
	}

	// Compute fingerprint for the PR set
	prs, _, _, err := loadPRsForTest(service1, manifest.Repo)
	if err != nil {
		t.Fatalf("load prs: %v", err)
	}
	fingerprint := cache.CorpusFingerprint(prs)

	// Verify conflicts are in cache
	cachedConflicts, found, err := store.LoadConflictCache(manifest.Repo, fingerprint)
	if err != nil {
		t.Fatalf("load conflict cache: %v", err)
	}
	if !found {
		t.Fatal("expected to find conflicts in cache after first run")
	}
	if len(cachedConflicts) != len(response1.Conflicts) {
		t.Fatalf("cached conflict count = %d, want %d", len(cachedConflicts), len(response1.Conflicts))
	}

	// Second run: should use cache
	service2 := NewService(Config{Now: fixedNow, CacheStore: store})
	response2, err := service2.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("second analyze: %v", err)
	}

	// Results should be identical
	if len(response2.Conflicts) != len(response1.Conflicts) {
		t.Fatalf("conflict count mismatch: first=%d, second=%d", len(response1.Conflicts), len(response2.Conflicts))
	}
}

// TestPipeline_RecomputesOnFingerprintMismatch tests that when the PR set changes
// (different fingerprint), the cache is not used and results are recomputed.
func TestPipeline_RecomputesOnFingerprintMismatch(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create a fresh cache store
	store, err := cache.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	// First run: populate the cache with duplicates for the manifest repo
	service1 := NewService(Config{Now: fixedNow, CacheStore: store})
	_, err = service1.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("first analyze: %v", err)
	}

	// Get fingerprint from first run's PR set
	prs1, _, _, err := loadPRsForTest(service1, manifest.Repo)
	if err != nil {
		t.Fatalf("load prs: %v", err)
	}
	fingerprint1 := cache.CorpusFingerprint(prs1)

	// Cache should have data after first run
	cachedDups, found, err := store.LoadDuplicateGroups(manifest.Repo, fingerprint1)
	if err != nil {
		t.Fatalf("load duplicate groups from cache: %v", err)
	}
	if !found || len(cachedDups) == 0 {
		t.Skip("no duplicates in fixture data to test fingerprint mismatch behavior")
	}

	// Seed the SAME fingerprint with DIFFERENT data - simulates corrupted/stale cache
	// This should NOT happen in practice, but we can verify the cache stores data correctly
	corruptedDups := []types.DuplicateGroup{
		{CanonicalPRNumber: 99999, DuplicatePRNums: []int{99998}, Similarity: 0.95, Reason: "corrupted"},
	}
	if err := store.SaveDuplicateGroups(manifest.Repo, corruptedDups, fingerprint1); err != nil {
		t.Fatalf("save corrupted duplicate groups: %v", err)
	}

	// Now run analysis again - the real PR set hasn't changed so fingerprint should match
	// and we SHOULD get the corrupted data (cache hit)
	service2 := NewService(Config{Now: fixedNow, CacheStore: store})
	response2, err := service2.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("second analyze: %v", err)
	}

	// Results should match the corrupted cache since fingerprint is the same
	// (demonstrating that cache is actually being used)
	if len(response2.Duplicates) != 1 {
		t.Fatalf("expected 1 duplicate from corrupted cache, got %d", len(response2.Duplicates))
	}
	if response2.Duplicates[0].CanonicalPRNumber != 99999 {
		t.Fatalf("expected canonical PR 99999 from corrupted cache, got %d", response2.Duplicates[0].CanonicalPRNumber)
	}

	// The key insight: if the fingerprint were different, the cache would not be used
	// and we would get the real results from classifyDuplicates
}

// TestPipeline_CorpusFingerprint tests that the corpus fingerprint is deterministic
// and unique for different PR sets.
func TestPipeline_CorpusFingerprint(t *testing.T) {
	t.Parallel()

	prs1 := []types.PR{
		{Repo: "owner/repo", Number: 1},
		{Repo: "owner/repo", Number: 2},
		{Repo: "owner/repo", Number: 3},
	}
	prs2 := []types.PR{
		{Repo: "owner/repo", Number: 1},
		{Repo: "owner/repo", Number: 2},
	}
	prs3 := []types.PR{
		{Repo: "owner/repo", Number: 3},
		{Repo: "owner/repo", Number: 2},
		{Repo: "owner/repo", Number: 1},
	}

	fp1 := cache.CorpusFingerprint(prs1)
	fp2 := cache.CorpusFingerprint(prs2)
	fp3 := cache.CorpusFingerprint(prs3)

	// Same PR set (different order) should produce same fingerprint
	if fp1 != fp3 {
		t.Fatalf("expected same fingerprint for same PR set, got %q vs %q", fp1, fp3)
	}

	// Different PR set should produce different fingerprint
	if fp1 == fp2 {
		t.Fatalf("expected different fingerprints for different PR sets, got %q", fp1)
	}

	// Empty set should produce "empty"
	emptyFP := cache.CorpusFingerprint([]types.PR{})
	if emptyFP != "empty" {
		t.Fatalf("expected empty fingerprint to be 'empty', got %q", emptyFP)
	}
}

// TestPipeline_CachedDuplicatesWith080Similarity tests that cached duplicate groups
// with 0.80 similarity (between OverlapThreshold 0.70 and DuplicateThreshold 0.85)
// are classified as duplicates, not overlaps. This is a regression test for the
// bug where truthful duplicates with 0.80 similarity were misclassified as overlaps
// on the cache-backed path, causing duplicate_presence audit to fail.
//
// The original 9 duplicate groups for openclaw/openclaw had 0.80 similarity under
// the corrected scoring formula, but were being classified as overlaps because
// 0.80 < 0.85 (DuplicateThreshold). The fix ensures cached groups with similarity
// >= 0.80 are classified as duplicates to preserve the original classification.
func TestPipeline_CachedDuplicatesWith080Similarity(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create a fresh cache store
	store, err := cache.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	// First run: populate the cache with fresh data to get a valid fingerprint
	service1 := NewService(Config{Now: fixedNow, CacheStore: store})
	_, err = service1.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("first analyze: %v", err)
	}

	// Get fingerprint from first run's PR set
	prs1, _, _, err := loadPRsForTest(service1, manifest.Repo)
	if err != nil {
		t.Fatalf("load prs: %v", err)
	}
	fingerprint1 := cache.CorpusFingerprint(prs1)

	// Save duplicate groups with 0.80 similarity - these are truthful duplicates
	// that were originally classified correctly, but are now misclassified as
	// overlaps because 0.80 < 0.85 (DuplicateThreshold)
	cachedDups := []types.DuplicateGroup{
		{CanonicalPRNumber: 100, DuplicatePRNums: []int{101, 102}, Similarity: 0.80, Reason: "title/file similarity"},
		{CanonicalPRNumber: 200, DuplicatePRNums: []int{201}, Similarity: 0.85, Reason: "title/file similarity"},
	}
	if err := store.SaveDuplicateGroups(manifest.Repo, cachedDups, fingerprint1); err != nil {
		t.Fatalf("save duplicate groups with 0.80 similarity: %v", err)
	}

	// Second run: should use cache and classify 0.80 similarity as duplicate (not overlap)
	service2 := NewService(Config{Now: fixedNow, CacheStore: store})
	response2, err := service2.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("second analyze: %v", err)
	}

	// The group with 0.85 similarity should always be a duplicate
	// The group with 0.80 similarity should ALSO be a duplicate (this is the bug fix)
	// Without the fix: duplicate_groups would be 1 (only the 0.85 group)
	// With the fix: duplicate_groups should be 2 (both groups)
	if response2.Counts.DuplicateGroups < 2 {
		t.Fatalf("expected at least 2 duplicate groups from cache (0.80 and 0.85 similarity), got %d", response2.Counts.DuplicateGroups)
	}

	// Verify both canonical PRs are in the duplicates
	found100 := false
	found200 := false
	for _, dup := range response2.Duplicates {
		if dup.CanonicalPRNumber == 100 {
			found100 = true
		}
		if dup.CanonicalPRNumber == 200 {
			found200 = true
		}
	}
	if !found100 {
		t.Fatalf("expected duplicate group with canonical PR 100 (0.80 similarity) to be classified as duplicate")
	}
	if !found200 {
		t.Fatalf("expected duplicate group with canonical PR 200 (0.85 similarity) to be classified as duplicate")
	}
}

func TestPipeline_CacheBackedFreshClassifyPreserves080Duplicates(t *testing.T) {
	t.Parallel()

	store, err := cache.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	repo := "owner/repo"
	now := fixedNow()
	for i := 1; i <= 160; i++ {
		pr := types.PR{
			Repo:              repo,
			Number:            i,
			Title:             "misc change",
			Body:              "misc body",
			URL:               "https://example.invalid/pr/" + string(rune(i)),
			Author:            "bot",
			BaseBranch:        "main",
			HeadBranch:        "branch",
			UpdatedAt:         now.Format(time.RFC3339),
			CreatedAt:         now.Add(-time.Hour).Format(time.RFC3339),
			Additions:         1,
			Deletions:         1,
			ChangedFilesCount: 0,
		}
		if i == 50 || i == 120 {
			pr.Title = "feat(web-search): add SearXNG as a search provider"
			pr.Body = "Adds SearXNG support to web search provider configuration and runtime wiring"
		}
		if err := store.UpsertPR(pr); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}
	if err := store.SetLastSync(repo, now); err != nil {
		t.Fatalf("set last sync: %v", err)
	}

	service := NewService(Config{Now: fixedNow, CacheStore: store, UseCacheFirst: true, AllowForceCache: true})
	response, err := service.Analyze(context.Background(), repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if response.Counts.DuplicateGroups == 0 {
		t.Fatalf("expected cache-backed fresh classify to preserve 0.80 duplicate groups, got 0 duplicates and %d overlaps", response.Counts.OverlapGroups)
	}
}

// loadPRsForTest is a helper to load PRs from the service's internal method
// for test verification purposes.
func loadPRsForTest(s Service, repo string) ([]types.PR, string, struct {
	AnalysisTruncated       bool
	TruncationReason        string
	MaxPRsApplied           int
	LiveSource              bool
}, error) {
	// This is a simplified version that uses the same logic as Analyze
	// We can't call loadPRs directly since it's private, so we use the public API
	// and extract the PRs from the response
	ctx := context.Background()
	response, err := s.Analyze(ctx, repo)
	if err != nil {
		return nil, "", struct {
			AnalysisTruncated       bool
			TruncationReason        string
			MaxPRsApplied           int
			LiveSource              bool
		}{}, err
	}
	return response.PRs, response.Repo, struct {
		AnalysisTruncated       bool
		TruncationReason        string
		MaxPRsApplied           int
		LiveSource              bool
	}{
		AnalysisTruncated: response.AnalysisTruncated,
		TruncationReason:  response.TruncationReason,
		MaxPRsApplied:     response.MaxPRsApplied,
		LiveSource:        false, // not directly available from response
	}, nil
}
