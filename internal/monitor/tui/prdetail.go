package tui

import (
	"fmt"
	"strings"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// PRDetailBoard renders a detailed view of a single PR (work item) for the monitor.
type PRDetailBoard struct {
	workItem *types.ActionWorkItem
}

func NewPRDetailBoard() *PRDetailBoard {
	return &PRDetailBoard{}
}

func (p *PRDetailBoard) SetWorkItem(item *types.ActionWorkItem) {
	p.workItem = item
}

func (p *PRDetailBoard) View(width int) string {
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
	sb.WriteString("|" + padRight("PR DETAIL", inner) + "|")
	sb.WriteString("\n")
	sb.WriteString(border)
	sb.WriteString("\n")
	if p == nil || p.workItem == nil {
		sb.WriteString("|" + padRight("no work item selected", inner) + "|\n")
		sb.WriteString(border)
		return sb.String()
	}
	wi := p.workItem
	// Basic info
	line1 := fmt.Sprintf("PR #%d  Lane: %s  State: %s", wi.PRNumber, wi.Lane, wi.State)
	sb.WriteString("|" + padRight(line1, inner) + "|\n")
	// Confidence and priority
	line2 := fmt.Sprintf("Confidence: %.2f  PriorityScore: %.2f", wi.Confidence, wi.PriorityScore)
	sb.WriteString("|" + padRight(line2, inner) + "|\n")
	// Risk flags
	if len(wi.RiskFlags) > 0 {
		line3 := "RiskFlags: " + strings.Join(wi.RiskFlags, ", ")
		sb.WriteString("|" + padRight(line3, inner) + "|\n")
	}
	// Reason trail
	if len(wi.ReasonTrail) > 0 {
		line4 := "Reasons: " + strings.Join(wi.ReasonTrail, ", ")
		sb.WriteString("|" + padRight(line4, inner) + "|\n")
	}
	// Evidence refs
	if len(wi.EvidenceRefs) > 0 {
		line5 := "Evidence: " + strings.Join(wi.EvidenceRefs, ", ")
		sb.WriteString("|" + padRight(line5, inner) + "|\n")
	}
	// Allowed actions
	if len(wi.AllowedActions) > 0 {
		var actions []string
		for _, a := range wi.AllowedActions {
			actions = append(actions, string(a))
		}
		line6 := "AllowedActions: " + strings.Join(actions, ", ")
		sb.WriteString("|" + padRight(line6, inner) + "|\n")
	}
	// Blocked reasons
	if len(wi.BlockedReasons) > 0 {
		line7 := "BlockedReasons: " + strings.Join(wi.BlockedReasons, ", ")
		sb.WriteString("|" + padRight(line7, inner) + "|\n")
	}
	// Required preflight checks
	if len(wi.RequiredPreflightChecks) > 0 {
		var checks []string
		for _, c := range wi.RequiredPreflightChecks {
			checks = append(checks, fmt.Sprintf("%s:%s", c.Check, c.Status))
		}
		line8 := "PreflightChecks: " + strings.Join(checks, ", ")
		sb.WriteString("|" + padRight(line8, inner) + "|\n")
	}
	sb.WriteString(border)
	return sb.String()
}
