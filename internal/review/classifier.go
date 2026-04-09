package review

import (
	"fmt"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type MergeSafetyResult struct {
	IsSafe     bool
	Confidence float64
	Reasons    []string
	Blockers   []string
}

func ClassifyMergeSafety(pr types.PR, conflictPairs []types.ConflictPair) MergeSafetyResult {
	var reasons, blockers []string
	confidence := 1.0

	switch pr.CIStatus {
	case "success", "passed", "green":
		reasons = append(reasons, "CI passing")
	case "failure", "failed", "red", "error":
		blockers = append(blockers, "CI failing")
		confidence -= 0.2
	case "pending", "running", "in_progress":
		confidence -= 0.1
		reasons = append(reasons, "CI pending")
	case "":
		confidence -= 0.1
	default:
		confidence -= 0.1
	}

	switch pr.ReviewStatus {
	case "approved":
		reasons = append(reasons, "approved")
	case "changes_requested":
		blockers = append(blockers, "changes requested")
		confidence -= 0.2
	case "pending", "review_required", "required":
		if !pr.IsDraft {
			blockers = append(blockers, "not reviewed")
			confidence -= 0.15
		} else {
			confidence -= 0.05
		}
	case "":
		confidence -= 0.1
	default:
		confidence -= 0.1
	}

	switch pr.Mergeable {
	case "true", "clean", "mergeable":
		reasons = append(reasons, "mergeable")
	case "false", "unclean", "conflicted", "dirty":
		blockers = append(blockers, "merge conflicts")
		confidence -= 0.25
	case "":
		confidence -= 0.15
	default:
		confidence -= 0.15
	}

	if pr.IsDraft {
		blockers = append(blockers, "draft PR")
		confidence -= 0.15
	} else {
		reasons = append(reasons, "not draft")
	}

	if len(conflictPairs) > 0 {
		conflictFiles := gatherConflictFiles(conflictPairs, pr.Number)
		if len(conflictFiles) > 0 {
			blockers = append(blockers, formatConflictBlocker(conflictFiles))
			confidence -= 0.2
		}
	} else {
		confidence -= 0.05
	}

	if confidence < 0.0 {
		confidence = 0.0
	}

	return MergeSafetyResult{
		IsSafe:     len(blockers) == 0,
		Confidence: confidence,
		Reasons:    reasons,
		Blockers:   blockers,
	}
}

func gatherConflictFiles(pairs []types.ConflictPair, prNumber int) []string {
	files := make(map[string]struct{})
	for _, pair := range pairs {
		if pair.SourcePR == prNumber || pair.TargetPR == prNumber {
			for _, f := range pair.FilesTouched {
				files[f] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(files))
	for f := range files {
		result = append(result, f)
	}
	return result
}

func formatConflictBlocker(files []string) string {
	if len(files) == 0 {
		return "semantic conflicts"
	}
	if len(files) == 1 {
		return "conflicts on file: " + files[0]
	}
	if len(files) == 2 {
		return "conflicts on files: " + files[0] + ", " + files[1]
	}
	return fmt.Sprintf("conflicts on %d files: %s, %s, %s and %d more",
		len(files), files[0], files[1], files[2], len(files)-3)
}

// ProblematicPRResult contains the classification result for a problematic PR.
type ProblematicPRResult struct {
	IsProblematic bool     `json:"is_problematic"`
	ProblemType   string   `json:"problem_type"`
	Confidence    float64  `json:"confidence"`
	Reasons       []string `json:"reasons"`
}

// ClassifyProblematicPR classifies a PR as problematic using deterministic heuristics.
// Problem types: spam, broken, suspicious, low_quality, none
func ClassifyProblematicPR(pr types.PR) ProblematicPRResult {
	confidence := 0.5

	// Check for spam indicators
	spamReasons, spamConfidence := checkSpam(pr)
	if len(spamReasons) > 0 {
		confidence = spamConfidence
		return ProblematicPRResult{
			IsProblematic: true,
			ProblemType:   "spam",
			Confidence:    confidence,
			Reasons:       spamReasons,
		}
	}

	// Check for broken indicators
	brokenReasons, brokenConfidence := checkBroken(pr)
	if len(brokenReasons) > 0 {
		confidence = brokenConfidence
		return ProblematicPRResult{
			IsProblematic: true,
			ProblemType:   "broken",
			Confidence:    confidence,
			Reasons:       brokenReasons,
		}
	}

	// Check for suspicious indicators
	suspiciousReasons, suspiciousConfidence := checkSuspicious(pr)
	if len(suspiciousReasons) > 0 {
		confidence = suspiciousConfidence
		return ProblematicPRResult{
			IsProblematic: true,
			ProblemType:   "suspicious",
			Confidence:    confidence,
			Reasons:       suspiciousReasons,
		}
	}

	// Check for low quality indicators
	lqReasons, lqConfidence := checkLowQuality(pr)
	if len(lqReasons) > 0 {
		confidence = lqConfidence
		return ProblematicPRResult{
			IsProblematic: true,
			ProblemType:   "low_quality",
			Confidence:    confidence,
			Reasons:       lqReasons,
		}
	}

	return ProblematicPRResult{
		IsProblematic: false,
		ProblemType:   "none",
		Confidence:    1.0,
		Reasons:       []string{},
	}
}

// checkSpam detects spam PRs: empty title/body, bot author, abnormal labels
func checkSpam(pr types.PR) ([]string, float64) {
	var reasons []string
	confidence := 0.0

	if pr.Title == "" {
		reasons = append(reasons, "empty title")
		confidence += 0.3
	}

	if pr.Body == "" {
		reasons = append(reasons, "empty body")
		confidence += 0.2
	}

	if pr.IsBot {
		reasons = append(reasons, "bot author")
		confidence += 0.3
	}

	for _, label := range pr.Labels {
		if len(label) <= 2 {
			reasons = append(reasons, fmt.Sprintf("abnormally short label: %q", label))
			confidence += 0.1
		}
	}

	if len(pr.Labels) > 10 {
		reasons = append(reasons, fmt.Sprintf("excessive labels: %d", len(pr.Labels)))
		confidence += 0.2
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	if confidence >= 0.4 {
		return reasons, confidence
	}
	return nil, 0.0
}

// checkBroken detects broken PRs: failed CI, merge conflicts, draft status
func checkBroken(pr types.PR) ([]string, float64) {
	var reasons []string
	confidence := 0.0

	// Failed CI
	switch pr.CIStatus {
	case "failure", "failed", "red", "error":
		reasons = append(reasons, "CI failing")
		confidence += 0.4
	case "":
		reasons = append(reasons, "CI status unknown")
		confidence += 0.1
	}

	// Merge conflicts
	switch pr.Mergeable {
	case "false", "unclean", "conflicted", "dirty":
		reasons = append(reasons, "merge conflicts")
		confidence += 0.35
	case "":
		reasons = append(reasons, "mergeable status unknown")
		confidence += 0.1
	}

	// Draft status
	if pr.IsDraft {
		reasons = append(reasons, "draft PR")
		confidence += 0.25
	}

	// Normalize confidence to max 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	if confidence >= 0.3 {
		return reasons, confidence
	}
	return nil, 0.0
}

// checkSuspicious detects suspicious PRs: very large diff, unusual file patterns, new author
func checkSuspicious(pr types.PR) ([]string, float64) {
	var reasons []string
	confidence := 0.0

	// Very large diff (additions > 2000 is unusual)
	if pr.Additions > 2000 {
		reasons = append(reasons, fmt.Sprintf("very large additions: +%d", pr.Additions))
		confidence += 0.3
	}

	// Very large diff (deletions > 2000 is unusual)
	if pr.Deletions > 2000 {
		reasons = append(reasons, fmt.Sprintf("very large deletions: -%d", pr.Deletions))
		confidence += 0.3
	}

	// Many changed files
	if pr.ChangedFilesCount > 50 {
		reasons = append(reasons, fmt.Sprintf("many changed files: %d", pr.ChangedFilesCount))
		confidence += 0.25
	}

	// Unusual file patterns (generated files, lock files, etc.)
	suspiciousPatterns := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "Gemfile.lock", "Cargo.lock", "go.sum", "poetry.lock"}
	patternCount := 0
	for _, file := range pr.FilesChanged {
		for _, pattern := range suspiciousPatterns {
			if file == pattern {
				patternCount++
				break
			}
		}
	}
	if patternCount > 3 {
		reasons = append(reasons, fmt.Sprintf("suspicious lock file changes: %d", patternCount))
		confidence += 0.2
	}

	// Normalize confidence to max 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	if confidence >= 0.3 {
		return reasons, confidence
	}
	return nil, 0.0
}

// checkLowQuality detects low quality PRs: empty body, no linked issues, minimal description
func checkLowQuality(pr types.PR) ([]string, float64) {
	var reasons []string
	confidence := 0.0

	// Empty body
	if pr.Body == "" {
		reasons = append(reasons, "empty body")
		confidence += 0.3
	}

	// Minimal description (body less than 20 characters)
	if len(pr.Body) > 0 && len(pr.Body) < 20 {
		reasons = append(reasons, fmt.Sprintf("minimal description: %d chars", len(pr.Body)))
		confidence += 0.2
	}

	// No linked issues check (body doesn't contain issue references like #123)
	hasIssueReference := containsIssueReference(pr.Body)
	if !hasIssueReference && pr.Body != "" {
		reasons = append(reasons, "no linked issues")
		confidence += 0.15
	}

	// No description at all
	if pr.Title != "" && pr.Body == "" {
		reasons = append(reasons, "no description provided")
		confidence += 0.25
	}

	// Normalize confidence to max 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	if confidence >= 0.3 {
		return reasons, confidence
	}
	return nil, 0.0
}

// containsIssueReference checks if text contains GitHub issue/PR references
func containsIssueReference(text string) bool {
	// Simple pattern matching for #number references
	// This is a basic check - doesn't cover all formats but catches common cases
	if len(text) < 2 {
		return false
	}
	// Look for # followed by digits
	inHash := false
	digitCount := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '#' {
			inHash = true
			digitCount = 0
		} else if inHash && c >= '0' && c <= '9' {
			digitCount++
			if digitCount >= 1 {
				return true
			}
		} else if inHash {
			inHash = false
		}
	}
	return false
}
