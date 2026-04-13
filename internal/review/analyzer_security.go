// Package review provides the Analyzer interface and analyzers for the agentic PR review system.
package review

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// SecurityAnalyzer detects security-related risks in PRs including:
// - Risky file paths (env files, secrets, credentials)
// - Auth/permission surface changes
// - Dependency changes with security implications
// - Large deletions in security-sensitive files
type SecurityAnalyzer struct{}

// NewSecurityAnalyzer creates a new SecurityAnalyzer that implements the Analyzer interface.
func NewSecurityAnalyzer() Analyzer {
	return &SecurityAnalyzer{}
}

// Metadata returns information about this analyzer for reporting purposes.
func (s *SecurityAnalyzer) Metadata() types.AnalyzerMetadata {
	return types.AnalyzerMetadata{
		Name:       "security",
		Version:    "0.1.0",
		Category:   "security",
		Confidence: 0.85,
	}
}

// securityFinding represents an individual security finding.
type securityFinding struct {
	finding    string
	confidence float64
	pattern    string
}

// riskyFilePatterns maps pattern names to regex patterns for detection.
var (
	// envAndSecretPatterns matches .env, secrets, credentials, passwords, tokens, keys
	envAndSecretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\.env$`),
		regexp.MustCompile(`(?i)\.env\.\w+$`),
		regexp.MustCompile(`(?i)_env$`),
		regexp.MustCompile(`(?i)secret`),
		regexp.MustCompile(`(?i)credential`),
		regexp.MustCompile(`(?i)password`),
		regexp.MustCompile(`(?i)token`),
		regexp.MustCompile(`(?i)\.key$`),
		regexp.MustCompile(`(?i)id_rsa`),
		regexp.MustCompile(`(?i)\.pem$`),
		regexp.MustCompile(`(?i)\.p12$`),
		regexp.MustCompile(`(?i)\.keystore$`),
		regexp.MustCompile(`(?i)oauth`),
		regexp.MustCompile(`(?i)api_key`),
		regexp.MustCompile(`(?i)apikey`),
	}

	// authRelatedPatterns matches auth and permission related files.
	authRelatedPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)auth`),
		regexp.MustCompile(`(?i)login`),
		regexp.MustCompile(`(?i)logout`),
		regexp.MustCompile(`(?i)permission`),
		regexp.MustCompile(`(?i)rbac`),
		regexp.MustCompile(`(?i)access_control`),
		regexp.MustCompile(`(?i)middleware`),
		regexp.MustCompile(`(?i)jwt`),
		regexp.MustCompile(`(?i)session`),
		regexp.MustCompile(`(?i)role`),
		regexp.MustCompile(`(?i)policy`),
		regexp.MustCompile(`(?i)secure`),
		regexp.MustCompile(`(?i)crypt`),
		regexp.MustCompile(`(?i)encrypt`),
		regexp.MustCompile(`(?i)paseto`),
		regexp.MustCompile(`(?i)bcrypt`),
		regexp.MustCompile(`(?i)scrypt`),
		regexp.MustCompile(`(?i)argon`),
	}

	// dependencyFilePatterns matches dependency manifest files.
	dependencyFilePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^go\.mod$`),
		regexp.MustCompile(`(?i)^go\.sum$`),
		regexp.MustCompile(`(?i)^package\.json$`),
		regexp.MustCompile(`(?i)^package-lock\.json$`),
		regexp.MustCompile(`(?i)^yarn\.lock$`),
		regexp.MustCompile(`(?i)^pnpm-lock\.yaml$`),
		regexp.MustCompile(`(?i)^requirements\.txt$`),
		regexp.MustCompile(`(?i)^Pipfile$`),
		regexp.MustCompile(`(?i)^Pipfile\.lock$`),
		regexp.MustCompile(`(?i)^poetry\.lock$`),
		regexp.MustCompile(`(?i)^pyproject\.toml$`),
		regexp.MustCompile(`(?i)^Cargo\.toml$`),
		regexp.MustCompile(`(?i)^Cargo\.lock$`),
		regexp.MustCompile(`(?i)^Gemfile$`),
		regexp.MustCompile(`(?i)^Gemfile\.lock$`),
		regexp.MustCompile(`(?i)^composer\.json$`),
		regexp.MustCompile(`(?i)^composer\.lock$`),
		regexp.MustCompile(`(?i)^Dockerfile$`),
		regexp.MustCompile(`(?i)^docker-compose\.ya?ml$`),
	}

	// securitySensitiveFilePatterns matches files that are security-sensitive
	// and large deletions should be flagged.
	securitySensitiveFilePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\.env(\.\w+)?$`),
		regexp.MustCompile(`(?i)auth`),
		regexp.MustCompile(`(?i)permission`),
		regexp.MustCompile(`(?i)role`),
		regexp.MustCompile(`(?i)policy`),
		regexp.MustCompile(`(?i)jwt`),
		regexp.MustCompile(`(?i)session`),
		regexp.MustCompile(`(?i)credential`),
		regexp.MustCompile(`(?i)secret`),
		regexp.MustCompile(`(?i)csrf`),
		regexp.MustCompile(`(?i)cors`),
		regexp.MustCompile(`(?i)rate_limit`),
		regexp.MustCompile(`(?i)firewall`),
		regexp.MustCompile(`(?i)iptable`),
	}
)

// Analyze examines PR data for security risks and returns findings.
func (s *SecurityAnalyzer) Analyze(ctx context.Context, prData PRData) (AnalyzerResult, error) {
	startTime := time.Now()

	pr := prData.PR
	var findings []types.AnalyzerFinding

	// 1. Detect risky file paths
	riskyFileFindings := s.detectRiskyFiles(pr.FilesChanged)
	findings = append(findings, riskyFileFindings...)

	// 2. Detect auth/permission surface changes
	authFindings := s.detectAuthChanges(pr.FilesChanged)
	findings = append(findings, authFindings...)

	// 3. Detect dependency changes with security implications
	dependencyFindings := s.detectDependencyChanges(pr.FilesChanged)
	findings = append(findings, dependencyFindings...)

	// 4. Detect large deletions in security-sensitive files
	deletionFindings := s.detectSecuritySensitiveDeletions(pr.FilesChanged, pr.Deletions)
	findings = append(findings, deletionFindings...)

	// Determine overall category and priority based on findings
	category, priority, confidence := s.classifySecurityRisk(findings)

	confidence = calculateConfidenceFromFindings(findings)
	confidence = capConfidenceByCategory(category, confidence)

	result := types.ReviewResult{
		Category:         category,
		PriorityTier:     priority,
		Confidence:       confidence,
		Reasons:          s.extractReasons(findings),
		AnalyzerFindings: findings,
	}

	return AnalyzerResult{
		Result:           result,
		AnalyzerName:     "security",
		AnalyzerVersion:  "0.1.0",
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Error:            nil,
		IsPartial:        false,
		SkippedReasons:   []string{},
		StartedAt:        startTime,
		CompletedAt:      time.Now(),
	}, nil
}

// detectRiskyFiles scans for risky file paths like .env, secrets, credentials.
func (s *SecurityAnalyzer) detectRiskyFiles(files []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	for _, file := range files {
		for _, pattern := range envAndSecretPatterns {
			if pattern.MatchString(file) {
				finding := types.AnalyzerFinding{
					AnalyzerName:    "security",
					AnalyzerVersion: "0.1.0",
					Finding:         "risky file path detected: " + file,
					Confidence:      0.90,
				}
				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// detectAuthChanges scans for auth/permission related file changes.
func (s *SecurityAnalyzer) detectAuthChanges(files []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	for _, file := range files {
		for _, pattern := range authRelatedPatterns {
			if pattern.MatchString(file) {
				finding := types.AnalyzerFinding{
					AnalyzerName:    "security",
					AnalyzerVersion: "0.1.0",
					Finding:         "auth/permission surface change detected: " + file,
					Confidence:      0.80,
				}
				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// detectDependencyChanges scans for dependency file changes.
func (s *SecurityAnalyzer) detectDependencyChanges(files []string) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	for _, file := range files {
		for _, pattern := range dependencyFilePatterns {
			if pattern.MatchString(file) {
				finding := types.AnalyzerFinding{
					AnalyzerName:    "security",
					AnalyzerVersion: "0.1.0",
					Finding:         "dependency change with security implications: " + file,
					Confidence:      0.75,
				}
				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// detectSecuritySensitiveDeletions flags large deletions in security-sensitive files.
func (s *SecurityAnalyzer) detectSecuritySensitiveDeletions(files []string, totalDeletions int) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	// Flag if significant deletions overall (>100 lines deleted)
	if totalDeletions > 100 {
		for _, file := range files {
			for _, pattern := range securitySensitiveFilePatterns {
				if pattern.MatchString(file) {
					finding := types.AnalyzerFinding{
						AnalyzerName:    "security",
						AnalyzerVersion: "0.1.0",
						Finding:         "large deletion in security-sensitive file: " + file,
						Confidence:      0.85,
					}
					findings = append(findings, finding)
					break
				}
			}
		}
	}

	return findings
}

// classifySecurityRisk determines the review category and priority tier based on findings.
func (s *SecurityAnalyzer) classifySecurityRisk(findings []types.AnalyzerFinding) (types.ReviewCategory, types.PriorityTier, float64) {
	if len(findings) == 0 {
		return types.ReviewCategoryMergeNow, types.PriorityTierFastMerge, 0.95
	}

	// Count findings by type
	riskyFileCount := 0
	authChangeCount := 0
	dependencyChangeCount := 0
	securityDeletionCount := 0

	for _, f := range findings {
		if strings.Contains(f.Finding, "risky file path") {
			riskyFileCount++
		} else if strings.Contains(f.Finding, "auth/permission") {
			authChangeCount++
		} else if strings.Contains(f.Finding, "dependency") {
			dependencyChangeCount++
		} else if strings.Contains(f.Finding, "deletion") {
			securityDeletionCount++
		}
	}

	// High severity: risky files or large auth changes
	if riskyFileCount > 0 || authChangeCount > 2 {
		return types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked, 0.85
	}

	// Medium severity: dependency changes or moderate auth changes
	if dependencyChangeCount > 0 || authChangeCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.80
	}

	// Low severity: only security-sensitive deletions
	if securityDeletionCount > 0 {
		return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.75
	}

	return types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, 0.70
}

// extractReasons extracts human-readable reason codes from findings.
func (s *SecurityAnalyzer) extractReasons(findings []types.AnalyzerFinding) []string {
	reasonSet := make(map[string]bool)

	for _, f := range findings {
		if strings.Contains(f.Finding, "risky file path") {
			reasonSet["risky_file_detected"] = true
		} else if strings.Contains(f.Finding, "auth/permission") {
			reasonSet["auth_surface_changed"] = true
		} else if strings.Contains(f.Finding, "dependency") {
			reasonSet["dependency_change"] = true
		} else if strings.Contains(f.Finding, "deletion") {
			reasonSet["security_deletion"] = true
		}
	}

	var reasons []string
	for reason := range reasonSet {
		reasons = append(reasons, reason)
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "no_security_issues")
	}

	return reasons
}
