package review

import (
	"strings"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func detectRiskyDiffPatterns(files []types.PRFile, hunks []types.DiffHunk) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding

	if finding, ok := firstAuthRiskFinding(files, hunks); ok {
		findings = append(findings, finding)
	}
	if finding, ok := firstDatabaseRiskFinding(files); ok {
		findings = append(findings, finding)
	}
	if finding, ok := firstCryptoRiskFinding(files); ok {
		findings = append(findings, finding)
	}

	return findings
}

func firstAuthRiskFinding(files []types.PRFile, hunks []types.DiffHunk) (types.AnalyzerFinding, bool) {
	for _, hunk := range hunks {
		content := strings.ToLower(hunk.Content)
		if strings.Contains(content, "token") || strings.Contains(content, "permission") || strings.Contains(content, "role") || strings.Contains(content, "session") {
			return types.AnalyzerFinding{
				AnalyzerName:    "security",
				AnalyzerVersion: "0.1.0",
				Finding:         "auth-sensitive diff pattern detected",
				Confidence:      0.88,
				Subsystem:       "auth",
				SignalType:      "risky_pattern",
				DiffHunk:        &hunk,
				EvidenceHash:    evidenceHash(hunk.Content),
			}, true
		}
	}
	for _, file := range files {
		if classifySubsystem(file.Path) == "auth" {
			content := strings.ToLower(file.Patch)
			if strings.Contains(content, "token") || strings.Contains(content, "permission") || strings.Contains(content, "role") || strings.Contains(content, "session") {
				return types.AnalyzerFinding{
					AnalyzerName:    "security",
					AnalyzerVersion: "0.1.0",
					Finding:         "auth-sensitive diff pattern detected",
					Confidence:      0.85,
					Subsystem:       "auth",
					SignalType:      "risky_pattern",
					Location: &types.CodeLocation{
						FilePath: file.Path,
						Snippet:  extractSnippet(file.Patch, 200),
					},
					EvidenceHash: evidenceHash(file.Patch),
				}, true
			}
		}
	}
	return types.AnalyzerFinding{}, false
}

func firstDatabaseRiskFinding(files []types.PRFile) (types.AnalyzerFinding, bool) {
	for _, file := range files {
		content := strings.ToLower(file.Patch)
		if classifySubsystem(file.Path) == "database" || strings.Contains(content, "select ") || strings.Contains(content, "insert ") || strings.Contains(content, "update ") || strings.Contains(content, "delete ") || strings.Contains(content, "alter table") || strings.Contains(content, "create table") || strings.Contains(content, "drop table") {
			return types.AnalyzerFinding{
				AnalyzerName:    "security",
				AnalyzerVersion: "0.1.0",
				Finding:         "data-safety diff pattern detected",
				Confidence:      0.82,
				Subsystem:       "database",
				SignalType:      "risky_pattern",
				Location: &types.CodeLocation{
					FilePath: file.Path,
					Snippet:  extractSnippet(file.Patch, 200),
				},
				EvidenceHash: evidenceHash(file.Patch),
			}, true
		}
	}
	return types.AnalyzerFinding{}, false
}

func firstCryptoRiskFinding(files []types.PRFile) (types.AnalyzerFinding, bool) {
	for _, file := range files {
		content := strings.ToLower(file.Patch)
		if strings.Contains(content, "encrypt") || strings.Contains(content, "decrypt") || strings.Contains(content, "bcrypt") || strings.Contains(content, "sha256") || strings.Contains(content, "private key") || strings.Contains(content, "api_key") || strings.Contains(content, "credential") || strings.Contains(content, "secret") {
			return types.AnalyzerFinding{
				AnalyzerName:    "security",
				AnalyzerVersion: "0.1.0",
				Finding:         "crypto-or-secret diff pattern detected",
				Confidence:      0.84,
				Subsystem:       "security",
				SignalType:      "risky_pattern",
				Location: &types.CodeLocation{
					FilePath: file.Path,
					Snippet:  extractSnippet(file.Patch, 200),
				},
				EvidenceHash: evidenceHash(file.Patch),
			}, true
		}
	}
	return types.AnalyzerFinding{}, false
}
