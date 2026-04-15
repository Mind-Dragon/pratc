package app

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestBuildReviewBucketsV14Vocabulary validates that buildReviewBuckets
// emits v1.4 bucket labels: now, future, duplicate, junk, blocked.
func TestBuildReviewBucketsV14Vocabulary(t *testing.T) {
	t.Parallel()

	// Seed with some counts across all categories
	categoryCount := map[types.ReviewCategory]int{
		types.ReviewCategoryMergeNow:                3,
		types.ReviewCategoryMergeAfterFocusedReview:  5,
		types.ReviewCategoryDuplicateSuperseded:     2,
		types.ReviewCategoryProblematicQuarantine:   1,
		types.ReviewCategoryUnknownEscalate:         4,
		types.ReviewCategory(""):                  0,
	}

	buckets := buildReviewBuckets(categoryCount)

	// Build a map for easy lookup
	bucketMap := make(map[string]int)
	for _, b := range buckets {
		bucketMap[b.Bucket] = b.Count
	}

	// v1.4 bucket labels
	expectedLabels := []string{"now", "future", "duplicate", "junk", "blocked"}

	for _, label := range expectedLabels {
		if _, ok := bucketMap[label]; !ok {
			t.Errorf("expected bucket label %q not found in output; got buckets: %v", label, buckets)
		}
	}

	// Verify the mapping is correct (not legacy labels)
	legacyLabels := []string{
		"Merge now",
		"Merge after focused review",
		"Duplicate / superseded",
		"Problematic / quarantine",
		"Unknown / escalate",
	}
	for _, label := range legacyLabels {
		if _, ok := bucketMap[label]; ok {
			t.Errorf("found legacy bucket label %q in output; expected v1.4 labels", label)
		}
	}

	// Verify counts map correctly
	if got := bucketMap["now"]; got != 3 {
		t.Errorf("bucketMap[\"now\"] = %d, want 3", got)
	}
	if got := bucketMap["future"]; got != 5 {
		t.Errorf("bucketMap[\"future\"] = %d, want 5", got)
	}
	if got := bucketMap["duplicate"]; got != 2 {
		t.Errorf("bucketMap[\"duplicate\"] = %d, want 2", got)
	}
	if got := bucketMap["junk"]; got != 1 {
		t.Errorf("bucketMap[\"junk\"] = %d, want 1", got)
	}
	if got := bucketMap["blocked"]; got != 4 {
		t.Errorf("bucketMap[\"blocked\"] = %d, want 4", got)
	}
}
