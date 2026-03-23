package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/formula"
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
	if response.Thresholds.Duplicate != 0.90 {
		t.Fatalf("duplicate threshold = %.2f, want 0.90", response.Thresholds.Duplicate)
	}
	if response.Thresholds.Overlap != 0.70 {
		t.Fatalf("overlap threshold = %.2f, want 0.70", response.Thresholds.Overlap)
	}
	if len(response.Clusters) == 0 {
		t.Fatal("expected non-empty clusters")
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

	service := NewService(Config{Now: fixedNow})
	response, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination, true)
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

func TestAnalyzeProvidesAuthFallbackGuidanceForLiveRepoWithoutToken(t *testing.T) {
	t.Parallel()

	service := NewService(Config{Now: fixedNow, AllowLive: true, Token: ""})
	_, err := service.Analyze(context.Background(), "openclaw/openclaw")
	if err == nil {
		t.Fatal("expected auth guidance error")
	}
	message := err.Error()
	if !strings.Contains(message, "GH_TOKEN") {
		t.Fatalf("expected GH_TOKEN guidance, got %q", message)
	}
	if !strings.Contains(message, "gh auth token") {
		t.Fatalf("expected gh auth token guidance, got %q", message)
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

func totalConflictFileEntries(conflicts []types.ConflictPair) int {
	total := 0
	for _, conflict := range conflicts {
		total += len(conflict.FilesTouched)
	}
	return total
}

func TestLiveProgressReporterEmitsMilestones(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter := newLiveProgressReporter(&out, 100)
	for i := 1; i <= 250; i++ {
		reporter(i, 0)
	}

	log := out.String()
	if !strings.Contains(log, "fetched 1 PRs") {
		t.Fatalf("expected first progress milestone, got %q", log)
	}
	if !strings.Contains(log, "fetched 100 PRs") {
		t.Fatalf("expected 100 milestone, got %q", log)
	}
	if !strings.Contains(log, "fetched 200 PRs") {
		t.Fatalf("expected 200 milestone, got %q", log)
	}
	if strings.Contains(log, "fetched 99 PRs") {
		t.Fatalf("unexpected non-milestone progress entry: %q", log)
	}
}

func TestWriteLivePhaseStatusIncludesCount(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	writeLivePhaseStatus(&out, "fetch complete, starting analysis", 5959)

	log := out.String()
	if !strings.Contains(log, "[live] fetch complete, starting analysis") {
		t.Fatalf("expected phase status message, got %q", log)
	}
	if !strings.Contains(log, "5959") {
		t.Fatalf("expected count in phase status message, got %q", log)
	}
}

func TestLiveAnalysisProgressReporterEmitsMilestones(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter := newLiveAnalysisProgressReporter(&out, 100)
	for i := 1; i <= 250; i++ {
		reporter(i, 250)
	}

	log := out.String()
	if !strings.Contains(log, "analyzed 1/250 PRs") {
		t.Fatalf("expected first progress milestone, got %q", log)
	}
	if !strings.Contains(log, "analyzed 100/250 PRs") {
		t.Fatalf("expected 100 milestone, got %q", log)
	}
	if !strings.Contains(log, "analyzed 200/250 PRs") {
		t.Fatalf("expected 200 milestone, got %q", log)
	}
	if !strings.Contains(log, "analyzed 250/250 PRs") {
		t.Fatalf("expected final completion milestone, got %q", log)
	}
	if strings.Contains(log, "analyzed 99/250 PRs") {
		t.Fatalf("unexpected non-milestone progress entry: %q", log)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, time.March, 19, 10, 0, 0, 0, time.UTC)
}
