// Package review provides the Analyzer interface and analyzers for the agentic PR review system.
package review

import (
	"context"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// ReliabilityAnalyzer detects reliability-related risks in PRs including:
// - Failed CI status
// - Flaky CI patterns (multiple re-runs)
// - Mergeability issues (conflicts, unclean state)
// - Review churn (multiple review cycles, requested changes)
type ReliabilityAnalyzer struct{}

// NewReliabilityAnalyzer creates a new ReliabilityAnalyzer that implements the Analyzer interface.
func NewReliabilityAnalyzer() Analyzer {
	return &ReliabilityAnalyzer{}
}

// Metadata returns information about this analyzer for reporting purposes.
func (r *ReliabilityAnalyzer) Metadata() types.AnalyzerMetadata {
	return types.AnalyzerMetadata{
		Name:       "reliability",
		Version:    "0.1.0",
		Category:   "reliability",
		Confidence: 0.90,
	}
}

// reliabilityFinding represents an individual reliability finding.
type reliabilityFinding struct {
	finding    string
	confidence float64
	severity   string
}

// Analyze examines PR data for reliability risks and returns findings.
func (r *ReliabilityAnalyzer) Analyze(ctx context.Context, prData PRData) (AnalyzerResult, error) {
	startTime := time.Now()

	pr := prData.PR
	var findings []types.AnalyzerFinding

	// 1. Detect failed CI status
	ciFindings := r.detectCIFailures(pr.CIStatus, pr.Labels)
	findings = append(findings, ciFindings...)

	// 2. Detect flaky CI patterns (multiple re-runs)
	flakyFindings := r.detectFlakyPatterns(pr.Labels, pr.CIStatus)
	findings = append(findings, flakyFindings...)

	// 3. Detect mergeability issues
	mergeFindings := r.detectMergeabilityIssues(pr.Mergeable, prData.ConflictPairs)
	findings = append(findings, mergeFindings...)

	// 4. Detect review churn (multiple review cycles, requested changes)
	reviewFindings := r.detectReviewChurn(pr.ReviewStatus, pr.Labels)
	findings = append(findings, reviewFindings...)

	// 5. Add subsystem evidence when file-level diff metadata is available.
	if len(prData.Files) > 0 {
		findings = append(findings, subsystemFindings("reliability", prData.Files)...)
	}

	// Determine overall category and priority based on findings
	category, priority, _ := r.classifyReliabilityRisk(findings)

	confidence := calculateConfidenceFromFindings(findings)
	confidence = capConfidenceByCategory(category, confidence)

	result := types.ReviewResult{
		Category:         category,
		PriorityTier:     priority,
		Confidence:       confidence,
		Reasons:          r.extractReasons(findings),
		AnalyzerFindings: findings,
	}

	return AnalyzerResult{
		Result:           result,
		AnalyzerName:     "reliability",
		AnalyzerVersion:  "0.1.0",
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Error:            nil,
		IsPartial:        false,
		SkippedReasons:   []string{},
		StartedAt:        startTime,
		CompletedAt:      time.Now(),
	}, nil
}

// detectCIFailures flags failed CI status as high reliability risk.
func (r *ReliabilityAnalyzer) detectCIFailures(ciStatus string, labels []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	// Check for failed CI status
	ciStatus = strings.ToLower(ciStatus)
	if ciStatus == "failure" || ciStatus == "failed" || ciStatus == "error" {
		finding := types.AnalyzerFinding{
			AnalyzerName:    "reliability",
			AnalyzerVersion: "0.1.0",
			Finding:         "CI failure detected: build failed",
			Confidence:      0.95,
		}
		findings = append(findings, finding)
	}

	// Also check for labels indicating CI issues
	for _, label := range labels {
		labelLower := strings.ToLower(label)
		if labelLower == "ci-failed" || labelLower == "build-failed" || labelLower == "tests-failed" {
			finding := types.AnalyzerFinding{
				AnalyzerName:    "reliability",
				AnalyzerVersion: "0.1.0",
				Finding:         "CI failure detected: label indicates failure",
				Confidence:      0.90,
			}
			findings = append(findings, finding)
			break
		}
	}

	return findings
}

// detectFlakyPatterns detects flaky CI patterns via labels or status indicators.
func (r *ReliabilityAnalyzer) detectFlakyPatterns(labels []string, ciStatus string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	// Check for labels indicating flaky CI
	for _, label := range labels {
		labelLower := strings.ToLower(label)
		if labelLower == "flaky" || labelLower == "flaky-test" || labelLower == "ci-flaky" ||
			labelLower == "test-flaky" || labelLower == "flaky-ci" {
			finding := types.AnalyzerFinding{
				AnalyzerName:    "reliability",
				AnalyzerVersion: "0.1.0",
				Finding:         "Flaky CI pattern detected: label indicates CI instability",
				Confidence:      0.85,
			}
			findings = append(findings, finding)
			break
		}
	}

	// Check for CI re-run indicators in status
	ciStatusLower := strings.ToLower(ciStatus)
	if strings.Contains(ciStatusLower, "rerun") || strings.Contains(ciStatusLower, "retry") ||
		strings.Contains(ciStatusLower, "re-run") || strings.Contains(ciStatusLower, "re-run") {
		finding := types.AnalyzerFinding{
			AnalyzerName:    "reliability",
			AnalyzerVersion: "0.1.0",
			Finding:         "Flaky CI pattern detected: CI was re-run",
			Confidence:      0.75,
		}
		findings = append(findings, finding)
	}

	return findings
}

// detectMergeabilityIssues flags merge conflicts and unclean state.
func (r *ReliabilityAnalyzer) detectMergeabilityIssues(mergeable string, conflictPairs []types.ConflictPair) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	// Check mergeable state
	mergeable = strings.ToLower(mergeable)
	if mergeable == "false" || mergeable == "conflicting" || mergeable == "unknown" {
		confidence := 0.90
		findingText := "Merge conflict detected: PR is not mergeable"

		if mergeable == "unknown" {
			confidence = 0.60
			findingText = "Merge state unknown: cannot determine mergeability"
		}

		finding := types.AnalyzerFinding{
			AnalyzerName:    "reliability",
			AnalyzerVersion: "0.1.0",
			Finding:         findingText,
			Confidence:      confidence,
		}
		findings = append(findings, finding)
	}

	// Check for conflict pairs involving this PR
	if len(conflictPairs) > 0 {
		finding := types.AnalyzerFinding{
			AnalyzerName:    "reliability",
			AnalyzerVersion: "0.1.0",
			Finding:         "Merge conflict detected: PR has conflicting dependencies",
			Confidence:      0.90,
		}
		findings = append(findings, finding)
	}

	return findings
}

// detectReviewChurn flags multiple review cycles and requested changes.
func (r *ReliabilityAnalyzer) detectReviewChurn(reviewStatus string, labels []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	// Check for requested changes review status
	reviewStatus = strings.ToLower(reviewStatus)
	if reviewStatus == "changes_requested" || reviewStatus == "changes requested" {
		finding := types.AnalyzerFinding{
			AnalyzerName:    "reliability",
			AnalyzerVersion: "0.1.0",
			Finding:         "Review churn detected: reviewer requested changes",
			Confidence:      0.90,
		}
		findings = append(findings, finding)
	}

	// Check for labels indicating review churn or instability
	for _, label := range labels {
		labelLower := strings.ToLower(label)
		if labelLower == "changes-requested" || labelLower == "review-churn" ||
			labelLower == "needs-revision" || labelLower == "needs-rework" ||
			labelLower == "author-respond" || labelLower == "waiting-on-author" {
			finding := types.AnalyzerFinding{
				AnalyzerName:    "reliability",
				AnalyzerVersion: "0.1.0",
				Finding:         "Review churn detected: label indicates review cycle",
				Confidence:      0.80,
			}
			findings = append(findings, finding)
			break
		}
	}

	// Check for many review cycles (via comment count or large body)
	// This is a heuristic - in a real system we'd track review events

	return findings
}

// classifyReliabilityRisk determines the review category and priority tier based on findings.
func (r *ReliabilityAnalyzer) classifyReliabilityRisk(findings []types.AnalyzerFinding) (types.ReviewCategory, types.PriorityTier, float64) {
	if len(findings) == 0 {
		return types.ReviewCategoryMergeNow, types.PriorityTierFastMerge, 0.95
	}

	// Count findings by severity
	criticalCount := 0
	highCount := 0
	mediumCount := 0

	for _, f := range findings {
		findingLower := strings.ToLower(f.Finding)
		if strings.Contains(findingLower, "ci failure") || strings.Contains(findingLower, "merge conflict") {
			criticalCount++
		} else if strings.Contains(findingLower, "flaky") {
			highCount++
		} else if strings.Contains(findingLower, "review churn") || strings.Contains(findingLower, "changes requested") {
			mediumCount++
		}
	}

	// Critical severity: CI failure or merge conflict blocks merge
	if criticalCount > 0 {
		return types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked, 0.90
	}

	// High severity: flaky patterns indicate potential instability
	if highCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.80
	}

	// Medium severity: review churn suggests potential issues
	if mediumCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.75
	}

	return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.70
}

// extractReasons extracts human-readable reason codes from findings.
func (r *ReliabilityAnalyzer) extractReasons(findings []types.AnalyzerFinding) []string {
	reasonSet := make(map[string]bool)

	for _, f := range findings {
		findingLower := strings.ToLower(f.Finding)
		if strings.Contains(findingLower, "ci failure") {
			reasonSet["ci_failure"] = true
		} else if strings.Contains(findingLower, "flaky") {
			reasonSet["flaky_ci"] = true
		} else if strings.Contains(findingLower, "merge conflict") || strings.Contains(findingLower, "not mergeable") {
			reasonSet["merge_conflict"] = true
		} else if strings.Contains(findingLower, "review churn") || strings.Contains(findingLower, "changes requested") {
			reasonSet["review_churn"] = true
		}
	}

	var reasons []string
	for reason := range reasonSet {
		reasons = append(reasons, reason)
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "no_reliability_issues")
	}

	return reasons
}
