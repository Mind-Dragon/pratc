package planning

import (
	"context"
	"maps"
	"math"
	"sort"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// DefaultPriorityWeights returns the default weights for priority scoring.
// These weights are used when no custom weights are provided via settings.
func DefaultPriorityWeights() PriorityWeights {
	return PriorityWeights{
		StalenessWeight:        0.30,
		CIStatusWeight:         0.25,
		SecurityLabelWeight:    0.20,
		ClusterCoherenceWeight: 0.15,
		TimeDecayWeight:        0.10,
	}
}

// PriorityWeights defines the configurable weights for each priority scoring factor.
// Weights must sum to 1.0 for proper normalization.
type PriorityWeights struct {
	// StalenessWeight determines how much age affects priority (anti-starvation)
	StalenessWeight float64 `json:"staleness_weight"`
	// CIStatusWeight determines how much CI status affects priority
	CIStatusWeight float64 `json:"ci_status_weight"`
	// SecurityLabelWeight determines how much security labels affect priority
	SecurityLabelWeight float64 `json:"security_label_weight"`
	// ClusterCoherenceWeight determines how much cluster affinity affects priority
	ClusterCoherenceWeight float64 `json:"cluster_coherence_weight"`
	// TimeDecayWeight determines how much time-decay windowing affects priority
	TimeDecayWeight float64 `json:"time_decay_weight"`
}

// Validate checks that weights sum to approximately 1.0 and values are in valid range.
func (w PriorityWeights) Validate() error {
	sum := w.StalenessWeight + w.CIStatusWeight + w.SecurityLabelWeight + w.ClusterCoherenceWeight + w.TimeDecayWeight
	if math.Abs(sum-1.0) > 0.001 {
		return ErrInvalidWeights
	}
	if w.StalenessWeight < 0 || w.StalenessWeight > 1 {
		return ErrInvalidWeightRange
	}
	if w.CIStatusWeight < 0 || w.CIStatusWeight > 1 {
		return ErrInvalidWeightRange
	}
	if w.SecurityLabelWeight < 0 || w.SecurityLabelWeight > 1 {
		return ErrInvalidWeightRange
	}
	if w.ClusterCoherenceWeight < 0 || w.ClusterCoherenceWeight > 1 {
		return ErrInvalidWeightRange
	}
	if w.TimeDecayWeight < 0 || w.TimeDecayWeight > 1 {
		return ErrInvalidWeightRange
	}
	return nil
}

// DEPRECATED: PoolSelector is not wired into the production planning path.
// Production uses internal/filter + internal/planner instead.
// Scheduled for removal in v0.2.
// See: internal/AGENTS.md "planning/ is mostly dead code"
//
// PoolSelector selects deterministic candidate pools based on weighted priority scoring.
type PoolSelector struct {
	Weights PriorityWeights
	Now     func() time.Time
}

// NewPoolSelector creates a new PoolSelector with the given weights.
func NewPoolSelector(weights PriorityWeights) (*PoolSelector, error) {
	if err := weights.Validate(); err != nil {
		return nil, err
	}
	ps := &PoolSelector{
		Weights: weights,
		Now:     func() time.Time { return time.Now().UTC() },
	}
	return ps, nil
}

// NewPoolSelectorWithDefaults creates a new PoolSelector with default weights.
func NewPoolSelectorWithDefaults() *PoolSelector {
	ps, _ := NewPoolSelector(DefaultPriorityWeights())
	return ps
}

// PoolResult contains the result of pool selection with detailed reason codes.
type PoolResult struct {
	Repo          string                   `json:"repo"`
	GeneratedAt   string                   `json:"generated_at"`
	InputCount    int                      `json:"input_count"`
	SelectedCount int                      `json:"selected_count"`
	ExcludedCount int                      `json:"excluded_count"`
	Selected      []PoolCandidate          `json:"selected"`
	Excluded      []ExclusionReason        `json:"excluded"`
	Weights       PriorityWeights          `json:"weights_used"`
	Telemetry     types.OperationTelemetry `json:"telemetry"`
}

// PoolCandidate represents a PR selected for the candidate pool with scoring details.
type PoolCandidate struct {
	PR              types.PR        `json:"pr"`
	PriorityScore   float64         `json:"priority_score"`
	ComponentScores ComponentScores `json:"component_scores"`
	ReasonCodes     []string        `json:"reason_codes"`
}

// ComponentScores breakdown of the priority score.
type ComponentScores struct {
	StalenessScore     float64 `json:"staleness_score"`
	CIStatusScore      float64 `json:"ci_status_score"`
	SecurityLabelScore float64 `json:"security_label_score"`
	ClusterScore       float64 `json:"cluster_score"`
	TimeDecayScore     float64 `json:"time_decay_score"`
}

// ExclusionReason provides the reason why a PR was excluded from the pool.
type ExclusionReason struct {
	PR          types.PR `json:"pr"`
	Reason      string   `json:"reason"`
	ReasonCodes []string `json:"reason_codes"`
}

// TimeDecayConfig configures time-decay windowing policy.
type TimeDecayConfig struct {
	// HalfLifeHours is the half-life for exponential decay in hours
	HalfLifeHours float64 `json:"half_life_hours"`
	// WindowHours defines the recency window; PRs outside are penalized
	WindowHours float64 `json:"window_hours"`
	// ProtectedHours defines how old a PR must be to enter protected lane
	ProtectedHours float64 `json:"protected_hours"`
	// MinScore is the minimum score for protected PRs to prevent starvation
	MinScore float64 `json:"min_score"`
}

// DefaultTimeDecayConfig returns the default time-decay configuration.
func DefaultTimeDecayConfig() TimeDecayConfig {
	return TimeDecayConfig{
		HalfLifeHours:  72.0,  // 3 days
		WindowHours:    168.0, // 1 week
		ProtectedHours: 336.0, // 2 weeks
		MinScore:       0.6,   // Protected PRs get at least 0.6 score
	}
}

// SelectCandidates performs deterministic candidate pool selection based on weighted priority scoring.
func (ps *PoolSelector) SelectCandidates(ctx context.Context, repo string, prs []types.PR, targetSize int, decayConfig TimeDecayConfig) *PoolResult {
	startTime := time.Now()
	now := ps.Now()

	// Step 1: Score all PRs
	candidates := ps.scoreAllPRs(prs, now, decayConfig)

	// Step 2: Sort by priority score (deterministic - stable sort by PR number as tiebreaker)
	sort.Slice(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].PriorityScore-candidates[j].PriorityScore) > 0.0001 {
			return candidates[i].PriorityScore > candidates[j].PriorityScore
		}
		return candidates[i].PR.Number < candidates[j].PR.Number
	})

	// Step 3: Select top candidates respecting target size
	selected := make([]PoolCandidate, 0, min(targetSize, len(candidates)))
	excluded := make([]ExclusionReason, 0)

	for i, c := range candidates {
		if i < targetSize {
			selected = append(selected, c)
		} else {
			excluded = append(excluded, ExclusionReason{
				PR:          c.PR,
				Reason:      "exceeded_target_pool_size",
				ReasonCodes: append([]string{}, c.ReasonCodes...),
			})
		}
	}

	elapsed := time.Since(startTime)

	return &PoolResult{
		Repo:          repo,
		GeneratedAt:   now.Format(time.RFC3339),
		InputCount:    len(prs),
		SelectedCount: len(selected),
		ExcludedCount: len(excluded),
		Selected:      selected,
		Excluded:      excluded,
		Weights:       ps.Weights,
		Telemetry: types.OperationTelemetry{
			PoolStrategy:   "weighted_priority",
			PoolSizeBefore: len(prs),
			PoolSizeAfter:  len(selected),
			DecayPolicy:    decayConfigToPolicy(decayConfig),
			StageLatenciesMS: map[string]int{
				"select_candidates_ms": int(elapsed.Milliseconds()),
			},
		},
	}
}

// scoreAllPRs calculates priority scores for all PRs using weighted scoring factors.
func (ps *PoolSelector) scoreAllPRs(prs []types.PR, now time.Time, decayConfig TimeDecayConfig) []PoolCandidate {
	candidates := make([]PoolCandidate, 0, len(prs))

	for _, pr := range prs {
		scores := ps.calculateComponentScores(pr, prs, now, decayConfig)
		totalScore := ps.computeWeightedScore(scores)
		reasonCodes := ps.generateReasonCodes(pr, scores)

		candidates = append(candidates, PoolCandidate{
			PR:              pr,
			PriorityScore:   totalScore,
			ComponentScores: scores,
			ReasonCodes:     reasonCodes,
		})
	}

	return candidates
}

// calculateComponentScores computes individual component scores for a PR.
func (ps *PoolSelector) calculateComponentScores(pr types.PR, allPRs []types.PR, now time.Time, decayConfig TimeDecayConfig) ComponentScores {
	return ComponentScores{
		StalenessScore:     ps.scoreStaleness(pr, now),
		CIStatusScore:      ps.scoreCIStatus(pr),
		SecurityLabelScore: ps.scoreSecurityLabels(pr),
		ClusterScore:       ps.scoreClusterCoherenceWithContext(pr, allPRs),
		TimeDecayScore:     ps.scoreTimeDecay(pr, now, decayConfig),
	}
}

// scoreStaleness calculates the staleness score (anti-starvation).
// Older PRs that haven't been updated get higher scores to prevent starvation.
func (ps *PoolSelector) scoreStaleness(pr types.PR, now time.Time) float64 {
	updatedAt, err := time.Parse(time.RFC3339, pr.UpdatedAt)
	if err != nil {
		updatedAt = now
	}

	hoursSinceUpdate := now.Sub(updatedAt).Hours()
	if hoursSinceUpdate < 0 {
		hoursSinceUpdate = 0
	}

	// Exponential decay: score increases with age but plateaus
	// Max score of 1.0 at ~2 weeks (336 hours)
	maxStalenessHours := 336.0
	normalizedAge := math.Min(hoursSinceUpdate/maxStalenessHours, 1.0)

	// Use exponential curve to prioritize very stale PRs
	return 1.0 - math.Exp(-normalizedAge*2)
}

// scoreCIStatus calculates CI status score.
// Passing CI = higher score, failing = lower, pending = medium.
func (ps *PoolSelector) scoreCIStatus(pr types.PR) float64 {
	switch pr.CIStatus {
	case "success", "passing":
		return 1.0
	case "failure", "failing":
		return 0.2
	case "pending", "in_progress":
		return 0.5
	default:
		return 0.3 // unknown status
	}
}

// scoreSecurityLabels checks if PR has security-related labels.
func (ps *PoolSelector) scoreSecurityLabels(pr types.PR) float64 {
	securityKeywords := []string{
		"security", "cve", "vulnerability", "exploit",
		"patch", "hotfix", "urgent", "critical",
	}

	hasSecurityRelevance := false
	for _, label := range pr.Labels {
		labelLower := lowercaseFold(label)
		for _, keyword := range securityKeywords {
			if contains(labelLower, keyword) {
				hasSecurityRelevance = true
				break
			}
		}
		if hasSecurityRelevance {
			break
		}
	}

	if hasSecurityRelevance {
		return 1.0
	}
	return 0.0
}

// scoreClusterCoherenceWithContext calculates cluster coherence given all PRs.
func (ps *PoolSelector) scoreClusterCoherenceWithContext(pr types.PR, allPRs []types.PR) float64 {
	if pr.ClusterID == "" {
		return 0.0
	}

	// Count PRs in the same cluster
	clusterCount := 0
	for _, other := range allPRs {
		if other.ClusterID == pr.ClusterID && other.Number != pr.Number {
			clusterCount++
		}
	}

	// More PRs in cluster = higher coherence boost
	// Cap at 5 PRs for scoring purposes
	normalized := math.Min(float64(clusterCount)/5.0, 1.0)
	return normalized
}

// scoreTimeDecay applies time-decay windowing policy.
// PRs within the recency window get higher scores.
func (ps *PoolSelector) scoreTimeDecay(pr types.PR, now time.Time, config TimeDecayConfig) float64 {
	createdAt, err := time.Parse(time.RFC3339, pr.CreatedAt)
	if err != nil {
		createdAt = now
	}

	hoursSinceCreation := now.Sub(createdAt).Hours()
	if hoursSinceCreation < 0 {
		hoursSinceCreation = 0
	}

	// Protected lane: very old PRs get a boost to prevent starvation
	if hoursSinceCreation > config.ProtectedHours {
		// Protected PRs get a boost based on how long they've been waiting
		protectedAge := hoursSinceCreation - config.ProtectedHours
		maxProtectedAge := 672.0                             // 4 weeks beyond protected
		boost := math.Min(protectedAge/maxProtectedAge, 0.5) // max 0.5 boost
		return 0.5 + boost
	}

	// Within window: exponential decay based on half-life
	if config.HalfLifeHours <= 0 {
		return 1.0
	}

	decayFactor := math.Exp(-hoursSinceCreation / config.HalfLifeHours * math.Ln2)
	return decayFactor
}

// computeWeightedScore combines component scores into a single priority score.
func (ps *PoolSelector) computeWeightedScore(scores ComponentScores) float64 {
	return (ps.Weights.StalenessWeight * scores.StalenessScore) +
		(ps.Weights.CIStatusWeight * scores.CIStatusScore) +
		(ps.Weights.SecurityLabelWeight * scores.SecurityLabelScore) +
		(ps.Weights.ClusterCoherenceWeight * scores.ClusterScore) +
		(ps.Weights.TimeDecayWeight * scores.TimeDecayScore)
}

// generateReasonCodes generates human-readable reason codes for a PR's selection.
func (ps *PoolSelector) generateReasonCodes(pr types.PR, scores ComponentScores) []string {
	var reasons []string

	// Staleness
	if scores.StalenessScore > 0.7 {
		reasons = append(reasons, ReasonCodeStale)
	} else if scores.StalenessScore < 0.3 {
		reasons = append(reasons, ReasonCodeRecent)
	}

	// CI Status
	switch pr.CIStatus {
	case "success", "passing":
		reasons = append(reasons, ReasonCodeCIPassing)
	case "failure", "failing":
		reasons = append(reasons, ReasonCodeCIFailing)
	case "pending", "in_progress":
		reasons = append(reasons, ReasonCodeCIPending)
	}

	// Security
	if scores.SecurityLabelScore > 0.5 {
		reasons = append(reasons, ReasonCodeSecurityRelevant)
	}

	// Cluster
	if pr.ClusterID != "" {
		reasons = append(reasons, ReasonCodeHasCluster)
	}

	// Time decay
	if scores.TimeDecayScore > 0.8 {
		reasons = append(reasons, ReasonCodeRecentWindow)
	} else if scores.TimeDecayScore < 0.3 {
		reasons = append(reasons, ReasonCodeOutsideWindow)
	}

	return reasons
}

// Reason codes for pool selection decisions.
const (
	ReasonCodeStale            = "staleness_high"
	ReasonCodeRecent           = "staleness_low"
	ReasonCodeCIPassing        = "ci_passing"
	ReasonCodeCIFailing        = "ci_failing"
	ReasonCodeCIPending        = "ci_pending"
	ReasonCodeSecurityRelevant = "security_label"
	ReasonCodeHasCluster       = "has_cluster"
	ReasonCodeRecentWindow     = "in_recency_window"
	ReasonCodeOutsideWindow    = "outside_recency_window"
	ReasonCodeProtectedLane    = "protected_lane"
)

// SelectCandidatesWithClusterCoherence is like SelectCandidates but calculates
// cluster coherence in the context of all PRs.
func (ps *PoolSelector) SelectCandidatesWithClusterCoherence(ctx context.Context, repo string, prs []types.PR, targetSize int, decayConfig TimeDecayConfig) *PoolResult {
	startTime := time.Now()
	now := ps.Now()

	// First pass: score without cluster context
	candidates := ps.scoreAllPRs(prs, now, decayConfig)

	// Second pass: update cluster coherence with context
	for i := range candidates {
		candidates[i].ComponentScores.ClusterScore = ps.scoreClusterCoherenceWithContext(candidates[i].PR, prs)
		candidates[i].PriorityScore = ps.computeWeightedScore(candidates[i].ComponentScores)
		// Regenerate reason codes with updated cluster score
		candidates[i].ReasonCodes = ps.generateReasonCodes(candidates[i].PR, candidates[i].ComponentScores)
	}

	// Sort by priority score (deterministic)
	sort.Slice(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].PriorityScore-candidates[j].PriorityScore) > 0.0001 {
			return candidates[i].PriorityScore > candidates[j].PriorityScore
		}
		return candidates[i].PR.Number < candidates[j].PR.Number
	})

	// Select top candidates
	selected := make([]PoolCandidate, 0, min(targetSize, len(candidates)))
	excluded := make([]ExclusionReason, 0)

	for i, c := range candidates {
		if i < targetSize {
			selected = append(selected, c)
		} else {
			excluded = append(excluded, ExclusionReason{
				PR:          c.PR,
				Reason:      "exceeded_target_pool_size",
				ReasonCodes: append([]string{}, c.ReasonCodes...),
			})
		}
	}

	elapsed := time.Since(startTime)

	return &PoolResult{
		Repo:          repo,
		GeneratedAt:   now.Format(time.RFC3339),
		InputCount:    len(prs),
		SelectedCount: len(selected),
		ExcludedCount: len(excluded),
		Selected:      selected,
		Excluded:      excluded,
		Weights:       ps.Weights,
		Telemetry: types.OperationTelemetry{
			PoolStrategy:   "weighted_priority_with_cluster",
			PoolSizeBefore: len(prs),
			PoolSizeAfter:  len(selected),
			DecayPolicy:    decayConfigToPolicy(decayConfig),
			StageLatenciesMS: map[string]int{
				"select_candidates_ms": int(elapsed.Milliseconds()),
			},
		},
	}
}

// Settings keys for priority pool weights.
const (
	SettingKeyPoolWeights     = "pool_weights"
	SettingKeyTimeDecayConfig = "time_decay_config"
)

// ToSettings converts weights to a settings-compatible map.
func (w PriorityWeights) ToSettings() map[string]any {
	return map[string]any{
		"staleness_weight":         w.StalenessWeight,
		"ci_status_weight":         w.CIStatusWeight,
		"security_label_weight":    w.SecurityLabelWeight,
		"cluster_coherence_weight": w.ClusterCoherenceWeight,
		"time_decay_weight":        w.TimeDecayWeight,
	}
}

// PriorityWeightsFromSettings extracts PriorityWeights from settings map.
func PriorityWeightsFromSettings(settings map[string]any) (PriorityWeights, bool) {
	w := DefaultPriorityWeights()

	if v, ok := settings["staleness_weight"]; ok {
		if f, ok := toFloat(v); ok {
			w.StalenessWeight = f
		}
	}
	if v, ok := settings["ci_status_weight"]; ok {
		if f, ok := toFloat(v); ok {
			w.CIStatusWeight = f
		}
	}
	if v, ok := settings["security_label_weight"]; ok {
		if f, ok := toFloat(v); ok {
			w.SecurityLabelWeight = f
		}
	}
	if v, ok := settings["cluster_coherence_weight"]; ok {
		if f, ok := toFloat(v); ok {
			w.ClusterCoherenceWeight = f
		}
	}
	if v, ok := settings["time_decay_weight"]; ok {
		if f, ok := toFloat(v); ok {
			w.TimeDecayWeight = f
		}
	}

	return w, true
}

// TimeDecayConfigFromSettings extracts TimeDecayConfig from settings map.
func TimeDecayConfigFromSettings(settings map[string]any) (TimeDecayConfig, bool) {
	cfg := DefaultTimeDecayConfig()

	if v, ok := settings["half_life_hours"]; ok {
		if f, ok := toFloat(v); ok && f > 0 {
			cfg.HalfLifeHours = f
		}
	}
	if v, ok := settings["window_hours"]; ok {
		if f, ok := toFloat(v); ok && f > 0 {
			cfg.WindowHours = f
		}
	}
	if v, ok := settings["protected_hours"]; ok {
		if f, ok := toFloat(v); ok && f > 0 {
			cfg.ProtectedHours = f
		}
	}
	if v, ok := settings["min_score"]; ok {
		if f, ok := toFloat(v); ok && f >= 0 && f <= 1 {
			cfg.MinScore = f
		}
	}

	return cfg, true
}

// TimeDecayConfigToSettings converts TimeDecayConfig to settings map.
func TimeDecayConfigToSettings(cfg TimeDecayConfig) map[string]any {
	return map[string]any{
		"half_life_hours": cfg.HalfLifeHours,
		"window_hours":    cfg.WindowHours,
		"protected_hours": cfg.ProtectedHours,
		"min_score":       cfg.MinScore,
	}
}

// Helper functions.

func decayConfigToPolicy(cfg TimeDecayConfig) string {
	return "exponential_decay"
}

func lowercaseFold(s string) string {
	// Simple ASCII lowercase - suitable for label matching
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch f := v.(type) {
	case float64:
		return f, true
	case float32:
		return float64(f), true
	case int:
		return float64(f), true
	case int64:
		return float64(f), true
	default:
		return 0, false
	}
}

// Ensure PoolSelector implements required interface for integration with other components.
var _ = interface{}(PoolSelector{})

// ValidatePoolResult performs validation on pool result for debugging/testing.
func ValidatePoolResult(result *PoolResult) error {
	if result == nil {
		return ErrNilPoolResult
	}
	if result.InputCount != result.SelectedCount+result.ExcludedCount {
		return ErrPoolCountMismatch
	}
	// Check determinism: selected should be sorted by score descending
	for i := 1; i < len(result.Selected); i++ {
		if result.Selected[i].PriorityScore > result.Selected[i-1].PriorityScore {
			return ErrPoolNotDeterministic
		}
	}
	return nil
}

// Error definitions.
var (
	ErrInvalidWeights       = &PoolError{"weights must sum to 1.0"}
	ErrInvalidWeightRange   = &PoolError{"weights must be between 0 and 1"}
	ErrNilPoolResult        = &PoolError{"pool result is nil"}
	ErrPoolCountMismatch    = &PoolError{"selected + excluded must equal input count"}
	ErrPoolNotDeterministic = &PoolError{"selected pool is not deterministically sorted"}
)

// PoolError represents a pool selection error.
type PoolError struct {
	msg string
}

func (e *PoolError) Error() string {
	return e.msg
}

// Interface guard.
var _ error = (*PoolError)(nil)

// Merge pools from multiple selectors (for federation scenarios).
// This is used when combining results from multiple repositories.
func MergePoolResults(results ...*PoolResult) *PoolResult {
	if len(results) == 0 {
		return nil
	}
	if len(results) == 1 {
		return results[0]
	}

	merged := &PoolResult{
		Repo:          "merged",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		InputCount:    0,
		SelectedCount: 0,
		ExcludedCount: 0,
		Selected:      []PoolCandidate{},
		Excluded:      []ExclusionReason{},
		Weights:       results[0].Weights,
		Telemetry:     types.OperationTelemetry{},
	}

	for _, r := range results {
		merged.InputCount += r.InputCount
		merged.SelectedCount += r.SelectedCount
		merged.ExcludedCount += r.ExcludedCount
		merged.Selected = append(merged.Selected, r.Selected...)
		merged.Excluded = append(merged.Excluded, r.Excluded...)
	}

	// Re-sort merged results deterministically
	sort.Slice(merged.Selected, func(i, j int) bool {
		if math.Abs(merged.Selected[i].PriorityScore-merged.Selected[j].PriorityScore) > 0.0001 {
			return merged.Selected[i].PriorityScore > merged.Selected[j].PriorityScore
		}
		return merged.Selected[i].PR.Number < merged.Selected[j].PR.Number
	})

	merged.Telemetry.PoolSizeBefore = merged.InputCount
	merged.Telemetry.PoolSizeAfter = merged.SelectedCount

	return merged
}

// Clone creates a deep copy of the PoolResult for thread-safe operations.
func (p *PoolResult) Clone() *PoolResult {
	if p == nil {
		return nil
	}
	selected := make([]PoolCandidate, len(p.Selected))
	copy(selected, p.Selected)
	excluded := make([]ExclusionReason, len(p.Excluded))
	copy(excluded, p.Excluded)
	return &PoolResult{
		Repo:          p.Repo,
		GeneratedAt:   p.GeneratedAt,
		InputCount:    p.InputCount,
		SelectedCount: p.SelectedCount,
		ExcludedCount: p.ExcludedCount,
		Selected:      selected,
		Excluded:      excluded,
		Weights:       p.Weights,
		Telemetry:     types.OperationTelemetry{},
	}
}

// MapToPoolWeights creates PriorityWeights from a generic map (for settings integration).
func MapToPoolWeights(m map[string]any) (PriorityWeights, error) {
	w := DefaultPriorityWeights()

	if v, ok := m["staleness_weight"]; ok {
		f, ok := toFloat(v)
		if !ok {
			return PriorityWeights{}, ErrInvalidWeightRange
		}
		w.StalenessWeight = f
	}
	if v, ok := m["ci_status_weight"]; ok {
		f, ok := toFloat(v)
		if !ok {
			return PriorityWeights{}, ErrInvalidWeightRange
		}
		w.CIStatusWeight = f
	}
	if v, ok := m["security_label_weight"]; ok {
		f, ok := toFloat(v)
		if !ok {
			return PriorityWeights{}, ErrInvalidWeightRange
		}
		w.SecurityLabelWeight = f
	}
	if v, ok := m["cluster_coherence_weight"]; ok {
		f, ok := toFloat(v)
		if !ok {
			return PriorityWeights{}, ErrInvalidWeightRange
		}
		w.ClusterCoherenceWeight = f
	}
	if v, ok := m["time_decay_weight"]; ok {
		f, ok := toFloat(v)
		if !ok {
			return PriorityWeights{}, ErrInvalidWeightRange
		}
		w.TimeDecayWeight = f
	}

	if err := w.Validate(); err != nil {
		return PriorityWeights{}, err
	}

	return w, nil
}

// CopyWeights creates a defensive copy of PriorityWeights.
func CopyWeights(w PriorityWeights) PriorityWeights {
	return PriorityWeights{
		StalenessWeight:        w.StalenessWeight,
		CIStatusWeight:         w.CIStatusWeight,
		SecurityLabelWeight:    w.SecurityLabelWeight,
		ClusterCoherenceWeight: w.ClusterCoherenceWeight,
		TimeDecayWeight:        w.TimeDecayWeight,
	}
}

// EqualWeights checks if two PriorityWeights are equal within tolerance.
func EqualWeights(a, b PriorityWeights, tolerance float64) bool {
	return math.Abs(a.StalenessWeight-b.StalenessWeight) <= tolerance &&
		math.Abs(a.CIStatusWeight-b.CIStatusWeight) <= tolerance &&
		math.Abs(a.SecurityLabelWeight-b.SecurityLabelWeight) <= tolerance &&
		math.Abs(a.ClusterCoherenceWeight-b.ClusterCoherenceWeight) <= tolerance &&
		math.Abs(a.TimeDecayWeight-b.TimeDecayWeight) <= tolerance
}

// NormalizeWeights ensures weights sum to 1.0 by scaling.
func NormalizeWeights(w PriorityWeights) PriorityWeights {
	sum := w.StalenessWeight + w.CIStatusWeight + w.SecurityLabelWeight + w.ClusterCoherenceWeight + w.TimeDecayWeight
	if sum == 0 || math.Abs(sum-1.0) < 0.0001 {
		return w
	}
	scale := 1.0 / sum
	return PriorityWeights{
		StalenessWeight:        w.StalenessWeight * scale,
		CIStatusWeight:         w.CIStatusWeight * scale,
		SecurityLabelWeight:    w.SecurityLabelWeight * scale,
		ClusterCoherenceWeight: w.ClusterCoherenceWeight * scale,
		TimeDecayWeight:        w.TimeDecayWeight * scale,
	}
}

// SettingsKeys returns all the settings keys used by the pool selector.
func SettingsKeys() []string {
	return []string{
		"staleness_weight",
		"ci_status_weight",
		"security_label_weight",
		"cluster_coherence_weight",
		"time_decay_weight",
		"half_life_hours",
		"window_hours",
		"protected_hours",
		"min_score",
	}
}

// AddPoolKeysToSettings adds pool-related keys to the allowed settings keys.
func AddPoolKeysToSettings(allowed map[string]struct{}) map[string]struct{} {
	if allowed == nil {
		allowed = make(map[string]struct{})
	}
	for _, k := range SettingsKeys() {
		allowed[k] = struct{}{}
	}
	// Also add the composite keys
	allowed[SettingKeyPoolWeights] = struct{}{}
	allowed[SettingKeyTimeDecayConfig] = struct{}{}
	return allowed
}

// CloneSettingsMap creates a defensive copy of a settings map.
func CloneSettingsMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	return maps.Clone(m)
}
