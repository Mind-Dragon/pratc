package filter

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPipeline_BuildCandidatePool(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{
			Number:    1,
			Title:     "Feature A",
			IsDraft:   false,
			Mergeable: "mergeable",
			CIStatus:  "success",
			UpdatedAt: "2026-03-21T10:00:00Z",
			ClusterID: "cluster-a",
		},
		{
			Number:    2,
			Title:     "Draft PR",
			IsDraft:   true,
			Mergeable: "mergeable",
			CIStatus:  "success",
			UpdatedAt: "2026-03-21T10:00:00Z",
			ClusterID: "",
		},
		{
			Number:    3,
			Title:     "Conflicting PR",
			IsDraft:   false,
			Mergeable: "conflicting",
			CIStatus:  "success",
			UpdatedAt: "2026-03-21T10:00:00Z",
			ClusterID: "cluster-b",
		},
		{
			Number:    4,
			Title:     "CI Failing PR",
			IsDraft:   false,
			Mergeable: "mergeable",
			CIStatus:  "failure",
			UpdatedAt: "2026-03-21T10:00:00Z",
			ClusterID: "",
		},
		{
			Number:    5,
			Title:     "Good PR B",
			IsDraft:   false,
			Mergeable: "mergeable",
			CIStatus:  "success",
			UpdatedAt: "2026-03-21T10:00:00Z",
			ClusterID: "cluster-a",
		},
	}

	clusterByPR := map[int]string{
		1: "cluster-a",
		5: "cluster-a",
	}

	pipeline := NewPipeline(now)
	pool, rejections := pipeline.BuildCandidatePool(prs, clusterByPR)

	if len(pool) != 2 {
		t.Errorf("BuildCandidatePool() pool size = %d, want 2", len(pool))
	}

	if len(rejections) != 3 {
		t.Errorf("BuildCandidatePool() rejections = %d, want 3", len(rejections))
	}

	rejectionReasons := make(map[int]string)
	for _, r := range rejections {
		rejectionReasons[r.PRNumber] = r.Reason
	}

	if reason, ok := rejectionReasons[2]; !ok || reason != "draft" {
		t.Errorf("PR 2 rejection reason = %q, want \"draft\"", reason)
	}
	if reason, ok := rejectionReasons[3]; !ok || reason != "merge conflict" {
		t.Errorf("PR 3 rejection reason = %q, want \"merge conflict\"", reason)
	}
	if reason, ok := rejectionReasons[4]; !ok || reason != "ci failure" {
		t.Errorf("PR 4 rejection reason = %q, want \"ci failure\"", reason)
	}

	poolNumbers := make(map[int]bool)
	for _, pr := range pool {
		poolNumbers[pr.Number] = true
	}
	if !poolNumbers[1] {
		t.Error("PR 1 should be in pool")
	}
	if !poolNumbers[5] {
		t.Error("PR 5 should be in pool")
	}
}

func TestPipeline_BuildCandidatePool_EmptyPool(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{Number: 1, IsDraft: true, Mergeable: "mergeable", CIStatus: "success"},
		{Number: 2, IsDraft: false, Mergeable: "conflicting", CIStatus: "success"},
		{Number: 3, IsDraft: false, Mergeable: "mergeable", CIStatus: "failure"},
	}

	pipeline := NewPipeline(now)
	pool, rejections := pipeline.BuildCandidatePool(prs, map[int]string{})

	if len(pool) != 0 {
		t.Errorf("BuildCandidatePool() pool size = %d, want 0", len(pool))
	}

	if len(rejections) != 3 {
		t.Errorf("BuildCandidatePool() rejections = %d, want 3", len(rejections))
	}
}

func TestPipeline_BuildCandidatePool_DoesNotCapByDefault(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	prs := make([]types.PR, 100)
	clusterByPR := make(map[int]string, 100)
	for i := range prs {
		prs[i] = types.PR{
			Number:       i + 1,
			Title:        "PR",
			CIStatus:     "success",
			ReviewStatus: "approved",
			Mergeable:    "mergeable",
		}
		clusterByPR[i+1] = "cluster-1"
	}

	pipeline := NewPipeline(now)
	pool, rejections := pipeline.BuildCandidatePool(prs, clusterByPR)

	if len(pool) != 100 {
		t.Fatalf("BuildCandidatePool() pool size = %d, want 100", len(pool))
	}
	if len(rejections) != 0 {
		t.Fatalf("BuildCandidatePool() rejections = %d, want 0", len(rejections))
	}
}

func TestPipeline_BuildCandidatePool_ClusterAssignment(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{
			Number:    10,
			IsDraft:   false,
			Mergeable: "mergeable",
			CIStatus:  "success",
			ClusterID: "",
		},
	}

	clusterByPR := map[int]string{
		10: "cluster-assigned",
	}

	pipeline := NewPipeline(now)
	pool, _ := pipeline.BuildCandidatePool(prs, clusterByPR)

	if len(pool) != 1 {
		t.Fatalf("BuildCandidatePool() pool size = %d, want 1", len(pool))
	}

	if pool[0].ClusterID != "cluster-assigned" {
		t.Errorf("PR ClusterID = %q, want \"cluster-assigned\"", pool[0].ClusterID)
	}
}
