package review

import (
	"math"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestCalculateConfidenceFromFindings_NoFindings(t *testing.T) {
	got := calculateConfidenceFromFindings(nil)
	if got != 0.95 {
		t.Fatalf("expected 0.95 for no findings, got %f", got)
	}
}

func TestCalculateConfidenceFromFindings_SingleFinding(t *testing.T) {
	findings := []types.AnalyzerFinding{{Confidence: 0.80}}
	got := calculateConfidenceFromFindings(findings)
	if math.Abs(got-0.80) > 0.001 {
		t.Fatalf("expected ~0.80 for single finding, got %f", got)
	}
}

func TestCalculateConfidenceFromFindings_MultipleFindings(t *testing.T) {
	findings := []types.AnalyzerFinding{
		{Confidence: 0.70},
		{Confidence: 0.80},
	}
	got := calculateConfidenceFromFindings(findings)
	// 0.8*0.6 + 0.75*0.4 + 0.02 = 0.48 + 0.30 + 0.02 = 0.80
	if math.Abs(got-0.80) > 0.001 {
		t.Fatalf("expected ~0.80 for two findings, got %f", got)
	}
}

func TestCalculateConfidenceFromFindings_CappedAt95(t *testing.T) {
	findings := []types.AnalyzerFinding{
		{Confidence: 0.95},
		{Confidence: 0.95},
		{Confidence: 0.95},
		{Confidence: 0.95},
		{Confidence: 0.95},
	}
	got := calculateConfidenceFromFindings(findings)
	if got != 0.95 {
		t.Fatalf("expected cap at 0.95, got %f", got)
	}
}

func TestCapConfidenceByCategory(t *testing.T) {
	cases := []struct {
		category types.ReviewCategory
		input    float64
		want     float64
	}{
		{types.ReviewCategoryMergeNow, 0.99, 0.95},
		{types.ReviewCategoryProblematicQuarantine, 0.95, 0.90},
		{types.ReviewCategoryUnknownEscalate, 0.95, 0.70},
		{types.ReviewCategoryMergeAfterFocusedReview, 0.95, 0.92},
		{types.ReviewCategoryDuplicateSuperseded, 0.95, 0.93},
		{types.ReviewCategoryMergeNow, 0.40, 0.50},
	}
	for _, tc := range cases {
		got := capConfidenceByCategory(tc.category, tc.input)
		if math.Abs(got-tc.want) > 0.001 {
			t.Fatalf("category %s: expected %f, got %f", tc.category, tc.want, got)
		}
	}
}
