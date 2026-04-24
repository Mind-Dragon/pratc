package data

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPRDetailView_AllFieldsRenderable(t *testing.T) {
	t.Parallel()

	// Create a sample PRDetailView with all required fields
	prView := PRDetailView{
		Title:          "Fix bug in authentication",
		Author:         "alice",
		Age:            time.Hour * 24,
		Status:         "open",
		Lane:           types.ActionLaneFastMerge,
		Bucket:         "now",
		Confidence:     0.95,
		Reasons:        []string{"tested", "reviewed"},
		DecisionLayers: []types.DecisionLayer{{Layer: 1, Name: "Gate1", Status: "clear"}},
		EvidenceRefs:   []string{"evidence1"},
		DuplicateRefs:  []int{123},
		SynthesisRefs:  []string{"synthesis1"},
		RiskFlags:      []string{"security"},
		AllowedActions: []types.ActionKind{types.ActionKindMerge},
		WorkItemID:     "work-123",
		State:          types.ActionWorkItemStateProposed,
	}

	// Ensure all fields are accessible (compilation check)
	_ = prView.Title
	_ = prView.Author
	_ = prView.Age
	_ = prView.Status
	_ = prView.Lane
	_ = prView.Bucket
	_ = prView.Confidence
	_ = prView.Reasons
	_ = prView.DecisionLayers
	_ = prView.EvidenceRefs
	_ = prView.DuplicateRefs
	_ = prView.SynthesisRefs
	_ = prView.RiskFlags
	_ = prView.AllowedActions
	_ = prView.WorkItemID
	_ = prView.State

	// If we reach here, the struct exists and fields are accessible.
	// The test passes as a compilation check.
}
