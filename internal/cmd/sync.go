package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
	prsync "github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

// Preflight analysis for sync operation.
func runPreflightAnalysis(ctx context.Context, repo string, rateLimit, reserveBuffer, resetBuffer, syncMaxPRs int) error {
	log := logger.New("sync")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	// Get current rate limit status
	budget := ratelimit.NewBudgetManager(
		ratelimit.WithRateLimit(rateLimit),
		ratelimit.WithReserveBuffer(reserveBuffer),
		ratelimit.WithResetBuffer(resetBuffer),
	)

	// Estimate number of PRs to sync
	// For preflight, we'll check the cache first to see how many PRs are already cached
	// If no cache, we need to estimate from GitHub API
	dbPath := os.Getenv("PRATC_DB_PATH")
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open cache store for preflight: %w", err)
	}
	defer store.Close()

	// Check cached PR count
	cachedPRs, err := store.ListPRs(cache.PRFilter{Repo: repo})
	if err != nil {
		// If no cache exists, we'll need to estimate from GitHub API
		// For preflight, we can try to fetch just the total count without fetching all PRs
		// But that still requires API calls. We'll assume we need to fetch all.
		// Let's try to get the total count via GitHub API with minimal requests
		// For now, we'll just use a placeholder
		log.Warn("Preflight: Could not read cache, estimating from GitHub API...")
		// We'll need to implement a lightweight count fetch
		// For now, return an error or estimate
		return fmt.Errorf("cannot read cache for preflight: %w", err)
	}

	cachedCount := len(cachedPRs)
	log.Info("Preflight analysis for repo %s", "repo", repo)
	log.Info("  Cached PRs: %d", "count", cachedCount)

	// If we have a cached snapshot, we can check when it was last synced
	// and estimate delta based on new PRs since then.
	// For simplicity, we'll assume we need to fetch all PRs if no recent sync.
	// In a real implementation, we'd query GitHub for the number of open PRs and compare with cached count.

	// Estimate API requests needed
	// Full enrichment: ~4 requests per PR (list + files + reviews + CI)
	// For preflight, we'll assume full enrichment unless --no-enrich flag is set
	// For now, use a conservative estimate
	estimatedRequests := ratelimit.EstimateSyncCost(cachedCount)
	log.Info("  Estimated API requests: %d", "requests", estimatedRequests)

	// Check current rate limit status
	remaining := budget.Remaining()
	resetAt := budget.ResetAt()
	log.Info("  Current rate limit: %d remaining (reserving %d)", "remaining", remaining, "reserve", budget.ReserveBuffer())
	log.Info("  Rate limit resets in: %v", "reset_in", time.Until(resetAt))

	// Estimate completion time
	if remaining < estimatedRequests {
		// Not enough budget, need to wait
		waitTime := time.Until(resetAt) + time.Duration(resetBuffer)*time.Second
		log.Warn("  WARNING: Insufficient rate limit budget!")
		log.Info("    Need: %d requests, Available: %d (after reserve)", "need", estimatedRequests, "available", remaining)
		log.Info("    Estimated wait time: %v", "wait_time", waitTime)
		log.Info("    Estimated completion: %s", "completion", time.Now().Add(waitTime).Format(time.RFC3339))
	} else {
		// Enough budget, estimate based on 1 req/sec rate
		estimatedDuration := time.Duration(estimatedRequests) * time.Second
		log.Info("  Estimated completion time: ~%v", "duration", estimatedDuration)
	}

	return nil
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Use a simple signal 0 check to see if process exists
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

func calculateETA(done, total int, elapsed time.Duration) time.Duration {
	if done <= 0 || total <= done {
		return 0
	}
	rate := float64(done) / elapsed.Seconds()
	remaining := total - done
	return time.Duration(float64(remaining)/rate) * time.Second
}

type syncCommandSummary struct {
	Started bool                       `json:"started"`
	Repo    string                     `json:"repo"`
	Status  string                     `json:"status"`
	JobID   string                     `json:"job_id"`
	Budget  string                     `json:"budget,omitempty"`
	Metrics *ratelimit.MetricsSnapshot `json:"metrics,omitempty"`
	Reused  bool                       `json:"reused,omitempty"`
}

func reuseCachedSyncSummary(store *cache.Store, repo string) (syncCommandSummary, bool, error) {
	if store == nil {
		return syncCommandSummary{}, false, nil
	}

	job, ok, err := store.GetLatestSyncJob(repo)
	if err != nil {
		return syncCommandSummary{}, false, fmt.Errorf("load latest sync job: %w", err)
	}
	if !ok || job.Status != cache.SyncJobStatusCompleted || job.LastSyncAt.IsZero() {
		return syncCommandSummary{}, false, nil
	}

	prs, err := store.ListPRs(cache.PRFilter{Repo: repo})
	if err != nil {
		return syncCommandSummary{}, false, fmt.Errorf("list cached prs: %w", err)
	}
	if len(prs) == 0 {
		return syncCommandSummary{}, false, nil
	}

	return syncCommandSummary{
		Started: true,
		Repo:    repo,
		Status:  string(cache.SyncJobStatusCompleted),
		JobID:   job.ID,
		Budget:  "local-first cache reuse",
		Reused:  true,
	}, true, nil
}

func RegisterSyncCommand() {
	var repo string
	var watch bool
	var interval time.Duration
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int
	var showProgress bool
	var syncMaxPRs int
	var refreshSync bool
	var force bool
	var preflight bool
	var wait bool

	// Main sync command
	command := &cobra.Command{
		Use:   "sync",
		Short: "Sync repository metadata and refs with rate-limit-aware scheduling.",
		Long: `Sync repository metadata and refs with rate-limit-aware scheduling.

The sync command fetches PR metadata and git refs from GitHub, respecting
rate limits and automatically pausing/resuming when budget is exhausted.

Examples:
  # One-time sync
  pratc sync --repo=owner/repo

  # Watch mode with progress display
  pratc sync --repo=owner/repo --watch --interval=5m --progress

  # Preflight analysis (dry run)
  pratc sync --repo=owner/repo --preflight

  # Custom rate limit configuration
  pratc sync --repo=owner/repo --rate-limit=5000 --reserve-buffer=300`,
		PreRun: func(cmd *cobra.Command, args []string) {
			if preflight {
				// Preflight check: estimate time and rate limit usage without actually syncing
				fmt.Fprintln(cmd.OutOrStdout(), "Running preflight analysis...")
				// Implementation will be added in Phase 1
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if preflight {
				return runPreflightAnalysis(ctx, repo, rateLimit, reserveBuffer, resetBuffer, syncMaxPRs)
			}

			requestID := uuid.New().String()
			ctx = logger.ContextWithRequestID(ctx, requestID)
			log := logger.FromContext(ctx)

			repo = types.NormalizeRepoName(repo)

			// Preflight analysis before acquiring lock
			if err := runPreflightAnalysis(ctx, repo, rateLimit, reserveBuffer, resetBuffer, syncMaxPRs); err != nil {
				return fmt.Errorf("preflight analysis failed: %w", err)
			}

			// Acquire repo lock
			var lock *RepoLock
			var err error
			if force {
				lock, err = ForceAcquireRepoLock(repo)
				if err != nil {
					return fmt.Errorf("force lock failed: %w", err)
				}
				log.Warn("force lock acquired - overriding existing instance")
			} else {
				lock, err = AcquireRepoLock(repo)
				if err != nil {
					return err
				}
			}
			defer lock.Release()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				select {
				case sig := <-sigChan:
					log.Info("received shutdown signal", "signal", sig.String())
					cancel()
				case <-ctx.Done():
				}
			}()

			dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
			if dbPath == "" {
				home, _ := os.UserHomeDir()
				dbPath = filepath.Join(home, ".pratc", "pratc.db")
			}
			store, usedFallback, err := openSyncStoreWithFallback(dbPath, log)
			if err != nil {
				return err
			}
			defer store.Close()
			if usedFallback {
				log.Warn("sync cache fallback active", "db_path", dbPath)
			}

			if !watch && !refreshSync {
				if summary, reused, err := reuseCachedSyncSummary(store, repo); err != nil {
					log.Warn("cache snapshot unavailable; continuing with live sync", "repo", repo, "error", err)
				} else if reused {
					log.Info("reusing local sync snapshot", "repo", repo)
					return writeJSON(cmd, summary)
				}
			}

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(rateLimit),
				ratelimit.WithReserveBuffer(reserveBuffer),
				ratelimit.WithResetBuffer(resetBuffer),
			)

			log.Info("starting sync", "repo", repo, "budget", budget.String())

			job, err := store.CreateSyncJob(repo)
			if err != nil {
				return fmt.Errorf("create sync job: %w", err)
			}

			startTime := time.Now()
			var lastProgress cache.SyncProgress
			var lastMetrics *ratelimit.Metrics

			// tokenRunner encapsulates the per-token runner/emit/metrics setup.
			// It can be invoked via attemptTokenFallback for automatic token rotation
			// on retryable auth/rate-limit errors.
			type tokenRunner struct {
				metrics *ratelimit.Metrics
				runner  prsync.Runner
				emit    func(string, map[string]any)
			}

			makeRunner := func(token string) (*tokenRunner, error) {
				m := ratelimit.NewMetrics()
				inner := prsync.NewDefaultRunner(nil, "", store, syncMaxPRs, token)
				guard := prsync.NewRateLimitGuard(budget, m, store, job.ID)
				r := prsync.NewRateLimitRunner(inner, guard, store, repo)
				emit := func(eventType string, payload map[string]any) {
					if eventType != "progress" {
						return
					}
					done, hasDone := payload["done"].(int)
					total, hasTotal := payload["total"].(int)
					if !hasDone || !hasTotal || total <= 0 {
						return
					}
					lastProgress.ProcessedPRs = done
					lastProgress.TotalPRs = total
					_ = store.UpdateSyncJobProgress(job.ID, lastProgress)

					if !showProgress && !watch {
						return
					}
					percent := (done * 100) / total
					elapsed := time.Since(startTime)
					eta := calculateETA(done, total, elapsed)
					fmt.Fprintf(cmd.OutOrStdout(), "\r[%3d%%] %d/%d PRs | Elapsed: %s | ETA: %s | Budget: %s",
						percent, done, total, elapsed.Round(time.Second), eta.Round(time.Second), budget.String())
				}
				return &tokenRunner{metrics: m, runner: r, emit: emit}, nil
			}

			// syncWithToken attempts sync using token fallback.
			syncWithToken := func(token string) error {
				tr, err := makeRunner(token)
				if err != nil {
					return err
				}
				lastMetrics = tr.metrics
				return tr.runner.Run(ctx, repo, tr.emit)
			}

			// Attempt sync with token fallback - on retryable auth/rate-limit errors,
			// fall through to the next available token
			pauseSync := func(resumeAt time.Time, reason string) error {
				log.Info("rate limit exhausted, pausing until resume", "resume_at", resumeAt, "reason", reason)
				return store.PauseSyncJob(job.ID, resumeAt, reason)
			}
			resumeAtFn := func() time.Time {
				return budget.ResetAt().Add(time.Duration(resetBuffer) * time.Second)
			}
			syncErr := waitAndRetrySync(ctx, wait, resumeAtFn, pauseSync, sleepUntilFn, func() error {
				return attemptTokenFallbackWithTrace(ctx, log, syncWithToken)
			})
			if showProgress || watch {
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if syncErr != nil {
				if strings.Contains(syncErr.Error(), "rate limit budget exhausted") {
					resumeAt := resumeAtFn()
					if wait {
						log.Info("sync waiting for resume", "repo", repo, "resume_at", resumeAt)
						return fmt.Errorf("unexpected exhausted budget while waiting for resume at %s: %w", resumeAt.Format(time.RFC3339), syncErr)
					}
					log.Info("sync paused", "repo", repo, "resume_at", resumeAt)
					fmt.Fprintf(cmd.OutOrStdout(), "Sync paused due to rate limits. Will resume after %s\n", resumeAt.Format(time.RFC3339))
					var metricsSnapshot ratelimit.MetricsSnapshot
					if lastMetrics != nil {
						metricsSnapshot = lastMetrics.Snapshot()
					}
					log.Info("sync exit", "repo", repo, "status", "paused", "job_id", job.ID)
					return writeJSON(cmd, syncCommandSummary{
						Started: true,
						Repo:    repo,
						Status:  "paused",
						JobID:   job.ID,
						Budget:  budget.String(),
						Metrics: &metricsSnapshot,
					})
				}
				log.Error("sync exit", "repo", repo, "status", "failed", "job_id", job.ID, "error", syncErr.Error())
				_ = store.MarkSyncJobFailed(job.ID, syncErr.Error())
				return syncErr
			}

			_ = store.MarkSyncJobComplete(job.ID, time.Now().UTC())

			if !watch {
				var metricsSnapshot ratelimit.MetricsSnapshot
				if lastMetrics != nil {
					metricsSnapshot = lastMetrics.Snapshot()
				}
				return writeJSON(cmd, syncCommandSummary{
					Started: true,
					Repo:    repo,
					Status:  "completed",
					JobID:   job.ID,
					Budget:  budget.String(),
					Metrics: &metricsSnapshot,
				})
			}

			if interval <= 0 {
				return fmt.Errorf("invalid value for --interval: must be greater than 0")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Watch mode enabled. Syncing %s every %s\n", repo, interval)
			fmt.Fprintf(cmd.OutOrStdout(), "Press Ctrl+C to stop.\n\n")

			scheduler := prsync.NewScheduler(store, prsync.WithCheckInterval(30*time.Second))
			if err := scheduler.Start(ctx); err != nil {
				return fmt.Errorf("start scheduler: %w", err)
			}
			defer scheduler.Stop()

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			const maxConsecutiveErrors = 5
			consecutiveErrors := 0
			backoff := interval

			for {
				select {
				case <-ctx.Done():
					fmt.Fprintln(cmd.OutOrStdout(), "\nSync watch stopped.")
					return nil
				case <-ticker.C:
					// Apply backoff if we had consecutive errors
					if consecutiveErrors > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "Backing off %v after %d consecutive errors...\n", backoff.Round(time.Second), consecutiveErrors)
						select {
						case <-ctx.Done():
							fmt.Fprintln(cmd.OutOrStdout(), "\nSync watch stopped.")
							return nil
						case <-time.After(backoff):
							// Double backoff for next potential error, max 30 minutes
							if backoff < 15*time.Minute {
								backoff = backoff * 2
							} else {
								backoff = 30 * time.Minute
							}
						}
					}

					if pausedJob, ok, _ := store.ResumeSyncJob(repo); ok {
						log.Info("resuming paused sync job", "job_id", pausedJob.ID)
						job = pausedJob
					} else {
						newJob, err := store.CreateSyncJob(repo)
						if err != nil {
							log.Error("failed to create sync job", "error", err)
							continue
						}
						job = newJob
					}

					startTime = time.Now()
					fmt.Fprintf(cmd.OutOrStdout(), "\n[%s] Starting sync... (watching)\n", time.Now().Format(time.RFC3339))

					if err := attemptTokenFallbackWithTrace(ctx, log, syncWithToken); err != nil {
						if strings.Contains(err.Error(), "rate limit budget exhausted") {
							resumeAt := budget.ResetAt().Add(time.Duration(resetBuffer) * time.Second)
							fmt.Fprintf(cmd.OutOrStdout(), "Sync paused. Will auto-resume after %s\n", resumeAt.Format(time.RFC3339))
							consecutiveErrors = 0
							backoff = interval
						} else {
							consecutiveErrors++
							log.Error("sync failed", "error", err)
							fmt.Fprintf(cmd.OutOrStdout(), "Sync error: %v (consecutive error %d/%d)\n", err, consecutiveErrors, maxConsecutiveErrors)
							if consecutiveErrors >= maxConsecutiveErrors {
								fmt.Fprintf(cmd.OutOrStdout(), "Too many consecutive errors. Stopping watch mode.\n")
								return fmt.Errorf("watch mode stopped after %d consecutive errors: %w", consecutiveErrors, err)
							}
						}
					} else {
						_ = store.MarkSyncJobComplete(job.ID, time.Now().UTC())
						fmt.Fprintf(cmd.OutOrStdout(), "Sync completed. Next sync in %s\n", interval)
						consecutiveErrors = 0
						backoff = interval
					}
				}
			}
		},
	}

	// Preflight subcommand
	preflightCmd := &cobra.Command{
		Use:   "preflight",
		Short: "Run preflight analysis to estimate sync requirements",
		Long: `Run preflight analysis to estimate sync requirements.

This command performs a dry-run analysis of the repository to estimate:
- Number of PRs to sync
- Estimated API requests needed
- Current rate limit status
- Estimated time to complete sync
- Whether sufficient rate limit budget is available

Examples:
  pratc sync preflight --repo=owner/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return runPreflightAnalysis(ctx, repo, rateLimit, reserveBuffer, resetBuffer, syncMaxPRs)
		},
	}
	preflightCmd.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	preflightCmd.MarkFlagRequired("repo")
	preflightCmd.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	preflightCmd.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	preflightCmd.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	preflightCmd.Flags().IntVar(&syncMaxPRs, "sync-max-prs", 0, "Max PRs to sync on this pass (0=no cap)")
	command.AddCommand(preflightCmd)

	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().BoolVar(&watch, "watch", false, "Run sync in watch mode with automatic resume")
	command.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Watch mode interval")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	command.Flags().BoolVar(&showProgress, "progress", false, "Show progress with ETA")
	command.Flags().IntVar(&syncMaxPRs, "sync-max-prs", 0, "Max PRs to sync on this pass (0=no cap)")
	command.Flags().BoolVar(&refreshSync, "refresh-sync", false, "Force a fresh sync even when a local snapshot already exists")
	command.Flags().BoolVar(&force, "force", false, "Override lock and force sync (use with caution)")
	command.Flags().BoolVar(&preflight, "preflight", false, "Run preflight analysis without syncing")
	command.Flags().BoolVar(&wait, "wait", false, "Automatically wait and resume when rate limit exhausted")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}
