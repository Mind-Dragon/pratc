package review

import (
	"context"
	"time"

	"github.com/jeffersonnunn/pratc/internal/settings"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type Orchestrator struct {
	analyzers     []Analyzer
	config        settings.AnalyzerConfig
	settingsStore *settings.Store
}

func NewOrchestrator(cfg settings.AnalyzerConfig, store *settings.Store) *Orchestrator {
	return &Orchestrator{
		analyzers:     make([]Analyzer, 0),
		config:        cfg,
		settingsStore: store,
	}
}

func (o *Orchestrator) RegisterAnalyzer(a Analyzer) {
	o.analyzers = append(o.analyzers, a)
}

func (o *Orchestrator) Analyzers() []Analyzer {
	result := make([]Analyzer, len(o.analyzers))
	copy(result, o.analyzers)
	return result
}

func (o *Orchestrator) Config() settings.AnalyzerConfig {
	return o.config
}

func (o *Orchestrator) SettingsStore() *settings.Store {
	return o.settingsStore
}

// Review executes the agentic PR review pipeline for the given PR data.
// The pipeline follows these stages:
//   - Load PRs: Gather all PR metadata, cluster assignments, and related data
//   - Deterministic: Apply deterministic classification logic (stateless rules)
//   - Analyzers: Invoke registered analyzers to produce findings
//   - Aggregate: Combine analyzer results into a final ReviewResult
//
// This method is a skeleton implementation that returns a placeholder result.
// Full implementation is deferred to Wave 5 (Deterministic review engine) and
// Wave 6 (Analyzer plugins).
//
// TODO(agentic-pr-review): Implement full pipeline
//   - Wave 5: Deterministic classification (draft detection, conflict check, CI status)
//   - Wave 6: Analyzer plugin invocation with timeout and error handling
//   - Wave 6: Result aggregation from multiple analyzers
func (o *Orchestrator) Review(ctx context.Context, prData PRData) (AnalyzerResult, error) {
	startedAt := time.Now()

	// TODO(agentic-pr-review): Implement full Review pipeline
	//
	// Pipeline stages:
	// 1. Load PRs - PR data already provided in prData parameter
	// 2. Deterministic - Apply stateless classification rules
	//    - Draft detection (is_draft flag)
	//    - Conflict detection (from ConflictPairs)
	//    - Staleness classification (from Staleness report)
	// 3. Analyzers - Invoke each registered analyzer
	//    - Run Analyze() on each analyzer with timeout
	//    - Handle analyzer errors gracefully (partial results)
	// 4. Aggregate - Combine results into final ReviewResult
	//    - Merge AnalyzerFindings from all analyzers
	//    - Determine overall Category and PriorityTier
	//    - Calculate aggregate Confidence

	// Placeholder result - returns a minimal valid result indicating
	// review has been processed but not fully implemented
	placeholderResult := types.ReviewResult{
		Category:         types.ReviewCategoryNeedsReview,
		PriorityTier:     types.PriorityTierReviewRequired,
		Confidence:       0.0,
		Reasons:          []string{"placeholder"},
		AnalyzerFindings: []types.AnalyzerFinding{},
	}

	return AnalyzerResult{
		Result:           placeholderResult,
		AnalyzerName:     "orchestrator",
		AnalyzerVersion:  "0.1.0",
		ProcessingTimeMs: time.Since(startedAt).Milliseconds(),
		Error:            nil,
		IsPartial:        false,
		SkippedReasons:   []string{"analyzer plugins not yet implemented"},
		StartedAt:        startedAt,
		CompletedAt:      time.Now(),
	}, nil
}
