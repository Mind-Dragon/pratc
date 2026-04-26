package tui

import (
	"fmt"
	"strings"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

type AuditLedgerPanel struct {
	ledger  data.AuditLedger
	cursor  int
}

func NewAuditLedgerPanel() *AuditLedgerPanel {
	return &AuditLedgerPanel{
		cursor: 0,
	}
}

func (p *AuditLedgerPanel) SetLedger(ledger data.AuditLedger) {
	p.ledger = ledger
	p.cursor = 0
}

func (p *AuditLedgerPanel) View(width int) string {
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
	sb.WriteString("|" + padRight("AUDIT LEDGER", inner) + "|\n")
	sb.WriteString(border + "\n")

	// List entries
	if len(p.ledger.Entries) == 0 {
		sb.WriteString("|" + padRight("No audit entries", inner) + "|\n")
		sb.WriteString(border)
		return sb.String()
	}

	header := fmt.Sprintf("%-8s %-12s %-20s %-25s", "Time", "Action", "Actor", "Reason")
	sb.WriteString("|" + padRight(header, inner) + "|\n")
	sb.WriteString(border + "\n")

	// Show last 10 entries
	start := len(p.ledger.Entries) - 10
	if start < 0 {
		start = 0
	}
	entries := p.ledger.Entries[start:]

	for i, e := range entries {
		globalIdx := start + i
		isCursor := globalIdx == (len(p.ledger.Entries)-1 - p.cursor)
		prefix := "  "
		if isCursor {
			prefix = "> "
		}
		timestamp := e.Timestamp.Format("15:04:05")
		action := e.Action
		if len(action) > 12 {
			action = action[:11] + "."
		}
		actor := e.Actor
		if len(actor) > 20 {
			actor = actor[:19] + "."
		}
		reason := e.Reason
		if len(reason) > 25 {
			reason = reason[:22] + "..."
		}
		line := fmt.Sprintf("%s%-8s %-12s %-20s %-25s",
			prefix, timestamp, action, actor, reason)
		sb.WriteString("|" + padRight(line, inner) + "|\n")
	}
	sb.WriteString(border)
	return sb.String()
}
