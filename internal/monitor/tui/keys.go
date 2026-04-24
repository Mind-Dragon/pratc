package tui

import tea "github.com/charmbracelet/bubbletea"

// Zone represents the active focus zone in the TUI.
type Zone int

const (
	ZoneJobs Zone = iota
	ZoneTimeline
	ZoneRateLimit
	ZoneConsole
	numNavigableZones
	ZoneDetail
	numZones
)

// ZoneNames returns the display name for each zone.
var ZoneNames = [...]string{"Jobs", "Timeline", "RateLimit", "Console", "Detail"}

// NextZone advances to the next primary zone in cycle.
func (z Zone) Next() Zone {
	return Zone((z + 1) % numNavigableZones)
}

// String returns the zone name.
func (z Zone) String() string {
	return ZoneNames[z]
}

// KeyBindings holds all key bindings for the TUI.
type KeyBindings struct {
	Quit    bool
	Help    bool
	Tab     bool
	Up      bool
	Down    bool
	Left    bool
	Right   bool
	Enter   bool
	Pause   bool
	Resume  bool
	Restart bool
}

// HandleKey processes a key message and returns updated KeyBindings.
func (m *Model) HandleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Always handle global keys first
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	}

	var handled bool

	switch msg.Type {
	case tea.KeyTab:
		m.ActiveZone = m.ActiveZone.Next()
		handled = true

	case tea.KeyRunes:
		for _, r := range msg.Runes {
			switch r {
			case 'q':
				return m, tea.Quit
			case '?':
				m.ShowHelp = !m.ShowHelp
				handled = true
			case 'p':
				m.IsPaused = true
				handled = true
			case 'r':
				if m.IsPaused {
					m.IsPaused = false
					handled = true
				}
			case 's':
				m.IsRestarting = true
				handled = true
			}
		}
	}

	// Zone-specific navigation
	if !handled {
		switch m.ActiveZone {
		case ZoneJobs:
			switch msg.Type {
			case tea.KeyUp:
				m.JobsPanel.cursor--
				if m.JobsPanel.cursor < 0 {
					m.JobsPanel.cursor = 0
				}
				handled = true
			case tea.KeyDown:
				m.JobsPanel.cursor++
				handled = true
			case tea.KeyEnter:
				m.IsViewingJob = true
				// For demo, select first work item from ActionLaneBoard
				if m.ActionLaneBoard != nil && m.ActionLaneBoard.WorkItemCount() > 0 {
					m.SelectedWorkItem = m.ActionLaneBoard.GetWorkItem(0)
				}
				m.ActiveZone = ZoneDetail
				handled = true
			}

		case ZoneTimeline:
			switch msg.Type {
			case tea.KeyLeft:
				handled = true
			case tea.KeyRight:
				handled = true
			}

		case ZoneConsole:
			switch msg.Type {
			case tea.KeyUp:
				m.ConsolePanel.scrollPos--
				if m.ConsolePanel.scrollPos < 0 {
					m.ConsolePanel.scrollPos = 0
				}
				m.ConsolePanel.offset = m.ConsolePanel.scrollPos
				handled = true
			case tea.KeyDown:
				m.ConsolePanel.scrollPos++
				maxScroll := len(m.ConsolePanel.entries) - m.ConsolePanel.maxLines
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.ConsolePanel.scrollPos > maxScroll {
					m.ConsolePanel.scrollPos = maxScroll
				}
				m.ConsolePanel.offset = m.ConsolePanel.scrollPos
				handled = true
			}
		case ZoneDetail:
			switch msg.Type {
			case tea.KeyEsc, tea.KeyEnter:
				// Exit detail view
				m.IsViewingJob = false
				m.SelectedWorkItem = nil
				m.ActiveZone = ZoneJobs
				handled = true
			}
		}
	}

	return m, nil
}

// HelpText returns the help overlay content.
func HelpText() string {
	return `KEYBINDINGS
==========
Global:
  Tab      Switch zones (Jobs → Timeline → RateLimit → Console)
  ?        Toggle this help overlay
  q        Quit

Navigation:
  ↑/↓      Navigate in Jobs zone / Console zone
  ←/→      Scroll timeline

Actions:
  Enter    View job details
  p        Pause monitoring
  r        Resume monitoring
  s        Restart sync`
}

// FooterHints returns context-sensitive footer hints based on active zone.
func (m Model) FooterHints() string {
	base := "Tab: Switch | q: Quit | ?: Help"

	switch m.ActiveZone {
	case ZoneJobs:
		return base + " | ↑↓: Navigate | Enter: Details"
	case ZoneTimeline:
		return base + " | ←→: Scroll time"
	case ZoneRateLimit:
		return base + " | p: Pause | r: Resume"
	case ZoneConsole:
		return base + " | ↑↓: Scroll logs"
	default:
		return base
	}
}
