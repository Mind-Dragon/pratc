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
	var forceCache bool
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

			repo = types.NormalizeRepoName(repo)

			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(rateLimit),
				ratelimit.WithReserveBuffer(reserveBuffer),
				ratelimit.WithResetBuffer(resetBuffer),
			)
			log.Info("graph budget initialized", "budget", budget.String())

			cfg := app.Config{AllowForceCache: forceCache, UseCacheFirst: useCacheFirst}
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
	command.Flags().BoolVar(&forceCache, "force-cache", false, "Use stale cached data without triggering a live sync (for offline analysis)")
	command.Flags().IntVar(&maxPRs, "max-prs", 0, "Max PRs to graph (0=no cap)")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}
