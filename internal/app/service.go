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

type Config struct {
	Now                     func() time.Time
	AllowLive               bool
	UseCacheFirst           bool
	IncludeReview           bool
	Token                   string
	MaxPRs                  int
	BeginningPRNumber       int
	EndingPRNumber          int
	PrecisionMode           string
	DeepCandidateSubsetSize int
	CacheStore              *cache.Store
}

type Service struct {
	now                     func() time.Time
	allowLive               bool
	useCacheFirst           bool
	includeReview           bool
	token                   string
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
		useCacheFirst:           useCacheFirst,
		includeReview:           includeReview,
		token:                   token,
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
	return types.HealthResponse{Status: "ok", Version: version.Version}
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

	log.Info("analysis started", "pr_count", len(prs))

	telemetry := types.OperationTelemetry{
		PoolStrategy:     "heuristic_analysis_pipeline",
		PoolSizeBefore:   len(prs),
		PoolSizeAfter:    len(prs),
		GraphDeltaEdges:  0,
		DecayPolicy:      "none",
		PairwiseShards:   estimatePairwiseShards(len(prs)),
		StageLatenciesMS: map[string]int{},
		StageDropCounts:  map[string]int{},
	}

	clusterStart := time.Now()
	clusters := planner.New().BuildClusters(prs)
	telemetry.StageLatenciesMS["clusters_ms"] = int(time.Since(clusterStart).Milliseconds())

	// Attempt ML-backed clustering via Voyage if configured.
	if s.mlBridge != nil && s.mlBridge.Available() {
		if mlClusters, _, err := s.mlBridge.Cluster(ctx, repoName, prs, logger.RequestIDFromContext(ctx)); err == nil && len(mlClusters) > 0 {
			clusters = mlClusters
		}
	}

	dupStart := time.Now()
	var mergedPRs []review.MergedPRRecord
	if s.cacheStore != nil {
		var err error
		mergedPRs, err = review.FetchMergedPRs(ctx, s.cacheStore, repoName)
		if err != nil {
			log.Warn("failed to fetch merged PRs", "error", err)
		}
	}
	duplicates, overlaps := classifyDuplicates(prs, mergedPRs)
	telemetry.StageLatenciesMS["duplicates_ms"] = int(time.Since(dupStart).Milliseconds())

	// Attempt ML-backed duplicate detection via Voyage if configured.
	if s.mlBridge != nil && s.mlBridge.Available() {
		if mlDups, mlOverlaps, err := s.mlBridge.Duplicates(ctx, repoName, prs, types.DuplicateThreshold, types.OverlapThreshold, logger.RequestIDFromContext(ctx)); err == nil {
			if len(mlDups) > 0 {
				duplicates = mlDups
			}
			if len(mlOverlaps) > 0 {
				overlaps = mlOverlaps
			}
		}
	}
	var conflictProgress func(processed int, total int)
	if meta.LiveSource {
		writeLivePhaseStatus(log, "analysis in progress", len(prs))
		conflictProgress = newLiveAnalysisProgressReporter(log, 100)
	}
	conflictStart := time.Now()
	conflicts := buildConflicts(repoName, prs, conflictProgress)
	telemetry.StageLatenciesMS["conflicts_ms"] = int(time.Since(conflictStart).Milliseconds())
	telemetry.GraphDeltaEdges = len(conflicts)
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
		},
		PRs:              prs,
		Clusters:         clusters,
		Duplicates:       duplicates,
		Overlaps:         overlaps,
		Conflicts:        conflicts,
		StalenessSignals: staleness,
		Telemetry:        &telemetry,
	}

	reviewPayload, err := s.buildReviewPayload(ctx, repoName, response)
	if err != nil {
		log.Warn("review analysis failed", "error", err)
		reviewPayload = &types.ReviewResponse{
			TotalPRs:      len(response.PRs),
			ReviewedPRs:   0,
			Categories:    []types.ReviewCategoryCount{},
			PriorityTiers: []types.PriorityTierCount{},
			Results:       []types.ReviewResult{},
		}
	}
	response.ReviewPayload = reviewPayload

	return response, nil
}

func (s Service) buildReviewPayload(ctx context.Context, repo string, response types.AnalysisResponse) (*types.ReviewResponse, error) {
	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}

	if len(response.PRs) == 0 {
		return &types.ReviewResponse{
			TotalPRs:      0,
			ReviewedPRs:   0,
			Categories:    []types.ReviewCategoryCount{},
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
	log := logger.New("app")
	if ctx != nil {
		log = logger.FromContext(ctx)
	}
	for _, pr := range response.PRs {
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
			Repo:            repo,
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

		result, err := orchestrator.Review(ctx, prData)
		if err != nil {
			// Log but continue — individual PR review failure should not block others
			log.Warn("review failed for PR", "pr", pr.Number, "repo", repo, "err", err)
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

	return &types.ReviewResponse{
		TotalPRs:      len(response.PRs),
		ReviewedPRs:   len(allResults),
		Categories:    categories,
		Buckets:       buckets,
		PriorityTiers: tiers,
		Results:       allResults,
	}, nil
}

func buildReviewBuckets(categoryCount map[types.ReviewCategory]int) []types.BucketCount {
	bucketLabels := map[types.ReviewCategory]string{
		types.ReviewCategoryMergeNow:                "Merge now",
		types.ReviewCategoryMergeAfterFocusedReview: "Merge after focused review",
		types.ReviewCategoryDuplicateSuperseded:     "Duplicate / superseded",
		types.ReviewCategoryProblematicQuarantine:   "Problematic / quarantine",
	}

	bucketCounts := make(map[string]int)
	for cat, label := range bucketLabels {
		bucketCounts[label] = categoryCount[cat]
	}
	bucketCounts["Unknown / escalate"] = categoryCount[types.ReviewCategoryUnknownEscalate] + categoryCount[types.ReviewCategory("")]

	var buckets []types.BucketCount
	for _, label := range []string{
		"Merge now",
		"Merge after focused review",
		"Duplicate / superseded",
		"Problematic / quarantine",
		"Unknown / escalate",
	} {
		buckets = append(buckets, types.BucketCount{Bucket: label, Count: bucketCounts[label]})
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

	// Attempt ML-backed clustering via Voyage if configured.
	if s.mlBridge != nil && s.mlBridge.Available() {
		if mlClusters, mlModel, err := s.mlBridge.Cluster(ctx, repoName, prs, logger.RequestIDFromContext(ctx)); err == nil && len(mlClusters) > 0 {
			clusters = mlClusters
			model = mlModel
		}
	} else if os.Getenv("VOYAGE_API_KEY") != "" {
		if configured := strings.TrimSpace(os.Getenv("VOYAGE_MODEL")); configured != "" {
			model = configured
		} else {
			model = "voyage-code-3"
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
	if target <= 0 {
		target = 20
	}
	if mode == "" {
		mode = formula.ModeCombination
	}

	// Create planner with all configured components
	plannerOpts := []planner.Option{
		planner.WithNow(nowFn()),
	}

	// Load priority weights from settings store and create pool selector if available
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
	if err == nil {
		defer settingsStore.Close()
		repoSettings, err := settingsStore.Get(ctx, repoName)
		if err == nil {
			if weights, ok := planning.PriorityWeightsFromSettings(repoSettings); ok {
				if ps, err := planning.NewPoolSelector(weights); err == nil {
					plannerOpts = append(plannerOpts, planner.WithPoolSelector(ps))
				}
			}
		}
	}

	// Delegate all scoring, filtering, and planning to the planner
	planResp, err := planner.New(plannerOpts...).Plan(ctx, repoName, prs, target, mode)
	if err != nil {
		return types.PlanResponse{}, fmt.Errorf("planner: %w", err)
	}

	// Preserve service-level metadata from loadPRs meta
	planResp.AnalysisTruncated = meta.AnalysisTruncated
	planResp.TruncationReason = meta.TruncationReason
	planResp.MaxPRsApplied = meta.MaxPRsApplied
	planResp.PRWindow = meta.PRWindow
	planResp.PrecisionMode = s.precisionMode
	planResp.DeepCandidateSubsetSize = 0

	return planResp, nil
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

	if targetRepo != manifest.Repo && strings.TrimSpace(s.token) == "" {
		resolved, err := gh.ResolveToken(ctx)
		if err != nil {
			return nil, "", truncationMeta{}, fmt.Errorf("missing auth for live repo %q: %w", targetRepo, err)
		}
		s.token = resolved
	}

	// Try cache first if enabled
	if s.useCacheFirst && s.cacheStore != nil {
		if cachedPRs, ok := s.tryLoadFromCache(targetRepo); ok && len(cachedPRs) > 0 {
			filtered, meta := s.applyIntakeControls(cachedPRs)
			meta.LiveSource = false
			writeLivePhaseStatus(log, "cache loaded, starting analysis", len(filtered))
			if s.mirrorAvailable || s.token != "" {
				s.enrichPRsWithFilesFromMirrorOrGraphQL(ctx, targetRepo, filtered)
			}
			return filtered, targetRepo, meta, nil
		}
	}

	if s.useCacheFirst && !s.allowLive {
		return nil, "", truncationMeta{}, fmt.Errorf("sync first: run `pratc sync --repo=%s` before analyze, or rerun with explicit live override", targetRepo)
	}

	if s.allowLive && s.token != "" {
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
	for i := range prs {
		if len(prs[i].FilesChanged) == 0 {
			files, fileErr := client.FetchPullRequestFiles(ctx, repoID, prs[i].Number)
			if fileErr == nil {
				prs[i].FilesChanged = files
			}
		}
	}
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
	if effectiveMaxPRs == -1 {
		effectiveMaxPRs = types.MaxTarget
	}
	if effectiveMaxPRs > 0 && len(output) > effectiveMaxPRs {
		output = output[:effectiveMaxPRs]
		meta.AnalysisTruncated = true
		meta.TruncationReason = "max_prs_cap"
		meta.MaxPRsApplied = effectiveMaxPRs
	}

	return output, meta
}

func classifyDuplicates(prs []types.PR, mergedPRs []review.MergedPRRecord) ([]types.DuplicateGroup, []types.DuplicateGroup) {
	duplicatesByCanonical := make(map[int]*types.DuplicateGroup)
	overlapsByCanonical := make(map[int]*types.DuplicateGroup)

	// Compare open PRs against each other
	for i := 0; i < len(prs); i++ {
		for j := i + 1; j < len(prs); j++ {
			score := similarity(prs[i], prs[j])
			if score < types.OverlapThreshold {
				continue
			}

			canonical := min(prs[i].Number, prs[j].Number)
			other := max(prs[i].Number, prs[j].Number)

			if score > types.DuplicateThreshold {
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

	// Compare open PRs against merged PR history
	for i := 0; i < len(prs); i++ {
		for _, merged := range mergedPRs {
			score := similarityWithMerged(prs[i], merged)
			if score < types.OverlapThreshold {
				continue
			}

			canonical := prs[i].Number
			other := -merged.PRNumber

			if score > types.DuplicateThreshold {
				group := duplicatesByCanonical[canonical]
				if group == nil {
					group = &types.DuplicateGroup{CanonicalPRNumber: canonical, Reason: "title/body/file similarity above duplicate threshold (compared against merged history)"}
					duplicatesByCanonical[canonical] = group
				}
				group.Similarity = maxFloat(group.Similarity, score)
				group.DuplicatePRNums = appendUniqueInt(group.DuplicatePRNums, other)
				continue
			}

			group := overlapsByCanonical[canonical]
			if group == nil {
				group = &types.DuplicateGroup{CanonicalPRNumber: canonical, Reason: "title/body/file similarity in overlap range (compared against merged history)"}
				overlapsByCanonical[canonical] = group
			}
			group.Similarity = maxFloat(group.Similarity, score)
			group.DuplicatePRNums = appendUniqueInt(group.DuplicatePRNums, other)
		}
	}

	duplicates := flattenGroups(duplicatesByCanonical)
	overlaps := flattenGroups(overlapsByCanonical)
	return duplicates, overlaps
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

func buildConflicts(repo string, prs []types.PR, progress func(processed int, total int)) []types.ConflictPair {
	g := graph.BuildWithProgress(repo, prs, progress)
	conflicts := make([]types.ConflictPair, 0)
	for _, edge := range g.Edges {
		if edge.EdgeType != graph.EdgeTypeConflict {
			continue
		}
		files := parseSharedFiles(edge.Reason)
		severity := "medium"
		if len(files) >= 3 {
			severity = "high"
		} else if len(files) == 0 {
			severity = "low"
		}

		conflicts = append(conflicts, types.ConflictPair{
			SourcePR:     edge.FromPR,
			TargetPR:     edge.ToPR,
			ConflictType: "file_overlap",
			FilesTouched: files,
			Severity:     severity,
			Reason:       edge.Reason,
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
