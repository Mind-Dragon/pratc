package review

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
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

	// Deterministic heuristic enrichments used for analyst-facing reports.
	safetyResult := ClassifyMergeSafety(prData.PR, prData.ConflictPairs)
	problemResult := ClassifyProblematicPR(prData.PR)

	decisionLayers := buildDecisionLayers(prData, safetyResult, problemResult, finalCategory, finalConfidence)
	reviewResult := types.ReviewResult{
		PRNumber:           prData.PR.Number,
		Title:              prData.PR.Title,
		Author:             prData.PR.Author,
		ClusterID:          prData.PR.ClusterID,
		ProblemType:        problemResult.ProblemType,
		Category:           finalCategory,
		PriorityTier:       types.PriorityTierReviewRequired,
		Confidence:         finalConfidence,
		Reasons:            mergeUniqueStrings(finalReasons, analystReasons(prData, safetyResult, problemResult), decisionLayerSummaries(decisionLayers)),
		DecisionLayers:     decisionLayers,
		AnalyzerFindings:   allFindings,
		Blockers:           mergeUniqueStrings(safetyResult.Blockers),
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
	reviewResult.Blockers = mergeUniqueStrings(reviewResult.Blockers)

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

	// Determine next action based on category and problem type.
	switch finalCategory {
	case types.ReviewCategoryMergeNow:
		reviewResult.NextAction = "merge"
		reviewResult.PriorityTier = DeterminePriorityTier(safetyResult, problemResult)
	case types.ReviewCategoryDuplicateSuperseded:
		reviewResult.NextAction = "duplicate"
		reviewResult.PriorityTier = types.PriorityTierBlocked
	case types.ReviewCategoryProblematicQuarantine:
		if problemResult.ProblemType == "spam" {
			reviewResult.NextAction = "close"
		} else {
			reviewResult.NextAction = "quarantine"
		}
		reviewResult.PriorityTier = types.PriorityTierBlocked
	case types.ReviewCategoryUnknownEscalate:
		reviewResult.NextAction = "escalate"
		reviewResult.PriorityTier = types.PriorityTierReviewRequired
	default:
		reviewResult.NextAction = "review"
		reviewResult.PriorityTier = DeterminePriorityTier(safetyResult, problemResult)
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

func buildDecisionLayers(prData PRData, safetyResult MergeSafetyResult, problemResult ProblematicPRResult, category types.ReviewCategory, confidence float64) []types.DecisionLayer {
	pr := prData.PR
	duplicateReasons := duplicateLayerReasons(prData.DuplicateGroups)
	garbageReasons := garbageLayerReasons(pr)
	staleReasons := staleLayerReasons(prData.Staleness)
	mergeableReasons := mergeableLayerReasons(pr, safetyResult)
	dependencyReasons := dependencyLayerReasons(prData)
	relatedCount := len(prData.RelatedPRs)
	filesTouched := len(pr.FilesChanged)
	changeFootprint := pr.Additions + pr.Deletions
	layer := func(num int, name, bucket, status string, reasons ...string) types.DecisionLayer {
		return types.DecisionLayer{
			Layer:   num,
			Name:    name,
			Bucket:  bucket,
			Status:  status,
			Reasons: mergeUniqueStrings(reasons),
		}
	}

	layers := []types.DecisionLayer{
		layer(1, "Garbage", bucketForLayer1(pr, garbageReasons), layerStatus(garbageReasons), garbageReasons...),
		layer(2, "Duplicates", "duplicate", layerStatus(duplicateReasons), duplicateReasons...),
		layer(3, "Obvious badness", bucketForBadness(problemResult), layerStatus(problemResult.Reasons), append(problemResult.Reasons, problemResult.ProblemType)...),
		layer(4, "Substance score", bucketForSubstance(safetyResult, confidence), layerStatus(safetyResult.Reasons), append([]string{fmt.Sprintf("confidence %.2f", confidence)}, safetyResult.Reasons...)...),
		layer(5, "Now vs future", bucketForCategory(category), layerStatus([]string{string(category)}), fmt.Sprintf("category %s", category)),
		layer(6, "Confidence", bucketForConfidence(confidence), layerStatus([]string{fmt.Sprintf("confidence %.2f", confidence)}), fmt.Sprintf("confidence %.2f", confidence)),
		layer(7, "Dependency", bucketForDependencies(dependencyReasons), layerStatus(dependencyReasons), dependencyReasons...),
		layer(8, "Blast radius", bucketForBlastRadius(changeFootprint), layerStatus([]string{fmt.Sprintf("files_changed %d", filesTouched)}), fmt.Sprintf("files_changed %d", filesTouched), fmt.Sprintf("diff footprint %d", changeFootprint)),
		layer(9, "Leverage", bucketForLeverage(relatedCount), layerStatus([]string{fmt.Sprintf("related PRs %d", relatedCount)}), fmt.Sprintf("related PRs %d", relatedCount), prData.ClusterLabel),
		layer(10, "Ownership", bucketForOwnership(pr), layerStatus([]string{pr.Author}), pr.Author, ownershipReason(pr)),
		layer(11, "Stability", bucketForStability(prData.Staleness, pr.Mergeable), layerStatus(staleReasons), staleReasons...),
		layer(12, "Mergeability", bucketForMergeability(pr.Mergeable), layerStatus(mergeableReasons), mergeableReasons...),
		layer(13, "Strategic weight", bucketForStrategicWeight(category, prData.ClusterLabel), layerStatus([]string{fmt.Sprintf("category %s", category)}), fmt.Sprintf("category %s", category), prData.ClusterLabel),
		layer(14, "Attention cost", bucketForAttentionCost(len(pr.Title), len(pr.Body), len(problemResult.Reasons)), layerStatus([]string{fmt.Sprintf("title %d chars", len(pr.Title))}), fmt.Sprintf("title %d chars", len(pr.Title)), fmt.Sprintf("body %d chars", len(pr.Body)), fmt.Sprintf("reason count %d", len(problemResult.Reasons))),
		layer(15, "Reversibility", bucketForReversibility(changeFootprint), layerStatus([]string{fmt.Sprintf("change footprint %d", changeFootprint)}), fmt.Sprintf("change footprint %d", changeFootprint), fmt.Sprintf("additions %d", pr.Additions), fmt.Sprintf("deletions %d", pr.Deletions)),
		layer(16, "Signal quality", bucketForSignalQuality(problemResult, confidence), layerStatus(problemResult.Reasons), append([]string{fmt.Sprintf("confidence %.2f", confidence)}, problemResult.Reasons...)...),
	}

	return layers
}


func decisionLayerSummaries(layers []types.DecisionLayer) []string {
	summaries := make([]string, 0, len(layers))
	for _, layer := range layers {
		if len(layer.Reasons) == 0 {
			summaries = append(summaries, fmt.Sprintf("L%d %s", layer.Layer, layer.Name))
			continue
		}
		summaries = append(summaries, fmt.Sprintf("L%d %s: %s", layer.Layer, layer.Name, strings.Join(layer.Reasons, "; ")))
	}
	return summaries
}

func garbageLayerReasons(pr types.PR) []string {
	reasons := []string{}
	if pr.IsDraft {
		reasons = append(reasons, "draft PR")
	}
	if pr.IsBot {
		reasons = append(reasons, "bot-authored PR")
	}
	if strings.TrimSpace(pr.Title) == "" {
		reasons = append(reasons, "empty title")
	}
	if strings.TrimSpace(pr.Body) == "" {
		reasons = append(reasons, "empty body")
	}
	return reasons
}

func duplicateLayerReasons(groups []types.DuplicateGroup) []string {
	reasons := make([]string, 0, len(groups)+1)
	for _, group := range groups {
		if group.Reason != "" {
			reasons = append(reasons, group.Reason)
		}
		if group.CanonicalPRNumber > 0 {
			reasons = append(reasons, fmt.Sprintf("canonical PR #%d", group.CanonicalPRNumber))
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no duplicate evidence")
	}
	return reasons
}

func staleLayerReasons(stale *types.StalenessReport) []string {
	if stale == nil {
		return []string{"no staleness signal"}
	}
	reasons := append([]string{}, stale.Reasons...)
	if stale.Score > 0 {
		reasons = append(reasons, fmt.Sprintf("staleness score %.1f", stale.Score))
	}
	return reasons
}

func mergeableLayerReasons(pr types.PR, safetyResult MergeSafetyResult) []string {
	reasons := append([]string{}, safetyResult.Reasons...)
	switch strings.ToLower(strings.TrimSpace(pr.Mergeable)) {
	case "mergeable", "true", "clean":
		reasons = append(reasons, "mergeable state clean")
	case "conflicting", "false", "unclean", "dirty":
		reasons = append(reasons, "merge conflicts present")
	default:
		reasons = append(reasons, "mergeability unknown")
	}
	return mergeUniqueStrings(reasons)
}

func dependencyLayerReasons(prData PRData) []string {
	reasons := make([]string, 0, len(prData.ConflictPairs)+1)
	for _, conflict := range prData.ConflictPairs {
		if conflict.Reason != "" {
			reasons = append(reasons, conflict.Reason)
		}
		if conflict.ConflictType != "" {
			reasons = append(reasons, conflict.ConflictType)
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no dependency blockers")
	}
	return reasons
}

func ownershipReason(pr types.PR) string {
	switch {
	case strings.TrimSpace(pr.Author) == "":
		return "unknown owner"
	case pr.IsBot:
		return "automated ownership"
	default:
		return fmt.Sprintf("owner %s", pr.Author)
	}
}

func layerStatus(reasons []string) string {
	if len(reasons) == 0 {
		return "clear"
	}
	return "observed"
}

func bucketForCategory(category types.ReviewCategory) string {
	switch category {
	case types.ReviewCategoryMergeNow:
		return "now"
	case types.ReviewCategoryMergeAfterFocusedReview:
		return "future"
	case types.ReviewCategoryDuplicateSuperseded:
		return "duplicate"
	case types.ReviewCategoryProblematicQuarantine:
		return "junk"
	case types.ReviewCategoryUnknownEscalate:
		return "blocked"
	default:
		return "low_value"
	}
}

func bucketForLayer1(pr types.PR, reasons []string) string {
	if pr.IsDraft || pr.IsBot || strings.TrimSpace(pr.Title) == "" || strings.TrimSpace(pr.Body) == "" {
		return "junk"
	}
	if len(reasons) > 0 {
		return "junk"
	}
	return "low_value"
}

func bucketForBadness(problemResult ProblematicPRResult) string {
	if !problemResult.IsProblematic {
		return "low_value"
	}
	switch problemResult.ProblemType {
	case "spam":
		return "junk"
	case "broken", "suspicious":
		return "blocked"
	default:
		return "low_value"
	}
}

func bucketForSubstance(safetyResult MergeSafetyResult, confidence float64) string {
	if !safetyResult.IsSafe {
		return "blocked"
	}
	switch {
	case confidence >= 0.85:
		return "high_value"
	case confidence >= 0.65:
		return "merge_candidate"
	case confidence >= 0.5:
		return "needs_review"
	default:
		return "low_value"
	}
}

func bucketForConfidence(confidence float64) string {
	switch {
	case confidence >= 0.85:
		return "high_value"
	case confidence >= 0.65:
		return "needs_review"
	case confidence >= 0.5:
		return "low_value"
	default:
		return "blocked"
	}
}

func bucketForDependencies(reasons []string) string {
	if len(reasons) == 0 || (len(reasons) == 1 && reasons[0] == "no dependency blockers") {
		return "now"
	}
	return "blocked"
}

func bucketForBlastRadius(changeFootprint int) string {
	switch {
	case changeFootprint >= 1000:
		return "blocked"
	case changeFootprint >= 250:
		return "needs_review"
	default:
		return "low_value"
	}
}

func bucketForLeverage(relatedCount int) string {
	switch {
	case relatedCount >= 5:
		return "high_value"
	case relatedCount >= 1:
		return "merge_candidate"
	default:
		return "low_value"
	}
}

func bucketForOwnership(pr types.PR) string {
	if pr.IsBot || strings.TrimSpace(pr.Author) == "" {
		return "needs_review"
	}
	return "high_value"
}

func bucketForStability(stale *types.StalenessReport, mergeable string) string {
	if stale != nil && stale.Score >= 75 {
		return "re_engage"
	}
	if strings.EqualFold(strings.TrimSpace(mergeable), "conflicting") {
		return "blocked"
	}
	return "now"
}

func bucketForMergeability(mergeable string) string {
	switch strings.ToLower(strings.TrimSpace(mergeable)) {
	case "mergeable", "true", "clean":
		return "now"
	case "conflicting", "false", "unclean", "dirty":
		return "blocked"
	default:
		return "needs_review"
	}
}

func bucketForStrategicWeight(category types.ReviewCategory, clusterLabel string) string {
	switch category {
	case types.ReviewCategoryMergeNow:
		return "high_value"
	case types.ReviewCategoryMergeAfterFocusedReview:
		return "merge_candidate"
	case types.ReviewCategoryUnknownEscalate:
		return "needs_review"
	default:
		if strings.TrimSpace(clusterLabel) != "" {
			return "re_engage"
		}
		return "low_value"
	}
}

func bucketForAttentionCost(titleLen, bodyLen, reasonCount int) string {
	cost := titleLen + bodyLen + reasonCount*20
	switch {
	case cost >= 500:
		return "low_value"
	case cost >= 200:
		return "needs_review"
	default:
		return "high_value"
	}
}

func bucketForReversibility(changeFootprint int) string {
	switch {
	case changeFootprint <= 20:
		return "high_value"
	case changeFootprint <= 200:
		return "merge_candidate"
	default:
		return "low_value"
	}
}

func bucketForSignalQuality(problemResult ProblematicPRResult, confidence float64) string {
	if problemResult.IsProblematic {
		return "junk"
	}
	switch {
	case confidence >= 0.85:
		return "high_value"
	case confidence >= 0.65:
		return "merge_candidate"
	default:
		return "low_value"
	}
}


func analystReasons(prData PRData, safetyResult MergeSafetyResult, problemResult ProblematicPRResult) []string {
	reasons := make([]string, 0, 8)
	reasons = append(reasons, safetyResult.Reasons...)
	reasons = append(reasons, problemResult.Reasons...)
	if prData.ClusterLabel != "" {
		reasons = append(reasons, fmt.Sprintf("cluster: %s", prData.ClusterLabel))
	} else if prData.ClusterID != "" {
		reasons = append(reasons, fmt.Sprintf("cluster: %s", prData.ClusterID))
	}
	if prData.Staleness != nil {
		reasons = append(reasons, prData.Staleness.Reasons...)
	}
	for _, dup := range prData.DuplicateGroups {
		if dup.Reason != "" {
			reasons = append(reasons, dup.Reason)
		}
	}
	return mergeUniqueStrings(reasons)
}

func mergeUniqueStrings(parts ...[]string) []string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, group := range parts {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			merged = append(merged, item)
		}
	}
	return merged
}
