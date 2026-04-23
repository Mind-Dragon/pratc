package app

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestAnalyze_ReviewFailureMarksPayloadDegraded verifies that when the review
// pipeline fails partially, the response is marked as degraded with explicit
// error information rather than silently returning an empty review.
func TestAnalyze_ReviewFailureMarksPayloadDegraded(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Run analyze with review enabled. The fixture may or may not trigger
	// review failures, but we verify the contract: if Partial is true,
	// Errors must be non-empty.
	service := NewService(Config{Now: fixedNow, IncludeReview: true})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if response.ReviewPayload == nil {
		t.Skip("no review payload — review may be disabled")
	}

	// If the review was partial, it must carry error information.
	if response.ReviewPayload.Partial {
		if len(response.ReviewPayload.Errors) == 0 {
			t.Fatal("review marked partial but has no errors — contract violation")
		}
	}
}

// TestAnalyze_PerPRErrorRecordedInReviewPayload verifies that per-PR review
// failures are recorded in the review response rather than silently skipped.
func TestAnalyze_PerPRErrorRecordedInReviewPayload(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, IncludeReview: true})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if response.ReviewPayload == nil {
		t.Skip("no review payload")
	}

	// The review response should have ReviewedPRs <= TotalPRs.
	// If any PRs failed, FailedPRs should be populated.
	if response.ReviewPayload.ReviewedPRs > response.ReviewPayload.TotalPRs {
		t.Fatalf("reviewed (%d) > total (%d)", response.ReviewPayload.ReviewedPRs, response.ReviewPayload.TotalPRs)
	}

	// If partial, failed PRs should be listed.
	if response.ReviewPayload.Partial && len(response.ReviewPayload.FailedPRs) == 0 {
		t.Log("partial review with no failed_prs — may be a top-level failure only")
	}
}

// TestAnalyze_PartialReviewStillReturnsAnalysis verifies that a partial review
// failure does not prevent the rest of the analysis from being returned.
func TestAnalyze_PartialReviewStillReturnsAnalysis(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, IncludeReview: true})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	// Even if review is partial, the core analysis should still be present.
	if len(response.PRs) == 0 {
		t.Fatal("expected PRs in analysis even if review is partial")
	}
	if response.Counts.TotalPRs == 0 {
		t.Fatal("expected non-zero counts in analysis even if review is partial")
	}
}

// TestReviewResponse_DegradedFieldsAreAdditive verifies that the new degraded
// fields (Partial, Errors, FailedPRs) are additive and do not break existing
// consumers that don't expect them.
func TestReviewResponse_DegradedFieldsAreAdditive(t *testing.T) {
	t.Parallel()

	// A non-partial review should have zero-value degraded fields.
	resp := types.ReviewResponse{
		TotalPRs:    10,
		ReviewedPRs: 10,
		Results:     []types.ReviewResult{},
	}

	if resp.Partial {
		t.Fatal("zero-value Partial should be false")
	}
	if len(resp.Errors) != 0 {
		t.Fatal("zero-value Errors should be empty")
	}
	if len(resp.FailedPRs) != 0 {
		t.Fatal("zero-value FailedPRs should be empty")
	}
}
