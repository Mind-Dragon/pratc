package app

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/testutil"
)

// BenchmarkPlanPerformance measures end-to-end Plan() latency on the fixture corpus.
// SLO: <90s for 5500 PRs (warm cache). Actual target is <5s p95.
func BenchmarkPlanPerformance(b *testing.B) {
	manifest, err := testutil.LoadManifest()
	if err != nil {
		b.Fatalf("load manifest: %v", err)
	}
	prs, err := testutil.LoadFixturePRs()
	if err != nil {
		b.Fatalf("load fixture PRs: %v", err)
	}
	b.Logf("Fixture: repo=%s, PR count=%d", manifest.Repo, len(prs))

	for _, strategy := range []string{"formula", "hierarchical"} {
		b.Run(strategy, func(b *testing.B) {
			service := NewService(Config{
				Now:              fixedNow,
				PlanningStrategy: strategy,
			})

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := service.Plan(context.Background(), manifest.Repo, 20, formula.ModeCombination)
				if err != nil {
					b.Fatalf("plan (%s): %v", strategy, err)
				}
			}
		})
	}
}
