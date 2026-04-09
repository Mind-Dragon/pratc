package app

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/review"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestReviewConfidenceByEvidenceTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pr       types.PR
		wantTier types.PriorityTier
		wantMax  float64
		wantMin  float64
	}{
		{
			name: "metadata only stays low confidence",
			pr: types.PR{
				Number:       1,
				Title:        "metadata only",
				Body:         "standard metadata-only change",
				ReviewStatus: "",
				CIStatus:     "",
				Mergeable:    "",
			},
			wantTier: types.PriorityTierReviewRequired,
			wantMax:  0.35,
		},
		{
			name: "metadata plus ci and diff evidence reaches medium confidence",
			pr: types.PR{
				Number:            2,
				Title:             "diff plus ci",
				Body:              "standard change with diff and CI",
				FilesChanged:      []string{"internal/app/service.go"},
				Additions:         42,
				Deletions:         7,
				ChangedFilesCount: 1,
				CIStatus:          "passed",
				Mergeable:         "mergeable",
				ReviewStatus:      "pending",
			},
			wantTier: types.PriorityTierReviewRequired,
			wantMin:  0.36,
			wantMax:  0.65,
		},
		{
			name: "high-risk pr without runtime evidence does not become fast merge",
			pr: types.PR{
				Number:            3,
				Title:             "large core change",
				Body:              "large risky change that needs more evidence",
				FilesChanged:      []string{"internal/app/service.go", "internal/review/classifier.go", "internal/cmd/root.go", "internal/types/models.go", "web/src/types/api.ts"},
				Additions:         600,
				Deletions:         250,
				ChangedFilesCount: 5,
				CIStatus:          "passed",
				Mergeable:         "mergeable",
				ReviewStatus:      "approved",
			},
			wantTier: types.PriorityTierReviewRequired,
			wantMax:  0.79,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			safety := review.ClassifyMergeSafety(tt.pr, nil)
			problem := review.ClassifyProblematicPR(tt.pr)
			tier := review.DeterminePriorityTier(safety, problem)

			if tier != tt.wantTier {
				t.Fatalf("priority tier = %s, want %s", tier, tt.wantTier)
			}

			if tt.wantMax > 0 && safety.Confidence > tt.wantMax {
				t.Fatalf("confidence = %.2f, want <= %.2f", safety.Confidence, tt.wantMax)
			}
			if tt.wantMin > 0 && safety.Confidence < tt.wantMin {
				t.Fatalf("confidence = %.2f, want >= %.2f", safety.Confidence, tt.wantMin)
			}
		})
	}
}
