package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/formula"
	gh "github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
	prsync "github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

const workflowRateLimitPauseReason = "rate limit budget exhausted"

type workflowSyncSummary struct {
	Repo        string                    `json:"repo"`
	GeneratedAt string                    `json:"generatedAt"`
	Status      string                    `json:"status"`
	JobID       string                    `json:"job_id"`
	Budget      string                    `json:"budget,omitempty"`
	Metrics     ratelimit.MetricsSnapshot `json:"metrics,omitempty"`
	ResumeAt    string                    `json:"resume_at,omitempty"`
}

func RegisterWorkflowCommand() {
	var repo string
	var outDir string
	var progress bool
	var useCacheFirst bool
	var forceLive bool
	var maxPRs int
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int
	var syncMaxPRs int
	var refreshSync bool
	var force bool

	command := &cobra.Command{
		Use:   "workflow",
		Short: "Run sync to completion, then analyze",
		Long: `Run a full prATC workflow for a repository.

The workflow command performs a blocking sync run, automatically waits through
 rate-limit pauses, then runs analysis once fresh sync data is available.

The workflow is service-friendly: it does not require a TTY. Use 'pratc monitor
--repo=owner/repo' in another terminal to watch live dashboard updates while the
workflow is running.

Examples:
  # Run the full workflow with progress output
  pratc workflow --repo=owner/repo --progress

  # Write artifacts to a custom directory
  pratc workflow --repo=owner/repo --out-dir=./projects/owner_repo/runs/latest

  # Use a higher max PR limit for large repos
  pratc workflow --repo=owner/repo --max-prs=5000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			if strings.TrimSpace(repo) == "" {
				return fmt.Errorf("repo is required")
			}

			repo = types.NormalizeRepoName(repo)

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

			token, err := gh.ResolveToken(ctx)
			if err != nil {
				return err
			}

			store, err := openWorkflowCacheStore()
			if err != nil {
				return err
			}
			defer store.Close()

			resolvedOutDir := strings.TrimSpace(outDir)
			if resolvedOutDir == "" {
				resolvedOutDir = defaultWorkflowOutDir(repo)
			}
			if err := os.MkdirAll(resolvedOutDir, 0o755); err != nil {
				return fmt.Errorf("create workflow output directory: %w", err)
			}
			if err := writeProjectManifest(resolvedOutDir, repo); err != nil {
				return err
			}
			log.Info("starting workflow", "repo", repo, "out_dir", resolvedOutDir)
			fmt.Fprintf(cmd.ErrOrStderr(), "Workflow output: %s\n", resolvedOutDir)
			if progress {
				fmt.Fprintf(cmd.ErrOrStderr(), "Use 'pratc monitor --repo=%s' in another terminal to watch live updates.\n", repo)
			}

			var syncSummary workflowSyncSummary
			var reused bool
			if !refreshSync {
				syncSummary, reused, err = reuseCachedWorkflowSyncSummary(store, repo)
				if err != nil {
					return err
				}
			}
			if reused {
				log.Info("reusing local sync snapshot", "repo", repo)
			}
			if !reused {
				syncSummary, err = runWorkflowSync(ctx, cmd, store, repo, token, progress, rateLimit, reserveBuffer, resetBuffer, syncMaxPRs)
				if err != nil {
					return err
				}
			}
			if err := writeWorkflowJSON(filepath.Join(resolvedOutDir, "sync.json"), syncSummary); err != nil {
				return err
			}

			service := app.NewService(app.Config{
				AllowLive:     forceLive,
				UseCacheFirst: useCacheFirst,
				MaxPRs:        maxPRs,
				IncludeReview: true,
				OnAnalyzeProgress: func(phase string, done, total int) {
					fmt.Fprintf(cmd.ErrOrStderr(), "\r[analyze %s] %d/%d ", phase, done, total)
					if phase == "done" {
						fmt.Fprintln(cmd.ErrOrStderr())
					}
				},
			})
			analyzeCtx := logger.ContextWithRequestID(ctx, requestID)
			analyzeLog := logger.FromContext(analyzeCtx)
			analyzeLog.Info("starting workflow analyze", "repo", repo)

			response, err := service.Analyze(analyzeCtx, repo)
			if err != nil {
				return err
			}

			if err := writeWorkflowJSON(filepath.Join(resolvedOutDir, "analyze.json"), response); err != nil {
				return err
			}
			if err := writeWorkflowJSON(filepath.Join(resolvedOutDir, "step-2-analyze.json"), response); err != nil {
				return err
			}

			analyzeLog.Info("starting workflow cluster", "repo", repo)
			clusterResponse, err := service.Cluster(analyzeCtx, repo)
			if err != nil {
				return err
			}
			if err := writeWorkflowJSON(filepath.Join(resolvedOutDir, "step-3-cluster.json"), clusterResponse); err != nil {
				return err
			}

			analyzeLog.Info("starting workflow graph", "repo", repo)
			graphResponse, err := service.Graph(analyzeCtx, repo)
			if err != nil {
				return err
			}
			if err := writeWorkflowJSON(filepath.Join(resolvedOutDir, "step-4-graph.json"), graphResponse); err != nil {
				return err
			}

			analyzeLog.Info("starting workflow plan", "repo", repo)
			planResponse, err := service.Plan(analyzeCtx, repo, 20, formula.ModeCombination)
			if err != nil {
				return err
			}
			if err := writeWorkflowJSON(filepath.Join(resolvedOutDir, "step-5-plan.json"), planResponse); err != nil {
				return err
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Workflow complete for %s\n", repo)
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(response)
		},
	}

	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&outDir, "out-dir", "", "Directory for workflow artifacts (defaults to projects/<repo>/runs/<timestamp>)")
	command.Flags().BoolVar(&progress, "progress", true, "Show sync progress while the workflow runs")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch during analyze")
	command.Flags().BoolVar(&forceLive, "force-live", false, "Skip cache check and force live fetch during analyze")
	command.Flags().IntVar(&maxPRs, "max-prs", 0, "Max PRs to analyze (0=no cap)")
	command.Flags().IntVar(&syncMaxPRs, "sync-max-prs", 0, "Max PRs to sync on the initial pass (0=no cap)")
	command.Flags().BoolVar(&refreshSync, "refresh-sync", false, "Force a fresh sync even when a local snapshot already exists")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	command.Flags().BoolVar(&force, "force", false, "Override lock and force workflow (use with caution)")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func defaultWorkflowOutDir(repo string) string {
	return projectRunDir(repo, time.Now())
}

func openWorkflowCacheStore() (*cache.Store, error) {
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cache store: %w", err)
	}
	return store, nil
}

func runWorkflowSync(ctx context.Context, cmd *cobra.Command, store *cache.Store, repo string, token string, progress bool, rateLimit, reserveBuffer, resetBuffer, syncMaxPRs int) (workflowSyncSummary, error) {
	job, err := loadWorkflowJob(store, repo)
	if err != nil {
		return workflowSyncSummary{}, err
	}

	for {
		budget := ratelimit.NewBudgetManager(
			ratelimit.WithRateLimit(rateLimit),
			ratelimit.WithReserveBuffer(reserveBuffer),
			ratelimit.WithResetBuffer(resetBuffer),
		)
		metrics := ratelimit.NewMetrics()
		innerRunner := prsync.NewDefaultRunner(nil, job.ID, store, syncMaxPRs, token)
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
			lastProgress.SnapshotCeiling = total
			_ = store.UpdateSyncJobProgress(job.ID, lastProgress)

			if !progress {
				return
			}
			percent := (done * 100) / total
			elapsed := time.Since(startTime)
			eta := calculateETA(done, total, elapsed)
			fmt.Fprintf(cmd.ErrOrStderr(), "\r[%3d%%] %d/%d PRs | Elapsed: %s | ETA: %s | Budget: %s",
				percent, done, total, elapsed.Round(time.Second), eta.Round(time.Second), budget.String())
		}

		syncErr := runner.Run(ctx, repo, emit)
		if progress {
			fmt.Fprintln(cmd.ErrOrStderr())
		}

		if syncErr == nil {
			if err := store.MarkSyncJobComplete(job.ID, time.Now().UTC()); err != nil {
				return workflowSyncSummary{}, err
			}
			return workflowSyncSummary{
				Repo:        repo,
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Status:      string(cache.SyncJobStatusCompleted),
				JobID:       job.ID,
				Budget:      budget.String(),
				Metrics:     metrics.Snapshot(),
			}, nil
		}

		if isWorkflowRateLimitPause(syncErr) {
			pausedJob, err := store.GetPausedSyncJobByRepo(repo)
			if err != nil {
				return workflowSyncSummary{}, fmt.Errorf("load paused sync job: %w", err)
			}
			job = pausedJob
			resumeAt := job.Progress.ScheduledResumeAt
			if resumeAt.IsZero() {
				resumeAt = budget.ResetAt().Add(time.Duration(resetBuffer) * time.Second)
			}
			pauseNotice, noticeErr := buildWorkflowRateLimitPauseNotice(store, repo, job)
			if noticeErr != nil {
				return workflowSyncSummary{}, noticeErr
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "%s. Waiting until %s\n", pauseNotice, resumeAt.Format(time.RFC3339))
			if err := sleepUntil(ctx, resumeAt); err != nil {
				if errors.Is(err, context.Canceled) {
					return workflowSyncSummary{}, fmt.Errorf("%s; analyze will continue on the cached snapshot now: %w", pauseNotice, err)
				}
				return workflowSyncSummary{}, err
			}
			job, err = cache.ResumeSyncJob(store, repo)
			if err != nil {
				return workflowSyncSummary{}, err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Resumed sync job %s\n", job.ID)
			continue
		}

		if markErr := store.MarkSyncJobFailed(job.ID, syncErr.Error()); markErr != nil {
			return workflowSyncSummary{}, fmt.Errorf("%w (mark failed job: %v)", syncErr, markErr)
		}
		return workflowSyncSummary{}, syncErr
	}
}

func loadWorkflowJob(store *cache.Store, repo string) (cache.SyncJob, error) {
	if job, ok, err := store.ResumeSyncJob(repo); err == nil && ok {
		return job, nil
	} else if err != nil {
		return cache.SyncJob{}, err
	}

	if job, err := cache.ResumeSyncJob(store, repo); err == nil {
		return job, nil
	} else if !strings.Contains(err.Error(), "no paused sync job found") {
		return cache.SyncJob{}, err
	}

	job, err := store.CreateSyncJob(repo)
	if err != nil {
		return cache.SyncJob{}, fmt.Errorf("create sync job: %w", err)
	}
	return job, nil
}

func reuseCachedWorkflowSyncSummary(store *cache.Store, repo string) (workflowSyncSummary, bool, error) {
	if store == nil {
		return workflowSyncSummary{}, false, nil
	}
	if job, ok, err := store.ResumeSyncJob(repo); err != nil {
		return workflowSyncSummary{}, false, fmt.Errorf("check active sync job: %w", err)
	} else if ok && job.ID != "" {
		return workflowSyncSummary{}, false, nil
	}

	lastSync, err := store.LastSync(repo)
	if err != nil {
		return workflowSyncSummary{}, false, fmt.Errorf("load last sync: %w", err)
	}
	if lastSync.IsZero() {
		return workflowSyncSummary{}, false, nil
	}

	prs, err := store.ListPRs(cache.PRFilter{Repo: repo})
	if err != nil {
		return workflowSyncSummary{}, false, fmt.Errorf("list cached prs: %w", err)
	}
	if len(prs) == 0 {
		return workflowSyncSummary{}, false, nil
	}

	progress, ok, err := store.GetSyncProgress(repo)
	if err != nil {
		return workflowSyncSummary{}, false, fmt.Errorf("load sync progress: %w", err)
	}
	if ok && progress.ProcessedPRs > 0 {
		lastSync = lastSync.UTC()
	}

	return workflowSyncSummary{
		Repo:        repo,
		GeneratedAt: lastSync.UTC().Format(time.RFC3339),
		Status:      string(cache.SyncJobStatusCompleted),
		JobID:       "",
		Budget:      "local-first cache reuse",
	}, true, nil
}

func isWorkflowRateLimitPause(err error) bool {
	return err != nil && strings.Contains(err.Error(), workflowRateLimitPauseReason)
}

func buildWorkflowRateLimitPauseNotice(store *cache.Store, repo string, job cache.SyncJob) (string, error) {
	cachedPRCount := 0
	if store != nil {
		prs, err := store.ListPRs(cache.PRFilter{Repo: repo})
		if err != nil {
			return "", fmt.Errorf("list cached prs: %w", err)
		}
		cachedPRCount = len(prs)
	}

	processed := job.Progress.ProcessedPRs
	if processed <= 0 && cachedPRCount > 0 {
		processed = cachedPRCount
	}

	total := job.Progress.TotalPRs
	if total > 0 && processed > 0 {
		remaining := total - processed
		if remaining < 0 {
			remaining = 0
		}
		return fmt.Sprintf("Sync paused due to rate limits after pulling %d/%d PRs (%d remain). Analyze will continue on the cached %d-PR snapshot now.", processed, total, remaining, cachedPRCount), nil
	}
	if processed > 0 {
		return fmt.Sprintf("Sync paused due to rate limits after pulling %d PRs. Analyze will continue on the cached %d-PR snapshot now.", processed, cachedPRCount), nil
	}
	if cachedPRCount > 0 {
		return fmt.Sprintf("Sync paused due to rate limits. Analyze will continue on the cached %d-PR snapshot now.", cachedPRCount), nil
	}
	return "Sync paused due to rate limits. Analyze will continue on the cached snapshot now.", nil
}

func sleepUntil(ctx context.Context, resumeAt time.Time) error {
	delay := time.Until(resumeAt)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func writeWorkflowJSON(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow artifact: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write workflow artifact %s: %w", path, err)
	}
	return nil
}
