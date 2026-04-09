package app

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestAnalyzeIncludesReviewPayload(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create service with IncludeReview enabled
	service := NewService(Config{
		Now:           fixedNow,
		IncludeReview: true,
	})

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	// Assert ReviewPayload is not nil
	if response.ReviewPayload == nil {
		t.Fatal("expected ReviewPayload to be populated when IncludeReview is true, got nil")
	}

	// Assert Results slice has entries
	if len(response.ReviewPayload.Results) == 0 {
		t.Fatalf("expected ReviewPayload.Results to have entries, got %d", len(response.ReviewPayload.Results))
	}

	// Assert each result has required fields
	for i, result := range response.ReviewPayload.Results {
		if result.Category == "" {
			t.Fatalf("result[%d]: Category should not be empty", i)
		}
		if result.Confidence < 0 || result.Confidence > 1 {
			t.Fatalf("result[%d]: Confidence should be between 0.0 and 1.0, got %f", i, result.Confidence)
		}
		// Blockers can be empty, but should not be nil (should be empty slice)
		if result.Blockers == nil {
			t.Fatalf("result[%d]: Blockers should not be nil", i)
		}
		// EvidenceReferences can be empty, but should not be nil
		if result.EvidenceReferences == nil {
			t.Fatalf("result[%d]: EvidenceReferences should not be nil", i)
		}
		if result.NextAction == "" {
			t.Fatalf("result[%d]: NextAction should not be empty", i)
		}
	}

	// Assert TotalPRs matches the number of PRs reviewed
	if response.ReviewPayload.TotalPRs != len(response.PRs) {
		t.Fatalf("ReviewPayload.TotalPRs = %d, want %d", response.ReviewPayload.TotalPRs, len(response.PRs))
	}

	// Assert ReviewedPRs is populated
	if response.ReviewPayload.ReviewedPRs == 0 {
		t.Fatal("expected ReviewPayload.ReviewedPRs to be greater than 0")
	}

	// Assert Categories are populated
	if len(response.ReviewPayload.Categories) == 0 {
		t.Fatal("expected ReviewPayload.Categories to have entries")
	}

	// Assert PriorityTiers are populated
	if len(response.ReviewPayload.PriorityTiers) == 0 {
		t.Fatal("expected ReviewPayload.PriorityTiers to have entries")
	}
}

func TestAnalyzeOmitsReviewPayloadWhenIncludeReviewIsFalse(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create service with IncludeReview disabled (default)
	service := NewService(Config{
		Now:           fixedNow,
		IncludeReview: false,
	})

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	// Assert ReviewPayload is nil when IncludeReview is false
	if response.ReviewPayload != nil {
		t.Fatal("expected ReviewPayload to be nil when IncludeReview is false")
	}
}

func TestAnalyzeReviewResultHasValidCategoryAndPriorityTier(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{
		Now:           fixedNow,
		IncludeReview: true,
	})

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if response.ReviewPayload == nil {
		t.Fatal("expected ReviewPayload to be populated")
	}

	// Validate that categories are valid ReviewCategory values
	validCategories := map[types.ReviewCategory]bool{
		types.ReviewCategoryMergeSafe:   true,
		types.ReviewCategoryDuplicate:   true,
		types.ReviewCategoryProblematic: true,
		types.ReviewCategoryNeedsReview: true,
	}

	validPriorityTiers := map[types.PriorityTier]bool{
		types.PriorityTierFastMerge:      true,
		types.PriorityTierReviewRequired: true,
		types.PriorityTierBlocked:        true,
	}

	for i, result := range response.ReviewPayload.Results {
		if !validCategories[result.Category] {
			t.Fatalf("result[%d]: invalid Category %q", i, result.Category)
		}
		if !validPriorityTiers[result.PriorityTier] {
			t.Fatalf("result[%d]: invalid PriorityTier %q", i, result.PriorityTier)
		}
	}
}
