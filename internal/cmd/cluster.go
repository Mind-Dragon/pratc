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
	var forceCache bool

	command := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster pull requests for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			repo = types.NormalizeRepoName(repo)

			service := app.NewService(buildCacheFirstConfig(useCacheFirst, forceCache, nil))
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
	command.Flags().BoolVar(&forceCache, "force-cache", false, "Use stale cached data without triggering a live sync (for offline analysis)")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}
