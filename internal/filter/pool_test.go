package filter

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestCapPool(t *testing.T) {
	prs := make([]types.PR, 100)
	for i := range prs {
		prs[i] = types.PR{Number: i + 1, Title: "PR"}
	}

	pool, rejections := CapPool(prs, 64)

	if len(pool) != 64 {
		t.Errorf("CapPool() pool size = %d, want 64", len(pool))
	}

	if len(rejections) != 36 {
		t.Errorf("CapPool() rejections = %d, want 36", len(rejections))
	}

	for _, r := range rejections {
		if r.Reason != "candidate pool cap" {
			t.Errorf("Rejection reason = %q, want \"candidate pool cap\"", r.Reason)
		}
	}
}

func TestCapPool_UnderCap(t *testing.T) {
	prs := make([]types.PR, 10)
	for i := range prs {
		prs[i] = types.PR{Number: i + 1, Title: "PR"}
	}

	pool, rejections := CapPool(prs, 64)

	if len(pool) != 10 {
		t.Errorf("CapPool() pool size = %d, want 10", len(pool))
	}

	if len(rejections) != 0 {
		t.Errorf("CapPool() rejections = %d, want 0", len(rejections))
	}
}

func TestCapPool_Empty(t *testing.T) {
	pool, rejections := CapPool([]types.PR{}, 64)

	if len(pool) != 0 {
		t.Errorf("CapPool() pool size = %d, want 0", len(pool))
	}

	if len(rejections) != 0 {
		t.Errorf("CapPool() rejections = %d, want 0", len(rejections))
	}
}

func TestAssignClusterIDs(t *testing.T) {
	prs := []types.PR{
		{Number: 1, ClusterID: ""},
		{Number: 2, ClusterID: ""},
		{Number: 3, ClusterID: "existing"},
	}

	clusterByPR := map[int]string{
		1: "assigned-cluster",
		2: "another-cluster",
	}

	AssignClusterIDs(prs, clusterByPR)

	if prs[0].ClusterID != "assigned-cluster" {
		t.Errorf("PR 1 ClusterID = %q, want \"assigned-cluster\"", prs[0].ClusterID)
	}
	if prs[1].ClusterID != "another-cluster" {
		t.Errorf("PR 2 ClusterID = %q, want \"another-cluster\"", prs[1].ClusterID)
	}
	if prs[2].ClusterID != "existing" {
		t.Errorf("PR 3 ClusterID = %q, want \"existing\"", prs[2].ClusterID)
	}
}

func TestSortPoolByPriority(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{
			Number:       3,
			CIStatus:     "failure",
			ReviewStatus: "changes_requested",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
		{
			Number:       1,
			CIStatus:     "success",
			ReviewStatus: "approved",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
		{
			Number:       2,
			CIStatus:     "pending",
			ReviewStatus: "",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
	}

	sorted := SortPoolByPriority(prs, now)

	if len(sorted) != 3 {
		t.Fatalf("SortPoolByPriority() result size = %d, want 3", len(sorted))
	}

	if sorted[0].Number != 1 {
		t.Errorf("First = %d, want 1 (highest priority)", sorted[0].Number)
	}
	if sorted[1].Number != 2 {
		t.Errorf("Second = %d, want 2", sorted[1].Number)
	}
	if sorted[2].Number != 3 {
		t.Errorf("Third = %d, want 3 (lowest priority)", sorted[2].Number)
	}
}

func TestSortPoolByPriority_Deterministic(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{
			Number:       2,
			CIStatus:     "success",
			ReviewStatus: "",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
		{
			Number:       1,
			CIStatus:     "success",
			ReviewStatus: "",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
	}

	sorted1 := SortPoolByPriority(prs, now)
	sorted2 := SortPoolByPriority(prs, now)

	if len(sorted1) != len(sorted2) {
		t.Fatal("SortPoolByPriority() results differ in length")
	}

	for i := range sorted1 {
		if sorted1[i].Number != sorted2[i].Number {
			t.Errorf("Position %d: PR %d vs PR %d (not deterministic)", i, sorted1[i].Number, sorted2[i].Number)
		}
	}
}
