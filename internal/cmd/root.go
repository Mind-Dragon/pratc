package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/version"
	"github.com/spf13/cobra"
)

// rootCmd is the base command for the CLI application
var rootCmd = &cobra.Command{
	Use:   "pratc",
	Short: "PR Air Traffic Control",
	Long:  "prATC is a CLI for pull request analysis, clustering, graphing, planning, and API serving.",
}

// ExecuteContext runs the root command with the given context
func ExecuteContext(ctx context.Context) {
	fmt.Fprintf(os.Stderr, "Harness Optimizer v%s built on %s\n", version.Version, version.BuildDate)

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

// isInvalidArgumentError checks if the error is due to invalid command arguments
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

// RegisterCommands registers all subcommands with the root command
func RegisterCommands() {
	// Register all command groups
	RegisterAnalyzeCommand()
	RegisterClusterCommand()
	RegisterGraphCommand()
	RegisterReportCommand()
	RegisterPlanCommand()
	RegisterServeCommand()
	RegisterSyncCommand()
	RegisterMirrorCommand()
	RegisterConfigCommand()
	RegisterAuditCommand()
	RegisterWorkflowCommand()
	RegisterMonitorCommand()
	RegisterPreflightCommand()
}
