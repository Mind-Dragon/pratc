package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

// CorpusOverviewPanel displays corpus statistics.
type CorpusOverviewPanel struct {
	stats data.CorpusStats
}

// NewCorpusOverviewPanel creates a new CorpusOverviewPanel with empty stats.
func NewCorpusOverviewPanel() *CorpusOverviewPanel {
	return &CorpusOverviewPanel{}
}

// SetStats updates the corpus statistics.
func (p *CorpusOverviewPanel) SetStats(stats data.CorpusStats) {
	p.stats = stats
}

// View renders the panel as a string with the given width.
func (p *CorpusOverviewPanel) View(width int) string {
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
	sb.WriteString("|" + padRight("CORPUS OVERVIEW", inner) + "|\n")
	sb.WriteString(border)
	sb.WriteString("\n")

	// Compute sync freshness color and text
	syncFreshness, syncColor := p.getSyncFreshness()
	// Audit entries indicator
	auditIndicator := "📋"

	// First content line
	totalPRsStr := fmt.Sprintf("%d", p.stats.TotalPRs)
	syncFreshnessStr := fmt.Sprintf("%s  [%s]", syncFreshness, syncColor)
	line1 := fmt.Sprintf("Total PRs: %12s    Sync Freshness: %s", totalPRsStr, syncFreshnessStr)
	sb.WriteString("|" + padRight(line1, inner) + "|\n")

	// Second content line
	activeJobsStr := fmt.Sprintf("%d", p.stats.SyncJobsActive)
	auditEntriesStr := fmt.Sprintf("%d", p.stats.AuditEntries)
	line2 := fmt.Sprintf("Active Sync Jobs: %5s    Audit Entries: %s  [%s]", activeJobsStr, auditEntriesStr, auditIndicator)
	sb.WriteString("|" + padRight(line2, inner) + "|\n")

	sb.WriteString(border)
	return sb.String()
}

// getSyncFreshness returns the formatted time since last sync and the appropriate colored circle emoji.
func (p *CorpusOverviewPanel) getSyncFreshness() (string, string) {
	if p.stats.LastSync.IsZero() {
		return "never", "⚪"
	}
	elapsed := time.Since(p.stats.LastSync)
	// Format as Xm ago (minutes)
	minutes := int(elapsed.Minutes())
	if minutes < 1 {
		return "just now", "🟢"
	}
	text := fmt.Sprintf("%dm ago", minutes)
	var color string
	if elapsed < 5*time.Minute {
		color = "🟢" // green circle
	} else if elapsed < 30*time.Minute {
		color = "🟡" // yellow circle
	} else {
		color = "🔴" // red circle
	}
	return text, color
}


