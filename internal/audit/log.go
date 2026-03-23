package audit

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrInvalidLimit = errors.New("limit must be >= 0")

type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Repo      string    `json:"repo"`
	Details   string    `json:"details,omitempty"`
}

type Store interface {
	Append(AuditEntry) error
	List(limit, offset int) ([]AuditEntry, error)
}

func ValidateLimit(limit int) error {
	if limit < 0 {
		return ErrInvalidLimit
	}
	return nil
}

type MemoryStore struct {
	mu      sync.Mutex
	entries []AuditEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make([]AuditEntry, 0),
	}
}

func (s *MemoryStore) Append(entry AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return nil
}

func (s *MemoryStore) List(limit, offset int) ([]AuditEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit < 0 || offset < 0 {
		return nil, ErrInvalidLimit
	}

	sorted := make([]AuditEntry, len(s.entries))
	copy(sorted, s.entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	if offset >= len(sorted) {
		return []AuditEntry{}, nil
	}

	end := offset + limit
	if limit == 0 {
		end = len(sorted)
	}
	if end > len(sorted) {
		end = len(sorted)
	}

	return sorted[offset:end], nil
}
