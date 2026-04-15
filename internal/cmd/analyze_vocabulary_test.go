package cmd

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestReviewBucketLabelV14Vocabulary validates that reviewBucketLabel
// emits v1.4 bucket labels: now, future, duplicate, junk, blocked.
func TestReviewBucketLabelV14Vocabulary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		category    types.ReviewCategory
		wantV14Label string
	}{
		{types.ReviewCategoryMergeNow, "now"},
		{types.ReviewCategoryMergeAfterFocusedReview, "future"},
		{types.ReviewCategoryDuplicateSuperseded, "duplicate"},
		{types.ReviewCategoryProblematicQuarantine, "junk"},
		{types.ReviewCategoryUnknownEscalate, "blocked"},
		{types.ReviewCategory(""), "blocked"}, // empty maps to blocked (unknown/escalate)
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			got := reviewBucketLabel(tt.category)
			if got != tt.wantV14Label {
				t.Errorf("reviewBucketLabel(%q) = %q, want %q", tt.category, got, tt.wantV14Label)
			}
		})
	}

	// Also verify legacy labels are NOT returned
	legacyLabels := []string{
		"Merge now",
		"Merge after focused review",
		"Duplicate / superseded",
		"Problematic / quarantine",
		"Unknown / escalate",
	}
	for _, cat := range legacyLabels {
		t.Run("no_legacy_"+cat, func(t *testing.T) {
			// These legacy category strings should not be returned by reviewBucketLabel
			// when using v1.4 vocabulary
			got := reviewBucketLabel(types.ReviewCategory(cat))
			if got == cat {
				t.Errorf("reviewBucketLabel(%q) = %q; should not return legacy label", cat, got)
			}
		})
	}
}
