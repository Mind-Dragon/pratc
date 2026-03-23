package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	return ValidateSettings(next)
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
	if err := ValidateSettings(next); err != nil {
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
