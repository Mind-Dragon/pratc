package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// AccessState represents the accessibility state of GitHub.
type AccessState int

const (
	// AccessStateUnknown indicates the access state has not been determined yet.
	AccessStateUnknown AccessState = iota
	// AccessStateReachableAuthenticated indicates GitHub is reachable and at least
	// one login is available and authenticated.
	AccessStateReachableAuthenticated
	// AccessStateReachableUnauthenticated indicates GitHub is reachable but no
	// login is available. Operations may succeed at a lower rate limit.
	AccessStateReachableUnauthenticated
	// AccessStateUnreachable indicates GitHub cannot be reached (network error,
	// offline, etc.). Live operations should be skipped.
	AccessStateUnreachable
)

// String returns a human-readable representation of the access state.
func (s AccessState) String() string {
	switch s {
	case AccessStateUnknown:
		return "unknown"
	case AccessStateReachableAuthenticated:
		return "reachable_authenticated"
	case AccessStateReachableUnauthenticated:
		return "reachable_unauthenticated"
	case AccessStateUnreachable:
		return "unreachable"
	default:
		return "invalid"
	}
}

// AccessStateResult holds the result of an access state check.
type AccessStateResult struct {
	State    AccessState
	Message  string
	Login    string // The login name selected, if any
	Token    string // The resolved token (in-memory only, not exported)
}

// DiscoverAccounts discovers all available GitHub accounts from the gh CLI.
// It returns a list of logins that have authenticated with gh.
func DiscoverAccounts(ctx context.Context) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return nil, fmt.Errorf("gh CLI not found: gh must be installed and in PATH")
	}

	cmd := exec.CommandContext(ctx, path, "auth", "status", "--json", "account")
	output, err := cmd.Output()
	if err != nil {
		// Check if this is a "not logged in" error
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil, fmt.Errorf("no GitHub accounts logged in via gh CLI")
			}
		}
		return nil, fmt.Errorf("gh auth status failed: %w", err)
	}

	// Parse JSON output
	var status struct {
		Accounts []struct {
			Login  string `json:"login"`
			Active bool   `json:"active"`
		} `json:"accounts"`
	}

	if err := json.Unmarshal(output, &status); err != nil {
		// Fall back to parsing text output
		return discoverAccountsFromText(ctx)
	}

	logins := make([]string, 0, len(status.Accounts))
	for _, acct := range status.Accounts {
		logins = append(logins, acct.Login)
	}
	return logins, nil
}

// discoverAccountsFromText parses gh auth status text output to extract accounts.
// This is a fallback when JSON parsing fails.
func discoverAccountsFromText(ctx context.Context) ([]string, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, path, "auth", "status")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh auth status failed: %w", err)
	}

	var logins []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Logged in to github.com account") {
			// Format: "  ✓ Logged in to github.com account <login>"
			parts := strings.Split(line, "account")
			if len(parts) >= 2 {
				login := strings.TrimSpace(parts[len(parts)-1])
				login = strings.Split(login, "(")[0]
				login = strings.TrimSpace(login)
				if login != "" {
					logins = append(logins, login)
				}
			}
		}
	}
	return logins, nil
}

// GetActiveLogin returns the currently active gh CLI login for github.com.
func GetActiveLogin(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh CLI not found: %w", err)
	}

	cmd := exec.CommandContext(ctx, path, "auth", "status", "--json", "account")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh auth status failed: %w", err)
	}

	var status struct {
		Accounts []struct {
			Login  string `json:"login"`
			Active bool   `json:"active"`
		} `json:"accounts"`
	}

	if err := json.Unmarshal(output, &status); err != nil {
		return getActiveLoginFromText(ctx)
	}

	for _, acct := range status.Accounts {
		if acct.Active {
			return acct.Login, nil
		}
	}
	return "", nil
}

// getActiveLoginFromText parses gh auth status text output to find the active login.
func getActiveLoginFromText(ctx context.Context) (string, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, path, "auth", "status")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh auth status failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "- Active account: true") {
			// Look for the login above this line
			continue
		}
		if strings.Contains(line, "github.com account") && !strings.Contains(line, "-") {
			parts := strings.Split(line, "account")
			if len(parts) >= 2 {
				login := strings.TrimSpace(parts[len(parts)-1])
				login = strings.Split(login, "(")[0]
				login = strings.TrimSpace(login)
				return login, nil
			}
		}
	}
	return "", nil
}

// ResolveTokenForLogin returns a token for a specific GitHub login.
// It uses `gh auth token` with the --hostname flag to get the token for the specified account.
func ResolveTokenForLogin(ctx context.Context, login string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh CLI not found: %w", err)
	}

	// Use gh auth token with hostname to get token for specific host
	cmd := exec.CommandContext(ctx, path, "auth", "token", "--hostname", "github.com")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token for login %q: %w", login, err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("token for login %q is empty", login)
	}

	return token, nil
}

// ResolveNamedLogin resolves a GitHub token using config-driven login selection.
// It selects the first available login from selectedLogins, respecting failover policy.
func ResolveNamedLogin(ctx context.Context, selectedLogins []string, failover bool) (AccessStateResult, error) {
	result := AccessStateResult{}

	// If no logins specified, fall back to default token resolution
	if len(selectedLogins) == 0 {
		token, err := ResolveToken(ctx)
		if err != nil {
			result.State = AccessStateUnreachable
			result.Message = "GitHub not accessible"
			return result, err
		}
		result.State = AccessStateReachableAuthenticated
		result.Login = ""
		result.Token = token
		result.Message = "using default GitHub token"
		return result, nil
	}

	// Try each configured login in order
	var lastErr error
	for _, login := range selectedLogins {
		token, err := ResolveTokenForLogin(ctx, login)
		if err != nil {
			lastErr = err
			if !failover {
				continue
			}
			// Try next login
			continue
		}

		result.State = AccessStateReachableAuthenticated
		result.Login = login
		result.Token = token
		result.Message = fmt.Sprintf("using named github login %s", login)
		return result, nil
	}

	// All configured logins failed
	if failover {
		// Try any available login as fallback
		token, err := ResolveToken(ctx)
		if err != nil {
			result.State = AccessStateUnreachable
			result.Message = "GitHub not accessible"
			return result, fmt.Errorf("no configured GitHub login available and gh auth token failed: %w", lastErr)
		}
		result.State = AccessStateReachableAuthenticated
		result.Login = ""
		result.Token = token
		result.Message = "using default GitHub token (configured logins unavailable)"
		return result, nil
	}

	result.State = AccessStateReachableUnauthenticated
	result.Message = fmt.Sprintf("GitHub reachable, no login available from configured list %v; making best efforts at unauthenticated rate", selectedLogins)
	return result, lastErr
}

// CheckAccessState determines the GitHub access state without making API calls.
// It checks if gh CLI is available and if there are any logged-in accounts.
func CheckAccessState(ctx context.Context) AccessState {
	if ctx == nil {
		ctx = context.Background()
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return AccessStateUnreachable
	}

	cmd := exec.CommandContext(ctx, path, "auth", "status")
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				// gh exists but not logged in
				return AccessStateReachableUnauthenticated
			}
		}
		// gh failed to run (network issue, etc.)
		return AccessStateUnreachable
	}

	// gh auth status succeeded - at least one login is available
	return AccessStateReachableAuthenticated
}

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
	tokens, err := DiscoverTokens(ctx)
	if err != nil {
		return "", err
	}
	if len(tokens) == 0 {
		return "", fmt.Errorf("GitHub auth unavailable: no tokens found")
	}
	return tokens[0], nil
}

// DiscoverTokens discovers all available GitHub tokens from configured sources.
// Tokens are returned in priority order.
//
// Discovery sources (in order):
// 1. PRATC_GITHUB_TOKENS - comma-separated list of tokens
// 2. GITHUB_TOKEN, GH_TOKEN, GITHUB_PAT - individual env vars
// 3. `gh auth token` from a logged-in gh CLI session
//
// Returns an error if no tokens are found.
func DiscoverTokens(ctx context.Context) ([]string, error) {
	var tokens []string

	// Check PRATC_GITHUB_TOKENS for comma-separated multi-token support
	if multi := strings.TrimSpace(os.Getenv("PRATC_GITHUB_TOKENS")); multi != "" {
		for _, tok := range strings.Split(multi, ",") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				tokens = append(tokens, tok)
			}
		}
		if len(tokens) > 0 {
			return tokens, nil
		}
	}

	// Check individual env vars
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT"} {
		if token := strings.TrimSpace(os.Getenv(key)); token != "" {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) > 0 {
		return tokens, nil
	}

	// Fall back to gh CLI
	if ctx == nil {
		ctx = context.Background()
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return nil, fmt.Errorf("GitHub auth unavailable: set GITHUB_TOKEN/GH_TOKEN/GITHUB_PAT or sign in with gh auth login")
	}

	cmd := exec.CommandContext(ctx, path, "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("GitHub auth unavailable: gh auth token failed: %w", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return nil, fmt.Errorf("GitHub auth unavailable: gh auth token returned an empty token")
	}

	return []string{token}, nil
}

// MultiTokenSource manages a pool of tokens and rotates through them.
// It is safe for concurrent use.
type MultiTokenSource struct {
	tokens     []string
	current    int
	mu         sync.Mutex
	onExhausted func(string)
}

// NewMultiTokenSource creates a new MultiTokenSource with the given tokens.
// If onExhausted is not nil, it will be called when a token is marked as exhausted.
func NewMultiTokenSource(tokens []string, onExhausted func(string)) *MultiTokenSource {
	filtered := tokens[:0]
	for _, t := range tokens {
		if t != "" {
			filtered = append(filtered, t)
		}
	}
	return &MultiTokenSource{
		tokens:      filtered,
		current:     0,
		onExhausted: onExhausted,
	}
}

// Token returns the next token in the rotation.
func (m *MultiTokenSource) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.tokens) == 0 {
		return "", fmt.Errorf("no tokens available")
	}
	token := m.tokens[m.current]
	m.current = (m.current + 1) % len(m.tokens)
	return token, nil
}

// MarkExhausted notifies the source that a token has been exhausted and should
// be deprioritized. The onExhausted callback is invoked if set.
func (m *MultiTokenSource) MarkExhausted(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.onExhausted != nil {
		m.onExhausted(token)
	}
}

// Remaining returns the number of tokens in the pool.
func (m *MultiTokenSource) Remaining() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tokens)
}

// NewMultiTokenSourceFromDiscovery creates a MultiTokenSource from discovered tokens.
// This is the runtime helper that sync/preflight should use to get multiple tokens
// for sequential fallback on retryable auth/rate-limit failures.
func NewMultiTokenSourceFromDiscovery(ctx context.Context, onExhausted func(string)) (*MultiTokenSource, error) {
	tokens, err := DiscoverTokens(ctx)
	if err != nil {
		return nil, err
	}
	return NewMultiTokenSource(tokens, onExhausted), nil
}

// IsRetryableError returns true if the error is a retryable auth/rate-limit error.
// These are errors where falling through to the next token might help.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 401 Unauthorized, 403 Forbidden (rate limited or auth failure),
	// 5xx errors, and explicit rate limit messages are retryable.
	return strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "Forbidden") ||
		strings.Contains(errStr, "Bad credentials")
}

// AttemptWithTokenFallback attempts an operation with tokens in sequence,
// falling through to the next token when a retryable auth/rate-limit error occurs.
// It returns the first non-retryable error or the last error if all tokens fail.
// The attemptFn is called with each token in order until it succeeds or a
// non-retryable error occurs.
func AttemptWithTokenFallback(ctx context.Context, tokens []string, attemptFn func(token string) error) error {
	if len(tokens) == 0 {
		return fmt.Errorf("no tokens available for attempt")
	}
	var lastErr error
	for i, token := range tokens {
		lastErr = attemptFn(token)
		if lastErr == nil {
			break
		}
		// If not retryable, return immediately
		if !IsRetryableError(lastErr) {
			return lastErr
		}
		// If this is the last token, don't bother trying next
		if i == len(tokens)-1 {
			break
		}
		// Log that we're falling through (caller should log context)
	}
	return lastErr
}
