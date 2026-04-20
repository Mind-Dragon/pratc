// Package types provides shared constants for the prATC codebase.
//
// This file centralizes magic numbers and configuration defaults to improve
// code readability and maintainability. All constants are exported (capitalized)
// so they can be used by other packages.
package types

import "time"

// Thresholds for duplicate and overlap detection in PR analysis.
// These values determine when PRs are considered duplicates or overlaps.
const (
	// DuplicateThreshold is the minimum similarity score (0.0-1.0) above which
	// two PRs are considered duplicates of each other on fresh classification.
	DuplicateThreshold = 0.85

	// CachedDuplicateThreshold preserves truthful duplicates from older cache-backed
	// corpora whose corrected similarity scores now land at 0.80.
	CachedDuplicateThreshold = 0.80

	// OverlapThreshold is the minimum similarity score (0.0-1.0) above which
	// two PRs are considered to have overlapping changes.
	OverlapThreshold = 0.70
)

// SimilarityWeights define the relative importance of different PR attributes
// when computing similarity scores for duplicate/overlap detection.
const (
	// TitleWeight is the weight given to title similarity in the
	// overall similarity score calculation.
	TitleWeight = 0.6

	// FileWeight is the weight given to file overlap in the
	// overall similarity score calculation.
	FileWeight = 0.3

	// BodyWeight is the weight given to body/description similarity in the
	// overall similarity score calculation.
	BodyWeight = 0.1
)

// ReviewConfidenceCaps impose upper bounds on merge confidence for high-risk PRs.
// These caps ensure that large or complex changes require stronger evidence
// before being recommended for fast merge.
const (
	// HighRiskConfidenceCap is the maximum confidence score (0.0-1.0) allowed
	// for PRs classified as high-risk (5+ changed files or 500+ line changes).
	HighRiskConfidenceCap = 0.79
)

// PlanDefaults contains configuration defaults for the merge plan generation.
const (
	// DefaultTarget is the default number of PRs to select in a merge plan
	// when no target is explicitly specified.
	DefaultTarget = 20

	// DefaultCandidatePoolCap is a legacy candidate-pool cap constant retained
	// for compatibility. The active BuildCandidatePool pipeline does not enforce
	// it by default.
	DefaultCandidatePoolCap = 100

	// MaxTarget is the upper bound for plan targets, derived from P1.3
	// rate limiting work to prevent excessive API usage.
	MaxTarget = 1000

	// PlanDryRunDefault is the default value for dry-run mode in plan generation.
	// true means plans are simulated by default for safety.
	PlanDryRunDefault = true

	// DefaultDeepCandidateSubsetSize is the default number of PRs to consider
	// in deep precision mode for detailed analysis.
	DefaultDeepCandidateSubsetSize = 64
)

// TimeConstants define common time durations used throughout the codebase.
const (
	// DefaultPageSize is the default number of items to request per page
	// when paginating through API results (GitHub list operations, etc.).
	DefaultPageSize = 100

	// SyncProgressReportInterval is the interval at which sync progress
	// is reported during long-running sync operations.
	SyncProgressReportInterval = 10 * time.Second
)

// GitHubURLPrefix is the base URL prefix for GitHub repository URLs.
const GitHubURLPrefix = "https://github.com/"

// NOTE: PairwiseShardSize, DefaultPoolCap, AnalyzeSLOMS, ClusterSLOMS, GraphSLOMS,
// and PlanSLOMS are defined in models.go to avoid circular imports.
