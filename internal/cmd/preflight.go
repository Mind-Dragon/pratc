package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

type preflightResult struct {
	Repo           string  `json:"repo"`
	GeneratedAt    string  `json:"generatedAt"`
	CachedPRs      int     `json:"cachedPRs"`
	LastSynced     string  `json:"lastSynced,omitempty"`
	GitHubOpenPRs  int     `json:"githubOpenPRs"`
	Delta          int     `json:"delta"`
	RateLimitRem   int     `json:"rateLimitRemaining"`
	RateLimitReset string  `json:"rateLimitReset"`
	EstAPICalls    int     `json:"estApiCalls"`
	EstTime        string  `json:"estTime"`
	EstTimeMinutes float64 `json:"estTimeMinutes"`
	LockStatus     string  `json:"lockStatus"`
	Recommendation string  `json:"recommendation"`
}

func RegisterPreflightCommand() {
	var repo string

	command := &cobra.Command{
		Use:   "preflight",
		Short: "Pre-flight check for repository sync planning",
		Long: `Performs a pre-flight check for repository sync planning.

Checks cache status, GitHub API rate limits, and estimates sync time.
Outputs a human-readable summary with recommendations.

Examples:
  # Run preflight check
  pratc preflight --repo=owner/repo

  # Get JSON output for scripting
  pratc preflight --repo=owner/repo --format=json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo = types.NormalizeRepoName(repo)

			log := logger.New("preflight")
			ctx := context.Background()

			dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
			if dbPath == "" {
				home, _ := os.UserHomeDir()
				dbPath = filepath.Join(home, ".pratc", "pratc.db")
			}
			store, err := cache.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open cache store: %w", err)
			}
			defer store.Close()

			result := preflightResult{
				Repo:        repo,
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			}
			log.Info("preflight analysis", "repo", repo)

			// Count cached PRs
			prs, err := store.ListPRs(cache.PRFilter{Repo: repo})
			if err == nil {
				result.CachedPRs = len(prs)
				log.Info("cached PRs", "count", result.CachedPRs)
			} else {
				log.Warn("preflight cache read failed", "repo", repo, "error", err)
			}

			// Get last sync time
			lastSync, err := store.LastSync(repo)
			if err == nil && !lastSync.IsZero() {
				result.LastSynced = lastSync.UTC().Format("2006-01-02")
			}

			// Query GitHub API for open PR count and rate limit using GraphQL
			owner, name, err := splitRepo(repo)
			if err != nil {
				return fmt.Errorf("parse repo: %w", err)
			}
			repoName := owner + "/" + name

			var openPRCount int
			var rateLimit ghRateLimit
			if err := attemptTokenFallbackWithTrace(ctx, log, func(token string) error {
				result, err := github.FetchOpenPRCountWithToken(ctx, token, repoName)
				if err != nil {
					return err
				}
				openPRCount = result.Count
				rateLimit = ghRateLimit{
					Remaining: result.RateLimit.Remaining,
					Reset:     result.RateLimit.ResetAt,
				}
				return nil
			}); err != nil {
				log.Warn("preflight token fallback failed", "repo", repoName, "error", err)
				result.GitHubOpenPRs = 0
			} else {
				result.GitHubOpenPRs = openPRCount
				if rateLimit.Remaining >= 0 {
					result.RateLimitRem = rateLimit.Remaining
				}
				if !rateLimit.Reset.IsZero() {
					result.RateLimitReset = rateLimit.Reset.UTC().Format("15:04 MST")
				}
			}

			// Calculate delta
			result.Delta = result.GitHubOpenPRs - result.CachedPRs
			if result.Delta < 0 {
				result.Delta = 0
			}

			// Estimate API calls needed
			// ~2 API calls per new PR (metadata + files) + pagination overhead
			result.EstAPICalls = estimateAPICalls(result.Delta)

			// Estimate time
			// At 5000 req/hr rate limit, with reserve buffer of 200
			availablePerHour := 5000 - 200
			requestsPerSecond := float64(availablePerHour) / 3600.0
			estSeconds := float64(result.EstAPICalls) / requestsPerSecond
			result.EstTimeMinutes = estSeconds / 60.0

			if estSeconds < 60 {
				result.EstTime = fmt.Sprintf("~%.0f seconds", estSeconds)
			} else if estSeconds < 3600 {
				result.EstTime = fmt.Sprintf("~%.0f minutes", estSeconds/60)
			} else {
				hours := estSeconds / 3600
				result.EstTime = fmt.Sprintf("~%.1f hours", hours)
			}

			// Check lock status
			locked, holder, err := LockStatus(repo)
			if err == nil && locked {
				if holder != nil {
					result.LockStatus = fmt.Sprintf("held by PID %d (started %s)", holder.PID, holder.StartTime)
				} else {
					result.LockStatus = "locked (another instance running)"
				}
			} else {
				result.LockStatus = "clear (no other prATC instance running)"
			}

			// Generate recommendation
			result.Recommendation = generateRecommendation(result.Delta, result.RateLimitRem, result.EstTimeMinutes)

			// Output
			return writePreflightOutput(cmd, result)
		},
	}

	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

type ghRateLimit struct {
	Remaining int
	Reset     time.Time
}

func estimateAPICalls(deltaPRs int) int {
	if deltaPRs <= 0 {
		return 0
	}
	// 2 API calls per new PR (PR metadata + files) + pagination overhead
	// Assume average 100 PRs per page
	pages := (deltaPRs + 99) / 100
	return deltaPRs*2 + pages
}

func generateRecommendation(delta, rateLimitRemaining int, estMinutes float64) string {
	if delta == 0 {
		return "Cache is up-to-date. No sync needed."
	}

	var suggestions []string

	if rateLimitRemaining < 500 {
		suggestions = append(suggestions, fmt.Sprintf("Low rate limit (%d remaining). Consider running sync during off-peak hours.", rateLimitRemaining))
	}

	if delta > 1000 {
		suggestions = append(suggestions, fmt.Sprintf("Large delta of %d PRs. Use --sync-max-prs=500 to cap initial fetch (~2 min).", delta))
	}

	if estMinutes > 60 {
		suggestions = append(suggestions, fmt.Sprintf("Estimated sync time (%.0f min) is long. Consider running with --watch for background sync.", estMinutes))
	}

	if len(suggestions) == 0 {
		return fmt.Sprintf("Use --sync-max-prs=%d to sync %d new PRs (~%.0f min estimated).",
			min(delta, 500), delta, minFloat(estMinutes, 30))
	}

	return strings.Join(suggestions, " ")
}

func writePreflightOutput(cmd *cobra.Command, result preflightResult) error {
	out := cmd.OutOrStdout()

	separator := strings.Repeat("-", 40)

	fmt.Fprintf(out, "\nPreflight for %s\n", result.Repo)
	fmt.Fprintf(out, "%s\n", separator)
	fmt.Fprintf(out, "Cache:          %s PRs", formatNumber(result.CachedPRs))
	if result.LastSynced != "" {
		fmt.Fprintf(out, " (last synced: %s)", result.LastSynced)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "GitHub:         %s open PRs\n", formatNumber(result.GitHubOpenPRs))
	fmt.Fprintf(out, "Delta:          %s PRs to fetch\n", formatNumber(result.Delta))

	if result.RateLimitRem > 0 {
		fmt.Fprintf(out, "Rate limit:     %d remaining (resets in %s)\n",
			result.RateLimitRem, result.RateLimitReset)
	}

	if result.EstAPICalls > 0 {
		fmt.Fprintf(out, "Est. API calls: ~%s\n", formatNumber(result.EstAPICalls))
		fmt.Fprintf(out, "Est. time:      %s (at 4800 req/hr)\n", result.EstTime)
	}

	fmt.Fprintf(out, "Lock status:    %s\n", result.LockStatus)
	fmt.Fprintf(out, "\nRecommendation: %s\n", result.Recommendation)
	fmt.Fprintf(out, "%s\n\n", separator)

	return nil
}

func formatNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return strconv.Itoa(n)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// splitRepo splits an owner/repo string into owner and repo parts.
func splitRepo(repo string) (string, string, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}
