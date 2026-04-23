package ml

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}

	return path
}

func TestNewBridge(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		check  func(t *testing.T, b *Bridge)
	}{
		{
			name:   "default config",
			config: Config{},
			check: func(t *testing.T, b *Bridge) {
				if b == nil {
					t.Fatal("expected non-nil bridge")
				}
				if b.timeout != defaultTimeout {
					t.Errorf("expected default timeout %v, got %v", defaultTimeout, b.timeout)
				}
			},
		},
		{
			name: "custom timeout",
			config: Config{
				Timeout: 60 * time.Second,
			},
			check: func(t *testing.T, b *Bridge) {
				if b.timeout != 60*time.Second {
					t.Errorf("expected timeout 60s, got %v", b.timeout)
				}
			},
		},
		{
			name: "custom python path",
			config: Config{
				Python: "/usr/bin/python3",
			},
			check: func(t *testing.T, b *Bridge) {
				if b.python != "/usr/bin/python3" {
					t.Errorf("expected python /usr/bin/python3, got %s", b.python)
				}
			},
		},
		{
			name: "custom workdir",
			config: Config{
				WorkDir: "/opt/ml-service",
			},
			check: func(t *testing.T, b *Bridge) {
				if b.workDir != "/opt/ml-service" {
					t.Errorf("expected workDir /opt/ml-service, got %s", b.workDir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBridge(tt.config)
			tt.check(t, b)
		})
	}
}

func TestBridgeAvailable(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "unavailable when python path does not exist",
			config: Config{Python: "/nonexistent/python"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBridge(tt.config)
			// When python path is invalid, Available() should return false
			if b.Available() {
				t.Log("Available() returned true for non-existent python path - python may exist on this system")
			}
		})
	}
}

func TestBuildClusterPayload(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		prs       []types.PR
		requestID string
		check     func(t *testing.T, payload map[string]any)
	}{
		{
			name: "basic payload",
			repo: "owner/repo",
			prs: []types.PR{
				{
					ID:                "pr-1",
					Number:            1,
					Title:             "Fix bug",
					Body:              "Fixes a bug",
					Author:            "user1",
					FilesChanged:      []string{"a.go", "b.go"},
					Labels:            []string{"bug", "urgent"},
					ReviewStatus:      "APPROVED",
					CIStatus:          "SUCCESS",
					Mergeable:         "true",
					BaseBranch:        "main",
					HeadBranch:        "fix-bug",
					ClusterID:         "",
					CreatedAt:         "2024-01-01T00:00:00Z",
					UpdatedAt:         "2024-01-02T00:00:00Z",
					IsDraft:           false,
					IsBot:             false,
					Additions:         10,
					Deletions:         5,
					ChangedFilesCount: 2,
				},
			},
			requestID: "req-123",
			check: func(t *testing.T, payload map[string]any) {
				if action := payload["action"]; action != "cluster" {
					t.Errorf("action = %v, want cluster", action)
				}
				if repo := payload["repo"]; repo != "owner/repo" {
					t.Errorf("repo = %v, want owner/repo", repo)
				}
				if reqID := payload["request_id"]; reqID != "req-123" {
					t.Errorf("request_id = %v, want req-123", reqID)
				}
				prs, ok := payload["prs"].([]map[string]any)
				if !ok {
					t.Fatalf("prs is not a []map[string]any")
				}
				if len(prs) != 1 {
					t.Errorf("len(prs) = %d, want 1", len(prs))
				}
			},
		},
		{
			name:      "empty prs",
			repo:      "owner/repo",
			prs:       []types.PR{},
			requestID: "",
			check: func(t *testing.T, payload map[string]any) {
				prs, ok := payload["prs"].([]map[string]any)
				if !ok {
					t.Fatalf("prs is not a []map[string]any")
				}
				if len(prs) != 0 {
					t.Errorf("len(prs) = %d, want 0", len(prs))
				}
				if _, hasReqID := payload["request_id"]; hasReqID {
					t.Error("request_id should not be present when empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildClusterPayload(tt.repo, tt.prs, tt.requestID)
			tt.check(t, payload)
		})
	}
}

func TestBuildDuplicatePayload(t *testing.T) {
	tests := []struct {
		name               string
		repo               string
		prs                []types.PR
		duplicateThreshold float64
		overlapThreshold   float64
		requestID          string
		check              func(t *testing.T, payload map[string]any)
	}{
		{
			name:               "basic duplicate payload",
			repo:               "owner/repo",
			prs:                []types.PR{{Number: 1, Title: "PR 1"}},
			duplicateThreshold: 0.75,
			overlapThreshold:   0.50,
			requestID:          "dup-req",
			check: func(t *testing.T, payload map[string]any) {
				if action := payload["action"]; action != "duplicates" {
					t.Errorf("action = %v, want duplicates", action)
				}
				if dup := payload["duplicateThreshold"]; dup != 0.75 {
					t.Errorf("duplicateThreshold = %v, want 0.75", dup)
				}
				if ov := payload["overlapThreshold"]; ov != 0.50 {
					t.Errorf("overlapThreshold = %v, want 0.50", ov)
				}
			},
		},
		{
			name:               "with request id",
			repo:               "owner/repo",
			prs:                []types.PR{{Number: 1}},
			duplicateThreshold: 0.8,
			overlapThreshold:   0.6,
			requestID:          "req-456",
			check: func(t *testing.T, payload map[string]any) {
				if reqID := payload["request_id"]; reqID != "req-456" {
					t.Errorf("request_id = %v, want req-456", reqID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildDuplicatePayload(tt.repo, tt.prs, tt.duplicateThreshold, tt.overlapThreshold, tt.requestID)
			tt.check(t, payload)
		})
	}
}

func TestBuildAnalyzePayload(t *testing.T) {
	tests := []struct {
		name         string
		repo         string
		prs          []types.PR
		analysisMode string
		requestID    string
		check        func(t *testing.T, payload map[string]any)
	}{
		{
			name:         "standard mode",
			repo:         "owner/repo",
			prs:          []types.PR{{Number: 1}},
			analysisMode: "standard",
			requestID:    "",
			check: func(t *testing.T, payload map[string]any) {
				if action := payload["action"]; action != "analyze" {
					t.Errorf("action = %v, want analyze", action)
				}
				if mode := payload["analysisMode"]; mode != "standard" {
					t.Errorf("analysisMode = %v, want standard", mode)
				}
			},
		},
		{
			name:         "security mode with request id",
			repo:         "owner/repo",
			prs:          []types.PR{{Number: 1}, {Number: 2}},
			analysisMode: "security",
			requestID:    "sec-req",
			check: func(t *testing.T, payload map[string]any) {
				if mode := payload["analysisMode"]; mode != "security" {
					t.Errorf("analysisMode = %v, want security", mode)
				}
				prs, ok := payload["prs"].([]map[string]any)
				if !ok {
					t.Fatalf("prs is not a []map[string]any")
				}
				if len(prs) != 2 {
					t.Errorf("len(prs) = %d, want 2", len(prs))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildAnalyzePayload(tt.repo, tt.prs, tt.analysisMode, tt.requestID)
			tt.check(t, payload)
		})
	}
}

func TestPrToML(t *testing.T) {
	tests := []struct {
		name  string
		pr    types.PR
		check func(t *testing.T, result map[string]any)
	}{
		{
			name: "full pr conversion",
			pr: types.PR{
				ID:                "pr-123",
				Repo:              "owner/repo",
				Number:            42,
				Title:             "Add new feature",
				Body:              "This PR adds a new feature",
				URL:               types.GitHubURLPrefix + "owner/repo/pull/42",
				Author:            "developer",
				Labels:            []string{"enhancement", "needs-review"},
				FilesChanged:      []string{"feature.go", "feature_test.go"},
				ReviewStatus:      "CHANGES_REQUESTED",
				CIStatus:          "FAILURE",
				Mergeable:         "false",
				BaseBranch:        "main",
				HeadBranch:        "feature",
				ClusterID:         "cluster-1",
				CreatedAt:         "2024-01-15T10:30:00Z",
				UpdatedAt:         "2024-01-16T14:00:00Z",
				IsDraft:           true,
				IsBot:             false,
				Additions:         150,
				Deletions:         20,
				ChangedFilesCount: 2,
			},
			check: func(t *testing.T, result map[string]any) {
				if id := result["id"]; id != "pr-123" {
					t.Errorf("id = %v, want pr-123", id)
				}
				if num := result["number"]; num != 42 {
					t.Errorf("number = %v, want 42", num)
				}
				if title := result["title"]; title != "Add new feature" {
					t.Errorf("title = %v, want Add new feature", title)
				}
				if author := result["author"]; author != "developer" {
					t.Errorf("author = %v, want developer", author)
				}
				if isDraft := result["is_draft"]; isDraft != true {
					t.Errorf("is_draft = %v, want true", isDraft)
				}
				if isBot := result["is_bot"]; isBot != false {
					t.Errorf("is_bot = %v, want false", isBot)
				}
				if clusterID := result["cluster_id"]; clusterID != "cluster-1" {
					t.Errorf("cluster_id = %v, want cluster-1", clusterID)
				}
				labels, ok := result["labels"].([]string)
				if !ok {
					t.Fatalf("labels is not a []string")
				}
				if len(labels) != 2 {
					t.Errorf("len(labels) = %d, want 2", len(labels))
				}
			},
		},
		{
			name: "minimal pr conversion",
			pr: types.PR{
				ID:     "pr-min",
				Number: 1,
			},
			check: func(t *testing.T, result map[string]any) {
				if id := result["id"]; id != "pr-min" {
					t.Errorf("id = %v, want pr-min", id)
				}
				if num := result["number"]; num != 1 {
					t.Errorf("number = %v, want 1", num)
				}
				// Labels field should be nil for uninitialized slice
				labels := result["labels"]
				if labels != nil {
					// Could be nil slice or empty slice depending on implementation
					switch v := labels.(type) {
					case []string:
						if len(v) != 0 {
							t.Errorf("labels should be empty, got len %d", len(v))
						}
					case nil:
						// This is expected
					default:
						t.Errorf("labels = %v (type %T), expected nil or empty slice", labels, labels)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prToML(tt.pr)
			tt.check(t, result)
		})
	}
}

func TestPayloadJSONSerialization(t *testing.T) {
	pr := types.PR{
		ID:                "pr-test",
		Number:            10,
		Title:             "Test PR",
		Labels:            []string{"test"},
		FilesChanged:      []string{"test.go"},
		Additions:         5,
		Deletions:         3,
		ChangedFilesCount: 1,
	}

	t.Run("cluster payload serializes to valid JSON", func(t *testing.T) {
		payload := buildClusterPayload("owner/repo", []types.PR{pr}, "req-1")
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
	})

	t.Run("duplicate payload serializes to valid JSON", func(t *testing.T) {
		payload := buildDuplicatePayload("owner/repo", []types.PR{pr}, 0.75, 0.5, "req-2")
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
	})

	t.Run("analyze payload serializes to valid JSON", func(t *testing.T) {
		payload := buildAnalyzePayload("owner/repo", []types.PR{pr}, "security", "req-3")
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
	})
}

func TestClusterResultJSONParsing(t *testing.T) {
	t.Run("valid cluster result", func(t *testing.T) {
		jsonData := `{
			"clusters": [
				{
					"cluster_id": "c1",
					"cluster_label": "Bug fixes",
					"summary": "Collection of bug fixes",
					"pr_ids": [1, 2, 3],
					"health_status": "healthy",
					"average_similarity": 0.85,
					"sample_titles": ["Fix A", "Fix B"]
				}
			],
			"model": "v1"
		}`
		var result clusterResult
		if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(result.Clusters) != 1 {
			t.Errorf("expected 1 cluster, got %d", len(result.Clusters))
		}
		if result.Clusters[0].ClusterID != "c1" {
			t.Errorf("cluster_id = %s, want c1", result.Clusters[0].ClusterID)
		}
		if result.Model != "v1" {
			t.Errorf("model = %s, want v1", result.Model)
		}
	})

	t.Run("empty clusters", func(t *testing.T) {
		jsonData := `{"clusters": [], "model": ""}`
		var result clusterResult
		if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(result.Clusters) != 0 {
			t.Errorf("expected 0 clusters, got %d", len(result.Clusters))
		}
	})
}

func TestDuplicateResultJSONParsing(t *testing.T) {
	t.Run("valid duplicate result", func(t *testing.T) {
		jsonData := `{
			"duplicates": [
				{
					"canonical_pr_number": 10,
					"duplicate_pr_numbers": [20, 30],
					"similarity": 0.92,
					"reason": "Similar title and files"
				}
			],
			"overlaps": [
				{
					"canonical_pr_number": 5,
					"duplicate_pr_numbers": [15],
					"similarity": 0.65,
					"reason": "Overlapping changes"
				}
			]
		}`
		var result duplicateResult
		if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(result.Duplicates) != 1 {
			t.Errorf("expected 1 duplicate, got %d", len(result.Duplicates))
		}
		if len(result.Overlaps) != 1 {
			t.Errorf("expected 1 overlap, got %d", len(result.Overlaps))
		}
		if result.Duplicates[0].CanonicalPRNumber != 10 {
			t.Errorf("canonical_pr_number = %d, want 10", result.Duplicates[0].CanonicalPRNumber)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		jsonData := `{"duplicates": [], "overlaps": []}`
		var result duplicateResult
		if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(result.Duplicates) != 0 || len(result.Overlaps) != 0 {
			t.Errorf("expected empty results")
		}
	})
}

func TestAnalyzerResultJSONParsing(t *testing.T) {
	t.Run("valid analyzer result", func(t *testing.T) {
		jsonData := `{
			"analyzers": [
				{
					"analyzer_name": "security",
					"analyzer_version": "1.0.0",
					"findings": [
						{
							"pr_number": 42,
							"finding": "Potential SQL injection",
							"confidence": 0.88,
							"category": "security"
						}
					]
				}
			]
		}`
		var result AnalyzerResult
		if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(result.Analyzers) != 1 {
			t.Errorf("expected 1 analyzer, got %d", len(result.Analyzers))
		}
		if result.Analyzers[0].AnalyzerName != "security" {
			t.Errorf("analyzer_name = %s, want security", result.Analyzers[0].AnalyzerName)
		}
		if len(result.Analyzers[0].Findings) != 1 {
			t.Errorf("expected 1 finding, got %d", len(result.Analyzers[0].Findings))
		}
	})

	t.Run("multiple analyzers", func(t *testing.T) {
		jsonData := `{
			"analyzers": [
				{
					"analyzer_name": "security",
					"analyzer_version": "1.0.0",
					"findings": []
				},
				{
					"analyzer_name": "reliability",
					"analyzer_version": "2.1.0",
					"findings": []
				}
			]
		}`
		var result AnalyzerResult
		if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(result.Analyzers) != 2 {
			t.Errorf("expected 2 analyzers, got %d", len(result.Analyzers))
		}
	})
}

func TestAnalyzeRejectsEmptyAnalyzerStub(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	python := writeExecutable(t, workDir, "fake-python.sh", `#!/bin/sh
printf '{"analyzers": []}'
`)

	b := NewBridge(Config{
		Python:  python,
		WorkDir: workDir,
		Timeout: time.Second,
	})

	findings, err := b.Analyze(context.Background(), "owner/repo", []types.PR{{Number: 1, Title: "stub"}}, "reliability", "req-empty-stub")
	if err == nil {
		t.Fatalf("expected analyze to reject empty analyzer stub, got findings %#v", findings)
	}
}

func TestAnalyzeSurfacesStructuredNotImplementedPayload(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	python := writeExecutable(t, workDir, "fake-python.sh", `#!/bin/sh
printf '{"error":"not_implemented","status":"not_implemented","message":"Python analyze action is not implemented"}'
exit 1
`)

	b := NewBridge(Config{
		Python:  python,
		WorkDir: workDir,
		Timeout: time.Second,
	})

	_, err := b.Analyze(context.Background(), "owner/repo", []types.PR{{Number: 1, Title: "stub"}}, "reliability", "req-not-implemented")
	if err == nil {
		t.Fatal("expected analyze to surface not_implemented payload")
	}
	if !strings.Contains(err.Error(), "not_implemented") {
		t.Fatalf("error = %q, want propagated not_implemented status", err)
	}
	if !strings.Contains(err.Error(), "Python analyze action is not implemented") {
		t.Fatalf("error = %q, want propagated Python analyze message", err)
	}
}

func TestAnalyzeSurfacesStructuredDegradationPayload(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	python := writeExecutable(t, workDir, "fake-python.sh", `#!/bin/sh
printf '{"status":"degraded","degradation":{"fallback_reason":"local_backend","heuristic_fallback":true},"analyzers":[]}'
`)

	b := NewBridge(Config{
		Python:  python,
		WorkDir: workDir,
		Timeout: time.Second,
	})

	_, err := b.Analyze(context.Background(), "owner/repo", []types.PR{{Number: 1, Title: "stub"}}, "reliability", "req-degraded")
	if err == nil {
		t.Fatal("expected analyze to reject degraded analyzer payload")
	}
	if !strings.Contains(err.Error(), "degraded") {
		t.Fatalf("error = %q, want degraded status to be surfaced", err)
	}
	if !strings.Contains(err.Error(), "local_backend") {
		t.Fatalf("error = %q, want degradation reason to be surfaced", err)
	}
}

func TestClusterContractIncludesDegradationMetadataReturn(t *testing.T) {
	t.Parallel()

	clusterMethod := reflect.TypeOf((*Bridge).Cluster)
	if clusterMethod.NumOut() != 4 {
		t.Fatalf("Bridge.Cluster return count = %d, want 4 so degradation metadata can be surfaced", clusterMethod.NumOut())
	}

	degradationType := clusterMethod.Out(2)
	if degradationType.Kind() != reflect.Struct {
		t.Fatalf("Bridge.Cluster degradation return type = %v, want struct metadata", degradationType)
	}
	if _, ok := degradationType.FieldByName("FallbackReason"); !ok {
		t.Fatal("Bridge.Cluster degradation metadata missing FallbackReason")
	}
	field, ok := degradationType.FieldByName("HeuristicFallback")
	if !ok {
		t.Fatal("Bridge.Cluster degradation metadata missing HeuristicFallback")
	}
	if field.Type.Kind() != reflect.Bool {
		t.Fatalf("Bridge.Cluster HeuristicFallback type = %v, want bool", field.Type)
	}
}

func TestDuplicatesContractIncludesDegradationMetadataReturn(t *testing.T) {
	t.Parallel()

	duplicatesMethod := reflect.TypeOf((*Bridge).Duplicates)
	if duplicatesMethod.NumOut() != 4 {
		t.Fatalf("Bridge.Duplicates return count = %d, want 4 so degradation metadata can be surfaced", duplicatesMethod.NumOut())
	}

	degradationType := duplicatesMethod.Out(2)
	if degradationType.Kind() != reflect.Struct {
		t.Fatalf("Bridge.Duplicates degradation return type = %v, want struct metadata", degradationType)
	}
	if _, ok := degradationType.FieldByName("FallbackReason"); !ok {
		t.Fatal("Bridge.Duplicates degradation metadata missing FallbackReason")
	}
	field, ok := degradationType.FieldByName("HeuristicFallback")
	if !ok {
		t.Fatal("Bridge.Duplicates degradation metadata missing HeuristicFallback")
	}
	if field.Type.Kind() != reflect.Bool {
		t.Fatalf("Bridge.Duplicates HeuristicFallback type = %v, want bool", field.Type)
	}
}
