package ml

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestDegradationMetadataContractIncludesEmbeddingsUsed(t *testing.T) {
	t.Parallel()

	metadataType := reflect.TypeOf(types.DegradationMetadata{})
	field, ok := metadataType.FieldByName("EmbeddingsUsed")
	if !ok {
		t.Fatal("DegradationMetadata missing EmbeddingsUsed field")
	}
	if field.Type.Kind() != reflect.Bool {
		t.Fatalf("EmbeddingsUsed type = %v, want bool", field.Type)
	}
	if field.Tag.Get("json") != "embeddings_used,omitempty" {
		t.Fatalf("EmbeddingsUsed json tag = %q, want embeddings_used,omitempty", field.Tag.Get("json"))
	}
}

func TestClusterPropagatesEmbeddingsUsedDegradationFromPython(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	python := writeExecutable(t, workDir, "fake-python.sh", `#!/bin/sh
printf '%s' '{"model":"voyage-large-2-instruct","degradation":{"embeddings_used":true},"clusters":[{"cluster_id":"emb","cluster_label":"Embedding cluster","summary":"embedding-backed","pr_ids":[1],"health_status":"healthy","average_similarity":0.99,"sample_titles":["Embedding-backed cluster"]}]}'
`)

	bridge := NewBridge(Config{Python: python, WorkDir: workDir, Timeout: time.Second})
	_, _, degradation, err := bridge.Cluster(context.Background(), "owner/repo", []types.PR{{Number: 1, Title: "Embedding-backed cluster"}}, "req-cluster-emb")
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}

	embeddingsField := reflect.ValueOf(degradation).FieldByName("EmbeddingsUsed")
	if !embeddingsField.IsValid() {
		t.Fatal("cluster degradation metadata missing EmbeddingsUsed")
	}
	if !embeddingsField.Bool() {
		t.Fatal("cluster degradation metadata lost embeddings_used=true from python payload")
	}
}

func TestDuplicatesPropagatesEmbeddingsUsedDegradationFromPython(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	python := writeExecutable(t, workDir, "fake-python.sh", `#!/bin/sh
printf '%s' '{"degradation":{"embeddings_used":true},"duplicates":[{"canonical_pr_number":1,"duplicate_pr_numbers":[2],"similarity":0.99,"reason":"embedding duplicate"}],"overlaps":[]}'
`)

	bridge := NewBridge(Config{Python: python, WorkDir: workDir, Timeout: time.Second})
	_, _, degradation, err := bridge.Duplicates(context.Background(), "owner/repo", []types.PR{{Number: 1, Title: "A"}, {Number: 2, Title: "B"}}, 0.9, 0.7, "req-dup-emb")
	if err != nil {
		t.Fatalf("duplicates: %v", err)
	}

	embeddingsField := reflect.ValueOf(degradation).FieldByName("EmbeddingsUsed")
	if !embeddingsField.IsValid() {
		t.Fatal("duplicate degradation metadata missing EmbeddingsUsed")
	}
	if !embeddingsField.Bool() {
		t.Fatal("duplicate degradation metadata lost embeddings_used=true from python payload")
	}
}

func TestAnalyzeResultPreservesEmbeddingsUsedInDegradationPayload(t *testing.T) {
	t.Parallel()

	var result AnalyzerResult
	if err := json.Unmarshal([]byte(`{"status":"degraded","degradation":{"embeddings_used":true,"fallback_reason":"backend_unavailable","heuristic_fallback":true},"analyzers":[]}`), &result); err != nil {
		t.Fatalf("unmarshal analyzer result: %v", err)
	}
	if result.Degradation == nil {
		t.Fatal("expected analyzer degradation metadata")
	}

	embeddingsField := reflect.ValueOf(*result.Degradation).FieldByName("EmbeddingsUsed")
	if !embeddingsField.IsValid() {
		t.Fatal("analyzer degradation metadata missing EmbeddingsUsed")
	}
	if !embeddingsField.Bool() {
		t.Fatal("analyzer degradation metadata lost embeddings_used=true from python payload")
	}
}
