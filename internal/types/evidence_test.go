package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCodeLocationJSON verifies CodeLocation serializes/deserializes correctly.
func TestCodeLocationJSON(t *testing.T) {
	loc := CodeLocation{
		FilePath:    "internal/review/analyzer.go",
		LineStart:   42,
		LineEnd:     55,
		ColumnStart: 1,
		ColumnEnd:   80,
		Snippet:     "func (a *Analyzer) Analyze(pr PRData)",
	}

	data, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("failed to marshal CodeLocation: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	expectedFields := []string{"file_path", "line_start", "line_end", "column_start", "column_end", "snippet"}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON missing field %q: %s", field, jsonStr)
		}
	}

	// Verify round-trip
	var parsed CodeLocation
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal CodeLocation: %v", err)
	}
	if parsed.FilePath != loc.FilePath {
		t.Errorf("FilePath mismatch: got %q, want %q", parsed.FilePath, loc.FilePath)
	}
	if parsed.LineStart != loc.LineStart {
		t.Errorf("LineStart mismatch: got %d, want %d", parsed.LineStart, loc.LineStart)
	}
}

// TestDiffHunkJSON verifies DiffHunk serializes/deserializes correctly.
func TestDiffHunkJSON(t *testing.T) {
	hunk := DiffHunk{
		OldPath:  "a/internal/review/analyzer.go",
		NewPath:  "b/internal/review/analyzer.go",
		OldStart: 40,
		OldLines: 5,
		NewStart: 40,
		NewLines: 8,
		Content:  "@@ -40,5 +40,8 @@\n func Analyze() {\n+    // new line\n     return\n }",
		Section:  "@@ -40,5 +40,8 @@ func Analyze()",
	}

	data, err := json.Marshal(hunk)
	if err != nil {
		t.Fatalf("failed to marshal DiffHunk: %v", err)
	}

	// Verify round-trip
	var parsed DiffHunk
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal DiffHunk: %v", err)
	}
	if parsed.OldPath != hunk.OldPath {
		t.Errorf("OldPath mismatch: got %q, want %q", parsed.OldPath, hunk.OldPath)
	}
	if parsed.NewStart != hunk.NewStart {
		t.Errorf("NewStart mismatch: got %d, want %d", parsed.NewStart, hunk.NewStart)
	}
}

// TestAnalyzerFindingWithLocation verifies AnalyzerFinding can include location and diff hunk.
func TestAnalyzerFindingWithLocation(t *testing.T) {
	finding := AnalyzerFinding{
		AnalyzerName:    "security",
		AnalyzerVersion: "0.1.0",
		Finding:         "potential SQL injection",
		Confidence:      0.85,
		Subsystem:       "auth",
		SignalType:      "risky_pattern",
		Location: &CodeLocation{
			FilePath:  "internal/db/query.go",
			LineStart: 42,
			LineEnd:   45,
			Snippet:   "db.Exec(\"SELECT * FROM users WHERE id = \" + userID)",
		},
		DiffHunk: &DiffHunk{
			OldPath:  "a/internal/db/query.go",
			NewPath:  "b/internal/db/query.go",
			OldStart: 40,
			OldLines: 2,
			NewStart: 40,
			NewLines: 5,
			Content:  "-    result := db.Query(...)\n+    result := db.Exec(...)",
		},
		EvidenceHash: "sha256:abc123...",
	}

	data, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("failed to marshal AnalyzerFinding: %v", err)
	}

	// Verify round-trip
	var parsed AnalyzerFinding
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal AnalyzerFinding: %v", err)
	}
	if parsed.AnalyzerName != finding.AnalyzerName {
		t.Errorf("AnalyzerName mismatch: got %q, want %q", parsed.AnalyzerName, finding.AnalyzerName)
	}
	if parsed.Subsystem != finding.Subsystem {
		t.Errorf("Subsystem mismatch: got %q, want %q", parsed.Subsystem, finding.Subsystem)
	}
	if parsed.SignalType != finding.SignalType {
		t.Errorf("SignalType mismatch: got %q, want %q", parsed.SignalType, finding.SignalType)
	}
	if parsed.Location == nil {
		t.Fatal("Location is nil after unmarshal")
	}
	if parsed.Location.FilePath != finding.Location.FilePath {
		t.Errorf("Location.FilePath mismatch: got %q, want %q", parsed.Location.FilePath, finding.Location.FilePath)
	}
	if parsed.DiffHunk == nil {
		t.Fatal("DiffHunk is nil after unmarshal")
	}
	if parsed.DiffHunk.NewPath != finding.DiffHunk.NewPath {
		t.Errorf("DiffHunk.NewPath mismatch: got %q, want %q", parsed.DiffHunk.NewPath, finding.DiffHunk.NewPath)
	}
	if parsed.EvidenceHash != finding.EvidenceHash {
		t.Errorf("EvidenceHash mismatch: got %q, want %q", parsed.EvidenceHash, finding.EvidenceHash)
	}
}

// TestAnalyzerFindingMinimal verifies AnalyzerFinding works without optional fields.
func TestAnalyzerFindingMinimal(t *testing.T) {
	finding := AnalyzerFinding{
		AnalyzerName:    "quality",
		AnalyzerVersion: "0.1.0",
		Finding:         "missing tests",
		Confidence:      0.70,
	}

	data, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("failed to marshal minimal AnalyzerFinding: %v", err)
	}

	// Verify round-trip
	var parsed AnalyzerFinding
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal minimal AnalyzerFinding: %v", err)
	}
	if parsed.Location != nil {
		t.Error("expected Location to be nil for minimal finding")
	}
	if parsed.DiffHunk != nil {
		t.Error("expected DiffHunk to be nil for minimal finding")
	}
}

// TestPRFileJSON verifies PRFile serializes/deserializes correctly.
func TestPRFileJSON(t *testing.T) {
	file := PRFile{
		Path:      "internal/types/models.go",
		Status:    "modified",
		Additions: 45,
		Deletions: 12,
		Patch:     "@@ -10,5 +10,8 @@...",
	}

	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("failed to marshal PRFile: %v", err)
	}

	// Verify round-trip
	var parsed PRFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal PRFile: %v", err)
	}
	if parsed.Path != file.Path {
		t.Errorf("Path mismatch: got %q, want %q", parsed.Path, file.Path)
	}
	if parsed.Additions != file.Additions {
		t.Errorf("Additions mismatch: got %d, want %d", parsed.Additions, file.Additions)
	}
	if parsed.Patch != file.Patch {
		t.Errorf("Patch mismatch: got %q, want %q", parsed.Patch, file.Patch)
	}
}

// TestReviewResultWithFindings verifies ReviewResult can include findings with evidence.
func TestReviewResultWithFindings(t *testing.T) {
	result := ReviewResult{
		Category:     "problematic_quarantine",
		PriorityTier: "blocked",
		Confidence:   0.92,
		Reasons:      []string{"security_issue", "missing_tests"},
		Blockers:     []string{"SQL injection vulnerability"},
		AnalyzerFindings: []AnalyzerFinding{
			{
				AnalyzerName:    "security",
				AnalyzerVersion: "0.1.0",
				Finding:         "sql_injection",
				Confidence:      0.95,
				Location: &CodeLocation{
					FilePath:  "internal/db/query.go",
					LineStart: 42,
				},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal ReviewResult: %v", err)
	}

	// Verify round-trip
	var parsed ReviewResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal ReviewResult: %v", err)
	}
	if len(parsed.AnalyzerFindings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(parsed.AnalyzerFindings))
	}
	if parsed.AnalyzerFindings[0].Location == nil {
		t.Fatal("expected finding to have Location")
	}
}
