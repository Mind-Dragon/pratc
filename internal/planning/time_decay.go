package planning

import (
	"math"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// TimeDecayWindow wraps a PR list and applies time-decay scoring with protected lane.
// It provides exponential decay weighting and anti-starvation for high-risk old PRs.
type TimeDecayWindow struct {
	config    TimeDecayConfig
	now       time.Time
	prs       []types.PR
	scores    map[int]float64 // PR.Number -> decay score
	protected map[int]bool    // PR.Number -> is protected
}

// TimeDecayStats provides telemetry for time-decay windowing.
type TimeDecayStats struct {
	// EligibleCount is the number of PRs within the recency window
	EligibleCount int `json:"eligible_count"`
	// ProtectedCount is the number of PRs in the protected lane (old but critical)
	ProtectedCount int `json:"protected_count"`
	// DecayMin is the minimum decay score applied
	DecayMin float64 `json:"decay_min"`
	// DecayMax is the maximum decay score applied
	DecayMax float64 `json:"decay_max"`
	// DecayAvg is the average decay score across all PRs
	DecayAvg float64 `json:"decay_avg"`
}

// NewTimeDecayWindow creates a new time-decay window with the given configuration.
func NewTimeDecayWindow(prs []types.PR, config TimeDecayConfig, now time.Time) *TimeDecayWindow {
	tdw := &TimeDecayWindow{
		config:    config,
		now:       now,
		prs:       prs,
		scores:    make(map[int]float64, len(prs)),
		protected: make(map[int]bool, len(prs)),
	}
	tdw.computeAllScores()
	return tdw
}

// computeAllScores calculates decay scores for all PRs.
func (tdw *TimeDecayWindow) computeAllScores() {
	for _, pr := range tdw.prs {
		score := tdw.ScorePR(pr)
		tdw.scores[pr.Number] = score
		tdw.protected[pr.Number] = tdw.isProtectedPR(pr)
	}
}

// ScorePR returns the composite time-decay score for a PR.
// Formula: baseScore * e^(-decay * ageHours) where decay = ln(2) / halfLifeHours
// Protected PRs (security/urgent) get a minimum score to prevent starvation.
func (tdw *TimeDecayWindow) ScorePR(pr types.PR) float64 {
	createdAt, err := time.Parse(time.RFC3339, pr.CreatedAt)
	if err != nil {
		// If parsing fails, treat as brand new PR
		createdAt = tdw.now
	}

	hoursSinceCreation := tdw.now.Sub(createdAt).Hours()
	if hoursSinceCreation < 0 {
		hoursSinceCreation = 0
	}

	// Check if PR qualifies for protected lane
	if tdw.isProtectedPR(pr) && hoursSinceCreation > tdw.config.ProtectedHours {
		// Protected lane: old critical PRs get minimum score to prevent starvation
		return tdw.config.MinScore
	}

	// Check if PR is outside the recency window
	if hoursSinceCreation > tdw.config.WindowHours {
		// Outside window but not protected: still apply decay, don't zero out
		// This ensures NO PR is permanently excluded
		if tdw.config.HalfLifeHours <= 0 {
			return 1.0
		}
		decayFactor := math.Exp(-math.Ln2 * hoursSinceCreation / tdw.config.HalfLifeHours)
		return decayFactor
	}

	// Within window: apply exponential decay
	if tdw.config.HalfLifeHours <= 0 {
		return 1.0
	}

	// Standard exponential decay formula: score = baseScore * e^(-ln(2) * ageHours / halfLifeHours)
	// This gives us true half-life behavior
	decayFactor := math.Exp(-math.Ln2 * hoursSinceCreation / tdw.config.HalfLifeHours)
	return decayFactor
}

// isProtectedPR checks if a PR has labels indicating it should be protected from starvation.
func (tdw *TimeDecayWindow) isProtectedPR(pr types.PR) bool {
	protectedKeywords := []string{
		"security", "cve", "vulnerability", "exploit",
		"urgent", "critical", "hotfix", "security-fix",
	}

	for _, label := range pr.Labels {
		labelLower := strings.ToLower(label)
		for _, keyword := range protectedKeywords {
			if strings.Contains(labelLower, keyword) {
				return true
			}
		}
	}
	return false
}

// SelectProtected returns all PRs that qualify for the protected lane.
// These are high-risk old PRs (security/urgent labels) that need starvation prevention.
func (tdw *TimeDecayWindow) SelectProtected() []types.PR {
	protected := make([]types.PR, 0)
	for _, pr := range tdw.prs {
		if !tdw.protected[pr.Number] {
			continue
		}
		// Check if PR is old enough to qualify for protected lane
		createdAt, err := time.Parse(time.RFC3339, pr.CreatedAt)
		if err == nil {
			hoursSinceCreation := tdw.now.Sub(createdAt).Hours()
			if hoursSinceCreation > tdw.config.ProtectedHours && tdw.isProtectedPR(pr) {
				protected = append(protected, pr)
			}
		}
	}
	return protected
}

// GetWindowStats returns telemetry statistics about the time-decay window.
func (tdw *TimeDecayWindow) GetWindowStats() TimeDecayStats {
	stats := TimeDecayStats{
		EligibleCount:  0,
		ProtectedCount: 0,
		DecayMin:       math.MaxFloat64,
		DecayMax:       0,
		DecayAvg:       0,
	}

	if len(tdw.prs) == 0 {
		stats.DecayMin = 0
		return stats
	}

	totalScore := 0.0
	for _, pr := range tdw.prs {
		score := tdw.scores[pr.Number]
		totalScore += score

		if score < stats.DecayMin {
			stats.DecayMin = score
		}
		if score > stats.DecayMax {
			stats.DecayMax = score
		}

		// Count PRs within recency window
		createdAt, err := time.Parse(time.RFC3339, pr.CreatedAt)
		if err == nil {
			hoursSinceCreation := tdw.now.Sub(createdAt).Hours()
			if hoursSinceCreation <= tdw.config.WindowHours {
				stats.EligibleCount++
			}
		}

		// Count protected PRs
		if tdw.protected[pr.Number] {
			stats.ProtectedCount++
		}
	}

	stats.DecayAvg = totalScore / float64(len(tdw.prs))

	return stats
}

// GetScore returns the pre-computed decay score for a specific PR.
func (tdw *TimeDecayWindow) GetScore(prNumber int) (float64, bool) {
	score, exists := tdw.scores[prNumber]
	return score, exists
}

// IsProtected returns whether a PR is in the protected lane.
func (tdw *TimeDecayWindow) IsProtected(prNumber int) bool {
	return tdw.protected[prNumber]
}

// GetReasonCodes returns reason codes explaining the time-decay score for a PR.
func (tdw *TimeDecayWindow) GetReasonCodes(pr types.PR) []string {
	var reasons []string

	score, exists := tdw.scores[pr.Number]
	if !exists {
		return []string{ReasonCodeTimeDecayUnknown}
	}

	// Check protected lane
	if tdw.protected[pr.Number] {
		reasons = append(reasons, ReasonCodeProtectedLane)
	}

	// Check recency window
	createdAt, err := time.Parse(time.RFC3339, pr.CreatedAt)
	if err == nil {
		hoursSinceCreation := tdw.now.Sub(createdAt).Hours()
		if hoursSinceCreation <= tdw.config.WindowHours {
			reasons = append(reasons, ReasonCodeTimeDecayWindow)
		} else {
			reasons = append(reasons, ReasonCodeOutsideWindow)
		}
	}

	// Check score thresholds
	if score > 0.8 {
		reasons = append(reasons, ReasonCodeRecentWindow)
	} else if score < 0.3 {
		reasons = append(reasons, ReasonCodeBelowMinScore)
	}

	// Always indicate decay was applied
	reasons = append(reasons, ReasonCodeDecayApplied)

	return reasons
}

// TimeDecayKeysToSettings returns all settings keys related to time-decay configuration.
func TimeDecayKeysToSettings() map[string]struct{} {
	return map[string]struct{}{
		"half_life_hours":         {},
		"window_hours":            {},
		"protected_hours":         {},
		"min_score":               {},
		SettingKeyTimeDecayConfig: {},
	}
}

// Validate validates the time-decay configuration.
func (c TimeDecayConfig) Validate() error {
	if c.HalfLifeHours < 0 {
		return ErrInvalidHalfLife
	}
	if c.WindowHours <= 0 {
		return ErrInvalidWindowHours
	}
	if c.ProtectedHours <= 0 {
		return ErrInvalidProtectedHours
	}
	if c.MinScore < 0 || c.MinScore > 1 {
		return ErrInvalidMinScore
	}
	return nil
}

// TimeDecayConfigFromSettingsWithMinScore extracts TimeDecayConfig from settings map including MinScore.
func TimeDecayConfigFromSettingsWithMinScore(settings map[string]any) (TimeDecayConfig, bool) {
	return TimeDecayConfigFromSettings(settings)
}

// TimeDecayConfigToSettingsWithMinScore converts TimeDecayConfig to settings map including MinScore.
func TimeDecayConfigToSettingsWithMinScore(cfg TimeDecayConfig) map[string]any {
	return TimeDecayConfigToSettings(cfg)
}

// Reason codes for time-decay decisions.
const (
	ReasonCodeTimeDecayWindow  = "time_decay_window"
	ReasonCodeBelowMinScore    = "below_min_score"
	ReasonCodeDecayApplied     = "decay_applied"
	ReasonCodeTimeDecayUnknown = "time_decay_unknown"
)

// Error definitions for time-decay configuration.
var (
	ErrInvalidHalfLife       = &TimeDecayError{"half_life_hours must be >= 0"}
	ErrInvalidWindowHours    = &TimeDecayError{"window_hours must be > 0"}
	ErrInvalidProtectedHours = &TimeDecayError{"protected_hours must be > 0"}
	ErrInvalidMinScore       = &TimeDecayError{"min_score must be between 0 and 1"}
)

// TimeDecayError represents a time-decay configuration error.
type TimeDecayError struct {
	msg string
}

func (e *TimeDecayError) Error() string {
	return e.msg
}

var _ error = (*TimeDecayError)(nil)
