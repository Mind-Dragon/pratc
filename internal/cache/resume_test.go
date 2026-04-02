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
