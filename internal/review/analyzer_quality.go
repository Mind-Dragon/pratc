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

	// Check 5: Test gap evidence - production code changed without corresponding test changes
	if len(prData.Files) > 0 {
		if testGapFindings := q.detectTestGap(prData.Files); len(testGapFindings) > 0 {
			findings = append(findings, testGapFindings...)
			for _, f := range testGapFindings {
				reasons = append(reasons, fmt.Sprintf("test gap detected: %s", f.Finding))
			}
		}
		if testEvidenceFindings := detectTestEvidence(prData.Files); len(testEvidenceFindings) > 0 {
			findings = append(findings, testEvidenceFindings...)
			for _, f := range testEvidenceFindings {
				reasons = append(reasons, fmt.Sprintf("test evidence detected: %s", f.Finding))
			}
		}
		findings = append(findings, subsystemFindings("quality", prData.Files)...)
	}

	// Classify based on findings
	category, priorityTier, _ := q.classifyPR(findings)

	confidence := calculateConfidenceFromFindings(findings)
	confidence = capConfidenceByCategory(category, confidence)

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

// testFilePatterns matches test files that should have corresponding production files.
var testFilePatterns = []*regexp.Regexp{
	regexp.MustCompile(`_(test|spec)_`),
	regexp.MustCompile(`\.test\.`),
	regexp.MustCompile(`\.spec\.`),
	regexp.MustCompile(`/test/`),
	regexp.MustCompile(`/tests/`),
	regexp.MustCompile(`_test\.go$`),
	regexp.MustCompile(`_test\.py$`),
	regexp.MustCompile(`_test\.js$`),
	regexp.MustCompile(`_test\.ts$`),
}

// isTestFile returns true if the file path looks like a test file.
func isTestFile(path string) bool {
	for _, pattern := range testFilePatterns {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

// isProductionCode returns true if the file looks like production code (not test, not config).
func isProductionCode(path string) bool {
	if isTestFile(path) {
		return false
	}
	// Skip common non-production files
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return false
	}
	if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".toml") {
		return false
	}
	if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt") {
		return false
	}
	// Skip vendor and generated directories
	if strings.Contains(path, "/vendor/") || strings.Contains(path, "/generated/") {
		return false
	}
	return true
}

// detectTestGap detects when production code is changed without corresponding test changes.
// It looks for production file changes and checks if there are any test file changes.
func (q *QualityAnalyzer) detectTestGap(files []types.PRFile) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	// Separate production files and test files
	var prodFiles []types.PRFile
	var testFiles []types.PRFile

	for _, file := range files {
		if isTestFile(file.Path) {
			testFiles = append(testFiles, file)
		} else if isProductionCode(file.Path) {
			prodFiles = append(prodFiles, file)
		}
	}

	// If there are production changes but no test changes, flag as test gap
	if len(prodFiles) > 0 && len(testFiles) == 0 {
		// Only flag if there are substantial production changes
		totalAdditions := 0
		for _, pf := range prodFiles {
			totalAdditions += pf.Additions
		}
		if totalAdditions >= 10 {
			// Find the most significant production file change
			var topProdFile types.PRFile
			maxAdditions := 0
			for _, pf := range prodFiles {
				if pf.Additions > maxAdditions {
					maxAdditions = pf.Additions
					topProdFile = pf
				}
			}

			finding := types.AnalyzerFinding{
				AnalyzerName:    "quality",
				AnalyzerVersion: "0.1.0",
				Finding:         fmt.Sprintf("production code changed without test evidence: %s", topProdFile.Path),
				Confidence:      0.75,
				Subsystem:       classifySubsystem(topProdFile.Path),
				SignalType:      "test_gap",
				Location: &types.CodeLocation{
					FilePath: topProdFile.Path,
					Snippet: extractTestGapSnippet(topProdFile.Patch, 200),
				},
			}
			findings = append(findings, finding)
		}
	}

	return findings
}

// extractTestGapSnippet extracts a meaningful snippet from a production file patch.
func extractTestGapSnippet(patch string, maxLen int) string {
	if patch == "" {
		return ""
	}
	lines := strings.Split(patch, "\n")
	var sb strings.Builder
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			sb.WriteString(line)
			sb.WriteString("\n")
			count++
			if count >= 5 || sb.Len() > maxLen {
				break
			}
		}
	}
	result := sb.String()
	if len(result) > maxLen {
		return result[:maxLen] + "..."
	}
	return result
}
