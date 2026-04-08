package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

// RateLimitPanel displays GitHub API rate limit as a vertical thermometer gauge.
type RateLimitPanel struct {
	remaining int
	total     int
	resetTime time.Time
	hasData   bool
}

// NewRateLimitPanel creates a new RateLimitPanel with no data.
func NewRateLimitPanel() *RateLimitPanel {
	return &RateLimitPanel{
		remaining: 0,
		total:     0,
		resetTime: time.Time{},
		hasData:   false,
	}
}

// SetRateLimit updates the rate limit data from a RateLimitView.
func (r *RateLimitPanel) SetRateLimit(rl data.RateLimitView) {
	r.remaining = rl.Remaining
	r.total = rl.Total
	r.resetTime = rl.ResetTime
	r.hasData = rl.Total > 0
}

// View renders the rate limit panel as a vertical thermometer gauge.
// The gauge shows remaining API calls out of total, with color zones
// (green >2000, amber 500-2000, red <500) and a countdown to reset.
func (r *RateLimitPanel) View(width int) string {
	if !r.hasData {
		return r.renderEmpty()
	}

	var sb strings.Builder
	sb.WriteString(r.renderHeader())
	sb.WriteString("\n")
	sb.WriteString(r.renderThermometer())
	sb.WriteString("\n")
	sb.WriteString(r.renderReadout())
	sb.WriteString("\n")
	sb.WriteString(r.renderCountdown())

	return sb.String()
}

func (r *RateLimitPanel) renderEmpty() string {
	return "RATE LIMIT\n[ no data ]"
}

func (r *RateLimitPanel) renderHeader() string {
	return "RATE LIMIT"
}

// renderThermometer draws a vertical thermometer gauge using Unicode block characters.
// The gauge has 10 segments representing the rate limit percentage.
func (r *RateLimitPanel) renderThermometer() string {
	const segments = 10
	const scale = 5000 // GitHub rate limit is 5000/hour

	// Calculate fill level (0 to segments)
	fillLevel := (r.remaining * segments) / scale
	if fillLevel > segments {
		fillLevel = segments
	}
	if fillLevel < 0 {
		fillLevel = 0
	}

	// Determine color based on remaining budget
	color := r.getColor()

	// Build thermometer display
	var sb strings.Builder

	// Top of thermometer
	sb.WriteString(color)
	sb.WriteString("  ┌─┐")
	sb.WriteString("\033[0m")
	sb.WriteString("\n")

	// Draw segments from top to bottom
	for i := segments; i >= 1; i-- {
		block := r.getBlockForLevel(i, fillLevel, color)
		sb.WriteString(color)
		sb.WriteString("  │")
		sb.WriteString(block)
		sb.WriteString("│")
		sb.WriteString("\033[0m")
		if i > 1 {
			sb.WriteString("\n")
		}
	}

	// Bottom of thermometer with base
	sb.WriteString(color)
	sb.WriteString("  └─┘")
	sb.WriteString("\033[0m")

	return sb.String()
}

// getBlockForLevel returns the Unicode block character for a given level.
// fillLevel is the calculated fill level, and targetLevel is the segment being rendered.
func (r *RateLimitPanel) getBlockForLevel(targetLevel, fillLevel int, color string) string {
	if targetLevel <= fillLevel {
		return "█"
	}
	// Partial fill for the segment that has partial fill
	remainingRatio := float64(fillLevel) / float64(10)
	if targetLevel == fillLevel+1 && remainingRatio > 0 {
		partial := int((remainingRatio - float64(targetLevel-1)) * 4)
		switch partial {
		case 1:
			return "\033[33m░\033[0m" + color
		case 2:
			return "\033[33m▒\033[0m" + color
		case 3:
			return "\033[33m▓\033[0m" + color
		}
	}
	return "\033[90m░\033[0m"
}

// getColor returns the ANSI color code based on remaining budget.
func (r *RateLimitPanel) getColor() string {
	if r.remaining > 2000 {
		return "\033[32m" // Green
	}
	if r.remaining >= 500 {
		return "\033[33m" // Yellow/Amber
	}
	return "\033[31m" // Red
}

// renderReadout displays the current value as "remaining / total".
func (r *RateLimitPanel) renderReadout() string {
	remainingStr := fmt.Sprintf("%4d", r.remaining)
	totalStr := fmt.Sprintf("%4d", r.total)

	color := r.getColor()
	return fmt.Sprintf("%s  %s / %s\033[0m", color, remainingStr, totalStr)
}

// renderCountdown shows the time remaining until the rate limit resets.
func (r *RateLimitPanel) renderCountdown() string {
	if r.resetTime.IsZero() {
		return "  Resets in: --:--"
	}

	now := time.Now()
	remaining := r.resetTime.Sub(now)
	if remaining < 0 {
		return "  Resets in: 0m 0s"
	}

	minutes := int(remaining.Minutes())
	seconds := int(remaining.Seconds()) % 60

	return fmt.Sprintf("  Resets in: %dm %ds", minutes, seconds)
}
