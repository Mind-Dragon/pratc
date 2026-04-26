package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
	"github.com/jeffersonnunn/pratc/internal/monitor/tui"
)

func TestCorpusOverviewPanel_Render(t *testing.T) {
	panel := tui.NewCorpusOverviewPanel()
	stats := data.CorpusStats{
		TotalPRs:       1234,
		LastSync:       time.Now().Add(-2 * time.Minute),
		SyncJobsActive: 5,
		AuditEntries:   4567,
	}
	panel.SetStats(stats)

	view := panel.View(94)
	if view == "" {
		t.Fatal("expected non-empty view")
	}

	// Check basic content
	if !strings.Contains(view, "CORPUS OVERVIEW") {
		t.Error("expected header 'CORPUS OVERVIEW'")
	}
	if !strings.Contains(view, "Total PRs:") {
		t.Error("expected 'Total PRs:'")
	}
	if !strings.Contains(view, "Sync Freshness:") {
		t.Error("expected 'Sync Freshness:'")
	}
	if !strings.Contains(view, "Active Sync Jobs:") {
		t.Error("expected 'Active Sync Jobs:'")
	}
	if !strings.Contains(view, "Audit Entries:") {
		t.Error("expected 'Audit Entries:'")
	}

	// Check that numbers appear
	if !strings.Contains(view, "1234") {
		t.Error("expected total PRs number 1234")
	}
	if !strings.Contains(view, "5") {
		t.Error("expected active sync jobs number 5")
	}
	if !strings.Contains(view, "4567") {
		t.Error("expected audit entries number 4567")
	}

	// Check that sync freshness shows "2m ago" (or similar)
	if !strings.Contains(view, "2m ago") {
		t.Error("expected sync freshness to contain '2m ago'")
	}

	// Check that panel has proper borders
	lines := strings.Split(view, "\n")
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "+") || !strings.HasSuffix(lines[0], "+") {
		t.Error("first line should be border")
	}
	if !strings.HasPrefix(lines[2], "+") || !strings.HasSuffix(lines[2], "+") {
		t.Error("third line should be border")
	}
}

func TestCorpusOverviewPanel_ZeroLastSync(t *testing.T) {
	panel := tui.NewCorpusOverviewPanel()
	stats := data.CorpusStats{
		TotalPRs:       0,
		LastSync:       time.Time{}, // zero
		SyncJobsActive: 0,
		AuditEntries:   0,
	}
	panel.SetStats(stats)
	view := panel.View(80)
	if !strings.Contains(view, "never") {
		t.Error("expected 'never' for zero LastSync")
	}
}

func TestCorpusOverviewPanel_Colors(t *testing.T) {
	panel := tui.NewCorpusOverviewPanel()
	now := time.Now()
	tests := []struct {
		name     string
		lastSync time.Time
		want     string // substring expected in view
	}{
		{"green", now.Add(-2 * time.Minute), "🟢"},
		{"yellow", now.Add(-10 * time.Minute), "🟡"},
		{"red", now.Add(-40 * time.Minute), "🔴"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := data.CorpusStats{
				TotalPRs:       1,
				LastSync:       tt.lastSync,
				SyncJobsActive: 0,
				AuditEntries:   0,
			}
			panel.SetStats(stats)
			view := panel.View(94)
			if !strings.Contains(view, tt.want) {
				t.Errorf("expected color %s in view", tt.want)
			}
		})
	}
}

func TestCorpusOverviewPanel_DebugPrint(t *testing.T) {
	panel := tui.NewCorpusOverviewPanel()
	stats := data.CorpusStats{
		TotalPRs:       1234,
		LastSync:       time.Now().Add(-2 * time.Minute),
		SyncJobsActive: 5,
		AuditEntries:   4567,
	}
	panel.SetStats(stats)
	view := panel.View(94)
	t.Logf("Panel output (width 94):\n%s", view)
	// Ensure each line is exactly 94 characters (including newline? we count lines)
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if len(line) > 94 {
			t.Errorf("Line %d exceeds width 94: length %d", i, len(line))
		}
	}
}

func TestCorpusOverviewPanel_Widths(t *testing.T) {
	panel := tui.NewCorpusOverviewPanel()
	stats := data.CorpusStats{
		TotalPRs:       1234,
		LastSync:       time.Now().Add(-2 * time.Minute),
		SyncJobsActive: 5,
		AuditEntries:   4567,
	}
	panel.SetStats(stats)
	
	// Test default width (94)
	view := panel.View(0)
	if view == "" {
		t.Error("expected non-empty view for width 0")
	}
	
	// Test minimum width (should be at least 20)
	view = panel.View(10)
	if view == "" {
		t.Error("expected non-empty view for width 10")
	}
	
	// Test larger width
	view = panel.View(120)
	if view == "" {
		t.Error("expected non-empty view for width 120")
	}
	
	// Ensure borders are present
	if !strings.Contains(view, "+") {
		t.Error("expected border lines")
	}
}

func TestCorpusOverviewPanel_LayoutDetails(t *testing.T) {
	panel := tui.NewCorpusOverviewPanel()
	stats := data.CorpusStats{
		TotalPRs:       1234,
		LastSync:       time.Now().Add(-2 * time.Minute),
		SyncJobsActive: 5,
		AuditEntries:   4567,
	}
	panel.SetStats(stats)
	view := panel.View(94)
	t.Logf("Panel output:\n%s", view)
	
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		t.Logf("Line %d length %d: %q", i, len(line), line)
		if len(line) > 94 {
			t.Errorf("Line %d exceeds width 94: length %d", i, len(line))
		}
	}
	// Ensure first line is border
	if !strings.HasPrefix(lines[0], "+") || !strings.HasSuffix(lines[0], "+") {
		t.Error("first line should be border")
	}
	// Ensure second line contains title
	if !strings.Contains(lines[1], "CORPUS OVERVIEW") {
		t.Error("second line should contain title")
	}
	// Ensure third line is border
	if !strings.HasPrefix(lines[2], "+") || !strings.HasSuffix(lines[2], "+") {
		t.Error("third line should be border")
	}
}
