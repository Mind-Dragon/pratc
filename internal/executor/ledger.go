package executor

import (
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// Ledger interface for tracking executor transitions
// This replaces the old MemoryLedger interface with a more detailed transition-based interface
type Ledger interface {
	// RecordTransition records a transition in the ledger
	// The combination of intent_id + transition is unique (enforced by UNIQUE constraint)
	RecordTransition(intentID, transition, preflightSnapshot string, mutationSnapshot *string) error

	// GetHistory retrieves all transitions for a given intent ID
	GetHistory(intentID string) ([]types.TransitionRecord, error)

	// IsExecuted checks if an intent has been executed (has "executed" transition)
	// This is for backward compatibility with MemoryLedger
	IsExecuted(key string) bool

	// Record records an execution result for backward compatibility with MemoryLedger
	Record(key string, result types.ExecutionResult) error
}

// MemoryLedger is an in-memory implementation of the Ledger interface
// This is the original implementation that will be replaced by SQLiteLedger
type MemoryLedger struct {
	transitions map[string][]types.TransitionRecord
}

// NewMemoryLedger creates a new in-memory ledger
func NewMemoryLedger() *MemoryLedger {
	return &MemoryLedger{
		transitions: make(map[string][]types.TransitionRecord),
	}
}

// RecordTransition records a transition in the in-memory ledger
func (l *MemoryLedger) RecordTransition(intentID, transition, preflightSnapshot string, mutationSnapshot *string) error {
	record := types.TransitionRecord{
		IntentID:          intentID,
		Transition:        transition,
		PreflightSnapshot: preflightSnapshot,
		MutationSnapshot:  mutationSnapshot,
		Timestamp:         time.Now().UTC(),
	}

	history := l.transitions[intentID]
	for i := range history {
		if history[i].Transition == transition {
			history[i] = record
			l.transitions[intentID] = history
			return nil
		}
	}
	l.transitions[intentID] = append(history, record)
	return nil
}

// GetHistory retrieves all transitions for a given intent ID
func (l *MemoryLedger) GetHistory(intentID string) ([]types.TransitionRecord, error) {
	return l.transitions[intentID], nil
}

// IsExecuted checks if an intent has been executed (has "executed" transition)
func (l *MemoryLedger) IsExecuted(key string) bool {
	history, ok := l.transitions[key]
	if !ok {
		return false
	}
	for _, record := range history {
		if record.Transition == "executed" {
			return true
		}
	}
	return false
}

// Record records an execution result for backward compatibility
func (l *MemoryLedger) Record(key string, result types.ExecutionResult) error {
	resultJSON := `{"result":"` + result.Result + `"}`
	return l.RecordTransition(key, "executed", resultJSON, nil)
}
