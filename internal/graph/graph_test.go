package graph

import (
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestBuildCreatesDependencyAndConflictEdges(t *testing.T) {
	prs := []types.PR{
		fixturePR(100, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixturePR(101, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
		fixturePR(102, "main", "feature-c", []string{"pkg/a.go", "pkg/c.go"}, "conflicting"),
		fixturePR(103, "release/1.0", "feature-d", []string{"pkg/a.go"}, "mergeable"),
	}

	graph := Build("acme/repo", prs)

	if len(graph.Nodes) != 4 {
		t.Fatalf("Build() node count = %d, want 4", len(graph.Nodes))
	}

	if !hasEdge(graph.Edges, 100, 101, EdgeTypeDependency) {
		t.Fatalf("Build() missing dependency edge 100 -> 101: %#v", graph.Edges)
	}

	if !hasEdge(graph.Edges, 100, 102, EdgeTypeConflict) {
		t.Fatalf("Build() missing conflict edge 100 -> 102: %#v", graph.Edges)
	}

	if hasEdge(graph.Edges, 100, 103, EdgeTypeConflict) {
		t.Fatalf("Build() created cross-base conflict edge 100 -> 103: %#v", graph.Edges)
	}
}

func TestTopologicalOrderRespectsDependencyEdges(t *testing.T) {
	prs := []types.PR{
		fixturePR(200, "main", "stack-1", []string{"pkg/a.go"}, "mergeable"),
		fixturePR(201, "stack-1", "stack-2", []string{"pkg/b.go"}, "mergeable"),
		fixturePR(202, "stack-2", "stack-3", []string{"pkg/c.go"}, "mergeable"),
		fixturePR(203, "main", "parallel", []string{"pkg/d.go"}, "mergeable"),
	}

	graph := Build("acme/repo", prs)

	order, err := graph.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder() error = %v", err)
	}

	if len(order) != 4 {
		t.Fatalf("TopologicalOrder() len = %d, want 4", len(order))
	}

	if !appearsBefore(order, 200, 201) || !appearsBefore(order, 201, 202) {
		t.Fatalf("TopologicalOrder() = %#v, want stacked PRs in dependency order", order)
	}
}

func TestTopologicalOrderDetectsDependencyCycles(t *testing.T) {
	prs := []types.PR{
		fixturePR(300, "branch-b", "branch-a", []string{"pkg/a.go"}, "mergeable"),
		fixturePR(301, "branch-a", "branch-b", []string{"pkg/b.go"}, "mergeable"),
	}

	graph := Build("acme/repo", prs)

	if _, err := graph.TopologicalOrder(); err == nil {
		t.Fatal("TopologicalOrder() error = nil, want cycle detection error")
	}
}

func TestBuildIncludesTelemetry(t *testing.T) {
	prs := []types.PR{
		fixturePR(500, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixturePR(501, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
		fixturePR(502, "main", "feature-c", []string{"pkg/a.go"}, "conflicting"),
	}

	graph := Build("acme/repo", prs)

	if graph.Telemetry.GraphDeltaEdges != len(graph.Edges) {
		t.Fatalf("graph_delta_edges = %d, want %d", graph.Telemetry.GraphDeltaEdges, len(graph.Edges))
	}
	if graph.Telemetry.PairwiseShards <= 0 {
		t.Fatalf("pairwise_shards = %d, want > 0", graph.Telemetry.PairwiseShards)
	}
	if len(graph.Telemetry.StageLatenciesMS) == 0 {
		t.Fatal("expected stage latency telemetry")
	}
}

func TestDOTIncludesGraphDeclarationAndEdges(t *testing.T) {
	prs := []types.PR{
		fixturePR(400, "main", "feature-a", []string{"pkg/a.go"}, "mergeable"),
		fixturePR(401, "feature-a", "feature-b", []string{"pkg/b.go"}, "mergeable"),
		fixturePR(402, "main", "feature-c", []string{"pkg/a.go"}, "conflicting"),
	}

	graph := Build("acme/repo", prs)
	dot := graph.DOT()

	for _, want := range []string{
		"digraph pratc",
		"\"PR 400\"",
		"\"PR 400\" -> \"PR 401\"",
		"label=\"depends_on\"",
		"label=\"conflicts_with\"",
	} {
		if !strings.Contains(dot, want) {
			t.Fatalf("DOT() missing %q in output:\n%s", want, dot)
		}
	}
}

func fixturePR(number int, baseBranch, headBranch string, files []string, mergeable string) types.PR {
	return types.PR{
		ID:                "acme/repo",
		Repo:              "acme/repo",
		Number:            number,
		Title:             "PR",
		BaseBranch:        baseBranch,
		HeadBranch:        headBranch,
		FilesChanged:      files,
		ChangedFilesCount: len(files),
		CIStatus:          "success",
		Mergeable:         mergeable,
	}
}

func hasEdge(edges []types.GraphEdge, fromPR, toPR int, edgeType string) bool {
	for _, edge := range edges {
		if edge.FromPR == fromPR && edge.ToPR == toPR && edge.EdgeType == edgeType {
			return true
		}
	}

	return false
}

func appearsBefore(nodes []types.GraphNode, leftPR, rightPR int) bool {
	leftIndex := -1
	rightIndex := -1

	for idx, node := range nodes {
		if node.PRNumber == leftPR {
			leftIndex = idx
		}
		if node.PRNumber == rightPR {
			rightIndex = idx
		}
	}

	return leftIndex >= 0 && rightIndex >= 0 && leftIndex < rightIndex
}
