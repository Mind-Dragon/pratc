package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/audit"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/settings"
	"github.com/spf13/cobra"
)

func writeJSON(cmd *cobra.Command, payload any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func openSettingsStore() (*settings.Store, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_SETTINGS_DB"))
	if path == "" {
		path = "pratc-settings.db"
	}
	store, err := settings.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open settings store: %w", err)
	}
	return store, nil
}

func openAuditStore() (*cache.AuditStore, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return cache.NewAuditStore(store), nil
}

func logAuditEntry(action, repo, details string) {
	auditStore, err := openAuditStore()
	if err != nil {
		return
	}
	defer auditStore.Close()
	entry := audit.AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Repo:      repo,
		Details:   details,
	}
	_ = auditStore.Append(entry)
}
