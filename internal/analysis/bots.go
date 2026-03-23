package analysis

import (
	"regexp"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// Hardcoded bot author patterns for v0.1.
var botAuthors = []string{
	"dependabot[bot]",
	"renovate[bot]",
	"github-actions[bot]",
	"snyk-bot",
}

// Hardcoded bot title patterns for v0.1.
// These are compiled once at init for efficiency.
var botTitlePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^Bump `),
	regexp.MustCompile(`^chore\(deps\)`),
	regexp.MustCompile(`^Update dependency`),
}

// IsBotPR returns true if the PR appears to be from a known bot account
// based on author name or title patterns.
func IsBotPR(pr types.PR) bool {
	// Check author pattern
	for _, botAuthor := range botAuthors {
		if pr.Author == botAuthor {
			return true
		}
	}

	// Check title patterns
	for _, pattern := range botTitlePatterns {
		if pattern.MatchString(pr.Title) {
			return true
		}
	}

	return false
}

// DetectBots scans the provided PRs and sets IsBot=true on any PRs
// that match bot author or title patterns. Returns the modified slice.
func DetectBots(prs []types.PR) []types.PR {
	for i := range prs {
		prs[i].IsBot = IsBotPR(prs[i])
	}
	return prs
}
