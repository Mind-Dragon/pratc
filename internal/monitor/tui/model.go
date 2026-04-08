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
}

type JobsZone struct {
	Placeholder string
}

type TimelineZone struct {
	Placeholder string
}

type RateLimitZone struct {
	Placeholder string
}

type ConsoleZone struct {
	Placeholder string
}

func New() Model {
	return Model{
		JobsZone: JobsZone{
			Placeholder: "No active jobs",
		},
		TimelineZone: TimelineZone{
			Placeholder: "No activity yet",
		},
		RateLimitZone: RateLimitZone{
			Placeholder: "Rate: 5000/5000",
		},
		ConsoleZone: ConsoleZone{
			Placeholder: "[INFO] Monitor initialized",
		},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyRunes:
			for _, r := range msg.Runes {
				if r == 'q' {
					return m, tea.Quit
				}
			}
		}
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

func getFooter() string {
	return "Tab: Switch | q: Quit"
}
