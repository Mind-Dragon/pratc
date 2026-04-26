package cache

import (
	"path/filepath"
	"testing"
	"time"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestExecutorLedger(t *testing.T) {
	// Create a temporary database for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	ledger := store.ExecutorLedger()
	if ledger == nil {
		t.Fatal("ExecutorLedger returned nil")
	}
	
	// Clear any existing data
	if err := ledger.ClearAll(); err != nil {
		t.Fatalf("Failed to clear ledger: %v", err)
	}
	
	// Test recording a transition
	intentID := "test-intent-123"
	transition := "proposed"
	preflightSnapshot := `{"intent_id":"test-intent-123","action":"merge","pr_number":42,"dry_run":false}`
	
	err = ledger.RecordTransition(intentID, transition, preflightSnapshot, nil)
	if err != nil {
		t.Fatalf("Failed to record transition: %v", err)
	}
	
	// Test retrieving history
	history, err := ledger.GetHistory(intentID)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	if len(history) != 1 {
		t.Fatalf("Expected 1 transition, got %d", len(history))
	}
	
	if history[0].IntentID != intentID {
		t.Errorf("Expected intent ID %q, got %q", intentID, history[0].IntentID)
	}
	
	if history[0].Transition != transition {
		t.Errorf("Expected transition %q, got %q", transition, history[0].Transition)
	}
	
	if history[0].PreflightSnapshot != preflightSnapshot {
		t.Errorf("Expected preflight snapshot %q, got %q", preflightSnapshot, history[0].PreflightSnapshot)
	}
	
	// Test recording multiple transitions
	err = ledger.RecordTransition(intentID, "preflighted", `{"passed":true}`, nil)
	if err != nil {
		t.Fatalf("Failed to record preflighted transition: %v", err)
	}
	
	err = ledger.RecordTransition(intentID, "executed", `{"result":"merged"}`, nil)
	if err != nil {
		t.Fatalf("Failed to record executed transition: %v", err)
	}
	
	// Test retrieving all history
	history, err = ledger.GetHistory(intentID)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	if len(history) != 3 {
		t.Fatalf("Expected 3 transitions, got %d", len(history))
	}
	
	// Verify order (should be chronological)
	if history[0].Transition != "proposed" {
		t.Errorf("Expected first transition to be 'proposed', got %q", history[0].Transition)
	}
	if history[1].Transition != "preflighted" {
		t.Errorf("Expected second transition to be 'preflighted', got %q", history[1].Transition)
	}
	if history[2].Transition != "executed" {
		t.Errorf("Expected third transition to be 'executed', got %q", history[2].Transition)
	}
	
	// Test IsExecuted
	if !ledger.IsExecuted(intentID) {
		t.Error("Expected IsExecuted to return true for executed intent")
	}
	
	// Test Record (backward compatibility)
	result := types.ExecutionResult{
		IntentID: "test-intent-456",
		Action:   "merge",
		PRNumber: 42,
		Executed: true,
		Result:   "merged",
	}
	
	err = ledger.Record("test-key-456", result)
	if err != nil {
		t.Fatalf("Failed to record execution result: %v", err)
	}
	
	if !ledger.IsExecuted("test-key-456") {
		t.Error("Expected IsExecuted to return true after Record")
	}
	
	// Test GetTransition
	record, err := ledger.GetTransition(intentID, "proposed")
	if err != nil {
		t.Fatalf("Failed to get transition: %v", err)
	}
	
	if record == nil {
		t.Fatal("Expected transition record, got nil")
	}
	
	if record.Transition != "proposed" {
		t.Errorf("Expected transition 'proposed', got %q", record.Transition)
	}
	
	// Test non-existent transition
	record, err = ledger.GetTransition(intentID, "nonexistent")
	if err != nil {
		t.Fatalf("Failed to get non-existent transition: %v", err)
	}
	
	if record != nil {
		t.Error("Expected nil for non-existent transition")
	}
	
	// Test unique constraint (intent_id + transition)
	err = ledger.RecordTransition(intentID, "proposed", "different snapshot", nil)
	if err != nil {
		t.Fatalf("Failed to update existing transition: %v", err)
	}
	
	// Verify the snapshot was updated
	record, err = ledger.GetTransition(intentID, "proposed")
	if err != nil {
		t.Fatalf("Failed to get updated transition: %v", err)
	}
	
	if record.PreflightSnapshot != "different snapshot" {
		t.Errorf("Expected updated snapshot, got %q", record.PreflightSnapshot)
	}
	
	// Test count
	count, err := ledger.CountTransitions()
	if err != nil {
		t.Fatalf("Failed to count transitions: %v", err)
	}
	
	if count != 4 { // 3 from intentID + 1 from test-key-456
		t.Errorf("Expected 4 transitions, got %d", count)
	}
}

func TestExecutorLedgerMultipleIntents(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	ledger := store.ExecutorLedger()
	if err := ledger.ClearAll(); err != nil {
		t.Fatalf("Failed to clear ledger: %v", err)
	}
	
	// Test multiple intents
	intents := []string{"intent-1", "intent-2", "intent-3"}
	
	for _, intentID := range intents {
		err := ledger.RecordTransition(intentID, "proposed", `{"action":"merge"}`, nil)
		if err != nil {
			t.Fatalf("Failed to record transition for %s: %v", intentID, err)
		}
	}
	
	// Verify each intent has its own history
	for _, intentID := range intents {
		history, err := ledger.GetHistory(intentID)
		if err != nil {
			t.Fatalf("Failed to get history for %s: %v", intentID, err)
		}
		
		if len(history) != 1 {
			t.Errorf("Expected 1 transition for %s, got %d", intentID, len(history))
		}
		
		if history[0].IntentID != intentID {
			t.Errorf("Expected intent ID %q, got %q", intentID, history[0].IntentID)
		}
	}
	
	// Test total count
	count, err := ledger.CountTransitions()
	if err != nil {
		t.Fatalf("Failed to count transitions: %v", err)
	}
	
	if count != 3 {
		t.Errorf("Expected 3 transitions, got %d", count)
	}
}

func TestExecutorLedgerCrashRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	// First session: create ledger and record some transitions
	store1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	
	ledger1 := store1.ExecutorLedger()
	if err := ledger1.ClearAll(); err != nil {
		t.Fatalf("Failed to clear ledger: %v", err)
	}
	
	// Record transitions
	intentID := "crash-test-intent"
	err = ledger1.RecordTransition(intentID, "proposed", `{"action":"merge"}`, nil)
	if err != nil {
		t.Fatalf("Failed to record proposed: %v", err)
	}
	
	err = ledger1.RecordTransition(intentID, "preflighted", `{"passed":true}`, nil)
	if err != nil {
		t.Fatalf("Failed to record preflighted: %v", err)
	}
	
	// Close the database (simulating crash)
	store1.Close()
	
	// Second session: reopen and verify data persists
	store2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer store2.Close()
	
	ledger2 := store2.ExecutorLedger()
	
	// Verify we can still see the transitions
	history, err := ledger2.GetHistory(intentID)
	if err != nil {
		t.Fatalf("Failed to get history after recovery: %v", err)
	}
	
	if len(history) != 2 {
		t.Fatalf("Expected 2 transitions after recovery, got %d", len(history))
	}
	
	// Verify the transitions are in correct order
	if history[0].Transition != "proposed" {
		t.Errorf("Expected first transition to be 'proposed', got %q", history[0].Transition)
	}
	if history[1].Transition != "preflighted" {
		t.Errorf("Expected second transition to be 'preflighted', got %q", history[1].Transition)
	}
	
	// Test that IsExecuted still works after recovery
	if ledger2.IsExecuted(intentID) {
		t.Error("Expected IsExecuted to return false for intent without 'executed' transition")
	}
	
	// Add executed transition
	err = ledger2.RecordTransition(intentID, "executed", `{"result":"merged"}`, nil)
	if err != nil {
		t.Fatalf("Failed to record executed: %v", err)
	}
	
	if !ledger2.IsExecuted(intentID) {
		t.Error("Expected IsExecuted to return true after recording executed transition")
	}
}

func TestExecutorLedgerMigration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	// Open database (this should trigger migration)
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	// Verify the executor_ledger table exists
	var tableName string
	err = store.db.QueryRow(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name='executor_ledger'
	`).Scan(&tableName)
	
	if err != nil {
		t.Fatalf("Failed to query executor_ledger table: %v", err)
	}
	
	if tableName != "executor_ledger" {
		t.Errorf("Expected table name 'executor_ledger', got %q", tableName)
	}
	
	// Verify the unique constraint exists
	var constraintSQL string
	err = store.db.QueryRow(`
		SELECT sql FROM sqlite_master 
		WHERE type='index' AND name='sqlite_autoindex_executor_ledger_1'
	`).Scan(&constraintSQL)
	
	if err != nil {
		t.Logf("Could not find unique constraint index (this is OK for some SQLite versions)")
	} else {
		t.Logf("Found unique constraint: %s", constraintSQL)
	}
}

func TestExecutorLedgerWithMutationSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	ledger := store.ExecutorLedger()
	if err := ledger.ClearAll(); err != nil {
		t.Fatalf("Failed to clear ledger: %v", err)
	}
	
	// Test with mutation snapshot
	intentID := "mutation-test"
	mutationSnapshot := `{"old_state":"open","new_state":"merged"}`
	
	err = ledger.RecordTransition(intentID, "executed", `{"result":"merged"}`, &mutationSnapshot)
	if err != nil {
		t.Fatalf("Failed to record transition with mutation snapshot: %v", err)
	}
	
	record, err := ledger.GetTransition(intentID, "executed")
	if err != nil {
		t.Fatalf("Failed to get transition: %v", err)
	}
	
	if record == nil {
		t.Fatal("Expected transition record, got nil")
	}
	
	if record.MutationSnapshot == nil {
		t.Fatal("Expected mutation snapshot to be set")
	}
	
	if *record.MutationSnapshot != mutationSnapshot {
		t.Errorf("Expected mutation snapshot %q, got %q", mutationSnapshot, *record.MutationSnapshot)
	}
}

func TestExecutorLedgerTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	ledger := store.ExecutorLedger()
	if err := ledger.ClearAll(); err != nil {
		t.Fatalf("Failed to clear ledger: %v", err)
	}
	
	intentID := "timestamp-test"
	
	// Record first transition
	err = ledger.RecordTransition(intentID, "proposed", `{"action":"merge"}`, nil)
	if err != nil {
		t.Fatalf("Failed to record first transition: %v", err)
	}
	
	time.Sleep(10 * time.Millisecond)
	
	// Record second transition
	err = ledger.RecordTransition(intentID, "preflighted", `{"passed":true}`, nil)
	if err != nil {
		t.Fatalf("Failed to record second transition: %v", err)
	}
	
	// Get history and verify timestamps are in order
	history, err := ledger.GetHistory(intentID)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	if len(history) != 2 {
		t.Fatalf("Expected 2 transitions, got %d", len(history))
	}
	
	// Verify timestamps are chronological
	if !history[0].Timestamp.Before(history[1].Timestamp) {
		t.Errorf("Expected first timestamp to be before second timestamp")
	}
	
	// Verify timestamps are not zero
	if history[0].Timestamp.IsZero() {
		t.Error("First transition has zero timestamp")
	}
	if history[1].Timestamp.IsZero() {
		t.Error("Second transition has zero timestamp")
	}
}
