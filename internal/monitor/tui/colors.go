package tui

// Dark Cockpit color palette - consistent with terminal monitor aesthetic.
// All colors are dark mode only.
const (
	// Background
	ColorBgDarkNavy = "\033[48;2;10;14;39m" // #0A0E27

	// Primary colors
	ColorCyan  = "\033[38;2;0;217;255m"  // #00D9FF - Active/Primary
	ColorGreen = "\033[38;2;0;255;157m"  // #00FF9D - Success
	ColorAmber = "\033[38;2;255;185;70m" // #FFB946 - Warning
	ColorRed   = "\033[38;2;255;77;77m"  // #FF4D4D - Critical/Error

	// Text colors
	ColorText      = "\033[38;2;224;230;241m" // #E0E6F1 - Primary text
	ColorTextMuted = "\033[38;2;139;146;160m" // #8B92A0 - Muted/secondary text (WCAG 4.5:1)

	// Reset
	ColorReset = "\033[0m"
)

// Status color mapping for sync jobs.
var StatusColors = map[string]string{
	"active":    ColorCyan,
	"paused":    ColorAmber,
	"failed":    ColorRed,
	"queued":    ColorTextMuted,
	"completed": ColorGreen,
}

// Log level color mapping.
var LogLevelColors = map[string]string{
	"error":   ColorRed,
	"err":     ColorRed,
	"warn":    ColorAmber,
	"warning": ColorAmber,
	"debug":   ColorCyan,
	"dbg":     ColorCyan,
	"info":    ColorText,
	"inf":     ColorText,
}

// GetStatusColor returns the ANSI color code for a given status.
// Falls back to muted text color for unknown statuses.
func GetStatusColor(status string) string {
	if color, ok := StatusColors[status]; ok {
		return color
	}
	return ColorTextMuted
}

// GetLogLevelColor returns the ANSI color code for a given log level.
// Falls back to primary text color for unknown levels.
func GetLogLevelColor(level string) string {
	if color, ok := LogLevelColors[level]; ok {
		return color
	}
	return ColorText
}

// GetRateLimitColor returns the ANSI color code based on remaining API budget.
// Green (>2000), Amber (500-2000), Red (<500).
func GetRateLimitColor(remaining int) string {
	if remaining > 2000 {
		return ColorGreen
	}
	if remaining >= 500 {
		return ColorAmber
	}
	return ColorRed
}

// GetTimelineBlockColor returns the ANSI color code for timeline activity blocks.
// Uses cyan variants based on intensity (0-7).
func GetTimelineBlockColor(level int) string {
	if level == 0 {
		return ColorCyan
	}
	if level >= 5 {
		// Bright cyan for high activity
		return "\033[96m"
	}
	return ColorCyan
}
