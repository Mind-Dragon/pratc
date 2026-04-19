package sync

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

func TestRateLimitGuardCheckBudget(t *testing.T) {
	t.Parallel()

	tempDir, err := os.MkdirTemp("", "pratc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := tempDir + "/test.db"
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open cache store: %v", err)
	}
	defer store.Close()

	// Create a sync job with realistic jobID != repo
	now := time.Now().UTC()
	jobID := "test/repo-" + strconv.FormatInt(now.UnixNano(), 10)
	_, err = store.DB().Exec(`
		INSERT INTO sync_jobs (id, repo, status, error_message, last_sync_at, created_at, updated_at)
		VALUES (?, ?, ?, '', '', ?, ?)
	`, jobID, "test/repo", "in_progress", now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	_, err = store.DB().Exec(`
		INSERT INTO sync_progress (repo, job_id, cursor, processed_prs, total_prs, next_scheduled_at, estimated_requests, last_sync_at, updated_at)
		VALUES (?, ?, '', 0, 0, '', 0, '', ?)
		ON CONFLICT(repo) DO UPDATE SET
			job_id = excluded.job_id,
			cursor = excluded.cursor,
			processed_prs = excluded.processed_prs,
			total_prs = excluded.total_prs,
			next_scheduled_at = excluded.next_scheduled_at,
			estimated_requests = excluded.estimated_requests,
			updated_at = excluded.updated_at
	`, "test/repo", jobID, now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to create sync progress: %v", err)
	}

	tests := []struct {
		name              string
		budgetRemaining   int
		estimatedRequests int
		expectChunkSize   int
		expectError       bool
		expectJobPaused   bool
	}{
		{
			name:              "sufficient budget returns chunk",
			budgetRemaining:   500,
			estimatedRequests: 100,
			expectChunkSize:   300,
			expectError:       false,
			expectJobPaused:   false,
		},
		{
			name:              "insufficient budget pauses job",
			budgetRemaining:   250,
			estimatedRequests: 100,
			expectChunkSize:   0,
			expectError:       true,
			expectJobPaused:   true,
		},
		{
			name:              "zero budget pauses job",
			budgetRemaining:   0,
			estimatedRequests: 100,
			expectChunkSize:   0,
			expectError:       true,
			expectJobPaused:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := ratelimit.NewMetrics()

			budget := ratelimit.NewBudgetManager()
			budget.RecordResponse(tt.budgetRemaining, time.Now().Add(1*time.Hour).Unix())

			guard := NewRateLimitGuard(budget, metrics, store, jobID)

			chunkSize, err := guard.CheckBudget(jobID, tt.estimatedRequests)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if chunkSize != tt.expectChunkSize {
					t.Errorf("expected chunk size %d, got %d", tt.expectChunkSize, chunkSize)
				}
				if err != nil {
					t.Logf("CheckBudget error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if chunkSize != tt.expectChunkSize {
					t.Errorf("expected chunk size %d, got %d", tt.expectChunkSize, chunkSize)
				}
			}

			if tt.expectJobPaused {
				pausedJobs, err := store.ListPausedSyncJobs()
				if err != nil {
					t.Fatalf("failed to list paused jobs: %v", err)
				}

				found := false
				for _, pausedJob := range pausedJobs {
					if pausedJob.Repo == "test/repo" {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("expected job to be paused but it was not")
				}

				snapshot := metrics.Snapshot()
				if snapshot.BudgetPauses != 1 {
					t.Errorf("expected 1 budget pause, got %d", snapshot.BudgetPauses)
				}
			}
		})
	}
}

func TestRateLimitGuardMetrics(t *testing.T) {
	t.Parallel()

	tempDir, err := os.MkdirTemp("", "pratc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := tempDir + "/test.db"
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open cache store: %v", err)
	}
	defer store.Close()

	// Create a sync job with realistic jobID != repo
	now := time.Now().UTC()
	jobID := "test/repo-" + strconv.FormatInt(now.UnixNano(), 10)
	_, err = store.DB().Exec(`
		INSERT INTO sync_jobs (id, repo, status, error_message, last_sync_at, created_at, updated_at)
		VALUES (?, ?, ?, '', '', ?, ?)
	`, jobID, "test/repo", "in_progress", now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	_, err = store.DB().Exec(`
		INSERT INTO sync_progress (repo, job_id, cursor, processed_prs, total_prs, next_scheduled_at, estimated_requests, last_sync_at, updated_at)
		VALUES (?, ?, '', 0, 0, '', 0, '', ?)
		ON CONFLICT(repo) DO UPDATE SET
			job_id = excluded.job_id,
			cursor = excluded.cursor,
			processed_prs = excluded.processed_prs,
			total_prs = excluded.total_prs,
			next_scheduled_at = excluded.next_scheduled_at,
			estimated_requests = excluded.estimated_requests,
			updated_at = excluded.updated_at
	`, "test/repo", jobID, now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to create sync progress: %v", err)
	}

	metrics := ratelimit.NewMetrics()
	budget := ratelimit.NewBudgetManager()
	budget.RecordResponse(500, time.Now().Add(1*time.Hour).Unix())

	guard := NewRateLimitGuard(budget, metrics, store, jobID)
	_, _ = guard.CheckBudget(jobID, 100)

	snapshot := metrics.Snapshot()
	if snapshot.RequestsTotal != 1 {
		t.Errorf("expected 1 request total, got %d", snapshot.RequestsTotal)
	}
}
