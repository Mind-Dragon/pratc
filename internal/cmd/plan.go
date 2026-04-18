package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

func RegisterPlanCommand() {
	var repo string
	var target int
	var mode string
	var format string
	var dryRun bool
	var includeBots bool
	var useCacheFirst bool
	var forceCache bool
	var maxPRs int
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int
	var planningStrategy string

	command := &cobra.Command{
		Use:   "plan",
		Short: "Generate a merge plan for a repository",
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
			log.Info("plan budget initialized", "budget", budget.String())

			selectedMode, err := parseMode(mode)
			if err != nil {
				return err
			}

			if !cmd.Flags().Changed("dry-run") {
				dryRun = true
			}

			cfg := buildCacheFirstConfig(useCacheFirst, forceCache, nil)
			if maxPRs > 0 {
				cfg.MaxPRs = maxPRs
			}
			cfg.PlanningStrategy = planningStrategy
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
	command.Flags().BoolVar(&forceCache, "force-cache", false, "Use stale cached data without triggering a live sync (for offline analysis)")
	command.Flags().IntVar(&maxPRs, "max-prs", 0, "Max PRs to consider (0=no cap)")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	command.Flags().StringVar(&planningStrategy, "planning-strategy", "", "Planning strategy: formula (default) or hierarchical")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
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
