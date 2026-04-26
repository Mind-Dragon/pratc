package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// ExecutorLedgerEntry represents a single transition in the executor ledger
type ExecutorLedgerEntry struct {
	ID               int64
	IntentID         string
	Transition       string
	PreflightSnapshot string
	MutationSnapshot  *string
	Timestamp        time.Time
}

// ExecutorLedger is a SQLite-backed append-only ledger for tracking executor transitions
type ExecutorLedger struct {
	db  *sql.DB
	now func() time.Time
}

// NewExecutorLedger creates a new SQLite-backed executor ledger
func NewExecutorLedger(db *sql.DB) *ExecutorLedger {
	return &ExecutorLedger{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// RecordTransition records a transition in the ledger
// The combination of intent_id + transition is unique (enforced by UNIQUE constraint)
func (l *ExecutorLedger) RecordTransition(intentID, transition, preflightSnapshot string, mutationSnapshot *string) error {
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
func (l *ExecutorLedger) GetHistory(intentID string) ([]types.TransitionRecord, error) {
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
		var entry ExecutorLedgerEntry
		var timestampStr string
		
		if err := rows.Scan(
			&entry.ID,
			&entry.IntentID,
			&entry.Transition,
			&entry.PreflightSnapshot,
			&entry.MutationSnapshot,
			&timestampStr,
		); err != nil {
			return nil, fmt.Errorf("scan transition record: %w", err)
		}
		
		timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", timestampStr, err)
		}
		
		records = append(records, types.TransitionRecord{
			IntentID:          entry.IntentID,
			Transition:        entry.Transition,
			PreflightSnapshot: entry.PreflightSnapshot,
			MutationSnapshot:  entry.MutationSnapshot,
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
func (l *ExecutorLedger) IsExecuted(key string) bool {
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
func (l *ExecutorLedger) Record(key string, result types.ExecutionResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal execution result: %w", err)
	}
	
	return l.RecordTransition(key, "executed", string(resultJSON), nil)
}

// GetTransition retrieves a specific transition for an intent
func (l *ExecutorLedger) GetTransition(intentID, transition string) (*types.TransitionRecord, error) {
	var timestampStr string
	var record types.TransitionRecord
	
	err := l.db.QueryRowContext(context.Background(), `
		SELECT intent_id, transition, preflight_snapshot, mutation_snapshot, timestamp
		FROM executor_ledger
		WHERE intent_id = ? AND transition = ?
	`, intentID, transition).Scan(
		&record.IntentID,
		&record.Transition,
		&record.PreflightSnapshot,
		&record.MutationSnapshot,
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
	
	return &record, nil
}

// CountTransitions returns the total number of transitions in the ledger
func (l *ExecutorLedger) CountTransitions() (int64, error) {
	var count int64
	err := l.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM executor_ledger`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count transitions: %w", err)
	}
	return count, nil
}

// ClearAll clears all entries from the ledger (for testing purposes)
func (l *ExecutorLedger) ClearAll() error {
	_, err := l.db.ExecContext(context.Background(), `DELETE FROM executor_ledger`)
	if err != nil {
		return fmt.Errorf("clear ledger: %w", err)
	}
	return nil
}
