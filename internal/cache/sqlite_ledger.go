package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	_ "modernc.org/sqlite"
)

// SQLiteLedger is a SQLite-backed append-only ledger for tracking executor transitions
// This implements the Ledger interface and opens its own database connection
type SQLiteLedger struct {
	db  *sql.DB
	now func() time.Time
}

// NewSQLiteLedger creates a new SQLite-backed executor ledger
// It opens its own database connection to the specified path
func NewSQLiteLedger(dbPath string) (*SQLiteLedger, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// Apply common pragmas for SQLite
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	// Apply SQLite pragmas
	pragmas := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA busy_timeout=5000;`,
		`PRAGMA foreign_keys=ON;`,
	}
	for _, stmt := range pragmas {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", stmt, err)
		}
	}

	// Ensure the executor_ledger table exists
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS executor_ledger (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			intent_id TEXT NOT NULL,
			transition TEXT NOT NULL,
			preflight_snapshot TEXT NOT NULL,
			mutation_snapshot TEXT,
			timestamp TEXT NOT NULL,
			UNIQUE(intent_id, transition)
		)
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create executor_ledger table: %w", err)
	}

	// Create index on intent_id for efficient queries
	_, err = db.ExecContext(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_executor_ledger_intent_id ON executor_ledger(intent_id)
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create intent_id index: %w", err)
	}

	// Create index on timestamp for efficient history queries
	_, err = db.ExecContext(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_executor_ledger_timestamp ON executor_ledger(timestamp DESC)
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create timestamp index: %w", err)
	}

	return &SQLiteLedger{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// Close closes the database connection
func (l *SQLiteLedger) Close() error {
	return l.db.Close()
}

// RecordTransition records a transition in the ledger
// The combination of intent_id + transition is unique (enforced by UNIQUE constraint)
// This is an append-only operation - existing transitions are updated, not duplicated
func (l *SQLiteLedger) RecordTransition(intentID, transition, preflightSnapshot string, mutationSnapshot *string) error {
	timestamp := l.now().Format(time.RFC3339Nano)

	_, err := l.db.ExecContext(context.Background(), `
		INSERT INTO executor_ledger (intent_id, transition, preflight_snapshot, mutation_snapshot, timestamp)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(intent_id, transition) DO UPDATE SET
			preflight_snapshot = excluded.preflight_snapshot,
			mutation_snapshot = excluded.mutation_snapshot,
			timestamp = excluded.timestamp
	`, intentID, transition, preflightSnapshot, mutationSnapshot, timestamp)

	if err != nil {
		return fmt.Errorf("record transition %q for intent %q: %w", transition, intentID, err)
	}

	return nil
}

// GetHistory retrieves all transitions for a given intent ID
func (l *SQLiteLedger) GetHistory(intentID string) ([]types.TransitionRecord, error) {
	rows, err := l.db.QueryContext(context.Background(), `
		SELECT id, intent_id, transition, preflight_snapshot, mutation_snapshot, timestamp
		FROM executor_ledger
		WHERE intent_id = ?
		ORDER BY timestamp ASC
	`, intentID)

	if err != nil {
		return nil, fmt.Errorf("query history for intent %q: %w", intentID, err)
	}
	defer rows.Close()

	var records []types.TransitionRecord
	for rows.Next() {
		var id int64
		var intentIDStr, transition, preflightSnapshot string
		var mutationSnapshot sql.NullString
		var timestampStr string

		if err := rows.Scan(
			&id,
			&intentIDStr,
			&transition,
			&preflightSnapshot,
			&mutationSnapshot,
			&timestampStr,
		); err != nil {
			return nil, fmt.Errorf("scan transition record: %w", err)
		}

		timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", timestampStr, err)
		}

		var mutationSnapshotPtr *string
		if mutationSnapshot.Valid {
			mutationSnapshotPtr = &mutationSnapshot.String
		}

		records = append(records, types.TransitionRecord{
			IntentID:          intentIDStr,
			Transition:        transition,
			PreflightSnapshot: preflightSnapshot,
			MutationSnapshot:  mutationSnapshotPtr,
			Timestamp:         timestamp,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transition records: %w", err)
	}

	return records, nil
}

// IsExecuted checks if an intent has been executed (has "executed" transition)
// This implements the Ledger interface for backward compatibility with MemoryLedger
func (l *SQLiteLedger) IsExecuted(key string) bool {
	var exists bool
	err := l.db.QueryRowContext(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM executor_ledger WHERE intent_id = ? AND transition = 'executed')
	`, key).Scan(&exists)

	if err != nil {
		return false
	}
	return exists
}

// Record records an execution result for backward compatibility with MemoryLedger
// This creates an "executed" transition with the result as preflight snapshot
func (l *SQLiteLedger) Record(key string, result types.ExecutionResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal execution result: %w", err)
	}

	return l.RecordTransition(key, "executed", string(resultJSON), nil)
}

// GetTransition retrieves a specific transition for an intent
func (l *SQLiteLedger) GetTransition(intentID, transition string) (*types.TransitionRecord, error) {
	var timestampStr string
	var record types.TransitionRecord
	var mutationSnapshot sql.NullString

	err := l.db.QueryRowContext(context.Background(), `
		SELECT intent_id, transition, preflight_snapshot, mutation_snapshot, timestamp
		FROM executor_ledger
		WHERE intent_id = ? AND transition = ?
	`, intentID, transition).Scan(
		&record.IntentID,
		&record.Transition,
		&record.PreflightSnapshot,
		&mutationSnapshot,
		&timestampStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query transition %q for intent %q: %w", transition, intentID, err)
	}

	timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp %q: %w", timestampStr, err)
	}
	record.Timestamp = timestamp

	if mutationSnapshot.Valid {
		record.MutationSnapshot = &mutationSnapshot.String
	}

	return &record, nil
}

// CountTransitions returns the total number of transitions in the ledger
func (l *SQLiteLedger) CountTransitions() (int64, error) {
	var count int64
	err := l.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM executor_ledger`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count transitions: %w", err)
	}
	return count, nil
}

// ClearAll clears all entries from the ledger (for testing purposes)
func (l *SQLiteLedger) ClearAll() error {
	_, err := l.db.ExecContext(context.Background(), `DELETE FROM executor_ledger`)
	if err != nil {
		return fmt.Errorf("clear ledger: %w", err)
	}
	return nil
}
