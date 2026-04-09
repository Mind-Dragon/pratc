package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
)

func TestBuildAnalyzeConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		useCacheFirst bool
		forceLive     bool
		wantAllowLive bool
	}{
		{
			name:          "sync-first without force-live",
			useCacheFirst: true,
			forceLive:     false,
			wantAllowLive: false,
		},
		{
			name:          "live override enabled",
			useCacheFirst: true,
			forceLive:     true,
			wantAllowLive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := buildAnalyzeConfig(tt.useCacheFirst, tt.forceLive, -1, false)
			if cfg.UseCacheFirst != tt.useCacheFirst {
				t.Fatalf("expected UseCacheFirst=%t, got %t", tt.useCacheFirst, cfg.UseCacheFirst)
			}
			if cfg.AllowLive != tt.wantAllowLive {
				t.Fatalf("expected AllowLive=%t, got %t", tt.wantAllowLive, cfg.AllowLive)
			}
		})
	}
}

func TestShouldWarnAnalyzeSync(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		useCacheFirst bool
		force         bool
		forceLive     bool
		want          bool
	}{
		{name: "warns when cache first and no overrides", useCacheFirst: true, want: true},
		{name: "skips when force compatibility flag set", useCacheFirst: true, force: true, want: false},
		{name: "skips when force-live set", useCacheFirst: true, forceLive: true, want: false},
		{name: "skips when cache-first disabled", useCacheFirst: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldWarnAnalyzeSync(tt.useCacheFirst, tt.force, tt.forceLive); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestBuildCacheFirstConfig(t *testing.T) {
	t.Parallel()

	cfg := buildCacheFirstConfig(true)
	if !cfg.UseCacheFirst {
		t.Fatal("expected UseCacheFirst=true")
	}

	cfg = buildCacheFirstConfig(false)
	if cfg.UseCacheFirst {
		t.Fatal("expected UseCacheFirst=false")
	}
}

func TestFormatAnalyzeSyncWarningIncludesRecommendedWorkflow(t *testing.T) {
	t.Parallel()

	warning := formatAnalyzeSyncWarning("owner/repo", 2, true)

	for _, want := range []string{
		"No recent sync data found for owner/repo",
		"Estimated GitHub API calls: ~7 (based on 2 open PRs)",
		"1) pratc sync --repo=owner/repo",
		"2) pratc analyze --repo=owner/repo",
		"Sync in progress",
	} {
		if !strings.Contains(warning, want) {
			t.Fatalf("expected warning to contain %q, got %q", want, warning)
		}
	}
}

func TestFormatAnalyzeSyncWarningFallsBackWhenOpenPRCountUnavailable(t *testing.T) {
	t.Parallel()

	warning := formatAnalyzeSyncWarning("owner/repo", 0, false)

	if !strings.Contains(warning, "Estimated GitHub API calls: unavailable") {
		t.Fatalf("expected fallback estimate message, got %q", warning)
	}
}

func TestCheckAnalyzeSyncWarningDataUsesOpenPRCountWhenSyncIsStale(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)
	t.Setenv("PRATC_CACHE_TTL", "1h")

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	if err := store.UpsertPR(types.PR{
		ID:         "PR_1",
		Repo:       "owner/repo",
		Number:     1,
		Title:      "PR 1",
		URL:        "https://github.com/owner/repo/pull/1",
		Author:     "octocat",
		BaseBranch: "main",
		HeadBranch: "feature/1",
		CreatedAt:  time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		UpdatedAt:  time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed pr: %v", err)
	}
	if err := store.SetLastSync("owner/repo", time.Now().UTC().Add(-2*time.Hour)); err != nil {
		t.Fatalf("seed last sync: %v", err)
	}

	openPRCount, hasOpenPRCount, shouldWarn := checkAnalyzeSyncWarningData("owner/repo")
	if !shouldWarn {
		t.Fatal("expected stale sync data to trigger warning")
	}
	if !hasOpenPRCount || openPRCount != 1 {
		t.Fatalf("expected open PR count to be available, got count=%d available=%t", openPRCount, hasOpenPRCount)
	}
}

func TestWriteAnalyzeTextShowsReviewBucketVocabulary(t *testing.T) {
	t.Parallel()

	response := types.AnalysisResponse{
		Repo:        "test/repo",
		GeneratedAt: "2026-04-09T12:00:00Z",
		Counts: types.Counts{
			TotalPRs: 10,
		},
		ReviewPayload: &types.ReviewResponse{
			TotalPRs:    10,
			ReviewedPRs: 10,
			Categories: []types.ReviewCategoryCount{
				{Category: "merge_safe", Count: 3},
				{Category: "needs_review", Count: 4},
				{Category: "duplicate", Count: 2},
				{Category: "problematic", Count: 1},
			},
			Buckets: []types.BucketCount{
				{Bucket: "Merge now", Count: 3},
				{Bucket: "Merge after focused review", Count: 4},
				{Bucket: "Duplicate / superseded", Count: 2},
				{Bucket: "Problematic / quarantine", Count: 1},
				{Bucket: "Unknown / escalate", Count: 0},
			},
		},
		PRs: []types.PR{
			{Number: 1, Title: "PR 1"},
			{Number: 2, Title: "PR 2"},
		},
	}

	cmd := &cobra.Command{}
	var buf strings.Builder
	cmd.SetOut(&buf)

	err := writeAnalyzeText(cmd, response, true)
	if err != nil {
		t.Fatalf("writeAnalyzeText failed: %v", err)
	}

	output := buf.String()

	bucketLabels := []string{
		"Merge now",
		"Merge after focused review",
		"Duplicate / superseded",
		"Problematic / quarantine",
		"Unknown / escalate",
	}

	for _, label := range bucketLabels {
		if !strings.Contains(output, label) {
			t.Errorf("expected output to contain bucket label %q, output was:\n%s", label, output)
		}
	}
}
