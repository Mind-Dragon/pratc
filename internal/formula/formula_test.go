package formula

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPermutationMatchesMAG40(t *testing.T) {
	got := Count(ModePermutation, 54, 4)
	want := big.NewInt(7590024)

	if got.Cmp(want) != 0 {
		t.Fatalf("Count(permutation, 54, 4) = %s, want %s", got.String(), want.String())
	}
}

func TestCombinationMatchesMAG40(t *testing.T) {
	got := Count(ModeCombination, 54, 5)
	want := big.NewInt(3162510)

	if got.Cmp(want) != 0 {
		t.Fatalf("Count(combination, 54, 5) = %s, want %s", got.String(), want.String())
	}
}

func TestWithReplacementMatchesMAG40(t *testing.T) {
	got := Count(ModeWithReplacement, 54, 4)
	want := big.NewInt(8503056)

	if got.Cmp(want) != 0 {
		t.Fatalf("Count(with_replacement, 54, 4) = %s, want %s", got.String(), want.String())
	}
}

func TestCountEdgeCases(t *testing.T) {
	testCases := []struct {
		name string
		mode Mode
		n    int
		k    int
		want string
	}{
		{name: "permutation k exceeds n", mode: ModePermutation, n: 3, k: 5, want: "0"},
		{name: "combination k exceeds n", mode: ModeCombination, n: 3, k: 5, want: "0"},
		{name: "zero choose zero", mode: ModeCombination, n: 0, k: 0, want: "1"},
		{name: "zero permutation zero", mode: ModePermutation, n: 0, k: 0, want: "1"},
		{name: "zero base with replacement", mode: ModeWithReplacement, n: 0, k: 4, want: "0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Count(tc.mode, tc.n, tc.k); got.String() != tc.want {
				t.Fatalf("Count(%s, %d, %d) = %s, want %s", tc.mode, tc.n, tc.k, got.String(), tc.want)
			}
		})
	}
}

func TestGenerateByIndexPermutation(t *testing.T) {
	pool := fixturePool("A", "B", "C", "D")

	selection, err := GenerateByIndex(ModePermutation, pool, 3, big.NewInt(5))
	if err != nil {
		t.Fatalf("GenerateByIndex() error = %v", err)
	}

	assertTitles(t, selection, "A", "D", "C")
}

func TestGenerateByIndexCombination(t *testing.T) {
	pool := fixturePool("A", "B", "C", "D")

	selection, err := GenerateByIndex(ModeCombination, pool, 2, big.NewInt(4))
	if err != nil {
		t.Fatalf("GenerateByIndex() error = %v", err)
	}

	assertTitles(t, selection, "B", "D")
}

func TestGenerateByIndexWithReplacement(t *testing.T) {
	pool := fixturePool("A", "B", "C")

	selection, err := GenerateByIndex(ModeWithReplacement, pool, 3, big.NewInt(5))
	if err != nil {
		t.Fatalf("GenerateByIndex() error = %v", err)
	}

	assertTitles(t, selection, "A", "B", "C")
}

func TestGenerateByIndexRejectsOutOfRangeIndex(t *testing.T) {
	pool := fixturePool("A", "B", "C")

	if _, err := GenerateByIndex(ModeCombination, pool, 2, big.NewInt(3)); err == nil {
		t.Fatal("GenerateByIndex() error = nil, want out of range error")
	}
}

func TestGenerateByIndexIsDirect(t *testing.T) {
	pool := make([]types.PR, 54)
	for idx := range pool {
		pool[idx] = fixturePR(idx+1, string(rune('A'+(idx%26)))).build()
	}

	start := time.Now()
	selection, err := GenerateByIndex(ModePermutation, pool, 4, big.NewInt(1000000))
	if err != nil {
		t.Fatalf("GenerateByIndex() error = %v", err)
	}

	if len(selection) != 4 {
		t.Fatalf("GenerateByIndex() len = %d, want 4", len(selection))
	}

	if elapsed := time.Since(start); elapsed > time.Millisecond {
		t.Fatalf("GenerateByIndex() took %s, want <= 1ms", elapsed)
	}
}

func TestScoreCandidateDeterministic(t *testing.T) {
	weights := ScoreWeights{
		Age:              0.25,
		Size:             0.20,
		CIStatus:         0.20,
		ReviewStatus:     0.20,
		ConflictCount:    0.10,
		ClusterCoherence: 0.05,
	}

	prs := []types.PR{
		fixturePR(101, "Improve planner").withCreatedAt("2026-02-01T00:00:00Z").withSignals("success", "approved", "cluster-a", 10, 20, 2).build(),
		fixturePR(102, "Stabilize planner").withCreatedAt("2026-02-10T00:00:00Z").withSignals("success", "approved", "cluster-a", 5, 8, 1).build(),
	}

	left, leftReasons := ScoreCandidate(prs, weights, map[int]int{101: 0, 102: 1}, time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC))
	right, rightReasons := ScoreCandidate(prs, weights, map[int]int{101: 0, 102: 1}, time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC))

	if math.Abs(left-right) > 1e-9 {
		t.Fatalf("ScoreCandidate() mismatch: left=%f right=%f", left, right)
	}

	if len(leftReasons) == 0 || len(rightReasons) == 0 {
		t.Fatalf("ScoreCandidate() reasons should not be empty: left=%v right=%v", leftReasons, rightReasons)
	}

	if len(leftReasons) != len(rightReasons) {
		t.Fatalf("ScoreCandidate() reasons length mismatch: %v vs %v", leftReasons, rightReasons)
	}
}

func TestTieredSearchExploresQuickThenThoroughThenExhaustive(t *testing.T) {
	engine := NewEngine(Config{
		Mode: ModeCombination,
		Tiers: []TierConfig{
			{Name: TierQuick, MaxCandidates: 2},
			{Name: TierThorough, MaxCandidates: 2},
			{Name: TierExhaustive, MaxCandidates: 2},
		},
		MaxPoolSize:        10,
		RequirePreFiltered: true,
	})

	pool := []types.PR{
		fixturePR(1, "Independent A").withCreatedAt("2026-02-01T00:00:00Z").withBaseBranch("main").withSignals("success", "approved", "cluster-a", 3, 2, 1).build(),
		fixturePR(2, "Independent B").withCreatedAt("2026-02-02T00:00:00Z").withBaseBranch("main").withSignals("success", "approved", "cluster-a", 2, 1, 1).build(),
		fixturePR(3, "Dependent C").withCreatedAt("2026-02-03T00:00:00Z").withBaseBranch("stack-a").withSignals("success", "review_required", "cluster-b", 4, 2, 1).build(),
		fixturePR(4, "Conflicting D").withCreatedAt("2026-02-04T00:00:00Z").withBaseBranch("main").withSignals("failure", "changes_requested", "cluster-c", 8, 3, 1).withMergeable("conflicting").build(),
	}

	result, err := engine.Search(SearchInput{
		Pool:        pool,
		Target:      2,
		PreFiltered: true,
		Now:         time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Tiers) != 3 {
		t.Fatalf("Search() tier count = %d, want 3", len(result.Tiers))
	}

	if result.Tiers[0].Name != TierQuick || result.Tiers[1].Name != TierThorough || result.Tiers[2].Name != TierExhaustive {
		t.Fatalf("Search() tier order = %#v", result.Tiers)
	}

	if result.Tiers[0].PoolSize != 2 {
		t.Fatalf("quick tier pool size = %d, want 2", result.Tiers[0].PoolSize)
	}

	if result.Tiers[1].PoolSize != 3 {
		t.Fatalf("thorough tier pool size = %d, want 3", result.Tiers[1].PoolSize)
	}

	if result.Tiers[2].PoolSize != 4 {
		t.Fatalf("exhaustive tier pool size = %d, want 4", result.Tiers[2].PoolSize)
	}
}

func TestSearchRejectsNonPreFilteredInput(t *testing.T) {
	engine := NewEngine(Config{
		Mode:               ModeCombination,
		MaxPoolSize:        5,
		RequirePreFiltered: true,
	})

	_, err := engine.Search(SearchInput{
		Pool:   fixturePool("A", "B", "C", "D", "E", "F"),
		Target: 2,
		Now:    time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("Search() error = nil, want pre-filtering error")
	}
}

func TestSearchReturnsBestCandidate(t *testing.T) {
	engine := NewEngine(Config{
		Mode:               ModeCombination,
		MaxPoolSize:        10,
		RequirePreFiltered: true,
		Tiers:              []TierConfig{{Name: TierQuick, MaxCandidates: 3}},
	})

	pool := []types.PR{
		fixturePR(10, "Strong A").withCreatedAt("2026-01-15T00:00:00Z").withBaseBranch("main").withSignals("success", "approved", "cluster-a", 2, 1, 1).build(),
		fixturePR(11, "Strong B").withCreatedAt("2026-01-18T00:00:00Z").withBaseBranch("main").withSignals("success", "approved", "cluster-a", 1, 1, 1).build(),
		fixturePR(12, "Weak C").withCreatedAt("2026-03-01T00:00:00Z").withBaseBranch("main").withSignals("failure", "changes_requested", "cluster-b", 20, 10, 6).withMergeable("conflicting").build(),
	}

	result, err := engine.Search(SearchInput{
		Pool:        pool,
		Target:      2,
		PreFiltered: true,
		Now:         time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(result.Best.Selected) != 2 {
		t.Fatalf("Search() best selected = %d, want 2", len(result.Best.Selected))
	}

	assertTitles(t, result.Best.Selected, "Strong A", "Strong B")
}

func fixturePool(titles ...string) []types.PR {
	pool := make([]types.PR, 0, len(titles))
	for idx, title := range titles {
		pool = append(pool, fixturePR(idx+1, title).build())
	}

	return pool
}

func fixturePR(number int, title string) prBuilder {
	return prBuilder{pr: types.PR{
		ID:                "acme/repo",
		Repo:              "acme/repo",
		Number:            number,
		Title:             title,
		BaseBranch:        "main",
		HeadBranch:        "feature",
		CIStatus:          "success",
		ReviewStatus:      "approved",
		Mergeable:         "mergeable",
		ClusterID:         "cluster-a",
		CreatedAt:         "2026-02-01T00:00:00Z",
		Additions:         1,
		Deletions:         1,
		ChangedFilesCount: 1,
	}}
}

type prBuilder struct {
	pr types.PR
}

func (b prBuilder) withCreatedAt(value string) prBuilder {
	b.pr.CreatedAt = value
	return b
}

func (b prBuilder) withSignals(ciStatus, reviewStatus, clusterID string, additions, deletions, changedFiles int) prBuilder {
	b.pr.CIStatus = ciStatus
	b.pr.ReviewStatus = reviewStatus
	b.pr.ClusterID = clusterID
	b.pr.Additions = additions
	b.pr.Deletions = deletions
	b.pr.ChangedFilesCount = changedFiles
	return b
}

func (b prBuilder) withBaseBranch(value string) prBuilder {
	b.pr.BaseBranch = value
	return b
}

func (b prBuilder) withMergeable(value string) prBuilder {
	b.pr.Mergeable = value
	return b
}

func (b prBuilder) build() types.PR {
	return b.pr
}

func assertTitles(t *testing.T, prs []types.PR, want ...string) {
	t.Helper()

	if len(prs) != len(want) {
		t.Fatalf("selection len = %d, want %d", len(prs), len(want))
	}

	for idx, pr := range prs {
		if pr.Title != want[idx] {
			t.Fatalf("selection[%d] title = %q, want %q", idx, pr.Title, want[idx])
		}
	}
}
