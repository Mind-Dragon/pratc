package sync

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestBootstrapFileSourceLoadsJSONL(t *testing.T) {
	// Create test JSONL file (line-by-line JSON)
	pr := types.PR{
		ID:                "gh-123",
		Repo:              "owner/repo",
		Number:            42,
		Title:             "Bootstrap Test",
		Body:              "",
		URL:               types.GitHubURLPrefix + "owner/repo/pull/42",
		Author:            "testuser",
		Labels:            []string{"bug"},
		FilesChanged:      nil,
		ReviewStatus:      "",
		CIStatus:          "",
		Mergeable:         "",
		BaseBranch:        "main",
		HeadBranch:        "feat",
		CreatedAt:         "2024-01-01T00:00:00Z",
		UpdatedAt:         "2024-01-02T00:00:00Z",
		IsDraft:           false,
		IsBot:             false,
		Additions:         10,
		Deletions:         5,
		ChangedFilesCount: 2,
	}

	prBytes, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("marshal PR: %v", err)
	}

	tmpfile, err := os.CreateTemp("", "bootstrap-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	// Write as JSONL (one JSON object per line)
	if _, err := tmpfile.Write(prBytes); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpfile.Close()

	source := NewBootstrapFileSource(tmpfile.Name(), "test_source")

	prs, err := source.Bootstrap(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("Bootstrap() error: %v", err)
	}

	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}

	if prs[0].Number != 42 {
		t.Errorf("expected PR number 42, got %d", prs[0].Number)
	}

	if prs[0].Author != "testuser" {
		t.Errorf("expected author 'testuser', got %q", prs[0].Author)
	}

	// Check provenance was set
	if prs[0].Provenance == nil {
		t.Error("expected provenance to be set")
	}

	if prs[0].Provenance["title"] != "test_source" {
		t.Errorf("expected provenance title='test_source', got %q", prs[0].Provenance["title"])
	}
}

func TestBootstrapFileSourceSkipsMismatchedRepo(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "bootstrap-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	// Write a PR for a different repo
	pr := types.PR{ID: "gh-999", Repo: "other/repo", Number: 99}
	prBytes, _ := json.Marshal(pr)
	if _, err := tmpfile.Write(prBytes); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpfile.Close()

	source := NewBootstrapFileSource(tmpfile.Name(), "test_source")

	prs, err := source.Bootstrap(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("Bootstrap() error: %v", err)
	}

	if len(prs) != 0 {
		t.Errorf("expected 0 PRs for mismatched repo, got %d", len(prs))
	}
}

func TestCompositeBootstrapFallsThroughToNextSource(t *testing.T) {
	// First source returns empty (no error), second source has data
	// This tests the fall-through behavior: empty+no-error → try next
	tmpfile, err := os.CreateTemp("", "bootstrap-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	pr := types.PR{ID: "gh-777", Repo: "owner/repo", Number: 77}
	prBytes, _ := json.Marshal(pr)
	if _, err := tmpfile.Write(prBytes); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpfile.Close()

	source1 := NewBootstrapFileSource("", "source1") // empty path returns nil, nil
	source2 := NewBootstrapFileSource(tmpfile.Name(), "source2")

	composite := CompositeBootstrapSource{Sources: []BootstrapSource{source1, source2}}

	prs, err := composite.Bootstrap(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("Composite Bootstrap() error: %v", err)
	}

	// Second source returned the PR
	if len(prs) != 1 {
		t.Errorf("expected 1 PR from composite, got %d", len(prs))
	}
	if prs[0].Number != 77 {
		t.Errorf("expected PR number 77, got %d", prs[0].Number)
	}
}

func TestCompositeBootstrapErrorsPropagate(t *testing.T) {
	// Both sources error - error should propagate
	source1 := NewBootstrapFileSource("/nonexistent/path1", "source1")
	source2 := NewBootstrapFileSource("/nonexistent/path2", "source2")

	composite := CompositeBootstrapSource{Sources: []BootstrapSource{source1, source2}}

	_, err := composite.Bootstrap(context.Background(), "owner/repo")
	if err == nil {
		t.Fatal("expected error from composite with all-failing sources")
	}
}

func TestDecodeBootstrapPRsJSONL(t *testing.T) {
	input := `{"id":"1","repo":"o/r","number":1}
{"id":"2","repo":"o/r","number":2}`

	prs, err := decodeBootstrapPRs([]byte(input))
	if err != nil {
		t.Fatalf("decodeBootstrapPRs error: %v", err)
	}

	if len(prs) != 2 {
		t.Errorf("expected 2 PRs, got %d", len(prs))
	}

	if prs[0].ID != "1" || prs[1].ID != "2" {
		t.Errorf("unexpected PR IDs: %v", []string{prs[0].ID, prs[1].ID})
	}
}

func TestDecodeBootstrapPRsEmpty(t *testing.T) {
	prs, err := decodeBootstrapPRs([]byte(""))
	if err != nil {
		t.Fatalf("decodeBootstrapPRs error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs for empty input, got %d", len(prs))
	}

	prs, err = decodeBootstrapPRs([]byte(strings.Repeat(" ", 10)))
	if err != nil {
		t.Fatalf("decodeBootstrapPRs error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs for whitespace input, got %d", len(prs))
	}
}
