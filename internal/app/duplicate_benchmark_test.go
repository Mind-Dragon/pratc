package app

import (
	"fmt"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/review"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/util"
)

func BenchmarkClassifyDuplicatesDenseSimilarity(b *testing.B) {
	prs := syntheticDuplicateHeavyPRs("owner/repo", 1500)
	merged := []review.MergedPRRecord(nil)
	emit := func(string, int, int) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifyDuplicates(prs, merged, emit, types.DuplicateThreshold)
	}
}

func BenchmarkClassifyDuplicatesSparseSimilarity(b *testing.B) {
	prs := syntheticAnalysisPRs("owner/repo", 6000)
	merged := []review.MergedPRRecord(nil)
	emit := func(string, int, int) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifyDuplicates(prs, merged, emit, types.DuplicateThreshold)
	}
}

func BenchmarkExactDuplicatePairsSparseSimilarity(b *testing.B) {
	prs := syntheticAnalysisPRs("owner/repo", 6000)
	titleTokens := make([][]string, len(prs))
	bodyTokens := make([][]string, len(prs))
	pairs := make([][2]int, 0, (len(prs)*(len(prs)-1))/2)
	for i := range prs {
		titleTokens[i] = util.Tokenize(prs[i].Title)
		bodyTokens[i] = util.Tokenize(prs[i].Body)
		for j := i + 1; j < len(prs); j++ {
			pairs = append(pairs, [2]int{i, j})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dups := make(map[int]*types.DuplicateGroup)
		ovs := make(map[int]*types.DuplicateGroup)
		runDuplicateCandidatePairs(prs, titleTokens, bodyTokens, pairs, types.CachedDuplicateThreshold, dups, ovs)
	}
}

func syntheticDuplicateHeavyPRs(repo string, count int) []types.PR {
	prs := make([]types.PR, 0, count)
	for i := 0; i < count; i++ {
		cluster := i % 25
		variant := i % 3
		prs = append(prs, types.PR{
			Repo:         repo,
			Number:       i + 1,
			Title:        fmt.Sprintf("auth flow fix cluster %02d variant %d", cluster, variant),
			Body:         fmt.Sprintf("fix auth oauth login token refresh cluster %02d variant %d", cluster, variant),
			FilesChanged: []string{fmt.Sprintf("auth/handler_%02d.go", cluster), fmt.Sprintf("auth/token_%02d.go", cluster)},
			BaseBranch:   "main",
			HeadBranch:   fmt.Sprintf("fix-auth-%02d-%d", cluster, variant),
		})
	}
	return prs
}
