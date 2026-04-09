package review

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jeffersonnunn/pratc/internal/settings"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// ResolvedFinding represents the outcome of resolving disagreement among multiple analyzers.
// It captures the final category decision along with metadata about consensus or disagreement.
type ResolvedFinding struct {
	// Category is the resolved review category after considering all analyzer findings.
	Category types.ReviewCategory `json:"category"`
	// Confidence is the overall confidence in the resolved finding, ranging from 0.0 to 1.0.
	// This may be reduced when analyzers disagree.
	Confidence float64 `json:"confidence"`
	// Reasons describes why this category was selected.
	Reasons []string `json:"reasons"`
	// DisagreementDetected indicates whether analyzers disagreed on the category.
	DisagreementDetected bool `json:"disagreement_detected"`
	// MajorityCategory is the category preferred by the majority of analyzers.
	MajorityCategory string `json:"majority_category"`
	// MinorityCategories lists categories that were not selected by the majority.
	MinorityCategories []string `json:"minority_categories"`
}

// ResolveDisagreement resolves disagreement among multiple analyzer findings using majority wins.
// The strategy:
//   - Unanimous: All analyzers agree → use unanimous result with full confidence.
//   - Majority: More than half agree → use majority category with reduced confidence.
//   - Split: No majority → mark as disagreement, use highest confidence result with penalty.
//
// Confidence reduction:
//   - Majority agreement: 0.8x penalty applied
//   - Split decision: 0.6x penalty applied to highest confidence finding
func ResolveDisagreement(findings []types.AnalyzerFinding) ResolvedFinding {
	if len(findings) == 0 {
		return ResolvedFinding{
			Category:             types.ReviewCategoryNeedsReview,
			Confidence:           0.0,
			Reasons:              []string{"no findings to analyze"},
			DisagreementDetected: false,
			MajorityCategory:     "",
			MinorityCategories:   []string{},
		}
	}

	if len(findings) == 1 {
		// Single analyzer - no disagreement possible
		return ResolvedFinding{
			Category:             types.ReviewCategory(findings[0].Finding),
			Confidence:           findings[0].Confidence,
			Reasons:              []string{"single analyzer result"},
			DisagreementDetected: false,
			MajorityCategory:     findings[0].Finding,
			MinorityCategories:   []string{},
		}
	}

	// Count occurrences of each finding/category
	categoryCount := make(map[string]int)
	categoryConfidence := make(map[string]float64)
	var totalConfidence float64

	for _, f := range findings {
		categoryCount[f.Finding]++
		categoryConfidence[f.Finding] += f.Confidence
		totalConfidence += f.Confidence
	}

	// Sort categories by count descending to find most common
	type categoryPair struct {
		category string
		count    int
	}
	var sorted []categoryPair
	for cat, count := range categoryCount {
		sorted = append(sorted, categoryPair{category: cat, count: count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].category < sorted[j].category
	})

	mostCommon := sorted[0].category
	mostCommonCount := sorted[0].count
	avgConfidence := totalConfidence / float64(len(findings))

	// Identify minority categories
	var minorityCategories []string
	for _, pair := range sorted[1:] {
		minorityCategories = append(minorityCategories, pair.category)
	}

	resolved := ResolvedFinding{
		MinorityCategories: minorityCategories,
	}

	// Confidence penalties for disagreement
	const majorityConfidencePenalty = 0.8
	const splitDecisionPenalty = 0.6

	// Determine resolution based on consensus level
	if mostCommonCount == len(findings) {
		// Unanimous agreement - full confidence
		resolved.Category = types.ReviewCategory(mostCommon)
		resolved.Confidence = avgConfidence
		resolved.DisagreementDetected = false
		resolved.Reasons = []string{fmt.Sprintf("unanimous agreement among %d analyzers", len(findings))}
	} else if float64(mostCommonCount) > float64(len(findings))/2 {
		// Majority agreement - reduced confidence
		resolved.Category = types.ReviewCategory(mostCommon)
		resolved.Confidence = avgConfidence * majorityConfidencePenalty
		resolved.DisagreementDetected = true
		resolved.Reasons = []string{
			fmt.Sprintf("majority agreement (%d of %d analyzers)", mostCommonCount, len(findings)),
			"confidence reduced due to analyzer disagreement",
		}
	} else {
		// Split decision - use highest confidence finding
		var highestConf float64
		var highestFinding string
		for _, f := range findings {
			if f.Confidence > highestConf {
				highestConf = f.Confidence
				highestFinding = f.Finding
			}
		}
		resolved.Category = types.ReviewCategory(highestFinding)
		resolved.Confidence = highestConf * splitDecisionPenalty
		resolved.DisagreementDetected = true
		resolved.Reasons = []string{
			fmt.Sprintf("split decision (%d analyzers, no majority)", len(findings)),
			"using highest confidence result with penalty",
		}
	}

	resolved.MajorityCategory = mostCommon

	return resolved
}

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
