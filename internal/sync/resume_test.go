package sync

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

func TestWorkerResumeSyncJob(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory cache: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	pastTime := time.Now().UTC().Add(-10 * time.Minute)
	if err := store.PauseSyncJob(job.ID, pastTime, "rate limit exceeded"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	worker := Worker{
		CacheStore: store,
		MirrorFactory: func(context.Context, string) (Mirror, error) {
			return &fakeMirror{drift: map[int]string{}}, nil
		},
		Metadata: fakeMetadata{snapshot: MetadataSnapshot{
			OpenPRs:       []int{1},
			ClosedPRs:     nil,
			RemotePRHeads: map[int]string{1: "abc"},
			SyncedPRs:     1,
		}},
	}

	result, err := worker.ResumeSyncJob(context.Background(), repo, nil, nil)
	if err != nil {
		t.Fatalf("expected resume sync job to succeed, got error: %v", err)
	}
	if result == nil || result.Repo != repo {
		t.Fatalf("unexpected sync result: %+v", result)
	}

	resumedJob, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get resumed sync job: %v", err)
	}
	if resumedJob.Status != cache.SyncJobStatusResuming {
		t.Fatalf("expected status %s, got %s", cache.SyncJobStatusResuming, resumedJob.Status)
	}
	if resumedJob.Error != "" {
		t.Fatalf("expected error cleared, got %q", resumedJob.Error)
	}
	if resumedJob.Progress.PauseReason != "" || !resumedJob.Progress.ScheduledResumeAt.IsZero() || !resumedJob.Progress.NextScheduledAt.IsZero() {
		t.Fatalf("expected pause fields cleared, got %+v", resumedJob.Progress)
	}
}

func TestResumeJob(t *testing.T) {
	t.Run("resume of overdue paused job succeeds", func(t *testing.T) {
		store, err := cache.Open(":memory:")
		if err != nil {
			t.Fatalf("failed to open in-memory cache: %v", err)
		}
		defer store.Close()

		repo := "test/repo"

		// Create a sync job
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with a past scheduled time
		pastTime := time.Now().UTC().Add(-10 * time.Minute)
		if err := store.PauseSyncJob(job.ID, pastTime, "rate limit exceeded"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Resume the job
		resumedJob, err := ResumeJob(store, repo)
		if err != nil {
			t.Fatalf("expected resume to succeed, got error: %v", err)
		}

		if resumedJob.Status != cache.SyncJobStatusResuming {
			t.Errorf("expected status to be resuming, got %s", resumedJob.Status)
		}

		if resumedJob.Error != "" {
			t.Errorf("expected error message to be empty, got %q", resumedJob.Error)
		}
	})

	t.Run("resume of non-existent job fails", func(t *testing.T) {
		store, err := cache.Open(":memory:")
		if err != nil {
			t.Fatalf("failed to open in-memory cache: %v", err)
		}
		defer store.Close()

		nonExistentRepo := "non/existent"
		_, err = ResumeJob(store, nonExistentRepo)
		if err == nil {
			t.Fatal("expected error for non-existent job, got nil")
		}
		if err.Error() != `resume job: no paused sync job found for repo "non/existent"` {
			t.Errorf("expected specific error message, got: %v", err)
		}
	})

	t.Run("resume of not-yet-due job fails", func(t *testing.T) {
		store, err := cache.Open(":memory:")
		if err != nil {
			t.Fatalf("failed to open in-memory cache: %v", err)
		}
		defer store.Close()

		repo := "test/repo"

		// Create a sync job
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with a future scheduled time
		futureTime := time.Now().UTC().Add(10 * time.Minute)
		if err := store.PauseSyncJob(job.ID, futureTime, "rate limit exceeded"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Try to resume the job
		_, err = ResumeJob(store, repo)
		if err == nil {
			t.Fatal("expected error for not-yet-due job, got nil")
		}
		if err.Error() != "sync job not yet due for resume, scheduled at "+futureTime.Format(time.RFC3339) {
			t.Errorf("expected specific error message, got: %v", err)
		}
	})

	t.Run("resume updates status to in_progress", func(t *testing.T) {
		store, err := cache.Open(":memory:")
		if err != nil {
			t.Fatalf("failed to open in-memory cache: %v", err)
		}
		defer store.Close()

		repo := "test/repo"

		// Create a sync job
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		// Pause the job with a past scheduled time
		pastTime := time.Now().UTC().Add(-5 * time.Minute)
		if err := store.PauseSyncJob(job.ID, pastTime, "rate limit exceeded"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		// Verify the job is paused
		pausedJobs, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		if len(pausedJobs) != 1 || pausedJobs[0].ID != job.ID {
			t.Fatalf("expected one paused job, got %d jobs", len(pausedJobs))
		}

		// Resume the job
		resumedJob, err := ResumeJob(store, repo)
		if err != nil {
			t.Fatalf("expected resume to succeed, got error: %v", err)
		}

		// Verify the job is no longer paused
		pausedJobs, err = store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs after resume: %v", err)
		}
		if len(pausedJobs) != 0 {
			t.Fatalf("expected no paused jobs after resume, got %d", len(pausedJobs))
		}

		if resumedJob.Status != cache.SyncJobStatusResuming {
			t.Errorf("expected status to be resuming, got %s", resumedJob.Status)
		}
	})
}
