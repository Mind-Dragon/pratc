package review

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestRunSecondPass(t *testing.T) {
	// Helper to create PRData
	makePRData := func(prNumber int, pr types.PR, conflictPairs []types.ConflictPair, staleness *types.StalenessReport) PRData {
		return PRData{
			PR:            pr,
			ConflictPairs: conflictPairs,
			Staleness:    staleness,
		}
	}

	// Helper to create a PR with common defaults
	makePR := func(number int, updates func(*types.PR)) types.PR {
		pr := types.PR{
			Number: number,
			Title:  "Test PR",
			Body:   "Test body",
			Author: "testauthor",
			CIStatus: "success",
			Mergeable: "true",
			IsDraft:   false,
			IsBot:     false,
			UpdatedAt: time.Now().Format(time.RFC3339),
			ChangedFilesCount: 3,
			Additions: 50,
			Deletions: 10,
			ReviewCount: 0,
			CommentCount: 0,
		}
		if updates != nil {
			updates(&pr)
		}
		return pr
	}

	// Helper to create a staleness report
	makeStaleness := func(score float64) *types.StalenessReport {
		return &types.StalenessReport{
			PRNumber: 1,
			Score:    score,
			Reasons:  []string{},
		}
	}

	// Helper to create conflict pairs
	makeConflictPairs := func(count int) []types.ConflictPair {
		pairs := make([]types.ConflictPair, count)
		for i := 0; i < count; i++ {
			pairs[i] = types.ConflictPair{
				SourcePR: 1,
				TargetPR: i + 2,
				FilesTouched: []string{"file.go"},
			}
		}
		return pairs
	}

	tests := []struct {
		name           string
		initialResults []types.ReviewResult
		prDataMap      map[int]PRData
		expected       map[int]struct {
			category           types.ReviewCategory
			reclassifiedFrom   string
			reclassReason      string
			temporalBucket     string
			hasGate17          bool
		}
	}{
		{
			name: "Rule 1: CI failing + recent push + not draft + <3 conflicts → needs_review",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       1,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				1: makePRData(1, makePR(1, func(pr *types.PR) {
					pr.CIStatus = "failure"
					pr.UpdatedAt = time.Now().AddDate(0, 0, -10).Format(time.RFC3339) // 10 days ago
					pr.IsDraft = false
				}), makeConflictPairs(2), nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				1: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "recoverable CI failure with recent activity",
					temporalBucket:   "future",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 1 NOT matched: CI failing but old push (>90d)",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       2,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				2: makePRData(2, makePR(2, func(pr *types.PR) {
					pr.CIStatus = "failure"
					pr.UpdatedAt = time.Now().AddDate(0, 0, -100).Format(time.RFC3339) // 100 days ago
					pr.IsDraft = false
				}), makeConflictPairs(2), nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				2: {
					category:       types.ReviewCategoryProblematicQuarantine,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "abandoned failing PR",
					temporalBucket:   "junk",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 2: Draft + activity + not stale → high_value",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       3,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				3: makePRData(3, makePR(3, func(pr *types.PR) {
					pr.IsDraft = true
					pr.ReviewCount = 2
					pr.CommentCount = 0
				}), nil, makeStaleness(30)), // Not stale
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				3: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "active draft with community engagement",
					temporalBucket:   "future",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 2 NOT matched: Draft but no activity, Mergeable not unknown → permanent_blocker",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       4,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				4: makePRData(4, makePR(4, func(pr *types.PR) {
					pr.IsDraft = true
					pr.ReviewCount = 0
					pr.CommentCount = 0
					pr.Mergeable = "true" // Not "unknown", so Rule 3 doesn't match
				}), nil, makeStaleness(30)),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				4: {
					category:         types.ReviewCategoryUnknownEscalate,
					reclassifiedFrom: "",
					reclassReason:    "permanent_blocker: no recovery path found",
					temporalBucket:   "blocked",
					hasGate17:        false,
				},
			},
		},
		{
			name: "Rule 3: Mergeable unknown + small size + no risk flags → needs_review",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       5,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				5: makePRData(5, makePR(5, func(pr *types.PR) {
					pr.Mergeable = "unknown"
					pr.ChangedFilesCount = 3
					pr.Additions = 20
					pr.Deletions = 5
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				5: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "small PR with unknown mergeability, likely safe",
					temporalBucket:   "future",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 3 NOT matched: Mergeable unknown but too many files → permanent_blocker",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       6,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				6: makePRData(6, makePR(6, func(pr *types.PR) {
					pr.Mergeable = "unknown"
					pr.ChangedFilesCount = 10 // > 5, too large for Rule 3
					pr.Additions = 20
					pr.Deletions = 5
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				6: {
					category:         types.ReviewCategoryUnknownEscalate,
					reclassifiedFrom: "",
					reclassReason:    "permanent_blocker: no recovery path found",
					temporalBucket:   "blocked",
					hasGate17:        false,
				},
			},
		},
		{
			name: "Rule 4: CI failing + old push + no reviews → junk",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       7,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				7: makePRData(7, makePR(7, func(pr *types.PR) {
					pr.CIStatus = "failure"
					pr.UpdatedAt = time.Now().AddDate(0, 0, -100).Format(time.RFC3339)
					pr.ReviewCount = 0
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				7: {
					category:         types.ReviewCategoryProblematicQuarantine,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "abandoned failing PR",
					temporalBucket:   "junk",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 5: >10 conflicts + stale → junk",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       8,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				8: makePRData(8, makePR(8, nil), makeConflictPairs(15), makeStaleness(60)),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				8: {
					category:         types.ReviewCategoryProblematicQuarantine,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "stale PR with excessive conflicts",
					temporalBucket:   "junk",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 5 NOT matched: >10 conflicts but not stale → permanent_blocker",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       9,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				9: makePRData(9, makePR(9, func(pr *types.PR) {
					pr.Mergeable = "true" // Not "unknown", so Rule 3 won't match
				}), makeConflictPairs(15), makeStaleness(30)), // Staleness score < 50, so Rule 5 doesn't match
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				9: {
					category:         types.ReviewCategoryUnknownEscalate,
					reclassifiedFrom: "",
					reclassReason:    "permanent_blocker: no recovery path found",
					temporalBucket:   "blocked",
					hasGate17:        false,
				},
			},
		},
		{
			name: "Rule 6: IsBot → junk",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       10,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				10: makePRData(10, makePR(10, func(pr *types.PR) {
					pr.IsBot = true
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				10: {
					category:         types.ReviewCategoryProblematicQuarantine,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "garbage markers detected",
					temporalBucket:   "junk",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 6: Empty title → junk",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       11,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				11: makePRData(11, makePR(11, func(pr *types.PR) {
					pr.Title = ""
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				11: {
					category:         types.ReviewCategoryProblematicQuarantine,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "garbage markers detected",
					temporalBucket:   "junk",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Rule 6: Empty body → junk",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       12,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				12: makePRData(12, makePR(12, func(pr *types.PR) {
					pr.Body = ""
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				12: {
					category:         types.ReviewCategoryProblematicQuarantine,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "garbage markers detected",
					temporalBucket:   "junk",
					hasGate17:        true,
				},
			},
		},
		{
			name: "No recovery: merge_now category is not processed",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       13,
					Category:       types.ReviewCategoryMergeNow,
					TemporalBucket: "now",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				13: makePRData(13, makePR(13, func(pr *types.PR) {
					pr.CIStatus = "failure" // Would match rule 1
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				13: {
					category:         types.ReviewCategoryMergeNow, // Unchanged
					reclassifiedFrom: "",
					reclassReason:    "",
					temporalBucket:   "now",
					hasGate17:        false,
				},
			},
		},
		{
			name: "No recovery: merge_after_focused_review category is not processed",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       14,
					Category:       types.ReviewCategoryMergeAfterFocusedReview,
					TemporalBucket: "future",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				14: makePRData(14, makePR(14, nil), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				14: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "",
					reclassReason:    "",
					temporalBucket:   "future",
					hasGate17:        false,
				},
			},
		},
		{
			name: "No recovery: duplicate_superseded category is not processed",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       15,
					Category:       types.ReviewCategoryDuplicateSuperseded,
					TemporalBucket: "duplicate",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				15: makePRData(15, makePR(15, nil), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				15: {
					category:         types.ReviewCategoryDuplicateSuperseded,
					reclassifiedFrom: "",
					reclassReason:    "",
					temporalBucket:   "duplicate",
					hasGate17:        false,
				},
			},
		},
		{
			name: "No recovery: problematic_quarantine category is not processed",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       16,
					Category:       types.ReviewCategoryProblematicQuarantine,
					TemporalBucket: "junk",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				16: makePRData(16, makePR(16, nil), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				16: {
					category:         types.ReviewCategoryProblematicQuarantine,
					reclassifiedFrom: "",
					reclassReason:    "",
					temporalBucket:   "junk",
					hasGate17:        false,
				},
			},
		},
		{
			name: "Permanent blocker: no rule matches",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       17,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				17: makePRData(17, makePR(17, func(pr *types.PR) {
					pr.Mergeable = "true"        // Not unknown
					pr.ChangedFilesCount = 20    // Too large for rule 3
					pr.IsDraft = false           // Not a draft
					pr.CIStatus = "success"      // Not failing
				}), makeConflictPairs(5), makeStaleness(40)), // Not stale, < 10 conflicts
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				17: {
					category:         types.ReviewCategoryUnknownEscalate, // Unchanged
					reclassifiedFrom: "",
					reclassReason:    "permanent_blocker: no recovery path found",
					temporalBucket:   "blocked",
					hasGate17:        false, // No gate 17 added
				},
			},
		},
		{
			name: "Deterministic: same input produces same output",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       18,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
				{
					PRNumber:       19,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{
				18: makePRData(18, makePR(18, func(pr *types.PR) {
					pr.CIStatus = "failure"
					pr.UpdatedAt = time.Now().AddDate(0, 0, -10).Format(time.RFC3339)
					pr.IsDraft = false
				}), makeConflictPairs(2), nil),
				19: makePRData(19, makePR(19, func(pr *types.PR) {
					pr.CIStatus = "failure"
					pr.UpdatedAt = time.Now().AddDate(0, 0, -10).Format(time.RFC3339)
					pr.IsDraft = false
				}), makeConflictPairs(2), nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				18: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "recoverable CI failure with recent activity",
					temporalBucket:   "future",
					hasGate17:        true,
				},
				19: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "recoverable CI failure with recent activity",
					temporalBucket:   "future",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Sorting: results sorted by PR number",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       21,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
				},
				{
					PRNumber:       20,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
				},
			},
			prDataMap: map[int]PRData{
				20: makePRData(20, makePR(20, func(pr *types.PR) {
					pr.Mergeable = "unknown" // Set to unknown so Rule 3 matches
				}), nil, nil),
				21: makePRData(21, makePR(21, func(pr *types.PR) {
					pr.Mergeable = "unknown" // Set to unknown so Rule 3 matches
				}), nil, nil),
			},
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				20: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "small PR with unknown mergeability, likely safe",
					temporalBucket:   "future",
					hasGate17:        true,
				},
				21: {
					category:         types.ReviewCategoryMergeAfterFocusedReview,
					reclassifiedFrom: "unknown_escalate",
					reclassReason:    "small PR with unknown mergeability, likely safe",
					temporalBucket:   "future",
					hasGate17:        true,
				},
			},
		},
		{
			name: "Missing PRData: PR skipped gracefully",
			initialResults: []types.ReviewResult{
				{
					PRNumber:       99,
					Category:       types.ReviewCategoryUnknownEscalate,
					TemporalBucket: "blocked",
					Reasons:        []string{"initial"},
				},
			},
			prDataMap: map[int]PRData{}, // Empty map
			expected: map[int]struct {
				category           types.ReviewCategory
				reclassifiedFrom   string
				reclassReason      string
				temporalBucket     string
				hasGate17          bool
			}{
				99: {
					category:         types.ReviewCategoryUnknownEscalate, // Unchanged
					reclassifiedFrom: "",
					reclassReason:    "",
					temporalBucket:   "blocked",
					hasGate17:        false,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy of initial results to avoid mutation between test runs
			results := make([]types.ReviewResult, len(tc.initialResults))
			copy(results, tc.initialResults)

			// Run the second pass
			updated := RunSecondPass(results, tc.prDataMap)

			// Check each expected PR
			for prNum, exp := range tc.expected {
				var found *types.ReviewResult
				for i := range updated {
					if updated[i].PRNumber == prNum {
						found = &updated[i]
						break
					}
				}

				if found == nil {
					t.Errorf("PR %d not found in results", prNum)
					continue
				}

				if found.Category != exp.category {
					t.Errorf("PR %d: expected category %s, got %s", prNum, exp.category, found.Category)
				}

				if found.ReclassifiedFrom != exp.reclassifiedFrom {
					t.Errorf("PR %d: expected reclassifiedFrom %q, got %q", prNum, exp.reclassifiedFrom, found.ReclassifiedFrom)
				}

				if found.ReclassificationReason != exp.reclassReason {
					t.Errorf("PR %d: expected reclassificationReason %q, got %q", prNum, exp.reclassReason, found.ReclassificationReason)
				}

				if found.TemporalBucket != exp.temporalBucket {
					t.Errorf("PR %d: expected temporalBucket %s, got %s", prNum, exp.temporalBucket, found.TemporalBucket)
				}

				hasGate17 := false
				for _, layer := range found.DecisionLayers {
					if layer.Layer == 17 && layer.Name == "Recovery Assessment" {
						hasGate17 = true
						break
					}
				}
				if hasGate17 != exp.hasGate17 {
					t.Errorf("PR %d: expected hasGate17=%v, got %v (layers=%v)", prNum, exp.hasGate17, hasGate17, found.DecisionLayers)
				}
			}
		})
	}
}

func TestIsCIFailing(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"failure", true},
		{"failed", true},
		{"red", true},
		{"error", true},
		{"FAILURE", true},
		{"Failed", true},
		{"success", false},
		{"passed", false},
		{"pending", false},
		{"", false},
	}

	for _, tc := range tests {
		result := isCIFailing(tc.status)
		if result != tc.expected {
			t.Errorf("isCIFailing(%q): expected %v, got %v", tc.status, tc.expected, result)
		}
	}
}

func TestIsMergeableUnknown(t *testing.T) {
	tests := []struct {
		mergeable string
		expected  bool
	}{
		{"unknown", true},
		{"", true},
		{"true", false},
		{"false", false},
		{"mergeable", false},
		{"UNKNOWN", true},
	}

	for _, tc := range tests {
		result := isMergeableUnknown(tc.mergeable)
		if result != tc.expected {
			t.Errorf("isMergeableUnknown(%q): expected %v, got %v", tc.mergeable, tc.expected, result)
		}
	}
}

func TestIsStale(t *testing.T) {
	tests := []struct {
		name     string
		staleness *types.StalenessReport
		expected bool
	}{
		{"nil staleness", nil, false},
		{"score 50", &types.StalenessReport{Score: 50}, false},
		{"score 51", &types.StalenessReport{Score: 51}, true},
		{"score 100", &types.StalenessReport{Score: 100}, true},
		{"score 0", &types.StalenessReport{Score: 0}, false},
	}

	for _, tc := range tests {
		result := isStale(tc.staleness)
		if result != tc.expected {
			t.Errorf("isStale(%+v): expected %v, got %v", tc.staleness, tc.expected, result)
		}
	}
}

func TestHasRiskFlags(t *testing.T) {
	makePR := func(changes int, additions, deletions int, ciStatus string) types.PR {
		return types.PR{
			ChangedFilesCount: changes,
			Additions:        additions,
			Deletions:        deletions,
			CIStatus:         ciStatus,
		}
	}

	tests := []struct {
		name        string
		pr          types.PR
		conflictCnt int
		expected    bool
	}{
		{"no risk", makePR(3, 50, 10, "success"), 2, false},
		{"too many files", makePR(15, 50, 10, "success"), 2, true},
		{"too many changes", makePR(3, 400, 200, "success"), 2, true},
		{"too many conflicts", makePR(3, 50, 10, "success"), 10, true},
		{"failing CI", makePR(3, 50, 10, "failure"), 2, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prData := PRData{
				PR:            tc.pr,
				ConflictPairs: make([]types.ConflictPair, tc.conflictCnt),
			}
			result := hasRiskFlags(tc.pr, prData)
			if result != tc.expected {
				t.Errorf("hasRiskFlags(): expected %v, got %v", tc.expected, result)
			}
		})
	}
}
