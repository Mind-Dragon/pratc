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
// When a token is found, it is also injected back into GITHUB_TOKEN and GH_TOKEN
// so downstream code that reads environment variables sees the same auth.
func ResolveToken(ctx context.Context) (string, error) {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT"} {
		if token := strings.TrimSpace(os.Getenv(key)); token != "" {
			injectTokenEnv(token)
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

	injectTokenEnv(token)
	return token, nil
}

func injectTokenEnv(token string) {
	_ = os.Setenv("GITHUB_TOKEN", token)
	_ = os.Setenv("GH_TOKEN", token)
}
