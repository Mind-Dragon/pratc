package settings

import (
	"encoding/json"
	"time"
)

// GitHubLogin represents a GitHub login/account entry with selection state.
type GitHubLogin struct {
	// Login is the GitHub username (e.g., "Mind-Dragon").
	Login string `json:"login"`
	// Selected indicates this login is currently selected for use.
	Selected bool `json:"selected"`
	// Active indicates this login is the gh CLI active account.
	Active bool `json:"active"`
}

// GitHubRuntimeConfig holds GitHub runtime configuration including login selection
// and failover policy.
type GitHubRuntimeConfig struct {
	// SelectedLogins specifies which GitHub logins to use, in priority order.
	// The first available login from this list will be selected at runtime.
	// Example: ["Mind-Dragon", "avirweb"]
	SelectedLogins []string `json:"selected_logins"`

	// FailoverIfUnavailable controls whether to fall through to the next login
	// in SelectedLogins when a login is unavailable or rate-limited.
	FailoverIfUnavailable bool `json:"failover_if_unavailable"`

	// AllowUnauthenticated controls whether to proceed without authentication
	// when no configured login is available. When false (default), an error
	// is returned if no login can be selected.
	AllowUnauthenticated bool `json:"allow_unauthenticated"`

	// ResyncLogins instructs the runtime to force a live refresh of the gh auth
	// context when true. When false (default), cached login information is used.
	// This is set by the --resync flag and is not persisted.
	ResyncLogins bool `json:"-"`
}

// DefaultGitHubRuntimeConfig returns a sensible default configuration.
func DefaultGitHubRuntimeConfig() GitHubRuntimeConfig {
	return GitHubRuntimeConfig{
		SelectedLogins:        nil, // No default selection; requires explicit config
		FailoverIfUnavailable: true,
		AllowUnauthenticated: false,
		ResyncLogins:         false,
	}
}

// GitHubRatePolicy holds GitHub API rate limit configuration.
type GitHubRatePolicy struct {
	// RateLimit is the total API requests allowed per hour (authenticated: 5000).
	RateLimit int `json:"rate_limit"`
	// ReserveBuffer is the minimum requests to keep in reserve.
	// Defaults to 200.
	ReserveBuffer int `json:"reserve_buffer"`
	// ResetBuffer is the seconds to wait after rate limit reset before resuming.
	// Defaults to 15.
	ResetBuffer int `json:"reset_buffer"`
	// UnauthenticatedRateLimit is the rate limit when operating without
	// a GitHub login (60 requests/hour for unauthenticated).
	UnauthenticatedRateLimit int `json:"unauthenticated_rate_limit"`
	// UnauthenticatedReserveBuffer is the reserve for unauthenticated mode.
	UnauthenticatedReserveBuffer int `json:"unauthenticated_reserve_buffer"`
}

// DefaultGitHubRatePolicy returns a sensible default rate policy.
func DefaultGitHubRatePolicy() GitHubRatePolicy {
	return GitHubRatePolicy{
		RateLimit:                   5000,
		ReserveBuffer:               200,
		ResetBuffer:                 15,
		UnauthenticatedRateLimit:    60,
		UnauthenticatedReserveBuffer: 10,
	}
}

// AnalyzerConfig holds configuration for the PR analyzer.
// It controls analyzer behavior including timeouts and category-specific thresholds.
type AnalyzerConfig struct {
	// Enabled indicates whether the analyzer is active.
	Enabled bool `json:"enabled"`

	// Timeout specifies the maximum duration for analyzer operations.
	Timeout time.Duration `json:"timeout"`

	// Thresholds maps category names to confidence thresholds (0.0-1.0).
	// Categories might include "duplicate", "overlap", "conflict", etc.
	Thresholds map[string]float64 `json:"thresholds"`
}

// DefaultAnalyzerConfig returns a sensible default configuration.
func DefaultAnalyzerConfig() AnalyzerConfig {
	return AnalyzerConfig{
		Enabled: true,
		Timeout: 5 * time.Minute,
		Thresholds: map[string]float64{
			"duplicate": 0.90,
			"overlap":   0.70,
		},
	}
}

// MarshalJSON implements custom JSON marshaling to handle duration serialization.
func (c AnalyzerConfig) MarshalJSON() ([]byte, error) {
	type Alias AnalyzerConfig
	return json.Marshal(&struct {
		Timeout string `json:"timeout"`
		*Alias
	}{
		Timeout: c.Timeout.String(),
		Alias:   (*Alias)(&c),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling to handle duration parsing.
func (c *AnalyzerConfig) UnmarshalJSON(data []byte) error {
	type Alias AnalyzerConfig
	aux := &struct {
		Timeout string `json:"timeout"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Timeout != "" {
		d, err := time.ParseDuration(aux.Timeout)
		if err != nil {
			return err
		}
		c.Timeout = d
	}
	return nil
}
