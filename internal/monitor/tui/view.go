package tui

import (
	"fmt"
	"strings"
)

func Render(m Model) string {
	header := m.getHeader()
	footer := m.getFooter()

	jobsPanel := renderJobsZone(m.JobsZone)
	timelinePanel := renderTimelineZone(m.TimelineZone)
	rateLimitPanel := renderRateLimitZone(m.RateLimitZone)

	zones := sideBySide(jobsPanel, timelinePanel, rateLimitPanel, 30, 40, 30)

	consolePanel := renderConsoleZone(m.ConsoleZone)

	var helpOverlay string
	if m.ShowHelp {
		helpOverlay = renderHelpOverlay()
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s%s",
		header,
		zones,
		consolePanel,
		footer,
		helpOverlay,
	)
}

func renderHelpOverlay() string {
	border := "+" + strings.Repeat("-", 60) + "+"
	content := "|" + padCenter("KEYBINDINGS", 60) + "|"

	var sb strings.Builder
	sb.WriteString("\n\n")
	sb.WriteString(border)
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString("\n")
	sb.WriteString(border)
	sb.WriteString("\n")

	lines := strings.Split(HelpText(), "\n")
	for _, line := range lines {
		sb.WriteString("|" + padCenter(line, 60) + "|\n")
	}
	sb.WriteString(border)

	return sb.String()
}

func renderJobsZone(z JobsZone) string {
	border := "+" + strings.Repeat("-", 28) + "+"
	content := "|" + padCenter("JOBS", 28) + "|"

	var sb strings.Builder
	sb.WriteString(border)
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString("\n")
	sb.WriteString(border)
	sb.WriteString("\n")

	lines := strings.Split(z.Placeholder, "\n")
	for _, line := range lines {
		sb.WriteString("|" + padCenter(line, 28) + "|\n")
	}
	sb.WriteString(border)

	return sb.String()
}

func renderTimelineZone(z TimelineZone) string {
	border := "+" + strings.Repeat("-", 38) + "+"
	content := "|" + padCenter("TIMELINE", 38) + "|"

	var sb strings.Builder
	sb.WriteString(border)
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString("\n")
	sb.WriteString(border)
	sb.WriteString("\n")

	lines := strings.Split(z.Placeholder, "\n")
	for _, line := range lines {
		sb.WriteString("|" + padCenter(line, 38) + "|\n")
	}
	sb.WriteString(border)

	return sb.String()
}

func renderRateLimitZone(z RateLimitZone) string {
	border := "+" + strings.Repeat("-", 28) + "+"
	content := "|" + padCenter("RATE LIMIT", 28) + "|"

	var sb strings.Builder
	sb.WriteString(border)
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString("\n")
	sb.WriteString(border)
	sb.WriteString("\n")

	lines := strings.Split(z.Placeholder, "\n")
	for _, line := range lines {
		sb.WriteString("|" + padCenter(line, 28) + "|\n")
	}
	sb.WriteString(border)

	return sb.String()
}

func renderConsoleZone(z ConsoleZone) string {
	border := "+" + strings.Repeat("-", 94) + "+"
	content := "|" + padCenter("CONSOLE", 94) + "|"

	var sb strings.Builder
	sb.WriteString(border)
	sb.WriteString("\n")
	sb.WriteString(content)
	sb.WriteString("\n")
	sb.WriteString(border)
	sb.WriteString("\n")

	lines := strings.Split(z.Placeholder, "\n")
	for _, line := range lines {
		sb.WriteString("|" + padCenter(line, 94) + "|\n")
	}
	sb.WriteString(border)

	return sb.String()
}

func sideBySide(left, center, right string, leftPct, centerPct, rightPct int) string {
	leftLines := strings.Split(left, "\n")
	centerLines := strings.Split(center, "\n")
	rightLines := strings.Split(right, "\n")

	maxLines := max(len(leftLines), len(centerLines), len(rightLines))

	var sb strings.Builder
	for i := 0; i < maxLines; i++ {
		l := getLine(leftLines, i)
		c := getLine(centerLines, i)
		r := getLine(rightLines, i)
		sb.WriteString(l + " " + c + " " + r + "\n")
	}

	return sb.String()
}

func getLine(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return strings.Repeat(" ", 30)
}

func padCenter(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := width - len(s)
	leftPad := pad / 2
	rightPad := pad - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

func max(a, b, c int) int {
	if a > b {
		if a > c {
			return a
		}
		return c
	}
	if b > c {
		return b
	}
	return c
}
