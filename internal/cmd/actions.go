package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

func RegisterActionsCommand() {
	rootCmd.AddCommand(newActionsCommand())
}

func newActionsCommand() *cobra.Command {
	var repo string
	var policy string
	var lane string
	var format string
	var dryRun bool
	var useCacheFirst bool
	var resync bool
	var forceCache bool
	var maxPRs int

	command := &cobra.Command{
		Use:   "actions",
		Short: "Generate an advisory ActionPlan for a repository",
		Long: `Generate an advisory ActionPlan for a repository.

The actions command is cache-first and read-only. It emits typed work items and
ActionIntents for later queue/executor phases, but it does not mutate GitHub.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			repo = types.NormalizeRepoName(repo)
			selectedPolicy, err := parsePolicyProfile(policy)
			if err != nil {
				return err
			}
			if lane != "" {
				if _, err := parseActionLaneFlag(lane); err != nil {
					return err
				}
			}

			cfg := buildCacheFirstConfig(useCacheFirst, resync, forceCache, nil)
			cfg.MaxPRs = maxPRs
			cfg.IncludeReview = true
			service := app.NewService(cfg)
			log.Info("starting actions plan", "repo", repo, "policy", selectedPolicy, "lane", lane, "dry_run", dryRun)

			plan, err := service.Actions(ctx, repo, app.ActionOptions{
				PolicyProfile: selectedPolicy,
				LaneFilter:    lane,
				DryRun:        dryRun,
			})
			if err != nil {
				return err
			}

			details := fmt.Sprintf("policy=%s lane=%s dry_run=%t", selectedPolicy, lane, dryRun)
			logAuditEntry("actions", repo, details)

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, plan)
			default:
				return fmt.Errorf("invalid format %q for actions", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&policy, "policy", string(types.PolicyProfileAdvisory), "Policy profile: advisory|guarded|autonomous")
	command.Flags().StringVar(&lane, "lane", "", "Optional action lane filter")
	command.Flags().StringVar(&format, "format", "json", "Output format: json")
	command.Flags().BoolVar(&dryRun, "dry-run", true, "Emit dry-run action intents only")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch (default, cached-first mode is always on)")
	command.Flags().BoolVar(&resync, "resync", false, "Force live refresh: skip cache and fetch fresh data from GitHub")
	command.Flags().BoolVar(&forceCache, "force-cache", false, "Offline mode: use stale cached data, never contact GitHub")
	command.Flags().IntVar(&maxPRs, "max-prs", 0, "Max PRs to consider (0=no cap)")
	_ = command.Flags().MarkHidden("force-cache")
	_ = command.MarkFlagRequired("repo")
	return command
}

func parsePolicyProfile(raw string) (types.PolicyProfile, error) {
	switch types.PolicyProfile(strings.ToLower(strings.TrimSpace(raw))) {
	case types.PolicyProfileAdvisory:
		return types.PolicyProfileAdvisory, nil
	case types.PolicyProfileGuarded:
		return types.PolicyProfileGuarded, nil
	case types.PolicyProfileAutonomous:
		return types.PolicyProfileAutonomous, nil
	default:
		return "", fmt.Errorf("invalid policy %q", raw)
	}
}

func parseActionLaneFlag(raw string) (types.ActionLane, error) {
	switch types.ActionLane(strings.TrimSpace(raw)) {
	case types.ActionLaneFastMerge,
		types.ActionLaneFixAndMerge,
		types.ActionLaneDuplicateClose,
		types.ActionLaneRejectOrClose,
		types.ActionLaneFocusedReview,
		types.ActionLaneFutureOrReengage,
		types.ActionLaneHumanEscalate:
		return types.ActionLane(raw), nil
	default:
		return "", fmt.Errorf("invalid lane %q", raw)
	}
}
