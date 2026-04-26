package executor

import (
	"fmt"
	"os"
	"strings"
)

// GetGitHubToken returns first non-empty env var among GITHUB_PAT, GITHUB_TOKEN.
// Trim whitespace. If both empty, return error "no GitHub token set (GITHUB_PAT or GITHUB_TOKEN)".
func GetGitHubToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv("GITHUB_PAT")); token != "" {
		return token, nil
	}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("no GitHub token set (GITHUB_PAT or GITHUB_TOKEN)")
}