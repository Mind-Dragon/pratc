package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
)

type Model struct {
	JobsZone        JobsZone
	TimelineZone    TimelineZone
	RateLimitZone   RateLimitZone
	ConsoleZone     ConsoleZone
	width           int
	height          int
	ActiveZone      Zone
	ShowHelp        bool
	IsPaused        bool
	IsRestarting    bool
	IsViewingJob    bool
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

func New() Model {
	return Model{
		JobsZone: JobsZone{
			Placeholder: "No active jobs",
			cursor:      0,
		},
		TimelineZone: TimelineZone{
			Placeholder:  "No activity yet",
			scrollOffset: 0,
		},
		RateLimitZone: RateLimitZone{
			Placeholder: "Rate: 5000/5000",
		},
		ConsoleZone: ConsoleZone{
			Placeholder:  "[INFO] Monitor initialized",
			scrollOffset: 0,
		},
		ActiveZone:      ZoneJobs,
		BudgetRemaining: 4200,
		BudgetTotal:     5000,
		ResetInMinutes:  43,
		GitHubOK:        true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.HandleKey(msg)
	case tickMsg:
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
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
