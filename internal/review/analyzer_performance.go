// Package review provides the Analyzer interface and analyzers for the agentic PR review system.
package review

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// PerformanceAnalyzer detects performance-related risks in PRs including:
// - Large diffs (additions + deletions > threshold)
// - Touched critical paths (core libs, APIs, configs)
// - Performance-sensitive file types (hot paths, cache layers, DB queries)
// - Diff size heuristics (files changed count, lines changed count)
type PerformanceAnalyzer struct{}

// NewPerformanceAnalyzer creates a new PerformanceAnalyzer that implements the Analyzer interface.
func NewPerformanceAnalyzer() Analyzer {
	return &PerformanceAnalyzer{}
}

// Metadata returns information about this analyzer for reporting purposes.
func (p *PerformanceAnalyzer) Metadata() types.AnalyzerMetadata {
	return types.AnalyzerMetadata{
		Name:       "performance",
		Version:    "0.1.0",
		Category:   "performance",
		Confidence: 0.80,
	}
}

// performanceFinding represents an individual performance finding.
type performanceFinding struct {
	finding    string
	confidence float64
	pattern    string
}

// Thresholds for performance detection.
const (
	// largeDiffLinesThreshold is the minimum total lines changed (additions + deletions)
	// to be considered a large diff.
	largeDiffLinesThreshold = 500

	// largeFileCountThreshold is the minimum number of files changed
	// to be considered a large change set.
	largeFileCountThreshold = 50

	// highAdditionsThreshold is the minimum number of additions
	// to flag as potentially problematic.
	highAdditionsThreshold = 300

	// highDeletionsThreshold is the minimum number of deletions
	// to flag as potentially problematic.
	highDeletionsThreshold = 300
)

// performanceSensitivePatterns maps pattern names to regex patterns for detection.
var (
	// criticalPathPatterns matches files in critical paths like core libs, APIs, configs.
	criticalPathPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^core/`),
		regexp.MustCompile(`(?i)^lib/`),
		regexp.MustCompile(`(?i)^libs/`),
		regexp.MustCompile(`(?i)^api/`),
		regexp.MustCompile(`(?i)^apis/`),
		regexp.MustCompile(`(?i)^v1/`),
		regexp.MustCompile(`(?i)^v2/`),
		regexp.MustCompile(`(?i)^internal/`),
		regexp.MustCompile(`(?i)^pkg/`),
		regexp.MustCompile(`(?i)^packages/`),
		regexp.MustCompile(`(?i)^src/`),
		regexp.MustCompile(`(?i)^source/`),
		regexp.MustCompile(`(?i)^app/`),
		regexp.MustCompile(`(?i)^services/`),
		regexp.MustCompile(`(?i)^service/`),
		regexp.MustCompile(`(?i)^server/`),
		regexp.MustCompile(`(?i)^config/`),
		regexp.MustCompile(`(?i)^configs/`),
		regexp.MustCompile(`(?i)^configuration/`),
		regexp.MustCompile(`(?i)^.github/`),
		regexp.MustCompile(`(?i)^.gitlab-ci`),
		regexp.MustCompile(`(?i)^.circleci/`),
		regexp.MustCompile(`(?i)^.aws/`),
		regexp.MustCompile(`(?i)^terraform/`),
		regexp.MustCompile(`(?i)^kubernetes/`),
		regexp.MustCompile(`(?i)^k8s/`),
	}

	// performanceSensitivePatterns matches performance-sensitive files like hot paths,
	// cache layers, database queries, and loop-intensive code.
	performanceSensitivePatterns = []*regexp.Regexp{
		// Cache layers
		regexp.MustCompile(`(?i)cache`),
		regexp.MustCompile(`(?i)cached`),
		regexp.MustCompile(`(?i)caching`),
		regexp.MustCompile(`(?i)redis`),
		regexp.MustCompile(`(?i)memcached`),
		regexp.MustCompile(`(?i)memcache`),

		// Database queries
		regexp.MustCompile(`(?i)query`),
		regexp.MustCompile(`(?i)database`),
		regexp.MustCompile(`(?i)db/`),
		regexp.MustCompile(`(?i)sql/`),
		regexp.MustCompile(`(?i)repo/`),
		regexp.MustCompile(`(?i)repository`),
		regexp.MustCompile(`(?i)model/`),
		regexp.MustCompile(`(?i)entity`),
		regexp.MustCompile(`(?i)schema`),
		regexp.MustCompile(`(?i)migration`),

		// Hot paths and performance-critical
		regexp.MustCompile(`(?i)hot`),
		regexp.MustCompile(`(?i)loop`),
		regexp.MustCompile(`(?i)iteration`),
		regexp.MustCompile(`(?i)batch`),
		regexp.MustCompile(`(?i)stream`),
		regexp.MustCompile(`(?i)pipeline`),

		// Async and concurrency
		regexp.MustCompile(`(?i)async`),
		regexp.MustCompile(`(?i)concurrent`),
		regexp.MustCompile(`(?i)parallel`),
		regexp.MustCompile(`(?i)worker`),
		regexp.MustCompile(`(?i)pool`),
		regexp.MustCompile(`(?i)queue`),

		// Memory and allocation
		regexp.MustCompile(`(?i)memory`),
		regexp.MustCompile(`(?i)alloc`),
		regexp.MustCompile(`(?i)gc/`),
		regexp.MustCompile(`(?i)garbage`),

		// Serialization
		regexp.MustCompile(`(?i)serial`),
		regexp.MustCompile(`(?i)deserialize`),
		regexp.MustCompile(`(?i)marshal`),
		regexp.MustCompile(`(?i)encode`),
		regexp.MustCompile(`(?i)decode`),

		// Network and I/O
		regexp.MustCompile(`(?i)http/`),
		regexp.MustCompile(`(?i)grpc`),
		regexp.MustCompile(`(?i)websocket`),
		regexp.MustCompile(`(?i)io/`),
		regexp.MustCompile(`(?i)fs/`),
		regexp.MustCompile(`(?i)file`),
	}

	// configFilePatterns matches configuration files that could affect performance.
	configFilePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\.yaml$`),
		regexp.MustCompile(`(?i)\.yml$`),
		regexp.MustCompile(`(?i)\.json$`),
		regexp.MustCompile(`(?i)\.toml$`),
		regexp.MustCompile(`(?i)\.ini$`),
		regexp.MustCompile(`(?i)\.conf$`),
		regexp.MustCompile(`(?i)\.config$`),
		regexp.MustCompile(`(?i)config\.go$`),
		regexp.MustCompile(`(?i)settings\.go$`),
	}
)

// Analyze examines PR data for performance risks and returns findings.
func (p *PerformanceAnalyzer) Analyze(ctx context.Context, prData PRData) (AnalyzerResult, error) {
	startTime := time.Now()

	pr := prData.PR
	var findings []types.AnalyzerFinding

	// 1. Detect large diffs (additions + deletions > threshold)
	largeDiffFindings := p.detectLargeDiffs(pr.Additions, pr.Deletions, pr.ChangedFilesCount)
	findings = append(findings, largeDiffFindings...)

	// 2. Detect many files changed
	fileCountFindings := p.detectManyFilesChanged(pr.ChangedFilesCount)
	findings = append(findings, fileCountFindings...)

	// 3. Detect touched critical paths
	criticalPathFindings := p.detectCriticalPathChanges(pr.FilesChanged)
	findings = append(findings, criticalPathFindings...)

	// 4. Detect performance-sensitive file changes
	perfSensitiveFindings := p.detectPerformanceSensitiveFiles(pr.FilesChanged)
	findings = append(findings, perfSensitiveFindings...)

	// 5. Detect high additions or deletions individually
	addDelFindings := p.detectHighAdditionsOrDeletions(pr.Additions, pr.Deletions)
	findings = append(findings, addDelFindings...)

	// 6. Detect config file changes that could affect performance
	configFindings := p.detectConfigChanges(pr.FilesChanged)
	findings = append(findings, configFindings...)

	// Determine overall category and priority based on findings
	category, priority, confidence := p.classifyPerformanceRisk(findings)

	result := types.ReviewResult{
		Category:         category,
		PriorityTier:     priority,
		Confidence:       confidence,
		Reasons:          p.extractReasons(findings),
		AnalyzerFindings: findings,
	}

	return AnalyzerResult{
		Result:           result,
		AnalyzerName:     "performance",
		AnalyzerVersion:  "0.1.0",
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Error:            nil,
		IsPartial:        false,
		SkippedReasons:   []string{},
		StartedAt:        startTime,
		CompletedAt:      time.Now(),
	}, nil
}

// detectLargeDiffs flags PRs with large total changes (additions + deletions > threshold).
func (p *PerformanceAnalyzer) detectLargeDiffs(additions, deletions, filesChanged int) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	totalChanges := additions + deletions

	if totalChanges > largeDiffLinesThreshold {
		// Calculate confidence based on how much over the threshold
		ratio := float64(totalChanges) / float64(largeDiffLinesThreshold)
		confidence := 0.70 + (0.20 * (ratio - 1) / ratio)
		if confidence > 0.95 {
			confidence = 0.95
		}

		finding := types.AnalyzerFinding{
			AnalyzerName:    "performance",
			AnalyzerVersion: "0.1.0",
			Finding:         "large diff detected: " + formatChangeStats(additions, deletions, filesChanged),
			Confidence:      confidence,
		}
		findings = append(findings, finding)
	}

	return findings
}

// detectManyFilesChanged flags PRs that change many files.
func (p *PerformanceAnalyzer) detectManyFilesChanged(filesChanged int) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	if filesChanged > largeFileCountThreshold {
		ratio := float64(filesChanged) / float64(largeFileCountThreshold)
		confidence := 0.75 + (0.15 * (ratio - 1) / ratio)
		if confidence > 0.90 {
			confidence = 0.90
		}

		finding := types.AnalyzerFinding{
			AnalyzerName:    "performance",
			AnalyzerVersion: "0.1.0",
			Finding:         "many files changed: " + itoa(filesChanged) + " files",
			Confidence:      confidence,
		}
		findings = append(findings, finding)
	}

	return findings
}

// detectCriticalPathChanges scans for changes to critical path files.
func (p *PerformanceAnalyzer) detectCriticalPathChanges(files []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	for _, file := range files {
		for _, pattern := range criticalPathPatterns {
			if pattern.MatchString(file) {
				finding := types.AnalyzerFinding{
					AnalyzerName:    "performance",
					AnalyzerVersion: "0.1.0",
					Finding:         "critical path change detected: " + file,
					Confidence:      0.80,
				}
				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// detectPerformanceSensitiveFiles scans for performance-sensitive file changes.
func (p *PerformanceAnalyzer) detectPerformanceSensitiveFiles(files []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	for _, file := range files {
		for _, pattern := range performanceSensitivePatterns {
			if pattern.MatchString(file) {
				finding := types.AnalyzerFinding{
					AnalyzerName:    "performance",
					AnalyzerVersion: "0.1.0",
					Finding:         "performance-sensitive file change detected: " + file,
					Confidence:      0.75,
				}
				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// detectHighAdditionsOrDeletions flags individual high additions or deletions.
func (p *PerformanceAnalyzer) detectHighAdditionsOrDeletions(additions, deletions int) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	if additions > highAdditionsThreshold {
		ratio := float64(additions) / float64(highAdditionsThreshold)
		confidence := 0.70 + (0.15 * (ratio - 1) / ratio)
		if confidence > 0.85 {
			confidence = 0.85
		}

		finding := types.AnalyzerFinding{
			AnalyzerName:    "performance",
			AnalyzerVersion: "0.1.0",
			Finding:         "high additions detected: +" + itoa(additions) + " lines",
			Confidence:      confidence,
		}
		findings = append(findings, finding)
	}

	if deletions > highDeletionsThreshold {
		ratio := float64(deletions) / float64(highDeletionsThreshold)
		confidence := 0.70 + (0.15 * (ratio - 1) / ratio)
		if confidence > 0.85 {
			confidence = 0.85
		}

		finding := types.AnalyzerFinding{
			AnalyzerName:    "performance",
			AnalyzerVersion: "0.1.0",
			Finding:         "high deletions detected: -" + itoa(deletions) + " lines",
			Confidence:      confidence,
		}
		findings = append(findings, finding)
	}

	return findings
}

// detectConfigChanges scans for configuration file changes that could affect performance.
func (p *PerformanceAnalyzer) detectConfigChanges(files []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	configFiles := make(map[string]bool)
	for _, file := range files {
		for _, pattern := range configFilePatterns {
			if pattern.MatchString(file) {
				if !configFiles[file] {
					configFiles[file] = true
					finding := types.AnalyzerFinding{
						AnalyzerName:    "performance",
						AnalyzerVersion: "0.1.0",
						Finding:         "config file change (may affect performance): " + file,
						Confidence:      0.70,
					}
					findings = append(findings, finding)
				}
				break
			}
		}
	}

	return findings
}

// classifyPerformanceRisk determines the review category and priority tier based on findings.
func (p *PerformanceAnalyzer) classifyPerformanceRisk(findings []types.AnalyzerFinding) (types.ReviewCategory, types.PriorityTier, float64) {
	if len(findings) == 0 {
		return types.ReviewCategoryMergeNow, types.PriorityTierFastMerge, 0.95
	}

	// Count findings by type
	largeDiffCount := 0
	manyFilesCount := 0
	criticalPathCount := 0
	perfSensitiveCount := 0
	highAddDelCount := 0
	configChangeCount := 0

	for _, f := range findings {
		if strings.Contains(f.Finding, "large diff") {
			largeDiffCount++
		} else if strings.Contains(f.Finding, "many files") {
			manyFilesCount++
		} else if strings.Contains(f.Finding, "critical path") {
			criticalPathCount++
		} else if strings.Contains(f.Finding, "performance-sensitive") {
			perfSensitiveCount++
		} else if strings.Contains(f.Finding, "high additions") || strings.Contains(f.Finding, "high deletions") {
			highAddDelCount++
		} else if strings.Contains(f.Finding, "config file") {
			configChangeCount++
		}
	}

	// High severity: large diff with critical path changes
	if largeDiffCount > 0 && criticalPathCount > 0 {
		return types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked, 0.85
	}

	// High severity: many files with performance-sensitive changes
	if manyFilesCount > 0 && perfSensitiveCount > 0 {
		return types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked, 0.80
	}

	// Medium severity: large diff without critical path
	if largeDiffCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.75
	}

	// Medium severity: many files changed
	if manyFilesCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.70
	}

	// Medium severity: critical path changes or performance-sensitive changes
	if criticalPathCount > 0 || perfSensitiveCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.75
	}

	// Lower severity: high additions/deletions or config changes
	if highAddDelCount > 0 || configChangeCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.65
	}

	return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.60
}

// extractReasons extracts human-readable reason codes from findings.
func (p *PerformanceAnalyzer) extractReasons(findings []types.AnalyzerFinding) []string {
	reasonSet := make(map[string]bool)

	for _, f := range findings {
		if strings.Contains(f.Finding, "large diff") {
			reasonSet["large_diff_detected"] = true
		} else if strings.Contains(f.Finding, "many files") {
			reasonSet["many_files_changed"] = true
		} else if strings.Contains(f.Finding, "critical path") {
			reasonSet["critical_path_changed"] = true
		} else if strings.Contains(f.Finding, "performance-sensitive") {
			reasonSet["performance_sensitive_changed"] = true
		} else if strings.Contains(f.Finding, "high additions") {
			reasonSet["high_additions"] = true
		} else if strings.Contains(f.Finding, "high deletions") {
			reasonSet["high_deletions"] = true
		} else if strings.Contains(f.Finding, "config file") {
			reasonSet["config_change"] = true
		}
	}

	var reasons []string
	for reason := range reasonSet {
		reasons = append(reasons, reason)
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "no_performance_issues")
	}

	return reasons
}

// formatChangeStats formats the change statistics for display.
func formatChangeStats(additions, deletions, filesChanged int) string {
	return "+" + itoa(additions) + " -" + itoa(deletions) + " lines in " + itoa(filesChanged) + " files"
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
