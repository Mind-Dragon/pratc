package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestAnalyze6000CorpusProof(t *testing.T) {
	t.Parallel()
	store := openBenchmarkCache(t)
	seedBenchmarkCachedPRs(t, store, "owner/repo", syntheticAnalysisPRs("owner/repo", 6000), fixedNow())

	service := Service{
		now:          fixedNow,
		useCacheFirst: true,
		cacheStore:   store,
		cacheTTL:     time.Hour,
		maxPRs:       6000,
	}

	start := time.Now()
	response, err := service.Analyze(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}
	if len(response.PRs) != 6000 {
		t.Fatalf("Analyze() PRs = %d, want 6000", len(response.PRs))
	}
	if response.AnalysisTruncated {
		t.Fatalf("Analyze() unexpectedly truncated: %s", response.TruncationReason)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Minute {
		t.Fatalf("Analyze() took %s, want <= 5m", elapsed)
	}
}

func BenchmarkAnalyze6000Corpus(b *testing.B) {
	store := openBenchmarkCache(b)
	seedBenchmarkCachedPRs(b, store, "owner/repo", syntheticAnalysisPRs("owner/repo", 6000), fixedNow())

	service := Service{
		now:          fixedNow,
		useCacheFirst: true,
		cacheStore:   store,
		cacheTTL:     time.Hour,
		maxPRs:       6000,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response, err := service.Analyze(context.Background(), "owner/repo")
		if err != nil {
			b.Fatalf("Analyze() failed: %v", err)
		}
		if len(response.PRs) != 6000 {
			b.Fatalf("Analyze() PRs = %d, want 6000", len(response.PRs))
		}
	}
}

func openBenchmarkCache(tb testing.TB) *cache.Store {
	tb.Helper()
	store, err := cache.Open(filepath.Join(tb.TempDir(), "pratc.db"))
	if err != nil {
		tb.Fatalf("open cache: %v", err)
	}
	tb.Cleanup(func() {
		if err := store.Close(); err != nil {
			tb.Fatalf("close cache: %v", err)
		}
	})
	return store
}

func seedBenchmarkCachedPRs(tb testing.TB, store *cache.Store, repo string, prs []types.PR, syncedAt time.Time) {
	tb.Helper()
	for _, pr := range prs {
		if pr.Repo != repo {
			continue
		}
		if err := store.UpsertPR(pr); err != nil {
			tb.Fatalf("seed cached pr %d: %v", pr.Number, err)
		}
	}
	if err := store.SetLastSync(repo, syncedAt); err != nil {
		tb.Fatalf("seed last sync: %v", err)
	}
}
