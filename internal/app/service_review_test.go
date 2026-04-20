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
		if result.PRNumber == 0 {
			t.Fatalf("result[%d]: PRNumber should be populated", i)
		}
		if result.Title == "" {
			t.Fatalf("result[%d]: Title should be populated", i)
		}
		if result.Author == "" {
			t.Fatalf("result[%d]: Author should be populated", i)
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

func TestAnalyzeIncludesReviewPayloadWhenIncludeReviewIsFalse(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create service with IncludeReview disabled to confirm v1.3 still wires review output.
	service := NewService(Config{
		Now:           fixedNow,
		IncludeReview: false,
	})

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if response.ReviewPayload == nil {
		t.Fatal("expected ReviewPayload to be populated even when IncludeReview is false")
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
		types.ReviewCategoryMergeNow:                true,
		types.ReviewCategoryDuplicateSuperseded:     true,
		types.ReviewCategoryProblematicQuarantine:   true,
		types.ReviewCategoryMergeAfterFocusedReview: true,
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

func TestAnalyzeReviewPayloadIncludesBucketCounts(t *testing.T) {
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

	if len(response.ReviewPayload.Buckets) == 0 {
		t.Fatal("expected ReviewPayload.Buckets to have entries")
	}

	expectedBuckets := map[string]bool{
		"now":       false,
		"future":    false,
		"duplicate": false,
		"junk":      false,
		"blocked":   false,
	}

	for _, bucket := range response.ReviewPayload.Buckets {
		if _, ok := expectedBuckets[bucket.Bucket]; ok {
			expectedBuckets[bucket.Bucket] = true
		}
	}

	for bucket, found := range expectedBuckets {
		if !found {
			t.Errorf("expected bucket %q not found in ReviewPayload.Buckets", bucket)
		}
	}

	totalFromBuckets := 0
	for _, bucket := range response.ReviewPayload.Buckets {
		totalFromBuckets += bucket.Count
	}
	if totalFromBuckets != response.ReviewPayload.ReviewedPRs {
		t.Errorf("bucket counts sum (%d) does not match ReviewedPRs (%d)", totalFromBuckets, response.ReviewPayload.ReviewedPRs)
	}
}

// TestAnalyzePRsEnrichedWithDecisionEngineData verifies that the decision engine
// surfaces bucket/reason/confidence/temporal information directly on each PR
// in the AnalysisResponse.PRs array.
func TestAnalyzePRsEnrichedWithDecisionEngineData(t *testing.T) {
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

	// Verify each PR is enriched with decision engine data
	enrichedCount := 0
	for i, pr := range response.PRs {
		// Find corresponding review result
		var correspondingResult *types.ReviewResult
		for j := range response.ReviewPayload.Results {
			if response.ReviewPayload.Results[j].PRNumber == pr.Number {
				correspondingResult = &response.ReviewPayload.Results[j]
				break
			}
		}

		if correspondingResult == nil {
			// PR not in review results is expected for some edge cases
			continue
		}

		enrichedCount++

		// Verify Confidence is surfaced on the PR
		if pr.Confidence != correspondingResult.Confidence {
			t.Errorf("pr[%d] (#%d): Confidence = %f, want %f",
				i, pr.Number, pr.Confidence, correspondingResult.Confidence)
		}

		// Verify Reasons are surfaced on the PR
		if len(pr.Reasons) != len(correspondingResult.Reasons) {
			t.Errorf("pr[%d] (#%d): Reasons length = %d, want %d",
				i, pr.Number, len(pr.Reasons), len(correspondingResult.Reasons))
		}

		// Verify TemporalBucket is surfaced on the PR
		if pr.TemporalBucket != correspondingResult.TemporalBucket {
			t.Errorf("pr[%d] (#%d): TemporalBucket = %q, want %q",
				i, pr.Number, pr.TemporalBucket, correspondingResult.TemporalBucket)
		}

		// Verify DecisionLayers is surfaced on the PR
		if len(pr.DecisionLayers) != len(correspondingResult.DecisionLayers) {
			t.Errorf("pr[%d] (#%d): DecisionLayers length = %d, want %d",
				i, pr.Number, len(pr.DecisionLayers), len(correspondingResult.DecisionLayers))
		}

		// Verify Category is surfaced on the PR
		if pr.Category != correspondingResult.Category {
			t.Errorf("pr[%d] (#%d): Category = %q, want %q",
				i, pr.Number, pr.Category, correspondingResult.Category)
		}

		// Verify PriorityTier is surfaced on the PR
		if pr.PriorityTier != correspondingResult.PriorityTier {
			t.Errorf("pr[%d] (#%d): PriorityTier = %q, want %q",
				i, pr.Number, pr.PriorityTier, correspondingResult.PriorityTier)
		}

		// Verify SubstanceScore is surfaced on the PR
		if pr.SubstanceScore != correspondingResult.SubstanceScore {
			t.Errorf("pr[%d] (#%d): SubstanceScore = %d, want %d",
				i, pr.Number, pr.SubstanceScore, correspondingResult.SubstanceScore)
		}
	}

	// Verify that at least some PRs were enriched
	if enrichedCount == 0 {
		t.Fatal("expected at least some PRs to be enriched with review data, got 0")
	}

	// Verify the count matches ReviewedPRs
	if enrichedCount != response.ReviewPayload.ReviewedPRs {
		t.Errorf("enrichedCount = %d, want %d (ReviewedPRs)",
			enrichedCount, response.ReviewPayload.ReviewedPRs)
	}
}

// TestAnalyzePRsHaveValidConfidenceRange verifies that all enriched PRs
// have confidence values in the valid range [0.0, 1.0].
func TestAnalyzePRsHaveValidConfidenceRange(t *testing.T) {
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

	for i, pr := range response.PRs {
		if pr.Confidence != 0 && (pr.Confidence < 0 || pr.Confidence > 1) {
			t.Errorf("pr[%d] (#%d): Confidence = %f, must be in [0.0, 1.0]",
				i, pr.Number, pr.Confidence)
		}
	}
}

// TestAnalyzePRsTemporalBucketValid verifies that temporal_bucket field
// contains valid values when review data is present.
func TestAnalyzePRsTemporalBucketValid(t *testing.T) {
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

	validTemporalBuckets := map[string]bool{
		"now":     true,
		"future":  true,
		"blocked": true,
		"":        true, // empty is valid when no review data
	}

	for i, pr := range response.PRs {
		if pr.Confidence > 0 && !validTemporalBuckets[pr.TemporalBucket] {
			t.Errorf("pr[%d] (#%d): TemporalBucket = %q, expected one of [now, future, blocked, \"\"]",
				i, pr.Number, pr.TemporalBucket)
		}
	}
}

// TestEnrichPRsWithReviewDataUnit tests the enrichPRsWithReviewData function directly.
func TestEnrichPRsWithReviewDataUnit(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 1, Title: "PR 1"},
		{Number: 2, Title: "PR 2"},
		{Number: 3, Title: "PR 3"},
	}

	results := []types.ReviewResult{
		{
			PRNumber:        1,
			Confidence:      0.95,
			Reasons:         []string{"reason1", "reason2"},
			SubstanceScore:  75,
			TemporalBucket:  "now",
			Category:        types.ReviewCategoryMergeNow,
			PriorityTier:    types.PriorityTierFastMerge,
			DecisionLayers: []types.DecisionLayer{
				{Layer: 1, Name: "Test", Bucket: "test_bucket"},
			},
		},
		{
			PRNumber:        2,
			Confidence:      0.5,
			Reasons:         []string{"reason3"},
			SubstanceScore:  30,
			TemporalBucket:  "future",
			Category:        types.ReviewCategoryMergeAfterFocusedReview,
			PriorityTier:    types.PriorityTierReviewRequired,
			DecisionLayers: []types.DecisionLayer{
				{Layer: 1, Name: "Test2", Bucket: "test_bucket2"},
			},
		},
	}

	enrichPRsWithReviewData(prs, results)

	// Verify PR 1 is enriched correctly
	if prs[0].Confidence != 0.95 {
		t.Errorf("PR 1: Confidence = %f, want 0.95", prs[0].Confidence)
	}
	if len(prs[0].Reasons) != 2 {
		t.Errorf("PR 1: Reasons length = %d, want 2", len(prs[0].Reasons))
	}
	if prs[0].SubstanceScore != 75 {
		t.Errorf("PR 1: SubstanceScore = %d, want 75", prs[0].SubstanceScore)
	}
	if prs[0].TemporalBucket != "now" {
		t.Errorf("PR 1: TemporalBucket = %q, want 'now'", prs[0].TemporalBucket)
	}
	if prs[0].Category != types.ReviewCategoryMergeNow {
		t.Errorf("PR 1: Category = %q, want %q", prs[0].Category, types.ReviewCategoryMergeNow)
	}
	if prs[0].PriorityTier != types.PriorityTierFastMerge {
		t.Errorf("PR 1: PriorityTier = %q, want %q", prs[0].PriorityTier, types.PriorityTierFastMerge)
	}
	if len(prs[0].DecisionLayers) != 1 {
		t.Errorf("PR 1: DecisionLayers length = %d, want 1", len(prs[0].DecisionLayers))
	}

	// Verify PR 2 is enriched correctly
	if prs[1].Confidence != 0.5 {
		t.Errorf("PR 2: Confidence = %f, want 0.5", prs[1].Confidence)
	}
	if prs[1].TemporalBucket != "future" {
		t.Errorf("PR 2: TemporalBucket = %q, want 'future'", prs[1].TemporalBucket)
	}

	// Verify PR 3 is NOT enriched (no corresponding result)
	if prs[2].Confidence != 0 {
		t.Errorf("PR 3: Confidence = %f, want 0 (no review result)", prs[2].Confidence)
	}
	if prs[2].TemporalBucket != "" {
		t.Errorf("PR 3: TemporalBucket = %q, want ''", prs[2].TemporalBucket)
	}
}
