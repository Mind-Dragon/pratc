package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/audit"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

func RegisterAuditCommand() {
	var limit int
	var format string

	command := &cobra.Command{
		Use:   "audit",
		Short: "Query audit log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := fmt.Sprintf("%d", time.Now().UnixNano())
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)

			log.Info("querying audit log", "limit", limit)
			if err := audit.ValidateLimit(limit); err != nil {
				return fmt.Errorf("invalid argument: %w", err)
			}

			dbPath := os.Getenv("PRATC_DB_PATH")
			if strings.TrimSpace(dbPath) == "" {
				home, _ := os.UserHomeDir()
				dbPath = filepath.Join(home, ".pratc", "pratc.db")
			}

			store, err := cache.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer store.Close()

			auditStore := cache.NewAuditStore(store)
			entries, err := auditStore.List(limit, 0)
			if err != nil {
				return fmt.Errorf("query audit entries: %w", err)
			}

			response := types.AuditResponse{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Entries:     make([]types.AuditEntryResponse, 0, len(entries)),
				Count:       len(entries),
			}

			for _, entry := range entries {
				response.Entries = append(response.Entries, types.AuditEntryResponse{
					Timestamp: entry.Timestamp.Format(time.RFC3339),
					Action:    entry.Action,
					Repo:      entry.Repo,
					Details:   entry.Details,
				})
			}

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for audit", format)
			}
		},
	}

	command.Flags().IntVar(&limit, "limit", 20, "Maximum number of entries to return (0 = all)")
	command.Flags().StringVar(&format, "format", "json", "Output format: json")
	rootCmd.AddCommand(command)
}

func parseOffset(raw string) int {
	offset, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}
