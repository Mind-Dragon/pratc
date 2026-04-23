// Package app provides the service layer for prATC.
//
// v1.3 SCOPE BOUNDARIES - ADVISORY-ONLY GUARANTEE:
//
// This package implements v1.3 of prATC, which is strictly advisory-only.
// The service layer NEVER performs GitHub mutations or automated actions.
//
// v1.3 MAY:
//   - RECOMMEND merge actions (via ReviewPayload.NextAction)
//   - PRIORITIZE PRs for review (via PriorityTier)
//   - QUARANTINE problematic PRs in reports (via ReviewCategoryProblematic)
//   - ESCALATE uncertain/high-risk PRs (via ReviewCategory)
//
// v1.3 MUST NOT:
//   - Auto-merge PRs (no GitHub merge API calls)
//   - Auto-approve PRs (no GitHub review submission)
//   - Post review decisions back to GitHub as actions
//   - Use GitHub Apps, OAuth flows, or webhooks
//   - Make automated decisions without human review
//
// All GitHub operations are read-only: FetchPullRequests, FetchPullRequestFiles.
// Authentication is token-based only (GITHUB_TOKEN/GH_TOKEN/GITHUB_PAT or gh CLI auth).
//
// Future versions (1.4+) may introduce automation with explicit opt-in only.
package app

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/formula"
	gh "github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/graph"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/ml"
	"github.com/jeffersonnunn/pratc/internal/planner"
	"github.com/jeffersonnunn/pratc/internal/planning"
	"github.com/jeffersonnunn/pratc/internal/repo"
	"github.com/jeffersonnunn/pratc/internal/review"
	"github.com/jeffersonnunn/pratc/internal/settings"
	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/util"
	"github.com/jeffersonnunn/pratc/internal/version"
)

// AnalyzeProgress is called during analysis to report phase-level progress.
// phase is one of: "loading", "clusters", "duplicates", "conflicts", "staleness", "review", "review_pr", "done".
// done/total are meaningful only for "review_pr" (iterating per-PR review).
type AnalyzeProgress func(phase string, done, total int)

type Config struct {
	Now                     func() time.Time
	AllowLive               bool
	AllowForceCache         bool
	UseCacheFirst           bool
	IncludeReview           bool
	Token                   string
	TokenSource             gh.TokenSource
	MaxPRs                  int
	BeginningPRNumber       int
	EndingPRNumber          int
	PrecisionMode           string
	DeepCandidateSubsetSize int
	CacheStore              *cache.Store
	PriorityWeights         planning.PriorityWeights
	TimeDecayConfig         planning.TimeDecayConfig
	// PlanningStrategy selects the planning algorithm:
	// "formula" (default) uses combinatorial formula engine after PoolSelector scoring.
	// "hierarchical" uses HierarchicalPlanner for 3-level cluster-based planning.
	PlanningStrategy string
	// OnAnalyzeProgress is an optional callback for per-phase progress reporting.
	OnAnalyzeProgress AnalyzeProgress
	// CollapseDuplicates enables duplicate group collapse before planning.
	// When true, superseded PRs are replaced with their canonical representative.
	CollapseDuplicates bool
	// DynamicTarget configures dynamic merge plan target calculation (v1.6.1).
	// When Enabled, target is computed as Ratio*len(pool) clamped to [MinTarget, MaxTarget].
	DynamicTarget DynamicTargetConfig
}

// DynamicTargetConfig configures dynamic merge plan target calculation.
// In v1.6.1, dynamic target is enabled by default.
type DynamicTargetConfig struct {
	Enabled   bool    // default true in v1.6.1
	Ratio     float64 // default 0.05 (5% of viable pool)
	MinTarget int     // default 20
	MaxTarget int     // default 100
}

type Service struct {
	now                     func() time.Time
	allowLive               bool
	allowForceCache         bool
	useCacheFirst           bool
	includeReview           bool
	token                   string
	tokenSource             gh.TokenSource
	maxPRs                  int
	beginningPRNumber       int
	endingPRNumber          int
	precisionMode           string
	deepCandidateSubsetSize int
	mlBridge                *ml.Bridge
	cacheStore              *cache.Store
	cacheTTL                time.Duration
	mirrorBaseDir           string
	mirrorAvailable         bool
	poolSelector            *planning.PoolSelector
	timeDecayConfig         planning.TimeDecayConfig
	hierarchicalPlanner     *planning.HierarchicalPlanner
	pairwiseExecutor        *planning.PairwiseExecutor
	planningStrategy        string // "formula" (default) or "hierarchical"
	onAnalyzeProgress       AnalyzeProgress
	collapseDuplicates      bool
	dynamicTarget           DynamicTargetConfig
}

// PlanOptions controls plan behavior. Zero values mean "use default".
type PlanOptions struct {
	Target              int
	Mode                formula.Mode
	ExcludeConflicts    bool
	CandidatePoolCap    int
	ScoreMin            float64
	StaleScoreThreshold float64
	CollapseDuplicates  bool
	DryRun              bool
}

const (
	precisionModeFast = "fast"
	precisionModeDeep = "deep"
)

func NewService(cfg Config) Service {
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	token := strings.TrimSpace(cfg.Token)

	if token == "" {
		if resolved, err := gh.ResolveToken(context.Background()); err == nil {
			token = resolved
		}
	}

	allowLive := cfg.AllowLive

	maxPRs := cfg.MaxPRs

	deepCandidateSubsetSize := cfg.DeepCandidateSubsetSize
	if deepCandidateSubsetSize <= 0 {
		deepCandidateSubsetSize = 64
	}

	useCacheFirst := cfg.UseCacheFirst
	includeReview := cfg.IncludeReview

	var cacheStore *cache.Store
	if cfg.CacheStore != nil {
		cacheStore = cfg.CacheStore
	} else if useCacheFirst {
		dbPath := os.Getenv("PRATC_DB_PATH")
		if dbPath == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				dbPath = filepath.Join(home, ".pratc", "pratc.db")
			}
		}
		if dbPath != "" {
			cacheStore, _ = cache.Open(dbPath)
		}
	}

	mirrorBaseDir, mirrorErr := repo.DefaultBaseDir()
	mirrorAvailable := mirrorErr == nil && mirrorBaseDir != ""

	cacheTTL := time.Hour
	if ttlStr := os.Getenv("PRATC_CACHE_TTL"); ttlStr != "" {
		if parsed, err := time.ParseDuration(ttlStr); err == nil {
			cacheTTL = parsed
		}
	}

	return Service{
		now:                     now,
		allowLive:               allowLive,
		allowForceCache:         cfg.AllowForceCache,
		useCacheFirst:           useCacheFirst,
		includeReview:           includeReview,
		token:                   token,
		tokenSource:             cfg.TokenSource,
		maxPRs:                  maxPRs,
		beginningPRNumber:       cfg.BeginningPRNumber,
		endingPRNumber:          cfg.EndingPRNumber,
		precisionMode:           normalizePrecisionMode(cfg.PrecisionMode),
		deepCandidateSubsetSize: deepCandidateSubsetSize,
		mlBridge:                ml.NewBridge(ml.Config{}),
		cacheStore:              cacheStore,
		cacheTTL:                cacheTTL,
		mirrorBaseDir:           mirrorBaseDir,
		mirrorAvailable:         mirrorAvailable,
		poolSelector:            mustNewPoolSelector(cfg.PriorityWeights),
		timeDecayConfig:         resolveTimeDecayConfig(cfg.TimeDecayConfig),
		hierarchicalPlanner:     mustNewHierarchicalPlanner(mustNewPoolSelector(cfg.PriorityWeights)),
		pairwiseExecutor:        mustNewPairwiseExecutor(),
		planningStrategy:        resolvePlanningStrategy(cfg.PlanningStrategy),
		onAnalyzeProgress:       cfg.OnAnalyzeProgress,
		collapseDuplicates:      cfg.CollapseDuplicates,
		dynamicTarget:           resolveDynamicTargetConfig(cfg.DynamicTarget),
	}
}

func normalizePrecisionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case precisionModeDeep:
		return precisionModeDeep
	default:
		return precisionModeFast
	}
}

// newPoolSelectorFromConfig creates a PoolSelector from PriorityWeights.
func newPoolSelectorFromConfig(weights planning.PriorityWeights) *planning.PoolSelector {
	ps, err := planning.NewPoolSelector(weights)
	if err != nil {
		return planning.NewPoolSelectorWithDefaults()
	}
	return ps
}

// mustNewPoolSelector creates a PoolSelector, panics on error.
func mustNewPoolSelector(weights planning.PriorityWeights) *planning.PoolSelector {
	// Zero-valued weights are invalid; fall back to defaults.
	if weights.StalenessWeight == 0 && weights.CIStatusWeight == 0 &&
		weights.SecurityLabelWeight == 0 && weights.ClusterCoherenceWeight == 0 &&
		weights.TimeDecayWeight == 0 {
		return planning.NewPoolSelectorWithDefaults()
	}
	ps, err := planning.NewPoolSelector(weights)
	if err != nil {
		return planning.NewPoolSelectorWithDefaults()
	}
	return ps
}

// resolveTimeDecayConfig returns the TimeDecayConfig as-is.
func resolveTimeDecayConfig(cfg planning.TimeDecayConfig) planning.TimeDecayConfig {
	return cfg
}

// resolvePlanningStrategy normalises the planning strategy config value.
func resolvePlanningStrategy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hierarchical":
		return "hierarchical"
	default:
		return "formula"
	}
}

func resolveDynamicTargetConfig(cfg DynamicTargetConfig) DynamicTargetConfig {
	if cfg.Ratio <= 0 {
		cfg.Ratio = 0.05
	}
	if cfg.MinTarget <= 0 {
		cfg.MinTarget = 20
	}
	if cfg.MaxTarget <= 0 {
		cfg.MaxTarget = 50
	}
	return cfg
}

func (s Service) hasGitHubAuth() bool {
	return strings.TrimSpace(s.token) != "" || s.tokenSource != nil
}

func fallbackDegradation(reason string) types.DegradationMetadata {
	return types.DegradationMetadata{
		FallbackReason:    reason,
		HeuristicFallback: true,
	}
}

// ComputeDynamicTarget computes the target based on the viable pool size.
// When Enabled is true, target = max(MinTarget, min(MaxTarget, Ratio*len(pool))).
// When Enabled is false, returns the fallback value unchanged.
func (c DynamicTargetConfig) ComputeDynamicTarget(viablePool int, fallback int) int {
	if !c.Enabled {
		return fallback
	}
	if viablePool <= 0 {
		return c.MinTarget
	}
	target := int(float64(viablePool) * c.Ratio)
	if target < c.MinTarget {
		target = c.MinTarget
	}
	if target > c.MaxTarget {
		target = c.MaxTarget
	}
	return target
}

// mustNewHierarchicalPlanner creates a HierarchicalPlanner with default config.
func mustNewHierarchicalPlanner(ps *planning.PoolSelector) *planning.HierarchicalPlanner {
	cfg := planning.DefaultHierarchicalConfig()
	hp, err := planning.NewHierarchicalPlanner(ps, cfg)
	if err != nil {
		// Should not happen with defaults; panic as a safety net.
		panic("invalid HierarchicalPlanner config: " + err.Error())
	}
	return hp
}

// mustNewPairwiseExecutor creates a PairwiseExecutor with default sharded config.
func mustNewPairwiseExecutor() *planning.PairwiseExecutor {
	pe, err := planning.NewPairwiseExecutorWithDefaults()
	if err != nil {
		panic("invalid PairwiseExecutor config: " + err.Error())
	}
	return pe
}

// ProcessOmniBatch is deprecated - use PlanOmni instead
// func (s *Service) ProcessOmniBatch(selector string, stageSize int, target int) (*OmniBatchResult, error) {
// 	expr, err := planning.Parse(selector)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	if s.cacheStore == nil {
// 		return nil, fmt.Errorf("cache unavailable")
// 	}
//
// 	repos, err := s.cacheStore.ListAllRepos()
// 	if err != nil {
// 		return nil, fmt.Errorf("cache: %w", err)
// 	}
//
// 	availableIDs := make([]int, 0)
// 	for _, repoName := range repos {
// 		prs, err := s.cacheStore.ListPRs(cache.PRFilter{Repo: repoName})
// 		if err != nil {
// 			return nil, fmt.Errorf("cache: %w", err)
// 		}
// 		for _, pr := range prs {
// 			availableIDs = append(availableIDs, pr.Number)
// 		}
// 	}
//
// 	bp := NewBatchProcessor(StageConfig{StageSize: stageSize})
// 	stages := bp.Process(expr, availableIDs)
//
// 	var allSelected []int
// 	for _, stage := range stages {
// 		allSelected = append(allSelected, stage.OutputIDs...)
// 	}
//
// 	if target < 0 {
// 		target = 0
// 	}
// 	if len(allSelected) > target {
// 		allSelected = allSelected[:target]
// 	}
//
// 	return &OmniBatchResult{
// 		Selector:   selector,
// 		StageCount: len(stages),
// 		Stages:     stages,
// 		Selected:   allSelected,
// 		Ordering:   allSelected,
// 	}, nil
// }

type truncationMeta struct {
	AnalysisTruncated bool
	TruncationReason  string
	MaxPRsApplied     int
	PRWindow          *types.PRWindow
	LiveSource        bool
}

func (s Service) Health() types.HealthResponse {
	return types.HealthResponse{Status: "ok", Version: version.Version, APIVersion: "v1.6"}
}

// DynamicTarget returns the DynamicTarget configuration.
func (s Service) DynamicTarget() DynamicTargetConfig {
	return s.dynamicTarget
}

// GetActiveSyncJob returns (true, jobID) if there is an active sync job for the given repo.
func (s Service) GetActiveSyncJob(repo string) (bool, string, error) {
	if s.cacheStore == nil {
		return false, "", nil
	}
	job, ok, err := s.cacheStore.ResumeSyncJob(repo)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "", nil
	}
	return true, job.ID, nil
}

func (s Service) emit(phase string, done, total int) {
	if s.onAnalyzeProgress != nil {
		s.onAnalyzeProgress(phase, done, total)
	}
}

func (s Service) Analyze(ctx context.Context, repo string) (types.AnalysisResponse, error) {
	log := logger.New("app")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	startTime := nowFn()
	defer func() {
		durationMs := int(nowFn().Sub(startTime).Milliseconds())
		log.Info("analyze operation completed", "duration_ms", durationMs)
		if durationMs > types.AnalyzeSLOMS {
			log.Error("analyze operation exceeded SLO", "duration_ms", durationMs, "slo_ms", types.AnalyzeSLOMS)
		}
	}()

	prs, repoName, meta, err := s.loadPRs(ctx, repo)
	if err != nil {
		return types.AnalysisResponse{}, err
	}

	s.emit("loading", 0, len(prs))
	log.Info("analysis started", "pr_count", len(prs))

	// Layer 1: Garbage detection (outer peel). Remove obviously bad PRs before deeper analysis.
	garbageStart := time.Now()
	garbage := classifyGarbage(prs)
	garbageElapsed := int(time.Since(garbageStart).Milliseconds())
	s.emit("garbage", len(garbage), len(prs))
	log.Info("garbage classification complete", "total", len(prs), "garbage", len(garbage))

	// Filter garbage PRs from further analysis
	garbageSet := make(map[int]bool, len(garbage))
	for _, g := range garbage {
		garbageSet[g.PRNumber] = true
	}
	analysisPRs := make([]types.PR, 0, len(prs)-len(garbage))
	for _, pr := range prs {
		if !garbageSet[pr.Number] {
			analysisPRs = append(analysisPRs, pr)
		}
	}
	prs = analysisPRs

	telemetry := types.OperationTelemetry{
		PoolStrategy:    "heuristic_analysis_pipeline",
		PoolSizeBefore:  len(prs),
		PoolSizeAfter:   len(prs),
		GraphDeltaEdges: 0,
		DecayPolicy:     "none",
		PairwiseShards:  estimatePairwiseShards(len(prs)),
		StageLatenciesMS: map[string]int{
			"garbage_ms": garbageElapsed,
		},
		StageDropCounts: map[string]int{},
	}

	clusterStart := time.Now()
	clusters := planner.New().BuildClusters(prs)
	clusterModel := ""
	clusterDegradation := types.DegradationMetadata{}
	telemetry.StageLatenciesMS["clusters_ms"] = int(time.Since(clusterStart).Milliseconds())
	s.emit("clusters", len(clusters), len(prs))

	// Attempt ML-backed clustering via Voyage if configured.
	// Skip ML bridge in force-cache mode to avoid subprocess overhead.
	if !s.allowForceCache && s.mlBridge != nil {
		clusterModel = "heuristic-fallback"
		if !s.mlBridge.Available() {
			clusterDegradation = fallbackDegradation("backend_unavailable")
		} else if mlClusters, mlModel, mlDegradation, err := s.mlBridge.Cluster(ctx, repoName, prs, logger.RequestIDFromContext(ctx)); err == nil {
			if len(mlClusters) > 0 {
				clusters = mlClusters
			}
			if mlModel != "" {
				clusterModel = mlModel
			}
			clusterDegradation = mlDegradation
		} else {
			clusterDegradation = fallbackDegradation("subprocess_error")
		}
	}

	// Compute corpus fingerprint for intermediate result caching
	var fingerprint string
	if s.cacheStore != nil && len(prs) > 0 {
		fingerprint = cache.CorpusFingerprint(prs)
	}

	var duplicates []types.DuplicateGroup
	var overlaps []types.DuplicateGroup
	duplicateDegradation := types.DegradationMetadata{}
	dupCacheHit := false

	// Duplicate detection: try cache first
	if s.cacheStore != nil && fingerprint != "" {
		if cachedGroups, found, err := s.cacheStore.LoadDuplicateGroups(repoName, fingerprint); err == nil && found {
			// Cache stores all groups; use similarity threshold to separate duplicates from overlaps.
			// Cached corpora may carry older-but-truthful duplicate groups scored at 0.80 after
			// the corrected formula, so preserve them as duplicates on cache-backed reruns.
			for _, g := range cachedGroups {
				if g.Similarity >= types.CachedDuplicateThreshold {
					duplicates = append(duplicates, g)
				} else if g.Similarity >= types.OverlapThreshold {
					overlaps = append(overlaps, g)
				}
			}
			dupCacheHit = true
			telemetry.StageLatenciesMS["duplicates_ms"] = 0
			s.emit("duplicates", len(duplicates), len(prs))
			log.Info("duplicate groups loaded from cache", "dup_count", len(duplicates), "overlap_count", len(overlaps))
		}
	}

	if !dupCacheHit {
		dupStart := time.Now()
		var mergedPRs []review.MergedPRRecord
		if s.cacheStore != nil {
			var err error
			mergedPRs, err = review.FetchMergedPRs(ctx, s.cacheStore, repoName)
			if err != nil {
				log.Warn("failed to fetch merged PRs", "error", err)
			}
		}
		duplicateThreshold := types.DuplicateThreshold
		if !meta.LiveSource {
			duplicateThreshold = types.CachedDuplicateThreshold
		}
		duplicates, overlaps = classifyDuplicates(prs, mergedPRs, s.emit, duplicateThreshold)
		telemetry.StageLatenciesMS["duplicates_ms"] = int(time.Since(dupStart).Milliseconds())
		s.emit("duplicates", len(duplicates), len(prs))

		// Attempt ML-backed duplicate detection via Voyage if configured.
		// Skip ML bridge in force-cache mode to avoid subprocess overhead.
		if !s.allowForceCache && s.mlBridge != nil {
			if !s.mlBridge.Available() {
				duplicateDegradation = fallbackDegradation("backend_unavailable")
			} else if mlDups, mlOverlaps, mlDegradation, err := s.mlBridge.Duplicates(ctx, repoName, prs, types.DuplicateThreshold, types.OverlapThreshold, logger.RequestIDFromContext(ctx)); err == nil {
				if len(mlDups) > 0 {
					duplicates = mlDups
				}
				if len(mlOverlaps) > 0 {
					overlaps = mlOverlaps
				}
				duplicateDegradation = mlDegradation
			} else {
				duplicateDegradation = fallbackDegradation("subprocess_error")
			}
		}

		// Save duplicate groups to cache (both duplicates and overlaps)
		if fingerprint != "" && (len(duplicates) > 0 || len(overlaps) > 0) {
			allGroups := make([]types.DuplicateGroup, 0, len(duplicates)+len(overlaps))
			allGroups = append(allGroups, duplicates...)
			allGroups = append(allGroups, overlaps...)
			if err := s.cacheStore.SaveDuplicateGroups(repoName, allGroups, fingerprint); err != nil {
				log.Warn("failed to save duplicate groups to cache", "error", err)
			}
		}
	}

	var conflictProgress func(processed int, total int)
	if meta.LiveSource {
		writeLivePhaseStatus(log, "analysis in progress", len(prs))
		conflictProgress = newLiveAnalysisProgressReporter(log, 100)
	}

	var conflicts []types.ConflictPair
	conflictCacheHit := false

	// Conflict detection: try cache first
	if s.cacheStore != nil && fingerprint != "" {
		if cachedConflicts, found, err := s.cacheStore.LoadConflictCache(repoName, fingerprint); err == nil && found {
			conflicts = cachedConflicts
			conflictCacheHit = true
			telemetry.StageLatenciesMS["conflicts_ms"] = 0
			telemetry.GraphDeltaEdges = len(conflicts)
			s.emit("conflicts", len(conflicts), len(prs))
			log.Info("conflicts loaded from cache", "count", len(conflicts))
		}
	}

	if !conflictCacheHit {
		conflictStart := time.Now()
		conflicts = buildConflicts(repoName, prs, conflictProgress)
		telemetry.StageLatenciesMS["conflicts_ms"] = int(time.Since(conflictStart).Milliseconds())
		telemetry.GraphDeltaEdges = len(conflicts)
		s.emit("conflicts", len(conflicts), len(prs))

		// Save conflicts to cache
		if fingerprint != "" && len(conflicts) > 0 {
			if err := s.cacheStore.SaveConflictCache(repoName, conflicts, fingerprint); err != nil {
				log.Warn("failed to save conflicts to cache", "error", err)
			}
		}
	}
	deepSubsetSize := 0
	if s.precisionMode == precisionModeDeep {
		deepSubsetSize = min(len(prs), s.deepCandidateSubsetSize)
	} else {
		for i := range conflicts {
			if len(conflicts[i].FilesTouched) > 1 {
				conflicts[i].FilesTouched = conflicts[i].FilesTouched[:1]
			}
		}
	}
	staleStart := time.Now()
	staleness := buildStaleness(prs, duplicates, nowFn())
	telemetry.StageLatenciesMS["staleness_ms"] = int(time.Since(staleStart).Milliseconds())
	s.emit("staleness", len(staleness), len(prs))

	response := types.AnalysisResponse{
		Repo:                    repoName,
		GeneratedAt:             nowFn().Format(time.RFC3339),
		AnalysisTruncated:       meta.AnalysisTruncated,
		TruncationReason:        meta.TruncationReason,
		MaxPRsApplied:           meta.MaxPRsApplied,
		PRWindow:                meta.PRWindow,
		PrecisionMode:           s.precisionMode,
		DeepCandidateSubsetSize: deepSubsetSize,
		Counts: types.Counts{
			TotalPRs:        len(prs),
			ClusterCount:    len(clusters),
			DuplicateGroups: len(duplicates),
			OverlapGroups:   len(overlaps),
			ConflictPairs:   len(conflicts),
			StalePRs:        len(staleness),
			GarbagePRs:      len(garbage),
		},
		PRs:          prs,
		Clusters:     clusters,
		ClusterModel: clusterModel,
		ClusterDegradation: func() *types.DegradationMetadata {
			if clusterDegradation == (types.DegradationMetadata{}) {
				return nil
			}
			degradation := clusterDegradation
			return &degradation
		}(),
		DuplicateDegradation: func() *types.DegradationMetadata {
			if duplicateDegradation == (types.DegradationMetadata{}) {
				return nil
			}
			degradation := duplicateDegradation
			return &degradation
		}(),
		Duplicates:       duplicates,
		Overlaps:         overlaps,
		Conflicts:        conflicts,
		StalenessSignals: staleness,
		GarbagePRs:       garbage,
		Telemetry:        &telemetry,
	}

	s.emit("review", 0, len(response.PRs))
	reviewPayload, err := s.buildReviewPayload(ctx, repoName, response)
	if err != nil {
		log.Warn("review analysis failed", "error", err)
		reviewPayload = &types.ReviewResponse{
			TotalPRs:      len(response.PRs),
			ReviewedPRs:   0,
			Categories:    []types.ReviewCategoryCount{},
			Buckets:       []types.BucketCount{},
			RiskBuckets:   []types.BucketCount{},
			PriorityTiers: []types.PriorityTierCount{},
			Results:       []types.ReviewResult{},
			Partial:       true,
			Errors:        []string{err.Error()},
		}
	}
	s.emit("done", len(response.PRs), len(response.PRs))

	response.ReviewPayload = reviewPayload

	// Build duplicate synthesis plans using review results and conflict data.
	if len(response.Duplicates) > 0 || len(response.Overlaps) > 0 {
		response.DuplicateSynthesis = buildDuplicateSynthesis(
			response.Duplicates,
			response.Overlaps,
			response.PRs,
			reviewPayload,
			response.Conflicts,
		)
	}

	// Collapse duplicate/overlap chains into a flattened corpus.
	if len(response.DuplicateSynthesis) > 0 {
		collapsedCorpus, collapsedPRs := buildCollapsedCorpus(response.DuplicateSynthesis, response.PRs)
		response.CollapsedCorpus = collapsedCorpus
		response.PRs = collapsedPRs
		response.Counts.CollapsedDuplicateGroups = collapsedCorpus.CollapsedGroupCount
	}

	// Enrich each PR with decision engine results if review was run.
	if reviewPayload != nil && len(reviewPayload.Results) > 0 {
		enrichPRsWithReviewData(response.PRs, reviewPayload.Results)
	}

	return response, nil
}

func (s Service) buildReviewPayload(ctx context.Context, repoName string, response types.AnalysisResponse) (*types.ReviewResponse, error) {
	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}

	if len(response.PRs) == 0 {
		return &types.ReviewResponse{
			TotalPRs:      0,
			ReviewedPRs:   0,
			Categories:    []types.ReviewCategoryCount{},
			Buckets:       []types.BucketCount{},
			RiskBuckets:   []types.BucketCount{},
			PriorityTiers: []types.PriorityTierCount{},
			Results:       []types.ReviewResult{},
		}, nil
	}

	settingsDB := os.Getenv("PRATC_SETTINGS_DB")
	if settingsDB == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			settingsDB = filepath.Join(home, ".pratc", "pratc-settings.db")
		} else {
			settingsDB = "./pratc-settings.db"
		}
	}

	settingsStore, err := settings.Open(settingsDB)
	if err != nil {
		return nil, fmt.Errorf("open settings store: %w", err)
	}
	defer settingsStore.Close()

	cfg := settings.DefaultAnalyzerConfig()
	orchestrator := review.NewOrchestrator(cfg, settingsStore)

	// Fetch diff evidence for all PRs if mirror is available
	diffEvidence := make(map[int]struct {
		Files []types.PRFile
		Hunks []types.DiffHunk
	})
	if s.mirrorAvailable && s.mirrorBaseDir != "" {
		remoteURL := fmt.Sprintf(types.GitHubURLPrefix+"%s.git", repoName)
		if m, err := repo.OpenOrCreate(ctx, s.mirrorBaseDir, repoName, remoteURL); err == nil {
			// Fetch all PR refs to ensure we have them
			prNumbers := make([]int, len(response.PRs))
			for i, pr := range response.PRs {
				prNumbers[i] = pr.Number
			}
			if len(prNumbers) > 0 {
				_ = m.FetchAll(ctx, prNumbers, nil)
			}
			// Fetch diff patches for each PR
			for _, pr := range response.PRs {
				baseBranch := pr.BaseBranch
				if files, hunks, err := m.GetDiffPatch(ctx, pr.Number, baseBranch); err == nil {
					diffEvidence[pr.Number] = struct {
						Files []types.PRFile
						Hunks []types.DiffHunk
					}{Files: files, Hunks: hunks}
				}
			}
		}
	}

	clusterMap := make(map[string]types.PRCluster)
	for _, cluster := range response.Clusters {
		clusterMap[cluster.ClusterID] = cluster
	}

	duplicateMap := make(map[int][]types.DuplicateGroup)
	for _, dup := range response.Duplicates {
		duplicateMap[dup.CanonicalPRNumber] = append(duplicateMap[dup.CanonicalPRNumber], dup)
	}
	for _, ov := range response.Overlaps {
		duplicateMap[ov.CanonicalPRNumber] = append(duplicateMap[ov.CanonicalPRNumber], ov)
	}

	conflictMap := make(map[int][]types.ConflictPair)
	for _, conflict := range response.Conflicts {
		conflictMap[conflict.SourcePR] = append(conflictMap[conflict.SourcePR], conflict)
		conflictMap[conflict.TargetPR] = append(conflictMap[conflict.TargetPR], conflict)
	}

	staleMap := make(map[int]types.StalenessReport)
	for _, stale := range response.StalenessSignals {
		staleMap[stale.PRNumber] = stale
	}

	// Build PR number lookup map for O(1) related PR lookups
	prByNumber := make(map[int]types.PR)
	for _, p := range response.PRs {
		prByNumber[p.Number] = p
	}

	var allResults []types.ReviewResult
	var failedPRs []int
	totalPRs := len(response.PRs)
	for i, pr := range response.PRs {
		// Emit per-PR progress every 50 PRs to avoid flooding.
		if (i+1)%50 == 0 || i+1 == totalPRs {
			s.emit("review_pr", i+1, totalPRs)
		}
		clusterLabel := ""
		if cluster, ok := clusterMap[pr.ClusterID]; ok {
			clusterLabel = cluster.ClusterLabel
		}

		var relatedPRs []types.PR
		if cluster, ok := clusterMap[pr.ClusterID]; ok {
			for _, prID := range cluster.PRIDs {
				if prID != pr.Number {
					if p, ok := prByNumber[prID]; ok {
						relatedPRs = append(relatedPRs, p)
					}
				}
			}
		}

		prData := review.PRData{
			PR:              pr,
			Repo:            repoName,
			ClusterID:       pr.ClusterID,
			ClusterLabel:    clusterLabel,
			RelatedPRs:      relatedPRs,
			DuplicateGroups: duplicateMap[pr.Number],
			ConflictPairs:   conflictMap[pr.Number],
			Staleness:       nil,
			AnalyzedAt:      nowFn(),
		}
		if stale, ok := staleMap[pr.Number]; ok {
			prData.Staleness = &stale
		}

		// Populate diff evidence if available
		if evidence, ok := diffEvidence[pr.Number]; ok {
			prData.Files = evidence.Files
			prData.DiffHunks = evidence.Hunks
		}

		result, err := orchestrator.Review(ctx, prData)
		if err != nil {
			failedPRs = append(failedPRs, pr.Number)
			continue
		}
		allResults = append(allResults, result.Result)
	}

	categoryCount := make(map[types.ReviewCategory]int)
	tierCount := make(map[types.PriorityTier]int)
	for _, r := range allResults {
		categoryCount[r.Category]++
		tierCount[r.PriorityTier]++
	}

	var categories []types.ReviewCategoryCount
	for cat, cnt := range categoryCount {
		categories = append(categories, types.ReviewCategoryCount{Category: string(cat), Count: cnt})
	}
	var tiers []types.PriorityTierCount
	for tier, cnt := range tierCount {
		tiers = append(tiers, types.PriorityTierCount{Tier: string(tier), Count: cnt})
	}

	buckets := buildReviewBuckets(categoryCount)
	riskBuckets := buildRiskBuckets(allResults)

	return &types.ReviewResponse{
		TotalPRs:      len(response.PRs),
		ReviewedPRs:   len(allResults),
		Categories:    categories,
		Buckets:       buckets,
		RiskBuckets:   riskBuckets,
		PriorityTiers: tiers,
		Results:       allResults,
		Partial:       len(failedPRs) > 0,
		FailedPRs:     failedPRs,
	}, nil
}

func buildReviewBuckets(categoryCount map[types.ReviewCategory]int) []types.BucketCount {
	// v1.4 bucket labels
	bucketLabels := map[types.ReviewCategory]string{
		types.ReviewCategoryMergeNow:                "now",
		types.ReviewCategoryMergeAfterFocusedReview: "future",
		types.ReviewCategoryDuplicateSuperseded:     "duplicate",
		types.ReviewCategoryProblematicQuarantine:   "junk",
	}

	bucketCounts := make(map[string]int)
	for cat, label := range bucketLabels {
		bucketCounts[label] = categoryCount[cat]
	}
	bucketCounts["blocked"] = categoryCount[types.ReviewCategoryUnknownEscalate] + categoryCount[types.ReviewCategory("")]

	var buckets []types.BucketCount
	for _, label := range []string{
		"now",
		"future",
		"duplicate",
		"junk",
		"blocked",
	} {
		buckets = append(buckets, types.BucketCount{Bucket: label, Count: bucketCounts[label]})
	}
	return buckets
}

// enrichPRsWithReviewData populates decision engine fields on each PR
// by matching ReviewResult entries to their corresponding PRs by number.
// This makes bucket/reason/confidence/temporal routing directly visible
// on each PR in the AnalysisResponse.PRs array.
func enrichPRsWithReviewData(prs []types.PR, results []types.ReviewResult) {
	// Build lookup map from PR number to ReviewResult
	resultByPRNum := make(map[int]types.ReviewResult, len(results))
	for _, result := range results {
		resultByPRNum[result.PRNumber] = result
	}

	// Enrich each PR with its review data if available
	for i := range prs {
		if result, ok := resultByPRNum[prs[i].Number]; ok {
			prs[i].Confidence = result.Confidence
			prs[i].Reasons = result.Reasons
			prs[i].SubstanceScore = result.SubstanceScore
			prs[i].TemporalBucket = result.TemporalBucket
			prs[i].DecisionLayers = result.DecisionLayers
			prs[i].Category = result.Category
			prs[i].PriorityTier = result.PriorityTier
		}
	}
}

func buildRiskBuckets(results []types.ReviewResult) []types.BucketCount {
	const (
		securityBucket    = "security_risk"
		reliabilityBucket = "reliability_risk"
		performanceBucket = "performance_risk"
	)
	seen := map[string]map[int]struct{}{
		securityBucket:    {},
		reliabilityBucket: {},
		performanceBucket: {},
	}
	for _, result := range results {
		found := map[string]struct{}{}
		for _, finding := range result.AnalyzerFindings {
			switch finding.AnalyzerName {
			case "security":
				found[securityBucket] = struct{}{}
			case "reliability":
				found[reliabilityBucket] = struct{}{}
			case "performance":
				found[performanceBucket] = struct{}{}
			}
		}
		for bucket := range found {
			if seen[bucket] == nil {
				seen[bucket] = make(map[int]struct{})
			}
			seen[bucket][result.PRNumber] = struct{}{}
		}
	}

	buckets := make([]types.BucketCount, 0, 3)
	for _, label := range []string{securityBucket, reliabilityBucket, performanceBucket} {
		buckets = append(buckets, types.BucketCount{Bucket: label, Count: len(seen[label])})
	}
	return buckets
}

func (s Service) Cluster(ctx context.Context, repo string) (types.ClusterResponse, error) {
	log := logger.New("app")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	startTime := nowFn()
	defer func() {
		durationMs := int(nowFn().Sub(startTime).Milliseconds())
		log.Info("cluster operation completed", "duration_ms", durationMs)
		if durationMs > types.ClusterSLOMS {
			log.Error("cluster operation exceeded SLO", "duration_ms", durationMs, "slo_ms", types.ClusterSLOMS)
		}
	}()

	prs, repoName, meta, err := s.loadPRs(ctx, repo)
	if err != nil {
		return types.ClusterResponse{}, err
	}

	model := "heuristic-fallback"
	clusters := planner.New().BuildClusters(prs)
	clusterDegradation := types.DegradationMetadata{}

	// Attempt ML-backed clustering when configured; keep heuristic results but surface degradation.
	if s.mlBridge != nil {
		if !s.mlBridge.Available() {
			clusterDegradation = fallbackDegradation("backend_unavailable")
		} else if mlClusters, mlModel, mlDegradation, err := s.mlBridge.Cluster(ctx, repoName, prs, logger.RequestIDFromContext(ctx)); err == nil {
			if len(mlClusters) > 0 {
				clusters = mlClusters
				model = mlModel
			}
			clusterDegradation = mlDegradation
		} else {
			clusterDegradation = fallbackDegradation("subprocess_error")
		}
	}

	return types.ClusterResponse{
		Repo:                    repoName,
		GeneratedAt:             nowFn().Format(time.RFC3339),
		AnalysisTruncated:       meta.AnalysisTruncated,
		TruncationReason:        meta.TruncationReason,
		MaxPRsApplied:           meta.MaxPRsApplied,
		PRWindow:                meta.PRWindow,
		PrecisionMode:           s.precisionMode,
		DeepCandidateSubsetSize: 0,
		Model:                   model,
		Thresholds: types.Thresholds{
			Duplicate: types.DuplicateThreshold,
			Overlap:   types.OverlapThreshold,
		},
		Degradation: func() *types.DegradationMetadata {
			if clusterDegradation == (types.DegradationMetadata{}) {
				return nil
			}
			degradation := clusterDegradation
			return &degradation
		}(),
		Clusters: clusters,
	}, nil
}

func (s Service) Graph(ctx context.Context, repo string) (types.GraphResponse, error) {
	log := logger.New("app")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	startTime := nowFn()
	defer func() {
		durationMs := int(nowFn().Sub(startTime).Milliseconds())
		log.Info("graph operation completed", "duration_ms", durationMs)
		if durationMs > types.GraphSLOMS {
			log.Error("graph operation exceeded SLO", "duration_ms", durationMs, "slo_ms", types.GraphSLOMS)
		}
	}()

	prs, repoName, _, err := s.loadPRs(ctx, repo)
	if err != nil {
		return types.GraphResponse{}, err
	}

	g := graph.Build(repoName, prs)
	return types.GraphResponse{
		Repo:        repoName,
		GeneratedAt: nowFn().Format(time.RFC3339),
		Nodes:       g.Nodes,
		Edges:       g.Edges,
		DOT:         g.DOT(),
	}, nil
}

func (s Service) Plan(ctx context.Context, repo string, target int, mode formula.Mode) (types.PlanResponse, error) {
	return s.PlanWithOptions(ctx, repo, PlanOptions{
		Target: target,
		Mode:   mode,
	})
}

// PlanWithOptions is the widened plan entrypoint. Zero-value options use defaults.
func (s Service) PlanWithOptions(ctx context.Context, repo string, opts PlanOptions) (types.PlanResponse, error) {
	log := logger.New("app")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	startTime := nowFn()
	defer func() {
		durationMs := int(nowFn().Sub(startTime).Milliseconds())
		log.Info("plan operation completed", "duration_ms", durationMs)
		if durationMs > types.PlanSLOMS {
			log.Error("plan operation exceeded SLO", "duration_ms", durationMs, "slo_ms", types.PlanSLOMS)
		}
	}()

	prs, repoName, meta, err := s.loadPRs(ctx, repo)
	if err != nil {
		return types.PlanResponse{}, err
	}

	target := opts.Target
	if target <= 0 {
		target = 20
	}
	mode := opts.Mode
	if mode == "" {
		mode = formula.ModeCombination
	}

	// Apply CollapseDuplicates from opts (overrides service-level default if explicitly set).
	collapseDup := s.collapseDuplicates
	if opts.CollapseDuplicates {
		collapseDup = true
	}

	// Duplicate collapse: detect duplicates and replace superseded PRs with canonicals.
	var collapsedCorpus *types.CollapsedCorpus
	if collapseDup {
		var fingerprint string
		if s.cacheStore != nil && len(prs) > 0 {
			fingerprint = cache.CorpusFingerprint(prs)
		}

		var duplicates []types.DuplicateGroup
		var overlaps []types.DuplicateGroup
		dupCacheHit := false

		if s.cacheStore != nil && fingerprint != "" {
			if cachedGroups, found, err := s.cacheStore.LoadDuplicateGroups(repoName, fingerprint); err == nil && found {
				for _, g := range cachedGroups {
					if g.Similarity >= types.CachedDuplicateThreshold {
						duplicates = append(duplicates, g)
					} else if g.Similarity >= types.OverlapThreshold {
						overlaps = append(overlaps, g)
					}
				}
				dupCacheHit = true
			}
		}

		if !dupCacheHit {
			duplicateThreshold := types.DuplicateThreshold
			if !meta.LiveSource {
				duplicateThreshold = types.CachedDuplicateThreshold
			}
			duplicates, overlaps = classifyDuplicates(prs, nil, nil, duplicateThreshold)
		}

		if len(duplicates) > 0 || len(overlaps) > 0 {
			synthesis := buildDuplicateSynthesis(duplicates, overlaps, prs, nil, nil)
			cc, updatedPRs := buildCollapsedCorpus(synthesis, prs)
			collapsedCorpus = &cc
			prs = updatedPRs
		}
	}

	planTelemetry := types.OperationTelemetry{
		PoolStrategy:     "heuristic_prefilter+formula_tiers",
		PoolSizeBefore:   len(prs),
		PoolSizeAfter:    0,
		GraphDeltaEdges:  0,
		DecayPolicy:      "none",
		PairwiseShards:   estimatePairwiseShards(len(prs)),
		StageLatenciesMS: map[string]int{},
		StageDropCounts:  map[string]int{},
	}

	clusterStart := time.Now()
	clusters := planner.New().BuildClusters(prs)
	planTelemetry.StageLatenciesMS["clusters_ms"] = int(time.Since(clusterStart).Milliseconds())
	clusterByPR := make(map[int]string)
	for _, cluster := range clusters {
		for _, prID := range cluster.PRIDs {
			clusterByPR[prID] = cluster.ClusterID
		}
	}

	pool := make([]types.PR, 0, len(prs))
	rejections := make([]types.PlanRejection, 0)
	for _, pr := range prs {
		if pr.IsDraft {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "draft"})
			planTelemetry.StageDropCounts["draft"]++
			continue
		}
		if pr.Mergeable == "conflicting" {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "merge conflict"})
			planTelemetry.StageDropCounts["merge_conflict"]++
			continue
		}
		if pr.CIStatus == "failure" {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "ci failure"})
			planTelemetry.StageDropCounts["ci_failure"]++
			continue
		}
		if collapsedCorpus != nil {
			if _, isSuperseded := collapsedCorpus.SupersededToCanonical[pr.Number]; isSuperseded {
				rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "superseded by duplicate collapse"})
				planTelemetry.StageDropCounts["duplicate_collapse"]++
				continue
			}
		}
		pr.ClusterID = clusterByPR[pr.Number]
		pool = append(pool, pr)
	}

	// Compute dynamic target based on viable pool size (v1.6.1).
	target = s.dynamicTarget.ComputeDynamicTarget(len(pool), target)

	// Adjust target if the collapsed pool is smaller than the target.
	if collapsedCorpus != nil && target > len(pool) {
		target = len(pool)
	}

	planTelemetry.PoolSizeAfter = len(pool)
	if len(pool) == 0 {
		return types.PlanResponse{
			Repo:                    repoName,
			GeneratedAt:             nowFn().Format(time.RFC3339),
			AnalysisTruncated:       meta.AnalysisTruncated,
			TruncationReason:        meta.TruncationReason,
			MaxPRsApplied:           meta.MaxPRsApplied,
			PRWindow:                meta.PRWindow,
			PrecisionMode:           s.precisionMode,
			DeepCandidateSubsetSize: 0,
			Target:                  target,
			CandidatePoolSize:       0,
			Strategy:                "formula+graph",
			Selected:                nil,
			Ordering:                nil,
			Rejections:              rejections,
			Telemetry:               &planTelemetry,
		}, nil
	}

	// Use PoolSelector for weighted scoring and time-decay windowing.
	poolResult := s.poolSelector.SelectCandidates(ctx, repoName, pool, target, s.timeDecayConfig)
	planTelemetry.PoolStrategy = poolResult.Telemetry.PoolStrategy
	planTelemetry.DecayPolicy = poolResult.Telemetry.DecayPolicy
	planTelemetry.PoolSizeAfter = poolResult.SelectedCount

	// Build lookup from PR number to PoolSelector candidate for rationale injection.
	poolCandidateByNum := make(map[int]planning.PoolCandidate, len(poolResult.Selected))
	for _, pc := range poolResult.Selected {
		poolCandidateByNum[pc.PR.Number] = pc
	}

	// Convert PoolResult.Selected back to []types.PR for the formula engine.
	poolPRs := make([]types.PR, 0, len(poolResult.Selected))
	for _, pc := range poolResult.Selected {
		poolPRs = append(poolPRs, pc.PR)
	}

	// Apply PlanOptions-based filters before the planner.
	preFilterCount := len(poolPRs)

	// ScoreMin: filter candidates below the minimum score threshold.
	if opts.ScoreMin > 0 {
		filtered := make([]types.PR, 0, len(poolPRs))
		for _, pc := range poolResult.Selected {
			if pc.PriorityScore >= opts.ScoreMin {
				filtered = append(filtered, pc.PR)
			} else {
				rejections = append(rejections, types.PlanRejection{PRNumber: pc.PR.Number, Reason: "below score_min threshold"})
				planTelemetry.StageDropCounts["score_min"]++
			}
		}
		poolPRs = filtered
	}

	// StaleScoreThreshold: filter candidates whose staleness score exceeds the threshold.
	if opts.StaleScoreThreshold > 0 {
		filtered := make([]types.PR, 0, len(poolPRs))
		staleByNum := make(map[int]float64, len(poolResult.Selected))
		for _, pc := range poolResult.Selected {
			staleByNum[pc.PR.Number] = pc.ComponentScores.StalenessScore
		}
		for _, pr := range poolPRs {
			if staleByNum[pr.Number] > opts.StaleScoreThreshold {
				rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "above stale_score_threshold"})
				planTelemetry.StageDropCounts["stale_threshold"]++
			} else {
				filtered = append(filtered, pr)
			}
		}
		poolPRs = filtered
	}

	// ExcludeConflicts: build conflict graph early and remove conflicting PRs.
	if opts.ExcludeConflicts && len(poolPRs) > 1 {
		conflictWarnings := buildConflictWarnings(repoName, poolPRs)
		filtered := make([]types.PR, 0, len(poolPRs))
		for _, pr := range poolPRs {
			if warnings, ok := conflictWarnings[pr.Number]; ok && len(warnings) > 0 {
				rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "conflict warning (exclude_conflicts)"})
				planTelemetry.StageDropCounts["exclude_conflicts"]++
			} else {
				filtered = append(filtered, pr)
			}
		}
		poolPRs = filtered
	}

	// CandidatePoolCap: cap the scored candidate pool.
	if opts.CandidatePoolCap > 0 && len(poolPRs) > opts.CandidatePoolCap {
		for _, pr := range poolPRs[opts.CandidatePoolCap:] {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "excluded by candidate_pool_cap"})
			planTelemetry.StageDropCounts["candidate_pool_cap"]++
		}
		poolPRs = poolPRs[:opts.CandidatePoolCap]
	}

	if len(poolPRs) != preFilterCount {
		planTelemetry.PoolSizeAfter = len(poolPRs)
	}

	// Adjust target if PlanOptions filters reduced the pool below the target.
	if target > len(poolPRs) && mode != formula.ModeWithReplacement {
		target = len(poolPRs)
	}

	// Return early if PlanOptions filters eliminated all candidates.
	if len(poolPRs) == 0 {
		return types.PlanResponse{
			Repo:              repoName,
			GeneratedAt:       nowFn().Format(time.RFC3339),
			Target:            target,
			CandidatePoolSize: 0,
			Strategy:          "formula+graph",
			Rejections:        rejections,
			Telemetry:         &planTelemetry,
			CollapsedCorpus:   collapsedCorpus,
		}, nil
	}

	// Route to hierarchical or formula planner based on planningStrategy.
	var selectedPRs []types.PR
	var orderedPRs []types.PR
	if s.planningStrategy == "hierarchical" {
		planTelemetry.PlanningStrategy = "hierarchical"
		planTelemetry = s.runHierarchicalPlan(ctx, repoName, poolPRs, poolCandidateByNum, planTelemetry, &rejections)
		// Build selectedPRs from the updated rejections by finding non-rejected PRs.
		selectedByNum := make(map[int]bool)
		for _, pr := range poolPRs {
			selectedByNum[pr.Number] = true
		}
		for _, r := range rejections {
			delete(selectedByNum, r.PRNumber)
		}
		for _, pr := range poolPRs {
			if selectedByNum[pr.Number] {
				selectedPRs = append(selectedPRs, pr)
			}
		}
		// orderedPRs will be rebuilt below using orderSelection.
		orderedPRs = orderSelection(repoName, selectedPRs)
	} else {
		planTelemetry.PlanningStrategy = "formula"
		// Formula engine path (existing code).
		pickCount := target
		if pickCount > len(poolPRs) && mode != formula.ModeWithReplacement {
			pickCount = len(poolPRs)
		}

		engineConfig := formula.DefaultConfig()
		engineConfig.Mode = mode
		engine := formula.NewEngine(engineConfig)
		searchStart := time.Now()
		searchResult, err := engine.Search(formula.SearchInput{
			Pool:        poolPRs,
			Target:      pickCount,
			PreFiltered: true,
			Now:         nowFn(),
		})
		if err != nil {
			return types.PlanResponse{}, fmt.Errorf("plan search: %w", err)
		}
		planTelemetry.StageLatenciesMS["formula_search_ms"] = int(time.Since(searchStart).Milliseconds())
		if searchResult.Telemetry.PairwiseShards > planTelemetry.PairwiseShards {
			planTelemetry.PairwiseShards = searchResult.Telemetry.PairwiseShards
		}

		// Deduplicate: formula engine may return duplicates in with_replacement mode.
		rawSelected := searchResult.Best.Selected
		seen := make(map[int]struct{}, len(rawSelected))
		selectedPRs = make([]types.PR, 0, len(rawSelected))
		for _, pr := range rawSelected {
			if _, ok := seen[pr.Number]; ok {
				continue
			}
			seen[pr.Number] = struct{}{}
			selectedPRs = append(selectedPRs, pr)
		}
		selectedByNumber := seen
		for _, pr := range poolPRs {
			if _, ok := selectedByNumber[pr.Number]; !ok {
				rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "not selected by strategy"})
				planTelemetry.StageDropCounts["not_selected_by_strategy"]++
			}
		}

		orderStart := time.Now()
		orderedPRs = orderSelection(repoName, selectedPRs)
		planTelemetry.StageLatenciesMS["ordering_ms"] = int(time.Since(orderStart).Milliseconds())
	}

	// Build conflict warnings before creating candidates that need them.
	warningGraph := graph.Build(repoName, orderedPRs)
	warningsByPR := buildConflictWarnings(repoName, orderedPRs)
	planTelemetry.GraphDeltaEdges = warningGraph.Telemetry.GraphDeltaEdges
	if warningGraph.Telemetry.PairwiseShards > planTelemetry.PairwiseShards {
		planTelemetry.PairwiseShards = warningGraph.Telemetry.PairwiseShards
	}

	selected := make([]types.MergePlanCandidate, 0, len(selectedPRs))
	for _, pr := range selectedPRs {
		if pc, ok := poolCandidateByNum[pr.Number]; ok {
			selected = append(selected, poolPlanCandidateFrom(pc, warningsByPR[pr.Number]))
		} else {
			selected = append(selected, candidateFromPR(pr, plannerPriority(pr, nowFn()), warningsByPR[pr.Number]))
		}
	}

	ordering := make([]types.MergePlanCandidate, 0, len(orderedPRs))
	for _, pr := range orderedPRs {
		if pc, ok := poolCandidateByNum[pr.Number]; ok {
			ordering = append(ordering, poolPlanCandidateFrom(pc, warningsByPR[pr.Number]))
		} else {
			ordering = append(ordering, candidateFromPR(pr, plannerPriority(pr, nowFn()), warningsByPR[pr.Number]))
		}
	}

	sort.Slice(rejections, func(i, j int) bool {
		return rejections[i].PRNumber < rejections[j].PRNumber
	})

	return types.PlanResponse{
		Repo:                    repoName,
		GeneratedAt:             nowFn().Format(time.RFC3339),
		AnalysisTruncated:       meta.AnalysisTruncated,
		TruncationReason:        meta.TruncationReason,
		MaxPRsApplied:           meta.MaxPRsApplied,
		PRWindow:                meta.PRWindow,
		PrecisionMode:           s.precisionMode,
		DeepCandidateSubsetSize: 0,
		Target:                  target,
		CandidatePoolSize:       len(poolPRs),
		Strategy:                "formula+graph",
		Selected:                selected,
		Ordering:                ordering,
		Rejections:              rejections,
		Telemetry:               &planTelemetry,
		CollapsedCorpus:         collapsedCorpus,
	}, nil
}

// PlanOmni executes an omni-batch plan using selector expressions to define stages.
// The selector syntax supports ranges (1-5), individual numbers (1,3,5), and wildcards (*).
func (s Service) PlanOmni(ctx context.Context, repo string, selector string) (types.OmniPlanResponse, error) {
	log := logger.New("app")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	startTime := nowFn()
	defer func() {
		durationMs := int(nowFn().Sub(startTime).Milliseconds())
		log.Info("plan omni operation completed", "duration_ms", durationMs)
	}()

	// Parse selector expression into stages
	stages, err := parseOmniSelector(selector)
	if err != nil {
		return types.OmniPlanResponse{}, fmt.Errorf("parse selector: %w", err)
	}

	// Get all PRs for the repo
	prs, repoName, _, err := s.loadPRs(ctx, repo)
	if err != nil {
		return types.OmniPlanResponse{}, err
	}

	// Build candidate pool using standard filtering
	pool := make([]types.PR, 0, len(prs))
	for _, pr := range prs {
		if pr.IsDraft {
			continue
		}
		if pr.Mergeable == "conflicting" {
			continue
		}
		if pr.CIStatus == "failure" {
			continue
		}
		pool = append(pool, pr)
	}

	// Sort by priority
	sort.Slice(pool, func(i, j int) bool {
		left := plannerPriority(pool[i], nowFn())
		right := plannerPriority(pool[j], nowFn())
		if left == right {
			return pool[i].Number < pool[j].Number
		}
		return left > right
	})

	// Apply selector to get selected PRs
	selected := make([]int, 0)
	ordering := make([]int, 0)
	stageResults := make([]types.OmniPlanStage, 0, len(stages))

	for _, stage := range stages {
		matched := 0
		stageSelected := 0

		for _, idx := range stage.Indices {
			if idx >= 0 && idx < len(pool) {
				matched++
				// Check if not already selected
				alreadySelected := false
				for _, sel := range selected {
					if sel == pool[idx].Number {
						alreadySelected = true
						break
					}
				}
				if !alreadySelected && stageSelected < stage.Size {
					selected = append(selected, pool[idx].Number)
					ordering = append(ordering, pool[idx].Number)
					stageSelected++
				}
			}
		}

		stageResults = append(stageResults, types.OmniPlanStage{
			StageSize: stage.Size,
			Matched:   matched,
			Selected:  stageSelected,
		})
	}

	return types.OmniPlanResponse{
		Repo:        repoName,
		GeneratedAt: nowFn().Format(time.RFC3339),
		Selector:    selector,
		Mode:        "omni_batch",
		StageCount:  len(stageResults),
		Stages:      stageResults,
		Selected:    selected,
		Ordering:    ordering,
	}, nil
}

// omniStage represents a parsed stage from selector expression.
type omniStage struct {
	Indices []int // Pool indices to consider
	Size    int   // Max PRs to select from this stage
}

// parseOmniSelector parses a selector expression like "1-5,10-15,*" into stages.
func parseOmniSelector(selector string) ([]omniStage, error) {
	if selector == "" {
		return []omniStage{{Indices: []int{}, Size: 20}}, nil
	}

	parts := strings.Split(selector, ",")
	stages := make([]omniStage, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		stage := omniStage{Size: 20} // default stage size

		if part == "*" {
			// Wildcard - will be filled at runtime
			stage.Indices = []int{}
		} else if strings.Contains(part, "-") {
			// Range like "1-5"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid range numbers: %s", part)
			}
			if start > end {
				return nil, fmt.Errorf("invalid range: start > end")
			}
			// Convert to 0-based indices
			for i := start - 1; i < end; i++ {
				stage.Indices = append(stage.Indices, i)
			}
		} else {
			// Single number
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid selector number: %s", part)
			}
			stage.Indices = []int{num - 1} // Convert to 0-based
		}

		stages = append(stages, stage)
	}

	if len(stages) == 0 {
		return []omniStage{{Indices: []int{}, Size: 20}}, nil
	}

	return stages, nil
}

func (s Service) loadPRs(ctx context.Context, repo string) ([]types.PR, string, truncationMeta, error) {
	log := logger.New("service")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	manifest, err := testutil.LoadManifest()
	if err != nil {
		return nil, "", truncationMeta{}, fmt.Errorf("load fixture manifest: %w", err)
	}

	targetRepo := strings.TrimSpace(repo)
	if targetRepo == "" {
		targetRepo = manifest.Repo
	}

	if targetRepo != manifest.Repo && strings.TrimSpace(s.token) == "" && s.tokenSource == nil {
		resolved, err := gh.ResolveToken(ctx)
		if err != nil {
			return nil, "", truncationMeta{}, fmt.Errorf("missing auth for live repo %q: %w", targetRepo, err)
		}
		s.token = resolved
		s.tokenSource = gh.NewMultiTokenSource([]string{resolved}, nil)
	}

	// Try cache first if enabled
	if s.useCacheFirst && s.cacheStore != nil {
		if cachedPRs, ok := s.tryLoadFromCache(targetRepo); ok && len(cachedPRs) > 0 {
			filtered, meta := s.applyIntakeControls(cachedPRs)
			meta.LiveSource = false
			writeLivePhaseStatus(log, "cache loaded, starting analysis", len(filtered))
			// Skip enrichment in force-cache mode; use whatever file data is already cached
			if !s.allowForceCache && (s.mirrorAvailable || s.hasGitHubAuth()) {
				s.enrichPRsWithFilesFromMirrorOrGraphQL(ctx, targetRepo, filtered)
			}
			return filtered, targetRepo, meta, nil
		}
	}

	if s.useCacheFirst && !s.allowLive {
		// force-cache path: load from stale cache, skip enrichment to avoid API calls
		if s.allowForceCache && s.cacheStore != nil {
			if cachedPRs, err := s.cacheStore.ListPRs(cache.PRFilter{Repo: targetRepo}); err == nil && len(cachedPRs) > 0 {
				filtered, meta := s.applyIntakeControls(cachedPRs)
				meta.LiveSource = false
				writeLivePhaseStatus(log, "stale cache loaded, starting analysis", len(filtered))
				return filtered, targetRepo, meta, nil
			}
		}
		return nil, "", truncationMeta{}, fmt.Errorf("sync first: run `pratc sync --repo=%s` before analyze, or rerun with explicit live override", targetRepo)
	}

	if s.allowLive && s.hasGitHubAuth() {
		livePRs, liveErr := s.fetchLivePRs(ctx, targetRepo)
		if liveErr == nil && len(livePRs) > 0 {
			filtered, meta := s.applyIntakeControls(livePRs)
			meta.LiveSource = true
			writeLivePhaseStatus(log, "fetch complete, starting analysis", len(filtered))
			if s.mirrorAvailable || s.token != "" {
				s.enrichPRsWithFilesFromMirrorOrGraphQL(ctx, targetRepo, filtered)
			}
			return filtered, targetRepo, meta, nil
		}
	}

	fixturePRs, err := testutil.LoadFixturePRs()
	if err != nil {
		return nil, "", truncationMeta{}, fmt.Errorf("load fixture prs: %w", err)
	}

	filtered := make([]types.PR, 0, len(fixturePRs))
	for _, pr := range fixturePRs {
		if pr.Repo == targetRepo {
			filtered = append(filtered, pr)
		}
	}
	if len(filtered) == 0 {
		if targetRepo != manifest.Repo {
			return nil, "", truncationMeta{}, fmt.Errorf("no fixture data for repo %q and no live snapshot available", targetRepo)
		}
		filtered = fixturePRs
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Number < filtered[j].Number
	})

	controlled, meta := s.applyIntakeControls(filtered)
	if s.mirrorAvailable || s.token != "" {
		s.enrichPRsWithFilesFromMirrorOrGraphQL(ctx, targetRepo, controlled)
	}
	return controlled, targetRepo, meta, nil
}

func (s Service) tryLoadFromCache(repo string) ([]types.PR, bool) {
	if s.cacheStore == nil {
		return nil, false
	}
	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	lastSync, err := s.cacheStore.LastSync(repo)
	if err != nil || nowFn().Sub(lastSync) > s.cacheTTL {
		return nil, false
	}
	prs, err := s.cacheStore.ListPRs(cache.PRFilter{Repo: repo})
	if err != nil || len(prs) == 0 {
		return nil, false
	}
	return prs, true
}

func (s Service) fetchLivePRs(ctx context.Context, repo string) ([]types.PR, error) {
	log := logger.New("service")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}

	client := gh.NewClient(gh.Config{
		Token:           s.token,
		ReserveRequests: 200,
	})
	prs, err := client.FetchPullRequests(ctx, repo, gh.PullRequestListOptions{PerPage: 100, Progress: newLiveProgressReporter(log, 100)})
	if err != nil {
		return nil, err
	}

	s.enrichPRsWithFilesFromMirrorOrGraphQL(ctx, repo, prs)

	return prs, nil
}

func (s Service) enrichPRsWithFilesFromMirrorOrGraphQL(ctx context.Context, repo string, prs []types.PR) {
	// In force-cache mode, enrichment is disabled — PRs use whatever data is in cache
	if s.allowForceCache {
		return
	}

	if s.mirrorAvailable {
		s.enrichFromMirror(ctx, repo, prs)
	} else {
		s.enrichFromGraphQL(ctx, repo, prs)
	}
}

func (s Service) enrichFromMirror(ctx context.Context, repoID string, prs []types.PR) {
	if s.mirrorBaseDir == "" {
		s.enrichFromGraphQL(ctx, repoID, prs)
		return
	}

	remoteURL := fmt.Sprintf(types.GitHubURLPrefix+"%s.git", repoID)
	mirror, err := repo.OpenOrCreate(ctx, s.mirrorBaseDir, repoID, remoteURL)
	if err != nil {
		s.enrichFromGraphQL(ctx, repoID, prs)
		return
	}

	prNumbers := make([]int, len(prs))
	for i, pr := range prs {
		prNumbers[i] = pr.Number
	}

	if len(prNumbers) > 0 {
		if err := mirror.FetchAll(ctx, prNumbers, nil); err != nil {
			s.enrichFromGraphQL(ctx, repoID, prs)
			return
		}
	}

	prFilesList, err := mirror.GetChangedFilesBatch(ctx, prNumbers, "main", 10)
	if err != nil {
		s.enrichFromGraphQL(ctx, repoID, prs)
		return
	}

	prFilesMap := make(map[int][]string)
	for _, pf := range prFilesList {
		prFilesMap[pf.PRNumber] = pf.Files
	}

	for i := range prs {
		if len(prs[i].FilesChanged) == 0 {
			if files, ok := prFilesMap[prs[i].Number]; ok {
				prs[i].FilesChanged = files
			}
		}
	}
}

func (s Service) enrichFromGraphQL(ctx context.Context, repoID string, prs []types.PR) {
	client := gh.NewClient(gh.Config{
		Token:           s.token,
		ReserveRequests: 200,
	})
	total := len(prs)
	for i := range prs {
		if len(prs[i].FilesChanged) == 0 {
			files, fileErr := client.FetchPullRequestFiles(ctx, repoID, prs[i].Number)
			if fileErr == nil {
				prs[i].FilesChanged = files
			}
		}
		if (i+1)%100 == 0 || i+1 == total {
			s.emit("enrich_files", i+1, total)
		}
	}
	s.emit("enrich_files_done", total, total)
}

func newLiveProgressReporter(log *logger.Logger, step int) func(processed int, total int) {
	if step <= 0 {
		step = 100
	}
	last := -1
	return func(processed int, _ int) {
		if log == nil || processed <= 0 {
			return
		}
		if processed != 1 && processed%step != 0 {
			return
		}
		if processed == last {
			return
		}
		last = processed
		log.Info("fetch progress", "fetched", processed)
	}
}

func writeLivePhaseStatus(log *logger.Logger, phase string, count int) {
	if log == nil || strings.TrimSpace(phase) == "" {
		return
	}
	if count > 0 {
		log.Info("phase status", "phase", phase, "pr_count", count)
		return
	}
	log.Info("phase status", "phase", phase)
}

func (s Service) applyIntakeControls(input []types.PR) ([]types.PR, truncationMeta) {
	meta := truncationMeta{}
	log := logger.New("app")
	output := make([]types.PR, len(input))
	copy(output, input)

	if s.beginningPRNumber > 0 || s.endingPRNumber > 0 {
		begin := s.beginningPRNumber
		end := s.endingPRNumber
		if begin <= 0 {
			begin = 1
		}
		if end > 0 && begin > end {
			begin, end = end, begin
		}
		windowed := make([]types.PR, 0, len(output))
		for _, pr := range output {
			if pr.Number < begin {
				continue
			}
			if end > 0 && pr.Number > end {
				continue
			}
			windowed = append(windowed, pr)
		}
		if len(windowed) != len(output) {
			meta.AnalysisTruncated = true
			meta.TruncationReason = "pr_window"
			meta.PRWindow = &types.PRWindow{BeginningPRNumber: begin, EndingPRNumber: end}
		}
		output = windowed
	}

	effectiveMaxPRs := s.maxPRs
	if effectiveMaxPRs < 0 {
		effectiveMaxPRs = 0
	}
	if effectiveMaxPRs > 0 && len(output) > effectiveMaxPRs {
		truncated := len(output) - effectiveMaxPRs
		log.Warn("pr corpus truncated by max_prs cap", "max_prs", effectiveMaxPRs, "truncated", truncated, "remaining", effectiveMaxPRs)
		output = output[:effectiveMaxPRs]
		meta.AnalysisTruncated = true
		meta.TruncationReason = "max_prs_cap"
		meta.MaxPRsApplied = effectiveMaxPRs
	}

	return output, meta
}

// classifyGarbage implements Layer 1 of the outer peel: identify PRs that are
// obviously bad and should not consume further analysis resources.
func classifyGarbage(prs []types.PR) []types.GarbagePR {
	var garbage []types.GarbagePR

	for _, pr := range prs {
		reasons := make([]string, 0, 3)

		// Check 1: Empty PR — no additions, no deletions, no files
		if pr.Additions == 0 && pr.Deletions == 0 && pr.ChangedFilesCount == 0 {
			reasons = append(reasons, "empty PR (no additions, deletions, or changed files)")
		}

		// Check 2: Bot PR
		if pr.IsBot {
			reasons = append(reasons, "bot-generated PR")
		}

		// Check 3: Spam patterns — very short or empty title
		title := strings.TrimSpace(pr.Title)
		if title == "" {
			reasons = append(reasons, "empty title")
		} else if len(title) <= 3 && title != "WIP" && title != "wip" {
			reasons = append(reasons, fmt.Sprintf("suspiciously short title (%q)", title))
		}

		// Check 4: Draft PR
		if pr.IsDraft && pr.Additions <= 1 && pr.Deletions <= 1 {
			reasons = append(reasons, "draft with minimal changes")
		}

		// Note: abandoned PR detection is handled by buildStaleness(), which has
		// access to merged PR history and can distinguish truly abandoned PRs
		// from old-but-still-relevant work. Do not duplicate that logic here.

		if len(reasons) > 0 {
			garbage = append(garbage, types.GarbagePR{
				PRNumber: pr.Number,
				Reason:   strings.Join(reasons, "; "),
			})
		}
	}

	return garbage
}

// classifyDuplicates compares PRs for similarity.
// Small corpora stay exact; larger corpora use MinHash/LSH candidate generation
// and then re-score only the candidate pairs with the exact existing formula.
func classifyDuplicates(prs []types.PR, mergedPRs []review.MergedPRRecord, emit func(phase string, done, total int), duplicateThreshold float64) ([]types.DuplicateGroup, []types.DuplicateGroup) {
	titleTokens := make([][]string, len(prs))
	bodyTokens := make([][]string, len(prs))
	for i := range prs {
		titleTokens[i] = util.Tokenize(prs[i].Title)
		if prs[i].Body != "" {
			bodyTokens[i] = util.Tokenize(prs[i].Body)
		}
	}

	duplicatesByCanonical := make(map[int]*types.DuplicateGroup)
	overlapsByCanonical := make(map[int]*types.DuplicateGroup)

	if len(prs) == 0 {
		return nil, nil
	}

	if len(prs)+len(mergedPRs) < minPRsForLSHDuplicateCandidates {
		pairs := make([][2]int, 0, (len(prs)*(len(prs)-1))/2)
		for i := 0; i < len(prs); i++ {
			for j := i + 1; j < len(prs); j++ {
				pairs = append(pairs, [2]int{i, j})
			}
		}
		runDuplicateCandidatePairs(prs, titleTokens, bodyTokens, pairs, duplicateThreshold, duplicatesByCanonical, overlapsByCanonical)
		for i := 0; i < len(prs); i++ {
			for _, merged := range mergedPRs {
				recordMergedDuplicate(prs[i], merged, duplicateThreshold, duplicatesByCanonical, overlapsByCanonical)
			}
		}
		if emit != nil {
			emit("duplicates_inner", len(pairs), len(prs))
		}
		return flattenGroups(duplicatesByCanonical), flattenGroups(overlapsByCanonical)
	}

	signatures := make([][]uint64, 0, len(prs)+len(mergedPRs))
	for i := range prs {
		signatures = append(signatures, util.MinHashSignature(duplicateLSHTokens(prs[i], titleTokens[i]), duplicateMinHashPermutations))
	}
	for _, merged := range mergedPRs {
		signatures = append(signatures, util.MinHashSignature(duplicateLSHTokensFromMerged(merged), duplicateMinHashPermutations))
	}

	candidatePairs := util.MinHashCandidatePairs(signatures, duplicateLSHBands)
	openOpenCandidates := make([][2]int, 0, len(candidatePairs))
	for _, pair := range candidatePairs {
		left, right := pair[0], pair[1]
		if right < len(prs) {
			openOpenCandidates = append(openOpenCandidates, pair)
			continue
		}
		if left < len(prs) && right >= len(prs) {
			merged := mergedPRs[right-len(prs)]
			recordMergedDuplicate(prs[left], merged, duplicateThreshold, duplicatesByCanonical, overlapsByCanonical)
		}
	}
	if len(openOpenCandidates) == 0 {
		for i := 0; i < len(prs); i++ {
			for _, merged := range mergedPRs {
				recordMergedDuplicate(prs[i], merged, duplicateThreshold, duplicatesByCanonical, overlapsByCanonical)
			}
		}
		if emit != nil {
			emit("duplicates_inner", 0, len(prs))
		}
		return flattenGroups(duplicatesByCanonical), flattenGroups(overlapsByCanonical)
	}

	runDuplicateCandidatePairs(prs, titleTokens, bodyTokens, openOpenCandidates, duplicateThreshold, duplicatesByCanonical, overlapsByCanonical)
	if emit != nil {
		emit("duplicates_inner", len(openOpenCandidates), len(prs))
	}
	return flattenGroups(duplicatesByCanonical), flattenGroups(overlapsByCanonical)
}

const (
	minPRsForLSHDuplicateCandidates = 150
	duplicateMinHashPermutations    = 8
	duplicateLSHBands               = 2
)

func runDuplicateCandidatePairs(
	prs []types.PR,
	titleTokens [][]string,
	bodyTokens [][]string,
	pairs [][2]int,
	duplicateThreshold float64,
	duplicatesByCanonical map[int]*types.DuplicateGroup,
	overlapsByCanonical map[int]*types.DuplicateGroup,
) {
	for _, pair := range pairs {
		left := pair[0]
		right := pair[1]
		if left < 0 || right < 0 || left >= len(prs) || right >= len(prs) || left >= right {
			continue
		}
		score := duplicateSimilarityScore(prs[left], prs[right], titleTokens[left], titleTokens[right], bodyTokens[left], bodyTokens[right])
		if score < types.OverlapThreshold {
			continue
		}
		canonical := min(prs[left].Number, prs[right].Number)
		other := max(prs[left].Number, prs[right].Number)
		if score >= duplicateThreshold {
			group := duplicatesByCanonical[canonical]
			if group == nil {
				group = &types.DuplicateGroup{CanonicalPRNumber: canonical, Reason: "title/body/file similarity above duplicate threshold"}
				duplicatesByCanonical[canonical] = group
			}
			group.Similarity = maxFloat(group.Similarity, score)
			group.DuplicatePRNums = appendUniqueInt(group.DuplicatePRNums, other)
			continue
		}
		group := overlapsByCanonical[canonical]
		if group == nil {
			group = &types.DuplicateGroup{CanonicalPRNumber: canonical, Reason: "title/body/file similarity in overlap range"}
			overlapsByCanonical[canonical] = group
		}
		group.Similarity = maxFloat(group.Similarity, score)
		group.DuplicatePRNums = appendUniqueInt(group.DuplicatePRNums, other)
	}
}

func duplicateSimilarityScore(left, right types.PR, leftTitleTokens, rightTitleTokens, leftBodyTokens, rightBodyTokens []string) float64 {
	titleScore := util.Jaccard(leftTitleTokens, rightTitleTokens)
	shortTitle := strings.ToLower(strings.TrimSpace(left.Title))
	longTitle := strings.ToLower(strings.TrimSpace(right.Title))
	if len(shortTitle) > len(longTitle) {
		shortTitle, longTitle = longTitle, shortTitle
	}
	if len(shortTitle) >= 10 && strings.Contains(longTitle, shortTitle) {
		titleScore = maxFloat(titleScore, 0.80)
	}
	bodyScore := 0.0
	if len(leftBodyTokens) > 0 && len(rightBodyTokens) > 0 {
		bodyScore = util.Jaccard(leftBodyTokens, rightBodyTokens)
	}
	fileScore := util.Jaccard(left.FilesChanged, right.FilesChanged)
	if len(left.FilesChanged) == 0 && len(right.FilesChanged) == 0 {
		fileScore = 0.5
	}
	score := round((0.4*titleScore)+(0.4*fileScore)+(0.2*bodyScore), 4)
	if fileScore > 0.8 && titleScore > 0.05 {
		score = maxFloat(score, 0.85)
	}
	if titleScore > 0.75 && fileScore == 0.5 {
		score = maxFloat(score, 0.80)
	}
	return score
}

func recordMergedDuplicate(
	pr types.PR,
	merged review.MergedPRRecord,
	duplicateThreshold float64,
	duplicatesByCanonical map[int]*types.DuplicateGroup,
	overlapsByCanonical map[int]*types.DuplicateGroup,
) {
	score := similarityWithMerged(pr, merged)
	if score < types.OverlapThreshold {
		return
	}
	canonical := pr.Number
	other := -merged.PRNumber
	if score >= duplicateThreshold {
		group := duplicatesByCanonical[canonical]
		if group == nil {
			group = &types.DuplicateGroup{CanonicalPRNumber: canonical, Reason: "title/body/file similarity above duplicate threshold (compared against merged history)"}
			duplicatesByCanonical[canonical] = group
		}
		group.Similarity = maxFloat(group.Similarity, score)
		group.DuplicatePRNums = appendUniqueInt(group.DuplicatePRNums, other)
		return
	}
	group := overlapsByCanonical[canonical]
	if group == nil {
		group = &types.DuplicateGroup{CanonicalPRNumber: canonical, Reason: "title/body/file similarity in overlap range (compared against merged history)"}
		overlapsByCanonical[canonical] = group
	}
	group.Similarity = maxFloat(group.Similarity, score)
	group.DuplicatePRNums = appendUniqueInt(group.DuplicatePRNums, other)
}

func duplicateLSHTokens(pr types.PR, titleTokens []string) []string {
	combined := make([]string, 0, len(titleTokens)+len(pr.FilesChanged))
	combined = append(combined, titleTokens...)
	for _, file := range pr.FilesChanged {
		combined = append(combined, strings.ToLower(strings.TrimSpace(file)))
	}
	return dedupeTokens(combined)
}

func duplicateLSHTokensFromMerged(merged review.MergedPRRecord) []string {
	return duplicateLSHTokens(types.PR{Title: merged.Title, Body: merged.Body, FilesChanged: merged.FilesChanged}, util.Tokenize(merged.Title))
}

func dedupeTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		trimmed := strings.TrimSpace(strings.ToLower(token))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func flattenGroups(input map[int]*types.DuplicateGroup) []types.DuplicateGroup {
	keys := make([]int, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	groups := make([]types.DuplicateGroup, 0, len(keys))
	for _, key := range keys {
		group := input[key]
		sort.Ints(group.DuplicatePRNums)
		groups = append(groups, *group)
	}

	return groups
}

func filterNoiseFiles(files []string) []string {
	return types.FilterNoiseFiles(files)
}

// isSourceFile returns true if the file looks like source code (not config, docs, or CI).
func isSourceFile(path string) bool {
	sourceExts := []string{".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".rs", ".java", ".rb", ".cpp", ".c", ".h"}
	for _, ext := range sourceExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func buildConflicts(repo string, prs []types.PR, progress func(processed int, total int)) []types.ConflictPair {
	return buildConflictsWithMinSignalFiles(repo, prs, progress, types.MinSharedSignalFilesForConflict)
}

func buildConflictsWithMinSignalFiles(repo string, prs []types.PR, progress func(processed int, total int), minSharedSignalFiles int) []types.ConflictPair {
	if minSharedSignalFiles <= 0 {
		minSharedSignalFiles = types.MinSharedSignalFilesForConflict
	}

	g := graph.BuildWithProgress(repo, prs, progress)
	conflicts := make([]types.ConflictPair, 0)
	for _, edge := range g.Edges {
		if edge.EdgeType != graph.EdgeTypeConflict {
			continue
		}
		files := parseSharedFiles(edge.Reason)

		// mergeability_signal: both PRs are marked conflicting by GitHub even with no
		// shared files. Always surface it — GitHub's mergeability judgment is authoritative
		// and does not depend on the shared-file threshold.
		isMergeabilitySignal := len(files) == 1 && files[0] == "mergeability_signal"
		if !isMergeabilitySignal {
			signalFiles := types.FilterNoiseFiles(files)
			if len(signalFiles) < minSharedSignalFiles {
				continue
			}
			files = signalFiles
		}

		conflictType := "attention_needed"
		severity := "low"
		if isMergeabilitySignal {
			severity = "medium"
		} else if len(files) >= 5 {
			severity = "high"
			conflictType = "merge_blocking"
		} else if len(files) >= minSharedSignalFiles {
			severity = "medium"
		}
		// Source code conflicts are more serious than config conflicts
		hasSourceCode := false
		for _, f := range files {
			if isSourceFile(f) {
				hasSourceCode = true
				break
			}
		}
		if hasSourceCode && len(files) >= 3 {
			severity = "high"
			conflictType = "merge_blocking"
		}

		conflicts = append(conflicts, types.ConflictPair{
			SourcePR:     edge.FromPR,
			TargetPR:     edge.ToPR,
			ConflictType: conflictType,
			FilesTouched: files,
			Severity:     severity,
			Reason:       fmt.Sprintf("%d shared files: %s", len(files), strings.Join(files, ", ")),
		})
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].SourcePR == conflicts[j].SourcePR {
			return conflicts[i].TargetPR < conflicts[j].TargetPR
		}
		return conflicts[i].SourcePR < conflicts[j].SourcePR
	})

	return conflicts
}

func newLiveAnalysisProgressReporter(log *logger.Logger, step int) func(processed int, total int) {
	if step <= 0 {
		step = 100
	}
	last := -1
	return func(processed int, total int) {
		if log == nil || processed <= 0 || total <= 0 {
			return
		}
		if processed != 1 && processed != total && processed%step != 0 {
			return
		}
		if processed == last {
			return
		}
		last = processed
		log.Info("analysis progress", "processed", processed, "total", total)
	}
}

func buildStaleness(prs []types.PR, duplicates []types.DuplicateGroup, now time.Time) []types.StalenessReport {
	log := logger.New("app")
	supersededBy := make(map[int][]int)
	for _, group := range duplicates {
		for _, duplicate := range group.DuplicatePRNums {
			supersededBy[duplicate] = appendUniqueInt(supersededBy[duplicate], group.CanonicalPRNumber)
		}
	}

	reports := make([]types.StalenessReport, 0)
	for _, pr := range prs {
		signals := make([]string, 0)
		reasons := make([]string, 0)
		score := 0.0

		updatedAt, err := time.Parse(time.RFC3339, pr.UpdatedAt)
		if err != nil {
			log.Warn("staleness: failed to parse PR updated_at",
				"pr_number", pr.Number,
				"updated_at", pr.UpdatedAt,
				"error", err)
		} else {
			days := now.Sub(updatedAt).Hours() / 24
			if days > 30 {
				signals = append(signals, "inactive")
				reasons = append(reasons, fmt.Sprintf("No updates in %.0f days", days))
				score += math.Min(days/2, 60)
			}
		}

		if pr.Mergeable == "conflicting" {
			signals = append(signals, "merge_conflict")
			reasons = append(reasons, "PR is currently conflicting")
			score += 20
		}

		superseded := supersededBy[pr.Number]
		if len(superseded) > 0 {
			signals = append(signals, "superseded")
			reasons = append(reasons, "Similar changes already represented by newer or canonical PRs")
			score += 20
		}

		if score > 0 {
			reports = append(reports, types.StalenessReport{
				PRNumber:     pr.Number,
				Score:        math.Min(score, 100),
				Signals:      signals,
				Reasons:      reasons,
				SupersededBy: superseded,
			})
		}
	}

	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Score == reports[j].Score {
			return reports[i].PRNumber < reports[j].PRNumber
		}
		return reports[i].Score > reports[j].Score
	})

	return reports
}

// buildDuplicateSynthesis creates synthesis plans for duplicate and near-duplicate groups.
// It examines each group's PRs using available review signals and nominates
// the best candidate for future merge-by-bot use.
func buildDuplicateSynthesis(
	duplicates []types.DuplicateGroup,
	overlaps []types.DuplicateGroup,
	prs []types.PR,
	reviewPayload *types.ReviewResponse,
	conflicts []types.ConflictPair,
) []types.DuplicateSynthesisPlan {
	if len(prs) == 0 {
		return nil
	}

	// Build PR lookup map
	prByNumber := make(map[int]types.PR, len(prs))
	for _, pr := range prs {
		prByNumber[pr.Number] = pr
	}

	// Build review results lookup
	reviewByNumber := make(map[int]types.ReviewResult)
	if reviewPayload != nil {
		reviewByNumber = make(map[int]types.ReviewResult, len(reviewPayload.Results))
		for _, r := range reviewPayload.Results {
			reviewByNumber[r.PRNumber] = r
		}
	}

	// Build conflict footprint: count conflicts per PR
	conflictFootprint := make(map[int]int)
	for _, c := range conflicts {
		conflictFootprint[c.SourcePR]++
		conflictFootprint[c.TargetPR]++
	}

	// Process duplicate groups
	var plans []types.DuplicateSynthesisPlan

	for _, dup := range duplicates {
		plan := buildSynthesisPlanForGroup(
			dup.CanonicalPRNumber,
			dup.DuplicatePRNums,
			dup.Similarity,
			dup.Reason,
			"duplicate",
			prByNumber,
			reviewByNumber,
			conflictFootprint,
		)
		plans = append(plans, plan)
	}

	// Process overlap groups
	for _, ov := range overlaps {
		plan := buildSynthesisPlanForGroup(
			ov.CanonicalPRNumber,
			ov.DuplicatePRNums,
			ov.Similarity,
			ov.Reason,
			"overlap",
			prByNumber,
			reviewByNumber,
			conflictFootprint,
		)
		plans = append(plans, plan)
	}

	// Sort plans by group ID for stable output
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].GroupID < plans[j].GroupID
	})

	return plans
}

// synthesisScoredCandidate is a helper type for ranking synthesis candidates.
type synthesisScoredCandidate struct {
	prNumber  int
	score     float64
	candidate types.DuplicateSynthesisCandidate
}

// buildSynthesisPlanForGroup creates a synthesis plan for a single duplicate/near-duplicate group.
func buildSynthesisPlanForGroup(
	canonicalPR int,
	duplicatePRs []int,
	similarity float64,
	reason string,
	groupType string,
	prByNumber map[int]types.PR,
	reviewByNumber map[int]types.ReviewResult,
	conflictFootprint map[int]int,
) types.DuplicateSynthesisPlan {
	// Collect all PR numbers in this group
	allPRs := make([]int, 0, 1+len(duplicatePRs))
	allPRs = append(allPRs, canonicalPR)
	for _, d := range duplicatePRs {
		if d > 0 { // Skip negative merged PR numbers
			allPRs = append(allPRs, d)
		}
	}

	// Score each candidate
	var scored []synthesisScoredCandidate

	for _, prNum := range allPRs {
		pr, ok := prByNumber[prNum]
		if !ok {
			continue
		}
		candidate, score := scoreSynthesisCandidate(pr, reviewByNumber[prNum], conflictFootprint[prNum])
		scored = append(scored, synthesisScoredCandidate{prNumber: prNum, score: score, candidate: candidate})
	}

	// Sort by synthesis score descending, then by PR number ascending as tiebreaker
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].prNumber < scored[j].prNumber
	})

	// Assign roles based on ranking
	candidates := make([]types.DuplicateSynthesisCandidate, len(scored))
	nominatedCanonical := 0
	if len(scored) > 0 {
		nominatedCanonical = scored[0].prNumber
		for i := range scored {
			c := scored[i].candidate
			switch {
			case i == 0:
				c.Role = "canonical"
			case i == 1 && scored[0].score > 0 && scored[0].score-scored[i].score < 0.15:
				// Within 0.15 of top score → alternate
				c.Role = "alternate"
			default:
				if scored[i].score > 0.3 {
					c.Role = "contributor"
				} else {
					c.Role = "excluded"
				}
			}
			c.SynthesisScore = scored[i].score
			candidates[i] = c
		}
	}

	// Build synthesis notes
	notes := buildSynthesisNotes(scored, candidates)

	return types.DuplicateSynthesisPlan{
		GroupID:              fmt.Sprintf("grp_%d_%s", canonicalPR, groupType),
		GroupType:            groupType,
		OriginalCanonicalPR:  canonicalPR,
		NominatedCanonicalPR: nominatedCanonical,
		Similarity:           similarity,
		Reason:               reason,
		Candidates:           candidates,
		SynthesisNotes:       notes,
	}
}

// scoreSynthesisCandidate computes a synthesis score for a PR based on available signals.
// Score is 0.0-1.0, higher is better for merge-by-bot candidacy.
func scoreSynthesisCandidate(
	pr types.PR,
	review types.ReviewResult,
	conflictCount int,
) (types.DuplicateSynthesisCandidate, float64) {
	var factors []string
	score := 0.0

	// Base quality signals
	substanceScore := 0.0
	if review.SubstanceScore > 0 {
		substanceScore = float64(review.SubstanceScore) / 100.0
		score += substanceScore * 0.25
		factors = append(factors, fmt.Sprintf("substance=%.2f", substanceScore))
	}

	// Confidence signal
	confidenceScore := review.Confidence
	if confidenceScore > 0 {
		score += confidenceScore * 0.20
		factors = append(factors, fmt.Sprintf("confidence=%.2f", confidenceScore))
	}

	// Mergeability signal
	switch pr.Mergeable {
	case "yes":
		score += 0.20
		factors = append(factors, "mergeable=yes")
	case "no":
		// Heavily penalize but don't exclude
		score += 0.05
		factors = append(factors, "mergeable=no")
	default:
		factors = append(factors, "mergeable=unknown")
	}

	// Draft penalty
	if pr.IsDraft {
		score *= 0.7
		factors = append(factors, "draft=true")
	}

	// Conflict footprint: penalize PRs with many conflicts
	conflictPenalty := math.Min(float64(conflictCount)*0.03, 0.15)
	score -= conflictPenalty
	if conflictCount > 0 {
		factors = append(factors, fmt.Sprintf("conflicts=%d", conflictCount))
	}

	// Test evidence bonus
	hasTestEvidence := false
	subsystemTags := make([]string, 0)
	riskyPatterns := make([]string, 0)
	subsystemSeen := make(map[string]struct{})
	patternSeen := make(map[string]struct{})
	for _, finding := range review.AnalyzerFindings {
		findingText := strings.ToLower(finding.Finding)
		if strings.Contains(strings.ToLower(finding.AnalyzerName), "test") ||
			strings.Contains(findingText, "test") {
			hasTestEvidence = true
		}
		if finding.Subsystem != "" {
			if _, ok := subsystemSeen[finding.Subsystem]; !ok {
				subsystemSeen[finding.Subsystem] = struct{}{}
				subsystemTags = append(subsystemTags, finding.Subsystem)
			}
		}
		if finding.SignalType == "risky_pattern" {
			label := strings.TrimSpace(finding.Finding)
			if label == "" {
				label = "risky_pattern"
			}
			if _, ok := patternSeen[label]; !ok {
				patternSeen[label] = struct{}{}
				riskyPatterns = append(riskyPatterns, label)
			}
		}
	}
	if hasTestEvidence {
		score += 0.10
		factors = append(factors, "has_test_evidence=true")
	}

	// Signal quality bonus
	signalQuality := "medium"
	if review.SignalQuality == "high" || review.Confidence >= 0.8 {
		signalQuality = "high"
		score += 0.10
	} else if review.SignalQuality == "low" || review.Confidence < 0.4 {
		signalQuality = "low"
		score -= 0.05
	}
	factors = append(factors, fmt.Sprintf("signal_quality=%s", signalQuality))

	// Ensure score is bounded
	score = math.Max(0, math.Min(1.0, score))

	// Build rationale
	rationale := fmt.Sprintf(
		"Synthesis candidacy based on: substance %.0f%%, confidence %.0f%%, mergeable=%s, conflicts=%d, test_evidence=%v, signal=%s",
		substanceScore*100, confidenceScore*100, pr.Mergeable, conflictCount, hasTestEvidence, signalQuality,
	)

	return types.DuplicateSynthesisCandidate{
		PRNumber:          pr.Number,
		Title:             pr.Title,
		Author:            pr.Author,
		Role:              "canonical", // default, will be overwritten
		SynthesisScore:    score,
		Confidence:        review.Confidence,
		SubstanceScore:    review.SubstanceScore,
		Mergeable:         pr.Mergeable,
		HasTestEvidence:   hasTestEvidence,
		SubsystemTags:     subsystemTags,
		RiskyPatterns:     riskyPatterns,
		ConflictFootprint: conflictCount,
		IsDraft:           pr.IsDraft,
		SignalQuality:     signalQuality,
		ScoringFactors:    factors,
		Rationale:         rationale,
	}, score
}

// buildSynthesisNotes generates human-readable guidance for a future merge bot.
func buildSynthesisNotes(scored []synthesisScoredCandidate, candidates []types.DuplicateSynthesisCandidate) []string {
	var notes []string

	if len(scored) == 0 {
		return notes
	}

	// Note about canonical selection
	if len(scored) > 0 {
		notes = append(notes, fmt.Sprintf(
			"Canonical PR #%d nominated with synthesis score %.2f",
			scored[0].prNumber, scored[0].score,
		))
	}

	// Check if original and nominated canonical differ
	// (this would require knowing original, passed separately if needed)

	// Note about alternates
	alternateCount := 0
	for i, c := range candidates {
		if c.Role == "alternate" {
			alternateCount++
			notes = append(notes, fmt.Sprintf(
				"Alternate candidate: PR #%d (score %.2f) should be preserved as backup",
				scored[i].prNumber, scored[i].score,
			))
		}
	}
	if alternateCount == 0 && len(scored) > 1 {
		notes = append(notes, "No strong alternate candidates identified; canonical PR is the clear best-of-group")
	}

	// Note about conflicts
	maxConflicts := 0
	maxConflictPR := 0
	for i, c := range candidates {
		if c.ConflictFootprint > maxConflicts {
			maxConflicts = c.ConflictFootprint
			maxConflictPR = scored[i].prNumber
		}
	}
	if maxConflicts > 0 {
		notes = append(notes, fmt.Sprintf(
			"PR #%d has the highest conflict footprint (%d conflicts); conflict resolution may be needed before synthesis",
			maxConflictPR, maxConflicts,
		))
	}

	// Note about test coverage
	hasTestEvidenceCount := 0
	for _, c := range candidates {
		if c.HasTestEvidence {
			hasTestEvidenceCount++
		}
	}
	if hasTestEvidenceCount == 0 {
		notes = append(notes, "Warning: no PR in this group has test evidence; synthesized PR may need test coverage added")
	} else {
		notes = append(notes, fmt.Sprintf("%d of %d candidates have test evidence", hasTestEvidenceCount, len(candidates)))
	}

	return notes
}

// buildCollapsedCorpus flattens duplicate/overlap chains into a single canonical
// per connected component. It returns the collapsed corpus and a copy of the PR
// slice annotated with IsCollapsedCanonical and SupersededPRs.
func buildCollapsedCorpus(
	synthesisPlans []types.DuplicateSynthesisPlan,
	prs []types.PR,
) (types.CollapsedCorpus, []types.PR) {
	if len(synthesisPlans) == 0 {
		return types.CollapsedCorpus{}, prs
	}

	// adjacency graph of PRs linked by duplicate/overlap relationships
	adj := make(map[int]map[int]struct{})
	addEdge := func(a, b int) {
		if a == b {
			return
		}
		if adj[a] == nil {
			adj[a] = make(map[int]struct{})
		}
		if adj[b] == nil {
			adj[b] = make(map[int]struct{})
		}
		adj[a][b] = struct{}{}
		adj[b][a] = struct{}{}
	}

	// Track best synthesis score per PR across all groups
	bestScore := make(map[int]float64)

	for _, plan := range synthesisPlans {
		canonical := plan.NominatedCanonicalPR
		if canonical == 0 {
			canonical = plan.OriginalCanonicalPR
		}
		for _, c := range plan.Candidates {
			addEdge(canonical, c.PRNumber)
			if c.SynthesisScore > bestScore[c.PRNumber] {
				bestScore[c.PRNumber] = c.SynthesisScore
			}
		}
	}

	// Find connected components (only components with >1 PR are collapsed groups)
	visited := make(map[int]bool)
	var components [][]int

	var dfs func(node int, comp *[]int)
	dfs = func(node int, comp *[]int) {
		visited[node] = true
		*comp = append(*comp, node)
		for neighbor := range adj[node] {
			if !visited[neighbor] {
				dfs(neighbor, comp)
			}
		}
	}

	for node := range adj {
		if !visited[node] {
			var comp []int
			dfs(node, &comp)
			if len(comp) > 1 {
				components = append(components, comp)
			}
		}
	}

	canonicalToSuperseded := make(map[int][]int)
	supersededToCanonical := make(map[int]int)

	for _, comp := range components {
		// Find best-scored PR in component using synthesis score tiebreaker.
		bestPR := comp[0]
		bestScoreVal := bestScore[bestPR]
		for _, prNum := range comp[1:] {
			if score := bestScore[prNum]; score > bestScoreVal {
				bestPR = prNum
				bestScoreVal = score
			} else if score == bestScoreVal && prNum < bestPR {
				bestPR = prNum
			}
		}

		var superseded []int
		for _, prNum := range comp {
			if prNum != bestPR {
				superseded = append(superseded, prNum)
				supersededToCanonical[prNum] = bestPR
			}
		}
		sort.Ints(superseded)
		canonicalToSuperseded[bestPR] = superseded
	}

	// Annotate PRs
	updatedPRs := make([]types.PR, len(prs))
	copy(updatedPRs, prs)
	for i := range updatedPRs {
		prNum := updatedPRs[i].Number
		if superseded, ok := canonicalToSuperseded[prNum]; ok {
			updatedPRs[i].IsCollapsedCanonical = true
			updatedPRs[i].SupersededPRs = superseded
		} else if canonical, ok := supersededToCanonical[prNum]; ok {
			updatedPRs[i].IsCollapsedCanonical = false
			updatedPRs[i].SupersededPRs = []int{canonical}
		}
	}

	totalSuperseded := 0
	for _, s := range canonicalToSuperseded {
		totalSuperseded += len(s)
	}

	return types.CollapsedCorpus{
		CanonicalToSuperseded: canonicalToSuperseded,
		SupersededToCanonical: supersededToCanonical,
		CollapsedGroupCount:   len(components),
		TotalSuperseded:       totalSuperseded,
	}, updatedPRs
}

func orderSelection(repo string, selected []types.PR) []types.PR {
	if len(selected) == 0 {
		return nil
	}

	g := graph.Build(repo, selected)
	orderedNodes, err := g.TopologicalOrder()
	if err != nil {
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

func buildConflictWarnings(repo string, selected []types.PR) map[int][]string {
	warnings := make(map[int][]string, len(selected))
	g := graph.Build(repo, selected)
	for _, edge := range g.Edges {
		if edge.EdgeType != graph.EdgeTypeConflict {
			continue
		}
		message := fmt.Sprintf("Conflicts with PR #%d", edge.ToPR)
		warnings[edge.FromPR] = appendUniqueString(warnings[edge.FromPR], message)
		message = fmt.Sprintf("Conflicts with PR #%d", edge.FromPR)
		warnings[edge.ToPR] = appendUniqueString(warnings[edge.ToPR], message)
	}

	for prNumber := range warnings {
		sort.Strings(warnings[prNumber])
	}

	return warnings
}

func candidateFromPR(pr types.PR, score float64, warnings []string) types.MergePlanCandidate {
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
		Score:            round(score, 4),
		Rationale:        plannerRationale(pr),
		FilesTouched:     files,
		ConflictWarnings: warnings,
	}
}

// poolPlanCandidateFrom converts a PoolSelector PoolCandidate to a MergePlanCandidate.
func poolPlanCandidateFrom(pc planning.PoolCandidate, warnings []string) types.MergePlanCandidate {
	files := pc.PR.FilesChanged
	if files == nil {
		files = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	return types.MergePlanCandidate{
		PRNumber:         pc.PR.Number,
		Title:            pc.PR.Title,
		Score:            round(pc.PriorityScore, 4),
		Rationale:        formatPoolCandidateRationale(pc),
		Reasons:          pc.ReasonCodes,
		FilesTouched:     files,
		ConflictWarnings: warnings,
	}
}

// formatPoolCandidateRationale formats PoolSelector reason codes into a human-readable rationale.
func formatPoolCandidateRationale(pc planning.PoolCandidate) string {
	if len(pc.ReasonCodes) == 0 {
		return "weighted priority scoring"
	}
	// Map reason codes to human-readable keywords.
	var components []string
	for _, code := range pc.ReasonCodes {
		switch code {
		case "staleness":
			components = append(components, "staleness")
		case "ci_status":
			components = append(components, "ci")
		case "security_label":
			components = append(components, "security")
		case "cluster":
			components = append(components, "cluster")
		case "recency", "time_decay":
			components = append(components, "recency")
		default:
			components = append(components, code)
		}
	}
	if len(components) == 0 {
		return "weighted priority scoring"
	}
	return components[0] + " weighted"
}

// formatPriorityTier converts a priority score to a tier label.
func formatPriorityTier(score float64) string {
	if score >= 0.8 {
		return "critical"
	}
	if score >= 0.6 {
		return "high"
	}
	if score >= 0.4 {
		return "medium"
	}
	return "low"
}

// runHierarchicalPlan executes the HierarchicalPlanner 3-level planning pipeline.
func (s Service) runHierarchicalPlan(
	ctx context.Context,
	repo string,
	prs []types.PR,
	poolCandidateByNum map[int]planning.PoolCandidate,
	telemetry types.OperationTelemetry,
	outRejections *[]types.PlanRejection,
) types.OperationTelemetry {
	if len(prs) == 0 {
		return telemetry
	}

	hierarchyStart := time.Now()
	result := s.hierarchicalPlanner.Plan(ctx, repo, prs, s.timeDecayConfig)
	telemetry.PlanningStrategy = "hierarchical"
	telemetry.HierarchicalComplexityReduction = result.ComplexityReduction

	// Merge hierarchy rejections into the output rejections list.
	for _, hr := range result.Rejections {
		*outRejections = append(*outRejections, types.PlanRejection{
			PRNumber: hr.PRNumber,
			Reason:   hr.Reason,
		})
	}

	// Copy hierarchical stage latencies into telemetry.
	for k, v := range result.Telemetry.StageLatenciesMS {
		if telemetry.StageLatenciesMS == nil {
			telemetry.StageLatenciesMS = make(map[string]int)
		}
		telemetry.StageLatenciesMS[k] = v
	}

	telemetry.StageLatenciesMS["hierarchical_plan_ms"] = int(time.Since(hierarchyStart).Milliseconds())
	return telemetry
}

func plannerPriority(pr types.PR, now time.Time) float64 {
	score := 0.0
	if pr.CIStatus == "success" {
		score += 3
	} else if pr.CIStatus == "pending" || pr.CIStatus == "unknown" {
		score += 1
	} else if pr.CIStatus == "failure" {
		score -= 2
	}

	if pr.ReviewStatus == "approved" {
		score += 2
	} else if pr.ReviewStatus == "changes_requested" {
		score -= 2
	}

	if pr.Mergeable == "mergeable" {
		score += 1
	}

	updatedAt, err := time.Parse(time.RFC3339, pr.UpdatedAt)
	if err == nil {
		ageDays := now.Sub(updatedAt).Hours() / 24
		score += math.Min(ageDays/15, 2)
	}

	if pr.IsBot {
		score += 0.5
	}

	return score
}

func plannerRationale(pr types.PR) string {
	parts := []string{}
	if pr.CIStatus == "success" {
		parts = append(parts, "CI passing")
	}
	if pr.ReviewStatus == "approved" {
		parts = append(parts, "review approved")
	}
	if pr.Mergeable == "mergeable" {
		parts = append(parts, "mergeable")
	}
	if pr.IsBot {
		parts = append(parts, "bot update")
	}
	if len(parts) == 0 {
		parts = append(parts, "selected by heuristic scoring")
	}
	return strings.Join(parts, "; ")
}

func similarity(left, right types.PR) float64 {
	titleScore := util.Jaccard(util.Tokenize(left.Title), util.Tokenize(right.Title))
	bodyScore := util.Jaccard(util.Tokenize(left.Body), util.Tokenize(right.Body))
	fileScore := util.Jaccard(left.FilesChanged, right.FilesChanged)
	if len(left.FilesChanged) == 0 && len(right.FilesChanged) == 0 {
		fileScore = 0.5
	}

	return round((0.6*titleScore)+(0.3*fileScore)+(0.1*bodyScore), 4)
}

func similarityWithMerged(pr types.PR, merged review.MergedPRRecord) float64 {
	titleScore := util.Jaccard(util.Tokenize(pr.Title), util.Tokenize(merged.Title))
	bodyScore := util.Jaccard(util.Tokenize(pr.Body), util.Tokenize(merged.Body))
	fileScore := util.Jaccard(pr.FilesChanged, merged.FilesChanged)
	if len(pr.FilesChanged) == 0 && len(merged.FilesChanged) == 0 {
		fileScore = 0.5
	}

	return round((0.6*titleScore)+(0.3*fileScore)+(0.1*bodyScore), 4)
}

func parseSharedFiles(reason string) []string {
	const prefix = "shared files:"
	if !strings.HasPrefix(reason, prefix) {
		return nil
	}
	raw := strings.TrimSpace(strings.TrimPrefix(reason, prefix))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}
	return files
}

func appendUniqueInt(values []int, candidate int) []int {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func appendUniqueString(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func round(value float64, precision int) float64 {
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func estimatePairwiseShards(poolSize int) int {
	if poolSize <= 0 {
		return 1
	}
	shards := (poolSize + types.PairwiseShardSize - 1) / types.PairwiseShardSize
	if shards < 1 {
		return 1
	}
	return shards
}
