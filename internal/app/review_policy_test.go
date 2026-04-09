package app

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/review"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestMetadataOnlyPRNeverLooksFastMerge(t *testing.T) {
	t.Parallel()

	pr := types.PR{
		Number:       42,
		Title:        "metadata only",
		Body:         "descriptive title and body without code changes",
		ReviewStatus: "",
		CIStatus:     "",
		Mergeable:    "",
	}

	safety := review.ClassifyMergeSafety(pr, nil)
	problem := review.ClassifyProblematicPR(pr)
	tier := review.DeterminePriorityTier(safety, problem)

	if safety.Confidence > 0.35 {
		t.Fatalf("confidence = %.2f, want <= 0.35 for metadata-only PRs", safety.Confidence)
	}
	if tier != types.PriorityTierReviewRequired {
		t.Fatalf("priority tier = %s, want %s", tier, types.PriorityTierReviewRequired)
	}
	if safety.IsSafe && safety.Confidence >= 0.8 {
		t.Fatal("metadata-only PR should not be classified as fast-merge safe")
	}
}
