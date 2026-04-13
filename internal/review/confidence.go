package review

import "github.com/jeffersonnunn/pratc/internal/types"

// calculateConfidenceFromFindings computes an evidence-backed confidence score
// from a slice of analyzer findings. It uses the maximum individual finding
// confidence as the base and applies a small bonus when multiple independent
// findings agree, capped at 0.95.
//
// When no findings are present, it returns 0.95 (absence of evidence is
// treated as high confidence for a clean bill of health).
func calculateConfidenceFromFindings(findings []types.AnalyzerFinding) float64 {
	if len(findings) == 0 {
		return 0.95
	}

	maxConfidence := 0.0
	sumConfidence := 0.0
	for _, f := range findings {
		if f.Confidence > maxConfidence {
			maxConfidence = f.Confidence
		}
		sumConfidence += f.Confidence
	}

	avgConfidence := sumConfidence / float64(len(findings))

	// Blend max and average, with a small bonus for multiple findings
	confidence := (maxConfidence*0.6 + avgConfidence*0.4)
	if len(findings) > 1 {
		confidence += 0.02 * float64(len(findings)-1)
	}

	// Hard cap and floor
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.50 {
		confidence = 0.50
	}
	return confidence
}

// capConfidenceByCategory applies a category-specific ceiling so that
// high-risk categories cannot claim implausibly high confidence without
// overwhelming evidence.
func capConfidenceByCategory(category types.ReviewCategory, confidence float64) float64 {
	switch category {
	case types.ReviewCategoryMergeNow:
		// Merge-now confidence should only be high when evidence is absent
		// or extremely strong. We leave it uncapped above.
		if confidence > 0.95 {
			return 0.95
		}
	case types.ReviewCategoryProblematicQuarantine:
		if confidence > 0.90 {
			return 0.90
		}
	case types.ReviewCategoryUnknownEscalate:
		if confidence > 0.70 {
			return 0.70
		}
	case types.ReviewCategoryMergeAfterFocusedReview:
		if confidence > 0.92 {
			return 0.92
		}
	case types.ReviewCategoryDuplicateSuperseded:
		if confidence > 0.93 {
			return 0.93
		}
	}
	if confidence < 0.50 {
		return 0.50
	}
	return confidence
}
