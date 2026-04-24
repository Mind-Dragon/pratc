// Package data provides data models and storage abstractions for the monitor package.
package data

import (
	"database/sql"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

// Store provides read-only access to sync job data from the SQLite cache.
// It wraps the cache.Store and exposes simplified views for the monitor.
type Store struct {
	cache *cache.Store
	wq    *workqueue.Queue
}

// NewStore creates a new data store backed by the provided cache store.
func NewStore(cacheStore *cache.Store) *Store {
	return &Store{cache: cacheStore, wq: nil}
}

// NewStoreWithWorkqueue creates a new data store backed by the provided cache store and workqueue.
func NewStoreWithWorkqueue(cacheStore *cache.Store, wq *workqueue.Queue) *Store {
	return &Store{cache: cacheStore, wq: wq}
}

// GetActiveJobs returns sync jobs that are currently active, paused, or queued.
// These are jobs that are currently running or waiting to run.
func (s *Store) GetActiveJobs() []SyncJobView {
	jobs, err := s.cache.ListSyncJobs()
	if err != nil {
		return nil
	}

	var views []SyncJobView
	for _, job := range jobs {
		switch job.Status {
		case cache.SyncJobStatusQueued, cache.SyncJobStatusRunning, cache.SyncJobStatusResuming, cache.SyncJobStatusPausedRateLimit:
			views = append(views, jobToView(job))
		}
	}
	return views
}

// GetJobHistory returns the most recently completed or failed sync jobs,
// ordered by updated_at descending, limited to the specified count.
func (s *Store) GetJobHistory(limit int) []SyncJobView {
	jobs, err := s.cache.ListSyncJobs()
	if err != nil {
		return nil
	}

	var views []SyncJobView
	for _, job := range jobs {
		if job.Status == cache.SyncJobStatusCompleted || job.Status == cache.SyncJobStatusFailed {
			views = append(views, jobToView(job))
			if len(views) >= limit {
				break
			}
		}
	}
	return views
}

// GetAllJobs returns all sync jobs that have a non-empty repo field,
// ordered by updated_at descending.
func (s *Store) GetAllJobs() []SyncJobView {
	jobs, err := s.cache.ListSyncJobs()
	if err != nil {
		return nil
	}

	var views []SyncJobView
	for _, job := range jobs {
		if job.Repo != "" {
			views = append(views, jobToView(job))
		}
	}
	return views
}

// jobToView converts a cache.SyncJob to a SyncJobView for monitor display.
func jobToView(job cache.SyncJob) SyncJobView {
	progress := 0
	if job.Progress.TotalPRs > 0 {
		progress = (job.Progress.ProcessedPRs * 100) / job.Progress.TotalPRs
	}

	status := mapCacheStatus(job.Status)
	detail := job.Error
	if detail == "" && job.Status == cache.SyncJobStatusCompleted {
		detail = "Completed successfully"
	}

	return SyncJobView{
		ID:       job.ID,
		Repo:     job.Repo,
		Progress: progress,
		Status:   status,
		Detail:   detail,
		ETA:      0,
		Batch:    0,
	}
}

// mapCacheStatus maps cache job statuses to monitor view statuses.
func mapCacheStatus(status cache.SyncJobStatus) string {
	switch status {
	case cache.SyncJobStatusQueued:
		return StatusQueued
	case cache.SyncJobStatusRunning:
		return StatusActive
	case cache.SyncJobStatusResuming:
		return StatusActive
	case cache.SyncJobStatusPausedRateLimit:
		return StatusPaused
	case cache.SyncJobStatusCompleted:
		return StatusCompleted
	case cache.SyncJobStatusFailed:
		return StatusFailed
	case cache.SyncJobStatusCanceled:
		return StatusFailed
	// Legacy states (for backward compatibility during transition)
	case cache.SyncJobStatusInProgress:
		return StatusActive
	case cache.SyncJobStatusPaused:
		return StatusPaused
	default:
		return StatusQueued
	}
}

// DB returns the underlying sql.DB for advanced operations.
// This is intended for cases where the Store's simple query methods are insufficient.
func (s *Store) DB() *sql.DB {
	return s.cache.DB()
}

// GetWorkqueue returns the workqueue associated with this store.
func (s *Store) GetWorkqueue() *workqueue.Queue {
	return s.wq
}

// GetCorpusStats returns aggregate statistics about the PR corpus.
// Delegates to cache.Store.GetCorpusStats.
func (s *Store) GetCorpusStats() (totalPRs int, lastSync time.Time, syncJobs int, auditEntries int, err error) {
	return s.cache.GetCorpusStats()
}

// GetAuditLedger returns recent audit log entries.
// Delegates to cache.Store.GetAuditLedger and converts types.
func (s *Store) GetAuditLedger(limit int) ([]AuditLedgerEntry, error) {
	cacheEntries, err := s.cache.GetAuditLedger(limit)
	if err != nil {
		return nil, err
	}
	entries := make([]AuditLedgerEntry, len(cacheEntries))
	for i, e := range cacheEntries {
		entries[i] = AuditLedgerEntry{
			Timestamp:  e.Timestamp,
			Action:     e.Action,
			WorkItemID: "",
			PRNumber:   0,
			Reason:     e.Details,
			Actor:      e.Repo,
		}
	}
	return entries, nil
}

// GetRecentProofBundles returns recent proof bundles from workqueue, converted to view refs.
func (s *Store) GetRecentProofBundles(limit int) ([]ProofBundleRef, error) {
	if s.wq == nil {
		return nil, nil
	}
	ledger, err := s.wq.GetExecutorLedger(limit)
	if err != nil {
		return nil, err
	}
	refs := make([]ProofBundleRef, len(ledger.ProofBundles))
	for i, pb := range ledger.ProofBundles {
		refs[i] = ProofBundleRef{
			ID:         pb.ID,
			WorkItemID: pb.WorkItemID,
			PRNumber:   pb.PRNumber,
			Summary:    pb.Summary,
			CreatedAt:  pb.CreatedAt,
		}
	}
	return refs, nil
}
