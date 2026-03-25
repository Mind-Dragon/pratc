package app

import (
	"sort"

	"github.com/jeffersonnunn/pratc/internal/planning"
)

type StageConfig struct {
	StageSize int
}

type StageResult struct {
	Stage     int
	StageSize int
	InputIDs  []int
	OutputIDs []int
	Rejected  int
	Reason    string
}

type BatchProcessor struct {
	config StageConfig
}

func NewBatchProcessor(config StageConfig) *BatchProcessor {
	if config.StageSize <= 0 {
		config.StageSize = 64
	}
	return &BatchProcessor{config: config}
}

func (bp *BatchProcessor) Process(expr *planning.SelectExpr, availableIDs []int) []StageResult {
	evaluator := planning.NewEvaluator(availableIDs)
	evalResult := evaluator.Eval(expr)
	matchedIDs := append([]int(nil), evalResult.SelectedIDs...)

	sort.Ints(matchedIDs)

	stageSize := bp.config.StageSize
	if stageSize <= 0 {
		stageSize = 64
	}

	var results []StageResult
	stageCount := (len(matchedIDs) + stageSize - 1) / stageSize
	if stageCount == 0 {
		stageCount = 1
	}

	for i := 0; i < stageCount; i++ {
		start := i * stageSize
		end := start + stageSize
		if end > len(matchedIDs) {
			end = len(matchedIDs)
		}
		stageIDs := matchedIDs[start:end]

		results = append(results, StageResult{
			Stage:     i + 1,
			StageSize: stageSize,
			InputIDs:  stageIDs,
			OutputIDs: stageIDs,
			Rejected:  0,
			Reason:    "",
		})
	}

	return results
}

func (bp *BatchProcessor) StageCount(total int) int {
	stageSize := bp.config.StageSize
	if stageSize <= 0 {
		stageSize = 64
	}
	if total <= 0 {
		return 1
	}
	return (total + stageSize - 1) / stageSize
}

// Recompose merges stage results, dedupes by PR number, and returns final ordering.
func Recompose(stages []StageResult) []int {
	seen := make(map[int]bool)
	var result []int

	// Process stages in order, add IDs in stage order
	for _, stage := range stages {
		for _, id := range stage.OutputIDs {
			if !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}

	// Deterministic: already in stage order (which is by PR number ascending)
	// But ensure global ordering is stable by also sorting final result
	// Use stable sort to preserve stage order as primary, PR number as tiebreaker
	sort.SliceStable(result, func(i, j int) bool {
		if result[i] != result[j] {
			return result[i] < result[j]
		}
		return result[i] < result[j]
	})

	return result
}

// Deduplicate removes duplicate PR numbers from a slice while preserving order.
func Deduplicate(ids []int) []int {
	seen := make(map[int]bool)
	var result []int
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}
