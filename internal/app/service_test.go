package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/ml"
	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestAnalyzeReturnsContractShape(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if response.Repo != manifest.Repo {
		t.Fatalf("repo = %q, want %q", response.Repo, manifest.Repo)
	}
	if response.GeneratedAt == "" {
		t.Fatal("generatedAt should not be empty")
	}
	if len(response.PRs) == 0 {
		t.Fatal("expected PRs in analysis response")
	}
	if len(response.Clusters) == 0 {
		t.Fatal("expected clusters in analysis response")
	}
	if response.Counts.TotalPRs != len(response.PRs) {
		t.Fatalf("total_prs = %d, want %d", response.Counts.TotalPRs, len(response.PRs))
	}
}

func TestAnalyzeIncludesReviewPayloadWhenEnabled(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, IncludeReview: true})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze with review: %v", err)
	}

	if response.ReviewPayload == nil {
		t.Fatal("expected review payload when review is enabled")
	}
	if response.ReviewPayload.TotalPRs != len(response.PRs) {
		t.Fatalf("review total_prs = %d, want %d", response.ReviewPayload.TotalPRs, len(response.PRs))
	}
	if len(response.ReviewPayload.Results) == 0 {
		t.Fatal("expected at least one review result")
	}
}

func TestClusterReturnsThresholdsAndClusters(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})
	response, err := service.Cluster(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}

	if response.Model == "" {
		t.Fatal("model should not be empty")
	}
	if response.Thresholds.Duplicate != 0.85 {
		t.Fatalf("duplicate threshold = %.2f, want 0.85", response.Thresholds.Duplicate)
	}
	if response.Thresholds.Overlap != 0.70 {
		t.Fatalf("overlap threshold = %.2f, want 0.70", response.Thresholds.Overlap)
	}
	if len(response.Clusters) == 0 {
		t.Fatal("expected non-empty clusters")
	}
}

func TestClusterDoesNotAdvertiseConfiguredVoyageModelWhenBridgeUnavailable(t *testing.T) {
	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	t.Setenv("VOYAGE_API_KEY", "test-key")
	t.Setenv("VOYAGE_MODEL", "voyage-honesty-check")

	service := NewService(Config{Now: fixedNow})
	service.mlBridge = &ml.Bridge{}

	response, err := service.Cluster(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}

	if response.Model != "heuristic-fallback" {
		t.Fatalf("expected explicit heuristic fallback model when ML bridge is unavailable, got %q", response.Model)
	}
}

func TestClusterSurfacesDegradationMetadataFromLocalBackend(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	bridge := newLocalBackendMLBridge(t)
	service := NewService(Config{Now: fixedNow})
	service.mlBridge = bridge

	response, err := service.Cluster(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}

	payload := marshalJSONMap(t, response)
	degradation, ok := payload["degradation"].(map[string]any)
	if !ok {
		t.Fatalf("cluster response missing degradation metadata: %v", payload)
	}
	if degradation["fallback_reason"] != "local_backend" {
		t.Fatalf("cluster degradation fallback_reason = %v, want local_backend", degradation["fallback_reason"])
	}
	if degradation["heuristic_fallback"] != true {
		t.Fatalf("cluster degradation heuristic_fallback = %v, want true", degradation["heuristic_fallback"])
	}
}

func TestAnalyzeSurfacesDuplicateDegradationMetadataFromLocalBackend(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	bridge := newLocalBackendMLBridge(t)
	service := NewService(Config{Now: fixedNow})
	service.mlBridge = bridge

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	payload := marshalJSONMap(t, response)
	degradation, ok := payload["duplicate_degradation"].(map[string]any)
	if !ok {
		t.Fatalf("analysis response missing duplicate degradation metadata: %v", payload)
	}
	if degradation["fallback_reason"] != "local_backend" {
		t.Fatalf("duplicate degradation fallback_reason = %v, want local_backend", degradation["fallback_reason"])
	}
	if degradation["heuristic_fallback"] != true {
		t.Fatalf("duplicate degradation heuristic_fallback = %v, want true", degradation["heuristic_fallback"])
	}
}

func TestGraphReturnsDOT(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})
	response, err := service.Graph(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	if response.DOT == "" {
		t.Fatal("dot output should not be empty")
	}
	if response.DOT[:7] != "digraph" {
		t.Fatalf("dot should start with digraph, got %q", response.DOT[:7])
	}
	if len(response.Nodes) == 0 {
		t.Fatal("graph should include nodes")
	}
}

func TestPlanReturnsTargetedOrdering(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, DynamicTarget: DynamicTargetConfig{Enabled: false}})
	response, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if response.Target != 5 {
		t.Fatalf("target = %d, want 5", response.Target)
	}
	if response.CandidatePoolSize == 0 {
		t.Fatal("candidate pool size should be greater than zero")
	}
	if len(response.Ordering) == 0 {
		t.Fatal("ordering should not be empty")
	}
	if len(response.Ordering) > 5 {
		t.Fatalf("ordering size = %d, expected <= 5", len(response.Ordering))
	}
}

func TestAnalyzeAddsPRWindowTruncationMetadata(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, BeginningPRNumber: 12, EndingPRNumber: 18})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze with window: %v", err)
	}

	if !response.AnalysisTruncated {
		t.Fatal("expected analysis_truncated to be true when window is applied")
	}
	if response.TruncationReason != "pr_window" {
		t.Fatalf("expected truncation_reason pr_window, got %q", response.TruncationReason)
	}
	if response.PRWindow == nil {
		t.Fatal("expected pr_window metadata")
	}
	if response.PRWindow.BeginningPRNumber != 12 || response.PRWindow.EndingPRNumber != 18 {
		t.Fatalf("unexpected pr window metadata: %+v", response.PRWindow)
	}
	for _, pr := range response.PRs {
		if pr.Number < 12 || pr.Number > 18 {
			t.Fatalf("pr number %d outside window", pr.Number)
		}
	}
}

func TestAnalyzeAddsMaxPRsMetadata(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, MaxPRs: 5})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze with max prs: %v", err)
	}

	if len(response.PRs) != 5 {
		t.Fatalf("expected 5 prs after cap, got %d", len(response.PRs))
	}
	if !response.AnalysisTruncated {
		t.Fatal("expected analysis_truncated true when max_prs cap is applied")
	}
	if response.TruncationReason != "max_prs_cap" {
		t.Fatalf("expected truncation_reason max_prs_cap, got %q", response.TruncationReason)
	}
	if response.MaxPRsApplied != 5 {
		t.Fatalf("expected max_prs_applied 5, got %d", response.MaxPRsApplied)
	}
}

func TestAnalyzeDoesNotTruncateByDefaultWhenMaxPRsUnset(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})
	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze without max prs: %v", err)
	}
	if response.AnalysisTruncated {
		t.Fatalf("expected no truncation when MaxPRs is unset, got reason %q", response.TruncationReason)
	}
	if response.TruncationReason != "" {
		t.Fatalf("expected empty truncation reason when MaxPRs is unset, got %q", response.TruncationReason)
	}
}

func TestAnalyzeProvidesAuthFallbackGuidanceForLiveRepoWithoutToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PATH", t.TempDir())

	service := Service{
		now:           fixedNow,
		allowLive:     true,
		useCacheFirst: false,
		includeReview: false,
		token:         "",
	}
	_, err := service.Analyze(context.Background(), "openclaw/openclaw")
	if err == nil {
		t.Fatal("expected auth guidance error")
	}
	message := err.Error()
	if !strings.Contains(message, "GITHUB_TOKEN") {
		t.Fatalf("expected GITHUB_TOKEN guidance, got %q", message)
	}
	if !strings.Contains(message, "gh auth login") {
		t.Fatalf("expected gh auth login guidance, got %q", message)
	}
}

func TestAnalyzeBlocksLiveFetchWithoutRecentSyncAndWithoutLiveOverride(t *testing.T) {
	t.Parallel()

	store := openTestCache(t)
	service := Service{
		now:           fixedNow,
		allowLive:     false,
		useCacheFirst: true,
		token:         "token",
		cacheStore:    store,
		cacheTTL:      time.Hour,
	}

	_, err := service.Analyze(context.Background(), "example/repo")
	if err == nil {
		t.Fatal("expected sync-first error")
	}
	message := err.Error()
	if !strings.Contains(message, "sync") || !strings.Contains(message, "analyze") {
		t.Fatalf("expected sync-first guidance, got %q", message)
	}
	if !strings.Contains(message, "pratc sync --repo=example/repo") {
		t.Fatalf("expected sync command guidance, got %q", message)
	}
}

func TestAnalyzeAllowsExplicitLiveOverrideWhenCacheIsMissing(t *testing.T) {
	t.Parallel()

	store := openTestCache(t)
	service := Service{
		now:           fixedNow,
		allowLive:     true,
		useCacheFirst: true,
		token:         "token",
		cacheStore:    store,
		cacheTTL:      time.Hour,
	}

	_, err := service.Analyze(context.Background(), "not-a-repo")
	if err == nil {
		t.Fatal("expected live fetch attempt to fail for invalid repo")
	}
	message := err.Error()
	if strings.Contains(message, "sync first") {
		t.Fatalf("expected explicit live override to skip sync-first error, got %q", message)
	}
	if !strings.Contains(message, "invalid repo") && !strings.Contains(message, "no fixture data") {
		t.Fatalf("expected live override path to proceed past sync gate, got %q", message)
	}
}

func TestNewServiceDoesNotEnableLiveAccessFromTokenOnly(t *testing.T) {
	t.Parallel()

	service := NewService(Config{Now: fixedNow, Token: "token"})
	if service.allowLive {
		t.Fatal("expected token alone to not enable live access")
	}
}

func TestAnalyzeUsesFreshCacheWhenSyncIsRecent(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	prs, err := testutil.LoadFixturePRs()
	if err != nil {
		t.Fatalf("load fixture prs: %v", err)
	}

	store := openTestCache(t)
	seedCachedPRs(t, store, manifest.Repo, prs, fixedNow())

	service := Service{
		now:           fixedNow,
		allowLive:     false,
		useCacheFirst: true,
		cacheStore:    store,
		cacheTTL:      time.Hour,
	}

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze from cache: %v", err)
	}
	if response.Repo != manifest.Repo {
		t.Fatalf("repo = %q, want %q", response.Repo, manifest.Repo)
	}
	if len(response.PRs) == 0 {
		t.Fatal("expected cached analysis to return prs")
	}
}

func TestClusterAllowsForceCacheWhenAnalyzeOutlastsCacheTTL(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	prs, err := testutil.LoadFixturePRs()
	if err != nil {
		t.Fatalf("load fixture prs: %v", err)
	}

	store := openTestCache(t)
	start := fixedNow()
	seedCachedPRs(t, store, manifest.Repo, prs, start)

	staleNow := func() time.Time { return start.Add(2 * time.Hour) }
	service := Service{
		now:             staleNow,
		allowLive:       false,
		allowForceCache: true,
		useCacheFirst:   true,
		cacheStore:      store,
		cacheTTL:        time.Hour,
	}

	response, err := service.Cluster(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("cluster with force-cache fallback: %v", err)
	}
	if response.Repo != manifest.Repo {
		t.Fatalf("repo = %q, want %q", response.Repo, manifest.Repo)
	}
	if len(response.Clusters) == 0 {
		t.Fatal("expected clusters from stale cache fallback")
	}
}

func TestAnalyzeAndPlanIncludeTelemetryContracts(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})
	analysis, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if analysis.Telemetry.PoolStrategy == "" {
		t.Fatal("expected analysis telemetry pool_strategy")
	}
	if analysis.Telemetry.PoolSizeBefore <= 0 {
		t.Fatalf("analysis pool_size_before = %d, want > 0", analysis.Telemetry.PoolSizeBefore)
	}
	if analysis.Telemetry.PoolSizeAfter <= 0 {
		t.Fatalf("analysis pool_size_after = %d, want > 0", analysis.Telemetry.PoolSizeAfter)
	}

	plan, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Telemetry.PoolStrategy == "" {
		t.Fatal("expected plan telemetry pool_strategy")
	}
	if plan.Telemetry.GraphDeltaEdges < 0 {
		t.Fatalf("plan graph_delta_edges = %d, want >= 0", plan.Telemetry.GraphDeltaEdges)
	}
	if plan.Telemetry.PairwiseShards <= 0 {
		t.Fatalf("plan pairwise_shards = %d, want > 0", plan.Telemetry.PairwiseShards)
	}
	if len(plan.Telemetry.StageDropCounts) == 0 {
		t.Fatal("expected plan stage drop counts telemetry")
	}
}

func TestPlanDoesNotCapCandidatePoolByDefault(t *testing.T) {
	t.Parallel()

	store := openTestCache(t)
	repo := "owner/repo"
	now := fixedNow()
	for i := 1; i <= 100; i++ {
		if err := store.UpsertPR(types.PR{
			Repo:         repo,
			Number:       i,
			Title:        fmt.Sprintf("PR %d", i),
			URL:          types.GitHubURLPrefix + repo + "/pull/" + fmt.Sprint(i),
			Author:       "octocat",
			BaseBranch:   "main",
			HeadBranch:   "feature",
			CIStatus:     "success",
			ReviewStatus: "approved",
			Mergeable:    "mergeable",
			UpdatedAt:    now.Format(time.RFC3339),
			CreatedAt:    now.Add(-time.Hour).Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}
	if err := store.SetLastSync(repo, now); err != nil {
		t.Fatalf("set last sync: %v", err)
	}

	service := NewService(Config{Now: fixedNow, CacheStore: store, UseCacheFirst: true, DynamicTarget: DynamicTargetConfig{Enabled: false}})
	response, err := service.Plan(context.Background(), repo, 100, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if response.CandidatePoolSize != 100 {
		t.Fatalf("candidate_pool_size = %d, want 100", response.CandidatePoolSize)
	}
}

func TestAnalyzeDeepPrecisionHasRicherConflictFileDetailThanFast(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	fastService := NewService(Config{Now: fixedNow, PrecisionMode: "fast"})
	fastResponse, err := fastService.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze fast: %v", err)
	}

	deepService := NewService(Config{Now: fixedNow, PrecisionMode: "deep"})
	deepResponse, err := deepService.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze deep: %v", err)
	}

	if fastResponse.PrecisionMode != "fast" {
		t.Fatalf("expected fast precision metadata, got %q", fastResponse.PrecisionMode)
	}
	if deepResponse.PrecisionMode != "deep" {
		t.Fatalf("expected deep precision metadata, got %q", deepResponse.PrecisionMode)
	}
	if deepResponse.DeepCandidateSubsetSize <= 0 {
		t.Fatalf("expected deep candidate subset size > 0, got %d", deepResponse.DeepCandidateSubsetSize)
	}

	fastEntries := totalConflictFileEntries(fastResponse.Conflicts)
	deepEntries := totalConflictFileEntries(deepResponse.Conflicts)
	if deepEntries <= fastEntries {
		t.Fatalf("expected deep conflict file detail to exceed fast mode (deep=%d fast=%d)", deepEntries, fastEntries)
	}
}

func TestAnalyzeCompletes6kCachedPRsWithinFiveMinutes(t *testing.T) {
	if !runSixKPerfProof() {
		t.Skip("set PRATC_ENABLE_6K_PERF_TEST=1 to run the 6k analyze perf proof")
	}

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	store := openTestCache(t)
	seedCachedPRs(t, store, manifest.Repo, syntheticAnalysisPRs(manifest.Repo, 6000), fixedNow())

	service := Service{
		now:           fixedNow,
		useCacheFirst: true,
		cacheStore:    store,
		cacheTTL:      time.Hour,
		maxPRs:        6000,
	}

	start := time.Now()
	response, err := service.Analyze(context.Background(), manifest.Repo)
	elapsed := time.Since(start)
	t.Logf("6k analysis: elapsed=%s total_prs=%d clusters=%d conflicts=%d", elapsed, len(response.PRs), response.Counts.ClusterCount, response.Counts.ConflictPairs)
	if err != nil {
		t.Fatalf("analyze 6k cached prs: %v", err)
	}
	if len(response.PRs) != 6000 {
		t.Fatalf("expected 6000 prs after explicit cap override, got %d", len(response.PRs))
	}
	if elapsed > 5*time.Minute {
		t.Fatalf("6k analysis took %s, want < 5m", elapsed)
	}
}

func totalConflictFileEntries(conflicts []types.ConflictPair) int {
	total := 0
	for _, conflict := range conflicts {
		total += len(conflict.FilesTouched)
	}
	return total
}

func syntheticAnalysisPRs(repo string, count int) []types.PR {
	prs := make([]types.PR, 0, count)
	for i := 1; i <= count; i++ {
		now := fixedNow()
		prs = append(prs, types.PR{
			Repo:              repo,
			Number:            i,
			Title:             fmt.Sprintf("release %04d", i),
			Body:              fmt.Sprintf("note %04d", i),
			URL:               fmt.Sprintf("https://example.invalid/%s/pull/%d", repo, i),
			Author:            "deterministic-bot",
			FilesChanged:      []string{fmt.Sprintf("synthetic/file-%04d.go", i)},
			CIStatus:          "success",
			Mergeable:         "mergeable",
			BaseBranch:        "main",
			HeadBranch:        fmt.Sprintf("synthetic-%04d", i),
			CreatedAt:         now.Add(-time.Hour).Format(time.RFC3339),
			UpdatedAt:         now.Add(-time.Minute).Format(time.RFC3339),
			Additions:         1,
			Deletions:         1,
			ChangedFilesCount: 1,
		})
	}
	return prs
}

func runSixKPerfProof() bool {
	value := strings.TrimSpace(os.Getenv("PRATC_ENABLE_6K_PERF_TEST"))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func TestLiveProgressReporterEmitsMilestones(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logger.NewForTest(&buf, "test")
	reporter := newLiveProgressReporter(log, 100)
	for i := 1; i <= 250; i++ {
		reporter(i, 0)
	}

	output := buf.String()
	if !strings.Contains(output, `"fetched":1`) {
		t.Fatalf("expected first progress milestone, got %q", output)
	}
	if !strings.Contains(output, `"fetched":100`) {
		t.Fatalf("expected 100 milestone, got %q", output)
	}
	if !strings.Contains(output, `"fetched":200`) {
		t.Fatalf("expected 200 milestone, got %q", output)
	}
	if strings.Contains(output, `"fetched":99`) {
		t.Fatalf("unexpected non-milestone progress entry: %q", output)
	}
}

func TestWriteLivePhaseStatusIncludesCount(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logger.NewForTest(&buf, "test")
	writeLivePhaseStatus(log, "fetch complete, starting analysis", 5959)

	output := buf.String()
	if !strings.Contains(output, `"phase":"fetch complete, starting analysis"`) {
		t.Fatalf("expected phase status message, got %q", output)
	}
	if !strings.Contains(output, `"pr_count":5959`) {
		t.Fatalf("expected count in phase status message, got %q", output)
	}
}

func TestLiveAnalysisProgressReporterEmitsMilestones(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logger.NewForTest(&buf, "test")
	reporter := newLiveAnalysisProgressReporter(log, 100)
	for i := 1; i <= 250; i++ {
		reporter(i, 250)
	}

	output := buf.String()
	if !strings.Contains(output, `"processed":1`) || !strings.Contains(output, `"total":250`) {
		t.Fatalf("expected first progress milestone, got %q", output)
	}
	if !strings.Contains(output, `"processed":100`) {
		t.Fatalf("expected 100 milestone, got %q", output)
	}
	if !strings.Contains(output, `"processed":200`) {
		t.Fatalf("expected 200 milestone, got %q", output)
	}
	if !strings.Contains(output, `"processed":250`) {
		t.Fatalf("expected final completion milestone, got %q", output)
	}
	if strings.Contains(output, `"processed":99`) {
		t.Fatalf("unexpected non-milestone progress entry: %q", output)
	}
}

func marshalJSONMap(t *testing.T, value any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	return payload
}

func newLocalBackendMLBridge(t *testing.T) *ml.Bridge {
	t.Helper()

	workDir := t.TempDir()
	python := filepath.Join(workDir, "fake-python.sh")
	script := `#!/bin/sh
payload=$(cat)
case "$payload" in
  *'"action":"cluster"'*)
    printf '%s' '{"model":"heuristic-fallback","degradation":{"fallback_reason":"local_backend","heuristic_fallback":true},"clusters":[{"cluster_id":"local-backend-cluster","cluster_label":"Local backend cluster","summary":"heuristic fallback cluster","pr_ids":[1],"health_status":"healthy","average_similarity":1.0,"sample_titles":["Local backend cluster"]}]}'
    ;;
  *'"action":"duplicates"'*)
    printf '%s' '{"degradation":{"fallback_reason":"local_backend","heuristic_fallback":true},"duplicates":[{"canonical_pr_number":1,"duplicate_pr_numbers":[2],"similarity":0.99,"reason":"heuristic local backend duplicate"}],"overlaps":[]}'
    ;;
  *)
    printf '%s' '{"analyzers":[]}'
    ;;
esac
`
	if err := os.WriteFile(python, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake python: %v", err)
	}

	return ml.NewBridge(ml.Config{
		Python:  python,
		WorkDir: workDir,
		Timeout: time.Second,
	})
}

func fixedNow() time.Time {
	return time.Date(2026, time.March, 19, 10, 0, 0, 0, time.UTC)
}

func openTestCache(t *testing.T) *cache.Store {
	t.Helper()

	store, err := cache.Open(filepath.Join(t.TempDir(), "pratc.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close cache: %v", err)
		}
	})
	return store
}

func seedCachedPRs(t *testing.T, store *cache.Store, repo string, prs []types.PR, syncedAt time.Time) {
	t.Helper()

	for _, pr := range prs {
		if pr.Repo != repo {
			continue
		}
		if err := store.UpsertPR(pr); err != nil {
			t.Fatalf("seed cached pr %d: %v", pr.Number, err)
		}
	}
	if err := store.SetLastSync(repo, syncedAt); err != nil {
		t.Fatalf("seed last sync: %v", err)
	}
}
