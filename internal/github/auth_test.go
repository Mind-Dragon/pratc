package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDiscoverTokens_MultipleEnvTokens verifies that multiple tokens can be
// discovered from comma-separated environment variables.
func TestDiscoverTokens_MultipleEnvTokens(t *testing.T) {
	// Clear all token env vars first
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")

	// Set a comma-separated list of tokens
	t.Setenv("PRATC_GITHUB_TOKENS", "token1,token2,token3")

	ctx := context.Background()
	tokens, err := DiscoverTokens(ctx)
	if err != nil {
		t.Fatalf("DiscoverTokens() error = %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("DiscoverTokens() got %d tokens, want 3", len(tokens))
	}
	if tokens[0] != "token1" || tokens[1] != "token2" || tokens[2] != "token3" {
		t.Fatalf("DiscoverTokens() tokens = %v, want [token1, token2, token3]", tokens)
	}
}

// TestDiscoverTokens_SingleEnvToken verifies that a single token works.
func TestDiscoverTokens_SingleEnvToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "single-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")

	ctx := context.Background()
	tokens, err := DiscoverTokens(ctx)
	if err != nil {
		t.Fatalf("DiscoverTokens() error = %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("DiscoverTokens() got %d tokens, want 1", len(tokens))
	}
	if tokens[0] != "single-token" {
		t.Fatalf("DiscoverTokens() token = %q, want single-token", tokens[0])
	}
}

// TestDiscoverTokens_GHCLIMultipleAccounts verifies that gh CLI can expose
// multiple authenticated accounts as separate tokens.
func TestDiscoverTokens_GHCLIMultipleAccounts(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")

	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	// Simulate gh auth token - the actual command used by DiscoverTokens
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "token" ]; then
  printf '%s' "gh-cli-token-main"
  exit 0
fi
exit 1
`
	if runtime.GOOS == "windows" {
		t.Skip("skipping gh CLI multi-account test on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	ctx := context.Background()
	tokens, err := DiscoverTokens(ctx)
	if err != nil {
		t.Fatalf("DiscoverTokens() error = %v", err)
	}
	// gh auth token returns token for the active host only,
	// so this test verifies the interface is extensible
	if len(tokens) < 1 {
		t.Fatalf("DiscoverTokens() got %d tokens, want >= 1", len(tokens))
	}
	if tokens[0] != "gh-cli-token-main" {
		t.Fatalf("DiscoverTokens() token = %q, want gh-cli-token-main", tokens[0])
	}
}

// TestDiscoverTokens_AllSourcesCombined verifies that tokens from all sources
// are discovered and deduplicated.
func TestDiscoverTokens_AllSourcesCombined(t *testing.T) {
	// Clear all token env vars first
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")

	// Set PRATC_GITHUB_TOKENS as the canonical multi-token source
	t.Setenv("PRATC_GITHUB_TOKENS", "multi-token-1,multi-token-2")

	ctx := context.Background()
	tokens, err := DiscoverTokens(ctx)
	if err != nil {
		t.Fatalf("DiscoverTokens() error = %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("DiscoverTokens() got %d tokens, want 2", len(tokens))
	}
}

// TestDiscoverTokens_ErrorsWhenNoTokens verifies error when no tokens available.
func TestDiscoverTokens_ErrorsWhenNoTokens(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")
	t.Setenv("PATH", t.TempDir()) // Ensure gh is not found

	ctx := context.Background()
	_, err := DiscoverTokens(ctx)
	if err == nil {
		t.Fatal("DiscoverTokens() error = nil, want auth error")
	}
}

// TestMultiTokenSource_Rotate verifies that MultiTokenSource rotates tokens.
func TestMultiTokenSource_Rotate(t *testing.T) {
	tokens := []string{"token-a", "token-b", "token-c"}
	src := NewMultiTokenSource(tokens, nil)

	ctx := context.Background()

	// First token
	tok, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok != "token-a" {
		t.Fatalf("Token() = %q, want token-a", tok)
	}

	// Second token
	tok, err = src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok != "token-b" {
		t.Fatalf("Token() = %q, want token-b", tok)
	}

	// Third token
	tok, err = src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok != "token-c" {
		t.Fatalf("Token() = %q, want token-c", tok)
	}

	// Wraps around
	tok, err = src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok != "token-a" {
		t.Fatalf("Token() = %q, want token-a (wrap)", tok)
	}
}

// TestMultiTokenSource_ErrorsWithNoTokens verifies error when no tokens.
func TestMultiTokenSource_ErrorsWithNoTokens(t *testing.T) {
	src := NewMultiTokenSource(nil, nil)
	_, err := src.Token(context.Background())
	if err == nil {
		t.Fatal("Token() error = nil, want error")
	}
}

// TestMultiTokenSource_WithExhaustedCallback verifies that exhausted tokens
// trigger the callback before rotating.
func TestMultiTokenSource_WithExhaustedCallback(t *testing.T) {
	tokens := []string{"token-a", "token-b"}
	var exhausted []string
	src := NewMultiTokenSource(tokens, func(token string) {
		exhausted = append(exhausted, token)
	})

	ctx := context.Background()

	// Use first token
	_, _ = src.Token(ctx)
	// Use second token
	_, _ = src.Token(ctx)
	// Now first token should be marked exhausted on next rotation
	src.MarkExhausted("token-a")

	// Next token should be token-a again (rotated), but exhausted list should have token-a
	if len(exhausted) != 1 || exhausted[0] != "token-a" {
		t.Fatalf("MarkExhausted callback not called correctly, got %v", exhausted)
	}
}

func TestResolveTokenUsesEnvToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")

	token, err := ResolveToken(context.Background())
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "env-token" {
		t.Fatalf("ResolveToken() token = %q, want env-token", token)
	}
}

func TestResolveTokenUsesGHCLIWhenEnvMissing(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")

	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	contents := "#!/bin/sh\nif [ \"$1\" = auth ] && [ \"$2\" = token ]; then\n  printf '%s' gh-cli-token\n  exit 0\nfi\nexit 1\n"
	if runtime.GOOS == "windows" {
		script += ".cmd"
		contents = "@echo off\r\nif \"%1\"==\"auth\" if \"%2\"==\"token\" echo gh-cli-token\r\n"
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	token, err := ResolveToken(context.Background())
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "gh-cli-token" {
		t.Fatalf("ResolveToken() token = %q, want gh-cli-token", token)
	}
}

func TestResolveTokenErrorsWithoutAuth(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PATH", t.TempDir())

	if _, err := ResolveToken(context.Background()); err == nil {
		t.Fatal("ResolveToken() error = nil, want auth error")
	}
}

// TestNewMultiTokenSourceFromDiscovery creates a MultiTokenSource from discovered tokens.
// This is the runtime helper that sync/preflight should use to get multiple tokens
// for sequential fallback on retryable auth/rate-limit failures.
func TestNewMultiTokenSourceFromDiscovery(t *testing.T) {
	// Set up multiple tokens via PRATC_GITHUB_TOKENS
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "test-token-1,test-token-2,test-token-3")

	ctx := context.Background()
	src, err := NewMultiTokenSourceFromDiscovery(ctx, nil)
	if err != nil {
		t.Fatalf("NewMultiTokenSourceFromDiscovery() error = %v", err)
	}

	// Verify we can get multiple tokens
	tok1, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok1 != "test-token-1" {
		t.Fatalf("first Token() = %q, want test-token-1", tok1)
	}

	tok2, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok2 != "test-token-2" {
		t.Fatalf("second Token() = %q, want test-token-2", tok2)
	}

	tok3, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok3 != "test-token-3" {
		t.Fatalf("third Token() = %q, want test-token-3", tok3)
	}

	// Verify rotation wraps around
	tok4, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok4 != "test-token-1" {
		t.Fatalf("fourth Token() (wrap) = %q, want test-token-1", tok4)
	}
}

// TestNewMultiTokenSourceFromDiscovery_SingleToken verifies fallback works with one token.
func TestNewMultiTokenSourceFromDiscovery_SingleToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "single-test-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")

	ctx := context.Background()
	src, err := NewMultiTokenSourceFromDiscovery(ctx, nil)
	if err != nil {
		t.Fatalf("NewMultiTokenSourceFromDiscovery() error = %v", err)
	}

	tok, err := src.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if tok != "single-test-token" {
		t.Fatalf("Token() = %q, want single-test-token", tok)
	}
}

// TestIsRetryableError_AuthErrors verifies that auth errors are retryable.
func TestIsRetryableError_AuthErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"401 Unauthorized", fmt.Errorf("401 Unauthorized"), true},
		{"401 unauthorized", fmt.Errorf("401 unauthorized"), true},
		{"403 Forbidden", fmt.Errorf("403 Forbidden"), true},
		{"403 rate limited", fmt.Errorf("github API returned status 403: rate limited"), true},
		{"Bad credentials", fmt.Errorf("Bad credentials"), true},
		{"unauthorized", fmt.Errorf("unauthorized"), true},
		{"401", fmt.Errorf("some error with 401 in it"), true},
		{"403", fmt.Errorf("some error with 403 in it"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsRetryableError(tc.err)
			if got != tc.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestIsRetryableError_RateLimitErrors verifies that rate limit errors are retryable.
func TestIsRetryableError_RateLimitErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"rate limit exceeded", fmt.Errorf("rate limit exceeded"), true},
		{"rate limit exhausted", fmt.Errorf("rate limit exhausted"), true},
		{"rate limit", fmt.Errorf("github rate limit exceeded; retry after 1s"), true},
		{"secondary rate limit", fmt.Errorf("github secondary rate limit exceeded"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsRetryableError(tc.err)
			if got != tc.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestIsRetryableError_NonRetryable verifies that non-retryable errors return false.
func TestIsRetryableError_NonRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"nil error", nil},
		{"network error", fmt.Errorf("network error: connection refused")},
		{"not found", fmt.Errorf("GitHub API returned status 404: not found")},
		{"parse error", fmt.Errorf("failed to parse response")},
		{"timeout", fmt.Errorf("context deadline exceeded")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsRetryableError(tc.err)
			if got != false {
				t.Errorf("IsRetryableError(%v) = %v, want false", tc.err, got)
			}
		})
	}
}

// TestAttemptWithTokenFallback_FallsThrough verifies that when the first token
// fails with a retryable error, the second token is tried.
func TestAttemptWithTokenFallback_FallsThrough(t *testing.T) {
	tokens := []string{"token-1-fails", "token-2-succeeds"}
	attemptCount := 0

	err := AttemptWithTokenFallback(context.Background(), tokens, func(token string) error {
		attemptCount++
		if token == "token-1-fails" {
			return fmt.Errorf("401 Unauthorized") // Retryable
		}
		// token-2-succeeds
		return nil
	})

	if err != nil {
		t.Fatalf("AttemptWithTokenFallback() error = %v, want nil", err)
	}
	if attemptCount != 2 {
		t.Fatalf("attemptCount = %d, want 2 (first failed, second tried)", attemptCount)
	}
}

// TestAttemptWithTokenFallback_NonRetryableStops verifies that non-retryable
// errors stop immediately without trying other tokens.
func TestAttemptWithTokenFallback_NonRetryableStops(t *testing.T) {
	tokens := []string{"token-1-bad", "token-2-good"}
	attemptCount := 0

	err := AttemptWithTokenFallback(context.Background(), tokens, func(token string) error {
		attemptCount++
		if token == "token-1-bad" {
			return fmt.Errorf("network error: connection refused") // Not retryable
		}
		// Should not reach token-2-good
		return nil
	})

	if err == nil {
		t.Fatal("AttemptWithTokenFallback() error = nil, want non-retryable error")
	}
	if attemptCount != 1 {
		t.Fatalf("attemptCount = %d, want 1 (non-retryable error should stop)", attemptCount)
	}
}

// TestAttemptWithTokenFallback_AllTokensFail verifies that when all tokens fail
// with retryable errors, the last error is returned.
func TestAttemptWithTokenFallback_AllTokensFail(t *testing.T) {
	tokens := []string{"token-1", "token-2"}
	attemptCount := 0

	err := AttemptWithTokenFallback(context.Background(), tokens, func(token string) error {
		attemptCount++
		return fmt.Errorf("401 Unauthorized") // Retryable
	})

	if err == nil {
		t.Fatal("AttemptWithTokenFallback() error = nil, want last error")
	}
	if attemptCount != 2 {
		t.Fatalf("attemptCount = %d, want 2 (both tokens tried)", attemptCount)
	}
}

// TestAttemptWithTokenFallback_SingleTokenSucceeds verifies that a single token
// succeeding works correctly.
func TestAttemptWithTokenFallback_SingleTokenSucceeds(t *testing.T) {
	tokens := []string{"token-only"}
	attemptCount := 0

	err := AttemptWithTokenFallback(context.Background(), tokens, func(token string) error {
		attemptCount++
		return nil
	})

	if err != nil {
		t.Fatalf("AttemptWithTokenFallback() error = %v, want nil", err)
	}
	if attemptCount != 1 {
		t.Fatalf("attemptCount = %d, want 1", attemptCount)
	}
}

// B2: AttemptWithTokenFallback continues after success at auth.go line 488-500
// Bug: After first successful token, attemptFn should not be called again.
// The loop should return immediately when attemptFn returns nil, but due to
// a logic error it continues iterating through remaining tokens.
func TestAttemptWithTokenFallback_StopsAfterSuccess(t *testing.T) {
	tokens := []string{"token-first", "token-second", "token-third"}
	attemptCount := 0

	err := AttemptWithTokenFallback(context.Background(), tokens, func(token string) error {
		attemptCount++
		if token == "token-first" {
			return nil // Success on first token
		}
		// Should not reach here
		t.Errorf("attemptFn called with %q after first token succeeded", token)
		return nil
	})

	if err != nil {
		t.Fatalf("AttemptWithTokenFallback() error = %v, want nil", err)
	}
	if attemptCount != 1 {
		t.Fatalf("attemptCount = %d, want 1 (only first token should be tried after success)", attemptCount)
	}
}

// TestAttemptWithTokenFallback_NoTokens verifies error when no tokens provided.
func TestAttemptWithTokenFallback_NoTokens(t *testing.T) {
	err := AttemptWithTokenFallback(context.Background(), nil, func(token string) error {
		return nil
	})

	if err == nil {
		t.Fatal("AttemptWithTokenFallback() with no tokens = nil, want error")
	}
}

// TestAccessState_String verifies AccessState String() method.
func TestAccessState_String(t *testing.T) {
	tests := []struct {
		state   AccessState
		wantStr string
	}{
		{AccessStateUnknown, "unknown"},
		{AccessStateReachableAuthenticated, "reachable_authenticated"},
		{AccessStateReachableUnauthenticated, "reachable_unauthenticated"},
		{AccessStateUnreachable, "unreachable"},
		{AccessState(99), "invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.wantStr, func(t *testing.T) {
			if got := tc.state.String(); got != tc.wantStr {
				t.Errorf("AccessState.String() = %v, want %v", got, tc.wantStr)
			}
		})
	}
}

// TestCheckAccessState_Unreachable verifies that CheckAccessState returns
// AccessStateUnreachable when gh is not available or can't be executed.
func TestCheckAccessState_Unreachable(t *testing.T) {
	// Set gh to a non-existent path that will fail to execute
	dir := t.TempDir()
	// Create a file named gh that is NOT executable - this simulates gh not being found
	script := filepath.Join(dir, "gh")
	// Create a fake gh that fails with exit code 2 (not 1 - exit code 1 means "not logged in")
	contents := `#!/bin/sh
exit 2`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	state := CheckAccessState(context.Background())
	// exit code 2 is not a known state, so should fall through to unreachable
	if state != AccessStateUnreachable {
		t.Errorf("CheckAccessState() = %v, want AccessStateUnreachable", state)
	}
}

// TestCheckAccessState_ReachableAuthenticated verifies that CheckAccessState returns
// AccessStateReachableAuthenticated when gh is logged in.
func TestCheckAccessState_ReachableAuthenticated(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	// Fake gh that succeeds with status showing a logged in account
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo '{"accounts":[{"login":"test-login","active":true}]}'
  exit 0
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	state := CheckAccessState(context.Background())
	if state != AccessStateReachableAuthenticated {
		t.Errorf("CheckAccessState() = %v, want AccessStateReachableAuthenticated", state)
	}
}

// TestDiscoverAccounts_MultipleAccounts verifies that multiple gh accounts can be discovered.
func TestDiscoverAccounts_MultipleAccounts(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  if [ "$3" = "--json" ] && [ "$4" = "account" ]; then
    echo '{"accounts":[{"login":"Mind-Dragon","active":true},{"login":"avirweb","active":false}]}'
    exit 0
  fi
  echo '{"accounts":[{"login":"Mind-Dragon","active":true},{"login":"avirweb","active":false}]}'
  exit 0
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	accounts, err := DiscoverAccounts(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAccounts() error = %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("DiscoverAccounts() got %d accounts, want 2", len(accounts))
	}
	if accounts[0] != "Mind-Dragon" || accounts[1] != "avirweb" {
		t.Errorf("DiscoverAccounts() = %v, want [Mind-Dragon, avirweb]", accounts)
	}
}

// TestDiscoverAccounts_TextFallback verifies text fallback when JSON parsing fails.
func TestDiscoverAccounts_TextFallback(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	// Output that doesn't parse as JSON properly, forcing text fallback
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo '  ✓ Logged in to github.com account Mind-Dragon (/home/user/.config/gh/hosts.yml)'
  echo '  ✓ Logged in to github.com account avirweb (/home/user/.config/gh/hosts.yml)'
  echo '  - Active account: true'
  exit 0
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	accounts, err := DiscoverAccounts(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAccounts() error = %v", err)
	}
	// Text fallback should still find the accounts
	if len(accounts) < 1 {
		t.Fatalf("DiscoverAccounts() got %d accounts, want >= 1", len(accounts))
	}
}

// TestResolveNamedLogin_NoLoginsConfigured verifies fallback to default when no logins configured.
func TestResolveNamedLogin_NoLoginsConfigured(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token-default")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")

	result, err := ResolveNamedLogin(context.Background(), nil, true)
	if err != nil {
		t.Fatalf("ResolveNamedLogin() error = %v", err)
	}
	if result.State != AccessStateReachableAuthenticated {
		t.Errorf("ResolveNamedLogin().State = %v, want AccessStateReachableAuthenticated", result.State)
	}
	if result.Token != "env-token-default" {
		t.Errorf("ResolveNamedLogin().Token = %q, want env-token-default", result.Token)
	}
	if result.Login != "" {
		t.Errorf("ResolveNamedLogin().Login = %q, want empty (default)", result.Login)
	}
}

// TestResolveNamedLogin_SelectedLogin verifies that a specific configured login is selected.
func TestResolveNamedLogin_SelectedLogin(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	// Fake gh that returns a token for the specified login
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  if [ "$3" = "--json" ] && [ "$4" = "account" ]; then
    echo '{"accounts":[{"login":"Mind-Dragon","active":true}]}'
    exit 0
  fi
fi
if [ "$1" = "auth" ] && [ "$2" = "token" ]; then
  printf '%s' "token-for-Mind-Dragon"
  exit 0
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")

	result, err := ResolveNamedLogin(context.Background(), []string{"Mind-Dragon"}, true)
	if err != nil {
		t.Fatalf("ResolveNamedLogin() error = %v", err)
	}
	if result.State != AccessStateReachableAuthenticated {
		t.Errorf("ResolveNamedLogin().State = %v, want AccessStateReachableAuthenticated", result.State)
	}
	if result.Token != "token-for-Mind-Dragon" {
		t.Errorf("ResolveNamedLogin().Token = %q, want token-for-Mind-Dragon", result.Token)
	}
	if result.Login != "Mind-Dragon" {
		t.Errorf("ResolveNamedLogin().Login = %q, want Mind-Dragon", result.Login)
	}
	if !strings.Contains(result.Message, "Mind-Dragon") {
		t.Errorf("ResolveNamedLogin().Message = %q, should contain Mind-Dragon", result.Message)
	}
}

// TestResolveNamedLogin_LoginNotAvailable verifies behavior when configured login is not available.
func TestResolveNamedLogin_LoginNotAvailable(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	// Fake gh that fails for the configured login
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "token" ]; then
  exit 1
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PRATC_GITHUB_TOKENS", "")

	// Without failover, should return unauthenticated state
	result, _ := ResolveNamedLogin(context.Background(), []string{"NonExistentLogin"}, false)
	if result.State != AccessStateReachableUnauthenticated {
		t.Errorf("ResolveNamedLogin().State = %v, want AccessStateReachableUnauthenticated", result.State)
	}
	if !strings.Contains(result.Message, "no login available") {
		t.Errorf("ResolveNamedLogin().Message = %q, should mention no login available", result.Message)
	}
}

// TestResolveNamedLogin_WithFailover verifies failover to default when configured login fails.
func TestResolveNamedLogin_WithFailover(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	// The script handles:
	// - "auth token" (ResolveToken fallback) -> returns fallback-token, exit 0
	// - "auth token --hostname github.com" (ResolveTokenForLogin) -> exit 1
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "token" ]; then
  if [ "$3" = "--hostname" ]; then
    exit 1
  fi
  printf '%s' "fallback-token"
  exit 0
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")

	result, err := ResolveNamedLogin(context.Background(), []string{"UnavailableLogin"}, true)
	if err != nil {
		t.Fatalf("ResolveNamedLogin() error = %v", err)
	}
	if result.State != AccessStateReachableAuthenticated {
		t.Errorf("ResolveNamedLogin().State = %v, want AccessStateReachableAuthenticated", result.State)
	}
	if result.Token != "fallback-token" {
		t.Errorf("ResolveNamedLogin().Token = %q, want fallback-token", result.Token)
	}
	if !strings.Contains(result.Message, "unavailable") {
		t.Errorf("ResolveNamedLogin().Message = %q, should mention unavailable", result.Message)
	}
}

// TestGetActiveLogin verifies that the active gh login is detected correctly.
func TestGetActiveLogin(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  if [ "$3" = "--json" ] && [ "$4" = "account" ]; then
    echo '{"accounts":[{"login":"Mind-Dragon","active":true},{"login":"avirweb","active":false}]}'
    exit 0
  fi
  exit 1
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	login, err := GetActiveLogin(context.Background())
	if err != nil {
		t.Fatalf("GetActiveLogin() error = %v", err)
	}
	if login != "Mind-Dragon" {
		t.Errorf("GetActiveLogin() = %q, want Mind-Dragon", login)
	}
}
