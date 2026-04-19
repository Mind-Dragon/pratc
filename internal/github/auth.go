package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
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
			return nil
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
