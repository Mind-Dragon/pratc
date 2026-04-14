package planning

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jeffersonnunn/pratc/internal/graph"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// HierarchicalPlanner implements three-level hierarchical planning:
// Level 1: Select cluster/batch order based on priority pool selector scores
// Level 2: Rank PRs within selected batches using weighted scoring
// Level 3: Emit final merge ordering respecting dependency constraints
//
// This reduces complexity from O(C(n,k)) to O(C(clusters,c) × C(avg_cluster_size,s))
type HierarchicalPlanner struct {
	poolSelector          *PoolSelector
	graphBuilder          *graph.IncrementalGraph
	maxClusters           int  // Number of clusters to select at Level 1
	maxPerCluster         int  // Number of PRs to select per cluster at Level 2
	targetTotal           int  // Final target number of PRs in merge plan
	depOrderingEnabled bool // Whether to use topological ordering at Level 3
}

// HierarchicalConfig configures the hierarchical planning pipeline.
type HierarchicalConfig struct {
	// MaxClusters is the maximum number of clusters to select at Level 1
	// Default: 5 clusters
	MaxClusters int `json:"max_clusters"`
	// MaxPerCluster is the maximum number of PRs to select per cluster at Level 2
	// Default: 10 PRs per cluster
	MaxPerCluster int `json:"max_per_cluster"`
	// TargetTotal is the final target number of PRs in the merge plan
	// Default: 20 PRs
	TargetTotal int `json:"target_total"`
	// UseDependencyOrdering enables topological sorting at Level 3
	// Default: true
	UseDependencyOrdering bool `json:"use_dependency_ordering"`
}

// DefaultHierarchicalConfig returns the default hierarchical planning configuration.
func DefaultHierarchicalConfig() HierarchicalConfig {
	return HierarchicalConfig{
		MaxClusters:           5,
		MaxPerCluster:         10,
		TargetTotal:           20,
		UseDependencyOrdering: true,
	}
}

// Validate validates the hierarchical configuration.
func (c HierarchicalConfig) Validate() error {
	if c.MaxClusters <= 0 {
		return ErrInvalidClusterCount
	}
	if c.MaxPerCluster <= 0 {
		return ErrInvalidPerClusterCount
	}
	if c.TargetTotal <= 0 {
		return ErrInvalidTargetTotal
	}
	if c.MaxClusters*c.MaxPerCluster < c.TargetTotal {
		return ErrInsufficientCandidatePool
	}
	return nil
}

// HierarchyResult contains the result of hierarchical planning.
type HierarchyResult struct {
	Repo                string                   `json:"repo"`
	GeneratedAt         string                   `json:"generated_at"`
	Config              HierarchicalConfig       `json:"config"`
	SelectedClusters    []ClusterSelection       `json:"selected_clusters"`
	FinalCandidates     []HierarchicalCandidate  `json:"final_candidates"`
	Ordering            []HierarchicalCandidate  `json:"ordering"`
	Rejections          []HierarchyRejection     `json:"rejections"`
	Telemetry           types.OperationTelemetry `json:"telemetry"`
	ComplexityReduction float64                  `json:"complexity_reduction"`
}

// ClusterSelection represents a cluster selected at Level 1.
type ClusterSelection struct {
	ClusterID       string   `json:"cluster_id"`
	ClusterLabel    string   `json:"cluster_label"`
	AveragePriority float64  `json:"average_priority"`
	PRCount         int      `json:"pr_count"`
	SelectedCount   int      `json:"selected_count"`
	ReasonCodes     []string `json:"reason_codes"`
}

// HierarchicalCandidate represents a PR selected through hierarchical planning.
type HierarchicalCandidate struct {
	PR              types.PR        `json:"pr"`
	ClusterID       string          `json:"cluster_id"`
	PriorityScore   float64         `json:"priority_score"`
	Level1Rank      int             `json:"level1_rank"`
	Level2Rank      int             `json:"level2_rank"`
	Level3Rank      int             `json:"level3_rank"`
	ComponentScores ComponentScores `json:"component_scores"`
	ReasonCodes     []string        `json:"reason_codes"`
	DependencyDepth int             `json:"dependency_depth"`
}

// HierarchyRejection contains rejection details for hierarchical planning.
type HierarchyRejection struct {
	PRNumber    int      `json:"pr_number"`
	Reason      string   `json:"reason"`
	ReasonCodes []string `json:"reason_codes"`
}

// NewHierarchicalPlanner creates a new hierarchical planner with the given pool selector.
func NewHierarchicalPlanner(ps *PoolSelector, cfg HierarchicalConfig) (*HierarchicalPlanner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &HierarchicalPlanner{
		poolSelector:          ps,
		maxClusters:          cfg.MaxClusters,
		maxPerCluster:         cfg.MaxPerCluster,
		targetTotal:           cfg.TargetTotal,
		depOrderingEnabled: cfg.UseDependencyOrdering,
	}, nil
}

// Plan executes the three-level hierarchical planning pipeline.
//
// Level 1: Select cluster/batch order based on priority pool selector scores
// Level 2: Rank PRs within selected batches using weighted scoring
// Level 3: Emit final merge ordering respecting dependency constraints
func (hp *HierarchicalPlanner) Plan(ctx context.Context, repo string, prs []types.PR, decayConfig TimeDecayConfig) *HierarchyResult {
	startTime := time.Now()
	now := time.Now().UTC()

	// Level 1: Select clusters based on aggregate priority scores
	level1Start := time.Now()
	clusterSelections := hp.selectClusters(prs, now, decayConfig)
	level1Duration := time.Since(level1Start)

	// Level 2: Rank PRs within selected clusters
	level2Start := time.Now()
	allCandidates := hp.rankWithinClusters(clusterSelections, prs, now, decayConfig)
	level2Duration := time.Since(level2Start)

	// Level 3: Apply dependency ordering and emit final merge order
	level3Start := time.Now()
	finalCandidates, ordering, rejections := hp.applyDependencyOrdering(allCandidates, prs)
	level3Duration := time.Since(level3Start)

	totalDuration := time.Since(startTime)

	// Calculate complexity reduction
	complexityReduction := hp.calculateComplexityReduction(len(prs), len(clusterSelections), len(allCandidates))

	// Build telemetry
	telemetry := types.OperationTelemetry{
		PoolStrategy:   "hierarchical_three_level",
		PoolSizeBefore: len(prs),
		PoolSizeAfter:  len(finalCandidates),
		StageLatenciesMS: map[string]int{
			"level1_cluster_selection_ms":   int(level1Duration.Milliseconds()),
			"level2_intra_cluster_rank_ms":  int(level2Duration.Milliseconds()),
			"level3_dependency_ordering_ms": int(level3Duration.Milliseconds()),
			"total_hierarchical_plan_ms":    int(totalDuration.Milliseconds()),
		},
		StageDropCounts: map[string]int{
			"clusters_evaluated":         len(clusterSelections),
			"clusters_selected":          min(len(clusterSelections), hp.maxClusters),
			"candidates_before_ordering": len(allCandidates),
			"candidates_after_ordering":  len(finalCandidates),
			"rejections":                 len(rejections),
		},
	}

	return &HierarchyResult{
		Repo:        repo,
		GeneratedAt: now.Format(time.RFC3339),
		Config: HierarchicalConfig{
			MaxClusters:           hp.maxClusters,
			MaxPerCluster:         hp.maxPerCluster,
			TargetTotal:           hp.targetTotal,
			UseDependencyOrdering: true,
		},
		SelectedClusters:    clusterSelections,
		FinalCandidates:     finalCandidates,
		Ordering:            ordering,
		Rejections:          rejections,
		Telemetry:           telemetry,
		ComplexityReduction: complexityReduction,
	}
}

// selectClusters performs Level 1: Cluster/batch selection based on aggregate priority scores.
func (hp *HierarchicalPlanner) selectClusters(prs []types.PR, now time.Time, decayConfig TimeDecayConfig) []ClusterSelection {
	// Group PRs by cluster
	clusters := make(map[string][]types.PR)
	unclustered := make([]types.PR, 0)

	for _, pr := range prs {
		if pr.ClusterID != "" {
			clusters[pr.ClusterID] = append(clusters[pr.ClusterID], pr)
		} else {
			unclustered = append(unclustered, pr)
		}
	}

	// Score each cluster based on aggregate priority of its PRs
	type clusterScore struct {
		clusterID       string
		clusterLabel    string
		avgPriority     float64
		prCount         int
		componentScores ComponentScores
	}

	scores := make([]clusterScore, 0, len(clusters))

	for clusterID, clusterPRs := range clusters {
		// Calculate aggregate priority for this cluster
		var totalPriority float64
		var sumComponents ComponentScores

		for _, pr := range clusterPRs {
			scores := hp.poolSelector.calculateComponentScores(pr, now, decayConfig)
			priority := hp.poolSelector.computeWeightedScore(scores)

			totalPriority += priority
			sumComponents.StalenessScore += scores.StalenessScore
			sumComponents.CIStatusScore += scores.CIStatusScore
			sumComponents.SecurityLabelScore += scores.SecurityLabelScore
			sumComponents.ClusterScore += scores.ClusterScore
			sumComponents.TimeDecayScore += scores.TimeDecayScore
		}

		avgPriority := totalPriority / float64(len(clusterPRs))
		sumComponents.StalenessScore /= float64(len(clusterPRs))
		sumComponents.CIStatusScore /= float64(len(clusterPRs))
		sumComponents.SecurityLabelScore /= float64(len(clusterPRs))
		sumComponents.ClusterScore /= float64(len(clusterPRs))
		sumComponents.TimeDecayScore /= float64(len(clusterPRs))

		// Determine cluster label from first PR
		clusterLabel := clusterID
		if len(clusterPRs) > 0 && clusterPRs[0].ClusterID != "" {
			clusterLabel = clusterPRs[0].ClusterID
		}

		scores = append(scores, clusterScore{
			clusterID:       clusterID,
			clusterLabel:    clusterLabel,
			avgPriority:     avgPriority,
			prCount:         len(clusterPRs),
			componentScores: sumComponents,
		})
	}

	// Sort clusters by average priority (descending)
	sort.Slice(scores, func(i, j int) bool {
		if math.Abs(scores[i].avgPriority-scores[j].avgPriority) > 0.0001 {
			return scores[i].avgPriority > scores[j].avgPriority
		}
		return scores[i].clusterID < scores[j].clusterID
	})

	// Select top clusters
	selected := make([]ClusterSelection, 0, min(len(scores), hp.maxClusters))
	for i, sc := range scores {
		if i >= hp.maxClusters {
			break
		}

		// Generate reason codes for cluster selection
		reasonCodes := []string{"high_priority_cluster"}
		if sc.componentScores.SecurityLabelScore > 0.5 {
			reasonCodes = append(reasonCodes, "security_relevant")
		}
		if sc.componentScores.CIStatusScore > 0.7 {
			reasonCodes = append(reasonCodes, "ci_passing_majority")
		}
		if sc.prCount >= 5 {
			reasonCodes = append(reasonCodes, "large_cluster")
		}

		selected = append(selected, ClusterSelection{
			ClusterID:       sc.clusterID,
			ClusterLabel:    sc.clusterLabel,
			AveragePriority: sc.avgPriority,
			PRCount:         sc.prCount,
			SelectedCount:   min(sc.prCount, hp.maxPerCluster),
			ReasonCodes:     reasonCodes,
		})
	}

	return selected
}

// rankWithinClusters performs Level 2: Rank PRs within selected clusters.
func (hp *HierarchicalPlanner) rankWithinClusters(clusterSelections []ClusterSelection, allPRs []types.PR, now time.Time, decayConfig TimeDecayConfig) []HierarchicalCandidate {
	// Build cluster ID to selection map
	clusterMap := make(map[string]ClusterSelection)
	for _, sel := range clusterSelections {
		clusterMap[sel.ClusterID] = sel
	}

	// Group PRs by cluster
	clusterPRs := make(map[string][]types.PR)
	for _, pr := range allPRs {
		if _, ok := clusterMap[pr.ClusterID]; ok {
			clusterPRs[pr.ClusterID] = append(clusterPRs[pr.ClusterID], pr)
		}
	}

	// Score and rank PRs within each cluster
	allCandidates := make([]HierarchicalCandidate, 0)

	for _, selection := range clusterSelections {
		clusterID := selection.ClusterID
		prs := clusterPRs[clusterID]

		// Score all PRs in this cluster
		type prWithScore struct {
			pr              types.PR
			priorityScore   float64
			componentScores ComponentScores
			reasonCodes     []string
		}

		scored := make([]prWithScore, 0, len(prs))
		for _, pr := range prs {
			scores := hp.poolSelector.calculateComponentScores(pr, now, decayConfig)
			priority := hp.poolSelector.computeWeightedScore(scores)
			reasons := hp.poolSelector.generateReasonCodes(pr, scores)

			scored = append(scored, prWithScore{
				pr:              pr,
				priorityScore:   priority,
				componentScores: scores,
				reasonCodes:     reasons,
			})
		}

		// Sort by priority score (descending), then by PR number (ascending) for determinism
		sort.Slice(scored, func(i, j int) bool {
			if math.Abs(scored[i].priorityScore-scored[j].priorityScore) > 0.0001 {
				return scored[i].priorityScore > scored[j].priorityScore
			}
			return scored[i].pr.Number < scored[j].pr.Number
		})

		// Select top PRs from this cluster
		maxSelect := min(len(scored), selection.SelectedCount)
		for rank := 0; rank < maxSelect; rank++ {
			ps := scored[rank]
			allCandidates = append(allCandidates, HierarchicalCandidate{
				PR:              ps.pr,
				ClusterID:       clusterID,
				PriorityScore:   ps.priorityScore,
				Level1Rank:      0, // Will be set later based on cluster rank
				Level2Rank:      rank + 1,
				Level3Rank:      0, // Will be set at Level 3
				ComponentScores: ps.componentScores,
				ReasonCodes:     ps.reasonCodes,
			})
		}
	}

	// Sort all candidates by cluster priority (Level 1 rank), then by intra-cluster rank (Level 2)
	// This ensures candidates from higher-priority clusters come first
	sort.Slice(allCandidates, func(i, j int) bool {
		// Find cluster ranks
		clusterIRank := hp.getClusterRank(allCandidates[i].ClusterID, clusterSelections)
		clusterJRank := hp.getClusterRank(allCandidates[j].ClusterID, clusterSelections)

		if clusterIRank != clusterJRank {
			return clusterIRank < clusterJRank
		}
		return allCandidates[i].Level2Rank < allCandidates[j].Level2Rank
	})

	// Set Level 1 ranks based on cluster selection order
	for i := range allCandidates {
		allCandidates[i].Level1Rank = hp.getClusterRank(allCandidates[i].ClusterID, clusterSelections)
	}

	return allCandidates
}

// getClusterRank returns the rank of a cluster in the selection order (0-based).
func (hp *HierarchicalPlanner) getClusterRank(clusterID string, selections []ClusterSelection) int {
	for i, sel := range selections {
		if sel.ClusterID == clusterID {
			return i
		}
	}
	return len(selections)
}

// applyDependencyOrdering performs Level 3: Apply dependency constraints and emit final merge ordering.
func (hp *HierarchicalPlanner) applyDependencyOrdering(candidates []HierarchicalCandidate, allPRs []types.PR) ([]HierarchicalCandidate, []HierarchicalCandidate, []HierarchyRejection) {
	if len(candidates) == 0 {
		return nil, nil, nil
	}

	// Build a subgraph of selected PRs for dependency analysis
	selectedPRNumbers := make(map[int]struct{})
	prByNumber := make(map[int]types.PR)
	candidateByPR := make(map[int]HierarchicalCandidate)

	for _, c := range candidates {
		selectedPRNumbers[c.PR.Number] = struct{}{}
		prByNumber[c.PR.Number] = c.PR
		candidateByPR[c.PR.Number] = c
	}

	// Build dependency graph for selected PRs
	depGraph := graph.Build("hierarchical_subset", allPRs)

	// Filter edges to only include selected PRs
	filteredEdges := make([]types.GraphEdge, 0)
	for _, edge := range depGraph.Edges {
		_, fromSelected := selectedPRNumbers[edge.FromPR]
		_, toSelected := selectedPRNumbers[edge.ToPR]
		if fromSelected && toSelected {
			filteredEdges = append(filteredEdges, edge)
		}
	}

	// Create a minimal graph for topological sort
	subGraph := graph.Graph{
		Repo:  "hierarchical_subset",
		Nodes: make([]types.GraphNode, 0, len(candidates)),
		Edges: filteredEdges,
	}

	for _, c := range candidates {
		subGraph.Nodes = append(subGraph.Nodes, types.GraphNode{
			PRNumber:  c.PR.Number,
			Title:     c.PR.Title,
			ClusterID: c.ClusterID,
			CIStatus:  c.PR.CIStatus,
		})
	}

	// Try topological sort if dependency ordering is enabled
	var ordered []HierarchicalCandidate
	var rejections []HierarchyRejection

	if hp.useDependencyOrdering() {
		orderedNodes, err := subGraph.TopologicalOrder()
		if err != nil {
			// Cycle detected - fall back to priority ordering
			ordered = hp.fallbackPriorityOrdering(candidates)
			rejections = append(rejections, HierarchyRejection{
				PRNumber:    -1,
				Reason:      "dependency_cycle_detected_falling_back_to_priority_ordering",
				ReasonCodes: []string{"cycle_detected"},
			})
		} else {
			// Map topological order back to candidates
			ordered = make([]HierarchicalCandidate, 0, len(orderedNodes))
			seen := make(map[int]struct{})

			for _, node := range orderedNodes {
				if c, ok := candidateByPR[node.PRNumber]; ok {
					c.DependencyDepth = len(ordered)
					ordered = append(ordered, c)
					seen[node.PRNumber] = struct{}{}
				}
			}

			// Add any remaining candidates not in the topological order (shouldn't happen, but safety)
			for _, c := range candidates {
				if _, ok := seen[c.PR.Number]; !ok {
					c.DependencyDepth = len(ordered)
					ordered = append(ordered, c)
				}
			}
		}
	} else {
		// No dependency ordering - use priority ordering
		ordered = hp.fallbackPriorityOrdering(candidates)
	}

	// Trim to target total and generate rejections
	if len(ordered) > hp.targetTotal {
		excess := ordered[hp.targetTotal:]
		ordered = ordered[:hp.targetTotal]

		for _, c := range excess {
			rejections = append(rejections, HierarchyRejection{
				PRNumber:    c.PR.Number,
				Reason:      "exceeded_target_total",
				ReasonCodes: append([]string{"target_limit_reached"}, c.ReasonCodes...),
			})
		}
	}

	// Set Level 3 ranks (final ordering positions)
	for i := range ordered {
		ordered[i].Level3Rank = i + 1
	}

	// Final candidates = ordered (already trimmed)
	finalCandidates := ordered

	return finalCandidates, ordered, rejections
}

// useDependencyOrdering returns whether to use dependency-based ordering.
func (hp *HierarchicalPlanner) useDependencyOrdering() bool {
	return hp.depOrderingEnabled
}

// fallbackPriorityOrdering sorts candidates by priority when dependency ordering fails.
func (hp *HierarchicalPlanner) fallbackPriorityOrdering(candidates []HierarchicalCandidate) []HierarchicalCandidate {
	// Make a copy to avoid modifying the input
	ordered := make([]HierarchicalCandidate, len(candidates))
	copy(ordered, candidates)

	// Sort by priority score (descending), then by PR number (ascending)
	sort.Slice(ordered, func(i, j int) bool {
		if math.Abs(ordered[i].PriorityScore-ordered[j].PriorityScore) > 0.0001 {
			return ordered[i].PriorityScore > ordered[j].PriorityScore
		}
		return ordered[i].PR.Number < ordered[j].PR.Number
	})

	return ordered
}

// calculateComplexityReduction computes the theoretical complexity reduction factor.
// Original: O(C(n,k)) where n=total PRs, k=target
// Hierarchical: O(C(clusters,c) × C(avg_cluster_size,s))
func (hp *HierarchicalPlanner) calculateComplexityReduction(totalPRs int, selectedClusters int, candidates int) float64 {
	if totalPRs == 0 || selectedClusters == 0 {
		return 1.0
	}

	// Original complexity (combinatorial)
	originalComplexity := hp.combinatorial(totalPRs, hp.targetTotal)

	// Hierarchical complexity
	clusterComplexity := hp.combinatorial(selectedClusters, hp.maxClusters)
	avgClusterSize := totalPRs / selectedClusters
	if avgClusterSize == 0 {
		avgClusterSize = 1
	}
	intraClusterComplexity := hp.combinatorial(avgClusterSize, hp.maxPerCluster)
	hierarchicalComplexity := float64(clusterComplexity) * float64(intraClusterComplexity)

	if hierarchicalComplexity == 0 {
		return 1.0
	}

	reduction := originalComplexity / hierarchicalComplexity
	if math.IsInf(reduction, 0) || math.IsNaN(reduction) {
		return 1.0e6 // Cap at 1 million for display purposes
	}

	return reduction
}

// combinatorial computes C(n,k) = n! / (k! × (n-k)!)
// Uses logarithms to avoid overflow for large values.
func (hp *HierarchicalPlanner) combinatorial(n, k int) float64 {
	if k > n || k < 0 {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n/2 {
		k = n - k
	}

	// Use logarithms to compute C(n,k)
	logResult := 0.0
	for i := 0; i < k; i++ {
		logResult += math.Log(float64(n - i))
		logResult -= math.Log(float64(i + 1))
	}

	return math.Exp(logResult)
}

// ConvertToMergePlan converts a HierarchyResult to types.MergePlan for API compatibility.
func (hr *HierarchyResult) ConvertToMergePlan() types.MergePlan {
	selected := make([]types.MergePlanCandidate, 0, len(hr.FinalCandidates))
	for _, c := range hr.FinalCandidates {
		selected = append(selected, types.MergePlanCandidate{
			PRNumber:         c.PR.Number,
			Title:            c.PR.Title,
			Score:            c.PriorityScore,
			Rationale:        fmt.Sprintf("L1:%d L2:%d L3:%d cluster:%s", c.Level1Rank, c.Level2Rank, c.Level3Rank, c.ClusterID),
			FilesTouched:     c.PR.FilesChanged,
			ConflictWarnings: []string{},
		})
	}

	ordering := make([]types.MergePlanCandidate, 0, len(hr.Ordering))
	for _, c := range hr.Ordering {
		ordering = append(ordering, types.MergePlanCandidate{
			PRNumber:         c.PR.Number,
			Title:            c.PR.Title,
			Score:            c.PriorityScore,
			Rationale:        fmt.Sprintf("hierarchical_order_depth_%d", c.DependencyDepth),
			FilesTouched:     c.PR.FilesChanged,
			ConflictWarnings: []string{},
		})
	}

	return types.MergePlan{
		PlanID:            "hierarchical_plan",
		Mode:              "hierarchical_three_level",
		FormulaExpression: fmt.Sprintf("C(%d,%d) × C(cluster,%d)", len(hr.FinalCandidates), hr.Config.TargetTotal, hr.Config.MaxPerCluster),
		Selected:          selected,
		Ordering:          ordering,
		TotalScore:        hr.calculateTotalScore(),
		Warnings:          hr.generateWarnings(),
	}
}

func (hr *HierarchyResult) calculateTotalScore() float64 {
	total := 0.0
	for _, c := range hr.FinalCandidates {
		total += c.PriorityScore
	}
	return total
}

func (hr *HierarchyResult) generateWarnings() []string {
	warnings := make([]string, 0)

	if len(hr.Rejections) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d candidates rejected due to constraints", len(hr.Rejections)))
	}

	if hr.ComplexityReduction < 10 {
		warnings = append(warnings, "low complexity reduction - consider increasing cluster count")
	}

	if hr.ComplexityReduction > 1e6 {
		warnings = append(warnings, "very high complexity reduction achieved")
	}

	return warnings
}

// Error definitions.
var (
	ErrInvalidClusterCount       = &HierarchyError{"max_clusters must be > 0"}
	ErrInvalidPerClusterCount    = &HierarchyError{"max_per_cluster must be > 0"}
	ErrInvalidTargetTotal        = &HierarchyError{"target_total must be > 0"}
	ErrInsufficientCandidatePool = &HierarchyError{"candidate pool (max_clusters × max_per_cluster) must be >= target_total"}
)

// HierarchyError represents a hierarchical planning error.
type HierarchyError struct {
	msg string
}

func (e *HierarchyError) Error() string {
	return e.msg
}

var _ error = (*HierarchyError)(nil)
