package review

import (
	"strings"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func detectDependencyImpact(files []types.PRFile) []types.AnalyzerFinding {
	var findings []types.AnalyzerFinding
	seen := make(map[string]struct{})
	for _, file := range files {
		path := strings.ToLower(file.Path)
		subsystem := classifySubsystem(file.Path)
		signal := ""
		finding := ""
		switch {
		case strings.Contains(path, "contracts/") || strings.Contains(path, "/api/") || strings.Contains(path, "openapi") || strings.Contains(path, "graphql"):
			signal = "dependency_impact"
			finding = "public API surface changed"
			if subsystem == "unknown" {
				subsystem = "api"
			}
		case strings.Contains(path, "internal/types/") || strings.Contains(path, "internal/util/") || strings.Contains(path, "/pkg/") || strings.Contains(path, "/lib/") || strings.Contains(path, "/libs/"):
			signal = "dependency_impact"
			finding = "shared module changed"
		case strings.Contains(path, "migration") || strings.Contains(path, "schema") || strings.HasSuffix(path, ".sql"):
			signal = "rollout_impact"
			finding = "schema or migration change requires rollout coordination"
			if subsystem == "unknown" {
				subsystem = "database"
			}
		case strings.Contains(path, "config") || strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".toml"):
			signal = "rollout_impact"
			finding = "configuration surface changed"
			if subsystem == "unknown" {
				subsystem = "config"
			}
		}
		if signal == "" {
			continue
		}
		key := signal + ":" + finding + ":" + subsystem
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		findings = append(findings, types.AnalyzerFinding{
			AnalyzerName:    "reliability",
			AnalyzerVersion: "0.1.0",
			Finding:         finding,
			Confidence:      0.72,
			Subsystem:       subsystem,
			SignalType:      signal,
			Location: &types.CodeLocation{
				FilePath: file.Path,
				Snippet:  extractTestGapSnippet(file.Patch, 200),
			},
		})
	}
	return findings
}
