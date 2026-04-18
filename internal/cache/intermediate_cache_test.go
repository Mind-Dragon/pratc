package cache

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestCorpusFingerprint tests that the corpus fingerprint is deterministic and unique
// for different PR sets.
func TestCorpusFingerprint(t *testing.T) {
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

	fp1 := CorpusFingerprint(prs1)
	fp2 := CorpusFingerprint(prs2)
	fp3 := CorpusFingerprint(prs3)

	// Same PR set (different order) should produce same fingerprint
	if fp1 != fp3 {
		t.Fatalf("expected same fingerprint for same PR set, got %q vs %q", fp1, fp3)
	}

	// Different PR set should produce different fingerprint
	if fp1 == fp2 {
		t.Fatalf("expected different fingerprints for different PR sets, got %q", fp1)
	}
}

// TestSaveAndLoadDuplicateGroups tests saving and loading duplicate groups from cache.
func TestSaveAndLoadDuplicateGroups(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	groups := []types.DuplicateGroup{
		{CanonicalPRNumber: 10, DuplicatePRNums: []int{20, 30}, Similarity: 0.95, Reason: "high similarity"},
		{CanonicalPRNumber: 15, DuplicatePRNums: []int{25}, Similarity: 0.88, Reason: "some overlap"},
	}
	fingerprint := "test-fingerprint-abc123"

	if err := store.SaveDuplicateGroups("owner/repo", groups, fingerprint); err != nil {
		t.Fatalf("save duplicate groups: %v", err)
	}

	loaded, found, err := store.LoadDuplicateGroups("owner/repo", fingerprint)
	if err != nil {
		t.Fatalf("load duplicate groups: %v", err)
	}
	if !found {
		t.Fatal("expected to find cached duplicate groups")
	}
	if len(loaded) != len(groups) {
		t.Fatalf("expected %d groups, got %d", len(groups), len(loaded))
	}
	// Check first group
	if loaded[0].CanonicalPRNumber != 10 {
		t.Fatalf("expected canonical PR 10, got %d", loaded[0].CanonicalPRNumber)
	}
	if len(loaded[0].DuplicatePRNums) != 2 {
		t.Fatalf("expected 2 duplicates, got %d", len(loaded[0].DuplicatePRNums))
	}
}

// TestSaveAndLoadConflictCache tests saving and loading conflict pairs from cache.
func TestSaveAndLoadConflictCache(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	conflicts := []types.ConflictPair{
		{SourcePR: 1, TargetPR: 2, ConflictType: "merge_blocking", FilesTouched: []string{"a.go", "b.go"}, Severity: "high", Reason: "2 shared files"},
		{SourcePR: 3, TargetPR: 4, ConflictType: "attention_needed", FilesTouched: []string{"c.go"}, Severity: "low", Reason: "1 shared file"},
	}
	fingerprint := "test-fingerprint-xyz789"

	if err := store.SaveConflictCache("owner/repo", conflicts, fingerprint); err != nil {
		t.Fatalf("save conflict cache: %v", err)
	}

	loaded, found, err := store.LoadConflictCache("owner/repo", fingerprint)
	if err != nil {
		t.Fatalf("load conflict cache: %v", err)
	}
	if !found {
		t.Fatal("expected to find cached conflict pairs")
	}
	if len(loaded) != len(conflicts) {
		t.Fatalf("expected %d conflicts, got %d", len(conflicts), len(loaded))
	}
	if loaded[0].SourcePR != 1 || loaded[0].TargetPR != 2 {
		t.Fatalf("expected first conflict (1,2), got (%d,%d)", loaded[0].SourcePR, loaded[0].TargetPR)
	}
}

// TestSaveAndLoadSubstanceCache tests saving and loading substance scores from cache.
func TestSaveAndLoadSubstanceCache(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	scores := map[int]int{
		1:  85,
		2:  72,
		3:  90,
		10: 65,
	}
	fingerprint := "test-fingerprint-substance"

	if err := store.SaveSubstanceCache("owner/repo", scores, fingerprint); err != nil {
		t.Fatalf("save substance cache: %v", err)
	}

	loaded, found, err := store.LoadSubstanceCache("owner/repo", fingerprint)
	if err != nil {
		t.Fatalf("load substance cache: %v", err)
	}
	if !found {
		t.Fatal("expected to find cached substance scores")
	}
	if len(loaded) != len(scores) {
		t.Fatalf("expected %d scores, got %d", len(scores), len(loaded))
	}
	if loaded[1] != 85 {
		t.Fatalf("expected score for PR 1 to be 85, got %d", loaded[1])
	}
}

// TestCacheInvalidation_FingerprintMismatch tests that cache is invalidated when
// the corpus fingerprint doesn't match.
func TestCacheInvalidation_FingerprintMismatch(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	groups := []types.DuplicateGroup{
		{CanonicalPRNumber: 10, DuplicatePRNums: []int{20}, Similarity: 0.95},
	}
	fingerprint1 := "fingerprint-1"
	fingerprint2 := "fingerprint-2"

	// Save with fingerprint1
	if err := store.SaveDuplicateGroups("owner/repo", groups, fingerprint1); err != nil {
		t.Fatalf("save duplicate groups: %v", err)
	}

	// Should find with same fingerprint
	_, found, err := store.LoadDuplicateGroups("owner/repo", fingerprint1)
	if err != nil {
		t.Fatalf("load with matching fingerprint: %v", err)
	}
	if !found {
		t.Fatal("expected cache hit with matching fingerprint")
	}

	// Should NOT find with different fingerprint
	_, found, err = store.LoadDuplicateGroups("owner/repo", fingerprint2)
	if err != nil {
		t.Fatalf("load with mismatched fingerprint: %v", err)
	}
	if found {
		t.Fatal("expected cache miss with mismatched fingerprint")
	}
}

// TestCacheInvalidation_SameFingerprint tests that cache is found when the same
// corpus fingerprint is used.
func TestCacheInvalidation_SameFingerprint(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	groups := []types.DuplicateGroup{
		{CanonicalPRNumber: 10, DuplicatePRNums: []int{20}, Similarity: 0.95},
	}
	fingerprint := "same-fingerprint"

	// Save
	if err := store.SaveDuplicateGroups("owner/repo", groups, fingerprint); err != nil {
		t.Fatalf("save duplicate groups: %v", err)
	}

	// Load twice - both should succeed
	for i := 0; i < 2; i++ {
		loaded, found, err := store.LoadDuplicateGroups("owner/repo", fingerprint)
		if err != nil {
			t.Fatalf("load attempt %d: %v", i+1, err)
		}
		if !found {
			t.Fatalf("attempt %d: expected cache hit", i+1)
		}
		if len(loaded) != 1 {
			t.Fatalf("attempt %d: expected 1 group, got %d", i+1, len(loaded))
		}
	}
}

// TestCorpusFingerprint_DeterministicHash tests that the same PR set always
// produces the same hash regardless of input order.
func TestCorpusFingerprint_DeterministicHash(t *testing.T) {
	t.Parallel()

	// Different orderings of the same PRs
	prsA := []types.PR{
		{Repo: "owner/repo", Number: 5},
		{Repo: "owner/repo", Number: 1},
		{Repo: "owner/repo", Number: 3},
	}
	prsB := []types.PR{
		{Repo: "owner/repo", Number: 1},
		{Repo: "owner/repo", Number: 3},
		{Repo: "owner/repo", Number: 5},
	}
	prsC := []types.PR{
		{Repo: "owner/repo", Number: 3},
		{Repo: "owner/repo", Number: 5},
		{Repo: "owner/repo", Number: 1},
	}

	fpA := CorpusFingerprint(prsA)
	fpB := CorpusFingerprint(prsB)
	fpC := CorpusFingerprint(prsC)

	if fpA != fpB || fpB != fpC {
		t.Fatalf("fingerprints should be identical for same PR set: A=%q B=%q C=%q", fpA, fpB, fpC)
	}
}

// TestCorpusFingerprint_EmptySet tests fingerprint of empty PR set.
func TestCorpusFingerprint_EmptySet(t *testing.T) {
	t.Parallel()

	var prs []types.PR
	fp := CorpusFingerprint(prs)
	if fp == "" {
		t.Fatal("expected non-empty fingerprint for empty PR set")
	}

	// Empty should differ from non-empty
	prsNonEmpty := []types.PR{{Repo: "owner/repo", Number: 1}}
	fpNonEmpty := CorpusFingerprint(prsNonEmpty)
	if fp == fpNonEmpty {
		t.Fatal("empty and non-empty should have different fingerprints")
	}
}

// TestDuplicateGroupsCache_PreservesJSON tests that complex data structures
// are preserved correctly through JSON serialization.
func TestDuplicateGroupsCache_PreservesJSON(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Create groups with various similarity scores and multiple duplicates
	groups := []types.DuplicateGroup{
		{
			CanonicalPRNumber: 100,
			DuplicatePRNums:   []int{101, 102, 103, 104, 105},
			Similarity:        0.9234,
			Reason:            "title and file overlap",
		},
		{
			CanonicalPRNumber: 200,
			DuplicatePRNums:   []int{201},
			Similarity:        0.8765,
			Reason:            "high title similarity",
		},
	}
	fingerprint := "json-preservation-test"

	if err := store.SaveDuplicateGroups("owner/repo", groups, fingerprint); err != nil {
		t.Fatalf("save duplicate groups: %v", err)
	}

	loaded, _, err := store.LoadDuplicateGroups("owner/repo", fingerprint)
	if err != nil {
		t.Fatalf("load duplicate groups: %v", err)
	}

	// Verify first group has all duplicates preserved
	if len(loaded[0].DuplicatePRNums) != 5 {
		t.Fatalf("expected 5 duplicates, got %d", len(loaded[0].DuplicatePRNums))
	}
	if loaded[0].Similarity != 0.9234 {
		t.Fatalf("expected similarity 0.9234, got %f", loaded[0].Similarity)
	}

	// Verify second group
	if len(loaded[1].DuplicatePRNums) != 1 {
		t.Fatalf("expected 1 duplicate, got %d", len(loaded[1].DuplicatePRNums))
	}
}

// TestConflictCache_PreservesFilesTouched tests that the shared files list
// is correctly preserved through the cache.
func TestConflictCache_PreservesFilesTouched(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	conflicts := []types.ConflictPair{
		{
			SourcePR:      1,
			TargetPR:      2,
			ConflictType:  "merge_blocking",
			FilesTouched:  []string{"internal/a.go", "internal/b.go", "internal/c.go"},
			Severity:      "high",
			Reason:        "3 shared source files",
		},
	}
	fingerprint := "files-touch-test"

	if err := store.SaveConflictCache("owner/repo", conflicts, fingerprint); err != nil {
		t.Fatalf("save conflict cache: %v", err)
	}

	loaded, _, err := store.LoadConflictCache("owner/repo", fingerprint)
	if err != nil {
		t.Fatalf("load conflict cache: %v", err)
	}

	if len(loaded[0].FilesTouched) != 3 {
		t.Fatalf("expected 3 files touched, got %d", len(loaded[0].FilesTouched))
	}
	if loaded[0].FilesTouched[0] != "internal/a.go" {
		t.Fatalf("expected first file 'internal/a.go', got %q", loaded[0].FilesTouched[0])
	}
}

// TestSubstanceCache_AllScoresPreserved tests that all PR scores are preserved.
func TestSubstanceCache_AllScoresPreserved(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Create a map with various scores
	scores := make(map[int]int)
	for i := 1; i <= 100; i++ {
		scores[i] = i * 2 // Scores from 2 to 200
	}
	fingerprint := "all-scores-test"

	if err := store.SaveSubstanceCache("owner/repo", scores, fingerprint); err != nil {
		t.Fatalf("save substance cache: %v", err)
	}

	loaded, _, err := store.LoadSubstanceCache("owner/repo", fingerprint)
	if err != nil {
		t.Fatalf("load substance cache: %v", err)
	}

	if len(loaded) != 100 {
		t.Fatalf("expected 100 scores, got %d", len(loaded))
	}
	// Spot check a few values
	if loaded[1] != 2 {
		t.Fatalf("expected score for PR 1 to be 2, got %d", loaded[1])
	}
	if loaded[50] != 100 {
		t.Fatalf("expected score for PR 50 to be 100, got %d", loaded[50])
	}
	if loaded[100] != 200 {
		t.Fatalf("expected score for PR 100 to be 200, got %d", loaded[100])
	}
}

// TestDuplicateGroupsCache_OverwriteSameFingerprint tests that saving with the
// same fingerprint overwrites the existing cache.
func TestDuplicateGroupsCache_OverwriteSameFingerprint(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	fingerprint := "overwrite-test"
	groups1 := []types.DuplicateGroup{
		{CanonicalPRNumber: 1, DuplicatePRNums: []int{10}, Similarity: 0.9},
	}
	groups2 := []types.DuplicateGroup{
		{CanonicalPRNumber: 2, DuplicatePRNums: []int{20}, Similarity: 0.85},
	}

	// Save first
	if err := store.SaveDuplicateGroups("owner/repo", groups1, fingerprint); err != nil {
		t.Fatalf("save first duplicate groups: %v", err)
	}

	// Overwrite with second
	if err := store.SaveDuplicateGroups("owner/repo", groups2, fingerprint); err != nil {
		t.Fatalf("save second duplicate groups: %v", err)
	}

	// Load should have the second
	loaded, _, err := store.LoadDuplicateGroups("owner/repo", fingerprint)
	if err != nil {
		t.Fatalf("load duplicate groups: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 group after overwrite, got %d", len(loaded))
	}
	if loaded[0].CanonicalPRNumber != 2 {
		t.Fatalf("expected canonical PR 2, got %d", loaded[0].CanonicalPRNumber)
	}
}

// TestLoadDuplicateGroups_NotFound tests that LoadDuplicateGroups returns found=false
// when no cache entry exists for the given repo and fingerprint.
func TestLoadDuplicateGroups_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	_, found, err := store.LoadDuplicateGroups("nonexistent/repo", "any-fingerprint")
	if err != nil {
		t.Fatalf("load from nonexistent repo: %v", err)
	}
	if found {
		t.Fatal("expected not found for nonexistent repo")
	}
}

// TestLoadConflictCache_NotFound tests that LoadConflictCache returns found=false
// when no cache entry exists.
func TestLoadConflictCache_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	_, found, err := store.LoadConflictCache("nonexistent/repo", "any-fingerprint")
	if err != nil {
		t.Fatalf("load from nonexistent repo: %v", err)
	}
	if found {
		t.Fatal("expected not found for nonexistent repo")
	}
}

// TestLoadSubstanceCache_NotFound tests that LoadSubstanceCache returns found=false
// when no cache entry exists.
func TestLoadSubstanceCache_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	_, found, err := store.LoadSubstanceCache("nonexistent/repo", "any-fingerprint")
	if err != nil {
		t.Fatalf("load from nonexistent repo: %v", err)
	}
	if found {
		t.Fatal("expected not found for nonexistent repo")
	}
}

// TestDuplicateGroupsCache_DifferentRepos tests that cache entries are isolated
// by repository.
func TestDuplicateGroupsCache_DifferentRepos(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	fingerprint := "same-fingerprint"
	groups1 := []types.DuplicateGroup{
		{CanonicalPRNumber: 1, DuplicatePRNums: []int{10}, Similarity: 0.9},
	}
	groups2 := []types.DuplicateGroup{
		{CanonicalPRNumber: 100, DuplicatePRNums: []int{200}, Similarity: 0.95},
	}

	// Save for repo1
	if err := store.SaveDuplicateGroups("owner/repo1", groups1, fingerprint); err != nil {
		t.Fatalf("save for repo1: %v", err)
	}

	// Save for repo2
	if err := store.SaveDuplicateGroups("owner/repo2", groups2, fingerprint); err != nil {
		t.Fatalf("save for repo2: %v", err)
	}

	// Load for repo1
	loaded1, _, err := store.LoadDuplicateGroups("owner/repo1", fingerprint)
	if err != nil {
		t.Fatalf("load for repo1: %v", err)
	}
	if loaded1[0].CanonicalPRNumber != 1 {
		t.Fatalf("expected repo1 canonical 1, got %d", loaded1[0].CanonicalPRNumber)
	}

	// Load for repo2
	loaded2, _, err := store.LoadDuplicateGroups("owner/repo2", fingerprint)
	if err != nil {
		t.Fatalf("load for repo2: %v", err)
	}
	if loaded2[0].CanonicalPRNumber != 100 {
		t.Fatalf("expected repo2 canonical 100, got %d", loaded2[0].CanonicalPRNumber)
	}
}

// BenchmarkCorpusFingerprint benchmarks the fingerprint computation.
func BenchmarkCorpusFingerprint(b *testing.B) {
	prs := make([]types.PR, 5000)
	for i := range prs {
		prs[i] = types.PR{
			Repo:   "owner/repo",
			Number: i + 1,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CorpusFingerprint(prs)
	}
}

// BenchmarkSaveDuplicateGroups benchmarks saving duplicate groups.
func BenchmarkSaveDuplicateGroups(b *testing.B) {
	store := newTestStoreForBench(b)
	groups := make([]types.DuplicateGroup, 100)
	for i := range groups {
		dups := make([]int, 10)
		for j := range dups {
			dups[j] = i*100 + j
		}
		groups[i] = types.DuplicateGroup{
			CanonicalPRNumber: i * 100,
			DuplicatePRNums:   dups,
			Similarity:        0.9,
			Reason:            "test",
		}
	}
	fingerprint := CorpusFingerprint([]types.PR{{Repo: "bench/repo", Number: 1}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.SaveDuplicateGroups("bench/repo", groups, fingerprint)
	}
}

// BenchmarkLoadDuplicateGroups benchmarks loading duplicate groups.
func BenchmarkLoadDuplicateGroups(b *testing.B) {
	store := newTestStoreForBench(b)
	groups := make([]types.DuplicateGroup, 100)
	for i := range groups {
		dups := make([]int, 10)
		for j := range dups {
			dups[j] = i*100 + j
		}
		groups[i] = types.DuplicateGroup{
			CanonicalPRNumber: i * 100,
			DuplicatePRNums:   dups,
			Similarity:        0.9,
			Reason:            "test",
		}
	}
	fingerprint := CorpusFingerprint([]types.PR{{Repo: "bench/repo", Number: 1}})
	store.SaveDuplicateGroups("bench/repo", groups, fingerprint)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.LoadDuplicateGroups("bench/repo", fingerprint)
	}
}

func newTestStoreForBench(b *testing.B) *Store {
	path := filepath.Join(b.TempDir(), "bench_cache.db")
	store, err := Open(path)
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	return store
}

// Verify JSON roundtrip manually for debugging
func TestDuplicateGroupJSONRoundtrip(t *testing.T) {
	t.Parallel()

	group := types.DuplicateGroup{
		CanonicalPRNumber: 10,
		DuplicatePRNums:   []int{20, 30, 40},
		Similarity:        0.95,
		Reason:            "test reason",
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var loaded types.DuplicateGroup
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.CanonicalPRNumber != group.CanonicalPRNumber {
		t.Fatalf("canonical mismatch: got %d, want %d", loaded.CanonicalPRNumber, group.CanonicalPRNumber)
	}
	if len(loaded.DuplicatePRNums) != len(group.DuplicatePRNums) {
		t.Fatalf("dup count mismatch: got %d, want %d", len(loaded.DuplicatePRNums), len(group.DuplicatePRNums))
	}
}
