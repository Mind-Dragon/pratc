package sync

import (
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

const defaultChunkSize = 100
const minChunkSize = 10

// CalculateChunkSize determines the optimal chunk size based on the current
// rate limit budget. It returns larger chunks when budget is high and
// progressively smaller chunks as budget depletes.
//
// Budget tiers:
//   - High budget (>2000 remaining): defaultChunkSize (100 PRs)
//   - Medium budget (500-2000): 50 PRs
//   - Low budget (200-500): 25 PRs
//   - Critical budget (<200): minChunkSize (10 PRs) or 0 if below reserve
//
// The function accounts for reserved budget and ensures we never exceed
// what the BudgetManager says we can afford.
func CalculateChunkSize(budget *ratelimit.BudgetManager) int {
	if budget == nil {
		return defaultChunkSize
	}

	remaining := budget.Remaining()

	if remaining <= 200 {
		return 0
	}

	var chunkSize int
	switch {
	case remaining > 2000:
		chunkSize = defaultChunkSize
	case remaining > 500:
		chunkSize = 50
	case remaining > 200:
		chunkSize = 25
	default:
		chunkSize = 0
	}

	requestsNeeded := chunkSize * 3
	if !budget.CanAfford(requestsNeeded) {
		affordableChunk := (remaining - 200) / 3
		if affordableChunk < minChunkSize {
			return 0
		}
		return affordableChunk
	}

	return chunkSize
}
