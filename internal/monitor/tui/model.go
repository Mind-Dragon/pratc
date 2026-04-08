package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
)

type Model struct {
	JobsZone      JobsZone
	TimelineZone  TimelineZone
	RateLimitZone RateLimitZone
	ConsoleZone   ConsoleZone
	width         int
	height        int
	ActiveZone    Zone
	ShowHelp      bool
	IsPaused      bool
	IsRestarting  bool
	IsViewingJob  bool
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
		ActiveZone: ZoneJobs,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.HandleKey(msg)
	}
	return m, nil
}

func (m Model) View() string {
	return Render(m)
}

func getHeader() string {
	now := time.Now().UTC()
	return fmt.Sprintf("prATC MONITOR [🟢 LIVE] UTC: %s", now.Format("15:04:05"))
}

func (m Model) getFooter() string {
	return m.FooterHints()
}
