package review

import (
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// RunQuickWinPass applies second-pass reclassification rules to low-value PRs.
// It identifies PRs that appear low-value but have characteristics suggesting
// hidden value (small focused PRs, docs PRs, PRs with quality findings, or abandoned PRs).
//
// Returns the number of PRs reclassified.
func RunQuickWinPass(results []types.ReviewResult, prDataMap map[int]PRData) int {
	reclassifiedCount := 0

	for i := range results {
		result := &results[i]

		// Only process PRs that suggest low value:
		// TemporalBucket == "future" AND SubstanceScore < 50
		// AND Category is NOT duplicate_superseded or problematic_quarantine
		if !isLowValueCandidate(result) {
			continue
		}

		// Get PR data for this result
		prData, ok := prDataMap[result.PRNumber]
		if !ok {
			continue
		}

		// Apply quick-win rules (first match wins)
		reclassified := false
		var reason string
		var batchTag string

		// Rule 1: Small PR with passing CI
		if isSmallPR(prData.PR) && hasPassingCI(prData.PR) && !prData.PR.IsDraft &&
			len(prData.ConflictPairs) < 3 && result.SubstanceScore > 30 {
			reason = "quick win: small focused PR with passing CI"
			batchTag = deriveBatchTag(prData.PR, result.SubstanceScore)
			result.Category = types.ReviewCategoryMergeAfterFocusedReview
			result.TemporalBucket = "now"
			reclassified = true
		}

		// Rule 2: Docs-only PR with passing CI
		if !reclassified && isDocsOnly(prData.PR) && hasPassingCI(prData.PR) &&
			len(prData.ConflictPairs) == 0 && result.SubstanceScore > 25 {
			reason = "quick win: documentation improvement"
			batchTag = "docs-batch"
			result.Category = types.ReviewCategoryMergeAfterFocusedReview
			result.TemporalBucket = "now"
			reclassified = true
		}

		// Rule 3: Has security/reliability/performance findings
		if !reclassified && hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
			reason = "hidden value: addresses quality concerns"
			batchTag = ""
			result.Category = types.ReviewCategoryMergeAfterFocusedReview
			result.TemporalBucket = "future"
			reclassified = true
		}

		// Rule 4: Abandoned PR (no activity >90d + CI failing + no reviews)
		if !reclassified && isAbandoned(prData.PR) && isFailingCI(prData.PR) &&
			prData.PR.ReviewCount == 0 && result.SubstanceScore < 20 {
			reason = "abandoned low-value PR"
			batchTag = ""
			originalCategory := result.Category
			result.Category = types.ReviewCategoryProblematicQuarantine
			result.TemporalBucket = "junk"
			result.ReclassifiedFrom = string(originalCategory)
			result.ReclassificationReason = reason
			reclassified = true
		}

		// Catchall: PR matched no rule but is a genuine low_value candidate
		if !reclassified {
			reason = "genuine low_value"
			result.ReclassificationReason = reason
			result.ReclassifiedFrom = "low_value"
			reclassifiedCount++
			continue
		}

		// If reclassified, apply common updates
		if reclassified {
			// For Rules 1-3, ReclassifiedFrom is always "low_value" since they came from low_value
			// For Rule 4, it's already set above to the original category
			if result.ReclassifiedFrom == "" {
				result.ReclassifiedFrom = "low_value"
			}
			result.ReclassificationReason = reason
			result.BatchTag = batchTag

			// Append Gate 17 "Value Reassessment" to DecisionLayers
			gate17 := types.DecisionLayer{
				Layer:     17,
				Name:      "Value Reassessment",
				CostTier:  "cheap",
				Bucket:    "quick_win",
				Status:    "observed",
				Reasons:   []string{reason},
				Continued: false,
				Terminal:  true,
			}
			result.DecisionLayers = append(result.DecisionLayers, gate17)
			reclassifiedCount++
		}
	}

	return reclassifiedCount
}

// isLowValueCandidate returns true if the result suggests a low-value PR
// eligible for quick-win reclassification.
func isLowValueCandidate(result *types.ReviewResult) bool {
	// Must be future temporal bucket
	if result.TemporalBucket != "future" {
		return false
	}

	// Must have low substance score
	if result.SubstanceScore >= 50 {
		return false
	}

	// Must NOT be duplicate_superseded or problematic_quarantine
	if result.Category == types.ReviewCategoryDuplicateSuperseded ||
		result.Category == types.ReviewCategoryProblematicQuarantine {
		return false
	}

	// Must have been originally low_value (not recovered from blocked)
	if result.ReclassifiedFrom == "blocked" {
		return false
	}

	return true
}

// isSmallPR returns true if the PR is small by file count or diff footprint.
func isSmallPR(pr types.PR) bool {
	// Small: <= 3 changed files OR <= 50 additions+deletions
	if pr.ChangedFilesCount <= 3 {
		return true
	}
	if pr.Additions+pr.Deletions <= 50 {
		return true
	}
	return false
}

// hasPassingCI returns true if CI status is passing or unknown.
func hasPassingCI(pr types.PR) bool {
	switch pr.CIStatus {
	case "success", "passed", "green":
		return true
	case "":
		return true // unknown is treated as passing for quick-win
	default:
		return false
	}
}

// isFailingCI returns true if CI status indicates failure.
func isFailingCI(pr types.PR) bool {
	switch pr.CIStatus {
	case "failure", "failed", "red", "error":
		return true
	default:
		return false
	}
}

// isDocsOnly returns true if all changed files are documentation.
func isDocsOnly(pr types.PR) bool {
	if len(pr.FilesChanged) == 0 {
		return false
	}

	for _, file := range pr.FilesChanged {
		lower := strings.ToLower(file)
		if strings.HasSuffix(lower, ".md") ||
			strings.HasSuffix(lower, ".txt") ||
			strings.HasSuffix(lower, ".rst") ||
			strings.HasPrefix(lower, "docs/") {
			continue
		}
		return false
	}
	return true
}

// hasQualityFindings returns true if there are security, reliability, or performance findings.
func hasQualityFindings(findings []types.AnalyzerFinding) bool {
	for _, f := range findings {
		finding := strings.ToLower(f.Finding)
		if strings.HasPrefix(finding, "security_") ||
			strings.HasPrefix(finding, "reliability_") ||
			strings.HasPrefix(finding, "performance_") {
			return true
		}
	}
	return false
}

// isAbandoned returns true if the PR has no activity for more than 90 days.
func isAbandoned(pr types.PR) bool {
	if pr.UpdatedAt == "" {
		return false
	}

	t, err := time.Parse(time.RFC3339, pr.UpdatedAt)
	if err != nil {
		return false
	}

	// Use integer division to avoid floating point precision issues at boundaries.
	// A PR is abandoned if days since update > 90, i.e., 91+ days.
	return int(time.Since(t).Hours()/24) > 90
}

// deriveBatchTag determines the batch tag for a reclassified PR.
func deriveBatchTag(pr types.PR, substanceScore int) string {
	totalChanges := pr.Additions + pr.Deletions

	// Dependency batch: PRs with dependencies or dependabot labels
	for _, label := range pr.Labels {
		lowerLabel := strings.ToLower(label)
		if strings.Contains(lowerLabel, "dependencies") || strings.Contains(lowerLabel, "dependabot") {
			return "dependency-batch"
		}
	}

	// Typo batch: tiny PRs with <= 10 additions+deletions
	if totalChanges <= 10 {
		return "typo-batch"
	}

	// Default: empty string (no batch tag)
	return ""
}
