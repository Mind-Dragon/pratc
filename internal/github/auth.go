package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ResolveToken returns a GitHub token from the configured runtime sources.
//
// Resolution order:
// 1. Explicit runtime env vars: GITHUB_TOKEN, GH_TOKEN, GITHUB_PAT
// 2. `gh auth token` from a logged-in gh CLI session
//
// The token is returned directly and must be passed explicitly to components
// that need it (e.g., via github.Client Config.Token). It is NOT injected
// into os.environ to avoid leaking credentials to subprocesses.
func ResolveToken(ctx context.Context) (string, error) {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT"} {
		if token := strings.TrimSpace(os.Getenv(key)); token != "" {
			return token, nil
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("GitHub auth unavailable: set GITHUB_TOKEN/GH_TOKEN/GITHUB_PAT or sign in with gh auth login")
	}

	cmd := exec.CommandContext(ctx, path, "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("GitHub auth unavailable: gh auth token failed: %w", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("GitHub auth unavailable: gh auth token returned an empty token")
	}

	return token, nil
}
