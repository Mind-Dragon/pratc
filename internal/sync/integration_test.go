package sync

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

type syncJobSnapshot struct {
	Status            string
	JobID             string
	ErrorMessage      string
	NextScheduledAt   string
	ScheduledResumeAt string
	PauseReason       string
	LastBudgetCheck   string
}

func openIntegrationStore(t *testing.T) (*cache.Store, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "sync.db")
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open integration store: %v", err)
	}

	return store, dbPath
}

func loadSyncJobSnapshot(t *testing.T, store *cache.Store, jobID string) syncJobSnapshot {
	t.Helper()

	var snapshot syncJobSnapshot
	err := store.DB().QueryRow(`
		SELECT
			j.status,
			COALESCE(p.job_id, ''),
			COALESCE(j.error_message, ''),
			COALESCE(p.next_scheduled_at, ''),
			COALESCE(p.scheduled_resume_at, ''),
			COALESCE(p.pause_reason, ''),
			COALESCE(p.last_budget_check, '')
		FROM sync_jobs j
		LEFT JOIN sync_progress p ON p.repo = j.repo
		WHERE j.id = ?
	`, jobID).Scan(
		&snapshot.Status,
		&snapshot.JobID,
		&snapshot.ErrorMessage,
		&snapshot.NextScheduledAt,
		&snapshot.ScheduledResumeAt,
		&snapshot.PauseReason,
		&snapshot.LastBudgetCheck,
	)
	if err != nil {
		t.Fatalf("failed to load sync job snapshot: %v", err)
	}

	return snapshot
}

func assertSnapshot(t *testing.T, got syncJobSnapshot, wantStatus, wantJobID string) {
	t.Helper()

	if got.Status != wantStatus {
		t.Fatalf("expected status %s, got %s", wantStatus, got.Status)
	}
	if got.JobID != wantJobID {
		t.Fatalf("expected job linkage %s, got %s", wantJobID, got.JobID)
	}
}

func TestIntegration_PauseResumeRestartLifecycle(t *testing.T) {
	t.Parallel()

	store, dbPath := openIntegrationStore(t)
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	resetAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)
	budget := ratelimit.NewBudgetManager()
	budget.RecordResponse(0, resetAt.Unix())
	guard := NewRateLimitGuard(budget, ratelimit.NewMetrics(), store, job.ID)

	chunkSize, err := guard.CheckBudget(repo, 100)
	if err == nil {
		t.Fatal("expected budget exhaustion to pause sync job, got nil")
	}
	if chunkSize != 0 {
		t.Fatalf("expected chunk size 0 after pause, got %d", chunkSize)
	}

	paused := loadSyncJobSnapshot(t, store, job.ID)
	assertSnapshot(t, paused, string(cache.SyncJobStatusPausedRateLimit), job.ID)
	wantResumeAt := resetAt.Add(15 * time.Second).Format(time.RFC3339)
	if paused.NextScheduledAt != wantResumeAt {
		t.Fatalf("expected next scheduled time %s, got %s", wantResumeAt, paused.NextScheduledAt)
	}
	if paused.ScheduledResumeAt != wantResumeAt {
		t.Fatalf("expected scheduled resume time %s, got %s", wantResumeAt, paused.ScheduledResumeAt)
	}
	if paused.PauseReason != "rate limit budget exhausted" {
		t.Fatalf("expected pause reason to persist, got %q", paused.PauseReason)
	}
	if paused.LastBudgetCheck == "" {
		t.Fatal("expected last budget check to persist")
	}

	if err := store.Close(); err != nil {
		t.Fatalf("failed to close store before restart: %v", err)
	}
	store, err = cache.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen store after restart: %v", err)
	}
	defer store.Close()

	restarted := loadSyncJobSnapshot(t, store, job.ID)
	assertSnapshot(t, restarted, string(cache.SyncJobStatusPausedRateLimit), job.ID)
	if restarted.NextScheduledAt != wantResumeAt || restarted.ScheduledResumeAt != wantResumeAt {
		t.Fatalf("expected restart to preserve pause schedule, got %+v", restarted)
	}

	resumed, err := ResumeJob(store, repo)
	if err != nil {
		t.Fatalf("expected resume after restart to succeed, got %v", err)
	}
	if resumed.Status != cache.SyncJobStatusResuming {
		t.Fatalf("expected resumed job to be resuming, got %s", resumed.Status)
	}

	after := loadSyncJobSnapshot(t, store, job.ID)
	assertSnapshot(t, after, string(cache.SyncJobStatusResuming), job.ID)
	if after.NextScheduledAt != "" || after.ScheduledResumeAt != "" || after.PauseReason != "" || after.LastBudgetCheck != "" {
		t.Fatalf("expected pause fields to clear after resume, got %+v", after)
	}
	if after.ErrorMessage != "" {
		t.Fatalf("expected error message to clear after resume, got %q", after.ErrorMessage)
	}
}

func TestIntegration_ResumeFailsWithMissingJobLinkage(t *testing.T) {
	t.Parallel()

	store, _ := openIntegrationStore(t)
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	pauseAt := time.Now().UTC().Add(-time.Hour)
	if err := store.PauseSyncJob(job.ID, pauseAt, "rate limit budget exhausted"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	if _, err := store.DB().Exec(`UPDATE sync_progress SET job_id = ? WHERE repo = ?`, "stale-job-linkage", repo); err != nil {
		t.Fatalf("failed to corrupt sync progress linkage: %v", err)
	}

	missing := loadSyncJobSnapshot(t, store, job.ID)
	if missing.Status != string(cache.SyncJobStatusPausedRateLimit) {
		t.Fatalf("expected job to remain paused_rate_limit, got %s", missing.Status)
	}
	if missing.JobID != "stale-job-linkage" {
		t.Fatalf("expected corrupted linkage to persist, got %s", missing.JobID)
	}
	if missing.NextScheduledAt == "" || missing.ScheduledResumeAt == "" || missing.PauseReason != "rate limit budget exhausted" || missing.LastBudgetCheck == "" {
		t.Fatalf("expected paused progress to remain persisted, got %+v", missing)
	}

	_, err = ResumeJob(store, repo)
	if err == nil {
		t.Fatal("expected resume to fail when sync_progress linkage is missing, got nil")
	}
	if !strings.Contains(err.Error(), "linkage") {
		t.Fatalf("expected explicit linkage error, got: %v", err)
	}

	after := loadSyncJobSnapshot(t, store, job.ID)
	assertSnapshot(t, after, string(cache.SyncJobStatusPausedRateLimit), "stale-job-linkage")
	if after.PauseReason != "rate limit budget exhausted" {
		t.Fatalf("expected paused state to remain unchanged, got %+v", after)
	}
}

// TestIntegration_FullPauseResumeCycle tests the full flow: create job → pause → scheduler resumes → job is in_progress.
func TestIntegration_FullPauseResumeCycle(t *testing.T) {
	t.Parallel()

	t.Run("scheduler resumes overdue paused job", func(t *testing.T) {
		t.Parallel()

		// Use shared in-memory DB for goroutine-safe access
		store, err := cache.Open("file::rl_integration_pause_resume?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
		}
		defer store.Close()

		repo := "test/repo"
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with a past time so it's overdue for resume
		pastTime := time.Now().Add(-24 * time.Hour)
		if err := store.PauseSyncJob(job.ID, pastTime, "rate limit exhausted"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Verify job is paused
		pausedJobs, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		if len(pausedJobs) != 1 {
			t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
		}

		// Create scheduler and run CheckAndResume
		scheduler := NewScheduler(store)
		if err := scheduler.CheckAndResume(context.Background()); err != nil {
			t.Fatalf("CheckAndResume failed: %v", err)
		}

		// Verify job is no longer paused
		pausedJobs, err = store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs after resume: %v", err)
		}
		if len(pausedJobs) != 0 {
			t.Errorf("expected 0 paused jobs after resume, got %d", len(pausedJobs))
		}

		// Verify the job status is now resuming
		resumedJob, err := store.GetSyncJob(job.ID)
		if err != nil {
			t.Fatalf("failed to get resumed job: %v", err)
		}
		if resumedJob.Status != cache.SyncJobStatusResuming {
			t.Errorf("expected status %s, got %s", cache.SyncJobStatusResuming, resumedJob.Status)
		}
	})

	t.Run("scheduler skips non-overdue paused job", func(t *testing.T) {
		t.Parallel()

		store, err := cache.Open("file::rl_integration_skip_future?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
		}
		defer store.Close()

		repo := "test/repo"
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with a future time so it's NOT overdue
		futureTime := time.Now().Add(10 * time.Minute)
		if err := store.PauseSyncJob(job.ID, futureTime, "rate limit exhausted"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Create scheduler and run CheckAndResume
		scheduler := NewScheduler(store)
		if err := scheduler.CheckAndResume(context.Background()); err != nil {
			t.Fatalf("CheckAndResume failed: %v", err)
		}

		// Verify job is still paused (not overdue)
		pausedJobs, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		if len(pausedJobs) != 1 {
			t.Errorf("expected 1 paused job (not overdue), got %d", len(pausedJobs))
		}
	})
}

// TestIntegration_GuardBlocksOnExhaustedBudget tests that the guard blocks when budget is exhausted.
func TestIntegration_GuardBlocksOnExhaustedBudget(t *testing.T) {
	t.Parallel()

	t.Run("guard returns error when budget exhausted", func(t *testing.T) {
		t.Parallel()

		store, err := cache.Open("file::rl_integration_guard?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
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

		// Create BudgetManager with 0 remaining (exhausted budget)
		budget := ratelimit.NewBudgetManager()
		budget.RecordResponse(0, time.Now().Add(1*time.Hour).Unix())

		metrics := ratelimit.NewMetrics()
		guard := NewRateLimitGuard(budget, metrics, store, jobID)

		chunkSize, err := guard.CheckBudget(jobID, 100)

		if err == nil {
			t.Error("expected error when budget exhausted, got nil")
		}
		if chunkSize != 0 {
			t.Errorf("expected chunk size 0, got %d", chunkSize)
		}

		// Verify job was paused
		pausedJobs, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		found := false
		for _, j := range pausedJobs {
			if j.Repo == "test/repo" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected job to be paused when budget exhausted")
		}

		// Verify metrics were updated
		snapshot := metrics.Snapshot()
		if snapshot.BudgetPauses != 1 {
			t.Errorf("expected 1 budget pause, got %d", snapshot.BudgetPauses)
		}
	})

	t.Run("guard allows request when budget sufficient", func(t *testing.T) {
		t.Parallel()

		store, err := cache.Open("file::rl_integration_guard_sufficient?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
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

		// Create BudgetManager with sufficient remaining budget
		budget := ratelimit.NewBudgetManager()
		budget.RecordResponse(500, time.Now().Add(1*time.Hour).Unix())

		metrics := ratelimit.NewMetrics()
		guard := NewRateLimitGuard(budget, metrics, store, jobID)

		chunkSize, err := guard.CheckBudget(jobID, 100)

		if err != nil {
			t.Errorf("expected no error when budget sufficient, got: %v", err)
		}
		if chunkSize <= 0 {
			t.Errorf("expected positive chunk size, got %d", chunkSize)
		}

		// Verify job was NOT paused
		pausedJobs, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		if len(pausedJobs) != 0 {
			t.Errorf("expected 0 paused jobs, got %d", len(pausedJobs))
		}
	})
}

// TestIntegration_ResumeJobNotDue tests that ResumeJob returns error for non-overdue jobs.
func TestIntegration_ResumeJobNotDue(t *testing.T) {
	t.Parallel()

	t.Run("ResumeJob returns error for future scheduled job", func(t *testing.T) {
		t.Parallel()

		store, err := cache.Open("file::rl_integration_resume_future?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
		}
		defer store.Close()

		repo := "test/repo"
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with a future time
		futureTime := time.Now().Add(10 * time.Minute)
		if err := store.PauseSyncJob(job.ID, futureTime, "rate limit exhausted"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Call ResumeJob and verify it returns error about not yet due
		_, err = ResumeJob(store, repo)
		if err == nil {
			t.Fatal("expected error for non-overdue job, got nil")
		}
		if !strings.Contains(err.Error(), "not yet due") {
			t.Errorf("expected error containing 'not yet due', got: %v", err)
		}
	})

	t.Run("ResumeJob returns error for zero scheduled time", func(t *testing.T) {
		t.Parallel()

		store, err := cache.Open("file::rl_integration_resume_zero?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
		}
		defer store.Close()

		repo := "test/repo"
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with zero time (no scheduled time)
		zeroTime := time.Time{}
		if err := store.PauseSyncJob(job.ID, zeroTime, "rate limit exhausted"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Call ResumeJob and verify it returns error about no scheduled time
		_, err = ResumeJob(store, repo)
		if err == nil {
			t.Fatal("expected error for job with no scheduled time, got nil")
		}
		if !strings.Contains(err.Error(), "no scheduled time") {
			t.Errorf("expected error containing 'no scheduled time', got: %v", err)
		}
	})

	t.Run("ResumeJob succeeds for overdue job", func(t *testing.T) {
		t.Parallel()

		store, err := cache.Open("file::rl_integration_resume_past?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
		}
		defer store.Close()

		repo := "test/repo"
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with a past time
		pastTime := time.Now().Add(-24 * time.Hour)
		if err := store.PauseSyncJob(job.ID, pastTime, "rate limit exhausted"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Call ResumeJob and verify it succeeds
		resumedJob, err := ResumeJob(store, repo)
		if err != nil {
			t.Fatalf("expected no error for overdue job, got: %v", err)
		}
		if resumedJob.Status != cache.SyncJobStatusResuming {
			t.Errorf("expected status %s, got %s", cache.SyncJobStatusResuming, resumedJob.Status)
		}
	})
}

// TestIntegration_ChunkSizeCalculation tests chunk size calculation with real BudgetManager.
func TestIntegration_ChunkSizeCalculation(t *testing.T) {
	t.Parallel()

	t.Run("chunk size with sufficient budget", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager()
		// NewBudgetManager defaults to 5000 remaining
		// With reserve buffer of 200, available = 4800
		// At 1 request per PR, chunk = min(totalPRs, 4800/1) = min(100, 4800) = 100

		chunkSize := ratelimit.CalculateChunkSize(100, budget.Remaining(), 200, ratelimit.WithRequestsPerPR(1))
		if chunkSize != 100 {
			t.Errorf("expected chunk size 100, got %d", chunkSize)
		}
	})

	t.Run("chunk size with limited budget", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager()
		budget.RecordResponse(250, time.Now().Add(1*time.Hour).Unix()) // Below reserve buffer + some

		// With 250 remaining and 200 reserve, available = 50
		// At 3 requests per PR (default), chunk = min(100, 50/3) = 16
		chunkSize := ratelimit.CalculateChunkSize(100, budget.Remaining(), 200, ratelimit.WithRequestsPerPR(3))
		if chunkSize != 16 {
			t.Errorf("expected chunk size 16, got %d", chunkSize)
		}
	})

	t.Run("chunk size with exhausted budget", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager()
		budget.RecordResponse(0, time.Now().Add(1*time.Hour).Unix())

		chunkSize := ratelimit.CalculateChunkSize(100, budget.Remaining(), 200, ratelimit.WithRequestsPerPR(1))
		if chunkSize != 0 {
			t.Errorf("expected chunk size 0 for exhausted budget, got %d", chunkSize)
		}
	})

	t.Run("chunk size limited by total PRs", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager()
		// Budget has plenty but totalPRs is smaller
		// available = 5000 - 200 = 4800
		// chunk = min(10, 4800/3) = min(10, 1600) = 10
		chunkSize := ratelimit.CalculateChunkSize(10, budget.Remaining(), 200, ratelimit.WithRequestsPerPR(3))
		if chunkSize != 10 {
			t.Errorf("expected chunk size 10 (limited by totalPRs), got %d", chunkSize)
		}
	})

	t.Run("BudgetManager CanAfford reflects correctly", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager()

		// Default 5000 remaining, reserve 200
		if !budget.CanAfford(100) {
			t.Error("expected CanAfford(100) to be true with 5000 remaining")
		}
		if budget.CanAfford(4900) {
			t.Error("expected CanAfford(4900) to be false (needs 4900 + 200 reserve = 5100)")
		}

		// Set below reserve
		budget.RecordResponse(150, time.Now().Add(1*time.Hour).Unix())
		if budget.CanAfford(100) {
			t.Error("expected CanAfford(100) to be false with 150 remaining (below 200 reserve)")
		}
	})

	t.Run("guard uses BudgetManager for chunk calculation", func(t *testing.T) {
		t.Parallel()

		store, err := cache.Open("file::rl_integration_chunk_guard?mode=memory&cache=shared")
		if err != nil {
			t.Fatalf("failed to open in-memory store: %v", err)
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

		// Budget with 5000 remaining
		budget := ratelimit.NewBudgetManager()
		metrics := ratelimit.NewMetrics()
		guard := NewRateLimitGuard(budget, metrics, store, jobID)

		chunkSize, err := guard.CheckBudget(jobID, 100)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// With 5000 remaining, 200 reserve, 100 estimated requests
		// available = 4800, chunkSize = min(1000000, 4800/1) but limited by the hardcoded 1000000
		// Actually looking at the guard code: CalculateChunkSize(1000000, g.budget.Remaining, 200, ratelimit.WithRequestsPerPR(1))
		// So it uses 1000000 as totalPRs, remaining=5000, reserve=200, requestsPerPR=1
		// available = 5000 - 200 = 4800
		// chunk = min(1000000, 4800/1) = 4800
		if chunkSize != 4800 {
			t.Errorf("expected chunk size 4800, got %d", chunkSize)
		}
	})
}
