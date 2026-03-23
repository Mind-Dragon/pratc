package audit

import (
	"testing"
	"time"
)

func TestAuditEntry_HasRequiredFields(t *testing.T) {
	entry := AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    "plan",
		Repo:      "owner/repo",
		Details:   "test details",
	}

	if entry.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if entry.Action == "" {
		t.Error("Action should not be empty")
	}
	if entry.Repo == "" {
		t.Error("Repo should not be empty")
	}
}

func TestMemoryStore_AppendAndList(t *testing.T) {
	store := NewMemoryStore()

	entries := []AuditEntry{
		{Timestamp: time.Now().UTC(), Action: "analyze", Repo: "owner/repo", Details: "analyze details"},
		{Timestamp: time.Now().UTC(), Action: "plan", Repo: "owner/repo2", Details: "plan details"},
		{Timestamp: time.Now().UTC(), Action: "cluster", Repo: "owner/repo", Details: "cluster details"},
	}

	for _, entry := range entries {
		if err := store.Append(entry); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	listed, err := store.List(10, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("expected 3 entries, got %d", len(listed))
	}
}

func TestMemoryStore_List_OrderedByTimestamp(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now().UTC()
	store.Append(AuditEntry{Timestamp: now.Add(-2 * time.Hour), Action: "first", Repo: "r/r", Details: ""})
	store.Append(AuditEntry{Timestamp: now.Add(-1 * time.Hour), Action: "second", Repo: "r/r", Details: ""})
	store.Append(AuditEntry{Timestamp: now, Action: "third", Repo: "r/r", Details: ""})

	listed, err := store.List(10, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(listed))
	}

	if listed[0].Action != "third" {
		t.Errorf("expected first entry to be 'third', got %q", listed[0].Action)
	}
	if listed[2].Action != "first" {
		t.Errorf("expected last entry to be 'first', got %q", listed[2].Action)
	}
}

func TestMemoryStore_List_Pagination(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		store.Append(AuditEntry{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Action:    "action",
			Repo:      "owner/repo",
			Details:   "",
		})
	}

	listed, err := store.List(3, 0)
	if err != nil {
		t.Fatalf("List with limit failed: %v", err)
	}
	if len(listed) != 3 {
		t.Errorf("expected 3 entries with limit, got %d", len(listed))
	}

	listed, err = store.List(10, 5)
	if err != nil {
		t.Fatalf("List with offset failed: %v", err)
	}
	if len(listed) != 5 {
		t.Errorf("expected 5 entries with offset 5, got %d", len(listed))
	}
}

func TestMemoryStore_List_EmptyStore(t *testing.T) {
	store := NewMemoryStore()

	listed, err := store.List(10, 0)
	if err != nil {
		t.Fatalf("List on empty store failed: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("expected 0 entries from empty store, got %d", len(listed))
	}
}

func TestValidateLimit(t *testing.T) {
	tests := []struct {
		name    string
		limit   int
		wantErr bool
	}{
		{"valid 10", 10, false},
		{"valid 1", 1, false},
		{"valid 0 returns all", 0, false},
		{"invalid negative", -1, true},
		{"invalid -100", -100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLimit(tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLimit(%d) error = %v, wantErr %v", tt.limit, err, tt.wantErr)
			}
		})
	}
}
