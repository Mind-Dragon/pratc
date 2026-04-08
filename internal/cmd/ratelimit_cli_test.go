package cmd

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

// TestRateLimitFlagParsing tests that rate limit flags are parsed correctly.
func TestRateLimitFlagParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		rateLimit     int
		reserveBuffer int
		resetBuffer   int
		wantLimit     int
		wantReserve   int
		wantReset     time.Duration
	}{
		{
			name:          "defaults",
			rateLimit:     5000,
			reserveBuffer: 200,
			resetBuffer:   15,
			wantLimit:     5000,
			wantReserve:   200,
			wantReset:     15 * time.Second,
		},
		{
			name:          "custom rate limit",
			rateLimit:     10000,
			reserveBuffer: 200,
			resetBuffer:   15,
			wantLimit:     10000,
			wantReserve:   200,
			wantReset:     15 * time.Second,
		},
		{
			name:          "custom reserve buffer",
			rateLimit:     5000,
			reserveBuffer: 500,
			resetBuffer:   15,
			wantLimit:     5000,
			wantReserve:   500,
			wantReset:     15 * time.Second,
		},
		{
			name:          "custom reset buffer",
			rateLimit:     5000,
			reserveBuffer: 200,
			resetBuffer:   30,
			wantLimit:     5000,
			wantReserve:   200,
			wantReset:     30 * time.Second,
		},
		{
			name:          "all custom",
			rateLimit:     3000,
			reserveBuffer: 300,
			resetBuffer:   60,
			wantLimit:     3000,
			wantReserve:   300,
			wantReset:     60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(tt.rateLimit),
				ratelimit.WithReserveBuffer(tt.reserveBuffer),
				ratelimit.WithResetBuffer(tt.resetBuffer),
			)

			if budget.Limit != tt.wantLimit {
				t.Errorf("expected Limit=%d, got %d", tt.wantLimit, budget.Limit)
			}
		})
	}
}

// TestBudgetManagerInitialization tests BudgetManager initialization with various options.
func TestBudgetManagerInitialization(t *testing.T) {
	t.Parallel()

	t.Run("default initialization", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager()

		if budget.Limit != 5000 {
			t.Errorf("expected Limit=5000, got %d", budget.Limit)
		}
		if budget.Remaining() != 5000 {
			t.Errorf("expected Remaining=5000, got %d", budget.Remaining())
		}
	})

	t.Run("initialization with custom rate limit", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager(ratelimit.WithRateLimit(10000))

		if budget.Limit != 10000 {
			t.Errorf("expected Limit=10000, got %d", budget.Limit)
		}
		if budget.Remaining() != 5000 {
			t.Errorf("expected Remaining=5000 (not capped since 5000 < 10000), got %d", budget.Remaining())
		}
	})

	t.Run("initialization with all options", func(t *testing.T) {
		t.Parallel()

		metrics := ratelimit.NewMetrics()
		budget := ratelimit.NewBudgetManager(
			ratelimit.WithRateLimit(3000),
			ratelimit.WithReserveBuffer(500),
			ratelimit.WithResetBuffer(30),
			ratelimit.WithMetrics(metrics),
		)

		if budget.Limit != 3000 {
			t.Errorf("expected Limit=3000, got %d", budget.Limit)
		}
		if budget.ShouldPause() {
			t.Error("ShouldPause should be false since remaining=5000 > reserve=500")
		}
	})
}

// TestPauseBehaviorWhenBudgetExhausted tests that operations pause correctly when budget is exhausted.
func TestPauseBehaviorWhenBudgetExhausted(t *testing.T) {
	tests := []struct {
		name             string
		remaining        int
		reserveBuffer    int
		estimatedRequest int
		wantPause        bool
		wantErr          bool
	}{
		{
			name:             "fresh budget does not pause",
			remaining:        5000,
			reserveBuffer:    200,
			estimatedRequest: 100,
			wantPause:        false,
			wantErr:          false,
		},
		{
			name:             "at reserve boundary pauses",
			remaining:        200,
			reserveBuffer:    200,
			estimatedRequest: 100,
			wantPause:        true,
			wantErr:          true,
		},
		{
			name:             "below reserve boundary pauses",
			remaining:        100,
			reserveBuffer:    200,
			estimatedRequest: 100,
			wantPause:        true,
			wantErr:          true,
		},
		{
			name:             "exhausted budget pauses",
			remaining:        0,
			reserveBuffer:    200,
			estimatedRequest: 100,
			wantPause:        true,
			wantErr:          true,
		},
		{
			name:             "insufficient budget for request",
			remaining:        500,
			reserveBuffer:    200,
			estimatedRequest: 400,
			wantPause:        false,
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "pratc.db")
			t.Setenv("PRATC_DB_PATH", dbPath)

			store, err := cache.Open(dbPath)
			if err != nil {
				t.Fatalf("open cache store: %v", err)
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

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(5000),
				ratelimit.WithReserveBuffer(tt.reserveBuffer),
			)
			budget.RecordResponse(tt.remaining, time.Now().Add(1*time.Hour).Unix())

			metrics := ratelimit.NewMetrics()
			guard := sync.NewRateLimitGuard(budget, metrics, store, jobID)

			// Test ShouldPause
			shouldPause := guard.ShouldPause()
			if shouldPause != tt.wantPause {
				t.Errorf("ShouldPause() = %v, want %v", shouldPause, tt.wantPause)
			}

			// Test CheckBudget
			chunkSize, err := guard.CheckBudget(jobID, tt.estimatedRequest)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckBudget() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantPause {
				pausedJobs, err := store.ListPausedSyncJobs()
				if err != nil {
					t.Fatalf("failed to list paused jobs: %v", err)
				}
				if len(pausedJobs) != 1 {
					t.Errorf("expected 1 paused job, got %d", len(pausedJobs))
				}
			} else if tt.wantErr {
				pausedJobs, err := store.ListPausedSyncJobs()
				if err != nil {
					t.Fatalf("failed to list paused jobs: %v", err)
				}
				if len(pausedJobs) != 1 {
					t.Errorf("expected 1 paused job (CheckBudget error paused job), got %d", len(pausedJobs))
				}
			} else {
				pausedJobs, err := store.ListPausedSyncJobs()
				if err != nil {
					t.Fatalf("failed to list paused jobs: %v", err)
				}
				if len(pausedJobs) != 0 {
					t.Errorf("expected 0 paused jobs, got %d", len(pausedJobs))
				}
				if chunkSize <= 0 && tt.estimatedRequest > 0 {
					t.Errorf("expected positive chunkSize, got %d", chunkSize)
				}
			}
		})
	}
}

// TestResumeFromPausedState tests that jobs can be resumed from paused state.
func TestResumeFromPausedState(t *testing.T) {
	t.Run("scheduler resumes overdue paused job", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "pratc.db")
		t.Setenv("PRATC_DB_PATH", dbPath)

		store, err := cache.Open(dbPath)
		if err != nil {
			t.Fatalf("open cache store: %v", err)
		}
		defer store.Close()

		repo := "test/repo"
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		pastTime := time.Now().Add(-24 * time.Hour)
		if err := store.PauseSyncJob(job.ID, pastTime, "rate limit exhausted"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		pausedJobs, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		if len(pausedJobs) != 1 {
			t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
		}

		scheduler := sync.NewScheduler(store)
		if err := scheduler.CheckAndResume(context.Background()); err != nil {
			t.Fatalf("CheckAndResume failed: %v", err)
		}

		pausedJobs, err = store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs after resume: %v", err)
		}
		if len(pausedJobs) != 0 {
			t.Errorf("expected 0 paused jobs after resume, got %d", len(pausedJobs))
		}

		resumedJob, err := store.GetSyncJob(job.ID)
		if err != nil {
			t.Fatalf("failed to get resumed job: %v", err)
		}
		if resumedJob.Status != cache.SyncJobStatusInProgress {
			t.Errorf("expected status %s, got %s", cache.SyncJobStatusInProgress, resumedJob.Status)
		}
	})

	t.Run("scheduler skips non-overdue paused job", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "pratc.db")
		t.Setenv("PRATC_DB_PATH", dbPath)

		store, err := cache.Open(dbPath)
		if err != nil {
			t.Fatalf("open cache store: %v", err)
		}
		defer store.Close()

		repo := "test/repo"
		job, err := store.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("failed to create sync job: %v", err)
		}

		futureTime := time.Now().Add(10 * time.Minute)
		if err := store.PauseSyncJob(job.ID, futureTime, "rate limit exhausted"); err != nil {
			t.Fatalf("failed to pause sync job: %v", err)
		}

		scheduler := sync.NewScheduler(store)
		if err := scheduler.CheckAndResume(context.Background()); err != nil {
			t.Fatalf("CheckAndResume failed: %v", err)
		}

		pausedJobs, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		if len(pausedJobs) != 1 {
			t.Errorf("expected 1 paused job (not overdue), got %d", len(pausedJobs))
		}
	})
}

// TestProgressAndETAOutput tests the progress and ETA calculation functions.
func TestProgressAndETAOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		done       int
		total      int
		elapsed    time.Duration
		wantZero   bool
		wantEstMin time.Duration
		wantEstMax time.Duration
	}{
		{
			name:     "zero done returns zero ETA",
			done:     0,
			total:    100,
			elapsed:  10 * time.Second,
			wantZero: true,
		},
		{
			name:     "done equals total returns zero ETA",
			done:     100,
			total:    100,
			elapsed:  10 * time.Second,
			wantZero: true,
		},
		{
			name:       "partial completion returns positive ETA",
			done:       50,
			total:      100,
			elapsed:    10 * time.Second,
			wantZero:   false,
			wantEstMin: 8 * time.Second, // ~10s remaining at same rate
			wantEstMax: 12 * time.Second,
		},
		{
			name:       "slow rate returns longer ETA",
			done:       10,
			total:      100,
			elapsed:    10 * time.Second,
			wantZero:   false,
			wantEstMin: 80 * time.Second, // ~90s remaining at 1 req/s
			wantEstMax: 100 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			eta := calculateETA(tt.done, tt.total, tt.elapsed)

			if tt.wantZero {
				if eta != 0 {
					t.Errorf("expected zero ETA, got %v", eta)
				}
			} else {
				if eta == 0 {
					t.Errorf("expected non-zero ETA, got 0")
				}
				if eta < tt.wantEstMin || eta > tt.wantEstMax {
					t.Errorf("ETA = %v, want between %v and %v", eta, tt.wantEstMin, tt.wantEstMax)
				}
			}
		})
	}
}

// TestBudgetManagerStringOutput tests the human-readable budget string output.
func TestBudgetManagerStringOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		remaining   int
		resetIn     time.Duration
		wantContain []string
	}{
		{
			name:        "fresh budget",
			remaining:   4800,
			resetIn:     45 * time.Minute,
			wantContain: []string{"4800", "5000"},
		},
		{
			name:        "half budget",
			remaining:   2500,
			resetIn:     30 * time.Minute,
			wantContain: []string{"2500", "5000"},
		},
		{
			name:        "low budget",
			remaining:   300,
			resetIn:     15 * time.Minute,
			wantContain: []string{"300", "5000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(5000),
				ratelimit.WithReserveBuffer(200),
			)
			budget.RecordResponse(tt.remaining, time.Now().Add(tt.resetIn).Unix())

			s := budget.String()

			if s == "" {
				t.Error("String() returned empty string")
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(s, want) {
					t.Errorf("String() should contain %q, got: %s", want, s)
				}
			}
		})
	}
}

// TestBudgetManagerWithMetrics tests that metrics are properly tracked.
func TestBudgetManagerWithMetrics(t *testing.T) {
	t.Parallel()

	t.Run("metrics track budget pauses", func(t *testing.T) {
		t.Parallel()

		metrics := ratelimit.NewMetrics()
		budget := ratelimit.NewBudgetManager(ratelimit.WithMetrics(metrics))

		// Initially should not pause
		if budget.ShouldPause() {
			t.Error("ShouldPause() = true, want false with fresh budget")
		}
		snap := metrics.Snapshot()
		if snap.BudgetPauses != 0 {
			t.Errorf("BudgetPauses = %d, want 0", snap.BudgetPauses)
		}

		// Set at reserve boundary
		budget.RecordResponse(200, time.Now().Add(30*time.Minute).Unix())

		// At boundary, should pause
		if !budget.ShouldPause() {
			t.Error("ShouldPause() = false, want true at reserve boundary")
		}
		snap = metrics.Snapshot()
		if snap.BudgetPauses != 1 {
			t.Errorf("BudgetPauses = %d, want 1", snap.BudgetPauses)
		}

		// Second pause
		if !budget.ShouldPause() {
			t.Error("ShouldPause() = false, want true at reserve boundary (second call)")
		}
		snap = metrics.Snapshot()
		if snap.BudgetPauses != 2 {
			t.Errorf("BudgetPauses = %d, want 2", snap.BudgetPauses)
		}
	})

	t.Run("no metrics does not panic", func(t *testing.T) {
		t.Parallel()

		budget := ratelimit.NewBudgetManager()

		// Set below reserve
		budget.RecordResponse(100, time.Now().Add(30*time.Minute).Unix())

		// Should pause without metrics
		if !budget.ShouldPause() {
			t.Error("ShouldPause() = false, want true below reserve")
		}
	})
}

// TestEstimatedCompletionTime tests the budget estimation functions.
func TestEstimatedCompletionTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		remaining int
		work      int
		wantMin   time.Duration
		wantMax   time.Duration
	}{
		{
			name:      "no work",
			remaining: 5000,
			work:      0,
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "sufficient budget",
			remaining: 5000,
			work:      100,
			wantMin:   95 * time.Second, // allow 5s tolerance
			wantMax:   105 * time.Second,
		},
		{
			name:      "insufficient budget - needs reset",
			remaining: 100,
			work:      500,
			wantMin:   20*time.Minute + 15*time.Second + 500*time.Second - 10*time.Second,
			wantMax:   21*time.Minute + 15*time.Second + 500*time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			budget := ratelimit.NewBudgetManager()
			resetTime := time.Now().Add(20 * time.Minute)
			budget.RecordResponse(tt.remaining, resetTime.Unix())

			got := budget.EstimatedCompletionTime(tt.work)

			if tt.wantMin == 0 && tt.wantMax == 0 {
				if got != 0 {
					t.Errorf("ExpectedCompletionTime(%d) = %v, want 0", tt.work, got)
				}
			} else {
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("EstimatedCompletionTime(%d) = %v, want between %v and %v",
						tt.work, got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

// TestCalculateChunkSize tests the chunk size calculation.
func TestCalculateChunkSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		totalPRs        int
		remainingBudget int
		reserveBuffer   int
		wantChunk       int
	}{
		{
			name:            "sufficient budget for all PRs",
			totalPRs:        100,
			remainingBudget: 5000,
			reserveBuffer:   200,
			wantChunk:       100,
		},
		{
			name:            "limited budget partial chunk",
			totalPRs:        100,
			remainingBudget: 300,
			reserveBuffer:   200,
			wantChunk:       33, // (300-200)/3 = 33 with 3 requests per PR
		},
		{
			name:            "exhausted budget",
			totalPRs:        100,
			remainingBudget: 0,
			reserveBuffer:   200,
			wantChunk:       0,
		},
		{
			name:            "zero total PRs",
			totalPRs:        0,
			remainingBudget: 5000,
			reserveBuffer:   200,
			wantChunk:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chunkSize := ratelimit.CalculateChunkSize(
				tt.totalPRs,
				tt.remainingBudget,
				tt.reserveBuffer,
				ratelimit.WithRequestsPerPR(3),
			)

			if chunkSize != tt.wantChunk {
				t.Errorf("CalculateChunkSize() = %d, want %d", chunkSize, tt.wantChunk)
			}
		})
	}
}
