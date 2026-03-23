package cache

import (
	"context"
	"database/sql"
	"time"

	"github.com/jeffersonnunn/pratc/internal/audit"
)

func (s *Store) AppendAuditEntry(entry audit.AuditEntry) error {
	_, err := s.db.Exec(`
		INSERT INTO audit_log (timestamp, action, repo, details)
		VALUES (?, ?, ?, ?)
	`, entry.Timestamp.UTC().Format(time.RFC3339), entry.Action, entry.Repo, entry.Details)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) ListAuditEntries(ctx context.Context, limit, offset int) ([]audit.AuditEntry, error) {
	var rows *sql.Rows
	var err error

	if limit <= 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT timestamp, action, repo, details
			FROM audit_log
			ORDER BY timestamp DESC
			LIMIT -1 OFFSET ?
		`, offset)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT timestamp, action, repo, details
			FROM audit_log
			ORDER BY timestamp DESC
			LIMIT ? OFFSET ?
		`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []audit.AuditEntry
	for rows.Next() {
		var e audit.AuditEntry
		var ts string
		if err := rows.Scan(&ts, &e.Action, &e.Repo, &e.Details); err != nil {
			return nil, err
		}
		e.Timestamp, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

type AuditStore struct {
	store *Store
}

func NewAuditStore(store *Store) *AuditStore {
	return &AuditStore{store: store}
}

func (a *AuditStore) Append(entry audit.AuditEntry) error {
	return a.store.AppendAuditEntry(entry)
}

func (a *AuditStore) List(limit, offset int) ([]audit.AuditEntry, error) {
	return a.store.ListAuditEntries(context.Background(), limit, offset)
}

func (a *AuditStore) Close() error {
	return a.store.Close()
}
