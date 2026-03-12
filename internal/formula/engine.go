package formula

import (
	"errors"
	"math/big"
	"strconv"
	"time"
)

var (
	ErrInputNotPreFiltered = errors.New("formula engine requires pre-filtered input")
	ErrPoolTooLarge        = errors.New("candidate pool exceeds configured maximum")
	ErrNoCandidates        = errors.New("no formula candidates available")
)

type Engine struct {
	config Config
}

func NewEngine(config Config) Engine {
	return Engine{config: config.withDefaults()}
}

func (e Engine) Search(input SearchInput) (SearchResult, error) {
	if e.config.RequirePreFiltered && !input.PreFiltered {
		return SearchResult{}, ErrInputNotPreFiltered
	}
	if e.config.MaxPoolSize > 0 && len(input.Pool) > e.config.MaxPoolSize {
		return SearchResult{}, ErrPoolTooLarge
	}
	if input.Target <= 0 {
		return SearchResult{}, ErrInvalidSelection
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	result := SearchResult{Tiers: make([]TierResult, 0, len(e.config.Tiers))}
	bestSet := false

	for _, tier := range e.config.Tiers {
		tierMode := tier.Mode
		if tierMode == "" {
			tierMode = e.config.Mode
		}

		pool := filterPoolForTier(tier.Name, input.Pool)
		tierResult := TierResult{
			Name:     tier.Name,
			PoolSize: len(pool),
		}

		pickCount := input.Target
		if e.config.PickCount > 0 {
			pickCount = e.config.PickCount
		}
		if len(pool) > 0 && pickCount > len(pool) && tierMode != ModeWithReplacement {
			pickCount = len(pool)
		}

		if len(pool) == 0 || pickCount == 0 {
			result.Tiers = append(result.Tiers, tierResult)
			continue
		}

		total := Count(tierMode, len(pool), pickCount)
		if total.Sign() == 0 {
			result.Tiers = append(result.Tiers, tierResult)
			continue
		}

		maxCandidates := tier.MaxCandidates
		if maxCandidates <= 0 {
			maxCandidates = 1
		}
		tierResult.CandidateCount = minBigInt(total, maxCandidates)

		conflicts := conflictCounts(pool)
		for candidateIndex := 0; candidateIndex < tierResult.CandidateCount; candidateIndex++ {
			selection, err := GenerateByIndex(tierMode, pool, pickCount, big.NewInt(int64(candidateIndex)))
			if err != nil {
				return SearchResult{}, err
			}

			score, reasons := ScoreCandidate(selection, e.config.ScoreWeights, conflicts, now)
			candidate := CandidateResult{
				Tier:              tier.Name,
				Mode:              tierMode,
				Selected:          cloneSelection(selection),
				Score:             score,
				Reasons:           normalizeReasons(reasons),
				FormulaExpression: formulaExpression(tierMode, len(pool), pickCount),
				Index:             strconv.Itoa(candidateIndex),
			}

			if len(tierResult.Best.Selected) == 0 || candidate.Score > tierResult.Best.Score || (candidate.Score == tierResult.Best.Score && selectionSignature(candidate.Selected) < selectionSignature(tierResult.Best.Selected)) {
				tierResult.Best = candidate
			}
			if !bestSet || candidate.Score > result.Best.Score || (candidate.Score == result.Best.Score && selectionSignature(candidate.Selected) < selectionSignature(result.Best.Selected)) {
				result.Best = candidate
				bestSet = true
			}
		}

		result.Tiers = append(result.Tiers, tierResult)
	}

	if !bestSet {
		return SearchResult{}, ErrNoCandidates
	}

	return result, nil
}

func minBigInt(value *big.Int, max int) int {
	maxValue := big.NewInt(int64(max))
	if value.Cmp(maxValue) < 0 {
		return int(value.Int64())
	}

	return max
}
