package types

import "strings"

// NormalizeRepoName returns a lowercase, trimmed version of the repository string.
// This ensures that "OpenClaw/OpenClaw" and "openclaw/openclaw" are treated as the same repo.
func NormalizeRepoName(repo string) string {
	return strings.ToLower(strings.TrimSpace(repo))
}
