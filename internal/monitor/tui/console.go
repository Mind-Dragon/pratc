package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

// ConsolePanel displays scrollable structured log entries with color-coded levels.
type ConsolePanel struct {
	entries   []data.LogEntry
	offset    int
	maxLines  int
	cursor    int
	scrollPos int
}

// NewConsolePanel creates a new ConsolePanel with default maxLines of 6.
func NewConsolePanel() *ConsolePanel {
	return &ConsolePanel{
		entries:   make([]data.LogEntry, 0),
		offset:    0,
		maxLines:  6,
		cursor:    0,
		scrollPos: 0,
	}
}

// AddEntry appends a single log entry and auto-scrolls to the bottom.
func (c *ConsolePanel) AddEntry(entry data.LogEntry) {
	c.entries = append(c.entries, entry)
	c.autoScroll()
}

// SetEntries replaces all entries and resets scroll to the bottom.
func (c *ConsolePanel) SetEntries(entries []data.LogEntry) {
	c.entries = entries
	c.autoScroll()
}

// Update handles keyboard input for scrolling through log entries.
// Supports: ↑ (scroll up), ↓ (scroll down).
func (c *ConsolePanel) Update(msg tea.Msg) tea.Cmd {
	if len(c.entries) == 0 {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			c.scrollPos--
			if c.scrollPos < 0 {
				c.scrollPos = 0
			}
			c.offset = c.scrollPos

		case tea.KeyDown:
			c.scrollPos++
			maxScroll := len(c.entries) - c.maxLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			if c.scrollPos > maxScroll {
				c.scrollPos = maxScroll
			}
			c.offset = c.scrollPos
		}
	}
	return nil
}

// autoScroll scrolls to show the newest entries.
func (c *ConsolePanel) autoScroll() {
	maxScroll := len(c.entries) - c.maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	c.scrollPos = maxScroll
	c.offset = c.scrollPos
}

// View renders the console panel with scrollable log entries.
// Each line format: [TIMESTAMP] [LEVEL] [REPO] message
// Color coding: White (info), Yellow (warn), Red (error), Cyan (debug).
func (c *ConsolePanel) View(width int) string {
	if len(c.entries) == 0 {
		return c.renderEmpty()
	}

	var sb strings.Builder
	sb.WriteString(c.renderHeader())
	sb.WriteString("\n")
	sb.WriteString(c.renderEntries())
	sb.WriteString("\n")
	sb.WriteString(c.renderScrollbar())

	return sb.String()
}

func (c *ConsolePanel) renderEmpty() string {
	return "DEBUG CONSOLE\n[ no logs ]"
}

func (c *ConsolePanel) renderHeader() string {
	return "DEBUG CONSOLE"
}

// renderEntries renders the visible log entries based on current offset and maxLines.
func (c *ConsolePanel) renderEntries() string {
	var sb strings.Builder

	visibleCount := 0
	for i := c.offset; i < len(c.entries) && visibleCount < c.maxLines; i++ {
		entry := c.entries[i]
		line := c.formatEntry(entry)
		sb.WriteString(line)
		visibleCount++
		if visibleCount < c.maxLines && i < len(c.entries)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// formatEntry formats a single log entry as: [TIMESTAMP] [LEVEL] [REPO] message
func (c *ConsolePanel) formatEntry(entry data.LogEntry) string {
	timestamp := entry.Timestamp.Format("15:04:05")
	level := c.colorizeLevel(entry.Level)
	repo := c.truncateRepo(entry.Repo, 20)
	message := entry.Message

	return fmt.Sprintf("[%s] %s [%s] %s", timestamp, level, repo, message)
}

// colorizeLevel returns ANSI-colored level string based on log level.
func (c *ConsolePanel) colorizeLevel(level string) string {
	switch strings.ToUpper(level) {
	case "ERROR", "ERR":
		return "\033[31m[ERROR]\033[0m"
	case "WARN", "WARNING":
		return "\033[33m[WARN]\033[0m"
	case "DEBUG", "DBG":
		return "\033[36m[DEBUG]\033[0m"
	case "INFO", "INF":
		return "\033[37m[INFO]\033[0m"
	default:
		return fmt.Sprintf("\033[37m[%s]\033[0m", strings.ToUpper(level))
	}
}

// truncateRepo truncates a repo name to fit within the given width.
func (c *ConsolePanel) truncateRepo(repo string, maxLen int) string {
	if len(repo) == 0 {
		return "-"
	}
	if len(repo) <= maxLen {
		return repo
	}
	return repo[:maxLen-2] + ".."
}

// renderScrollbar shows a visual indicator of scroll position.
func (c *ConsolePanel) renderScrollbar() string {
	total := len(c.entries)
	if total <= c.maxLines {
		return ""
	}

	visibleCount := c.maxLines
	scrollPercent := float64(c.offset) / float64(total-visibleCount)
	if scrollPercent > 1 {
		scrollPercent = 1
	}

	position := int(scrollPercent * float64(visibleCount))
	if position >= visibleCount {
		position = visibleCount - 1
	}

	var sb strings.Builder
	sb.WriteString("[")
	for i := range visibleCount {
		if i == position {
			sb.WriteString("▼")
		} else {
			sb.WriteString("─")
		}
	}
	sb.WriteString("]")

	return sb.String()
}

// EntryCount returns the number of log entries.
func (c *ConsolePanel) EntryCount() int {
	return len(c.entries)
}
