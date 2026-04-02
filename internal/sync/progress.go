package sync

import (
	"fmt"
	"math"

	ratelimitPkg "github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

// FormatSyncEstimate returns a human-readable estimate of how long a sync will take,
// including the number of requests needed and estimated duration.
func FormatSyncEstimate(totalPRs int, options ratelimitPkg.EstimateOptions) string {
	if totalPRs <= 0 {
		return "No sync needed"
	}

	requests := ratelimitPkg.EstimateRequests(totalPRs, options)
	duration := ratelimitPkg.EstimateSyncDuration(requests, nil)

	// Calculate cycles (1 cycle = 1 hour at 4800 req/hour)
	cycles := int(math.Ceil(float64(requests) / 4800))

	// Format duration as Xh Ym
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	var durationStr string
	if hours > 0 {
		durationStr = fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		durationStr = fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("Estimated: %s requests across %d cycles (~%s at 4,800 req/hour)",
		formatNumber(requests), cycles, durationStr)
}

// formatNumber adds thousand separators to a number
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
