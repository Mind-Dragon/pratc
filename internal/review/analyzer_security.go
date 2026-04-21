// Package review provides the Analyzer interface and analyzers for the agentic PR review system.
package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	// 1. Detect risky file paths (fallback to FilesChanged if no diff evidence)
	if len(prData.Files) > 0 {
		riskyFileFindings := s.detectRiskyFilesWithEvidence(prData.Files)
		findings = append(findings, riskyFileFindings...)
	} else {
		riskyFileFindings := s.detectRiskyFiles(pr.FilesChanged)
		findings = append(findings, riskyFileFindings...)
	}

	// 2. Detect auth/permission surface changes with diff evidence
	if len(prData.DiffHunks) > 0 {
		authFindings := s.detectAuthChangesWithEvidence(prData.DiffHunks)
		findings = append(findings, authFindings...)
	} else {
		authFindings := s.detectAuthChanges(pr.FilesChanged)
		findings = append(findings, authFindings...)
	}

	// 3. Detect dependency changes with security implications
	dependencyFindings := s.detectDependencyChanges(pr.FilesChanged)
	findings = append(findings, dependencyFindings...)

	// 4. Detect large deletions in security-sensitive files
	deletionFindings := s.detectSecuritySensitiveDeletions(pr.FilesChanged, pr.Deletions)
	findings = append(findings, deletionFindings...)

	// Determine overall category and priority based on findings.
	// The confidence returned by classifySecurityRisk is deliberately discarded;
	// evidence-backed confidence is computed below from the actual findings.
	category, priority, _ := s.classifySecurityRisk(findings)

	confidence := calculateConfidenceFromFindings(findings)
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

// evidenceHash computes a SHA-256 hash of the given content and returns it
// in the format "sha256:<hex>" for use as an AnalyzerFinding.EvidenceHash.
func evidenceHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(h[:])
}

// detectRiskyFilesWithEvidence scans files with full patch data for risky files
// and emits findings with concrete location evidence.
func (s *SecurityAnalyzer) detectRiskyFilesWithEvidence(files []types.PRFile) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	for _, file := range files {
		for _, pattern := range envAndSecretPatterns {
			if pattern.MatchString(file.Path) {
				// Build location evidence from patch if available
				location := &types.CodeLocation{
					FilePath: file.Path,
					Snippet: extractSnippet(file.Patch, 200),
				}

				finding := types.AnalyzerFinding{
					AnalyzerName:    "security",
					AnalyzerVersion: "0.1.0",
					Finding:         "risky file path detected: " + file.Path,
					Confidence:      0.90,
					Location:        location,
					EvidenceHash:    evidenceHash(file.Patch),
				}
				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// detectAuthChangesWithEvidence scans diff hunks for auth/permission-related changes
// and emits findings with concrete diff evidence.
func (s *SecurityAnalyzer) detectAuthChangesWithEvidence(hunks []types.DiffHunk) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	for _, hunk := range hunks {
		for _, pattern := range authRelatedPatterns {
			if pattern.MatchString(hunk.NewPath) {
				finding := types.AnalyzerFinding{
					AnalyzerName:    "security",
					AnalyzerVersion: "0.1.0",
					Finding:         "auth/permission surface change detected in " + hunk.NewPath,
					Confidence:      0.80,
					Location: &types.CodeLocation{
						FilePath:  hunk.NewPath,
						LineStart: hunk.NewStart,
						LineEnd:   hunk.NewStart + hunk.NewLines - 1,
						Snippet:   truncateSnippet(hunk.Content, 300),
					},
					DiffHunk: &types.DiffHunk{
						OldPath:  hunk.OldPath,
						NewPath:  hunk.NewPath,
						OldStart: hunk.OldStart,
						OldLines: hunk.OldLines,
						NewStart: hunk.NewStart,
						NewLines: hunk.NewLines,
						Content:  truncateSnippet(hunk.Content, 500),
						Section:  hunk.Section,
					},
					EvidenceHash: evidenceHash(hunk.Content),
				}
				findings = append(findings, finding)
				break
			}
		}
	}

	return findings
}

// extractSnippet extracts a meaningful snippet from patch content.
func extractSnippet(patch string, maxLen int) string {
	if patch == "" {
		return ""
	}
	lines := strings.Split(patch, "\n")
	var sb strings.Builder
	for _, line := range lines {
		if len(line) > 0 && (strings.HasPrefix(line, "+") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "-")) {
			sb.WriteString(line[:min(len(line), 80)])
			sb.WriteString("\n")
			if sb.Len() > maxLen {
				break
			}
		}
	}
	result := sb.String()
	if len(result) > maxLen {
		return result[:maxLen] + "..."
	}
	return result
}

// truncateSnippet truncates a string to maxLen characters.
func truncateSnippet(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
