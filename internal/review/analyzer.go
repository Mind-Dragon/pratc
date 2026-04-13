// Package review provides the Analyzer interface and related types for the agentic PR review system.
//
// Analyzer Guarantees:
//
//	Advisory-only: Analyzers produce recommendations, not actions. They classify PRs by
//	category and priority tier but never trigger merges, approvals, or any automated
//	actions. All analyzer output is for human review and decision-making.
//
//	Read-only: Analyzers cannot modify PR state, labels, comments, or any GitHub state.
//	They only read PR metadata and produce analysis results. The orchestrator may
//	apply labels based on analyzer output, but analyzers themselves are strictly read-only.
//
//	Stateless: Analyzers must not maintain state between invocations. Each Analyze()
//	call is independent. Analyzers receive all necessary context via PRData and must
//	not rely on external state, caches, or persistent storage.
//
//	These guarantees align with v0.1 constraints: no auto-merge, no GitHub App/OAuth/webhooks,
//	and no automated actions. Analyzers are pure functions from PR data to recommendations.
//
// Timeout Policy:
//
//	Analyzers have configurable timeouts to prevent runaway analysis. The default
//	timeout is 30 seconds, with a minimum of 5 seconds and maximum of 120 seconds.
//	Use WithTimeout() to customize per-analyzer timeout.
//
//	DefaultAnalyzerTimeout = 30s  // Recommended for most analyzers
//	MinAnalyzerTimeout     = 5s   // Floor to ensure analyzers have time to work
//	MaxAnalyzerTimeout     = 120s // Ceiling to prevent resource exhaustion
package review

import (
	"context"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// Timeout policy constants for analyzer execution.
// These bounds ensure analyzers have sufficient time to complete while
// preventing runaway analysis from consuming excessive resources.
const (
	// DefaultAnalyzerTimeout is the recommended timeout for most analyzers.
	DefaultAnalyzerTimeout = 30 * time.Second

	// MinAnalyzerTimeout is the floor to ensure analyzers have time to work.
	MinAnalyzerTimeout = 5 * time.Second

	// MaxAnalyzerTimeout is the ceiling to prevent resource exhaustion.
	MaxAnalyzerTimeout = 120 * time.Second
)

// PRData contains all PR metadata needed for an analyzer to produce a review result.
type PRData struct {
	// PR is the pull request under analysis.
	PR types.PR

	// Repo is the full repository name (e.g., "owner/repo").
	Repo string

	// ClusterID is the cluster assignment for this PR, if any.
	ClusterID string

	// ClusterLabel is the human-readable cluster label.
	ClusterLabel string

	// RelatedPRs are PRs in the same cluster or with overlapping changes.
	RelatedPRs []types.PR

	// DuplicateGroups are duplicate groups this PR belongs to.
	DuplicateGroups []types.DuplicateGroup

	// ConflictPairs are conflict relationships involving this PR.
	ConflictPairs []types.ConflictPair

	// Staleness is the staleness report for this PR, if any.
	Staleness *types.StalenessReport

	// AnalyzedAt is when this PR data was assembled for analysis.
	AnalyzedAt time.Time

	// Files contains the changed files in this PR with patch data.
	Files []types.PRFile

	// DiffHunks are the parsed diff hunks from the PR, if available.
	DiffHunks []types.DiffHunk
}

// AnalyzerResult wraps the review result with metadata and error handling fields.
type AnalyzerResult struct {
	// Result is the outcome of the agentic PR review.
	Result types.ReviewResult

	// AnalyzerName is the name of the analyzer that produced this result.
	AnalyzerName string

	// AnalyzerVersion is the semantic version of the analyzer.
	AnalyzerVersion string

	// ProcessingTimeMs is how long the analysis took in milliseconds.
	ProcessingTimeMs int64

	// Error is the error that occurred during analysis, if any.
	Error error

	// IsPartial indicates whether the result is partial due to errors or truncation.
	IsPartial bool

	// SkippedReasons lists why certain analyzers were skipped.
	SkippedReasons []string

	// StartedAt is when the analysis began.
	StartedAt time.Time

	// CompletedAt is when the analysis completed.
	CompletedAt time.Time
}

// Analyzer is the interface for PR analyzers in the agentic review system.
// Analyzers inspect PR metadata and produce a review result classifying the PR
// by category and priority tier.
type Analyzer interface {
	// Analyze examines the PR data and returns a review result or an error.
	Analyze(ctx context.Context, prData PRData) (AnalyzerResult, error)

	// Metadata returns information about this analyzer for reporting purposes.
	Metadata() types.AnalyzerMetadata
}

// AnalyzerOption configures an Analyzer.
type AnalyzerOption func(*analyzerConfig)

type analyzerConfig struct {
	timeout time.Duration
}

// WithTimeout sets a maximum duration for analysis.
func WithTimeout(timeout time.Duration) AnalyzerOption {
	return func(cfg *analyzerConfig) {
		cfg.timeout = timeout
	}
}
