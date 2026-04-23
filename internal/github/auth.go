package github

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
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
	State   AccessState
	Message string
	Login   string // The login name selected, if any
	Token   string // The resolved token (in-memory only, not exported)
}

// TokenInfo describes a discovered GitHub token without exposing the token in logs.
type TokenInfo struct {
	Token       string
	Source      string
	Fingerprint string
}

func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:12]
}

func addTokenInfo(tokens *[]TokenInfo, seen map[string]struct{}, token, source string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	if _, ok := seen[token]; ok {
		return
	}
	seen[token] = struct{}{}
	*tokens = append(*tokens, TokenInfo{Token: token, Source: source, Fingerprint: tokenFingerprint(token)})
}

func addTokenList(tokens *[]TokenInfo, seen map[string]struct{}, value, source string) {
	for _, tok := range strings.Split(value, ",") {
		addTokenInfo(tokens, seen, tok, source)
	}
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
	login = strings.TrimSpace(login)
	if login != "" {
		active, err := GetActiveLogin(ctx)
		if err != nil {
			return "", fmt.Errorf("resolve active login for %q: %w", login, err)
		}
		if active != "" && active != login {
			return "", fmt.Errorf("requested login %q is not the active gh account (%q)", login, active)
		}
	}

	// Use gh auth token with hostname to get token for specific host.
	// gh currently returns the token for the active login on that host, so we
	// explicitly verify the active account above instead of silently returning
	// the wrong login's token.
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
// Tokens are returned in priority order and deduplicated.
func DiscoverTokens(ctx context.Context) ([]string, error) {
	infos, err := DiscoverTokenInfos(ctx)
	if err != nil {
		return nil, err
	}
	tokens := make([]string, 0, len(infos))
	for _, info := range infos {
		tokens = append(tokens, info.Token)
	}
	return tokens, nil
}

// DiscoverTokenInfos discovers GitHub tokens with redaction-safe source metadata.
// Sources, in priority order:
//  1. PRATC_GITHUB_TOKENS, GITHUB_TOKEN, GH_TOKEN, GITHUB_PAT
//  2. local .env-style files
//  3. prATC settings DB token keys
//  4. every cached gh account in ~/.config/gh/hosts.yml
//  5. gh auth token fallback for the active account
func DiscoverTokenInfos(ctx context.Context) ([]TokenInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var infos []TokenInfo
	seen := map[string]struct{}{}

	addTokenList(&infos, seen, os.Getenv("PRATC_GITHUB_TOKENS"), "env:PRATC_GITHUB_TOKENS")
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT"} {
		addTokenInfo(&infos, seen, os.Getenv(key), "env:"+key)
	}

	for _, envPath := range dotenvPaths() {
		loadTokensFromDotenv(envPath, &infos, seen)
	}
	loadTokensFromSettingsDB(ctx, &infos, seen)
	loadTokensFromGHHosts(&infos, seen)
	loadTokenFromGHCLI(ctx, &infos, seen)

	if len(infos) == 0 {
		return nil, fmt.Errorf("GitHub auth unavailable: no tokens found in env, .env, prATC config, gh hosts, or gh auth token")
	}
	return infos, nil
}

func dotenvPaths() []string {
	paths := []string{".env", ".env.local"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".pratc", ".env"), filepath.Join(home, ".config", "pratc", ".env"))
	}
	return paths
}

func loadTokensFromDotenv(path string, infos *[]TokenInfo, seen map[string]struct{}) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch key {
		case "PRATC_GITHUB_TOKENS":
			addTokenList(infos, seen, value, "dotenv:"+path+":"+key)
		case "GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT":
			addTokenInfo(infos, seen, value, "dotenv:"+path+":"+key)
		}
	}
}

func loadTokensFromSettingsDB(ctx context.Context, infos *[]TokenInfo, seen map[string]struct{}) {
	paths := []string{strings.TrimSpace(os.Getenv("PRATC_SETTINGS_DB")), "pratc-settings.db"}
	for _, dbPath := range paths {
		if dbPath == "" {
			continue
		}
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			continue
		}
		rows, err := db.QueryContext(ctx, `SELECT key, value_json FROM settings WHERE key IN ('github_tokens','github_token','github_auth','github_runtime')`)
		if err == nil {
			for rows.Next() {
				var key, valueJSON string
				if rows.Scan(&key, &valueJSON) == nil {
					addTokensFromJSONValue(infos, seen, valueJSON, "settings:"+dbPath+":"+key)
				}
			}
			_ = rows.Close()
		}
		_ = db.Close()
	}
}

func addTokensFromJSONValue(infos *[]TokenInfo, seen map[string]struct{}, raw, source string) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return
	}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			addTokenList(infos, seen, t, source)
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			for k, item := range t {
				lk := strings.ToLower(k)
				if strings.Contains(lk, "token") || strings.Contains(lk, "pat") {
					walk(item)
				}
			}
		}
	}
	walk(v)
}

func loadTokensFromGHHosts(infos *[]TokenInfo, seen map[string]struct{}) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "gh", "hosts.yml"))
	if err != nil {
		return
	}
	var hosts map[string]struct {
		OAuthToken string `yaml:"oauth_token"`
		User       string `yaml:"user"`
		Users      map[string]struct {
			OAuthToken string `yaml:"oauth_token"`
		} `yaml:"users"`
	}
	if err := yaml.Unmarshal(data, &hosts); err != nil {
		return
	}
	for host, entry := range hosts {
		if entry.OAuthToken != "" {
			name := entry.User
			if name == "" {
				name = "active"
			}
			addTokenInfo(infos, seen, entry.OAuthToken, "gh-hosts:"+host+":"+name)
		}
		for login, user := range entry.Users {
			addTokenInfo(infos, seen, user.OAuthToken, "gh-hosts:"+host+":"+login)
		}
	}
}

func loadTokenFromGHCLI(ctx context.Context, infos *[]TokenInfo, seen map[string]struct{}) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return
	}
	cmd := exec.CommandContext(ctx, path, "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	addTokenInfo(infos, seen, string(output), "gh-cli:active")
}

// MultiTokenSource manages a pool of tokens and rotates through them.
// It is safe for concurrent use.
type MultiTokenSource struct {
	tokens      []string
	current     int
	mu          sync.Mutex
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

// MarkFailed notifies the source that a token has been exhausted and should
// be deprioritized. The onExhausted callback is invoked if set.
func (m *MultiTokenSource) MarkFailed(token string) {
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
	// Auth failures: 401, bad credentials — next token might work.
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "Bad credentials") || strings.Contains(errStr, "unauthorized") {
		return true
	}
	// Rate limit: 403 with rate-limit context — next token might have quota.
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "rate_limit") {
		return true
	}
	// Generic 403 (Forbidden) without rate-limit context is NOT retryable
	// because it likely means the token lacks permission, not that it's exhausted.
	return false
}

// TokenSource provides tokens for GitHub API requests.
// Implementations may rotate through multiple tokens.
type TokenSource interface {
	// Token returns the current token.
	Token(ctx context.Context) (string, error)
	// MarkFailed notifies the source that a token failed with a retryable error.
	// The source may rotate to the next token.
	MarkFailed(token string)
}

// singleTokenSource is a TokenSource backed by a single token.
type singleTokenSource struct {
	token string
}

func (s *singleTokenSource) Token(ctx context.Context) (string, error) {
	if s.token == "" {
		return "", fmt.Errorf("no token available")
	}
	return s.token, nil
}

func (s *singleTokenSource) MarkFailed(token string) {
	// Single token — nothing to rotate to.
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
