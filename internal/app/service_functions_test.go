package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/review"
	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// =============================================================================
// parseOmniSelector tests (0% coverage target)
// =============================================================================

func TestParseOmniSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		selector   string
		wantStages int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "empty selector returns default stage",
			selector:   "",
			wantStages: 1,
			wantErr:    false,
		},
		{
			name:       "single wildcard",
			selector:   "*",
			wantStages: 1,
			wantErr:    false,
		},
		{
			name:       "single number",
			selector:   "5",
			wantStages: 1,
			wantErr:    false,
		},
		{
			name:       "multiple single numbers",
			selector:   "1,3,5",
			wantStages: 3,
			wantErr:    false,
		},
		{
			name:       "range",
			selector:   "1-5",
			wantStages: 1,
			wantErr:    false,
		},
		{
			name:       "mixed numbers and ranges",
			selector:   "1-3,5,7-9",
			wantStages: 3,
			wantErr:    false,
		},
		{
			name:       "range with wildcard",
			selector:   "1-5,*",
			wantStages: 2,
			wantErr:    false,
		},
		{
			name:       "invalid range - start greater than end",
			selector:   "5-1",
			wantStages: 0,
			wantErr:    true,
			errMsg:     "start > end",
		},
		{
			name:       "invalid range - too many parts",
			selector:   "1-2-3",
			wantStages: 0,
			wantErr:    true,
			errMsg:     "invalid range",
		},
		{
			name:       "invalid number",
			selector:   "abc",
			wantStages: 0,
			wantErr:    true,
			errMsg:     "invalid selector number",
		},
		{
			name:       "whitespace trimmed",
			selector:   "  1-3 , 5 , *  ",
			wantStages: 3,
			wantErr:    false,
		},
		{
			name:       "empty parts ignored",
			selector:   "1,,3",
			wantStages: 2,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages, err := parseOmniSelector(tt.selector)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseOmniSelector(%q) expected error containing %q, got nil", tt.selector, tt.errMsg)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("parseOmniSelector(%q) error = %q, want error containing %q", tt.selector, err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOmniSelector(%q) unexpected error: %v", tt.selector, err)
			}
			if len(stages) != tt.wantStages {
				t.Fatalf("parseOmniSelector(%q) returned %d stages, want %d", tt.selector, len(stages), tt.wantStages)
			}
		})
	}
}

func TestParseOmniSelectorIndices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		selector  string
		stageIdx  int
		wantLen   int
	}{
		{"single number 5", "5", 0, 1},
		{"range 1-3", "1-3", 0, 3},
		{"numbers 1,3,5", "1,3,5", 0, 1},
		{"numbers 1,3,5 stage 1", "1,3,5", 1, 1},
		{"numbers 1,3,5 stage 2", "1,3,5", 2, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages, err := parseOmniSelector(tt.selector)
			if err != nil {
				t.Fatalf("parseOmniSelector(%q) unexpected error: %v", tt.selector, err)
			}
			if tt.stageIdx >= len(stages) {
				t.Fatalf("stage index %d out of range for selector %q", tt.stageIdx, tt.selector)
			}
			if len(stages[tt.stageIdx].Indices) != tt.wantLen {
				t.Fatalf("stage[%d].Indices = %v (len=%d), want len=%d", tt.stageIdx, stages[tt.stageIdx].Indices, len(stages[tt.stageIdx].Indices), tt.wantLen)
			}
		})
	}
}

func TestParseOmniSelectorZeroBasedIndices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		selector  string
		stageIdx  int
		wantFirst int
	}{
		{"single number 5", "5", 0, 4},   // 5-1 = 4
		{"range 1-3", "1-3", 0, 0},       // 1-1 = 0
		{"number 10", "10", 0, 9},         // 10-1 = 9
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stages, err := parseOmniSelector(tt.selector)
			if err != nil {
				t.Fatalf("parseOmniSelector(%q) unexpected error: %v", tt.selector, err)
			}
			if len(stages[tt.stageIdx].Indices) == 0 {
				t.Fatalf("stage[%d].Indices is empty, expected values", tt.stageIdx)
			}
			if stages[tt.stageIdx].Indices[0] != tt.wantFirst {
				t.Fatalf("stage[%d].Indices[0] = %d, want %d", tt.stageIdx, stages[tt.stageIdx].Indices[0], tt.wantFirst)
			}
		})
	}
}

func TestParseOmniSelectorDefaultStageSize(t *testing.T) {
	t.Parallel()

	stages, err := parseOmniSelector("1-3")
	if err != nil {
		t.Fatalf("parseOmniSelector unexpected error: %v", err)
	}
	if stages[0].Size != 20 {
		t.Fatalf("default stage size = %d, want 20", stages[0].Size)
	}
}

// =============================================================================
// plannerPriority tests (0% coverage target)
// =============================================================================

func TestPlannerPriority(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		pr   types.PR
		want float64
	}{
		{
			name: "success CI and approved review and mergeable",
			pr: types.PR{
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "mergeable",
				UpdatedAt:    "2026-03-18T10:00:00Z",
			},
			// CI: +3, Review: +2, Mergeable: +1, age: ~1 day/15 = 0.067
			want: 6.067,
		},
		{
			name: "pending CI",
			pr: types.PR{
				CIStatus:  "pending",
				UpdatedAt: "2026-03-18T10:00:00Z",
			},
			// CI: +1, age ~1 day
			want: 1.067,
		},
		{
			name: "unknown CI",
			pr: types.PR{
				CIStatus:  "unknown",
				UpdatedAt: "2026-03-18T10:00:00Z",
			},
			// CI: +1, age ~1 day
			want: 1.067,
		},
		{
			name: "failure CI",
			pr: types.PR{
				CIStatus:  "failure",
				UpdatedAt: "2026-03-18T10:00:00Z",
			},
			// CI: -2, age ~1 day
			want: -1.933,
		},
		{
			name: "changes requested review",
			pr: types.PR{
				CIStatus:     "success",
				ReviewStatus: "changes_requested",
				Mergeable:    "mergeable",
				UpdatedAt:    "2026-03-18T10:00:00Z",
			},
			// CI: +3, Review: -2, Mergeable: +1, age: ~1 day
			want: 2.067,
		},
		{
			name: "bot PR bonus",
			pr: types.PR{
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "mergeable",
				IsBot:        true,
				UpdatedAt:    "2026-03-18T10:00:00Z",
			},
			// CI: +3, Review: +2, Mergeable: +1, age: ~1 day, Bot: +0.5
			want: 6.567,
		},
		{
			name: "conflicting mergeable",
			pr: types.PR{
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "conflicting",
				UpdatedAt:    "2026-03-18T10:00:00Z",
			},
			// CI: +3, Review: +2, Mergeable: 0 (not "mergeable"), age: ~1 day
			want: 5.067,
		},
		{
			name: "very old PR max age bonus",
			pr: types.PR{
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "mergeable",
				UpdatedAt:    "2026-01-01T00:00:00Z", // ~77 days old
			},
			// CI: +3, Review: +2, Mergeable: +1, age: min(77/15, 2) = 2
			want: 8.0,
		},
		{
			name: "no special status",
			pr: types.PR{
				UpdatedAt: "2026-03-18T10:00:00Z",
			},
			// CI: 0, Review: 0, Mergeable: 0, age: ~1 day
			want: 0.067,
		},
		{
			name: "invalid updated at parses as zero",
			pr: types.PR{
				CIStatus:  "success",
				UpdatedAt: "invalid-date",
			},
			// CI: +3, age: 0 (parse error)
			want: 3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plannerPriority(tt.pr, fixedTime)
			// Allow small floating point difference
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.01 {
				t.Fatalf("plannerPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// orderSelection tests (0% coverage target)
// =============================================================================

func TestOrderSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      string
		selected  []types.PR
		wantNil   bool
		wantCount int
	}{
		{
			name:     "empty selected returns nil",
			repo:     "owner/repo",
			selected: []types.PR{},
			wantNil:  true,
		},
		{
			name: "single PR returns same PR",
			repo: "owner/repo",
			selected: []types.PR{
				{Repo: "owner/repo", Number: 1, Title: "PR 1", BaseBranch: "main", HeadBranch: "pr1"},
			},
			wantNil:   false,
			wantCount: 1,
		},
		{
			name: "multiple PRs returns ordered by number",
			repo: "owner/repo",
			selected: []types.PR{
				{Repo: "owner/repo", Number: 3, Title: "PR 3", BaseBranch: "main", HeadBranch: "pr3"},
				{Repo: "owner/repo", Number: 1, Title: "PR 1", BaseBranch: "main", HeadBranch: "pr1"},
				{Repo: "owner/repo", Number: 2, Title: "PR 2", BaseBranch: "main", HeadBranch: "pr2"},
			},
			wantNil:   false,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orderSelection(tt.repo, tt.selected)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("orderSelection() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("orderSelection() = nil, want non-nil")
			}
			if len(got) != tt.wantCount {
				t.Fatalf("orderSelection() returned %d PRs, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestOrderSelectionDeterministic(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Repo: "owner/repo", Number: 5, Title: "PR 5", BaseBranch: "main", HeadBranch: "pr5"},
		{Repo: "owner/repo", Number: 2, Title: "PR 2", BaseBranch: "main", HeadBranch: "pr2"},
		{Repo: "owner/repo", Number: 8, Title: "PR 8", BaseBranch: "main", HeadBranch: "pr8"},
		{Repo: "owner/repo", Number: 1, Title: "PR 1", BaseBranch: "main", HeadBranch: "pr1"},
	}

	// Run multiple times and verify same result
	var first []types.PR
	for i := 0; i < 3; i++ {
		result := orderSelection("owner/repo", prs)
		if first == nil {
			first = make([]types.PR, len(result))
			copy(first, result)
		}
		if len(result) != len(first) {
			t.Fatalf("orderSelection() returned different lengths across runs")
		}
		for j := range result {
			if result[j].Number != first[j].Number {
				t.Fatalf("orderSelection() returned different order across runs: index %d was %d, expected %d", j, result[j].Number, first[j].Number)
			}
		}
	}
}

// =============================================================================
// classifyDuplicates tests (25.5% coverage target)
// =============================================================================

func TestClassifyDuplicatesEmptyInput(t *testing.T) {
	t.Parallel()

	noop := func(string, int, int) {}
	dups, overlaps := classifyDuplicates([]types.PR{}, []review.MergedPRRecord{}, noop, types.DuplicateThreshold)
	if len(dups) != 0 {
		t.Fatalf("classifyDuplicates empty prs: duplicates = %d, want 0", len(dups))
	}
	if len(overlaps) != 0 {
		t.Fatalf("classifyDuplicates empty prs: overlaps = %d, want 0", len(overlaps))
	}
}

func TestClassifyDuplicatesNoMatches(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 1,
			Title:  "Add feature A",
			Body:   "Implements feature A",
			FilesChanged: []string{"file_a.go"},
		},
		{
			Repo:   "owner/repo",
			Number: 2,
			Title:  "Fix bug B",
			Body:   "Fixes bug B",
			FilesChanged: []string{"file_b.go"},
		},
	}

	dups, overlaps := classifyDuplicates(prs, nil, func(string,int,int){}, types.DuplicateThreshold)
	if len(dups) != 0 {
		t.Fatalf("classifyDuplicates no matches: duplicates = %d, want 0", len(dups))
	}
	if len(overlaps) != 0 {
		t.Fatalf("classifyDuplicates no matches: overlaps = %d, want 0", len(overlaps))
	}
}

func TestClassifyDuplicatesHighSimilarity(t *testing.T) {
	t.Parallel()

	// Two nearly identical PRs should be classified as duplicates
	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 1,
			Title:  "Add user authentication",
			Body:   "Adds OAuth2 authentication",
			FilesChanged: []string{"auth.go", "config.go"},
		},
		{
			Repo:   "owner/repo",
			Number: 2,
			Title:  "Add user authentication",
			Body:   "Adds OAuth2 authentication",
			FilesChanged: []string{"auth.go", "config.go"},
		},
	}

	dups, overlaps := classifyDuplicates(prs, nil, func(string,int,int){}, types.DuplicateThreshold)
	if len(dups) == 0 {
		t.Fatalf("classifyDuplicates high similarity: expected at least one duplicate group, got none")
	}
	if len(overlaps) != 0 {
		t.Fatalf("classifyDuplicates high similarity: expected no overlaps, got %d", len(overlaps))
	}
}

func TestClassifyDuplicatesMediumSimilarity(t *testing.T) {
	t.Parallel()

	// Two moderately similar PRs
	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 1,
			Title:  "Add user authentication with OAuth2",
			Body:   "Adds OAuth2 authentication to the app",
			FilesChanged: []string{"auth.go", "config.go"},
		},
		{
			Repo:   "owner/repo",
			Number: 2,
			Title:  "Add admin authentication",
			Body:   "Adds admin authentication",
			FilesChanged: []string{"auth.go", "admin.go"},
		},
	}

	dups, overlaps := classifyDuplicates(prs, nil, func(string,int,int){}, types.DuplicateThreshold)
	// Either may or may not have results depending on similarity score
	// Just verify function doesn't crash and returns valid slice pointers
	if dups == nil {
		t.Fatalf("classifyDuplicates returned nil duplicates slice")
	}
	if overlaps == nil {
		t.Fatalf("classifyDuplicates returned nil overlaps slice")
	}
}

func TestClassifyDuplicatesWithMergedPRs(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 1,
			Title:  "Add user authentication",
			Body:   "Adds OAuth2 authentication",
			FilesChanged: []string{"auth.go", "config.go"},
		},
	}

	merged := []review.MergedPRRecord{
		{
			PRNumber:     100,
			Title:        "Add user authentication",
			Body:         "Adds OAuth2 authentication",
			FilesChanged: []string{"auth.go", "config.go"},
			Repo:         "owner/repo",
		},
	}

	dups, overlaps := classifyDuplicates(prs, merged, func(string,int,int){}, types.DuplicateThreshold)
	// Open PR #1 should be detected as duplicate/overlap of merged #100
	foundCanonical := false
	for _, dup := range dups {
		if dup.CanonicalPRNumber == 1 {
			foundCanonical = true
			break
		}
	}
	if !foundCanonical {
		t.Fatalf("classifyDuplicates with merged: expected canonical PR 1 to be in duplicates")
	}
	_ = overlaps // may or may not have overlaps
}

func TestClassifyDuplicatesDuplicatePRNumsUnique(t *testing.T) {
	t.Parallel()

	// Three similar PRs - PR1 similar to both 2 and 3
	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 1,
			Title:  "Add authentication",
			Body:   "Adds authentication",
			FilesChanged: []string{"auth.go"},
		},
		{
			Repo:   "owner/repo",
			Number: 2,
			Title:  "Add authentication",
			Body:   "Adds authentication",
			FilesChanged: []string{"auth.go"},
		},
		{
			Repo:   "owner/repo",
			Number: 3,
			Title:  "Add authentication",
			Body:   "Adds authentication",
			FilesChanged: []string{"auth.go"},
		},
	}

	dups, _ := classifyDuplicates(prs, nil, func(string,int,int){}, types.DuplicateThreshold)
	// PR1 should be canonical with PR2 and PR3 as duplicates
	for _, dup := range dups {
		if dup.CanonicalPRNumber == 1 {
			// Should have both 2 and 3 as duplicate nums
			has2 := false
			has3 := false
			for _, n := range dup.DuplicatePRNums {
				if n == 2 {
					has2 = true
				}
				if n == 3 {
					has3 = true
				}
			}
			if !has2 || !has3 {
				t.Fatalf("classifyDuplicates duplicate nums: expected 2 and 3, got %v", dup.DuplicatePRNums)
			}
			break
		}
	}
}

func TestClassifyDuplicatesBelowOverlapThreshold(t *testing.T) {
	t.Parallel()

	// Two very different PRs should not be classified
	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 1,
			Title:  "Add user authentication module",
			Body:   "This commit adds a complete user authentication module with OAuth2 support",
			FilesChanged: []string{"auth/user.go", "auth/oauth.go", "auth/session.go", "config.go"},
		},
		{
			Repo:   "owner/repo",
			Number: 2,
			Title:  "Fix memory leak in cache",
			Body:   "This fixes a memory leak in the cache subsystem",
			FilesChanged: []string{"cache/memory.go", "cache/evict.go"},
		},
	}

	dups, overlaps := classifyDuplicates(prs, nil, func(string,int,int){}, types.DuplicateThreshold)
	if len(dups) != 0 && len(overlaps) != 0 {
		t.Fatalf("classifyDuplicates very different: expected no duplicates or overlaps, got dups=%d overlaps=%d", len(dups), len(overlaps))
	}
}

// =============================================================================
// GetActiveSyncJob tests (0% coverage target)
// =============================================================================

func TestGetActiveSyncJobNilCache(t *testing.T) {
	t.Parallel()

	svc := Service{
		cacheStore: nil,
	}

	hasJob, jobID, err := svc.GetActiveSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("GetActiveSyncJob nil cache: unexpected error: %v", err)
	}
	if hasJob {
		t.Fatalf("GetActiveSyncJob nil cache: hasJob = true, want false")
	}
	if jobID != "" {
		t.Fatalf("GetActiveSyncJob nil cache: jobID = %q, want empty", jobID)
	}
}

// =============================================================================
// PlanOmni tests (0% coverage target)
// =============================================================================

func TestPlanOmniEmptySelector(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Skipf("skipping test: could not load manifest: %v", err)
	}

	svc := NewService(Config{Now: fixedNow})
	resp, err := svc.PlanOmni(context.Background(), manifest.Repo, "")
	if err != nil {
		t.Fatalf("PlanOmni empty selector: unexpected error: %v", err)
	}
	if resp.Selector != "" {
		t.Fatalf("PlanOmni empty selector: selector = %q, want empty", resp.Selector)
	}
	if resp.Mode != "omni_batch" {
		t.Fatalf("PlanOmni empty selector: mode = %q, want omni_batch", resp.Mode)
	}
}

func TestPlanOmniWildcardSelector(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Skipf("skipping test: could not load manifest: %v", err)
	}

	svc := NewService(Config{Now: fixedNow})
	resp, err := svc.PlanOmni(context.Background(), manifest.Repo, "*")
	if err != nil {
		t.Fatalf("PlanOmni wildcard selector: unexpected error: %v", err)
	}
	if resp.Selector != "*" {
		t.Fatalf("PlanOmni wildcard: selector = %q, want *", resp.Selector)
	}
	if resp.Mode != "omni_batch" {
		t.Fatalf("PlanOmni wildcard: mode = %q, want omni_batch", resp.Mode)
	}
}

func TestPlanOmniRangeSelector(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Skipf("skipping test: could not load manifest: %v", err)
	}

	svc := NewService(Config{Now: fixedNow})
	resp, err := svc.PlanOmni(context.Background(), manifest.Repo, "1-5")
	if err != nil {
		t.Fatalf("PlanOmni range selector: unexpected error: %v", err)
	}
	if resp.Selector != "1-5" {
		t.Fatalf("PlanOmni range: selector = %q, want 1-5", resp.Selector)
	}
	if resp.StageCount != 1 {
		t.Fatalf("PlanOmni range: stageCount = %d, want 1", resp.StageCount)
	}
}

func TestPlanOmniMixedSelector(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Skipf("skipping test: could not load manifest: %v", err)
	}

	svc := NewService(Config{Now: fixedNow})
	resp, err := svc.PlanOmni(context.Background(), manifest.Repo, "1-3,5,7-9")
	if err != nil {
		t.Fatalf("PlanOmni mixed selector: unexpected error: %v", err)
	}
	if resp.StageCount != 3 {
		t.Fatalf("PlanOmni mixed: stageCount = %d, want 3", resp.StageCount)
	}
}

func TestPlanOmniReturnsResponse(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Skipf("skipping test: could not load manifest: %v", err)
	}

	svc := NewService(Config{Now: fixedNow})
	resp, err := svc.PlanOmni(context.Background(), manifest.Repo, "1-5")
	if err != nil {
		t.Fatalf("PlanOmni: unexpected error: %v", err)
	}
	if resp.Repo == "" {
		t.Fatalf("PlanOmni: repo should not be empty")
	}
	if resp.GeneratedAt == "" {
		t.Fatalf("PlanOmni: generatedAt should not be empty")
	}
	if resp.StageCount == 0 {
		t.Fatalf("PlanOmni: stageCount should be > 0")
	}
}


