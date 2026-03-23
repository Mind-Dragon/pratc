package graph

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

const (
	EdgeTypeDependency = "depends_on"
	EdgeTypeConflict   = "conflicts_with"
)

var errDependencyCycle = errors.New("dependency cycle detected")

type Graph struct {
	Repo      string
	Nodes     []types.GraphNode
	Edges     []types.GraphEdge
	Telemetry types.OperationTelemetry
}

func Build(repo string, prs []types.PR) Graph {
	return BuildWithProgress(repo, prs, nil)
}

func BuildWithProgress(repo string, prs []types.PR, progress func(processed int, total int)) Graph {
	buildStart := time.Now()
	sortedPRs := slices.Clone(prs)
	sort.Slice(sortedPRs, func(i, j int) bool {
		return sortedPRs[i].Number < sortedPRs[j].Number
	})

	nodes := make([]types.GraphNode, 0, len(sortedPRs))
	edges := make([]types.GraphEdge, 0)
	seenEdges := make(map[string]struct{})
	pairsEvaluated := 0

	for _, pr := range sortedPRs {
		nodes = append(nodes, types.GraphNode{
			PRNumber:  pr.Number,
			Title:     pr.Title,
			ClusterID: pr.ClusterID,
			CIStatus:  pr.CIStatus,
		})
	}

	for i, left := range sortedPRs {
		if progress != nil {
			progress(i+1, len(sortedPRs))
		}
		for j := i + 1; j < len(sortedPRs); j++ {
			right := sortedPRs[j]
			pairsEvaluated++

			if left.HeadBranch != "" && right.BaseBranch == left.HeadBranch {
				appendEdge(&edges, seenEdges, types.GraphEdge{
					FromPR:   left.Number,
					ToPR:     right.Number,
					EdgeType: EdgeTypeDependency,
					Reason:   fmt.Sprintf("base branch %q depends on head branch %q", right.BaseBranch, left.HeadBranch),
				})
			}

			if right.HeadBranch != "" && left.BaseBranch == right.HeadBranch {
				appendEdge(&edges, seenEdges, types.GraphEdge{
					FromPR:   right.Number,
					ToPR:     left.Number,
					EdgeType: EdgeTypeDependency,
					Reason:   fmt.Sprintf("base branch %q depends on head branch %q", left.BaseBranch, right.HeadBranch),
				})
			}

			if conflictFiles := conflictFiles(left, right); len(conflictFiles) > 0 {
				appendEdge(&edges, seenEdges, types.GraphEdge{
					FromPR:   left.Number,
					ToPR:     right.Number,
					EdgeType: EdgeTypeConflict,
					Reason:   fmt.Sprintf("shared files: %s", strings.Join(conflictFiles, ", ")),
				})
			}
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromPR != edges[j].FromPR {
			return edges[i].FromPR < edges[j].FromPR
		}
		if edges[i].ToPR != edges[j].ToPR {
			return edges[i].ToPR < edges[j].ToPR
		}
		return edges[i].EdgeType < edges[j].EdgeType
	})

	return Graph{
		Repo:  repo,
		Nodes: nodes,
		Edges: edges,
		Telemetry: types.OperationTelemetry{
			PoolStrategy:    "graph_pairwise_scan",
			PoolSizeBefore:  len(sortedPRs),
			PoolSizeAfter:   len(sortedPRs),
			GraphDeltaEdges: len(edges),
			DecayPolicy:     "none",
			PairwiseShards:  estimatePairwiseShards(len(sortedPRs)),
			StageLatenciesMS: map[string]int{
				"build_total_ms": int(time.Since(buildStart).Milliseconds()),
			},
			StageDropCounts: map[string]int{
				"pairs_without_edges": pairsEvaluated - len(edges),
			},
		},
	}
}

func (g Graph) TopologicalOrder() ([]types.GraphNode, error) {
	nodeByPR := make(map[int]types.GraphNode, len(g.Nodes))
	inDegree := make(map[int]int, len(g.Nodes))
	dependents := make(map[int][]int, len(g.Nodes))

	for _, node := range g.Nodes {
		nodeByPR[node.PRNumber] = node
		inDegree[node.PRNumber] = 0
	}

	for _, edge := range g.Edges {
		if edge.EdgeType != EdgeTypeDependency {
			continue
		}
		if _, ok := nodeByPR[edge.FromPR]; !ok {
			continue
		}
		if _, ok := nodeByPR[edge.ToPR]; !ok {
			continue
		}

		dependents[edge.FromPR] = append(dependents[edge.FromPR], edge.ToPR)
		inDegree[edge.ToPR]++
	}

	ready := make([]int, 0)
	for prNumber, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, prNumber)
		}
	}
	sort.Ints(ready)

	order := make([]types.GraphNode, 0, len(g.Nodes))
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		order = append(order, nodeByPR[current])

		children := dependents[current]
		sort.Ints(children)
		for _, next := range children {
			inDegree[next]--
			if inDegree[next] == 0 {
				ready = append(ready, next)
				sort.Ints(ready)
			}
		}
	}

	if len(order) != len(g.Nodes) {
		return nil, errDependencyCycle
	}

	return order, nil
}

func (g Graph) DOT() string {
	var builder strings.Builder
	builder.WriteString("digraph pratc {\n")
	builder.WriteString("  rankdir=LR;\n")

	for _, node := range g.Nodes {
		title := node.Title
		if title == "" {
			title = "Untitled"
		}
		builder.WriteString(fmt.Sprintf("  \"PR %d\" [label=\"PR %d: %s\"];\n", node.PRNumber, node.PRNumber, escapeDOT(title)))
	}

	for _, edge := range g.Edges {
		style := "solid"
		color := "black"
		if edge.EdgeType == EdgeTypeConflict {
			style = "dashed"
			color = "red"
		}

		builder.WriteString(
			fmt.Sprintf(
				"  \"PR %d\" -> \"PR %d\" [label=%q color=%q style=%q];\n",
				edge.FromPR,
				edge.ToPR,
				edge.EdgeType,
				color,
				style,
			),
		)
	}

	builder.WriteString("}\n")

	return builder.String()
}

func appendEdge(edges *[]types.GraphEdge, seen map[string]struct{}, edge types.GraphEdge) {
	key := fmt.Sprintf("%d:%d:%s", edge.FromPR, edge.ToPR, edge.EdgeType)
	if _, ok := seen[key]; ok {
		return
	}

	seen[key] = struct{}{}
	*edges = append(*edges, edge)
}

func conflictFiles(left, right types.PR) []string {
	if left.BaseBranch == "" || right.BaseBranch == "" || left.BaseBranch != right.BaseBranch {
		return nil
	}

	shared := intersectFiles(left.FilesChanged, right.FilesChanged)
	if len(shared) == 0 && left.Mergeable != "conflicting" && right.Mergeable != "conflicting" {
		return nil
	}
	if len(shared) == 0 {
		return []string{"mergeability_signal"}
	}

	return shared
}

func intersectFiles(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}

	rightSet := make(map[string]struct{}, len(right))
	for _, path := range right {
		rightSet[path] = struct{}{}
	}

	shared := make([]string, 0)
	for _, path := range left {
		if _, ok := rightSet[path]; ok {
			shared = append(shared, path)
		}
	}

	sort.Strings(shared)
	return slices.Compact(shared)
}

func escapeDOT(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func estimatePairwiseShards(poolSize int) int {
	if poolSize <= 0 {
		return 1
	}
	const shardSize = 256
	shards := (poolSize + shardSize - 1) / shardSize
	if shards < 1 {
		return 1
	}
	return shards
}
