// Package ratelimit provides budget tracking and chunk size calculation for GitHub API rate limits.
package ratelimit

// chunkConfig holds configuration for chunk size calculation.
type chunkConfig struct {
	requestsPerPR int
}

// ChunkOption is a functional option for configuring chunk size calculation.
type ChunkOption func(*chunkConfig)

// WithRequestsPerPR sets the number of API requests per PR.
// Default is 3 (files + reviews + CI amortized).
func WithRequestsPerPR(n int) ChunkOption {
	return func(c *chunkConfig) {
		if n > 0 {
			c.requestsPerPR = n
		}
	}
}

func defaultChunkConfig() chunkConfig {
	return chunkConfig{
		requestsPerPR: 3,
	}
}

// CalculateChunkSize determines how many PRs to process in the current rate limit cycle.
//
// Formula: available = max(0, remainingBudget - reserveBuffer)
//
//	chunkSize = min(totalPRs, available / requestsPerPR)
//
// Parameters:
//   - totalPRs: total number of PRs to process
//   - remainingBudget: remaining API requests in current rate limit window
//   - reserveBuffer: minimum requests to keep in reserve (default: 200)
//
// Returns the number of PRs to process in this chunk.
func CalculateChunkSize(totalPRs, remainingBudget, reserveBuffer int, opts ...ChunkOption) int {
	if totalPRs <= 0 {
		return 0
	}
	if remainingBudget <= 0 {
		return 0
	}

	cfg := defaultChunkConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	available := remainingBudget - reserveBuffer
	if available <= 0 {
		return 0
	}

	chunkSize := available / cfg.requestsPerPR
	if chunkSize > totalPRs {
		return totalPRs
	}
	return chunkSize
}
