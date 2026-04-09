package app

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/review"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestMetadataOnlyCapsAtLowTier verifies that PRs with only metadata evidence
// (no diff, no CI, no approval) have confidence capped at 0.35.
func TestMetadataOnlyCapsAtLowTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pr   types.PR
	}{
		{
			name: "empty PR with no evidence",
			pr: types.PR{
				Number:       1,
				Title:        "Empty PR",
				Body:         "",
				FilesChanged: []string{},
				Additions:    0,
				Deletions:    0,
				CIStatus:     "",
				ReviewStatus: "",
				Mergeable:    "true",
				IsDraft:      false,
			},
		},
		{
			name: "metadata only with pending CI",
			pr: types.PR{
				Number:       2,
				Title:        "Pending PR",
				Body:         "Some description",
				FilesChanged: []string{},
				Additions:    0,
				Deletions:    0,
				CIStatus:     "pending",
				ReviewStatus: "",
				Mergeable:    "true",
				IsDraft:      false,
			},
		},
		{
			name: "metadata only with review required",
			pr: types.PR{
				Number:       3,
				Title:        "Needs review",
				Body:         "Description here",
				FilesChanged: []string{},
				Additions:    0,
				Deletions:    0,
				CIStatus:     "",
				ReviewStatus: "review_required",
				Mergeable:    "true",
				IsDraft:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := review.ClassifyMergeSafety(tt.pr, nil)

			if result.Confidence > 0.35 {
				t.Errorf("metadata-only confidence = %.2f, want <= 0.35", result.Confidence)
			}
		})
	}
}

// TestDiffAndCICapsAtMediumTier verifies that PRs with diff and CI evidence
// but no approval have confidence capped at 0.65.
func TestDiffAndCICapsAtMediumTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pr   types.PR
	}{
		{
			name: "diff + CI success - no approval",
			pr: types.PR{
				Number:            10,
				Title:             "Feature with CI",
				Body:              "Has code and passing CI",
				FilesChanged:      []string{"feature.go"},
				Additions:         100,
				Deletions:         20,
				ChangedFilesCount: 1,
				CIStatus:          "success",
				ReviewStatus:      "review_required",
				Mergeable:         "true",
				IsDraft:           false,
			},
		},
		{
			name: "diff + CI passed - no approval",
			pr: types.PR{
				Number:            11,
				Title:             "Bug fix with CI",
				Body:              "Fixes bug with tests passing",
				FilesChanged:      []string{"bugfix.go", "bugfix_test.go"},
				Additions:         50,
				Deletions:         10,
				ChangedFilesCount: 2,
				CIStatus:          "passed",
				ReviewStatus:      "pending",
				Mergeable:         "true",
				IsDraft:           false,
			},
		},
		{
			name: "diff + CI green - no approval",
			pr: types.PR{
				Number:            12,
				Title:             "Refactoring with CI",
				Body:              "Code cleanup with CI passing",
				FilesChanged:      []string{"cleanup.go"},
				Additions:         200,
				Deletions:         150,
				ChangedFilesCount: 1,
				CIStatus:          "green",
				ReviewStatus:      "",
				Mergeable:         "true",
				IsDraft:           false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := review.ClassifyMergeSafety(tt.pr, nil)

			if result.Confidence > 0.65 {
				t.Errorf("diff+CI without approval confidence = %.2f, want <= 0.65", result.Confidence)
			}
		})
	}
}

// TestHighRiskPRCapsAt79 verifies that high-risk PRs (ChangedFilesCount >= 5
// or additions+deletions >= 500) have confidence capped at 0.79.
func TestHighRiskPRCapsAt79(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pr   types.PR
	}{
		{
			name: "high risk - 5 files changed",
			pr: types.PR{
				Number:            20,
				Title:             "Multi-file change",
				Body:              "Changes 5 files",
				FilesChanged:      []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
				Additions:         50,
				Deletions:         25,
				ChangedFilesCount: 5,
				CIStatus:          "success",
				ReviewStatus:      "approved",
				Mergeable:         "true",
				IsDraft:           false,
			},
		},
		{
			name: "high risk - 6 files changed",
			pr: types.PR{
				Number:            21,
				Title:             "Large refactoring",
				Body:              "Changes 6 files",
				FilesChanged:      []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go"},
				Additions:         100,
				Deletions:         50,
				ChangedFilesCount: 6,
				CIStatus:          "success",
				ReviewStatus:      "approved",
				Mergeable:         "true",
				IsDraft:           false,
			},
		},
		{
			name: "high risk - 500 lines changed",
			pr: types.PR{
				Number:       22,
				Title:        "Large feature",
				Body:         "400 additions + 100 deletions = 500",
				FilesChanged: []string{"feature.go"},
				Additions:    400,
				Deletions:    100,
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "true",
				IsDraft:      false,
			},
		},
		{
			name: "high risk - 550 lines changed",
			pr: types.PR{
				Number:       23,
				Title:        "Massive change",
				Body:         "300 additions + 250 deletions = 550",
				FilesChanged: []string{"bigchange.go"},
				Additions:    300,
				Deletions:    250,
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "true",
				IsDraft:      false,
			},
		},
		{
			name: "high risk with all evidence - still capped",
			pr: types.PR{
				Number:            24,
				Title:             "Core system change",
				Body:              "High risk but approved",
				FilesChanged:      []string{"core/a.go", "core/b.go", "core/c.go", "core/d.go", "core/e.go"},
				Additions:         250,
				Deletions:         100,
				ChangedFilesCount: 5,
				CIStatus:          "success",
				ReviewStatus:      "approved",
				Mergeable:         "true",
				IsDraft:           false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := review.ClassifyMergeSafety(tt.pr, nil)

			if result.Confidence > 0.79 {
				t.Errorf("high-risk PR confidence = %.2f, want <= 0.79", result.Confidence)
			}
		})
	}
}

// TestReviewConfidenceByEvidenceTier is the original comprehensive test
// covering all three evidence tiers with priority tier validation.
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
				Number:       100,
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
				Number:            101,
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
				Number:            102,
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
