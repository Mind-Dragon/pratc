package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

type Model struct {
	// Broadcaster for receiving data updates
	broadcaster     *data.Broadcaster
	updateChan      chan data.DataUpdate
	refreshInterval time.Duration

	// Panel components for real data display
	JobsPanel      *JobsList
	TimelinePanel  *TimelinePanel
	RateLimitPanel *RateLimitPanel
	ConsolePanel   *ConsolePanel

	// Zone state
	width        int
	height       int
	ActiveZone   Zone
	ShowHelp     bool
	IsPaused     bool
	IsRestarting bool
	IsViewingJob bool

	// Legacy fields for header/footer compatibility
	BudgetRemaining int
	BudgetTotal     int
	ResetInMinutes  int
	GitHubOK        bool
}

type JobsZone struct {
	Placeholder string
	cursor      int
}

type TimelineZone struct {
	Placeholder  string
	scrollOffset int
}

type RateLimitZone struct {
	Placeholder string
}

type ConsoleZone struct {
	Placeholder  string
	scrollOffset int
}

func New(broadcaster *data.Broadcaster) Model {
	m := Model{
		broadcaster:     broadcaster,
		updateChan:      make(chan data.DataUpdate, 64),
		refreshInterval: time.Second,
		JobsPanel:       NewJobsList(),
		TimelinePanel:   NewTimelinePanel(),
		RateLimitPanel:  NewRateLimitPanel(),
		ConsolePanel:    NewConsolePanel(),
		ActiveZone:      ZoneJobs,
		BudgetRemaining: 4200,
		BudgetTotal:     5000,
		ResetInMinutes:  43,
		GitHubOK:        true,
	}
	return m
}

func (m *Model) SetRefreshInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	m.refreshInterval = interval
}

func (m *Model) tickInterval() time.Duration {
	if m.refreshInterval <= 0 {
		return time.Second
	}
	return m.refreshInterval
}

func (m *Model) Init() tea.Cmd {
	if m.broadcaster != nil {
		ch := m.broadcaster.Subscribe()
		go m.receiveUpdates(ch)
	}
	return tea.Tick(m.tickInterval(), func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *Model) receiveUpdates(ch chan data.DataUpdate) {
	for update := range ch {
		m.updateChan <- update
	}
}

type TickMsg time.Time

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.HandleKey(msg)
	case TickMsg:
		m.JobsPanel.Update(TickMsg(msg))
		return m, tea.Tick(m.tickInterval(), func(t time.Time) tea.Msg {
			return TickMsg(t)
		})
	case data.DataUpdate:
		return m.handleDataUpdate(msg)
	}
	return m, nil
}

func (m *Model) handleDataUpdate(update data.DataUpdate) (tea.Model, tea.Cmd) {
	if len(update.SyncJobs) > 0 {
		m.JobsPanel.SetJobs(update.SyncJobs)
	}
	if update.RateLimit.Total > 0 {
		m.RateLimitPanel.SetRateLimit(update.RateLimit)
		m.BudgetRemaining = update.RateLimit.Remaining
		m.BudgetTotal = update.RateLimit.Total
		if !update.RateLimit.ResetTime.IsZero() {
			m.ResetInMinutes = int(update.RateLimit.ResetTime.Sub(time.Now()).Minutes())
		}
		m.GitHubOK = true
	}
	if len(update.ActivityBuckets) > 0 {
		m.TimelinePanel.SetBuckets(update.ActivityBuckets)
	}
	if len(update.RecentLogs) > 0 {
		m.ConsolePanel.SetEntries(update.RecentLogs)
	}
	return m, nil
}

func (m Model) View() string {
	return Render(m)
}

func (m Model) getHeader() string {
	now := time.Now().UTC()
	githubIndicator := "✅"
	if !m.GitHubOK {
		githubIndicator = "⚠️"
	}
	return fmt.Sprintf("prATC MONITOR [🟢 LIVE] UTC: %s | Budget: %s | Resets: %dm | GitHub: %s",
		now.Format("15:04:05"),
		formatBudget(m.BudgetRemaining, m.BudgetTotal),
		m.ResetInMinutes,
		githubIndicator,
	)
}

func formatBudget(remaining, total int) string {
	return fmt.Sprintf("%d/%d", remaining, total)
}

func (m Model) getFooter() string {
	return m.FooterHints()
}
