package review

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/jeffersonnunn/pratc/internal/settings"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// ResolvedFinding represents the outcome of resolving disagreement among multiple analyzers.
// It captures the final category decision along with metadata about consensus or disagreement.
// Raw findings are preserved for transparency and auditability.
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
	// RawFindings contains all individual analyzer findings that contributed to this resolution.
	// Preserved for transparency, auditability, and debugging analyzer disagreements.
	RawFindings []types.AnalyzerFinding `json:"raw_findings"`
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
			Category:             types.ReviewCategoryUnknownEscalate,
			Confidence:           0.0,
			Reasons:              []string{"no findings to analyze"},
			DisagreementDetected: false,
			MajorityCategory:     "",
			MinorityCategories:   []string{},
			RawFindings:          []types.AnalyzerFinding{},
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
			RawFindings:          findings,
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
		RawFindings:        findings,
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

// ReviewSummary aggregates review results across multiple PRs.
// It provides categorized counts and tracks analyzer disagreements for reporting.
type ReviewSummary struct {
	// TotalPRs is the total number of PRs included in the summary.
	TotalPRs int `json:"total_prs"`
	// Categories maps each review category to its count of PRs.
	Categories map[types.ReviewCategory]int `json:"categories"`
	// PriorityTiers maps each priority tier to its count of PRs.
	PriorityTiers map[types.PriorityTier]int `json:"priority_tiers"`
	// DisagreementCount is the number of PRs where analyzers disagreed.
	DisagreementCount int `json:"disagreement_count"`
}

// GenerateReviewSummary aggregates multiple ResolvedFinding results into a ReviewSummary.
// It counts PRs by category, priority tier, and tracks disagreements.
func GenerateReviewSummary(results []ResolvedFinding) ReviewSummary {
	summary := ReviewSummary{
		TotalPRs:          len(results),
		Categories:        make(map[types.ReviewCategory]int),
		PriorityTiers:     make(map[types.PriorityTier]int),
		DisagreementCount: 0,
	}

	for _, result := range results {
		summary.Categories[result.Category]++

		if result.DisagreementDetected {
			summary.DisagreementCount++
		}
	}

	return summary
}

type Orchestrator struct {
	analyzers     []Analyzer
	config        settings.AnalyzerConfig
	settingsStore *settings.Store
}

func NewOrchestrator(cfg settings.AnalyzerConfig, store *settings.Store) *Orchestrator {
	o := &Orchestrator{
		analyzers:     make([]Analyzer, 0),
		config:        cfg,
		settingsStore: store,
	}
	o.RegisterAnalyzer(NewSecurityAnalyzer())
	o.RegisterAnalyzer(NewReliabilityAnalyzer())
	o.RegisterAnalyzer(NewPerformanceAnalyzer())
	o.RegisterAnalyzer(NewQualityAnalyzer())
	return o
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
// Timeout Behavior:
//   - Each analyzer invocation is wrapped with a timeout context
//   - Default timeout is 30 seconds (DefaultAnalyzerTimeout)
//   - Timeout can be configured via AnalyzerConfig.Timeout
//   - If an analyzer times out, it is skipped with a warning logged
//   - Other analyzers continue execution
//   - Partial results are returned if any analyzers succeed
//
// Deterministic Invocation Order:
//   - Analyzers are invoked in sorted order by name to ensure reproducibility
//   - This guarantees consistent results across multiple runs
func (o *Orchestrator) Review(ctx context.Context, prData PRData) (AnalyzerResult, error) {
	startedAt := time.Now()

	// Determine timeout - use config or default
	timeout := o.config.Timeout
	if timeout <= 0 {
		timeout = DefaultAnalyzerTimeout
	}

	// Clamp timeout to min/max bounds to prevent runaway analysis
	if timeout < MinAnalyzerTimeout {
		timeout = MinAnalyzerTimeout
	}
	if timeout > MaxAnalyzerTimeout {
		timeout = MaxAnalyzerTimeout
	}

	// Get enabled analyzers
	analyzers := o.analyzers
	if len(analyzers) == 0 {
		// No analyzers registered - return empty result
		placeholderResult := types.ReviewResult{
			Category:           types.ReviewCategoryUnknownEscalate,
			PriorityTier:       types.PriorityTierReviewRequired,
			Confidence:         0.0,
			Reasons:            []string{"no analyzers registered"},
			Blockers:           []string{},
			EvidenceReferences: []string{},
			NextAction:         "human_review",
			AnalyzerFindings:   []types.AnalyzerFinding{},
		}
		return AnalyzerResult{
			Result:           placeholderResult,
			AnalyzerName:     "orchestrator",
			AnalyzerVersion:  "0.1.0",
			ProcessingTimeMs: time.Since(startedAt).Milliseconds(),
			Error:            nil,
			IsPartial:        false,
			SkippedReasons:   []string{"no analyzers registered"},
			StartedAt:        startedAt,
			CompletedAt:      time.Now(),
		}, nil
	}

	// Sort analyzers by name for deterministic invocation order
	sortedAnalyzers := make([]Analyzer, len(analyzers))
	copy(sortedAnalyzers, analyzers)
	sort.Slice(sortedAnalyzers, func(i, j int) bool {
		return sortedAnalyzers[i].Metadata().Name < sortedAnalyzers[j].Metadata().Name
	})

	// Collect findings from all analyzers
	// Collect findings and category votes from all analyzers
	var allFindings []types.AnalyzerFinding
	var skippedReasons []string
	categoryCounts := make(map[types.ReviewCategory]int)
	categoryConfidence := make(map[types.ReviewCategory]float64)

	for _, analyzer := range sortedAnalyzers {
		analyzerName := analyzer.Metadata().Name

		// Create timeout context for this analyzer invocation
		analyzerCtx, cancel := context.WithTimeout(ctx, timeout)

		// Run analyzer
		result, err := analyzer.Analyze(analyzerCtx, prData)
		// Always cancel context to prevent context leak
		cancel()

		if err != nil {
			// Check if this was a timeout
			if analyzerCtx.Err() == context.DeadlineExceeded {
				log.Printf("WARN: analyzer %q timed out after %v, skipping", analyzerName, timeout)
				skippedReasons = append(skippedReasons, fmt.Sprintf("analyzer %q timed out after %v", analyzerName, timeout))
			} else {
				log.Printf("WARN: analyzer %q failed: %v, skipping", analyzerName, err)
				skippedReasons = append(skippedReasons, fmt.Sprintf("analyzer %q failed: %v", analyzerName, err))
			}
			continue
		}

		allFindings = append(allFindings, result.Result.AnalyzerFindings...)
		categoryCounts[result.Result.Category]++
		categoryConfidence[result.Result.Category] += result.Result.Confidence
	}

	// Aggregate analyzer result categories into a final category
	var finalCategory types.ReviewCategory
	var finalConfidence float64
	var finalReasons []string

	if len(categoryCounts) == 0 {
		finalCategory = types.ReviewCategoryUnknownEscalate
		finalConfidence = 0.0
		finalReasons = []string{"all analyzers failed or timed out"}
	} else {
		type categoryPair struct {
			category   types.ReviewCategory
			count      int
			confidence float64
		}
		pairs := make([]categoryPair, 0, len(categoryCounts))
		for cat, count := range categoryCounts {
			pairs = append(pairs, categoryPair{category: cat, count: count, confidence: categoryConfidence[cat]})
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].count != pairs[j].count {
				return pairs[i].count > pairs[j].count
			}
			if pairs[i].confidence != pairs[j].confidence {
				return pairs[i].confidence > pairs[j].confidence
			}
			return pairs[i].category < pairs[j].category
		})
		finalCategory = pairs[0].category
		finalConfidence = categoryConfidence[finalCategory] / float64(categoryCounts[finalCategory])
		if len(pairs) > 1 {
			finalReasons = []string{fmt.Sprintf("majority agreement among %d analyzers", categoryCounts[finalCategory])}
		} else {
			finalReasons = []string{"single analyzer result"}
		}
	}

	reviewResult := types.ReviewResult{
		Category:           finalCategory,
		PriorityTier:       types.PriorityTierReviewRequired,
		Confidence:         finalConfidence,
		Reasons:            finalReasons,
		AnalyzerFindings:   allFindings,
		Blockers:           []string{},
		EvidenceReferences: []string{},
		NextAction:         "review",
	}

	// Populate blockers based on PR data and category
	if prData.Staleness != nil && prData.Staleness.Score > 50 {
		reviewResult.Blockers = append(reviewResult.Blockers, "stale: PR has staleness signals")
	}
	if len(prData.ConflictPairs) > 0 {
		reviewResult.Blockers = append(reviewResult.Blockers, "conflict: PR has file conflicts with other PRs")
	}
	if len(prData.DuplicateGroups) > 0 {
		reviewResult.Blockers = append(reviewResult.Blockers, "duplicate: PR may duplicate existing changes")
	}

	// Populate evidence references
	if prData.Staleness != nil {
		reviewResult.EvidenceReferences = append(reviewResult.EvidenceReferences, fmt.Sprintf("staleness_score:%.1f", prData.Staleness.Score))
	}
	reviewResult.EvidenceReferences = append(reviewResult.EvidenceReferences, fmt.Sprintf("cluster:%s", prData.ClusterID))
	if len(prData.ConflictPairs) > 0 {
		reviewResult.EvidenceReferences = append(reviewResult.EvidenceReferences, fmt.Sprintf("conflicts:%d", len(prData.ConflictPairs)))
	}
	if len(prData.DuplicateGroups) > 0 {
		reviewResult.EvidenceReferences = append(reviewResult.EvidenceReferences, fmt.Sprintf("duplicates:%d", len(prData.DuplicateGroups)))
	}

	// Determine next action based on category
	switch finalCategory {
	case types.ReviewCategoryMergeNow:
		reviewResult.NextAction = "merge"
		reviewResult.PriorityTier = types.PriorityTierFastMerge
	case types.ReviewCategoryDuplicateSuperseded:
		reviewResult.NextAction = "resolve_duplicate"
		reviewResult.PriorityTier = types.PriorityTierBlocked
	case types.ReviewCategoryProblematicQuarantine:
		reviewResult.NextAction = "address_issues"
		reviewResult.PriorityTier = types.PriorityTierBlocked
	case types.ReviewCategoryUnknownEscalate:
		reviewResult.NextAction = "human_review"
		reviewResult.PriorityTier = types.PriorityTierReviewRequired
	default:
		reviewResult.NextAction = "review"
		reviewResult.PriorityTier = types.PriorityTierReviewRequired
	}

	isPartial := len(skippedReasons) > 0

	return AnalyzerResult{
		Result:           reviewResult,
		AnalyzerName:     "orchestrator",
		AnalyzerVersion:  "0.1.0",
		ProcessingTimeMs: time.Since(startedAt).Milliseconds(),
		Error:            nil,
		IsPartial:        isPartial,
		SkippedReasons:   skippedReasons,
		StartedAt:        startedAt,
		CompletedAt:      time.Now(),
	}, nil
}
