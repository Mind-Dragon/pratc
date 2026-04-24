package tui

import (
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPRDetailBoard_AllFieldsRendered(t *testing.T) {
	t.Parallel()

	board := NewPRDetailBoard()
	workItem := &types.ActionWorkItem{
		ID:            "wi-123",
		PRNumber:      42,
		Lane:          types.ActionLaneFastMerge,
		State:         types.ActionWorkItemStateProposed,
		PriorityScore: 0.85,
		Confidence:    0.95,
		RiskFlags:     []string{"security", "complex"},
		ReasonTrail:   []string{"tested", "reviewed"},
		EvidenceRefs:  []string{"evidence1", "evidence2"},
		AllowedActions: []types.ActionKind{
			types.ActionKindMerge,
			types.ActionKindComment,
		},
		BlockedReasons: []string{"missing_approval"},
		RequiredPreflightChecks: []types.ActionPreflight{
			{Check: "ci_pass", Status: "pending"},
		},
	}
	board.SetWorkItem(workItem)

	view := board.View(80)
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if !strings.Contains(view, "PR #42") {
		t.Errorf("expected PR #42 in view, got: %s", view)
	}
	if !strings.Contains(view, "Lane: fast_merge") {
		t.Errorf("expected Lane fast_merge in view")
	}
	if !strings.Contains(view, "State: proposed") {
		t.Errorf("expected State proposed in view")
	}
	if !strings.Contains(view, "Confidence: 0.95") {
		t.Errorf("expected Confidence 0.95 in view")
	}
	if !strings.Contains(view, "RiskFlags: security, complex") {
		t.Errorf("expected RiskFlags in view")
	}
	if !strings.Contains(view, "Reasons: tested, reviewed") {
		t.Errorf("expected Reasons in view")
	}
	if !strings.Contains(view, "Evidence: evidence1, evidence2") {
		t.Errorf("expected EvidenceRefs in view")
	}
	if !strings.Contains(view, "AllowedActions: merge, comment") {
		t.Errorf("expected AllowedActions in view")
	}
	if !strings.Contains(view, "BlockedReasons: missing_approval") {
		t.Errorf("expected BlockedReasons in view")
	}
	if !strings.Contains(view, "PreflightChecks: ci_pass:pending") {
		t.Errorf("expected PreflightChecks in view")
	}
	if strings.Contains(view, `\n`) {
		t.Fatalf("view contains literal escaped newline: %q", view)
	}
}
