package formula

import (
	"cmp"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

const (
	TierQuick      = "quick"
	TierThorough   = "thorough"
	TierExhaustive = "exhaustive"
)

type SearchInput struct {
	Pool        []types.PR
	Target      int
	PreFiltered bool
	Now         time.Time
}

type CandidateResult struct {
	Tier              string
	Mode              Mode
	Selected          []types.PR
	Score             float64
	Reasons           []string
	FormulaExpression string
	Index             string
}

type TierResult struct {
	Name           string
	PoolSize       int
	CandidateCount int
	Best           CandidateResult
}

type SearchResult struct {
	Tiers     []TierResult
	Best      CandidateResult
	Telemetry types.OperationTelemetry
}

func filterPoolForTier(name string, pool []types.PR) []types.PR {
	filtered := make([]types.PR, 0, len(pool))
	for _, pr := range pool {
		switch name {
		case TierQuick:
			if pr.BaseBranch == "main" && pr.Mergeable != "conflicting" && pr.CIStatus == "success" {
				filtered = append(filtered, pr)
			}
		case TierThorough:
			if pr.Mergeable != "conflicting" {
				filtered = append(filtered, pr)
			}
		case TierExhaustive:
			filtered = append(filtered, pr)
		default:
			filtered = append(filtered, pr)
		}
	}

	slices.SortFunc(filtered, func(left, right types.PR) int {
		return cmp.Compare(left.Number, right.Number)
	})

	return filtered
}

func formulaExpression(mode Mode, n, k int) string {
	switch mode {
	case ModePermutation:
		return fmt.Sprintf("P(%d,%d)", n, k)
	case ModeCombination:
		return fmt.Sprintf("C(%d,%d)", n, k)
	case ModeWithReplacement:
		return fmt.Sprintf("%d^%d", n, k)
	default:
		return fmt.Sprintf("unknown(%d,%d)", n, k)
	}
}

func conflictCounts(pool []types.PR) map[int]int {
	counts := make(map[int]int, len(pool))
	for _, pr := range pool {
		counts[pr.Number] = 0
	}

	for i, left := range pool {
		for j := i + 1; j < len(pool); j++ {
			right := pool[j]
			if left.BaseBranch != right.BaseBranch {
				continue
			}

			if left.Mergeable == "conflicting" || right.Mergeable == "conflicting" || sharesFiles(left.FilesChanged, right.FilesChanged) {
				counts[left.Number]++
				counts[right.Number]++
			}
		}
	}

	return counts
}

func sharesFiles(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}

	seen := make(map[string]struct{}, len(left))
	for _, path := range left {
		seen[path] = struct{}{}
	}

	for _, path := range right {
		if _, ok := seen[path]; ok {
			return true
		}
	}

	return false
}

func cloneSelection(selection []types.PR) []types.PR {
	cloned := make([]types.PR, len(selection))
	copy(cloned, selection)
	return cloned
}

func normalizeReasons(reasons []string) []string {
	cloned := slices.Clone(reasons)
	sort.Strings(cloned)
	return cloned
}

func selectionSignature(selection []types.PR) string {
	titles := make([]string, 0, len(selection))
	for _, pr := range selection {
		titles = append(titles, pr.Title)
	}

	return strings.Join(titles, "|")
}
