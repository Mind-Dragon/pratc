package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/repo"
	"github.com/spf13/cobra"
)

func mirrorDBPath() string {
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	return dbPath
}

func classifyMirrorVolume(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	switch {
	case strings.HasPrefix(cleaned, "/mnt/clawdata2/"):
		return "clawdata2"
	case strings.HasPrefix(cleaned, "/mnt/clawdata1/"):
		return "clawdata1"
	case strings.Contains(cleaned, ".cache/pratc"):
		return "home-cache"
	default:
		return cleaned
	}
}

func mirrorDirectorySize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func formatMirrorTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	return ts.UTC().Format(time.RFC3339)
}

func writeMirrorList(cmd *cobra.Command, baseDir string) error {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found (directory does not exist)")
			return nil
		}
		return fmt.Errorf("read mirror directory: %w", err)
	}
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found")
		return nil
	}

	store, err := cache.Open(mirrorDBPath())
	if err != nil {
		store = nil
	} else {
		defer store.Close()
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "REPO\tPATH\tSIZE\tLAST_SYNC")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		owner := entry.Name()
		ownerPath := filepath.Join(baseDir, owner)
		subEntries, err := os.ReadDir(ownerPath)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			repoName := strings.TrimSuffix(sub.Name(), ".git")
			repoIdentifier := owner + "/" + repoName
			repoPath := filepath.Join(ownerPath, sub.Name())
			size, sizeErr := mirrorDirectorySize(repoPath)
			if sizeErr != nil {
				size = 0
			}
			lastSync := time.Time{}
			if store != nil {
				if syncedAt, err := store.LastSync(repoIdentifier); err == nil {
					lastSync = syncedAt
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\t%s\n", repoIdentifier, repoPath, size, formatMirrorTimestamp(lastSync))
		}
	}
	return nil
}

func writeMirrorInfo(cmd *cobra.Command, baseDir, repoIdentifier string) error {
	repoPath, err := repo.MirrorPath(baseDir, repoIdentifier)
	if err != nil {
		return fmt.Errorf("invalid repo format: %w", err)
	}
	info, err := os.Stat(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("mirror not found for %s", repoIdentifier)
		}
		return fmt.Errorf("stat mirror path: %w", err)
	}

	size, sizeErr := mirrorDirectorySize(repoPath)
	if sizeErr != nil {
		size = 0
	}

	store, err := cache.Open(mirrorDBPath())
	if err != nil {
		store = nil
	} else {
		defer store.Close()
	}

	lastSync := time.Time{}
	if store != nil {
		if syncedAt, err := store.LastSync(repoIdentifier); err == nil {
			lastSync = syncedAt
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Repository: %s\n", repoIdentifier)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Path: %s\n", repoPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exists: %v\n", info.IsDir())
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Disk usage: %d bytes\n", size)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last modified: %s\n", info.ModTime().UTC().Format(time.RFC3339))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last sync: %s\n", formatMirrorTimestamp(lastSync))
	return nil
}

func writeMirrorDoctor(cmd *cobra.Command, baseDir, repoIdentifier string) error {
	resolvedBaseDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		resolvedBaseDir = baseDir
	}
	if resolvedBaseDir == "" {
		resolvedBaseDir = baseDir
	}
	volume := classifyMirrorVolume(resolvedBaseDir)
	expectedVolume := classifyMirrorVolume(baseDir)
	onExpectedDisk := volume == expectedVolume

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Mirror base: %s\n", baseDir)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resolved base: %s\n", resolvedBaseDir)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Storage volume: %s\n", volume)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Expected volume: %s\n", expectedVolume)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "On expected data disk: %v\n", onExpectedDisk)

	if stat, err := os.Stat(resolvedBaseDir); err == nil && stat.IsDir() {
		if size, sizeErr := mirrorDirectorySize(resolvedBaseDir); sizeErr == nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Mirror size: %d bytes\n", size)
		}
	}

	store, err := cache.Open(mirrorDBPath())
	if err != nil {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Last sync: unavailable (cache open failed)")
	} else {
		defer store.Close()
		if repoIdentifier != "" {
			if syncedAt, err := store.LastSync(repoIdentifier); err == nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last sync: %s\n", formatMirrorTimestamp(syncedAt))
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last sync: unavailable for %s\n", repoIdentifier)
			}
		} else if jobs, err := store.ListSyncJobs(); err == nil && len(jobs) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last sync: %s (%s)\n", formatMirrorTimestamp(jobs[0].LastSyncAt), jobs[0].Repo)
		}
	}
	return nil
}

func RegisterMirrorCommand() {
	baseDir, err := repo.DefaultBaseDir()
	if err != nil {
		log := logger.New("cli")
		log.Error("failed to resolve mirror base directory", "err", err)
		return
	}

	mirrorCmd := &cobra.Command{
		Use:   "mirror",
		Short: "Manage git mirrors",
		Long:  "List, inspect, and clean up git mirrors used for PR analysis",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all synced repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("listing mirrors", "base_dir", baseDir)
			return writeMirrorList(cmd, baseDir)
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info [owner/repo]",
		Short: "Show detailed stats for a mirror",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("mirror info", "repo", args[0])
			return writeMirrorInfo(cmd, baseDir, args[0])
		},
	}

	doctorCmd := &cobra.Command{
		Use:   "doctor [owner/repo]",
		Short: "Inspect mirror storage health and placement",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("mirror doctor", "repo", strings.Join(args, " "))
			repoIdentifier := ""
			if len(args) > 0 {
				repoIdentifier = args[0]
			}
			return writeMirrorDoctor(cmd, baseDir, repoIdentifier)
		},
	}

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove mirrors for repos no longer tracked",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("pruning mirrors", "base_dir", baseDir)

			dryRun, _ := cmd.Flags().GetBool("dry-run")

			dbPath := os.Getenv("PRATC_DB_PATH")
			if dbPath == "" {
				home, _ := os.UserHomeDir()
				dbPath = filepath.Join(home, ".pratc", "pratc.db")
			}

			store, err := cache.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open cache: %w", err)
			}
			defer store.Close()

			cachedRepos, err := store.ListAllRepos()
			if err != nil {
				return fmt.Errorf("list cached repos: %w", err)
			}

			entries, err := os.ReadDir(baseDir)
			if err != nil {
				if os.IsNotExist(err) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found")
					return nil
				}
				return fmt.Errorf("read mirror directory: %w", err)
			}

			var toRemove []string
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				owner := entry.Name()
				ownerPath := filepath.Join(baseDir, owner)
				subEntries, _ := os.ReadDir(ownerPath)
				for _, sub := range subEntries {
					if !sub.IsDir() {
						continue
					}
					repoName := sub.Name()
					fullRepo := owner + "/" + repoName
					if !isRepoInList(fullRepo, cachedRepos) {
						toRemove = append(toRemove, filepath.Join(ownerPath, repoName))
					}
				}
			}

			if len(toRemove) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No unused mirrors found")
				return nil
			}

			if dryRun {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "The following mirrors would be removed (use without --dry-run to confirm):")
				for _, path := range toRemove {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), path)
				}
				return nil
			}

			for _, path := range toRemove {
				if err := os.RemoveAll(path); err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStderr(), "Failed to remove %s: %v\n", path, err)
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed: %s\n", path)
				}
			}
			return nil
		},
	}
	pruneCmd.Flags().Bool("dry-run", false, "Show what would be removed without deleting")

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove ALL mirrors (nuclear option)",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("cleaning all mirrors", "base_dir", baseDir)

			confirm, _ := cmd.Flags().GetBool("yes")
			if !confirm {
				return fmt.Errorf("refusing to clean all mirrors without --yes flag")
			}
			entries, err := os.ReadDir(baseDir)
			if err != nil {
				if os.IsNotExist(err) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors to clean")
					return nil
				}
				return fmt.Errorf("read mirror directory: %w", err)
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				path := filepath.Join(baseDir, entry.Name())
				if err := os.RemoveAll(path); err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStderr(), "Failed to remove %s: %v\n", path, err)
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "All mirrors removed from %s\n", baseDir)
			return nil
		},
	}
	cleanCmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	migrateCmd := &cobra.Command{
		Use:   "migrate [owner/repo]",
		Short: "Migrate a legacy mirror into the current location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("migrating mirror", "repo", args[0])
			return runMirrorMigrate(cmd, args[0], baseDir)
		},
	}

	mirrorCmd.AddCommand(listCmd, infoCmd, doctorCmd, pruneCmd, cleanCmd, migrateCmd)
	rootCmd.AddCommand(mirrorCmd)
}

func runMirrorMigrate(cmd *cobra.Command, repoIdentifier, baseDir string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current working directory: %w", err)
	}

	plan, err := repo.PlanLegacyMirrorMigration(root, baseDir, repoIdentifier)
	if err != nil {
		if strings.Contains(err.Error(), "destination mirror already exists") {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Destination mirror already exists for %s\n", repoIdentifier)
		}
		return err
	}

	if !plan.ShouldMigrate {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No legacy mirror found for %s\n", repoIdentifier)
		return nil
	}

	if err := repo.MigrateLegacyMirror(root, baseDir, repoIdentifier); err != nil {
		return fmt.Errorf("migrate legacy mirror: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Migrated legacy mirror for %s: %s -> %s\n", repoIdentifier, plan.Source, plan.Destination)
	return nil
}

func isRepoInList(repo string, list []string) bool {
	for _, r := range list {
		if r == repo {
			return true
		}
	}
	return false
}
