package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/jeffersonnunn/pratc/internal/monitor/data"
	"github.com/jeffersonnunn/pratc/internal/monitor/tui"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestTUI_RenderWithoutCrash(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view from new model")
	}

	if !strings.Contains(view, "prATC MONITOR") {
		t.Error("expected header to contain 'prATC MONITOR'")
	}
	if !strings.Contains(view, "Budget:") {
		t.Error("expected header to contain 'Budget:'")
	}
	if !strings.Contains(view, "GitHub:") {
		t.Error("expected header to contain 'GitHub:'")
	}
}

func TestTUI_KeyboardNavigation(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	if m.ActiveZone != tui.ZoneJobs {
		t.Errorf("expected initial zone to be ZoneJobs, got %v", m.ActiveZone)
	}

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	_, _ = m.Update(tabMsg)

	if m.ActiveZone != tui.ZoneTimeline {
		t.Errorf("expected zone to advance to ZoneTimeline after Tab, got %v", m.ActiveZone)
	}

	_, _ = m.Update(tabMsg)
	if m.ActiveZone != tui.ZoneRateLimit {
		t.Errorf("expected zone to advance to ZoneRateLimit after 2nd Tab, got %v", m.ActiveZone)
	}

	_, _ = m.Update(tabMsg)
	if m.ActiveZone != tui.ZoneConsole {
		t.Errorf("expected zone to advance to ZoneConsole after 3rd Tab, got %v", m.ActiveZone)
	}

	_, _ = m.Update(tabMsg)
	if m.ActiveZone != tui.ZoneJobs {
		t.Errorf("expected zone to cycle back to ZoneJobs after 4th Tab, got %v", m.ActiveZone)
	}
}

func TestTUI_QuitKey(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(qMsg)

	if cmd == nil {
		t.Error("expected non-nil cmd after 'q' key press")
	}
}

func TestTUI_EscapeKey(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(escMsg)

	if cmd == nil {
		t.Error("expected non-nil cmd after Escape key press")
	}
}

func TestTUI_JobsNavigation(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	_, _ = m.Update(upMsg)

	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	_, _ = m.Update(downMsg)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view after navigation")
	}
}

func TestTUI_HelpToggle(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	if m.ShowHelp {
		t.Error("expected ShowHelp to be false initially")
	}

	helpMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	_, _ = m.Update(helpMsg)

	if !m.ShowHelp {
		t.Error("expected ShowHelp to be true after '?' key")
	}

	_, _ = m.Update(helpMsg)

	if m.ShowHelp {
		t.Error("expected ShowHelp to be false after second '?' key")
	}
}

func TestTUI_DataUpdatesRenderCorrectly(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	update := data.DataUpdate{
		Timestamp: time.Now(),
		SyncJobs: []data.SyncJobView{
			{ID: "job1", Repo: "owner/repo", Progress: 50, Status: "active"},
		},
	}

	_, _ = m.Update(update)

	view := m.View()
	if !strings.Contains(view, "owner/repo") {
		t.Error("expected view to contain repo name after data update")
	}
}

func TestTUI_ActionPlanDashboardRendersReadOnlyLaneSummary(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)
	plan := &types.ActionPlan{
		Repo:          "openclaw/openclaw",
		PolicyProfile: types.PolicyProfileAdvisory,
		CorpusSnapshot: types.ActionCorpusSnapshot{
			TotalPRs: 3,
		},
		Lanes: []types.ActionLaneSummary{
			{Lane: types.ActionLaneFastMerge, Count: 2},
			{Lane: types.ActionLaneHumanEscalate, Count: 1},
		},
	}

	_, _ = m.Update(data.DataUpdate{Timestamp: time.Now(), ActionPlan: plan})

	view := m.View()
	for _, expected := range []string{"ACTIONS", "read-only", "openclaw/openclaw", "advisory", "fast_merge", "2", "human_escalate", "1"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected action dashboard to contain %q; view:\n%s", expected, view)
		}
	}
}

func TestTUI_RateLimitUpdate(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	resetTime := time.Now().Add(1 * time.Hour)
	update := data.DataUpdate{
		Timestamp: time.Now(),
		RateLimit: data.RateLimitView{
			Remaining: 4200,
			Total:     5000,
			ResetTime: resetTime,
		},
	}

	_, _ = m.Update(update)

	if m.BudgetRemaining != 4200 {
		t.Errorf("expected BudgetRemaining 4200, got %d", m.BudgetRemaining)
	}
	if m.BudgetTotal != 5000 {
		t.Errorf("expected BudgetTotal 5000, got %d", m.BudgetTotal)
	}
}

func TestTUI_ConsoleScroll(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)
	m.ActiveZone = tui.ZoneConsole

	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	_, _ = m.Update(upMsg)

	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	_, _ = m.Update(downMsg)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view after console scroll")
	}
}

func TestTUI_TimelineScroll(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)
	m.ActiveZone = tui.ZoneTimeline

	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	_, _ = m.Update(leftMsg)

	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	_, _ = m.Update(rightMsg)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view after timeline scroll")
	}
}

func TestTUI_PauseResume(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	pMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	_, _ = m.Update(pMsg)

	if !m.IsPaused {
		t.Error("expected IsPaused to be true after 'p' key")
	}

	rMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	_, _ = m.Update(rMsg)

	if m.IsPaused {
		t.Error("expected IsPaused to be false after 'r' key")
	}
}

func TestTUI_AllPanelsDisplayData(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	update := data.DataUpdate{
		Timestamp: time.Now(),
		SyncJobs: []data.SyncJobView{
			{ID: "job1", Repo: "test/repo", Progress: 75, Status: "active", Detail: "Syncing"},
			{ID: "job2", Repo: "other/repo", Progress: 30, Status: "queued"},
		},
		RateLimit: data.RateLimitView{
			Remaining: 3500,
			Total:     5000,
			ResetTime: time.Now().Add(45 * time.Minute),
		},
		ActivityBuckets: []data.ActivityBucket{
			{TimeWindow: time.Now().Add(-1 * time.Hour), RequestCount: 150, JobCount: 3},
			{TimeWindow: time.Now(), RequestCount: 200, JobCount: 5},
		},
		RecentLogs: []data.LogEntry{
			{Timestamp: time.Now(), Level: "info", Repo: "test/repo", Message: "Job started"},
			{Timestamp: time.Now(), Level: "warn", Repo: "test/repo", Message: "Rate limit low"},
		},
	}

	_, _ = m.Update(update)

	view := m.View()

	if !strings.Contains(view, "test/repo") {
		t.Error("expected view to contain 'test/repo' from Jobs panel")
	}

	if !strings.Contains(view, "Job started") {
		t.Error("expected view to contain log message from Console panel")
	}
}

func TestTUI_ViewJobDetail(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)
	m.ActiveZone = tui.ZoneJobs

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, _ = m.Update(enterMsg)

	if !m.IsViewingJob {
		t.Error("expected IsViewingJob to be true after Enter in Jobs zone")
	}
}

func TestTUI_TickUpdates(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	for i := 0; i < 5; i++ {
		tickMsg := tui.TickMsg(time.Now())
		_, _ = m.Update(tickMsg)

		view := m.View()
		if view == "" {
			t.Error("expected non-empty view after tick updates")
		}
	}
}

func TestTUI_ZoneNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		zone     tui.Zone
		expected string
	}{
		{tui.ZoneJobs, "Jobs"},
		{tui.ZoneTimeline, "Timeline"},
		{tui.ZoneRateLimit, "RateLimit"},
		{tui.ZoneConsole, "Console"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.zone.String() != tt.expected {
				t.Errorf("expected zone name %q, got %q", tt.expected, tt.zone.String())
			}
		})
	}
}

func TestTUI_NextZoneCyclesCorrectly(t *testing.T) {
	t.Parallel()

	zone := tui.ZoneJobs

	expected := []tui.Zone{
		tui.ZoneTimeline,
		tui.ZoneRateLimit,
		tui.ZoneConsole,
		tui.ZoneJobs,
	}

	for i, exp := range expected {
		next := zone.Next()
		if next != exp {
			t.Errorf("cycle %d: expected %v, got %v", i, exp, next)
		}
		zone = next
	}
}

func TestTUI_HelpTextNotEmpty(t *testing.T) {
	t.Parallel()

	help := tui.HelpText()
	if help == "" {
		t.Error("expected non-empty help text")
	}

	if !strings.Contains(help, "KEYBINDINGS") {
		t.Error("expected help text to contain 'KEYBINDINGS'")
	}
	if !strings.Contains(help, "Tab") {
		t.Error("expected help text to contain 'Tab'")
	}
	if !strings.Contains(help, "q") {
		t.Error("expected help text to contain 'q' for quit")
	}
}

func TestTUI_FooterHintsChangesPerZone(t *testing.T) {
	t.Parallel()

	m := tui.New(nil)

	zones := []struct {
		zone       tui.Zone
		shouldBeIn string
	}{
		{tui.ZoneJobs, "Navigate"},
		{tui.ZoneTimeline, "Scroll time"},
		{tui.ZoneRateLimit, "Pause"},
		{tui.ZoneConsole, "Scroll logs"},
	}

	for _, tt := range zones {
		t.Run(tt.zone.String(), func(t *testing.T) {
			m.ActiveZone = tt.zone
			hints := m.FooterHints()
			if !strings.Contains(hints, tt.shouldBeIn) {
				t.Errorf("expected footer hints for %v to contain %q, got %q", tt.zone, tt.shouldBeIn, hints)
			}
		})
	}
}
