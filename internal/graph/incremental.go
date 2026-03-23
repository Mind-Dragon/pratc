package graph

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type PRFingerprint struct {
	PRNumber   int
	BaseBranch string
	HeadBranch string
	FilesHash  string
	Mergeable  string
	ClusterID  string
	UpdatedAt  string
}

type EdgeCacheKey struct {
	FromPR   int
	ToPR     int
	EdgeType string
}

type EdgeCacheEntry struct {
	Key          EdgeCacheKey
	Edge         types.GraphEdge
	ValidatedAt  time.Time
	Fingerprints [2]PRFingerprint
}

type GraphDelta struct {
	EdgesAdded   int `json:"edges_added"`
	EdgesRemoved int `json:"edges_removed"`
	NodesUpdated int `json:"nodes_updated"`

	AddedEdges   []types.GraphEdge `json:"added_edges,omitempty"`
	RemovedEdges []types.GraphEdge `json:"removed_edges,omitempty"`
	UpdatedNodes []int             `json:"updated_nodes,omitempty"`
}

type IncrementalGraph struct {
	graph            Graph
	edgeCache        map[EdgeCacheKey]EdgeCacheEntry
	fingerprints     map[int]PRFingerprint
	needsFullRebuild bool
	lastFullRebuild  time.Time
	repo             string
	delta            GraphDelta
	Telemetry        types.OperationTelemetry
}

func NewIncrementalGraph(repo string) *IncrementalGraph {
	return &IncrementalGraph{
		graph: Graph{
			Repo:  repo,
			Nodes: make([]types.GraphNode, 0),
			Edges: make([]types.GraphEdge, 0),
		},
		edgeCache:    make(map[EdgeCacheKey]EdgeCacheEntry),
		fingerprints: make(map[int]PRFingerprint),
		repo:         repo,
		Telemetry: types.OperationTelemetry{
			StageLatenciesMS: make(map[string]int),
			StageDropCounts:  make(map[string]int),
		},
	}
}

func ComputeFingerprint(pr types.PR) PRFingerprint {
	sortedFiles := slices.Clone(pr.FilesChanged)
	sort.Strings(sortedFiles)
	filesStr := strings.Join(sortedFiles, "|")
	filesHash := fmt.Sprintf("%x", sha256.Sum256([]byte(filesStr)))

	return PRFingerprint{
		PRNumber:   pr.Number,
		BaseBranch: pr.BaseBranch,
		HeadBranch: pr.HeadBranch,
		FilesHash:  filesHash,
		Mergeable:  pr.Mergeable,
		ClusterID:  pr.ClusterID,
		UpdatedAt:  pr.UpdatedAt,
	}
}

func (ig *IncrementalGraph) FingerprintChanged(pr types.PR) bool {
	newFP := ComputeFingerprint(pr)
	oldFP, exists := ig.fingerprints[pr.Number]

	if !exists {
		return true
	}

	return newFP.BaseBranch != oldFP.BaseBranch ||
		newFP.HeadBranch != oldFP.HeadBranch ||
		newFP.FilesHash != oldFP.FilesHash ||
		newFP.Mergeable != oldFP.Mergeable ||
		newFP.ClusterID != oldFP.ClusterID
}

func (ig *IncrementalGraph) Update(prs []types.PR) (Graph, GraphDelta) {
	updateStart := time.Now()

	ig.delta = GraphDelta{
		AddedEdges:   make([]types.GraphEdge, 0),
		RemovedEdges: make([]types.GraphEdge, 0),
		UpdatedNodes: make([]int, 0),
	}

	if ig.needsFullRebuild {
		return ig.fullRebuild(prs, updateStart), ig.delta
	}

	changedPRs := make(map[int]types.PR)
	unchangedPRs := make(map[int]types.PR)
	newPRs := make(map[int]types.PR)

	for _, pr := range prs {
		if _, exists := ig.fingerprints[pr.Number]; !exists {
			newPRs[pr.Number] = pr
		} else if ig.FingerprintChanged(pr) {
			changedPRs[pr.Number] = pr
		} else {
			unchangedPRs[pr.Number] = pr
		}
	}

	totalPRs := len(prs)
	changedCount := len(changedPRs) + len(newPRs)

	if totalPRs == 0 || float64(changedCount)/float64(totalPRs) > 0.5 {
		return ig.fullRebuild(prs, updateStart), ig.delta
	}

	return ig.incrementalUpdate(prs, changedPRs, newPRs, unchangedPRs, updateStart), ig.delta
}

func (ig *IncrementalGraph) fullRebuild(prs []types.PR, startTime time.Time) Graph {
	oldEdges := make(map[EdgeCacheKey]EdgeCacheEntry)
	for k, v := range ig.edgeCache {
		oldEdges[k] = v
	}
	oldNodeCount := len(ig.graph.Nodes)

	ig.graph = Build(ig.repo, prs)
	// Copy initialized Telemetry maps into the returned graph copy
	ig.graph.Telemetry = ig.Telemetry

	ig.edgeCache = make(map[EdgeCacheKey]EdgeCacheEntry, len(ig.graph.Edges))
	ig.fingerprints = make(map[int]PRFingerprint, len(prs))

	for _, pr := range prs {
		ig.fingerprints[pr.Number] = ComputeFingerprint(pr)
	}

	now := time.Now()
	for _, edge := range ig.graph.Edges {
		key := EdgeCacheKey{
			FromPR:   edge.FromPR,
			ToPR:     edge.ToPR,
			EdgeType: edge.EdgeType,
		}

		var leftPR, rightPR types.PR
		for _, pr := range prs {
			if pr.Number == edge.FromPR {
				leftPR = pr
			}
			if pr.Number == edge.ToPR {
				rightPR = pr
			}
		}

		ig.edgeCache[key] = EdgeCacheEntry{
			Key:         key,
			Edge:        edge,
			ValidatedAt: now,
			Fingerprints: [2]PRFingerprint{
				ComputeFingerprint(leftPR),
				ComputeFingerprint(rightPR),
			},
		}
	}

	ig.delta.EdgesAdded = len(ig.graph.Edges)
	ig.delta.AddedEdges = slices.Clone(ig.graph.Edges)

	if len(ig.graph.Nodes) != oldNodeCount {
		ig.delta.NodesUpdated = len(ig.graph.Nodes) - oldNodeCount
		if ig.delta.NodesUpdated < 0 {
			ig.delta.NodesUpdated = -ig.delta.NodesUpdated
		}
	}

	ig.Telemetry.StageLatenciesMS["full_rebuild_ms"] = int(time.Since(startTime).Milliseconds())
	ig.Telemetry.StageDropCounts["full_rebuilds"]++

	ig.lastFullRebuild = time.Now()
	ig.needsFullRebuild = false

	return ig.graph
}

func (ig *IncrementalGraph) incrementalUpdate(
	allPRs []types.PR,
	changedPRs map[int]types.PR,
	newPRs map[int]types.PR,
	unchangedPRs map[int]types.PR,
	startTime time.Time,
) Graph {
	prsToReevaluate := make(map[int]types.PR)

	for num, pr := range changedPRs {
		prsToReevaluate[num] = pr
	}
	for num, pr := range newPRs {
		prsToReevaluate[num] = pr
	}

	for num := range prsToReevaluate {
		ig.fingerprints[num] = ComputeFingerprint(prsToReevaluate[num])
		ig.delta.UpdatedNodes = append(ig.delta.UpdatedNodes, num)
	}

	sortedPRs := slices.Clone(allPRs)
	sort.Slice(sortedPRs, func(i, j int) bool {
		return sortedPRs[i].Number < sortedPRs[j].Number
	})

	nodes := make([]types.GraphNode, 0, len(sortedPRs))
	for _, pr := range sortedPRs {
		nodes = append(nodes, types.GraphNode{
			PRNumber:  pr.Number,
			Title:     pr.Title,
			ClusterID: pr.ClusterID,
			CIStatus:  pr.CIStatus,
		})
	}
	ig.graph.Nodes = nodes

	newEdges := make([]types.GraphEdge, 0)
	seenEdges := make(map[string]struct{})
	now := time.Now()

	for i, left := range sortedPRs {
		for j := i + 1; j < len(sortedPRs); j++ {
			right := sortedPRs[j]

			_, leftIsChanged := changedPRs[left.Number]
			_, leftIsNew := newPRs[left.Number]
			_, rightIsChanged := changedPRs[right.Number]
			_, rightIsNew := newPRs[right.Number]

			edgeAffected := leftIsChanged || leftIsNew || rightIsChanged || rightIsNew

			if !edgeAffected {
				key := EdgeCacheKey{
					FromPR:   left.Number,
					ToPR:     right.Number,
					EdgeType: EdgeTypeDependency,
				}

				if left.HeadBranch != "" && right.BaseBranch == left.HeadBranch {
					if cached, ok := ig.edgeCache[key]; ok {
						appendEdge(&newEdges, seenEdges, cached.Edge)
					} else {
						edge := types.GraphEdge{
							FromPR:   left.Number,
							ToPR:     right.Number,
							EdgeType: EdgeTypeDependency,
							Reason:   fmt.Sprintf("base branch %q depends on head branch %q", right.BaseBranch, left.HeadBranch),
						}
						appendEdge(&newEdges, seenEdges, edge)
						ig.edgeCache[key] = EdgeCacheEntry{
							Key:         key,
							Edge:        edge,
							ValidatedAt: now,
							Fingerprints: [2]PRFingerprint{
								ig.fingerprints[left.Number],
								ig.fingerprints[right.Number],
							},
						}
						ig.delta.EdgesAdded++
						ig.delta.AddedEdges = append(ig.delta.AddedEdges, edge)
					}
				}

				if right.HeadBranch != "" && left.BaseBranch == right.HeadBranch {
					key.ToPR = left.Number
					key.FromPR = right.Number
					if cached, ok := ig.edgeCache[key]; ok {
						appendEdge(&newEdges, seenEdges, cached.Edge)
					} else {
						edge := types.GraphEdge{
							FromPR:   right.Number,
							ToPR:     left.Number,
							EdgeType: EdgeTypeDependency,
							Reason:   fmt.Sprintf("base branch %q depends on head branch %q", left.BaseBranch, right.HeadBranch),
						}
						appendEdge(&newEdges, seenEdges, edge)
						ig.edgeCache[key] = EdgeCacheEntry{
							Key:         key,
							Edge:        edge,
							ValidatedAt: now,
							Fingerprints: [2]PRFingerprint{
								ig.fingerprints[right.Number],
								ig.fingerprints[left.Number],
							},
						}
						ig.delta.EdgesAdded++
						ig.delta.AddedEdges = append(ig.delta.AddedEdges, edge)
					}
				}

				key.EdgeType = EdgeTypeConflict
				key.FromPR = left.Number
				key.ToPR = right.Number
				if conflictFiles := conflictFiles(left, right); len(conflictFiles) > 0 {
					if cached, ok := ig.edgeCache[key]; ok {
						appendEdge(&newEdges, seenEdges, cached.Edge)
					} else {
						edge := types.GraphEdge{
							FromPR:   left.Number,
							ToPR:     right.Number,
							EdgeType: EdgeTypeConflict,
							Reason:   fmt.Sprintf("shared files: %s", strings.Join(conflictFiles, ", ")),
						}
						appendEdge(&newEdges, seenEdges, edge)
						ig.edgeCache[key] = EdgeCacheEntry{
							Key:         key,
							Edge:        edge,
							ValidatedAt: now,
							Fingerprints: [2]PRFingerprint{
								ig.fingerprints[left.Number],
								ig.fingerprints[right.Number],
							},
						}
						ig.delta.EdgesAdded++
						ig.delta.AddedEdges = append(ig.delta.AddedEdges, edge)
					}
				}
				continue
			}

			for _, et := range []string{EdgeTypeDependency, EdgeTypeConflict} {
				oldKey := EdgeCacheKey{FromPR: left.Number, ToPR: right.Number, EdgeType: et}
				if oldEntry, exists := ig.edgeCache[oldKey]; exists {
					ig.delta.RemovedEdges = append(ig.delta.RemovedEdges, oldEntry.Edge)
					ig.delta.EdgesRemoved++
				}
				reverseKey := EdgeCacheKey{FromPR: right.Number, ToPR: left.Number, EdgeType: et}
				if oldEntry, exists := ig.edgeCache[reverseKey]; exists {
					ig.delta.RemovedEdges = append(ig.delta.RemovedEdges, oldEntry.Edge)
					ig.delta.EdgesRemoved++
				}
			}

			if left.HeadBranch != "" && right.BaseBranch == left.HeadBranch {
				edge := types.GraphEdge{
					FromPR:   left.Number,
					ToPR:     right.Number,
					EdgeType: EdgeTypeDependency,
					Reason:   fmt.Sprintf("base branch %q depends on head branch %q", right.BaseBranch, left.HeadBranch),
				}
				appendEdge(&newEdges, seenEdges, edge)
				key := EdgeCacheKey{
					FromPR:   left.Number,
					ToPR:     right.Number,
					EdgeType: EdgeTypeDependency,
				}
				ig.edgeCache[key] = EdgeCacheEntry{
					Key:         key,
					Edge:        edge,
					ValidatedAt: now,
					Fingerprints: [2]PRFingerprint{
						ig.fingerprints[left.Number],
						ig.fingerprints[right.Number],
					},
				}
				ig.delta.EdgesAdded++
				ig.delta.AddedEdges = append(ig.delta.AddedEdges, edge)
			}

			if right.HeadBranch != "" && left.BaseBranch == right.HeadBranch {
				edge := types.GraphEdge{
					FromPR:   right.Number,
					ToPR:     left.Number,
					EdgeType: EdgeTypeDependency,
					Reason:   fmt.Sprintf("base branch %q depends on head branch %q", left.BaseBranch, right.HeadBranch),
				}
				appendEdge(&newEdges, seenEdges, edge)
				key := EdgeCacheKey{
					FromPR:   right.Number,
					ToPR:     left.Number,
					EdgeType: EdgeTypeDependency,
				}
				ig.edgeCache[key] = EdgeCacheEntry{
					Key:         key,
					Edge:        edge,
					ValidatedAt: now,
					Fingerprints: [2]PRFingerprint{
						ig.fingerprints[right.Number],
						ig.fingerprints[left.Number],
					},
				}
				ig.delta.EdgesAdded++
				ig.delta.AddedEdges = append(ig.delta.AddedEdges, edge)
			}

			if conflictFiles := conflictFiles(left, right); len(conflictFiles) > 0 {
				edge := types.GraphEdge{
					FromPR:   left.Number,
					ToPR:     right.Number,
					EdgeType: EdgeTypeConflict,
					Reason:   fmt.Sprintf("shared files: %s", strings.Join(conflictFiles, ", ")),
				}
				appendEdge(&newEdges, seenEdges, edge)
				key := EdgeCacheKey{
					FromPR:   left.Number,
					ToPR:     right.Number,
					EdgeType: EdgeTypeConflict,
				}
				ig.edgeCache[key] = EdgeCacheEntry{
					Key:         key,
					Edge:        edge,
					ValidatedAt: now,
					Fingerprints: [2]PRFingerprint{
						ig.fingerprints[left.Number],
						ig.fingerprints[right.Number],
					},
				}
				ig.delta.EdgesAdded++
				ig.delta.AddedEdges = append(ig.delta.AddedEdges, edge)
			}
		}
	}

	sort.Slice(newEdges, func(i, j int) bool {
		if newEdges[i].FromPR != newEdges[j].FromPR {
			return newEdges[i].FromPR < newEdges[j].FromPR
		}
		if newEdges[i].ToPR != newEdges[j].ToPR {
			return newEdges[i].ToPR < newEdges[j].ToPR
		}
		return newEdges[i].EdgeType < newEdges[j].EdgeType
	})

	ig.graph.Edges = newEdges

	ig.Telemetry.StageLatenciesMS["incremental_update_ms"] = int(time.Since(startTime).Milliseconds())
	ig.Telemetry.StageDropCounts["incremental_updates"]++
	ig.Telemetry.StageDropCounts["changed_prs"] = len(changedPRs)
	ig.Telemetry.StageDropCounts["new_prs"] = len(newPRs)

	return ig.graph
}

func (ig *IncrementalGraph) Invalidate() {
	ig.needsFullRebuild = true
}

func (ig *IncrementalGraph) GetDelta() GraphDelta {
	return ig.delta
}

func (ig *IncrementalGraph) GetCacheStats() (cachedEdges int, fingerprintCount int, needsRebuild bool) {
	return len(ig.edgeCache), len(ig.fingerprints), ig.needsFullRebuild
}

func (ig *IncrementalGraph) GetGraph() Graph {
	return ig.graph
}

func (ig *IncrementalGraph) Clear() {
	ig.graph = Graph{
		Repo:  ig.repo,
		Nodes: make([]types.GraphNode, 0),
		Edges: make([]types.GraphEdge, 0),
	}
	ig.edgeCache = make(map[EdgeCacheKey]EdgeCacheEntry)
	ig.fingerprints = make(map[int]PRFingerprint)
	ig.delta = GraphDelta{}
	ig.needsFullRebuild = false
}
