package report

import (
	"path/filepath"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestLoadPlanSection_ExpandedPlanFields(t *testing.T) {
	tmp := t.TempDir()

	plan := types.PlanResponse{
		Repo:              "owner/repo",
		GeneratedAt:       "2026-04-21T12:00:00Z",
		Target:            50,
		CandidatePoolSize: 42,
		Strategy:          "formula+graph",
		Selected:          []types.MergePlanCandidate{{PRNumber: 11, Title: "Useful feature", Score: 0.93, Rationale: "good"}},
		Ordering:          []types.MergePlanCandidate{{PRNumber: 11, Title: "Useful feature", Score: 0.93, Rationale: "good"}},
		Rejections:        []types.PlanRejection{{PRNumber: 12, Reason: "duplicate"}},
		Telemetry:         &types.OperationTelemetry{PoolSizeBefore: 200, PoolSizeAfter: 42},
		CollapsedCorpus:   &types.CollapsedCorpus{CollapsedGroupCount: 7, TotalSuperseded: 13},
	}
	writeJSON(t, filepath.Join(tmp, "step-5-plan.json"), plan)

	section, err := LoadPlanSection(tmp, "owner/repo")
	if err != nil {
		t.Fatalf("LoadPlanSection: %v", err)
	}
	if section.PoolSizeBefore != 200 || section.PoolSizeAfter != 42 {
		t.Fatalf("unexpected pool sizes: before=%d after=%d", section.PoolSizeBefore, section.PoolSizeAfter)
	}
	if section.CollapsedGroups != 7 || section.TotalSuperseded != 13 {
		t.Fatalf("unexpected collapse summary: groups=%d superseded=%d", section.CollapsedGroups, section.TotalSuperseded)
	}
	lines := section.expansionSummaryLines()
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 expansion summary lines, got %#v", lines)
	}
}
