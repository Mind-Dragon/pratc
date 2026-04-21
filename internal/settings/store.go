package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

const (
	ScopeGlobal = "global"
	ScopeRepo   = "repo"
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open settings database: %w", err)
	}
	s := &Store{db: db, now: func() time.Time { return time.Now().UTC() }}
	if err := s.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scope TEXT NOT NULL CHECK(scope IN ('global', 'repo')),
			repo TEXT NOT NULL DEFAULT '',
			key TEXT NOT NULL,
			value_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(scope, repo, key)
		);
		CREATE INDEX IF NOT EXISTS idx_settings_scope_repo ON settings(scope, repo);
	`)
	if err != nil {
		return fmt.Errorf("initialize settings schema: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, repo string) (map[string]any, error) {
	result := map[string]any{}
	if err := s.loadInto(ctx, result, ScopeGlobal, ""); err != nil {
		return nil, err
	}
	if repo != "" {
		if err := s.loadInto(ctx, result, ScopeRepo, repo); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// List returns all settings for the given scope and repo.
// Use scope=ScopeGlobal with empty repo for global settings,
// or scope=ScopeRepo with "owner/repo" for repo-specific settings.
func (s *Store) List(ctx context.Context, scope, repo string) (map[string]any, error) {
	result := map[string]any{}
	if err := s.loadInto(ctx, result, scope, repo); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) loadInto(ctx context.Context, into map[string]any, scope, repo string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value_json FROM settings WHERE scope = ? AND repo = ?`, scope, repo)
	if err != nil {
		return fmt.Errorf("query settings (%s/%s): %w", scope, repo, err)
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return fmt.Errorf("scan setting: %w", err)
		}
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return fmt.Errorf("decode setting %q: %w", key, err)
		}
		into[key] = value
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate settings: %w", err)
	}
	return nil
}

func (s *Store) ValidateSet(ctx context.Context, scope, repo, key string, value any) error {
	if err := validateScopeAndRepo(scope, repo); err != nil {
		return err
	}
	current, err := s.Get(ctx, repo)
	if err != nil {
		return err
	}
	next := copyMap(current)
	next[key] = value
	return ValidateSettingsWithScope(next, scope)
}

func (s *Store) Set(ctx context.Context, scope, repo, key string, value any) error {
	if err := s.ValidateSet(ctx, scope, repo, key, value); err != nil {
		return err
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal setting value: %w", err)
	}
	now := s.now().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO settings (scope, repo, key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope, repo, key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at
	`, scope, repo, key, string(valueJSON), now, now)
	if err != nil {
		return fmt.Errorf("upsert setting: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, scope, repo, key string) error {
	if err := validateScopeAndRepo(scope, repo); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE scope = ? AND repo = ? AND key = ?`, scope, repo, key)
	if err != nil {
		return fmt.Errorf("delete setting: %w", err)
	}
	return nil
}

func (s *Store) ExportYAML(ctx context.Context, scope, repo string) ([]byte, error) {
	if err := validateScopeAndRepo(scope, repo); err != nil {
		return nil, err
	}
	settingsMap := map[string]any{}
	if err := s.loadInto(ctx, settingsMap, scope, repo); err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(settingsMap)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}
	return out, nil
}

func (s *Store) ImportYAML(ctx context.Context, scope, repo string, content []byte) error {
	if err := validateScopeAndRepo(scope, repo); err != nil {
		return err
	}
	var data map[string]any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}
	current, err := s.Get(ctx, repo)
	if err != nil {
		return err
	}
	next := copyMap(current)
	for key, value := range data {
		next[key] = value
	}
	if err := ValidateSettingsWithScope(next, scope); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := s.now().Format(time.RFC3339)
	for key, value := range data {
		valueJSON, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return fmt.Errorf("marshal setting value: %w", marshalErr)
		}
		_, execErr := tx.ExecContext(ctx, `
			INSERT INTO settings (scope, repo, key, value_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(scope, repo, key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at
		`, scope, repo, key, string(valueJSON), now, now)
		if execErr != nil {
			return fmt.Errorf("upsert setting in import: %w", execErr)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func validateScopeAndRepo(scope, repo string) error {
	switch scope {
	case ScopeGlobal:
		if repo != "" {
			return fmt.Errorf("global scope must not include repo")
		}
	case ScopeRepo:
		if repo == "" {
			return fmt.Errorf("repo scope requires repo identifier")
		}
	default:
		return fmt.Errorf("invalid scope %q", scope)
	}
	return nil
}

func copyMap(src map[string]any) map[string]any {
	dst := map[string]any{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (s *Store) GetAnalyzerConfig(ctx context.Context, repo string) (AnalyzerConfig, error) {
	if err := validateRepoFormat(repo); err != nil {
		return AnalyzerConfig{}, err
	}

	cfg := DefaultAnalyzerConfig()

	globalSettings, err := s.List(ctx, ScopeGlobal, "")
	if err != nil {
		return AnalyzerConfig{}, fmt.Errorf("load global analyzer config: %w", err)
	}
	if raw, ok := globalSettings["analyzer_config"]; ok {
		if globalCfg, err := parseAnalyzerConfig(raw); err == nil {
			cfg = mergeAnalyzerConfig(cfg, globalCfg)
		}
	}

	if repo != "" {
		repoSettings, err := s.List(ctx, ScopeRepo, repo)
		if err != nil {
			return AnalyzerConfig{}, fmt.Errorf("load repo analyzer config: %w", err)
		}
		if raw, ok := repoSettings["analyzer_config"]; ok {
			if repoCfg, err := parseAnalyzerConfig(raw); err == nil {
				cfg = mergeAnalyzerConfig(cfg, repoCfg)
			}
		}
	}

	return cfg, nil
}

func (s *Store) SetAnalyzerConfig(ctx context.Context, repo string, cfg AnalyzerConfig) error {
	if err := validateRepoFormat(repo); err != nil {
		return err
	}

	scope := ScopeGlobal
	if repo != "" {
		scope = ScopeRepo
	}

	return s.Set(ctx, scope, repo, "analyzer_config", cfg)
}

func validateRepoFormat(repo string) error {
	if repo == "" {
		return nil
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid repo format %q: expected owner/repo", repo)
	}
	return nil
}

func parseAnalyzerConfig(raw any) (AnalyzerConfig, error) {
	var cfg AnalyzerConfig

	switch v := raw.(type) {
	case AnalyzerConfig:
		return v, nil
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return cfg, fmt.Errorf("marshal analyzer config: %w", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("unmarshal analyzer config: %w", err)
		}
		return cfg, nil
	default:
		return cfg, fmt.Errorf("unexpected analyzer config type: %T", raw)
	}
}

func mergeAnalyzerConfig(base, override AnalyzerConfig) AnalyzerConfig {
	result := base

	if override.Enabled != base.Enabled {
		result.Enabled = override.Enabled
	}
	if override.Timeout != 0 {
		result.Timeout = override.Timeout
	}
	if len(override.Thresholds) > 0 {
		if result.Thresholds == nil {
			result.Thresholds = make(map[string]float64)
		}
		for k, v := range override.Thresholds {
			result.Thresholds[k] = v
		}
	}

	return result
}

// GetGitHubRuntimeConfig loads the GitHub runtime configuration.
// It merges global and repo-specific settings, with repo settings overriding global.
func (s *Store) GetGitHubRuntimeConfig(ctx context.Context, repo string) (GitHubRuntimeConfig, error) {
	cfg := DefaultGitHubRuntimeConfig()

	globalSettings, err := s.List(ctx, ScopeGlobal, "")
	if err != nil {
		return cfg, fmt.Errorf("load global github_runtime config: %w", err)
	}
	if raw, ok := globalSettings["github_runtime"]; ok {
		if globalCfg, err := parseGitHubRuntimeConfig(raw); err == nil {
			cfg = mergeGitHubRuntimeConfig(cfg, globalCfg)
		}
	}

	if repo != "" {
		repoSettings, err := s.List(ctx, ScopeRepo, repo)
		if err != nil {
			return cfg, fmt.Errorf("load repo github_runtime config: %w", err)
		}
		if raw, ok := repoSettings["github_runtime"]; ok {
			if repoCfg, err := parseGitHubRuntimeConfig(raw); err == nil {
				cfg = mergeGitHubRuntimeConfig(cfg, repoCfg)
			}
		}
	}

	return cfg, nil
}

// SetGitHubRuntimeConfig saves the GitHub runtime configuration.
func (s *Store) SetGitHubRuntimeConfig(ctx context.Context, repo string, cfg GitHubRuntimeConfig) error {
	scope := ScopeGlobal
	if repo != "" {
		scope = ScopeRepo
	}
	return s.Set(ctx, scope, repo, "github_runtime", cfg)
}

// GetGitHubRatePolicy loads the GitHub rate policy configuration.
// It merges global and repo-specific settings, with repo settings overriding global.
func (s *Store) GetGitHubRatePolicy(ctx context.Context, repo string) (GitHubRatePolicy, error) {
	cfg := DefaultGitHubRatePolicy()

	globalSettings, err := s.List(ctx, ScopeGlobal, "")
	if err != nil {
		return cfg, fmt.Errorf("load global github_rate_policy config: %w", err)
	}
	if raw, ok := globalSettings["github_rate_policy"]; ok {
		if globalCfg, err := parseGitHubRatePolicy(raw); err == nil {
			cfg = mergeGitHubRatePolicy(cfg, globalCfg)
		}
	}

	if repo != "" {
		repoSettings, err := s.List(ctx, ScopeRepo, repo)
		if err != nil {
			return cfg, fmt.Errorf("load repo github_rate_policy config: %w", err)
		}
		if raw, ok := repoSettings["github_rate_policy"]; ok {
			if repoCfg, err := parseGitHubRatePolicy(raw); err == nil {
				cfg = mergeGitHubRatePolicy(cfg, repoCfg)
			}
		}
	}

	return cfg, nil
}

// SetGitHubRatePolicy saves the GitHub rate policy configuration.
func (s *Store) SetGitHubRatePolicy(ctx context.Context, repo string, cfg GitHubRatePolicy) error {
	scope := ScopeGlobal
	if repo != "" {
		scope = ScopeRepo
	}
	return s.Set(ctx, scope, repo, "github_rate_policy", cfg)
}

func parseGitHubRuntimeConfig(raw any) (GitHubRuntimeConfig, error) {
	var cfg GitHubRuntimeConfig
	switch v := raw.(type) {
	case GitHubRuntimeConfig:
		return v, nil
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return cfg, fmt.Errorf("marshal github_runtime config: %w", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("unmarshal github_runtime config: %w", err)
		}
		return cfg, nil
	default:
		return cfg, fmt.Errorf("unexpected github_runtime config type: %T", raw)
	}
}

func mergeGitHubRuntimeConfig(base, override GitHubRuntimeConfig) GitHubRuntimeConfig {
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

func parseGitHubRatePolicy(raw any) (GitHubRatePolicy, error) {
	var cfg GitHubRatePolicy
	switch v := raw.(type) {
	case GitHubRatePolicy:
		return v, nil
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return cfg, fmt.Errorf("marshal github_rate_policy config: %w", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("unmarshal github_rate_policy config: %w", err)
		}
		return cfg, nil
	default:
		return cfg, fmt.Errorf("unexpected github_rate_policy config type: %T", raw)
	}
}

func mergeGitHubRatePolicy(base, override GitHubRatePolicy) GitHubRatePolicy {
	result := base
	if override.RateLimit != 0 {
		result.RateLimit = override.RateLimit
	}
	if override.ReserveBuffer != 0 {
		result.ReserveBuffer = override.ReserveBuffer
	}
	if override.ResetBuffer != 0 {
		result.ResetBuffer = override.ResetBuffer
	}
	if override.UnauthenticatedRateLimit != 0 {
		result.UnauthenticatedRateLimit = override.UnauthenticatedRateLimit
	}
	if override.UnauthenticatedReserveBuffer != 0 {
		result.UnauthenticatedReserveBuffer = override.UnauthenticatedReserveBuffer
	}
	return result
}
