package types

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestGoAndPythonProduceIdenticalJSON(t *testing.T) {
	t.Parallel()

	report := sampleAnalysisResponse()

	goJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal go payload: %v", err)
	}

	pythonPath, ok := findPython()
	if !ok {
		t.Skip("python interpreter not available")
	}

	repoRoot := repoRoot(t)
	pythonFile := filepath.Join(repoRoot, "ml-service", "src", "pratc_ml", "models.py")
	cmd := exec.Command(pythonPath, pythonFile)
	cmd.Env = append(os.Environ(), "PRATC_SAMPLE_ANALYSIS_JSON="+string(goJSON))
	cmd.Dir = repoRoot

	pythonJSON, err := cmd.Output()
	if err != nil {
		t.Fatalf("run python serializer: %v", err)
	}

	if !bytes.Equal(normalizeJSON(t, goJSON), normalizeJSON(t, pythonJSON)) {
		t.Fatalf("go and python json differ\ngo=%s\npython=%s", normalizeJSON(t, goJSON), normalizeJSON(t, pythonJSON))
	}
}

func TestJSONSchemaFilesCoverTaskContracts(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	requiredSchemas := map[string][]string{
		"analysis-response.json":           {"repo", "generatedAt", "counts", "clusters", "duplicates", "overlaps", "conflicts", "stalenessSignals"},
		"cluster-response.json":            {"repo", "generatedAt", "model", "thresholds", "clusters"},
		"duplicate-response.json":          {"repo", "generatedAt", "duplicates", "overlaps"},
		"semantic-conflict-response.json":  {"repo", "generatedAt", "conflicts"},
		"graph-response.json":              {"repo", "generatedAt", "nodes", "edges", "dot"},
		"plan-response.json":               {"repo", "generatedAt", "target", "candidatePoolSize", "strategy", "selected", "ordering", "rejections"},
		"health-response.json":             {"status", "version"},
		"cluster-request.json":             {"repo", "prs", "model", "minClusterSize"},
		"duplicate-detection-request.json": {"repo", "prs", "duplicateThreshold", "overlapThreshold"},
		"semantic-analysis-request.json":   {"repo", "prs", "analysisMode"},
	}

	for file, fields := range requiredSchemas {
		file := file
		fields := fields
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(repoRoot, "contracts", file))
			if err != nil {
				t.Fatalf("read schema %s: %v", file, err)
			}

			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatalf("parse schema %s: %v", file, err)
			}

			properties, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("schema %s missing properties", file)
			}

			required, ok := schema["required"].([]any)
			if !ok {
				t.Fatalf("schema %s missing required list", file)
			}

			for _, field := range fields {
				if _, ok := properties[field]; !ok {
					t.Fatalf("schema %s missing property %s", file, field)
				}
				if !contains(required, field) {
					t.Fatalf("schema %s missing required entry %s", file, field)
				}
			}
		})
	}
}

func TestTypeScriptInterfacesMirrorCanonicalFields(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(repoRoot, "web", "src", "types", "api.ts"))
	if err != nil {
		t.Fatalf("read typescript file: %v", err)
	}

	for _, token := range []string{
		"export interface PR",
		"files_changed: string[];",
		"cluster_id: string;",
		"export interface PRCluster",
		"cluster_label: string;",
		"health_status: string;",
		"export interface AnalysisResponse",
		"generatedAt: string;",
		"stalenessSignals: StalenessReport[];",
		"export interface PlanResponse",
		"candidatePoolSize: number;",
	} {
		if !bytes.Contains(raw, []byte(token)) {
			t.Fatalf("expected token %q in api.ts", token)
		}
	}
}

func normalizeJSON(t *testing.T, raw []byte) []byte {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}

	normalized, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-encode json: %v", err)
	}

	return normalized
}

func contains(items []any, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func findPython() (string, bool) {
	candidates := []string{"python3", "/opt/homebrew/bin/python3.14", "/opt/homebrew/bin/python3.13", "/opt/homebrew/bin/python3.11"}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	return root
}

func sampleAnalysisResponse() AnalysisResponse {
	pr := PR{
		ID:                "PR_kwDOAA",
		Repo:              "owner/repo",
		Number:            101,
		Title:             "Reduce planner conflicts",
		Body:              "Introduces a smaller candidate pool.",
		URL:               "https://github.com/owner/repo/pull/101",
		Author:            "octocat",
		Labels:            []string{"planner", "ci"},
		FilesChanged:      []string{"internal/planner/plan.go", "internal/types/models.go"},
		ReviewStatus:      "approved",
		CIStatus:          "passing",
		Mergeable:         "mergeable",
		BaseBranch:        "main",
		HeadBranch:        "planner/reduce-conflicts",
		ClusterID:         "cluster-1",
		CreatedAt:         "2026-03-12T10:00:00Z",
		UpdatedAt:         "2026-03-12T11:00:00Z",
		IsDraft:           false,
		IsBot:             false,
		Additions:         42,
		Deletions:         7,
		ChangedFilesCount: 2,
	}

	cluster := PRCluster{
		ClusterID:         "cluster-1",
		ClusterLabel:      "planner conflicts",
		Summary:           "Planner-focused PRs that reduce merge contention.",
		PRIDs:             []int{101},
		HealthStatus:      "green",
		AverageSimilarity: 0.94,
		SampleTitles:      []string{"Reduce planner conflicts"},
	}

	dup := DuplicateGroup{
		CanonicalPRNumber: 101,
		DuplicatePRNums:   []int{104},
		Similarity:        0.93,
		Reason:            "Title and file overlap exceed duplicate threshold.",
	}

	conflict := ConflictPair{
		SourcePR:     101,
		TargetPR:     102,
		ConflictType: "file_overlap",
		FilesTouched: []string{"internal/planner/plan.go"},
		Severity:     "medium",
		Reason:       "Both PRs modify the planner ordering logic.",
	}

	stale := StalenessReport{
		PRNumber:     103,
		Score:        88,
		Signals:      []string{"superseded", "inactive"},
		Reasons:      []string{"A merged PR already touched the same files.", "No updates in 45 days."},
		SupersededBy: []int{99},
	}

	return AnalysisResponse{
		Repo:        "owner/repo",
		GeneratedAt: "2026-03-12T12:00:00Z",
		Counts: Counts{
			TotalPRs:        3,
			ClusterCount:    1,
			DuplicateGroups: 1,
			OverlapGroups:   1,
			ConflictPairs:   1,
			StalePRs:        1,
		},
		PRs:              []PR{pr},
		Clusters:         []PRCluster{cluster},
		Duplicates:       []DuplicateGroup{dup},
		Overlaps:         []DuplicateGroup{dup},
		Conflicts:        []ConflictPair{conflict},
		StalenessSignals: []StalenessReport{stale},
	}
}

func TestSampleAnalysisResponseContainsExpectedKeys(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(sampleAnalysisResponse())
	if err != nil {
		t.Fatalf("marshal sample response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode sample response: %v", err)
	}

	want := []string{"repo", "generatedAt", "counts", "prs", "clusters", "duplicates", "overlaps", "conflicts", "stalenessSignals"}
	for _, key := range want {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing key %s", key)
		}
	}

	prs, ok := decoded["prs"].([]any)
	if !ok || len(prs) != 1 {
		t.Fatalf("unexpected prs payload: %#v", decoded["prs"])
	}
}

func TestSchemaFieldSetsStayStable(t *testing.T) {
	t.Parallel()

	got := reflect.ValueOf(sampleAnalysisResponse())
	if got.NumField() != 16 {
		t.Fatalf("unexpected analysis response field count: %d", got.NumField())
	}
}
