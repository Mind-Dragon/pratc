package settings

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAnalyzerConfig_JSON(t *testing.T) {
	config := AnalyzerConfig{
		Enabled: true,
		Timeout: 5 * time.Minute,
		Thresholds: map[string]float64{
			"duplicate": 0.90,
			"overlap":   0.70,
		},
	}

	// Test marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify snake_case keys
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"enabled"`) {
		t.Errorf("expected snake_case 'enabled' key, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"timeout"`) {
		t.Errorf("expected snake_case 'timeout' key, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"thresholds"`) {
		t.Errorf("expected snake_case 'thresholds' key, got: %s", jsonStr)
	}

	// Test round-trip
	var parsed AnalyzerConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Enabled != config.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", parsed.Enabled, config.Enabled)
	}
	if parsed.Timeout != config.Timeout {
		t.Errorf("Timeout mismatch: got %v, want %v", parsed.Timeout, config.Timeout)
	}
	if len(parsed.Thresholds) != len(config.Thresholds) {
		t.Errorf("Thresholds length mismatch: got %d, want %d", len(parsed.Thresholds), len(config.Thresholds))
	}
}

func TestAnalyzerConfig_Defaults(t *testing.T) {
	config := DefaultAnalyzerConfig()

	if !config.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if config.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout to be 5m by default, got %v", config.Timeout)
	}
	if config.Thresholds["duplicate"] != 0.90 {
		t.Errorf("expected duplicate threshold 0.90, got %v", config.Thresholds["duplicate"])
	}
	if config.Thresholds["overlap"] != 0.70 {
		t.Errorf("expected overlap threshold 0.70, got %v", config.Thresholds["overlap"])
	}
}

func TestGitHubRuntimeConfig_Defaults(t *testing.T) {
	cfg := DefaultGitHubRuntimeConfig()

	if cfg.SelectedLogins != nil {
		t.Errorf("expected SelectedLogins to be nil by default, got %v", cfg.SelectedLogins)
	}
	if !cfg.FailoverIfUnavailable {
		t.Error("expected FailoverIfUnavailable to be true by default")
	}
	if cfg.AllowUnauthenticated {
		t.Error("expected AllowUnauthenticated to be false by default")
	}
}

func TestGitHubRatePolicy_Defaults(t *testing.T) {
	cfg := DefaultGitHubRatePolicy()

	if cfg.RateLimit != 5000 {
		t.Errorf("expected RateLimit to be 5000 by default, got %d", cfg.RateLimit)
	}
	if cfg.ReserveBuffer != 200 {
		t.Errorf("expected ReserveBuffer to be 200 by default, got %d", cfg.ReserveBuffer)
	}
	if cfg.ResetBuffer != 15 {
		t.Errorf("expected ResetBuffer to be 15 by default, got %d", cfg.ResetBuffer)
	}
	if cfg.UnauthenticatedRateLimit != 60 {
		t.Errorf("expected UnauthenticatedRateLimit to be 60 by default, got %d", cfg.UnauthenticatedRateLimit)
	}
	if cfg.UnauthenticatedReserveBuffer != 10 {
		t.Errorf("expected UnauthenticatedReserveBuffer to be 10 by default, got %d", cfg.UnauthenticatedReserveBuffer)
	}
}

func TestGitHubRuntimeConfig_JSON(t *testing.T) {
	cfg := GitHubRuntimeConfig{
		SelectedLogins:        []string{"Mind-Dragon", "avirweb"},
		FailoverIfUnavailable: true,
		AllowUnauthenticated: false,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"selected_logins"`) {
		t.Errorf("expected snake_case 'selected_logins' key, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"failover_if_unavailable"`) {
		t.Errorf("expected snake_case 'failover_if_unavailable' key, got: %s", jsonStr)
	}

	// Test round-trip
	var parsed GitHubRuntimeConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(parsed.SelectedLogins) != 2 {
		t.Errorf("SelectedLogins length mismatch: got %d, want 2", len(parsed.SelectedLogins))
	}
	if parsed.SelectedLogins[0] != "Mind-Dragon" || parsed.SelectedLogins[1] != "avirweb" {
		t.Errorf("SelectedLogins mismatch: got %v", parsed.SelectedLogins)
	}
}

func TestGitHubRatePolicy_JSON(t *testing.T) {
	cfg := GitHubRatePolicy{
		RateLimit:                    3000,
		ReserveBuffer:                100,
		ResetBuffer:                  30,
		UnauthenticatedRateLimit:    30,
		UnauthenticatedReserveBuffer: 5,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"rate_limit"`) {
		t.Errorf("expected snake_case 'rate_limit' key, got: %s", jsonStr)
	}

	// Test round-trip
	var parsed GitHubRatePolicy
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.RateLimit != 3000 {
		t.Errorf("RateLimit mismatch: got %d, want 3000", parsed.RateLimit)
	}
	if parsed.UnauthenticatedRateLimit != 30 {
		t.Errorf("UnauthenticatedRateLimit mismatch: got %d, want 30", parsed.UnauthenticatedRateLimit)
	}
}
