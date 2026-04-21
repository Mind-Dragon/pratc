package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

func RegisterGraphCommand() {
	var repo string
	var format string
	var useCacheFirst bool
	var resync bool
	var forceCache bool
	var maxPRs int
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int

	command := &cobra.Command{
		Use:   "graph",
		Short: "Render a dependency graph for a repository",
		Long: `Render a dependency graph for a repository.

By default, graph uses cached data when available and only fetches live
data when the cache is stale or missing. Use --resync to force a live refresh.

Examples:
  # Default: use cache if available, fetch live data if needed
  pratc graph --repo=owner/repo

  # Force live refresh of all data
  pratc graph --repo=owner/repo --resync

  # Work offline with stale cache (never contact GitHub)
  pratc graph --repo=owner/repo --force-cache`,
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			repo = types.NormalizeRepoName(repo)

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(rateLimit),
				ratelimit.WithReserveBuffer(reserveBuffer),
				ratelimit.WithResetBuffer(resetBuffer),
			)
			log.Info("graph budget initialized", "budget", budget.String())

			cfg := buildClusterConfig(useCacheFirst, resync, forceCache)
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
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch (default, cached-first mode is always on)")
	command.Flags().BoolVar(&resync, "resync", false, "Force live refresh: skip cache and fetch fresh data from GitHub")
	command.Flags().BoolVar(&forceCache, "force-cache", false, "Offline mode: use stale cached data, never contact GitHub")
	command.Flags().IntVar(&maxPRs, "max-prs", 0, "Max PRs to graph (0=no cap)")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	_ = command.Flags().MarkHidden("force-cache")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}
