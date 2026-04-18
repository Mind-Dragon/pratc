package cache

import (
	"strings"
	"testing"
	"time"
)

func TestResumeSyncJobByID(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	// Pause the job
	pastTime := time.Now().Add(-24 * time.Hour)
	if err := store.PauseSyncJob(job.ID, pastTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	// Verify the job is paused
	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
	}

	// Resume the job
	if err := store.ResumeSyncJobByID(job.ID); err != nil {
		t.Fatalf("failed to resume sync job: %v", err)
	}

	// Verify the job is no longer paused
	pausedJobs, err = store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs after resume: %v", err)
	}
	if len(pausedJobs) != 0 {
		t.Fatalf("expected 0 paused jobs after resume, got %d", len(pausedJobs))
	}
}

func TestGetPausedSyncJobByRepo(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	// Pause the job
	pastTime := time.Now().Add(-24 * time.Hour)
	if err := store.PauseSyncJob(job.ID, pastTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	gotJob, err := store.GetPausedSyncJobByRepo(repo)
	if err != nil {
		t.Fatalf("failed to get paused sync job by repo: %v", err)
	}

	if gotJob.ID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, gotJob.ID)
	}
	if gotJob.Status != SyncJobStatusPausedRateLimit {
		t.Errorf("expected status %s, got %s", SyncJobStatusPausedRateLimit, gotJob.Status)
	}
}

func TestResumeSyncJobClearsPauseFields(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	if err := store.UpdateSyncJobProgress(job.ID, SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 12,
		TotalPRs:     34,
	}); err != nil {
		t.Fatalf("failed to update progress: %v", err)
	}

	pastTime := time.Now().Add(-24 * time.Hour)
	if err := store.PauseSyncJob(job.ID, pastTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	resumed, err := ResumeSyncJob(store, repo)
	if err != nil {
		t.Fatalf("failed to resume sync job: %v", err)
	}

	if resumed.Status != SyncJobStatusResuming {
		t.Fatalf("expected status %s, got %s", SyncJobStatusResuming, resumed.Status)
	}
	if resumed.Error != "" {
		t.Fatalf("expected error cleared, got %q", resumed.Error)
	}
	if resumed.Progress.Cursor != "cursor-1" || resumed.Progress.ProcessedPRs != 12 || resumed.Progress.TotalPRs != 34 {
		t.Fatalf("expected progress preserved, got %+v", resumed.Progress)
	}
	if !resumed.Progress.NextScheduledAt.IsZero() {
		t.Fatalf("expected next scheduled time cleared, got %s", resumed.Progress.NextScheduledAt)
	}
	if !resumed.Progress.ScheduledResumeAt.IsZero() {
		t.Fatalf("expected scheduled resume cleared, got %s", resumed.Progress.ScheduledResumeAt)
	}
	if resumed.Progress.PauseReason != "" {
		t.Fatalf("expected pause reason cleared, got %q", resumed.Progress.PauseReason)
	}
	if !resumed.Progress.LastBudgetCheck.IsZero() {
		t.Fatalf("expected last budget check cleared, got %s", resumed.Progress.LastBudgetCheck)
	}
}

func TestResumeSyncJobRebindsSharedProgressRow(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	pausedJob, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create paused job: %v", err)
	}
	if err := store.UpdateSyncJobProgress(pausedJob.ID, SyncProgress{
		Cursor:       "cursor-paused",
		ProcessedPRs: 10,
		TotalPRs:     40,
	}); err != nil {
		t.Fatalf("failed to seed paused job progress: %v", err)
	}
	if err := store.PauseSyncJob(pausedJob.ID, time.Now().Add(-time.Hour), "rate limited"); err != nil {
		t.Fatalf("failed to pause job: %v", err)
	}

	activeJob, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create active job: %v", err)
	}
	if err := store.UpdateSyncJobProgress(activeJob.ID, SyncProgress{
		Cursor:       "cursor-active",
		ProcessedPRs: 12,
		TotalPRs:     50,
	}); err != nil {
		t.Fatalf("failed to seed active job progress: %v", err)
	}

	resumed, err := ResumeSyncJob(store, repo)
	if err != nil {
		t.Fatalf("failed to resume paused job with shared progress row: %v", err)
	}
	if resumed.ID != pausedJob.ID {
		t.Fatalf("expected paused job %s to resume, got %s", pausedJob.ID, resumed.ID)
	}
	if resumed.Progress.Cursor != "cursor-active" {
		t.Fatalf("expected shared progress row to be rebound, got %+v", resumed.Progress)
	}
	if resumed.Status != SyncJobStatusResuming {
		t.Fatalf("expected resuming status, got %s", resumed.Status)
	}
}

// Test 1: Invalid jobID in PauseSyncJob
// Verifies that PauseSyncJob returns an error when called with a non-existent job ID
func TestPauseSyncJobInvalidJobID(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	// Create a valid job first
	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	// Delete the job to simulate a stale/invalid ID scenario
	// (In real usage, this would be a fake ID or a job that was deleted)
	_, err = store.db.Exec("DELETE FROM sync_jobs WHERE id = ?", job.ID)
	if err != nil {
		t.Fatalf("failed to delete sync job: %v", err)
	}

	// Try to pause the deleted job
	pauseTime := time.Now().Add(1 * time.Hour)
	err = store.PauseSyncJob(job.ID, pauseTime, "rate limit")

	// Expected behavior: returns an error indicating the job doesn't exist
	if err == nil {
		t.Fatal("expected error when pausing non-existent job, got nil")
	}

	// Error should mention lookup failure
	if !strings.Contains(err.Error(), "lookup sync job repo after pause") && !strings.Contains(err.Error(), "no rows") {
		t.Fatalf("expected error about job lookup, got: %v", err)
	}
}

// Test 2: Invalid jobID in ResumeSyncJobByID
// Verifies that ResumeSyncJobByID returns an error when called with a non-existent job ID
func TestResumeSyncJobByIDInvalidJobID(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	// Try to resume a job with a fake ID that doesn't exist
	fakeJobID := "non-existent-job-id-12345"
	err = store.ResumeSyncJobByID(fakeJobID)

	// Expected behavior: returns an error indicating the job doesn't exist
	if err == nil {
		t.Fatal("expected error when resuming non-existent job, got nil")
	}

	// Error should indicate no job found
	if !strings.Contains(err.Error(), "no job found with ID") {
		t.Fatalf("expected error 'no job found with ID', got: %v", err)
	}
}

// Test 3: Idempotency - double pause
// Documents behavior when pausing an already-paused job
func TestPauseSyncJobIdempotency(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	// First pause
	firstPauseTime := time.Now().Add(1 * time.Hour)
	firstReason := "first rate limit"
	if err := store.PauseSyncJob(job.ID, firstPauseTime, firstReason); err != nil {
		t.Fatalf("failed to pause sync job first time: %v", err)
	}

	// Verify first pause worked
	got, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get sync job after first pause: %v", err)
	}
	if got.Status != SyncJobStatusPausedRateLimit {
		t.Fatalf("expected status %s after first pause, got %s", SyncJobStatusPausedRateLimit, got.Status)
	}

	// Second pause (idempotency test)
	secondPauseTime := time.Now().Add(2 * time.Hour)
	secondReason := "second rate limit"
	err = store.PauseSyncJob(job.ID, secondPauseTime, secondReason)

	// Current behavior: No error returned, job remains paused with updated fields
	// This is acceptable - the job stays paused, and the pause fields are updated
	if err != nil {
		t.Fatalf("unexpected error on second pause: %v", err)
	}

	// Verify job is still paused
	got, err = store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get sync job after second pause: %v", err)
	}
	if got.Status != SyncJobStatusPausedRateLimit {
		t.Fatalf("expected status %s after second pause, got %s", SyncJobStatusPausedRateLimit, got.Status)
	}

	// Verify the pause fields were updated with the second pause values
	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
	}

	// The second pause should have updated the fields
	if pausedJobs[0].Progress.PauseReason != secondReason {
		t.Fatalf("expected pause reason %q after second pause, got %q", secondReason, pausedJobs[0].Progress.PauseReason)
	}

	// Job remains paused - idempotency achieved through state consistency
	t.Log("Idempotency behavior: Second pause succeeds, updates pause fields, job remains paused")
}

// Test 4: Idempotency - double resume
// Documents behavior when resuming an already-resumed (in-progress) job
func TestResumeSyncJobByIDIdempotency(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	// Pause the job first
	pauseTime := time.Now().Add(-24 * time.Hour)
	if err := store.PauseSyncJob(job.ID, pauseTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	// First resume
	if err := store.ResumeSyncJobByID(job.ID); err != nil {
		t.Fatalf("failed to resume sync job first time: %v", err)
	}

	// Verify job is now resuming
	got, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get sync job after first resume: %v", err)
	}
	if got.Status != SyncJobStatusResuming {
		t.Fatalf("expected status %s after first resume, got %s", SyncJobStatusResuming, got.Status)
	}

	// Second resume (idempotency test) - job is already resuming, not paused
	err = store.ResumeSyncJobByID(job.ID)

	// Current behavior: Succeeds (no error) because the UPDATE doesn't check status
	// The job status is set to resuming again (no-op), rowsAffected = 1
	if err != nil {
		t.Fatalf("unexpected error on second resume: %v", err)
	}

	// Verify job is still resuming (state unchanged)
	got, err = store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get sync job after second resume: %v", err)
	}
	if got.Status != SyncJobStatusResuming {
		t.Fatalf("expected status %s to remain unchanged, got %s", SyncJobStatusResuming, got.Status)
	}

	t.Log("Idempotency behavior: Second resume succeeds (no-op), job remains resuming")
}
