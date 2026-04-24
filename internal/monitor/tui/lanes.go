package tui

import (
	"fmt"
	"strings"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// ActionLaneBoard renders a read-only action-plan lane summary for the monitor.
type ActionLaneBoard struct {
	plan *types.ActionPlan
}

func NewActionLaneBoard() *ActionLaneBoard {
	return &ActionLaneBoard{}
}

func (a *ActionLaneBoard) SetPlan(plan *types.ActionPlan) {
	a.plan = plan
}

func (a *ActionLaneBoard) GetWorkItem(index int) *types.ActionWorkItem {
	if a == nil || a.plan == nil || index < 0 || index >= len(a.plan.WorkItems) {
		return nil
	}
	return &a.plan.WorkItems[index]
}

func (a *ActionLaneBoard) WorkItemCount() int {
	if a == nil || a.plan == nil {
		return 0
	}
	return len(a.plan.WorkItems)
}

func (a *ActionLaneBoard) View(width int) string {
	if width <= 0 {
		width = 94
	}
	inner := width - 2
	if inner < 20 {
		inner = 20
	}
	border := "+" + strings.Repeat("-", inner) + "+"
	var sb strings.Builder
	sb.WriteString(border)
	sb.WriteString("\n")
	sb.WriteString("|" + padRight("ACTIONS (read-only)", inner) + "|\n")
	sb.WriteString(border)
	sb.WriteString("\n")
	if a == nil || a.plan == nil {
		sb.WriteString("|" + padRight("no action plan loaded", inner) + "|\n")
		sb.WriteString(border)
		return sb.String()
	}
	plan := a.plan
	summary := fmt.Sprintf("repo=%s policy=%s total_prs=%d", plan.Repo, plan.PolicyProfile, plan.CorpusSnapshot.TotalPRs)
	sb.WriteString("|" + padRight(summary, inner) + "|\n")
	lanes := plan.Lanes
	if len(lanes) == 0 {
		sb.WriteString("|" + padRight("lanes: none", inner) + "|\n")
	} else {
		parts := make([]string, 0, len(lanes))
		for _, lane := range lanes {
			parts = append(parts, fmt.Sprintf("%s:%d", lane.Lane, lane.Count))
		}
		sb.WriteString("|" + padRight(strings.Join(parts, "  "), inner) + "|\n")
	}
	sb.WriteString(border)
	return sb.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
