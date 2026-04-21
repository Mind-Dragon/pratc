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

func RegisterClusterCommand() {
	var repo string
	var format string
	var useCacheFirst bool
	var resync bool
	var forceCache bool

	command := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster pull requests for a repository",
		Long: `Cluster pull requests for a repository.

By default, cluster uses cached data when available and only fetches live
data when the cache is stale or missing. Use --resync to force a live refresh.

Examples:
  # Default: use cache if available, fetch live data if needed
  pratc cluster --repo=owner/repo

  # Force live refresh of all data
  pratc cluster --repo=owner/repo --resync

  # Work offline with stale cache (never contact GitHub)
  pratc cluster --repo=owner/repo --force-cache`,
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			repo = types.NormalizeRepoName(repo)

			cfg := buildClusterConfig(useCacheFirst, resync, forceCache)
			service := app.NewService(cfg)
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
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch (default, cached-first mode is always on)")
	command.Flags().BoolVar(&resync, "resync", false, "Force live refresh: skip cache and fetch fresh data from GitHub")
	command.Flags().BoolVar(&forceCache, "force-cache", false, "Offline mode: use stale cached data, never contact GitHub")
	_ = command.Flags().MarkHidden("force-cache")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}
