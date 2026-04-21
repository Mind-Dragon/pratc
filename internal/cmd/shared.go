package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/audit"
	"github.com/jeffersonnunn/pratc/internal/cache"
	gh "github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/settings"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/spf13/cobra"
)

// GitHubAccess holds the resolved GitHub access state for runtime use.
type GitHubAccess struct {
	Token   string // Resolved token (in-memory only)
	Login   string // Selected login name (if any)
	Message string // Human-readable access state message
	State   gh.AccessState
}

// ResolveGitHubAccess resolves GitHub access using settings-driven config.
// It checks access state, resolves named logins from config, and returns
// the token along with truthful messaging about the access state.
// Tokens are never exported to env or written to disk.
func ResolveGitHubAccess(ctx context.Context, repo string) (GitHubAccess, error) {
	access := GitHubAccess{State: gh.AccessStateUnknown}

	// First check if GitHub is reachable at all
	access.State = gh.CheckAccessState(ctx)
	if access.State == gh.AccessStateUnreachable {
		access.Message = "GitHub unreachable (offline or network issue)"
		return access, fmt.Errorf("github: %s", access.Message)
	}

	// Load GitHub runtime config from settings
	store, err := openSettingsStore()
	if err != nil {
		// Fall back to default token resolution if settings unavailable
		return resolveGitHubAccessWithDefaults(ctx)
	}
	defer store.Close()

	runtimeCfg, err := store.GetGitHubRuntimeConfig(ctx, repo)
	if err != nil {
		return resolveGitHubAccessWithDefaults(ctx)
	}

	ratePolicy, err := store.GetGitHubRatePolicy(ctx, repo)
	if err != nil {
		ratePolicy = settings.DefaultGitHubRatePolicy()
	}

	// Use config values for rate policy
	_ = ratePolicy // consumed by caller via BudgetManager

	// Resolve named login using configured logins
	result, err := gh.ResolveNamedLogin(ctx, runtimeCfg.SelectedLogins, runtimeCfg.FailoverIfUnavailable)
	if err != nil && result.State == gh.AccessStateUnreachable {
		access.Message = "GitHub not accessible"
		return access, fmt.Errorf("github: %s", access.Message)
	}

	access.Token = result.Token
	access.Login = result.Login
	access.Message = result.Message
	access.State = result.State

	return access, nil
}

// resolveGitHubAccessWithDefaults resolves GitHub access using only environment/gh CLI.
func resolveGitHubAccessWithDefaults(ctx context.Context) (GitHubAccess, error) {
	access := GitHubAccess{State: gh.AccessStateUnknown}

	state := gh.CheckAccessState(ctx)
	if state == gh.AccessStateUnreachable {
		access.Message = "GitHub unreachable (offline or network issue)"
		return access, fmt.Errorf("github: %s", access.Message)
	}

	token, err := gh.ResolveToken(ctx)
	if err != nil {
		access.Message = "GitHub reachable but no login available"
		access.State = gh.AccessStateReachableUnauthenticated
		return access, err
	}

	access.Token = token
	access.Message = "using default GitHub token"
	access.State = gh.AccessStateReachableAuthenticated
	return access, nil
}

// BuildBudgetManagerFromPolicy creates a BudgetManager using rate policy from settings.
// If settings are unavailable, it falls back to the provided defaults.
func BuildBudgetManagerFromPolicy(ctx context.Context, repo string, defaultRateLimit, defaultReserve, defaultReset int) *ratelimit.BudgetManager {
	store, err := openSettingsStore()
	if err != nil {
		return ratelimit.NewBudgetManager(
			ratelimit.WithRateLimit(defaultRateLimit),
			ratelimit.WithReserveBuffer(defaultReserve),
			ratelimit.WithResetBuffer(defaultReset),
		)
	}
	defer store.Close()

	policy, err := store.GetGitHubRatePolicy(ctx, repo)
	if err != nil {
		policy = settings.DefaultGitHubRatePolicy()
	}

	return ratelimit.NewBudgetManager(
		ratelimit.WithRateLimit(policy.RateLimit),
		ratelimit.WithReserveBuffer(policy.ReserveBuffer),
		ratelimit.WithResetBuffer(policy.ResetBuffer),
	)
}

func writeJSON(cmd *cobra.Command, payload any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func openSettingsStore() (*settings.Store, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_SETTINGS_DB"))
	if path == "" {
		path = "pratc-settings.db"
	}
	store, err := settings.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open settings store: %w", err)
	}
	return store, nil
}

func openAuditStore() (*cache.AuditStore, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return cache.NewAuditStore(store), nil
}

func logAuditEntry(action, repo, details string) {
	auditStore, err := openAuditStore()
	if err != nil {
		return
	}
	defer auditStore.Close()
	entry := audit.AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Repo:      repo,
		Details:   details,
	}
	_ = auditStore.Append(entry)
}
