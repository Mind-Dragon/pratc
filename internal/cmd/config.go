package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeffersonnunn/pratc/internal/settings"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// apiKeyPath returns the path to the API key file.
func apiKeyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pratc", "api-key")
}

// loadOrGenerateAPIKey loads the API key from disk or generates a new one.
// It respects PRATC_API_KEY environment variable as an override.
func loadOrGenerateAPIKey() (string, error) {
	// Check environment variable override first
	if envKey := os.Getenv("PRATC_API_KEY"); envKey != "" {
		return envKey, nil
	}

	keyFile := apiKeyPath()

	// Try to read existing key
	if data, err := os.ReadFile(keyFile); err == nil {
		key := strings.TrimSpace(string(data))
		if len(key) == 64 { // 32 bytes = 64 hex chars
			return key, nil
		}
	}

	// Generate new 32-byte key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("generate API key: %w", err)
	}
	key := hex.EncodeToString(keyBytes)

	// Ensure directory exists
	dir := filepath.Dir(keyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create API key dir: %w", err)
	}

	// Write key with mode 0600
	if err := os.WriteFile(keyFile, []byte(key), 0600); err != nil {
		return "", fmt.Errorf("write API key: %w", err)
	}

	return key, nil
}

// getAPIKey returns the configured API key (loads it if needed).
// This is used to verify incoming requests.
func getAPIKey() (string, error) {
	return loadOrGenerateAPIKey()
}

func RegisterConfigCommand() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration settings",
		Long:  "Get, set, list, delete, export, and import configuration settings",
	}

	var scope string
	var repo string

	// get subcommand
	getCmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			vals, err := store.Get(ctx, repo)
			if err != nil {
				return err
			}
			if v, ok := vals[key]; ok {
				fmt.Fprintln(cmd.OutOrStdout(), v)
			}
			return nil
		},
	}
	getCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	getCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// set subcommand
	setCmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			// Security: reject github_token at repo scope
			if key == "github_token" && scope == settings.ScopeRepo {
				return fmt.Errorf("github_token cannot be set at repo scope for security reasons")
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			if err := store.ValidateSet(ctx, scope, repo, key, value); err != nil {
				return err
			}
			if err := store.Set(ctx, scope, repo, key, value); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set %s=%s at %s scope\n", key, value, scope)
			return nil
		},
	}
	setCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	setCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// list subcommand
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all config key-value pairs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			content, err := store.ExportYAML(ctx, scope, repo)
			if err != nil {
				return err
			}

			var settingsMap map[string]any
			if err := yaml.Unmarshal(content, &settingsMap); err != nil {
				return fmt.Errorf("failed to parse settings: %w", err)
			}

			for key, value := range settingsMap {
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%v\n", key, value)
			}
			return nil
		},
	}
	listCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	listCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// delete subcommand
	deleteCmd := &cobra.Command{
		Use:   "delete [key]",
		Short: "Delete a config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			if err := store.Delete(ctx, scope, repo, key); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s from %s scope\n", key, scope)
			return nil
		},
	}
	deleteCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	deleteCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// export subcommand
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export settings as YAML",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			content, err := store.ExportYAML(ctx, scope, repo)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), string(content))
			return nil
		},
	}
	exportCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	exportCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	// import subcommand
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import settings from YAML (stdin)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate scope and repo
			if scope == settings.ScopeRepo && repo == "" {
				return fmt.Errorf("repo identifier required for repo scope")
			}
			if scope != settings.ScopeGlobal && scope != settings.ScopeRepo {
				return fmt.Errorf("invalid scope %q, must be 'global' or 'repo'", scope)
			}

			content, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			store, err := openSettingsStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := cmd.Context()
			if err := store.ImportYAML(ctx, scope, repo, content); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Settings imported successfully")
			return nil
		},
	}
	importCmd.Flags().StringVar(&scope, "scope", "global", "Scope (global or repo)")
	importCmd.Flags().StringVar(&repo, "repo", "", "Repository identifier (required for repo scope)")

	configCmd.AddCommand(getCmd, setCmd, listCmd, deleteCmd, exportCmd, importCmd)
	rootCmd.AddCommand(configCmd)
}
