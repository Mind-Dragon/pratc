package graph

import (
	"slices"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestIncrementalGraph_FullRebuildCreatesGraph(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixtureIncrementalPR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
		fixtureIncrementalPR(102, "main", "feature-c", []string{"pkg/a.go", "pkg/c.go"}, "conflicting"),
	}

	ig := NewIncrementalGraph("acme/repo")
	graph, delta := ig.Update(prs)

	if len(graph.Nodes) != 3 {
		t.Fatalf("Update() node count = %d, want 3", len(graph.Nodes))
	}

	if len(graph.Edges) == 0 {
		t.Fatalf("Update() created no edges, want > 0")
	}

	if delta.EdgesAdded != len(graph.Edges) {
		t.Fatalf("delta.EdgesAdded = %d, want %d", delta.EdgesAdded, len(graph.Edges))
	}
}

func TestIncrementalGraph_DetectsFingerprintChange(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")

	_, exists := ig.fingerprints[prs[0].Number]
	if exists {
		t.Fatal("Expected PR to not exist before update")
	}

	ig.Update(prs)

	if ig.FingerprintChanged(prs[0]) {
		t.Fatal("FingerprintChanged() = true for unchanged PR, want false")
	}

	modifiedPR := fixtureIncrementalPR(100, "main", "feature-a-v2", []string{"pkg/a.go"}, "mergeable")
	if !ig.FingerprintChanged(modifiedPR) {
		t.Fatal("FingerprintChanged() = false for modified PR, want true")
	}
}

func TestIncrementalGraph_IncrementalUpdateRecomputesAffectedEdges(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixtureIncrementalPR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
		fixtureIncrementalPR(102, "main", "feature-c", []string{"pkg/c.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	_, firstDelta := ig.Update(prs)

	if firstDelta.EdgesAdded == 0 {
		t.Fatal("First update should add edges")
	}

	modifiedPR := fixtureIncrementalPR(100, "main", "feature-a-v2", []string{"pkg/a.go", "pkg/d.go"}, "mergeable")
	updatedPRs := []types.PR{
		modifiedPR,
		prs[1],
		prs[2],
	}

	_, secondDelta := ig.Update(updatedPRs)

	if len(secondDelta.UpdatedNodes) == 0 {
		t.Fatal("Incremental update should report node updates")
	}

	cachedEdges, fingerprintCount, needsRebuild := ig.GetCacheStats()
	if cachedEdges == 0 {
		t.Fatal("Expected cached edges after update")
	}
	if fingerprintCount != 3 {
		t.Fatalf("fingerprintCount = %d, want 3", fingerprintCount)
	}
	if needsRebuild {
		t.Fatal("Did not expect needsRebuild flag")
	}
}

func TestIncrementalGraph_InvalidateTriggersFullRebuild(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixtureIncrementalPR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	ig.Update(prs)

	ig.Invalidate()

	_, delta := ig.Update(prs)

	if _, _, needsRebuild := ig.GetCacheStats(); needsRebuild {
		t.Fatal("Expected needsRebuild to be cleared after full rebuild")
	}

	if delta.EdgesAdded == 0 {
		t.Fatal("Full rebuild should report edges added")
	}
}

func TestIncrementalGraph_ChangedPRsTriggerEdgeRecomputation(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixtureIncrementalPR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	_, firstDelta := ig.Update(prs)

	if firstDelta.EdgesAdded == 0 {
		t.Fatal("First update should add edges")
	}

	modifiedPR := fixtureIncrementalPR(100, "main", "feature-a-v2", []string{"pkg/a.go", "pkg/d.go"}, "mergeable")
	updatedPRs := []types.PR{modifiedPR, prs[1]}

	_, secondDelta := ig.Update(updatedPRs)

	if len(secondDelta.RemovedEdges) == 0 {
		t.Fatal("Expected some edges to be removed when PR changed")
	}
}

func TestIncrementalGraph_DeltaTracking(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixtureIncrementalPR(101, "main", "feature-b", []string{"pkg/b.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	_, firstDelta := ig.Update(prs)

	if firstDelta.EdgesAdded != len(firstDelta.AddedEdges) {
		t.Fatalf("EdgesAdded (%d) != len(AddedEdges) (%d)", firstDelta.EdgesAdded, len(firstDelta.AddedEdges))
	}

	if len(firstDelta.RemovedEdges) != 0 {
		t.Fatalf("Expected no removed edges on first build, got %d", len(firstDelta.RemovedEdges))
	}

	modifiedPR := fixtureIncrementalPR(100, "main", "feature-a-v2", []string{"pkg/a.go", "pkg/d.go"}, "mergeable")
	updatedPRs := []types.PR{modifiedPR, prs[1]}
	_, secondDelta := ig.Update(updatedPRs)

	if len(secondDelta.RemovedEdges) > 0 && secondDelta.EdgesRemoved != len(secondDelta.RemovedEdges) {
		t.Fatalf("EdgesRemoved (%d) != len(RemovedEdges) (%d)", secondDelta.EdgesRemoved, len(secondDelta.RemovedEdges))
	}
}

func TestIncrementalGraph_GetGraphReturnsCurrentState(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	ig.Update(prs)

	graph := ig.GetGraph()
	if len(graph.Nodes) != 1 {
		t.Fatalf("GetGraph() returned %d nodes, want 1", len(graph.Nodes))
	}
	if graph.Repo != "acme/repo" {
		t.Fatalf("GetGraph() repo = %q, want %q", graph.Repo, "acme/repo")
	}
}

func TestIncrementalGraph_Clear(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	ig.Update(prs)
	ig.Clear()

	graph := ig.GetGraph()
	if len(graph.Nodes) != 0 {
		t.Fatalf("Clear() left %d nodes, want 0", len(graph.Nodes))
	}
	if len(graph.Edges) != 0 {
		t.Fatalf("Clear() left %d edges, want 0", len(graph.Edges))
	}

	cached, fps, needsRebuild := ig.GetCacheStats()
	if cached != 0 {
		t.Fatalf("Clear() left %d cached edges, want 0", cached)
	}
	if fps != 0 {
		t.Fatalf("Clear() left %d fingerprints, want 0", fps)
	}
	if needsRebuild {
		t.Fatal("Clear() left needsRebuild=true, want false")
	}
}

func TestIncrementalGraph_TelemetryIncludesTiming(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixtureIncrementalPR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	graph, _ := ig.Update(prs)

	if _, ok := graph.Telemetry.StageLatenciesMS["full_rebuild_ms"]; !ok {
		t.Fatal("Expected full_rebuild_ms telemetry")
	}

	modifiedPR := fixtureIncrementalPR(100, "main", "feature-a-v2", []string{"pkg/a.go"}, "mergeable")
	updatedPRs := []types.PR{modifiedPR, prs[1]}
	graph, _ = ig.Update(updatedPRs)

	if _, ok := graph.Telemetry.StageLatenciesMS["incremental_update_ms"]; !ok {
		t.Fatal("Expected incremental_update_ms telemetry")
	}
	if graph.Telemetry.StageDropCounts["changed_prs"] != 1 {
		t.Fatalf("Expected changed_prs=1, got %d", graph.Telemetry.StageDropCounts["changed_prs"])
	}
}

func TestIncrementalGraph_NewPRsTriggerEdgeAdditions(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
	}

	ig := NewIncrementalGraph("acme/repo")
	ig.Update(prs)

	newPR := fixtureIncrementalPR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable")
	updatedPRs := append(slices.Clone(prs), newPR)

	_, delta := ig.Update(updatedPRs)

	if len(delta.UpdatedNodes) == 0 {
		t.Fatal("Expected node update when adding new PR")
	}

	cached, fps, _ := ig.GetCacheStats()
	if cached == 0 {
		t.Fatal("Expected cached edges after adding PR")
	}
	if fps != 2 {
		t.Fatalf("Expected 2 fingerprints, got %d", fps)
	}
}

func TestIncrementalGraph_DeterministicEdgeOrdering(t *testing.T) {
	prs := []types.PR{
		fixtureIncrementalPR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixtureIncrementalPR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
		fixtureIncrementalPR(102, "main", "feature-c", []string{"pkg/c.go"}, "mergeable"),
	}

	ig1 := NewIncrementalGraph("acme/repo")
	graph1, _ := ig1.Update(slices.Clone(prs))

	ig2 := NewIncrementalGraph("acme/repo")
	graph2, _ := ig2.Update(slices.Clone(prs))

	if len(graph1.Edges) != len(graph2.Edges) {
		t.Fatalf("Edge counts differ: %d vs %d", len(graph1.Edges), len(graph2.Edges))
	}

	for i := range graph1.Edges {
		if graph1.Edges[i] != graph2.Edges[i] {
			t.Fatalf("Edge %d differs: %#v vs %#v", i, graph1.Edges[i], graph2.Edges[i])
		}
	}
}

func TestComputeFingerprint_DeterministicHash(t *testing.T) {
	pr1 := types.PR{
		Number:       100,
		BaseBranch:   "main",
		HeadBranch:   "feature-a",
		FilesChanged: []string{"pkg/a.go", "pkg/b.go"},
		Mergeable:    "mergeable",
		ClusterID:    "cluster-1",
		UpdatedAt:    "2024-01-01",
	}

	pr2 := types.PR{
		Number:       100,
		BaseBranch:   "main",
		HeadBranch:   "feature-a",
		FilesChanged: []string{"pkg/b.go", "pkg/a.go"},
		Mergeable:    "mergeable",
		ClusterID:    "cluster-1",
		UpdatedAt:    "2024-01-01",
	}

	fp1 := ComputeFingerprint(pr1)
	fp2 := ComputeFingerprint(pr2)

	if fp1.FilesHash != fp2.FilesHash {
		t.Fatalf("FilesHash differs for same files in different order: %q vs %q", fp1.FilesHash, fp2.FilesHash)
	}
}

func TestComputeFingerprint_DifferentFiles(t *testing.T) {
	pr1 := types.PR{
		Number:       100,
		BaseBranch:   "main",
		HeadBranch:   "feature-a",
		FilesChanged: []string{"pkg/a.go"},
		Mergeable:    "mergeable",
	}

	pr2 := types.PR{
		Number:       100,
		BaseBranch:   "main",
		HeadBranch:   "feature-a",
		FilesChanged: []string{"pkg/b.go"},
		Mergeable:    "mergeable",
	}

	fp1 := ComputeFingerprint(pr1)
	fp2 := ComputeFingerprint(pr2)

	if fp1.FilesHash == fp2.FilesHash {
		t.Fatal("Expected different hashes for different files")
	}
}

func fixtureIncrementalPR(number int, baseBranch, headBranch string, files []string, mergeable string) types.PR {
	return types.PR{
		ID:                "acme/repo",
		Repo:              "acme/repo",
		Number:            number,
		Title:             "PR " + string(rune('A'+number%26)),
		BaseBranch:        baseBranch,
		HeadBranch:        headBranch,
		FilesChanged:      files,
		ChangedFilesCount: len(files),
		CIStatus:          "success",
		Mergeable:         mergeable,
		UpdatedAt:         "2024-01-01",
	}
}

func BenchmarkIncrementalGraph_FullRebuild_1000PRs(b *testing.B) {
	prs := make([]types.PR, 1000)
	for i := 0; i < 1000; i++ {
		prs[i] = types.PR{
			Number:       100 + i,
			BaseBranch:   "main",
			HeadBranch:   "feature-" + string(rune('a'+i%26)),
			FilesChanged: []string{"pkg/a.go"},
			Mergeable:    "mergeable",
			UpdatedAt:    "2024-01-01",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ig := NewIncrementalGraph("acme/repo")
		ig.Update(prs)
	}
}

func BenchmarkIncrementalGraph_IncrementalUpdate_1000PRs_10Changed(b *testing.B) {
	prs := make([]types.PR, 1000)
	for i := 0; i < 1000; i++ {
		prs[i] = types.PR{
			Number:       100 + i,
			BaseBranch:   "main",
			HeadBranch:   "feature-" + string(rune('a'+i%26)),
			FilesChanged: []string{"pkg/a.go"},
			Mergeable:    "mergeable",
			UpdatedAt:    "2024-01-01",
		}
	}

	ig := NewIncrementalGraph("acme/repo")
	ig.Update(prs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			prs[j].HeadBranch = "feature-" + string(rune('a'+j%26)) + "-v2"
			prs[j].UpdatedAt = "2024-01-02"
		}
		ig.Update(prs)
	}
}
