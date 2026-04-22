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
	var resync bool
	var forceCache bool
	var maxPRs int
	var rateLimit int
	var reserveBuffer int
	var resetBuffer int
	var planningStrategy string
	var collapseDuplicates bool
	var targetRatio float64
	var minTarget int
	var maxTarget int

	command := &cobra.Command{
		Use:   "plan",
		Short: "Generate a merge plan for a repository",
		Long: `Generate a merge plan for a repository.

By default, plan uses cached data when available and only fetches live
data when the cache is stale or missing. Use --resync to force a live refresh.

Examples:
  # Default: use cache if available, fetch live data if needed
  pratc plan --repo=owner/repo

  # Force live refresh of all data
  pratc plan --repo=owner/repo --resync

  # Work offline with stale cache (never contact GitHub)
  pratc plan --repo=owner/repo --force-cache`,
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

			if err := validateTargetRatio(targetRatio); err != nil {
				return err
			}

			if !cmd.Flags().Changed("dry-run") {
				dryRun = true
			}

			cfg := buildCacheFirstConfig(useCacheFirst, resync, forceCache, nil)
			if maxPRs > 0 {
				cfg.MaxPRs = maxPRs
			}
			cfg.PlanningStrategy = planningStrategy
			cfg.CollapseDuplicates = collapseDuplicates
			cfg.DynamicTarget = app.DynamicTargetConfig{
				Enabled:   true,
				Ratio:     targetRatio,
				MinTarget: minTarget,
				MaxTarget: maxTarget,
			}
			service := app.NewService(cfg)
			log.Info("starting plan", "repo", repo, "target", target, "mode", selectedMode, "budget", budget.String())
			response, err := service.Plan(ctx, repo, target, selectedMode)
			if err != nil {
				return err
			}

			details := fmt.Sprintf("target=%d mode=%s dry_run=%t include_bots=%t collapse_duplicates=%t", target, selectedMode, dryRun, includeBots, collapseDuplicates)
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
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch (default, cached-first mode is always on)")
	command.Flags().BoolVar(&resync, "resync", false, "Force live refresh: skip cache and fetch fresh data from GitHub")
	command.Flags().BoolVar(&forceCache, "force-cache", false, "Offline mode: use stale cached data, never contact GitHub")
	command.Flags().IntVar(&maxPRs, "max-prs", 0, "Max PRs to consider (0=no cap)")
	command.Flags().IntVar(&rateLimit, "rate-limit", 5000, "GitHub API rate limit per hour")
	command.Flags().IntVar(&reserveBuffer, "reserve-buffer", 200, "Minimum requests to keep in reserve")
	command.Flags().IntVar(&resetBuffer, "reset-buffer", 15, "Seconds to wait after rate limit reset")
	command.Flags().StringVar(&planningStrategy, "planning-strategy", "", "Planning strategy: formula (default) or hierarchical")
	command.Flags().BoolVar(&collapseDuplicates, "collapse-duplicates", true, "Collapse duplicate groups before planning (default true)")
	command.Flags().Float64Var(&targetRatio, "target-ratio", 0.05, "Dynamic target ratio: proportion of viable pool to plan (0.05=5%)")
	command.Flags().IntVar(&minTarget, "min-target", 20, "Minimum target when using dynamic target calculation")
	command.Flags().IntVar(&maxTarget, "max-target", 100, "Maximum target when using dynamic target calculation")
	_ = command.Flags().MarkHidden("force-cache")
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

// validateTargetRatio checks that target-ratio is within valid bounds.
// A ratio > 1.0 means more than 100% of the pool, which is nonsensical.
func validateTargetRatio(ratio float64) error {
	if ratio > 1.0 {
		return fmt.Errorf("target-ratio %.2f exceeds maximum of 1.0", ratio)
	}
	return nil
}
