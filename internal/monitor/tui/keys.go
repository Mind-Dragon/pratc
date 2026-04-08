package tui

import "github.com/charmbracelet/bubbletea"

// Zone represents the active focus zone in the TUI.
type Zone int

const (
	ZoneJobs Zone = iota
	ZoneTimeline
	ZoneRateLimit
	ZoneConsole
	numZones
)

// ZoneNames returns the display name for each zone.
var ZoneNames = [...]string{"Jobs", "Timeline", "RateLimit", "Console"}

// NextZone advances to the next zone in cycle.
func (z Zone) Next() Zone {
	return Zone((z + 1) % numZones)
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
				m.JobsZone.cursor--
				if m.JobsZone.cursor < 0 {
					m.JobsZone.cursor = 0
				}
				handled = true
			case tea.KeyDown:
				m.JobsZone.cursor++
				handled = true
			case tea.KeyEnter:
				// View job details - handled by IsViewingJob flag
				m.IsViewingJob = true
				handled = true
			}

		case ZoneTimeline:
			switch msg.Type {
			case tea.KeyLeft:
				m.TimelineZone.scrollOffset--
				handled = true
			case tea.KeyRight:
				m.TimelineZone.scrollOffset++
				handled = true
			}

		case ZoneConsole:
			switch msg.Type {
			case tea.KeyUp:
				m.ConsoleZone.scrollOffset--
				if m.ConsoleZone.scrollOffset < 0 {
					m.ConsoleZone.scrollOffset = 0
				}
				handled = true
			case tea.KeyDown:
				m.ConsoleZone.scrollOffset++
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
