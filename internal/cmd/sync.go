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
	"github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	prsync "github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/spf13/cobra"
)

func calculateETA(done, total int, elapsed time.Duration) time.Duration {
	if done <= 0 || total <= done {
		return 0
	}
	rate := float64(done) / elapsed.Seconds()
	remaining := total - done
	return time.Duration(float64(remaining)/rate) * time.Second
}

func RegisterSyncCommand() {
	var repo string
	var watch bool
	var interval time.Duration
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int
	var showProgress bool

	command := &cobra.Command{
		Use:   "sync",
		Short: "Sync repository metadata and refs",
		Long: `Sync repository metadata and refs with rate-limit-aware scheduling.

The sync command fetches PR metadata and git refs from GitHub, respecting
rate limits and automatically pausing/resuming when budget is exhausted.

Examples:
  # One-time sync
  pratc sync --repo=owner/repo

  # Watch mode with progress display
  pratc sync --repo=owner/repo --watch --interval=5m --progress

  # Custom rate limit configuration
  pratc sync --repo=owner/repo --rate-limit=5000 --reserve-buffer=300`,
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

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

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(rateLimit),
				ratelimit.WithReserveBuffer(reserveBuffer),
				ratelimit.WithResetBuffer(resetBuffer),
			)

			dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
			if dbPath == "" {
				home, _ := os.UserHomeDir()
				dbPath = filepath.Join(home, ".pratc", "pratc.db")
			}
			store, err := cache.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open cache store: %w", err)
			}
			defer store.Close()

			if _, err := github.ResolveToken(ctx); err != nil {
				return err
			}
			metrics := ratelimit.NewMetrics()
			innerRunner := prsync.NewDefaultRunner(nil, "", store)

			log.Info("starting sync", "repo", repo, "budget", budget.String())

			job, err := store.CreateSyncJob(repo)
			if err != nil {
				return fmt.Errorf("create sync job: %w", err)
			}

			guard := prsync.NewRateLimitGuard(budget, metrics, store, job.ID)
			runner := prsync.NewRateLimitRunner(innerRunner, guard, store, repo)

			startTime := time.Now()
			var lastProgress cache.SyncProgress
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

			syncErr := runner.Run(ctx, repo, emit)

			if showProgress || watch {
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if syncErr != nil {
				if strings.Contains(syncErr.Error(), "rate limit budget exhausted") {
					resumeAt := budget.ResetAt().Add(time.Duration(resetBuffer) * time.Second)
					fmt.Fprintf(cmd.OutOrStdout(), "Sync paused due to rate limits. Will resume after %s\n", resumeAt.Format(time.RFC3339))
					return writeJSON(cmd, map[string]any{
						"started":   true,
						"repo":      repo,
						"status":    "paused",
						"job_id":    job.ID,
						"resume_at": resumeAt.Format(time.RFC3339),
						"budget":    budget.String(),
					})
				}
				_ = store.MarkSyncJobFailed(job.ID, syncErr.Error())
				return syncErr
			}

			_ = store.MarkSyncJobComplete(job.ID, time.Now().UTC())

			if !watch {
				return writeJSON(cmd, map[string]any{
					"started": true,
					"repo":    repo,
					"status":  "completed",
					"job_id":  job.ID,
					"budget":  budget.String(),
					"metrics": metrics.Snapshot(),
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

			for {
				select {
				case <-ctx.Done():
					fmt.Fprintln(cmd.OutOrStdout(), "\nSync watch stopped.")
					return nil
				case <-ticker.C:
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
					fmt.Fprintf(cmd.OutOrStdout(), "\n[%s] Starting sync...\n", time.Now().Format(time.RFC3339))

					if err := runner.Run(ctx, repo, emit); err != nil {
						if strings.Contains(err.Error(), "rate limit budget exhausted") {
							resumeAt := budget.ResetAt().Add(time.Duration(resetBuffer) * time.Second)
							fmt.Fprintf(cmd.OutOrStdout(), "Sync paused. Will auto-resume after %s\n", resumeAt.Format(time.RFC3339))
						} else {
							log.Error("sync failed", "error", err)
							fmt.Fprintf(cmd.OutOrStdout(), "Sync error: %v\n", err)
						}
					} else {
						_ = store.MarkSyncJobComplete(job.ID, time.Now().UTC())
						fmt.Fprintf(cmd.OutOrStdout(), "Sync completed. Next sync in %s\n", interval)
					}
				}
			}
		},
	}

	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().BoolVar(&watch, "watch", false, "Run sync in watch mode with automatic resume")
	command.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Watch mode interval")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	command.Flags().BoolVar(&showProgress, "progress", false, "Show progress with ETA")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}
