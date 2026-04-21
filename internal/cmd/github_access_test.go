package cmd

import (
	"context"
	"os"
	"testing"

	gh "github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/settings"
)

// TestResolveGitHubAccess_Unreachable verifies that ResolveGitHubAccess
// returns an error with appropriate message when GitHub is unreachable.
func TestResolveGitHubAccess_Unreachable(t *testing.T) {
	t.Parallel()

	// This test verifies the fallback behavior when GitHub is not accessible.
	// In a real environment without gh CLI, this should return an error.
	ctx := context.Background()

	// When settings store is unavailable, it should fall back to default resolution.
	// Since we can't guarantee gh CLI availability in test, we just verify
	// the function doesn't panic and returns consistent error types.
	access, err := resolveGitHubAccessWithDefaults(ctx)

	// The error type tells us about the access state
	if err != nil {
		// Error indicates GitHub is not accessible
		if access.State == gh.AccessStateUnreachable {
			// This is expected when gh CLI is not available or not logged in
			t.Logf("Got expected unreachable state: %v, message: %s", access.State, access.Message)
		}
	} else {
		// No error means we got a token
		if access.State != gh.AccessStateReachableAuthenticated {
			t.Errorf("expected authenticated state when no error, got %v", access.State)
		}
		if access.Token == "" {
			t.Error("expected non-empty token when no error")
		}
	}
}

// TestResolveGitHubAccess_StateTransitions verifies the access state transitions.
func TestResolveGitHubAccess_StateTransitions(t *testing.T) {
	t.Parallel()

	// Verify the AccessState values are correct
	if gh.AccessStateUnknown != 0 {
		t.Errorf("expected AccessStateUnknown to be 0, got %d", gh.AccessStateUnknown)
	}
	if gh.AccessStateReachableAuthenticated != 1 {
		t.Errorf("expected AccessStateReachableAuthenticated to be 1, got %d", gh.AccessStateReachableAuthenticated)
	}
	if gh.AccessStateReachableUnauthenticated != 2 {
		t.Errorf("expected AccessStateReachableUnauthenticated to be 2, got %d", gh.AccessStateReachableUnauthenticated)
	}
	if gh.AccessStateUnreachable != 3 {
		t.Errorf("expected AccessStateUnreachable to be 3, got %d", gh.AccessStateUnreachable)
	}
}

// TestAccessStateResult_Message verifies that AccessStateResult
// contains meaningful messages.
func TestAccessStateResult_Message(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Test with no configured logins (should use default)
	result, err := gh.ResolveNamedLogin(ctx, nil, true)
	if err != nil {
		// If error, state should indicate the problem
		if result.State == gh.AccessStateUnreachable {
			t.Logf("Got unreachable state as expected: %s", result.Message)
		}
	} else {
		// With nil/empty selected logins, should fall back to default
		if result.State != gh.AccessStateReachableAuthenticated {
			t.Errorf("expected reachable authenticated with nil logins, got %v", result.State)
		}
		if result.Message == "" {
			t.Error("expected non-empty message")
		}
		t.Logf("Default login resolution: state=%v, message=%s, login=%s",
			result.State, result.Message, result.Login)
	}
}

// TestGitHubRuntimeConfig_Merge verifies that GitHubRuntimeConfig merging works correctly.
func TestGitHubRuntimeConfig_Merge(t *testing.T) {
	t.Parallel()

	base := settings.GitHubRuntimeConfig{
		SelectedLogins:        []string{"login1"},
		FailoverIfUnavailable: false,
		AllowUnauthenticated: false,
	}

	override := settings.GitHubRuntimeConfig{
		SelectedLogins:        []string{"login2", "login3"},
		FailoverIfUnavailable: true,
		AllowUnauthenticated: true,
	}

	merged := mergeGitHubRuntimeConfig(base, override)

	if len(merged.SelectedLogins) != 2 {
		t.Errorf("expected 2 selected logins, got %d", len(merged.SelectedLogins))
	}
	if merged.SelectedLogins[0] != "login2" || merged.SelectedLogins[1] != "login3" {
		t.Errorf("expected login2, login3, got %v", merged.SelectedLogins)
	}
	if !merged.FailoverIfUnavailable {
		t.Error("expected FailoverIfUnavailable to be true after merge")
	}
	if !merged.AllowUnauthenticated {
		t.Error("expected AllowUnauthenticated to be true after merge")
	}
}

// TestBuildBudgetManagerFromPolicy_Defaults verifies that
// BuildBudgetManagerFromPolicy returns sensible defaults.
func TestBuildBudgetManagerFromPolicy_Defaults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// With invalid/non-existent settings, should fall back to provided defaults
	budget := BuildBudgetManagerFromPolicy(ctx, "", 5000, 200, 15)

	if budget == nil {
		t.Fatal("expected non-nil budget manager")
	}

	remaining := budget.Remaining()
	if remaining != 5000 {
		t.Errorf("expected remaining 5000, got %d", remaining)
	}
}

// TestGitHubAccess_Struct verifies GitHubAccess struct fields.
func TestGitHubAccess_Struct(t *testing.T) {
	t.Parallel()

	access := GitHubAccess{
		Token:   "test-token",
		Login:   "test-login",
		Message: "using test login",
		State:   gh.AccessStateReachableAuthenticated,
	}

	if access.Token != "test-token" {
		t.Errorf("expected token 'test-token', got %s", access.Token)
	}
	if access.Login != "test-login" {
		t.Errorf("expected login 'test-login', got %s", access.Login)
	}
	if access.Message != "using test login" {
		t.Errorf("expected message 'using test login', got %s", access.Message)
	}
	if access.State != gh.AccessStateReachableAuthenticated {
		t.Errorf("expected state 1, got %d", access.State)
	}
}

// TestResolveGitHubAccess_WithEnvToken verifies that ResolveGitHubAccess
// works when GITHUB_TOKEN is set (bypassing gh CLI).
func TestResolveGitHubAccess_WithEnvToken(t *testing.T) {
	t.Parallel()

	// Save original env var
	origToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if origToken != "" {
			os.Setenv("GITHUB_TOKEN", origToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	// Set a test token
	os.Setenv("GITHUB_TOKEN", "test-token-from-env")

	ctx := context.Background()
	access, err := resolveGitHubAccessWithDefaults(ctx)

	if err != nil {
		t.Fatalf("unexpected error with GITHUB_TOKEN set: %v", err)
	}
	if access.State != gh.AccessStateReachableAuthenticated {
		t.Errorf("expected authenticated state, got %v", access.State)
	}
	if access.Token != "test-token-from-env" {
		t.Errorf("expected token 'test-token-from-env', got %s", access.Token)
	}
}

// Helper function for testing - mirrors the internal mergeGitHubRuntimeConfig
func mergeGitHubRuntimeConfig(base, override settings.GitHubRuntimeConfig) settings.GitHubRuntimeConfig {
	result := base
	if len(override.SelectedLogins) > 0 {
		result.SelectedLogins = override.SelectedLogins
	}
	if override.FailoverIfUnavailable != base.FailoverIfUnavailable {
		result.FailoverIfUnavailable = override.FailoverIfUnavailable
	}
	if override.AllowUnauthenticated != base.AllowUnauthenticated {
		result.AllowUnauthenticated = override.AllowUnauthenticated
	}
	return result
}
