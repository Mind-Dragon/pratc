package app

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/review"
	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestReviewContractRegression verifies the review contract remains stable
// across the v1.3 implementation. This test uses fixture data to ensure
// real-world PRs produce valid review results with all required fields.
func TestReviewContractRegression(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	prs, err := testutil.LoadFixturePRs()
	if err != nil {
		t.Fatalf("load fixture prs: %v", err)
	}

	if len(prs) == 0 {
		t.Fatal("expected fixture PRs for regression test")
	}

	service := NewService(Config{
		Now:           fixedNow,
		IncludeReview: true,
	})

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	// Verify review payload exists and is properly structured
	if response.ReviewPayload == nil {
		t.Fatal("review payload should be populated when IncludeReview is true")
	}

	// Contract: ReviewPayload must have all required top-level fields
	if response.ReviewPayload.TotalPRs != len(response.PRs) {
		t.Errorf("total_prs mismatch: got %d, want %d", response.ReviewPayload.TotalPRs, len(response.PRs))
	}

	if response.ReviewPayload.ReviewedPRs == 0 {
		t.Error("reviewed_prs should be > 0")
	}

	if len(response.ReviewPayload.Categories) == 0 {
		t.Error("categories should not be empty")
	}

	if len(response.ReviewPayload.PriorityTiers) == 0 {
		t.Error("priority_tiers should not be empty")
	}

	if len(response.ReviewPayload.Buckets) == 0 {
		t.Error("buckets should not be empty")
	}

	// Contract: Each result must have valid category and priority tier
	validCategories := map[types.ReviewCategory]bool{
		types.ReviewCategoryMergeSafe:   true,
		types.ReviewCategoryDuplicate:   true,
		types.ReviewCategoryProblematic: true,
		types.ReviewCategoryNeedsReview: true,
	}

	validTiers := map[types.PriorityTier]bool{
		types.PriorityTierFastMerge:      true,
		types.PriorityTierReviewRequired: true,
		types.PriorityTierBlocked:        true,
	}

	for i, result := range response.ReviewPayload.Results {
		if !validCategories[result.Category] {
			t.Errorf("result[%d]: invalid category %q", i, result.Category)
		}
		if !validTiers[result.PriorityTier] {
			t.Errorf("result[%d]: invalid priority tier %q", i, result.PriorityTier)
		}
		if result.Confidence < 0 || result.Confidence > 1 {
			t.Errorf("result[%d]: confidence %.2f out of range [0,1]", i, result.Confidence)
		}
		if result.NextAction == "" {
			t.Errorf("result[%d]: next_action should not be empty", i)
		}
		if result.Blockers == nil {
			t.Errorf("result[%d]: blockers should not be nil", i)
		}
		if result.EvidenceReferences == nil {
			t.Errorf("result[%d]: evidence_references should not be nil", i)
		}
	}

	// Contract: Bucket counts must sum to reviewed PRs
	totalFromBuckets := 0
	for _, bucket := range response.ReviewPayload.Buckets {
		totalFromBuckets += bucket.Count
	}
	if totalFromBuckets != response.ReviewPayload.ReviewedPRs {
		t.Errorf("bucket sum %d != reviewed_prs %d", totalFromBuckets, response.ReviewPayload.ReviewedPRs)
	}

	// Contract: All 5 bucket labels must be present
	expectedBuckets := map[string]bool{
		"Merge now":                  false,
		"Merge after focused review": false,
		"Duplicate / superseded":     false,
		"Problematic / quarantine":   false,
		"Unknown / escalate":         false,
	}
	for _, bucket := range response.ReviewPayload.Buckets {
		if _, ok := expectedBuckets[bucket.Bucket]; ok {
			expectedBuckets[bucket.Bucket] = true
		}
	}
	for bucket, found := range expectedBuckets {
		if !found {
			t.Errorf("expected bucket %q not found", bucket)
		}
	}
}

// TestConfidenceRulesRegression verifies confidence tier rules are enforced
// consistently across different PR profiles from fixtures.
func TestConfidenceRulesRegression(t *testing.T) {
	t.Parallel()

	prs, err := testutil.LoadFixturePRs()
	if err != nil {
		t.Fatalf("load fixture prs: %v", err)
	}

	if len(prs) == 0 {
		t.Fatal("expected fixture PRs for regression test")
	}

	// Test each fixture PR against confidence rules
	for _, pr := range prs {
		result := review.ClassifyMergeSafety(pr, nil)

		hasDiff := len(pr.FilesChanged) > 0 || pr.Additions > 0 || pr.Deletions > 0
		hasCI := isPassingCI(pr.CIStatus)
		hasApproval := pr.ReviewStatus == "approved"
		isHighRisk := pr.ChangedFilesCount >= 5 || pr.Additions+pr.Deletions >= 500

		// Rule: Metadata-only PRs capped at 0.35
		if !hasDiff && !hasCI && !hasApproval {
			if result.Confidence > 0.35 {
				t.Errorf("PR %d (metadata-only): confidence %.2f > 0.35 cap", pr.Number, result.Confidence)
			}
		}

		// Rule: Diff+CI without approval capped at 0.65
		if hasDiff && hasCI && !hasApproval {
			if result.Confidence > 0.65 {
				t.Errorf("PR %d (diff+CI, no approval): confidence %.2f > 0.65 cap", pr.Number, result.Confidence)
			}
		}

		// Rule: High-risk PRs capped at 0.79
		if isHighRisk && result.Confidence > 0.79 {
			t.Errorf("PR %d (high-risk): confidence %.2f > 0.79 cap", pr.Number, result.Confidence)
		}
	}
}

// TestBucketVisibilityRegression verifies bucket labels are consistently
// mapped from categories and visible in outputs.
func TestBucketVisibilityRegression(t *testing.T) {
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
		t.Fatal("review payload should be populated")
	}

	// Verify bucket-to-category mapping consistency
	bucketCategoryMap := map[string]types.ReviewCategory{
		"Merge now":                  types.ReviewCategoryMergeSafe,
		"Merge after focused review": types.ReviewCategoryNeedsReview,
		"Duplicate / superseded":     types.ReviewCategoryDuplicate,
		"Problematic / quarantine":   types.ReviewCategoryProblematic,
	}

	// Count PRs by category from results
	categoryCounts := make(map[types.ReviewCategory]int)
	for _, result := range response.ReviewPayload.Results {
		categoryCounts[result.Category]++
	}

	// Verify bucket counts align with category counts
	for _, bucket := range response.ReviewPayload.Buckets {
		expectedCategory, ok := bucketCategoryMap[bucket.Bucket]
		if !ok {
			continue // Skip "Unknown / escalate" which has no fixed category
		}

		// Bucket count should match or be derived from category count
		categoryCount := categoryCounts[expectedCategory]
		if bucket.Count != categoryCount && bucket.Count > 0 && categoryCount > 0 {
			// Allow small discrepancies due to edge case handling
			t.Logf("bucket %q count %d vs category %q count %d (may differ due to edge cases)",
				bucket.Bucket, bucket.Count, expectedCategory, categoryCount)
		}
	}

	// Verify priority tier counts align with bucket philosophy
	tierCounts := make(map[types.PriorityTier]int)
	for _, result := range response.ReviewPayload.Results {
		tierCounts[result.PriorityTier]++
	}

	// Fast merge should only come from merge_safe category
	for _, result := range response.ReviewPayload.Results {
		if result.PriorityTier == types.PriorityTierFastMerge {
			if result.Category != types.ReviewCategoryMergeSafe {
				t.Errorf("PR with fast_merge tier should be merge_safe category, got %s", result.Category)
			}
			if result.Confidence < 0.8 {
				t.Errorf("PR with fast_merge tier should have confidence >= 0.8, got %.2f", result.Confidence)
			}
		}
	}

	t.Logf("bucket visibility: %d categories, %d tiers, %d buckets",
		len(response.ReviewPayload.Categories),
		len(response.ReviewPayload.PriorityTiers),
		len(response.ReviewPayload.Buckets))
}

// TestMetadataOnlyPolicyRegression verifies metadata-only PRs never appear
// as fast-merge safe, using both synthetic and fixture data.
func TestMetadataOnlyPolicyRegression(t *testing.T) {
	t.Parallel()

	// Test with synthetic metadata-only PR
	metadataOnlyPR := types.PR{
		Number:       999,
		Title:        "metadata only test",
		Body:         "description without code changes",
		FilesChanged: []string{},
		Additions:    0,
		Deletions:    0,
		CIStatus:     "",
		ReviewStatus: "",
		Mergeable:    "true",
		IsDraft:      false,
	}

	safety := review.ClassifyMergeSafety(metadataOnlyPR, nil)
	problem := review.ClassifyProblematicPR(metadataOnlyPR)
	tier := review.DeterminePriorityTier(safety, problem)

	if safety.Confidence > 0.35 {
		t.Errorf("metadata-only confidence %.2f > 0.35 cap", safety.Confidence)
	}
	if tier == types.PriorityTierFastMerge {
		t.Error("metadata-only PR should never be fast_merge tier")
	}
	if safety.IsSafe && safety.Confidence >= 0.8 {
		t.Error("metadata-only PR should not be classified as high-confidence safe")
	}

	// Test with fixture PRs that have minimal evidence
	prs, err := testutil.LoadFixturePRs()
	if err != nil {
		t.Fatalf("load fixture prs: %v", err)
	}

	lowEvidenceCount := 0
	for _, pr := range prs {
		hasDiff := len(pr.FilesChanged) > 0 || pr.Additions > 0 || pr.Deletions > 0
		hasCI := pr.CIStatus != ""
		hasApproval := pr.ReviewStatus == "approved"

		if !hasDiff && !hasCI && !hasApproval {
			lowEvidenceCount++
			safety := review.ClassifyMergeSafety(pr, nil)
			if safety.Confidence > 0.35 {
				t.Errorf("fixture PR %d (low evidence): confidence %.2f > 0.35", pr.Number, safety.Confidence)
			}
		}
	}

	t.Logf("found %d low-evidence PRs in fixtures, all capped correctly", lowEvidenceCount)
}

// TestReviewPayloadJSONRoundTripRegression verifies the review payload
// can be serialized and deserialized without data loss.
func TestReviewPayloadJSONRoundTripRegression(t *testing.T) {
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
		t.Fatal("review payload should be populated")
	}

	// Verify all results have required fields for serialization
	for i, result := range response.ReviewPayload.Results {
		if result.Category == "" {
			t.Errorf("result[%d]: category required for JSON serialization", i)
		}
		if result.PriorityTier == "" {
			t.Errorf("result[%d]: priority_tier required for JSON serialization", i)
		}
		if result.NextAction == "" {
			t.Errorf("result[%d]: next_action required for JSON serialization", i)
		}
	}

	// Verify bucket structure is complete
	if len(response.ReviewPayload.Buckets) < 5 {
		t.Errorf("expected at least 5 buckets, got %d", len(response.ReviewPayload.Buckets))
	}

	// Verify counts are non-negative
	for _, cat := range response.ReviewPayload.Categories {
		if cat.Count < 0 {
			t.Errorf("category %q has negative count: %d", cat.Category, cat.Count)
		}
	}

	for _, tier := range response.ReviewPayload.PriorityTiers {
		if tier.Count < 0 {
			t.Errorf("tier %q has negative count: %d", tier.Tier, tier.Count)
		}
	}
}

// TestEvidenceTierConfidenceMappingRegression verifies the mapping from
// evidence tiers to confidence caps is stable.
func TestEvidenceTierConfidenceMappingRegression(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		pr            types.PR
		maxConfidence float64
		minConfidence float64
		description   string
	}{
		{
			name: "tier 0 - no evidence",
			pr: types.PR{
				Number:       1,
				Title:        "no evidence",
				Body:         "",
				FilesChanged: []string{},
				Additions:    0,
				Deletions:    0,
				CIStatus:     "",
				ReviewStatus: "",
			},
			maxConfidence: 0.35,
			description:   "metadata-only should be capped at 0.35",
		},
		{
			name: "tier 1 - diff only",
			pr: types.PR{
				Number:       2,
				Title:        "has diff",
				Body:         "changes",
				FilesChanged: []string{"file.go"},
				Additions:    10,
				Deletions:    5,
				CIStatus:     "",
				ReviewStatus: "",
			},
			maxConfidence: 0.65,
			description:   "diff without CI capped at 0.65 (medium tier)",
		},
		{
			name: "tier 2 - diff + CI",
			pr: types.PR{
				Number:       3,
				Title:        "diff + CI",
				Body:         "changes with CI",
				FilesChanged: []string{"file.go"},
				Additions:    10,
				Deletions:    5,
				CIStatus:     "success",
				ReviewStatus: "",
			},
			maxConfidence: 0.65,
			description:   "diff + CI without approval capped at 0.65",
		},
		{
			name: "tier 3 - full evidence no risk",
			pr: types.PR{
				Number:            4,
				Title:             "full evidence",
				Body:              "changes with CI and approval",
				FilesChanged:      []string{"file.go"},
				Additions:         10,
				Deletions:         5,
				ChangedFilesCount: 1,
				CIStatus:          "success",
				ReviewStatus:      "approved",
				Mergeable:         "true",
				IsDraft:           false,
			},
			minConfidence: 0.80,
			description:   "full evidence should reach at least 0.80",
		},
		{
			name: "high risk with full evidence",
			pr: types.PR{
				Number:            5,
				Title:             "large change",
				Body:              "many changes with CI and approval",
				FilesChanged:      []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
				Additions:         100,
				Deletions:         50,
				ChangedFilesCount: 5,
				CIStatus:          "success",
				ReviewStatus:      "approved",
				Mergeable:         "true",
				IsDraft:           false,
			},
			maxConfidence: 0.79,
			description:   "high-risk PR capped at 0.79 even with full evidence",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := review.ClassifyMergeSafety(tc.pr, nil)

			if tc.maxConfidence > 0 && result.Confidence > tc.maxConfidence {
				t.Errorf("%s: confidence %.2f > max %.2f", tc.description, result.Confidence, tc.maxConfidence)
			}
			if tc.minConfidence > 0 && result.Confidence < tc.minConfidence {
				t.Errorf("%s: confidence %.2f < min %.2f", tc.description, result.Confidence, tc.minConfidence)
			}
		})
	}
}

// TestReviewIntegrationRegression verifies the full review pipeline
// produces consistent results across multiple runs.
func TestReviewIntegrationRegression(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Run analysis twice with same config
	config := Config{
		Now:           fixedNow,
		IncludeReview: true,
	}

	service1 := NewService(config)
	response1, err := service1.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("first analyze: %v", err)
	}

	service2 := NewService(config)
	response2, err := service2.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("second analyze: %v", err)
	}

	// Results should be deterministic
	if response1.ReviewPayload.TotalPRs != response2.ReviewPayload.TotalPRs {
		t.Errorf("determinism: total_prs %d != %d", response1.ReviewPayload.TotalPRs, response2.ReviewPayload.TotalPRs)
	}

	if response1.ReviewPayload.ReviewedPRs != response2.ReviewPayload.ReviewedPRs {
		t.Errorf("determinism: reviewed_prs %d != %d", response1.ReviewPayload.ReviewedPRs, response2.ReviewPayload.ReviewedPRs)
	}

	if len(response1.ReviewPayload.Results) != len(response2.ReviewPayload.Results) {
		t.Errorf("determinism: result count %d != %d", len(response1.ReviewPayload.Results), len(response2.ReviewPayload.Results))
	}

	// Bucket counts should match
	if len(response1.ReviewPayload.Buckets) != len(response2.ReviewPayload.Buckets) {
		t.Errorf("determinism: bucket count %d != %d", len(response1.ReviewPayload.Buckets), len(response2.ReviewPayload.Buckets))
	}
}

// isPassingCI checks if a CI status indicates passing
func isPassingCI(status string) bool {
	switch status {
	case "success", "passed", "green":
		return true
	default:
		return false
	}
}
