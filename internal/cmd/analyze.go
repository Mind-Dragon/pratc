package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

var analyzeSyncMu sync.Mutex

const (
	analyzeSyncWaitTimeout  = 250 * time.Millisecond
	analyzeSyncPollInterval = 10 * time.Millisecond
)

type analyzeSyncInProgressResponse struct {
	Repo        string `json:"repo"`
	GeneratedAt string `json:"generatedAt"`
	SyncStatus  string `json:"sync_status"`
	JobID       string `json:"job_id,omitempty"`
	Message     string `json:"message"`
}

func analyzeSyncDBPath() string {
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	return dbPath
}

func checkAnalyzeSyncWarningData(repo string) (int, bool, bool) {
	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		return 0, false, true
	}
	defer store.Close()

	openPRCount, hasOpenPRCount := 0, false
	if prs, err := store.ListPRs(cache.PRFilter{Repo: repo}); err == nil {
		openPRCount = len(prs)
		hasOpenPRCount = true
	}

	lastSync, err := store.LastSync(repo)
	if err != nil || lastSync.IsZero() {
		return openPRCount, hasOpenPRCount, true
	}

	cacheTTL := time.Hour
	if ttlStr := strings.TrimSpace(os.Getenv("PRATC_CACHE_TTL")); ttlStr != "" {
		if parsed, parseErr := time.ParseDuration(ttlStr); parseErr == nil {
			cacheTTL = parsed
		}
	}

	if time.Since(lastSync) > cacheTTL {
		return openPRCount, hasOpenPRCount, true
	}

	return openPRCount, hasOpenPRCount, false
}

func buildAnalyzeSyncInProgressResponse(repo, jobID string) analyzeSyncInProgressResponse {
	return analyzeSyncInProgressResponse{
		Repo:        repo,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SyncStatus:  "in_progress",
		JobID:       jobID,
		Message:     "sync in progress",
	}
}

func buildAnalyzeSyncTimeoutResponse(repo, jobID string, timeout time.Duration) analyzeSyncInProgressResponse {
	return analyzeSyncInProgressResponse{
		Repo:        repo,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SyncStatus:  "in_progress",
		JobID:       jobID,
		Message:     fmt.Sprintf("sync still in progress after waiting %s", timeout),
	}
}

func analyzeSyncActive(repo string) bool {
	analyzeSyncMu.Lock()
	defer analyzeSyncMu.Unlock()

	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		return false
	}
	defer store.Close()

	_, ok, err := store.ResumeSyncJob(repo)
	return err == nil && ok
}

func waitForAnalyzeSyncCompletion(repo string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for analyzeSyncActive(repo) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(analyzeSyncPollInterval)
	}
	return true
}

func currentAnalyzeSyncJobID(repo string) (string, bool) {
	analyzeSyncMu.Lock()
	defer analyzeSyncMu.Unlock()

	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		return "", false
	}
	defer store.Close()

	job, ok, err := store.ResumeSyncJob(repo)
	if err != nil || !ok {
		return "", false
	}
	return job.ID, true
}

func startAnalyzeBackgroundSync(repo string) (string, error) {
	analyzeSyncMu.Lock()
	defer analyzeSyncMu.Unlock()

	dbPath := analyzeSyncDBPath()
	store, err := cache.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("open sync store: %w", err)
	}
	defer store.Close()

	if job, ok, err := store.ResumeSyncJob(repo); err == nil && ok {
		return job.ID, nil
	}

	job, err := store.CreateSyncJob(repo)
	if err != nil {
		return "", fmt.Errorf("create sync job: %w", err)
	}

	if _, err := github.ResolveToken(context.Background()); err != nil {
		return "", err
	}

	manager := newRepoSyncManager(dbPath, job.ID)
	if err := manager.Start(repo); err != nil {
		if markErr := store.MarkSyncJobFailed(job.ID, err.Error()); markErr != nil {
			return "", fmt.Errorf("start background sync: %w (mark failed job: %v)", err, markErr)
		}
		return "", fmt.Errorf("start background sync: %w", err)
	}

	return job.ID, nil
}

func RegisterAnalyzeCommand() {
	var repo string
	var format string
	var useCacheFirst bool
	var forceLive bool
	var force bool
	var maxPRs int
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int
	var enableReview bool

	command := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze pull requests for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(rateLimit),
				ratelimit.WithReserveBuffer(reserveBuffer),
				ratelimit.WithResetBuffer(resetBuffer),
			)
			log.Info("analyze budget initialized", "budget", budget.String())

			cfg := buildAnalyzeConfig(useCacheFirst, forceLive, maxPRs, enableReview)
			service := app.NewService(cfg)

			if shouldWarnAnalyzeSync(useCacheFirst, force, forceLive) {
				if openPRCount, hasOpenPRCount, shouldWarn := checkAnalyzeSyncWarningData(repo); shouldWarn {
					fmt.Fprint(os.Stderr, formatAnalyzeSyncWarning(repo, openPRCount, hasOpenPRCount))
					jobID, err := startAnalyzeBackgroundSync(repo)
					if err != nil {
						return err
					}
					return writeJSON(cmd, buildAnalyzeSyncInProgressResponse(repo, jobID))
				}
			}

			log.Info("starting analyze", "repo", repo, "budget", budget.String())
			response, err := service.Analyze(ctx, repo)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, response)
			case "text":
				return writeAnalyzeText(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for analyze", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&format, "format", "json", "Output format: json|text")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch")
	command.Flags().BoolVar(&forceLive, "force-live", false, "Skip cache check and force live fetch")
	command.Flags().BoolVar(&force, "force", false, "Compatibility flag to skip sync warning")
	command.Flags().IntVar(&maxPRs, "max-prs", -1, "Max PRs to analyze (-1=default 1000, 0=no cap)")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	command.Flags().BoolVar(&enableReview, "review", false, "Legacy compatibility flag; review output is always included in v1.3")
	_ = command.Flags().MarkHidden("force")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func buildAnalyzeConfig(useCacheFirst, forceLive bool, maxPRs int, includeReview bool) app.Config {
	_ = includeReview
	return app.Config{AllowLive: forceLive, UseCacheFirst: useCacheFirst, MaxPRs: maxPRs, IncludeReview: true}
}

func buildCacheFirstConfig(useCacheFirst bool) app.Config {
	return app.Config{UseCacheFirst: useCacheFirst}
}

func shouldWarnAnalyzeSync(useCacheFirst, force, forceLive bool) bool {
	return !force && !forceLive && useCacheFirst
}

func estimateAnalyzeSyncAPICalls(openPRCount int) int {
	if openPRCount <= 0 {
		return 1
	}
	return ((openPRCount + 99) / 100) + (openPRCount * 3)
}

func formatAnalyzeSyncWarning(repo string, openPRCount int, hasOpenPRCount bool) string {
	estimateLine := "   Estimated GitHub API calls: unavailable until sync data is available.\n"
	if hasOpenPRCount {
		estimateLine = fmt.Sprintf("   Estimated GitHub API calls: ~%d (based on %d open PRs)\n", estimateAnalyzeSyncAPICalls(openPRCount), openPRCount)
	}

	return fmt.Sprintf(`⚠️  No recent sync data found for %s
%s   Sync in progress — background sync job started.
   Recommended workflow:
   1) pratc workflow --repo=%s --progress
   2) pratc monitor --repo=%s
   Tip: use 'pratc sync --repo=%s --watch' for periodic refreshes.

`, repo, estimateLine, repo, repo, repo)
}

func reviewBucketLabel(category types.ReviewCategory) string {
	switch category {
	case types.ReviewCategoryMergeNow:
		return "Merge now"
	case types.ReviewCategoryMergeAfterFocusedReview:
		return "Merge after focused review"
	case types.ReviewCategoryDuplicateSuperseded:
		return "Duplicate / superseded"
	case types.ReviewCategoryProblematicQuarantine:
		return "Problematic / quarantine"
	case types.ReviewCategoryUnknownEscalate:
		return "Unknown / escalate"
	default:
		return "Unknown / escalate"
	}
}

func writeAnalyzeText(cmd *cobra.Command, response types.AnalysisResponse) error {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Repository: %s\n", response.Repo)
	fmt.Fprintf(out, "Generated:  %s\n\n", response.GeneratedAt)

	fmt.Fprintf(out, "Summary\n")
	fmt.Fprintf(out, "  Total PRs:        %d\n", response.Counts.TotalPRs)
	fmt.Fprintf(out, "  Clusters:         %d\n", response.Counts.ClusterCount)
	fmt.Fprintf(out, "  Duplicate Groups: %d\n", response.Counts.DuplicateGroups)
	fmt.Fprintf(out, "  Overlap Groups:   %d\n", response.Counts.OverlapGroups)
	fmt.Fprintf(out, "  Conflict Pairs:   %d\n", response.Counts.ConflictPairs)
	fmt.Fprintf(out, "  Stale PRs:        %d\n\n", response.Counts.StalePRs)

	if response.ReviewPayload != nil {
		review := response.ReviewPayload
		fmt.Fprintf(out, "Review Analysis\n")
		fmt.Fprintf(out, "  PRs Reviewed: %d / %d\n\n", review.ReviewedPRs, review.TotalPRs)

		if len(review.Buckets) > 0 {
			fmt.Fprintf(out, "  By Bucket:\n")
			for _, bucket := range review.Buckets {
				fmt.Fprintf(out, "    %-28s %d\n", bucket.Bucket+":", bucket.Count)
			}
			fmt.Fprintln(out)
		} else if len(review.Categories) > 0 {
			fmt.Fprintf(out, "  By Category:\n")
			for _, cat := range review.Categories {
				bucketLabel := reviewBucketLabel(types.ReviewCategory(cat.Category))
				fmt.Fprintf(out, "    %-28s %d\n", bucketLabel+":", cat.Count)
			}
			fmt.Fprintln(out)
		}

		if len(review.PriorityTiers) > 0 {
			fmt.Fprintf(out, "  By Priority:\n")
			for _, tier := range review.PriorityTiers {
				fmt.Fprintf(out, "    %-15s %d\n", tier.Tier+":", tier.Count)
			}
			fmt.Fprintln(out)
		}

		if len(review.Results) > 0 {
			fmt.Fprintf(out, "  Sample Findings:\n")
			sampleCount := 5
			if len(review.Results) < sampleCount {
				sampleCount = len(review.Results)
			}
			for i := 0; i < sampleCount; i++ {
				result := review.Results[i]
				bucketLabel := reviewBucketLabel(result.Category)
				fmt.Fprintf(out, "    PR #%d: %s (%.0f%% confidence)\n",
					response.PRs[i].Number, bucketLabel, result.Confidence*100)
			}
			if len(review.Results) > sampleCount {
				fmt.Fprintf(out, "    ... and %d more\n", len(review.Results)-sampleCount)
			}
		}
	}

	return nil
}
