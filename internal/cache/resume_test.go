package cache

import (
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
	if gotJob.Status != SyncJobStatusPaused {
		t.Errorf("expected status %s, got %s", SyncJobStatusPaused, gotJob.Status)
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

	if resumed.Status != SyncJobStatusInProgress {
		t.Fatalf("expected status %s, got %s", SyncJobStatusInProgress, resumed.Status)
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
