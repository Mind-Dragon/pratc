package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

type ExecutorConsolePanel struct {
	state        data.ExecutorState
	proofBundles []data.ProofBundleRef
	cursor       int
}

func NewExecutorConsolePanel() *ExecutorConsolePanel {
	return &ExecutorConsolePanel{
		cursor: 0,
	}
}

func (p *ExecutorConsolePanel) SetState(state data.ExecutorState) {
	p.state = state
}

func (p *ExecutorConsolePanel) SetProofBundles(bundles []data.ProofBundleRef) {
	p.proofBundles = bundles
	p.cursor = 0
}

func (p *ExecutorConsolePanel) View(width int) string {
	if width <= 0 {
		width = 94
	}
	inner := width - 2
	if inner < 20 {
		inner = 20
	}
	border := "+" + strings.Repeat("-", inner) + "+"
	var sb strings.Builder

	// Header
	sb.WriteString(border + "\n")
	sb.WriteString("|" + padRight("EXECUTOR CONSOLE", inner) + "|\n")
	sb.WriteString(border + "\n")

	// Summary
	pending := p.state.PendingIntents
	claimed := p.state.ClaimedItems
	inprog := p.state.InProgressItems
	completed := p.state.CompletedItems
	failed := p.state.FailedItems
	summary := fmt.Sprintf("Queue: Pending:%d Claimed:%d InProgress:%d Completed:%d Fail:%d",
		pending, claimed, inprog, completed, failed)
	sb.WriteString("|" + padRight(summary, inner) + "|\n")

	// Proof bundle count
	pbCount := p.state.ProofBundleCount
	pbLine := fmt.Sprintf("Proof Bundles: %d", pbCount)
	sb.WriteString("|" + padRight(pbLine, inner) + "|\n")
	sb.WriteString(border + "\n")

	// List entries
	if len(p.proofBundles) == 0 {
		sb.WriteString("|" + padRight("No recent proof bundles", inner) + "|\n")
		sb.WriteString(border)
		return sb.String()
	}

	header := fmt.Sprintf("%-12s %-6s %-30s %-8s", "ID", "PR", "Summary", "Age")
	sb.WriteString("|" + padRight(header, inner) + "|\n")
	sb.WriteString(border + "\n")

	// Show last 5
	start := len(p.proofBundles) - 5
	if start < 0 {
		start = 0
	}
	bundles := p.proofBundles[start:]
	now := time.Now()

	for i, b := range bundles {
		globalIdx := start + i
		isCursor := globalIdx == (len(p.proofBundles)-1 - p.cursor)
		prefix := "  "
		if isCursor {
			prefix = "> "
		}
		id := b.WorkItemID
		if id == "" {
			id = b.ID
		}
		if len(id) > 12 {
			id = id[:11] + "."
		}
		summary := b.Summary
		if len(summary) > 30 {
			summary = summary[:27] + "..."
		}
		age := now.Sub(b.CreatedAt)
		ageStr := fmt.Sprintf("%dm", int(age.Minutes()))
		if age.Minutes() < 1 {
			ageStr = "<1m"
		} else if age.Hours() >= 1 {
			ageStr = fmt.Sprintf("%dh", int(age.Hours()))
		}
		line := fmt.Sprintf("%s%-12s %-6d %-30s %-8s",
			prefix, id, b.PRNumber, summary, ageStr)
		sb.WriteString("|" + padRight(line, inner) + "|\n")
	}
	sb.WriteString(border)
	return sb.String()
}
