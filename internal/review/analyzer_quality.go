// Package review provides the Analyzer interface and related types for the agentic PR review system.
package review

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// QualityAnalyzer detects quality issues in PRs using deterministic heuristics.
// It classifies PRs as problematic if they have:
//   - Empty or minimal body/description
//   - No linked issues (Fixes #, Closes #, etc.)
//   - Suspicious patterns (ALL CAPS, excessive punctuation)
//   - Noisy changes (excessive files/changes without purpose)
//
// QualityAnalyzer is stateless and deterministic - each Analyze() call is independent.
type QualityAnalyzer struct{}

// NewQualityAnalyzer creates a new QualityAnalyzer that implements the Analyzer interface.
func NewQualityAnalyzer() Analyzer {
	return &QualityAnalyzer{}
}

// Metadata returns information about this analyzer for reporting purposes.
func (q *QualityAnalyzer) Metadata() types.AnalyzerMetadata {
	return types.AnalyzerMetadata{
		Name:       "quality",
		Version:    "0.1.0",
		Category:   "quality",
		Confidence: 0.85,
	}
}

// Analyze examines the PR data for quality issues and returns a review result.
func (q *QualityAnalyzer) Analyze(ctx context.Context, prData PRData) (AnalyzerResult, error) {
	startedAt := time.Now()

	pr := prData.PR
	var findings []types.AnalyzerFinding
	var reasons []string

	// Check 1: Empty or minimal body
	if q.isEmptyBody(pr.Body) {
		findings = append(findings, types.AnalyzerFinding{
			AnalyzerName:    "quality",
			AnalyzerVersion: "0.1.0",
			Finding:         "empty_body",
			Confidence:      0.95,
		})
		reasons = append(reasons, "PR body is empty or whitespace only")
	} else if q.isMinimalDescription(pr.Body) {
		findings = append(findings, types.AnalyzerFinding{
			AnalyzerName:    "quality",
			AnalyzerVersion: "0.1.0",
			Finding:         "minimal_description",
			Confidence:      0.80,
		})
		reasons = append(reasons, "PR body is minimal (< 50 characters)")
	}

	// Check 2: Missing linked issues
	if !q.hasLinkedIssues(pr.Body) {
		findings = append(findings, types.AnalyzerFinding{
			AnalyzerName:    "quality",
			AnalyzerVersion: "0.1.0",
			Finding:         "no_linked_issues",
			Confidence:      0.75,
		})
		reasons = append(reasons, "PR does not reference any linked issues (Fixes #, Closes #, Resolves #)")
	}

	// Check 3: Suspicious patterns
	if suspicious := q.detectSuspiciousPatterns(pr.Title, pr.Body); len(suspicious) > 0 {
		for _, pattern := range suspicious {
			findings = append(findings, types.AnalyzerFinding{
				AnalyzerName:    "quality",
				AnalyzerVersion: "0.1.0",
				Finding:         pattern,
				Confidence:      0.70,
			})
			reasons = append(reasons, fmt.Sprintf("suspicious pattern detected: %s", pattern))
		}
	}

	// Check 4: Noisy PR (excessive changes without clear purpose)
	if q.isNoisyPR(pr.Body, pr.ChangedFilesCount, pr.Additions, pr.Deletions) {
		findings = append(findings, types.AnalyzerFinding{
			AnalyzerName:    "quality",
			AnalyzerVersion: "0.1.0",
			Finding:         "noisy_pr",
			Confidence:      0.65,
		})
		reasons = append(reasons, "PR has excessive changes without clear purpose or description")
	}

	// Determine category and priority tier
	category, priorityTier, confidence := q.classifyPR(findings)

	completedAt := time.Now()

	return AnalyzerResult{
		Result: types.ReviewResult{
			Category:         category,
			PriorityTier:     priorityTier,
			Confidence:       confidence,
			Reasons:          reasons,
			AnalyzerFindings: findings,
		},
		AnalyzerName:     "quality",
		AnalyzerVersion:  "0.1.0",
		ProcessingTimeMs: completedAt.Sub(startedAt).Milliseconds(),
		Error:            nil,
		IsPartial:        false,
		SkippedReasons:   []string{},
		StartedAt:        startedAt,
		CompletedAt:      completedAt,
	}, nil
}

// isEmptyBody returns true if the PR body is empty or whitespace only.
func (q *QualityAnalyzer) isEmptyBody(body string) bool {
	return strings.TrimSpace(body) == ""
}

// isMinimalDescription returns true if the PR body is too short to be useful.
func (q *QualityAnalyzer) isMinimalDescription(body string) bool {
	trimmed := strings.TrimSpace(body)
	// Consider < 50 characters as minimal
	return len(trimmed) > 0 && len(trimmed) < 50
}

// linkedIssuePatterns matches common issue reference patterns in PR bodies.
var linkedIssuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:fixes|closes|resolves|addressed?|implements?)[\s:#]+#\d+`),
	regexp.MustCompile(`(?i)#\d+(?:\s|$)`),
}

// hasLinkedIssues returns true if the PR body references any issues.
func (q *QualityAnalyzer) hasLinkedIssues(body string) bool {
	if body == "" {
		return false
	}
	for _, pattern := range linkedIssuePatterns {
		if pattern.MatchString(body) {
			return true
		}
	}
	return false
}

// detectSuspiciousPatterns returns a list of suspicious patterns found in title and body.
func (q *QualityAnalyzer) detectSuspiciousPatterns(title, body string) []string {
	var patterns []string

	// Check for ALL CAPS title (shouting)
	if q.isAllCapsTitle(title) {
		patterns = append(patterns, "all_caps_title")
	}

	// Check for excessive punctuation
	if q.hasExcessivePunctuation(title) {
		patterns = append(patterns, "excessive_punctuation")
	}

	// Check for placeholder text
	if q.hasPlaceholderText(body) {
		patterns = append(patterns, "placeholder_text")
	}

	return patterns
}

// isAllCapsTitle returns true if the title is mostly ALL CAPS (shouting).
func (q *QualityAnalyzer) isAllCapsTitle(title string) bool {
	if len(title) == 0 {
		return false
	}
	upperCount := 0
	letterCount := 0
	for _, r := range title {
		if unicode.IsLetter(r) {
			letterCount++
			if unicode.IsUpper(r) {
				upperCount++
			}
		}
	}
	// Consider shouting if > 50% of letters are uppercase and there are at least 3 words
	if letterCount >= 15 { // at least ~3 words worth of letters
		return float64(upperCount)/float64(letterCount) > 0.5
	}
	return false
}

// hasExcessivePunctuation returns true if the title has excessive punctuation.
func (q *QualityAnalyzer) hasExcessivePunctuation(title string) bool {
	excessiveCount := strings.Count(title, "!") + strings.Count(title, "?") + strings.Count(title, "...")
	return excessiveCount >= 3
}

// hasPlaceholderText returns true if the body contains common placeholder text.
func (q *QualityAnalyzer) hasPlaceholderText(body string) bool {
	lower := strings.ToLower(body)
	placeholders := []string{
		"todo",
		"fill in",
		"tbd",
		"xxx",
		"replace this",
		"[ ]",
		"undefined",
		"null",
	}
	for _, p := range placeholders {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isNoisyPR returns true if the PR has excessive changes without clear purpose.
func (q *QualityAnalyzer) isNoisyPR(body string, changedFiles, additions, deletions int) bool {
	trimmed := strings.TrimSpace(body)

	// Consider noisy if:
	// - > 20 files changed AND body is minimal (< 100 chars)
	// - OR > 1000 net changes (additions + deletions) without clear description
	netChanges := additions + deletions
	if changedFiles > 20 && len(trimmed) < 100 {
		return true
	}

	if netChanges > 1000 && len(trimmed) < 150 {
		return true
	}

	return false
}

// classifyPR determines the category, priority tier, and confidence based on findings.
func (q *QualityAnalyzer) classifyPR(findings []types.AnalyzerFinding) (types.ReviewCategory, types.PriorityTier, float64) {
	if len(findings) == 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.5
	}

	// Count findings by severity
	highSeverity := 0
	mediumSeverity := 0

	for _, f := range findings {
		switch f.Finding {
		case "empty_body", "no_linked_issues":
			highSeverity++
		case "minimal_description", "noisy_pr":
			mediumSeverity++
		default:
			mediumSeverity++
		}
	}

	// Classify based on severity
	if highSeverity >= 2 {
		return types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked, 0.85
	}

	if highSeverity == 1 || mediumSeverity >= 2 {
		return types.ReviewCategoryProblematicQuarantine, types.PriorityTierReviewRequired, 0.75
	}

	if mediumSeverity == 1 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.65
	}

	return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.5
}
