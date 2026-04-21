package review

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// =============================================================================
// Security Analyzer Diff-Evidence Tests
// =============================================================================

func TestDetectRiskyFilesWithEvidence_EnvFile(t *testing.T) {
	t.Parallel()

	sec := &SecurityAnalyzer{}
	files := []types.PRFile{
		{
			Path:      ".env.production",
			Status:    "added",
			Additions: 5,
			Deletions: 0,
			Patch:     "+API_KEY=secret\n+PASSWORD=secret\n",
		},
	}

	findings := sec.detectRiskyFilesWithEvidence(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Location == nil {
		t.Fatal("expected location to be set")
	}
	if findings[0].Location.FilePath != ".env.production" {
		t.Errorf("expected file path .env.production, got %s", findings[0].Location.FilePath)
	}
	if findings[0].Confidence != 0.90 {
		t.Errorf("expected confidence 0.90, got %f", findings[0].Confidence)
	}
}

func TestDetectRiskyFilesWithEvidence_SecretFile(t *testing.T) {
	t.Parallel()

	sec := &SecurityAnalyzer{}
	files := []types.PRFile{
		{
			Path:      "secrets.json",
			Status:    "modified",
			Additions: 10,
			Deletions: 2,
			Patch:     "+ \"api_token\": \"xxx\"\n",
		},
	}

	findings := sec.detectRiskyFilesWithEvidence(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestDetectRiskyFilesWithEvidence_NoMatch(t *testing.T) {
	t.Parallel()

	sec := &SecurityAnalyzer{}
	files := []types.PRFile{
		{
			Path:      "src/main.go",
			Status:    "modified",
			Additions: 20,
			Deletions: 5,
			Patch:     "+func main() {}\n",
		},
	}

	findings := sec.detectRiskyFilesWithEvidence(files)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestDetectAuthChangesWithEvidence_JwtFile(t *testing.T) {
	t.Parallel()

	sec := &SecurityAnalyzer{}
	hunks := []types.DiffHunk{
		{
			OldPath:  "a/auth/jwt.go",
			NewPath:  "b/auth/jwt.go",
			OldStart: 10,
			OldLines: 5,
			NewStart: 10,
			NewLines: 7,
			Section:  "func ValidateToken()",
			Content:  "@@ -10,5 +10,7 @@ func ValidateToken() {\n old line\n+new token validation\n+another new line",
		},
	}

	findings := sec.detectAuthChangesWithEvidence(hunks)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Location == nil {
		t.Fatal("expected location to be set")
	}
	if findings[0].Location.FilePath != "b/auth/jwt.go" {
		t.Errorf("expected file path b/auth/jwt.go, got %s", findings[0].Location.FilePath)
	}
	if findings[0].Location.LineStart != 10 {
		t.Errorf("expected line start 10, got %d", findings[0].Location.LineStart)
	}

	if findings[0].DiffHunk == nil {
		t.Fatal("expected diff hunk to be set")
	}
	if findings[0].DiffHunk.NewPath != "b/auth/jwt.go" {
		t.Errorf("expected diff hunk new path b/auth/jwt.go, got %s", findings[0].DiffHunk.NewPath)
	}
}

func TestDetectAuthChangesWithEvidence_SessionFile(t *testing.T) {
	t.Parallel()

	sec := &SecurityAnalyzer{}
	hunks := []types.DiffHunk{
		{
			OldPath:  "a/middleware/session.go",
			NewPath:  "b/middleware/session.go",
			OldStart: 1,
			OldLines: 20,
			NewStart: 1,
			NewLines: 25,
			Section:  "func NewSession()",
			Content:  "@@ -1,20 +1,25 @@ func NewSession() {\n old session code\n+new session handling\n",
		},
	}

	findings := sec.detectAuthChangesWithEvidence(hunks)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestDetectAuthChangesWithEvidence_NoMatch(t *testing.T) {
	t.Parallel()

	sec := &SecurityAnalyzer{}
	hunks := []types.DiffHunk{
		{
			OldPath:  "a/src/main.go",
			NewPath:  "b/src/main.go",
			OldStart: 1,
			OldLines: 10,
			NewStart: 1,
			NewLines: 15,
			Section:  "func main()",
			Content:  "@@ -1,10 +1,15 @@ func main() {\n+fmt.Println(\"hello\")\n",
		},
	}

	findings := sec.detectAuthChangesWithEvidence(hunks)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestSecurityAnalyzer_Analyze_WithDiffEvidence(t *testing.T) {
	t.Parallel()

	sec := NewSecurityAnalyzer()
	prData := PRData{
		PR: types.PR{
			Number: 1,
			Title:  "Add JWT validation",
			Body:   "Implements token validation",
		},
		Files: []types.PRFile{
			{
				Path:      "auth/jwt.go",
				Status:    "modified",
				Additions: 20,
				Deletions: 5,
				Patch:     "+func ValidateToken() {}\n",
			},
		},
		DiffHunks: []types.DiffHunk{
			{
				OldPath:  "a/auth/jwt.go",
				NewPath:  "b/auth/jwt.go",
				OldStart: 10,
				OldLines: 5,
				NewStart: 10,
				NewLines: 7,
				Section:  "func ValidateToken()",
				Content:  "@@ -10,5 +10,7 @@ func ValidateToken() {\n+new token validation\n",
			},
		},
	}

	result, err := sec.Analyze(context.Background(), prData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have findings since we have auth changes
	if len(result.Result.AnalyzerFindings) == 0 {
		t.Fatal("expected findings for auth changes")
	}

	// Verify diff evidence is present in at least one finding
	foundDiffEvidence := false
	for _, f := range result.Result.AnalyzerFindings {
		if f.DiffHunk != nil || f.Location != nil {
			foundDiffEvidence = true
			break
		}
	}
	if !foundDiffEvidence {
		t.Error("expected at least one finding to have diff evidence (Location or DiffHunk)")
	}
}

// =============================================================================
// Quality Analyzer Diff-Evidence Tests
// =============================================================================

func TestDetectTestGap_ProdCodeWithoutTests(t *testing.T) {
	t.Parallel()

	q := &QualityAnalyzer{}
	files := []types.PRFile{
		{
			Path:      "src/auth.go",
			Status:    "modified",
			Additions: 50,
			Deletions: 0,
			Patch:     "+func NewAuth() {}\n+func Validate() {}\n",
		},
	}

	findings := q.detectTestGap(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Location == nil {
		t.Fatal("expected location to be set")
	}
	if findings[0].Location.FilePath != "src/auth.go" {
		t.Errorf("expected file path src/auth.go, got %s", findings[0].Location.FilePath)
	}
	if findings[0].Confidence != 0.75 {
		t.Errorf("expected confidence 0.75, got %f", findings[0].Confidence)
	}
}

func TestDetectTestGap_ProdCodeWithTests(t *testing.T) {
	t.Parallel()

	q := &QualityAnalyzer{}
	files := []types.PRFile{
		{
			Path:      "src/auth.go",
			Status:    "modified",
			Additions: 30,
			Deletions: 0,
			Patch:     "+func NewAuth() {}\n",
		},
		{
			Path:      "src/auth_test.go",
			Status:    "modified",
			Additions: 20,
			Deletions: 0,
			Patch:     "+func TestAuth() {}\n",
		},
	}

	findings := q.detectTestGap(files)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings since tests exist, got %d", len(findings))
	}
}

func TestDetectTestGap_SmallChanges(t *testing.T) {
	t.Parallel()

	q := &QualityAnalyzer{}
	files := []types.PRFile{
		{
			Path:      "src/auth.go",
			Status:    "modified",
			Additions: 5, // Less than 10 threshold
			Deletions: 0,
			Patch:     "+// small comment\n",
		},
	}

	findings := q.detectTestGap(files)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for small changes, got %d", len(findings))
	}
}

func TestDetectTestGap_ConfigFile(t *testing.T) {
	t.Parallel()

	q := &QualityAnalyzer{}
	files := []types.PRFile{
		{
			Path:      "config.yaml",
			Status:    "modified",
			Additions: 50,
			Deletions: 0,
			Patch:     "+new: config\n",
		},
	}

	findings := q.detectTestGap(files)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for config files, got %d", len(findings))
	}
}

func TestQualityAnalyzer_Analyze_WithDiffEvidence(t *testing.T) {
	t.Parallel()

	q := NewQualityAnalyzer()
	prData := PRData{
		PR: types.PR{
			Number: 1,
			Title:  "Implement new feature",
			Body:   "This adds a new feature",
		},
		Files: []types.PRFile{
			{
				Path:      "src/feature.go",
				Status:    "added",
				Additions: 80,
				Deletions: 0,
				Patch:     "+func NewFeature() {}\n+func DoThing() {}\n",
			},
		},
	}

	result, err := q.Analyze(context.Background(), prData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have findings since we have prod changes without tests
	if len(result.Result.AnalyzerFindings) == 0 {
		t.Fatal("expected findings for test gap")
	}

	// Verify location evidence is present in test gap finding
	foundLocationEvidence := false
	for _, f := range result.Result.AnalyzerFindings {
		if f.Location != nil && f.Location.FilePath == "src/feature.go" {
			foundLocationEvidence = true
			break
		}
	}
	if !foundLocationEvidence {
		t.Error("expected test gap finding to have location evidence for src/feature.go")
	}
}
