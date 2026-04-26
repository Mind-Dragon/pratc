package executor

import (
	"testing"
)

// TestGetGitHubToken_Precedence verifies that GITHUB_PAT takes precedence over GITHUB_TOKEN.
func TestGetGitHubToken_Precedence(t *testing.T) {
	// Clear both env vars first
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("GITHUB_TOKEN", "")

	// Set both, GITHUB_PAT should win
	t.Setenv("GITHUB_PAT", "pat-token")
	t.Setenv("GITHUB_TOKEN", "env-token")

	token, err := GetGitHubToken()
	if err != nil {
		t.Fatalf("GetGitHubToken() error = %v", err)
	}
	if token != "pat-token" {
		t.Errorf("GetGitHubToken() = %q, want pat-token (GITHUB_PAT precedence)", token)
	}

	// Now only GITHUB_TOKEN set
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("GITHUB_TOKEN", "env-token")
	token, err = GetGitHubToken()
	if err != nil {
		t.Fatalf("GetGitHubToken() error = %v", err)
	}
	if token != "env-token" {
		t.Errorf("GetGitHubToken() = %q, want env-token", token)
	}
}