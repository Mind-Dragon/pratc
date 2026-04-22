package review

import "github.com/jeffersonnunn/pratc/internal/types"

func detectTestEvidence(files []types.PRFile) []types.AnalyzerFinding {
	var prodFiles []types.PRFile
	var testFiles []types.PRFile
	for _, file := range files {
		if isTestFile(file.Path) {
			testFiles = append(testFiles, file)
		} else if isProductionCode(file.Path) {
			prodFiles = append(prodFiles, file)
		}
	}
	if len(prodFiles) == 0 || len(testFiles) == 0 {
		return nil
	}

	totalProdAdds := 0
	totalTestAdds := 0
	for _, f := range prodFiles {
		totalProdAdds += f.Additions
	}
	for _, f := range testFiles {
		totalTestAdds += f.Additions
	}
	dominant := prodFiles[0]
	for _, f := range prodFiles[1:] {
		if f.Additions > dominant.Additions {
			dominant = f
		}
	}

	findings := []types.AnalyzerFinding{{
		AnalyzerName:    "quality",
		AnalyzerVersion: "0.1.0",
		Finding:         "production code changed with accompanying tests",
		Confidence:      0.70,
		Subsystem:       classifySubsystem(dominant.Path),
		SignalType:      "test_evidence",
		Location: &types.CodeLocation{FilePath: dominant.Path, Snippet: extractTestGapSnippet(dominant.Patch, 200)},
	}}
	if totalTestAdds*4 < totalProdAdds {
		findings = append(findings, types.AnalyzerFinding{
			AnalyzerName:    "quality",
			AnalyzerVersion: "0.1.0",
			Finding:         "test coverage appears partial for production change",
			Confidence:      0.60,
			Subsystem:       classifySubsystem(dominant.Path),
			SignalType:      "coverage_partial",
			Location: &types.CodeLocation{FilePath: dominant.Path, Snippet: extractTestGapSnippet(dominant.Patch, 200)},
		})
	}
	return findings
}
