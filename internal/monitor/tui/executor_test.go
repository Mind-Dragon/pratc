package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

func TestExecutorConsolePanel_Renders(t *testing.T) {
	panel := NewExecutorConsolePanel()
	state := data.ExecutorState{
		PendingIntents:   12,
		ClaimedItems:     3,
		InProgressItems:  5,
		CompletedItems:   127,
		FailedItems:      2,
		ProofBundleCount: 3,
	}
	bundles := []data.ProofBundleRef{
		{
			ID:         "pb-001",
			WorkItemID: "wi-001",
			PRNumber:   42,
			Summary:    "Applied fix",
			CreatedAt:  time.Now(),
		},
	}
	panel.SetState(state)
	panel.SetProofBundles(bundles)
	view := panel.View(94)
	if view == "" {
		t.Fatal("empty view")
	}
	if !strings.Contains(view, "EXECUTOR CONSOLE") {
		t.Error("missing header")
	}
	if !strings.Contains(view, "Pending:12") {
		t.Error("missing pending count")
	}
	if !strings.Contains(view, "Proof Bundles:") {
		t.Error("missing proof bundle count")
	}
}
