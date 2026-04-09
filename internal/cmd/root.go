package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/audit"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/monitor/data"
	"github.com/jeffersonnunn/pratc/internal/monitor/server"
	"github.com/jeffersonnunn/pratc/internal/planning"
	"github.com/jeffersonnunn/pratc/internal/repo"
	"github.com/jeffersonnunn/pratc/internal/report"
	"github.com/jeffersonnunn/pratc/internal/settings"
	prsync "github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var rootCmd = &cobra.Command{
	Use:   "pratc",
	Short: "PR Air Traffic Control",
	Long:  "prATC is a CLI for pull request analysis, clustering, graphing, planning, and API serving.",
}

const (
	analyzeSyncWaitTimeout  = 250 * time.Millisecond
	analyzeSyncPollInterval = 10 * time.Millisecond
)

var analyzeSyncMu sync.Mutex

func ExecuteContext(ctx context.Context) {
	fmt.Fprintf(os.Stderr, "Github Pull Request Air Traffic Control v%s %s by Jefferson Nunn\n", version.Version, version.BuildDate)

	// Show config locations
	settingsDB := os.Getenv("PRATC_SETTINGS_DB")
	if settingsDB == "" {
		settingsDB = "./pratc-settings.db (default)"
	}
	cacheDB := os.Getenv("PRATC_DB_PATH")
	if cacheDB == "" {
		home, _ := os.UserHomeDir()
		cacheDB = filepath.Join(home, ".pratc", "pratc.db")
	}
	fmt.Fprintf(os.Stderr, "Using Config from: settings=%s | cache=%s\n", settingsDB, cacheDB)

	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return
	}

	log := logger.New("cli")
	log.Error("command failed", "err", err)
	if isInvalidArgumentError(err) {
		os.Exit(2)
	}

	os.Exit(1)
}

func isInvalidArgumentError(err error) bool {
	message := err.Error()
	patterns := []string{
		"required flag",
		"unknown command",
		"unknown flag",
		"unknown shorthand flag",
		"accepts",
		"invalid value for",
		"invalid argument",
		"invalid format",
		"invalid mode",
	}

	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}

	return false
}

func calculateETA(done, total int, elapsed time.Duration) time.Duration {
	if done <= 0 || total <= done {
		return 0
	}
	rate := float64(done) / elapsed.Seconds()
	remaining := total - done
	return time.Duration(float64(remaining)/rate) * time.Second
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

type analyzeSyncInProgressResponse struct {
	Repo        string `json:"repo"`
	GeneratedAt string `json:"generatedAt"`
	SyncStatus  string `json:"sync_status"`
	JobID       string `json:"job_id,omitempty"`
	Message     string `json:"message"`
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
				return writeAnalyzeText(cmd, response, enableReview)
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
	command.Flags().BoolVar(&enableReview, "review", false, "Run review analysis and include review categories in output")
	_ = command.Flags().MarkHidden("force")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func buildAnalyzeConfig(useCacheFirst, forceLive bool, maxPRs int, includeReview bool) app.Config {
	return app.Config{AllowLive: forceLive, UseCacheFirst: useCacheFirst, MaxPRs: maxPRs, IncludeReview: includeReview}
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
   1) pratc sync --repo=%s
   2) pratc analyze --repo=%s
   Tip: use 'pratc sync --repo=%s --watch' to monitor progress.

`, repo, estimateLine, repo, repo, repo)
}

func RegisterClusterCommand() {
	var repo string
	var format string
	var useCacheFirst bool

	command := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster pull requests for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			service := app.NewService(buildCacheFirstConfig(useCacheFirst))
			log.Info("starting cluster", "repo", repo)
			response, err := service.Cluster(ctx, repo)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for cluster", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&format, "format", "json", "Output format: json")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterGraphCommand() {
	var repo string
	var format string
	var useCacheFirst bool
	var maxPRs int
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int

	command := &cobra.Command{
		Use:   "graph",
		Short: "Render a dependency graph for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(rateLimit),
				ratelimit.WithReserveBuffer(reserveBuffer),
				ratelimit.WithResetBuffer(resetBuffer),
			)
			log.Info("graph budget initialized", "budget", budget.String())

			cfg := app.Config{UseCacheFirst: useCacheFirst}
			if maxPRs >= 0 {
				cfg.MaxPRs = maxPRs
			}
			service := app.NewService(cfg)
			log.Info("starting graph", "repo", repo, "budget", budget.String())
			response, err := service.Graph(ctx, repo)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "dot", "":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), response.DOT)
				return err
			case "json":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for graph", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&format, "format", "dot", "Output format: dot|json")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch")
	command.Flags().IntVar(&maxPRs, "max-prs", -1, "Max PRs to graph (-1=default, 0=no cap)")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterReportCommand() {
	var repo string
	var inputDir string
	var output string
	var format string

	command := &cobra.Command{
		Use:   "report",
		Short: "Generate a PDF report from analysis artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			log.Info("starting report", "repo", repo, "input_dir", inputDir, "output", output, "format", format)

			// Validate input directory and required files
			if err := report.ValidateInputDir(inputDir); err != nil {
				log.Warn("input directory validation failed", "error", err)
				return fmt.Errorf("invalid input directory: %w", err)
			}

			missingFiles := report.ValidateRequiredFiles(inputDir)
			if len(missingFiles) > 0 {
				log.Warn("missing required files", "missing", missingFiles)
				return fmt.Errorf("missing required input files: %v", missingFiles)
			}

			// Create PDF exporter
			exporter := report.NewPDFExporter(repo, "prATC Scalability Report")

			// Add cover section
			cover := &report.CoverSection{
				Repo:        repo,
				Title:       "Scalability Analysis Report",
				GeneratedAt: time.Now(),
				Summary:     "This report provides an overview of pull request metrics, clustering analysis, and merge recommendations.",
			}
			exporter.AddSection(cover)

			// Add executive summary section
			summary, err := report.LoadSummarySection(inputDir, repo)
			if err != nil {
				log.Warn("failed to load summary section", "error", err)
				return fmt.Errorf("failed to load summary data: %w", err)
			}
			exporter.AddSection(summary)

			metrics, err := report.LoadMetricsSection(inputDir, repo)
			if err != nil {
				log.Warn("failed to load metrics section", "error", err)
				return fmt.Errorf("failed to load metrics data: %w", err)
			}
			exporter.AddSection(metrics)

			// Add cluster analysis section
			clusterSection, err := report.LoadClusterSection(inputDir, repo)
			if err != nil {
				log.Warn("failed to load cluster section", "error", err)
				return fmt.Errorf("failed to load cluster data: %w", err)
			}
			exporter.AddSection(clusterSection)

			// Add graph section with real structure visualization
			graphSection, err := report.LoadGraphSection(inputDir, repo)
			if err != nil {
				log.Warn("failed to load graph section", "error", err)
				return fmt.Errorf("failed to load graph data: %w", err)
			}
			exporter.AddSection(graphSection)

			planSection, err := report.LoadPlanSection(inputDir, repo)
			if err != nil {
				log.Warn("failed to load plan section", "error", err)
				return fmt.Errorf("failed to load plan data: %w", err)
			}
			exporter.AddSection(planSection)

			// Generate PDF bytes
			pdfBytes, err := exporter.Export()
			if err != nil {
				return fmt.Errorf("failed to generate PDF: %w", err)
			}

			// Write to output file
			if err := os.WriteFile(output, pdfBytes, 0644); err != nil {
				return fmt.Errorf("failed to write PDF file: %w", err)
			}

			log.Info("report generated successfully", "output", output, "size_bytes", len(pdfBytes))
			fmt.Fprintf(cmd.OutOrStdout(), "PDF report generated: %s (%d bytes)\n", output, len(pdfBytes))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&inputDir, "input-dir", "", "Input directory containing JSON artifacts")
	command.Flags().StringVar(&output, "output", "", "Output PDF file path")
	command.Flags().StringVar(&format, "format", "pdf", "Output format: pdf|json")
	_ = command.MarkFlagRequired("repo")
	_ = command.MarkFlagRequired("input-dir")
	_ = command.MarkFlagRequired("output")
	rootCmd.AddCommand(command)
}

func RegisterPlanCommand() {
	var repo string
	var target int
	var mode string
	var format string
	var dryRun bool
	var includeBots bool
	var useCacheFirst bool
	var maxPRs int
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int

	command := &cobra.Command{
		Use:   "plan",
		Short: "Generate a merge plan for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(rateLimit),
				ratelimit.WithReserveBuffer(reserveBuffer),
				ratelimit.WithResetBuffer(resetBuffer),
			)
			log.Info("plan budget initialized", "budget", budget.String())

			selectedMode, err := parseMode(mode)
			if err != nil {
				return err
			}

			if !cmd.Flags().Changed("dry-run") {
				dryRun = true
			}

			cfg := buildCacheFirstConfig(useCacheFirst)
			if maxPRs > 0 {
				cfg.MaxPRs = maxPRs
			} else if maxPRs == -1 {
				cfg.MaxPRs = 1000
			}
			service := app.NewService(cfg)
			log.Info("starting plan", "repo", repo, "target", target, "mode", selectedMode, "budget", budget.String())
			response, err := service.Plan(ctx, repo, target, selectedMode)
			if err != nil {
				return err
			}

			details := fmt.Sprintf("target=%d mode=%s dry_run=%t include_bots=%t", target, selectedMode, dryRun, includeBots)
			logAuditEntry("plan", repo, details)

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for plan", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().IntVar(&target, "target", 20, "Number of PRs to include in merge plan")
	command.Flags().StringVar(&mode, "mode", string(formula.ModeCombination), "Formula mode: combination|permutation|with_replacement")
	command.Flags().StringVar(&format, "format", "json", "Output format: json")
	command.Flags().BoolVar(&dryRun, "dry-run", true, "Plan only; do not execute (always true by default)")
	command.Flags().BoolVar(&includeBots, "include-bots", false, "Include bot PRs in merge plan (default excludes bots)")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch")
	command.Flags().IntVar(&maxPRs, "max-prs", -1, "Max PRs to consider (-1=default 1000, 0=no cap)")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterServeCommand() {
	var port int
	var repo string
	var useCacheFirst bool

	command := &cobra.Command{
		Use:   "serve",
		Short: "Serve the prATC API",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("starting server", "port", port, "repo", repo)
			return runServer(ctx, port, repo, useCacheFirst)
		},
	}
	command.Flags().IntVar(&port, "port", 8080, "Port to bind the API server to")
	command.Flags().StringVar(&repo, "repo", "", "Optional default repository for API routes")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", false, "Check cache before live fetch for analyze/cluster endpoints")
	rootCmd.AddCommand(command)
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

func writeJSON(cmd *cobra.Command, payload any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func writeAnalyzeText(cmd *cobra.Command, response types.AnalysisResponse, includeReview bool) error {
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

	if includeReview && response.ReviewPayload != nil {
		review := response.ReviewPayload
		fmt.Fprintf(out, "Review Analysis\n")
		fmt.Fprintf(out, "  PRs Reviewed: %d / %d\n\n", review.ReviewedPRs, review.TotalPRs)

		if len(review.Categories) > 0 {
			fmt.Fprintf(out, "  By Category:\n")
			for _, cat := range review.Categories {
				fmt.Fprintf(out, "    %-15s %d\n", cat.Category+":", cat.Count)
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
				fmt.Fprintf(out, "    PR #%d: %s (%.0f%% confidence)\n",
					response.PRs[i].Number, result.Category, result.Confidence*100)
			}
			if len(review.Results) > sampleCount {
				fmt.Fprintf(out, "    ... and %d more\n", len(review.Results)-sampleCount)
			}
		}
	}

	return nil
}

func parseMode(raw string) (formula.Mode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(formula.ModeCombination):
		return formula.ModeCombination, nil
	case string(formula.ModePermutation):
		return formula.ModePermutation, nil
	case string(formula.ModeWithReplacement):
		return formula.ModeWithReplacement, nil
	default:
		return "", fmt.Errorf("invalid mode %q", raw)
	}
}

func runServer(ctx context.Context, port int, defaultRepo string, useCacheFirst bool) error {
	service := app.NewService(buildCacheFirstConfig(useCacheFirst))
	settingsStore, err := openSettingsStore()
	if err != nil {
		return err
	}
	defer settingsStore.Close()

	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	cacheStore, err := cache.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open cache store: %w", err)
	}
	defer cacheStore.Close()

	repoSync := newRepoSyncManager("", "")

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeHTTPJSON(w, http.StatusOK, service.Health())
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeHTTPJSON(w, http.StatusOK, service.Health())
	})
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		handleSettings(w, r, settingsStore)
	})
	mux.HandleFunc("/api/settings/export", func(w http.ResponseWriter, r *http.Request) {
		handleExportSettings(w, r, settingsStore)
	})
	mux.HandleFunc("/api/settings/import", func(w http.ResponseWriter, r *http.Request) {
		handleImportSettings(w, r, settingsStore)
	})

	mux.HandleFunc("/api/sync/jobs", handleListSyncJobs)
	mux.HandleFunc("/api/sync/jobs/paused", handleListPausedSyncJobs)
	mux.HandleFunc("/api/sync/events", handleSyncEvents(cacheStore))

	// Monitor WebSocket endpoint
	githubClient := github.NewClient(github.Config{
		Token:           os.Getenv("GITHUB_TOKEN"),
		ReserveRequests: 200,
	})
	monitorBroadcaster := data.NewBroadcaster(
		data.NewStore(cacheStore),
		data.NewRateLimitFetcher(githubClient),
		data.NewTimelineAggregator(cacheStore),
	)
	go monitorBroadcaster.Start(ctx)
	defer monitorBroadcaster.Stop()

	websocketServer := server.NewWebSocketServer(monitorBroadcaster)
	mux.HandleFunc("/monitor/stream", websocketServer.ServeHTTP)

	mux.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		handleAnalyze(w, r, service, repoFromQuery(r, defaultRepo))
	})
	mux.HandleFunc("/cluster", func(w http.ResponseWriter, r *http.Request) {
		handleCluster(w, r, service, repoFromQuery(r, defaultRepo))
	})
	mux.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		handleGraph(w, r, service, repoFromQuery(r, defaultRepo))
	})
	mux.HandleFunc("/plan", func(w http.ResponseWriter, r *http.Request) {
		handlePlan(w, r, service, repoFromQuery(r, defaultRepo))
	})

	mux.HandleFunc("/api/repos/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/repos/") && strings.Contains(path, "/plan/omni") {
			parts := strings.Split(strings.TrimPrefix(path, "/api/repos/"), "/")
			if len(parts) >= 3 {
				owner := parts[0]
				repoName := parts[1]
				fullRepo := owner + "/" + repoName
				handlePlanOmni(w, r, service, fullRepo)
				return
			}
		}
		handleRepoAction(w, r, service, repoSync)
	})

	server := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: corsMiddleware(mux)}

	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type settingsStore interface {
	Get(ctx context.Context, repo string) (map[string]any, error)
	Set(ctx context.Context, scope, repo, key string, value any) error
	Delete(ctx context.Context, scope, repo, key string) error
	ValidateSet(ctx context.Context, scope, repo, key string, value any) error
	ExportYAML(ctx context.Context, scope, repo string) ([]byte, error)
	ImportYAML(ctx context.Context, scope, repo string, content []byte) error
}

type repoSyncAPI interface {
	Start(repo string) error
	Stream(repo string, w http.ResponseWriter, r *http.Request)
}

var newRepoSyncManager = func(jobDBPath, jobID string) *prsync.Manager {
	var jobRecorder prsync.JobRecorder
	if strings.TrimSpace(jobDBPath) != "" {
		jobRecorder = prsync.NewDBJobRecorder(jobDBPath)
	}
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	cacheStore, _ := cache.Open(dbPath)
	return prsync.NewManager(prsync.NewDefaultRunner(jobRecorder, jobID, cacheStore))
}

func getSyncStatus(repo string) map[string]any {
	status := map[string]any{
		"repo":             repo,
		"last_sync":        time.Time{},
		"pr_count":         0,
		"status":           "never",
		"in_progress":      false,
		"progress_percent": 0,
	}

	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(dbPath)
	if err != nil {
		status["error"] = "cache unavailable"
		return status
	}
	defer store.Close()

	if prs, err := store.ListPRs(cache.PRFilter{Repo: repo}); err == nil {
		status["pr_count"] = len(prs)
	}

	if job, ok, err := store.ResumeSyncJob(repo); err == nil && ok {
		status["status"] = "in_progress"
		status["in_progress"] = true
		status["last_sync"] = job.LastSyncAt
		if job.Progress.TotalPRs > 0 {
			percent := (job.Progress.ProcessedPRs * 100) / job.Progress.TotalPRs
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
			status["progress_percent"] = percent
		}
		return status
	} else if err != nil {
		status["error"] = err.Error()
		return status
	}

	lastSync, err := store.LastSync(repo)
	if err != nil {
		status["error"] = err.Error()
		return status
	}
	if !lastSync.IsZero() {
		status["last_sync"] = lastSync
		status["status"] = "completed"
		status["progress_percent"] = 100
	}

	return status
}

func handleRepoAction(w http.ResponseWriter, r *http.Request, service app.Service, syncAPI repoSyncAPI) {
	repo, action, ok := parseRepoActionPath(r.URL.Path)
	if !ok {
		writeHTTPError(w, http.StatusNotFound, "route not found")
		return
	}

	switch action {
	case "analyze":
		handleAnalyze(w, r, service, repo)
	case "cluster":
		handleCluster(w, r, service, repo)
	case "graph":
		handleGraph(w, r, service, repo)
	case "plan", "plans":
		handlePlan(w, r, service, repo)
	case "sync":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if syncAPI == nil {
			writeHTTPError(w, http.StatusInternalServerError, "sync API unavailable")
			return
		}
		if err := syncAPI.Start(repo); err != nil {
			writeHTTPError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeHTTPJSON(w, http.StatusAccepted, map[string]any{"started": true, "repo": repo})
	case "sync/stream":
		if syncAPI == nil {
			writeHTTPError(w, http.StatusInternalServerError, "sync API unavailable")
			return
		}
		syncAPI.Stream(repo, w, r)
	case "sync/status":
		syncStatus := getSyncStatus(repo)
		writeHTTPJSON(w, http.StatusOK, syncStatus)
	default:
		writeHTTPError(w, http.StatusNotFound, "route not found")
	}
}

func openSettingsStore() (*settings.Store, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_SETTINGS_DB"))
	if path == "" {
		path = "pratc-settings.db"
	}
	store, err := settings.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open settings store: %w", err)
	}
	return store, nil
}

func openAuditStore() (*cache.AuditStore, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return cache.NewAuditStore(store), nil
}

func logAuditEntry(action, repo, details string) {
	auditStore, err := openAuditStore()
	if err != nil {
		return
	}
	defer auditStore.Close()
	entry := audit.AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Repo:      repo,
		Details:   details,
	}
	_ = auditStore.Append(entry)
}

type setSettingRequest struct {
	Scope string `json:"scope"`
	Repo  string `json:"repo"`
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func handleSettings(w http.ResponseWriter, r *http.Request, store settingsStore) {
	switch r.Method {
	case http.MethodGet:
		repo := strings.TrimSpace(r.URL.Query().Get("repo"))
		payload, err := store.Get(r.Context(), repo)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeHTTPJSON(w, http.StatusOK, payload)
	case http.MethodPost:
		var req setSettingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Key) == "" {
			writeHTTPError(w, http.StatusBadRequest, "key is required")
			return
		}
		validateOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("validateOnly")), "true")
		if validateOnly {
			if err := store.ValidateSet(r.Context(), req.Scope, req.Repo, req.Key, req.Value); err != nil {
				writeHTTPError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeHTTPJSON(w, http.StatusOK, map[string]any{"valid": true})
			return
		}
		if err := store.Set(r.Context(), req.Scope, req.Repo, req.Key, req.Value); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeHTTPJSON(w, http.StatusOK, map[string]any{"updated": true})
	case http.MethodDelete:
		scope := strings.TrimSpace(r.URL.Query().Get("scope"))
		repo := strings.TrimSpace(r.URL.Query().Get("repo"))
		key := strings.TrimSpace(r.URL.Query().Get("key"))
		if key == "" {
			writeHTTPError(w, http.StatusBadRequest, "key is required")
			return
		}
		if err := store.Delete(r.Context(), scope, repo, key); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeHTTPJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleExportSettings(w http.ResponseWriter, r *http.Request, store settingsStore) {
	if !ensureGET(w, r) {
		return
	}
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	content, err := store.ExportYAML(r.Context(), scope, repo)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func handleImportSettings(w http.ResponseWriter, r *http.Request, store settingsStore) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	if err := store.ImportYAML(r.Context(), scope, repo, body); err != nil {
		writeHTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeHTTPJSON(w, http.StatusOK, map[string]any{"imported": true})
}

func handleListSyncJobs(w http.ResponseWriter, r *http.Request) {
	if !ensureGET(w, r) {
		return
	}
	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer store.Close()

	jobs, err := store.ListSyncJobs()
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, map[string]any{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"jobs":        jobs,
	})
}

func handleListPausedSyncJobs(w http.ResponseWriter, r *http.Request) {
	if !ensureGET(w, r) {
		return
	}
	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer store.Close()

	jobs, err := store.ListPausedSyncJobs()
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, map[string]any{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"jobs":        jobs,
	})
}

func handleSyncEvents(store *cache.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGET(w, r) {
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeHTTPError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		connectedEvent := map[string]any{"type": "connected", "timestamp": time.Now().UTC().Format(time.RFC3339)}
		connectedData, _ := json.Marshal(connectedEvent)
		fmt.Fprintf(w, "data: %s\n\n", connectedData)
		flusher.Flush()

		lastSeen := make(map[string]cache.SyncJobStatus)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				jobs, err := store.ListSyncJobs()
				if err != nil {
					continue
				}

				current := make(map[string]cache.SyncJobStatus)
				for _, job := range jobs {
					current[job.ID] = job.Status
				}

				for jobID, status := range current {
					if prevStatus, existed := lastSeen[jobID]; !existed {
						emitSyncEvent(w, flusher, "created", jobID, status, "")
					} else if prevStatus != status {
						emitStatusChange(w, flusher, prevStatus, status, jobID, "")
					}
				}

				for jobID, prevStatus := range lastSeen {
					if _, stillExists := current[jobID]; !stillExists {
						if prevStatus == cache.SyncJobStatusFailed {
							emitSyncEvent(w, flusher, "failed", jobID, prevStatus, "")
						} else {
							emitSyncEvent(w, flusher, "completed", jobID, prevStatus, "")
						}
					}
				}

				lastSeen = current
			}
		}
	}
}

func emitSyncEvent(w http.ResponseWriter, flusher http.Flusher, eventType, jobID string, status cache.SyncJobStatus, message string) {
	payload := map[string]any{
		"type":    eventType,
		"jobId":   jobID,
		"status":  status,
		"message": message,
	}
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func emitStatusChange(w http.ResponseWriter, flusher http.Flusher, from, to cache.SyncJobStatus, jobID, message string) {
	var eventType string
	switch to {
	case cache.SyncJobStatusPaused:
		eventType = "paused"
	case cache.SyncJobStatusInProgress:
		if from == cache.SyncJobStatusPaused {
			eventType = "resumed"
		} else {
			eventType = "created"
		}
	case cache.SyncJobStatusCompleted:
		eventType = "completed"
	case cache.SyncJobStatusFailed:
		eventType = "failed"
	default:
		eventType = "changed"
	}
	emitSyncEvent(w, flusher, eventType, jobID, to, message)
}

func handleAnalyze(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	useCacheFirst := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("use_cache_first")), "true")

	maxPRs := 0
	if maxPRsStr := strings.TrimSpace(r.URL.Query().Get("max_prs")); maxPRsStr != "" {
		if v, err := strconv.Atoi(maxPRsStr); err == nil {
			maxPRs = v
		}
	}

	analyzeSvc := service
	if useCacheFirst || maxPRs > 0 {
		analyzeSvc = app.NewService(app.Config{UseCacheFirst: useCacheFirst, MaxPRs: maxPRs})
	}

	if !useCacheFirst && analyzeSyncActive(repo) {
		if !waitForAnalyzeSyncCompletion(repo, analyzeSyncWaitTimeout) {
			jobID, _ := currentAnalyzeSyncJobID(repo)
			writeHTTPJSON(w, http.StatusAccepted, buildAnalyzeSyncTimeoutResponse(repo, jobID, analyzeSyncWaitTimeout))
			return
		}
	}

	if !useCacheFirst {
		if _, _, shouldWarn := checkAnalyzeSyncWarningData(repo); shouldWarn {
			jobID, err := startAnalyzeBackgroundSync(repo)
			if err != nil {
				writeHTTPError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeHTTPJSON(w, http.StatusAccepted, buildAnalyzeSyncInProgressResponse(repo, jobID))
			return
		}
	}

	response, err := analyzeSvc.Analyze(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func handleCluster(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	response, err := service.Cluster(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func handleGraph(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	response, err := service.Graph(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "dot") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, response.DOT)
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func handlePlan(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	// Parse and validate query parameters
	query := r.URL.Query()
	var validationErrors []string

	// target: int, default 20, must be > 0
	target := 20
	if raw := strings.TrimSpace(query.Get("target")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			validationErrors = append(validationErrors, "target must be a positive integer")
		} else {
			target = parsed
		}
	}

	// mode: string, default "combination"
	mode, err := parseMode(query.Get("mode"))
	if err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	// cluster_id: string, optional, no validation needed beyond presence
	clusterID := strings.TrimSpace(query.Get("cluster_id"))
	_ = clusterID // reserved for future use

	// exclude_conflicts: bool, default false
	excludeConflicts := false
	if raw := strings.TrimSpace(query.Get("exclude_conflicts")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			validationErrors = append(validationErrors, "exclude_conflicts must be a boolean (true/false)")
		} else {
			excludeConflicts = parsed
		}
	}
	_ = excludeConflicts // reserved for future use

	// stale_score_threshold: float 0..1, default 0.0
	staleScoreThreshold := 0.0
	if raw := strings.TrimSpace(query.Get("stale_score_threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			validationErrors = append(validationErrors, "stale_score_threshold must be a number")
		} else if parsed < 0 || parsed > 1 {
			validationErrors = append(validationErrors, "stale_score_threshold must be between 0 and 1")
		} else {
			staleScoreThreshold = parsed
		}
	}
	_ = staleScoreThreshold // reserved for future use

	// candidate_pool_cap: int 1..500, default 100
	candidatePoolCap := 100
	if raw := strings.TrimSpace(query.Get("candidate_pool_cap")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			validationErrors = append(validationErrors, "candidate_pool_cap must be an integer")
		} else if parsed < 1 || parsed > 500 {
			validationErrors = append(validationErrors, "candidate_pool_cap must be between 1 and 500")
		} else {
			candidatePoolCap = parsed
		}
	}
	_ = candidatePoolCap // reserved for future use

	// score_min: float 0..100, default 0.0
	scoreMin := 0.0
	if raw := strings.TrimSpace(query.Get("score_min")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			validationErrors = append(validationErrors, "score_min must be a number")
		} else if parsed < 0 || parsed > 100 {
			validationErrors = append(validationErrors, "score_min must be between 0 and 100")
		} else {
			scoreMin = parsed
		}
	}
	_ = scoreMin // reserved for future use

	// Return validation errors if any
	if len(validationErrors) > 0 {
		writeHTTPError(w, http.StatusBadRequest, strings.Join(validationErrors, "; "))
		return
	}

	response, err := service.Plan(r.Context(), repo, target, mode)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

// handlePlanOmni handles GET /api/repos/:owner/:repo/plan/omni
// Accepts a selector expression (ID, range, AND/OR combos) and returns
// a staged plan splitting matched PRs into batches of stage_size.
//
// Query params:
//   - selector (required): PR selector expression, e.g. "1-100", "50-150 AND 200-300"
//   - target (default 20): number of PRs to select from the first stage
//   - stage_size (default 64): max PRs per processing stage
func handlePlanOmni(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	query := r.URL.Query()

	selectorStr := strings.TrimSpace(query.Get("selector"))
	if selectorStr == "" {
		writeHTTPJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "MISSING_SELECTOR",
			"message": "selector parameter is required",
			"status":  "400 Bad Request",
		})
		return
	}

	expr, err := planning.Parse(selectorStr)
	if err != nil {
		writeHTTPJSON(w, http.StatusBadRequest, map[string]any{
			"error":   string(planning.ErrSelectorInvalidSyntax),
			"message": err.Error(),
			"status":  "400 Bad Request",
		})
		return
	}

	stageSize := 64
	if raw := strings.TrimSpace(query.Get("stage_size")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeHTTPJSON(w, http.StatusBadRequest, map[string]any{
				"error":   "INVALID_STAGE_SIZE",
				"message": "stage_size must be a positive integer",
				"status":  "400 Bad Request",
			})
			return
		}
		stageSize = parsed
	}

	target := 20
	if raw := strings.TrimSpace(query.Get("target")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeHTTPJSON(w, http.StatusBadRequest, map[string]any{
				"error":   "INVALID_TARGET",
				"message": "target must be a positive integer",
				"status":  "400 Bad Request",
			})
			return
		}
		target = parsed
	}

	allIDs := expr.AllIDs()

	var stages []types.OmniPlanStage
	totalMatched := len(allIDs)
	stageCount := (totalMatched + stageSize - 1) / stageSize
	if stageCount == 0 {
		stageCount = 1
	}

	for i := 0; i < stageCount; i++ {
		start := i * stageSize
		end := start + stageSize
		if end > totalMatched {
			end = totalMatched
		}
		stageMatched := end - start
		stageSelected := 0
		if i == 0 && target < stageMatched {
			stageSelected = target
		}
		stages = append(stages, types.OmniPlanStage{
			Stage:     i + 1,
			StageSize: stageSize,
			Matched:   stageMatched,
			Selected:  stageSelected,
		})
	}

	selected := make([]int, 0)
	ordering := make([]int, 0)
	if totalMatched > 0 {
		limit := target
		if limit > totalMatched {
			limit = totalMatched
		}
		for i := 0; i < limit; i++ {
			selected = append(selected, allIDs[i])
			ordering = append(ordering, allIDs[i])
		}
	}

	response := types.OmniPlanResponse{
		Repo:        repo,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Selector:    selectorStr,
		Mode:        "omni_batch",
		StageCount:  stageCount,
		Stages:      stages,
		Selected:    selected,
		Ordering:    ordering,
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseRepoActionPath(path string) (repo string, action string, ok bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 5 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "repos" {
		return "", "", false
	}

	return parts[2] + "/" + parts[3], strings.Join(parts[4:], "/"), true
}

func repoFromQuery(r *http.Request, fallback string) string {
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repo != "" {
		return repo
	}
	return strings.TrimSpace(fallback)
}

func ensureGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
	return false
}

func ensureRepo(w http.ResponseWriter, repo string) bool {
	if strings.TrimSpace(repo) != "" {
		return true
	}
	writeHTTPError(w, http.StatusBadRequest, "missing repo parameter")
	return false
}

func writeHTTPJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	writeHTTPJSON(w, status, map[string]string{"error": message})
}

func mirrorDBPath() string {
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	return dbPath
}

func mirrorDirectorySize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func formatMirrorTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	return ts.UTC().Format(time.RFC3339)
}

func writeMirrorList(cmd *cobra.Command, baseDir string) error {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found (directory does not exist)")
			return nil
		}
		return fmt.Errorf("read mirror directory: %w", err)
	}
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found")
		return nil
	}

	store, err := cache.Open(mirrorDBPath())
	if err != nil {
		store = nil
	} else {
		defer store.Close()
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "REPO\tPATH\tSIZE\tLAST_SYNC")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		owner := entry.Name()
		ownerPath := filepath.Join(baseDir, owner)
		subEntries, err := os.ReadDir(ownerPath)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			repoName := strings.TrimSuffix(sub.Name(), ".git")
			repoIdentifier := owner + "/" + repoName
			repoPath := filepath.Join(ownerPath, sub.Name())
			size, sizeErr := mirrorDirectorySize(repoPath)
			if sizeErr != nil {
				size = 0
			}
			lastSync := time.Time{}
			if store != nil {
				if syncedAt, err := store.LastSync(repoIdentifier); err == nil {
					lastSync = syncedAt
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\t%s\n", repoIdentifier, repoPath, size, formatMirrorTimestamp(lastSync))
		}
	}
	return nil
}

func writeMirrorInfo(cmd *cobra.Command, baseDir, repoIdentifier string) error {
	repoPath, err := repo.MirrorPath(baseDir, repoIdentifier)
	if err != nil {
		return fmt.Errorf("invalid repo format: %w", err)
	}
	info, err := os.Stat(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("mirror not found for %s", repoIdentifier)
		}
		return fmt.Errorf("stat mirror path: %w", err)
	}

	size, sizeErr := mirrorDirectorySize(repoPath)
	if sizeErr != nil {
		size = 0
	}

	store, err := cache.Open(mirrorDBPath())
	if err != nil {
		store = nil
	} else {
		defer store.Close()
	}

	lastSync := time.Time{}
	if store != nil {
		if syncedAt, err := store.LastSync(repoIdentifier); err == nil {
			lastSync = syncedAt
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Repository: %s\n", repoIdentifier)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Path: %s\n", repoPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exists: %v\n", info.IsDir())
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Disk usage: %d bytes\n", size)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last modified: %s\n", info.ModTime().UTC().Format(time.RFC3339))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last sync: %s\n", formatMirrorTimestamp(lastSync))
	return nil
}

func RegisterMirrorCommand() {
	baseDir, err := repo.DefaultBaseDir()
	if err != nil {
		log := logger.New("cli")
		log.Error("failed to resolve mirror base directory", "err", err)
		return
	}

	mirrorCmd := &cobra.Command{
		Use:   "mirror",
		Short: "Manage git mirrors",
		Long:  "List, inspect, and clean up git mirrors used for PR analysis",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all synced repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("listing mirrors", "base_dir", baseDir)
			return writeMirrorList(cmd, baseDir)
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info [owner/repo]",
		Short: "Show detailed stats for a mirror",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("mirror info", "repo", args[0])
			return writeMirrorInfo(cmd, baseDir, args[0])
		},
	}

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove mirrors for repos no longer tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("pruning mirrors", "base_dir", baseDir)

			dryRun, _ := cmd.Flags().GetBool("dry-run")

			dbPath := os.Getenv("PRATC_DB_PATH")
			if dbPath == "" {
				home, _ := os.UserHomeDir()
				dbPath = filepath.Join(home, ".pratc", "pratc.db")
			}

			store, err := cache.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open cache: %w", err)
			}
			defer store.Close()

			cachedRepos, err := store.ListAllRepos()
			if err != nil {
				return fmt.Errorf("list cached repos: %w", err)
			}

			entries, err := os.ReadDir(baseDir)
			if err != nil {
				if os.IsNotExist(err) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found")
					return nil
				}
				return fmt.Errorf("read mirror directory: %w", err)
			}

			var toRemove []string
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				owner := entry.Name()
				ownerPath := filepath.Join(baseDir, owner)
				subEntries, _ := os.ReadDir(ownerPath)
				for _, sub := range subEntries {
					if !sub.IsDir() {
						continue
					}
					repoName := sub.Name()
					fullRepo := owner + "/" + repoName
					if !isRepoInList(fullRepo, cachedRepos) {
						toRemove = append(toRemove, filepath.Join(ownerPath, repoName))
					}
				}
			}

			if len(toRemove) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No unused mirrors found")
				return nil
			}

			if dryRun {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "The following mirrors would be removed (use without --dry-run to confirm):")
				for _, path := range toRemove {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), path)
				}
				return nil
			}

			for _, path := range toRemove {
				if err := os.RemoveAll(path); err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStderr(), "Failed to remove %s: %v\n", path, err)
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed: %s\n", path)
				}
			}
			return nil
		},
	}
	pruneCmd.Flags().Bool("dry-run", false, "Show what would be removed without deleting")

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove ALL mirrors (nuclear option)",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("cleaning all mirrors", "base_dir", baseDir)

			confirm, _ := cmd.Flags().GetBool("yes")
			if !confirm {
				return fmt.Errorf("refusing to clean all mirrors without --yes flag")
			}
			entries, err := os.ReadDir(baseDir)
			if err != nil {
				if os.IsNotExist(err) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors to clean")
					return nil
				}
				return fmt.Errorf("read mirror directory: %w", err)
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				path := filepath.Join(baseDir, entry.Name())
				if err := os.RemoveAll(path); err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStderr(), "Failed to remove %s: %v\n", path, err)
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "All mirrors removed from %s\n", baseDir)
			return nil
		},
	}
	cleanCmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	migrateCmd := &cobra.Command{
		Use:   "migrate [owner/repo]",
		Short: "Migrate a legacy mirror into the current location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("migrating mirror", "repo", args[0])
			return runMirrorMigrate(cmd, args[0], baseDir)
		},
	}

	mirrorCmd.AddCommand(listCmd, infoCmd, pruneCmd, cleanCmd, migrateCmd)
	rootCmd.AddCommand(mirrorCmd)
}

func runMirrorMigrate(cmd *cobra.Command, repoIdentifier, baseDir string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current working directory: %w", err)
	}

	plan, err := repo.PlanLegacyMirrorMigration(root, baseDir, repoIdentifier)
	if err != nil {
		if strings.Contains(err.Error(), "destination mirror already exists") {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Destination mirror already exists for %s\n", repoIdentifier)
		}
		return err
	}

	if !plan.ShouldMigrate {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No legacy mirror found for %s\n", repoIdentifier)
		return nil
	}

	if err := repo.MigrateLegacyMirror(root, baseDir, repoIdentifier); err != nil {
		return fmt.Errorf("migrate legacy mirror: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Migrated legacy mirror for %s: %s -> %s\n", repoIdentifier, plan.Source, plan.Destination)
	return nil
}

func isRepoInList(repo string, list []string) bool {
	for _, r := range list {
		if r == repo {
			return true
		}
	}
	return false
}

func RegisterConfigCommand() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration settings",
		Long:  "Get, set, list, delete, export, and import configuration settings",
	}

	var scope string
	var repo string

	// get subcommand
	getCmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			vals, err := store.Get(ctx, repo)
			if err != nil {
				return err
			}
			if v, ok := vals[key]; ok {
				fmt.Fprintln(cmd.OutOrStdout(), v)
			}
			return nil
		},
	}
	getCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	getCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// set subcommand
	setCmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			// Security: reject github_token at repo scope
			if key == "github_token" && scope == settings.ScopeRepo {
				return fmt.Errorf("github_token cannot be set at repo scope for security reasons")
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			if err := store.ValidateSet(ctx, scope, repo, key, value); err != nil {
				return err
			}
			if err := store.Set(ctx, scope, repo, key, value); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set %s=%s at %s scope\n", key, value, scope)
			return nil
		},
	}
	setCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	setCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// list subcommand
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all config key-value pairs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			content, err := store.ExportYAML(ctx, scope, repo)
			if err != nil {
				return err
			}

			var settingsMap map[string]any
			if err := yaml.Unmarshal(content, &settingsMap); err != nil {
				return fmt.Errorf("failed to parse settings: %w", err)
			}

			for key, value := range settingsMap {
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%v\n", key, value)
			}
			return nil
		},
	}
	listCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	listCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// delete subcommand
	deleteCmd := &cobra.Command{
		Use:   "delete [key]",
		Short: "Delete a config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			if err := store.Delete(ctx, scope, repo, key); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s from %s scope\n", key, scope)
			return nil
		},
	}
	deleteCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	deleteCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// export subcommand
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export settings as YAML",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			content, err := store.ExportYAML(ctx, scope, repo)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), string(content))
			return nil
		},
	}
	exportCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	exportCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// import subcommand
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import settings from YAML (stdin)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			content, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			if err := store.ImportYAML(ctx, scope, repo, content); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Settings imported successfully")
			return nil
		},
	}
	importCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	importCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	configCmd.AddCommand(getCmd, setCmd, listCmd, deleteCmd, exportCmd, importCmd)
	rootCmd.AddCommand(configCmd)
}
