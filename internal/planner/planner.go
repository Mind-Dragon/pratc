package planner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/filter"
	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/graph"
	"github.com/jeffersonnunn/pratc/internal/planning"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/util"
)

// Planner orchestrates the planning pipeline by coordinating filter, formula, and graph engines.
type Planner struct {
	filterPipeline      *filter.Pipeline
	formulaConfig       formula.Config
	now                 func() time.Time
	validator           *PlanInputValidator
	includeBots         bool
	poolSelector        *planning.PoolSelector
	hierarchicalPlanner *planning.HierarchicalPlanner
	pairwiseExecutor    *planning.PairwiseExecutor
	timeDecayWindow     *planning.TimeDecayWindow
}

// Option configures a Planner.
type Option func(*Planner)

// WithNow sets the time function for the planner.
func WithNow(now time.Time) Option {
	return func(p *Planner) {
		p.now = func() time.Time { return now }
	}
}

// WithValidator sets a custom validator for the planner.
func WithValidator(v *PlanInputValidator) Option {
	return func(p *Planner) {
		p.validator = v
	}
}

// WithIncludeBots sets whether bot PRs should be included in the candidate pool.
func WithIncludeBots(includeBots bool) Option {
	return func(p *Planner) {
		p.includeBots = includeBots
	}
}

// WithPoolSelector sets the pool selector for cluster coherence-aware candidate selection.
func WithPoolSelector(poolSelector *planning.PoolSelector) Option {
	return func(p *Planner) {
		p.poolSelector = poolSelector
	}
}

// WithHierarchicalPlanner sets the hierarchical planner for three-level planning.
func WithHierarchicalPlanner(hp *planning.HierarchicalPlanner) Option {
	return func(p *Planner) {
		p.hierarchicalPlanner = hp
	}
}

// WithPairwiseExecutor sets the pairwise executor for sharded conflict detection.
func WithPairwiseExecutor(pe *planning.PairwiseExecutor) Option {
	return func(p *Planner) {
		p.pairwiseExecutor = pe
	}
}

// WithTimeDecayWindow sets the time-decay window for enhanced decay telemetry.
func WithTimeDecayWindow(tdw *planning.TimeDecayWindow) Option {
	return func(p *Planner) {
		p.timeDecayWindow = tdw
	}
}

// New creates a new Planner with optional configuration.
func New(opts ...Option) *Planner {
	p := &Planner{
		now:         func() time.Time { return time.Now().UTC() },
		validator:   NewPlanInputValidator(),
		includeBots: false,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Plan executes the planning pipeline and returns a merge plan.
func (p *Planner) Plan(ctx context.Context, repo string, prs []types.PR, target int, mode formula.Mode) (types.PlanResponse, error) {
	// Apply defaults
	if target <= 0 {
		target = 20
	}
	if mode == "" {
		mode = formula.ModeCombination
	}

	// Validate input
	if err := p.validator.ValidatePlanInput(target, nil); err != nil {
		return types.PlanResponse{}, fmt.Errorf("invalid plan input: %w", err)
	}

	// Build clusters using title similarity
	clusters := p.BuildClusters(prs)
	clusterByPR := make(map[int]string)
	for _, cluster := range clusters {
		for _, prID := range cluster.PRIDs {
			clusterByPR[prID] = cluster.ClusterID
		}
	}

	// Use hierarchical planning if configured
	if p.hierarchicalPlanner != nil {
		// Assign cluster IDs to each PR for hierarchical planning
		prsWithClusters := make([]types.PR, len(prs))
		copy(prsWithClusters, prs)
		for i := range prsWithClusters {
			if cid, ok := clusterByPR[prsWithClusters[i].Number]; ok {
				prsWithClusters[i].ClusterID = cid
			}
		}

		// Execute hierarchical planning
		hierarchyResult := p.hierarchicalPlanner.Plan(ctx, repo, prsWithClusters, planning.DefaultTimeDecayConfig())

		// Convert hierarchical result to PlanResponse
		selected := make([]types.MergePlanCandidate, 0, len(hierarchyResult.FinalCandidates))
		for _, c := range hierarchyResult.FinalCandidates {
			files := c.PR.FilesChanged
			if files == nil {
				files = []string{}
			}
			selected = append(selected, types.MergePlanCandidate{
				PRNumber:         c.PR.Number,
				Title:            c.PR.Title,
				Score:            p.round(c.PriorityScore, 4),
				Rationale:        fmt.Sprintf("L1:%d L2:%d L3:%d cluster:%s", c.Level1Rank, c.Level2Rank, c.Level3Rank, c.ClusterID),
				FilesTouched:     files,
				ConflictWarnings: []string{},
			})
		}

		ordering := make([]types.MergePlanCandidate, 0, len(hierarchyResult.Ordering))
		for _, c := range hierarchyResult.Ordering {
			files := c.PR.FilesChanged
			if files == nil {
				files = []string{}
			}
			ordering = append(ordering, types.MergePlanCandidate{
				PRNumber:         c.PR.Number,
				Title:            c.PR.Title,
				Score:            p.round(c.PriorityScore, 4),
				Rationale:        fmt.Sprintf("hierarchical_order_depth_%d", c.DependencyDepth),
				FilesTouched:     files,
				ConflictWarnings: []string{},
			})
		}

		rejections := make([]types.PlanRejection, 0, len(hierarchyResult.Rejections))
		for _, r := range hierarchyResult.Rejections {
			rejections = append(rejections, types.PlanRejection{
				PRNumber: r.PRNumber,
				Reason:   r.Reason,
			})
		}

		return types.PlanResponse{
			Repo:              repo,
			GeneratedAt:       p.now().Format(time.RFC3339),
			Target:            target,
			CandidatePoolSize: hierarchyResult.Telemetry.PoolSizeAfter,
			Strategy:          "hierarchical_three_level",
			Selected:          selected,
			Ordering:          ordering,
			Rejections:        rejections,
			Telemetry:         &hierarchyResult.Telemetry,
		}, nil
	}

	// Initialize filter pipeline if not set
	if p.filterPipeline == nil {
		p.filterPipeline = filter.NewPipeline(p.now()).WithIncludeBots(p.includeBots)
	}

	// Apply filter pipeline
	pool, rejections := p.filterPipeline.BuildCandidatePool(prs, clusterByPR)

	// Apply time-decay window for enhanced decay telemetry if configured
	var decayStats planning.TimeDecayStats
	if p.timeDecayWindow != nil && len(pool) > 0 {
		tdw := planning.NewTimeDecayWindow(pool, p.timeDecayWindow.Config(), p.now())
		decayStats = tdw.GetWindowStats()
	}

	// Apply pool selector for cluster coherence-aware selection if configured
	if p.poolSelector != nil {
		poolResult := p.poolSelector.SelectCandidatesWithClusterCoherence(ctx, repo, pool, target, planning.DefaultTimeDecayConfig())
		// Extract PRs from selected candidates
		selectedPRs := make([]types.PR, 0, len(poolResult.Selected))
		for _, candidate := range poolResult.Selected {
			selectedPRs = append(selectedPRs, candidate.PR)
		}
		// Update pool with selected PRs
		pool = selectedPRs
		// Track rejections from pool selector
		for _, excluded := range poolResult.Excluded {
			rejections = append(rejections, types.PlanRejection{
				PRNumber: excluded.PR.Number,
				Reason:   excluded.Reason,
			})
		}
	}

	// Handle empty pool case
	if len(pool) == 0 {
		return types.PlanResponse{
			Repo:              repo,
			GeneratedAt:       p.now().Format(time.RFC3339),
			Target:            target,
			CandidatePoolSize: 0,
			Strategy:          "formula+graph",
			Selected:          nil,
			Ordering:          nil,
			Rejections:        rejections,
			Telemetry: &types.OperationTelemetry{
				PoolSizeBefore:   len(prs),
				PoolSizeAfter:    0,
				PoolStrategy:     "formula+graph",
				StageLatenciesMS: make(map[string]int),
				StageDropCounts:  make(map[string]int),
			},
		}, nil
	}

	// Calculate pick count
	pickCount := target
	if pickCount > len(pool) && mode != formula.ModeWithReplacement {
		pickCount = len(pool)
	}

	// Initialize formula config if not set
	if p.formulaConfig.Mode == "" {
		p.formulaConfig = formula.DefaultConfig()
	}
	p.formulaConfig.Mode = mode

	// Execute formula engine search
	engine := formula.NewEngine(p.formulaConfig)
	searchResult, err := engine.Search(formula.SearchInput{
		Pool:        pool,
		Target:      pickCount,
		PreFiltered: true,
		Now:         p.now(),
	})
	if err != nil {
		return types.PlanResponse{}, fmt.Errorf("plan search: %w", err)
	}

	// Deduplicate: formula engine may return duplicates in with_replacement mode
	rawSelected := searchResult.Best.Selected
	seen := make(map[int]struct{}, len(rawSelected))
	selectedPRs := make([]types.PR, 0, len(rawSelected))
	for _, pr := range rawSelected {
		if _, ok := seen[pr.Number]; ok {
			continue
		}
		seen[pr.Number] = struct{}{}
		selectedPRs = append(selectedPRs, pr)
	}

	// Track unselected PRs as rejections
	selectedByNumber := seen
	for _, pr := range pool {
		if _, ok := selectedByNumber[pr.Number]; !ok {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "not selected by strategy"})
		}
	}

	// Capture pairwise shards from formula engine for telemetry
	pairwiseShards := searchResult.Telemetry.PairwiseShards

	// Build dependency-ordered list
	orderedPRs := p.orderSelection(repo, selectedPRs)

	// Run sharded pairwise conflict detection if executor is configured
	var pairwiseResult *planning.PairwiseResult
	warningsByPR := make(map[int][]string)
	if p.pairwiseExecutor != nil && len(orderedPRs) > 0 {
		pairwiseResult, _ = p.pairwiseExecutor.ExecuteSharded(ctx, repo, orderedPRs)
		if pairwiseResult != nil {
			for _, conflict := range pairwiseResult.Conflicts {
				msg := fmt.Sprintf("pairwise_conflict: %s with PR #%d", conflict.ConflictType, conflict.TargetPR)
				warningsByPR[conflict.SourcePR] = p.appendUniqueString(warningsByPR[conflict.SourcePR], msg)
			}
			// Use pairwise executor's shard count if available
			if pairwiseResult.Telemetry.PairwiseShards > pairwiseShards {
				pairwiseShards = pairwiseResult.Telemetry.PairwiseShards
			}
		}
	}

	// Build selected candidates
	selected := make([]types.MergePlanCandidate, 0, len(selectedPRs))
	for _, pr := range selectedPRs {
		selected = append(selected, p.candidateFromPR(pr, filter.PlannerPriority(pr, p.now()), nil))
	}

	// Build conflict warnings (merge graph-based and pairwise)
	graphWarnings := p.buildConflictWarnings(repo, orderedPRs)
	for prNum, warns := range graphWarnings {
		for _, w := range warns {
			warningsByPR[prNum] = p.appendUniqueString(warningsByPR[prNum], w)
		}
	}

	// Build ordering with warnings
	ordering := make([]types.MergePlanCandidate, 0, len(orderedPRs))
	for _, pr := range orderedPRs {
		ordering = append(ordering, p.candidateFromPR(pr, filter.PlannerPriority(pr, p.now()), warningsByPR[pr.Number]))
	}

	// Sort rejections deterministically
	sort.Slice(rejections, func(i, j int) bool {
		return rejections[i].PRNumber < rejections[j].PRNumber
	})

	// Build telemetry, including pairwise shard metrics and time-decay stats if available
	stageLatencies := make(map[string]int)
	stageDropCounts := make(map[string]int)

	// Always aggregate filter-stage rejections into stage drop counts
	for _, r := range rejections {
		stageDropCounts[r.Reason]++
	}

	if pairwiseResult != nil {
		if lat, ok := pairwiseResult.Telemetry.StageLatenciesMS["pairwise_total_ms"]; ok {
			stageLatencies["pairwise_sharded"] = lat
		}
		if cnt, ok := pairwiseResult.Telemetry.StageDropCounts["conflicts_found"]; ok {
			stageDropCounts["pairwise_conflicts"] = cnt
		}
		stageDropCounts["pairwise_shards"] = pairwiseResult.Telemetry.PairwiseShards
	}
	// Add time-decay window telemetry if configured
	if p.timeDecayWindow != nil {
		stageDropCounts["decay_eligible"] = decayStats.EligibleCount
		stageDropCounts["decay_protected"] = decayStats.ProtectedCount
		stageDropCounts["decay_min"] = int(decayStats.DecayMin * 1000)
		stageDropCounts["decay_max"] = int(decayStats.DecayMax * 1000)
		stageDropCounts["decay_avg"] = int(decayStats.DecayAvg * 1000)
	}

	return types.PlanResponse{
		Repo:              repo,
		GeneratedAt:       p.now().Format(time.RFC3339),
		Target:            target,
		CandidatePoolSize: len(pool),
		Strategy:          "formula+graph",
		Selected:          selected,
		Ordering:          ordering,
		Rejections:        rejections,
		Telemetry: &types.OperationTelemetry{
			PoolSizeBefore:   len(prs),
			PoolSizeAfter:    len(pool),
			PoolStrategy:     "formula+graph",
			PairwiseShards:  pairwiseShards,
			StageLatenciesMS: stageLatencies,
			StageDropCounts:  stageDropCounts,
		},
	}, nil
}

// BuildClusters groups PRs into clusters based on branch, labels, or bot status.
func (p *Planner) BuildClusters(prs []types.PR) []types.PRCluster {
	clusterMap := make(map[string][]types.PR)
	for _, pr := range prs {
		key := p.clusterKey(pr)
		clusterMap[key] = append(clusterMap[key], pr)
	}

	keys := make([]string, 0, len(clusterMap))
	for key := range clusterMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	clusters := make([]types.PRCluster, 0, len(keys))
	for i, key := range keys {
		members := clusterMap[key]
		sort.Slice(members, func(a, b int) bool { return members[a].Number < members[b].Number })

		prIDs := make([]int, 0, len(members))
		titles := make([]string, 0, min(3, len(members)))
		health := "green"
		for _, member := range members {
			prIDs = append(prIDs, member.Number)
			if len(titles) < 3 {
				titles = append(titles, member.Title)
			}
			if member.Mergeable == "conflicting" || member.CIStatus == "failure" {
				health = "red"
			} else if health != "red" && (member.CIStatus == "pending" || member.CIStatus == "unknown") {
				health = "yellow"
			}
		}

		clusters = append(clusters, types.PRCluster{
			ClusterID:         fmt.Sprintf("%s-%02d", p.sanitizeClusterID(key), i+1),
			ClusterLabel:      key,
			Summary:           fmt.Sprintf("%d pull requests grouped by %s", len(members), key),
			PRIDs:             prIDs,
			HealthStatus:      health,
			AverageSimilarity: p.averageTitleSimilarity(members),
			SampleTitles:      titles,
		})
	}

	return clusters
}

// clusterKey determines the cluster key for a PR.
func (p *Planner) clusterKey(pr types.PR) string {
	if pr.IsBot || p.containsLabel(pr.Labels, "dependencies") || p.containsLabel(pr.Labels, "dependabot") {
		return "dependency updates"
	}
	if pr.BaseBranch != "" && pr.BaseBranch != "main" {
		return "branch " + pr.BaseBranch
	}
	if len(pr.Labels) > 0 {
		return pr.Labels[0]
	}
	parts := util.Tokenize(pr.Title)
	if len(parts) == 0 {
		return "general"
	}
	if len(parts) > 2 {
		return parts[0] + " " + parts[1]
	}
	return stringsJoin(parts, " ")
}

// orderSelection orders selected PRs using topological sort based on dependencies.
func (p *Planner) orderSelection(repo string, selected []types.PR) []types.PR {
	if len(selected) == 0 {
		return nil
	}

	g := graph.Build(repo, selected)
	orderedNodes, err := g.TopologicalOrder()
	if err != nil {
		// Cycle detected - fall back to PR number ordering
		cloned := make([]types.PR, len(selected))
		copy(cloned, selected)
		sort.Slice(cloned, func(i, j int) bool { return cloned[i].Number < cloned[j].Number })
		return cloned
	}

	byNumber := make(map[int]types.PR, len(selected))
	for _, pr := range selected {
		byNumber[pr.Number] = pr
	}

	ordered := make([]types.PR, 0, len(selected))
	for _, node := range orderedNodes {
		ordered = append(ordered, byNumber[node.PRNumber])
	}

	return ordered
}

// buildConflictWarnings builds conflict warnings for selected PRs.
func (p *Planner) buildConflictWarnings(repo string, selected []types.PR) map[int][]string {
	warnings := make(map[int][]string)
	g := graph.Build(repo, selected)
	for _, edge := range g.Edges {
		if edge.EdgeType != graph.EdgeTypeConflict {
			continue
		}
		message := fmt.Sprintf("Conflicts with PR #%d", edge.ToPR)
		warnings[edge.FromPR] = p.appendUniqueString(warnings[edge.FromPR], message)
		message = fmt.Sprintf("Conflicts with PR #%d", edge.FromPR)
		warnings[edge.ToPR] = p.appendUniqueString(warnings[edge.ToPR], message)
	}

	for prNumber := range warnings {
		sort.Strings(warnings[prNumber])
	}

	return warnings
}

// candidateFromPR creates a MergePlanCandidate from a PR.
func (p *Planner) candidateFromPR(pr types.PR, score float64, warnings []string) types.MergePlanCandidate {
	files := pr.FilesChanged
	if files == nil {
		files = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}

	return types.MergePlanCandidate{
		PRNumber:         pr.Number,
		Title:            pr.Title,
		Score:            p.round(score, 4),
		Rationale:        filter.PlannerRationale(pr),
		FilesTouched:     files,
		ConflictWarnings: warnings,
	}
}

// averageTitleSimilarity calculates the average title similarity among PRs.
func (p *Planner) averageTitleSimilarity(prs []types.PR) float64 {
	if len(prs) <= 1 {
		return 1
	}

	total := 0.0
	pairs := 0.0
	for i := 0; i < len(prs); i++ {
		for j := i + 1; j < len(prs); j++ {
			total += util.Jaccard(util.Tokenize(prs[i].Title), util.Tokenize(prs[j].Title))
			pairs++
		}
	}
	if pairs == 0 {
		return 1
	}

	return p.round(total/pairs, 4)
}

// Helper functions

func (p *Planner) containsLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

func (p *Planner) round(value float64, places int) float64 {
	multiplier := 1.0
	for i := 0; i < places; i++ {
		multiplier *= 10
	}
	return float64(int(value*multiplier+0.5)) / multiplier
}

func (p *Planner) appendUniqueString(slice []string, value string) []string {
	for _, v := range slice {
		if v == value {
			return slice
		}
	}
	return append(slice, value)
}

func (p *Planner) sanitizeClusterID(key string) string {
	result := make([]byte, 0, len(key))
	for _, r := range []byte(key) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result = append(result, r)
		} else if r == ' ' {
			result = append(result, '-')
		}
	}
	return string(result)
}

func stringsJoin(slice []string, sep string) string {
	if len(slice) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(slice[0])
	for _, s := range slice[1:] {
		b.WriteString(sep)
		b.WriteString(s)
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
