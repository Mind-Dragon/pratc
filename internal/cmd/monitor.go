package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/monitor/data"
	"github.com/jeffersonnunn/pratc/internal/monitor/tui"
	"github.com/spf13/cobra"
)

func RegisterMonitorCommand() {
	var repo string
	var debug bool
	var refresh time.Duration

	command := &cobra.Command{
		Use:   "monitor",
		Short: "Start the TUI monitor dashboard",
		Long: `Start the TUI monitor dashboard for real-time sync job monitoring.

The monitor dashboard displays:
- Active sync jobs and their progress
- GitHub API rate limit status
- Activity timeline
- Console output

Examples:
  # Start monitor with default settings
  pratc monitor

  # Monitor a specific repository
  pratc monitor --repo=owner/repo

  # Enable debug logging
  pratc monitor --debug

  # Custom refresh interval
  pratc monitor --refresh=5s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				select {
				case <-sigChan:
					cancel()
					fmt.Fprintln(cmd.OutOrStdout(), "\nShutting down monitor...")
				case <-ctx.Done():
				}
			}()

			log := logger.New("monitor")
			if debug {
				log = logger.New("monitor:debug")
			}
			log.Info("starting monitor", "repo", repo, "refresh", refresh)

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

			store := data.NewStore(cacheStore)

			token, err := github.ResolveToken(ctx)
			if err != nil {
				return err
			}
			rateLimitFetcher := data.NewRateLimitFetcher(token)

			timelineAgg := data.NewTimelineAggregator(cacheStore)

			broadcaster := data.NewBroadcaster(store, rateLimitFetcher, timelineAgg)

			if refresh > 0 {
				log.Info("using custom refresh interval", "refresh", refresh)
			}

			model := tui.New(broadcaster)
			model.SetRefreshInterval(refresh)
			broadcaster.Start(ctx)

			p := tea.NewProgram(&model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				broadcaster.Stop()
				return fmt.Errorf("run TUI: %w", err)
			}

			broadcaster.Stop()
			log.Info("monitor stopped")
			return nil
		},
	}

	command.Flags().StringVar(&repo, "repo", "", "Filter to specific repository (owner/repo format)")
	command.Flags().BoolVar(&debug, "debug", false, "Enable verbose debug logging")
	command.Flags().DurationVar(&refresh, "refresh", 2*time.Second, "Custom refresh interval (default: 2s)")

	rootCmd.AddCommand(command)
}
