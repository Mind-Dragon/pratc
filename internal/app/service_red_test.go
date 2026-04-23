package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/ml"
	"github.com/jeffersonnunn/pratc/internal/testutil"
)

func TestClusterSurfacesBackendUnavailableDegradationMetadata(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, MaxPRs: 5})
	service.mlBridge = &ml.Bridge{}

	response, err := service.Cluster(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	if response.Degradation == nil {
		t.Fatal("expected cluster degradation metadata when bridge is unavailable")
	}
	if response.Degradation.FallbackReason != "backend_unavailable" {
		t.Fatalf("cluster fallback_reason = %q, want backend_unavailable", response.Degradation.FallbackReason)
	}
	if !response.Degradation.HeuristicFallback {
		t.Fatal("expected cluster heuristic fallback when bridge is unavailable")
	}
}

func TestAnalyzeSurfacesBackendUnavailableDuplicateDegradationMetadata(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, MaxPRs: 5})
	service.mlBridge = &ml.Bridge{}

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if response.DuplicateDegradation == nil {
		t.Fatal("expected duplicate degradation metadata when bridge is unavailable")
	}
	if response.DuplicateDegradation.FallbackReason != "backend_unavailable" {
		t.Fatalf("duplicate fallback_reason = %q, want backend_unavailable", response.DuplicateDegradation.FallbackReason)
	}
	if !response.DuplicateDegradation.HeuristicFallback {
		t.Fatal("expected duplicate heuristic fallback when bridge is unavailable")
	}
}

func TestClusterSurfacesSubprocessErrorDegradationMetadata(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, MaxPRs: 5})
	service.mlBridge = newFailingMLBridge(t)

	response, err := service.Cluster(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	if response.Degradation == nil {
		t.Fatal("expected cluster degradation metadata when ML subprocess fails")
	}
	if response.Degradation.FallbackReason != "subprocess_error" {
		t.Fatalf("cluster fallback_reason = %q, want subprocess_error", response.Degradation.FallbackReason)
	}
	if !response.Degradation.HeuristicFallback {
		t.Fatal("expected cluster heuristic fallback when ML subprocess fails")
	}
}

func TestAnalyzeSurfacesSubprocessErrorDuplicateDegradationMetadata(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, MaxPRs: 5})
	service.mlBridge = newFailingMLBridge(t)

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if response.DuplicateDegradation == nil {
		t.Fatal("expected duplicate degradation metadata when ML subprocess fails")
	}
	if response.DuplicateDegradation.FallbackReason != "subprocess_error" {
		t.Fatalf("duplicate fallback_reason = %q, want subprocess_error", response.DuplicateDegradation.FallbackReason)
	}
	if !response.DuplicateDegradation.HeuristicFallback {
		t.Fatal("expected duplicate heuristic fallback when ML subprocess fails")
	}
}

func TestAnalyzeSurfacesClusterDegradationMetadataFromLocalBackend(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})
	service.mlBridge = newLocalBackendMLBridge(t)

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	payload := marshalJSONMap(t, response)
	degradation, ok := payload["cluster_degradation"].(map[string]any)
	if !ok {
		t.Fatalf("analysis response missing cluster degradation metadata: %v", payload)
	}
	if degradation["fallback_reason"] != "local_backend" {
		t.Fatalf("cluster degradation fallback_reason = %v, want local_backend", degradation["fallback_reason"])
	}
	if degradation["heuristic_fallback"] != true {
		t.Fatalf("cluster degradation heuristic_fallback = %v, want true", degradation["heuristic_fallback"])
	}
}

func TestAnalyzeSurfacesClusterModelTruthFromLocalBackend(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})
	service.mlBridge = newLocalBackendMLBridge(t)

	response, err := service.Analyze(context.Background(), manifest.Repo)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	payload := marshalJSONMap(t, response)
	if payload["cluster_model"] != "heuristic-fallback" {
		t.Fatalf("analysis cluster_model = %v, want heuristic-fallback", payload["cluster_model"])
	}
}

func newFailingMLBridge(t *testing.T) *ml.Bridge {
	t.Helper()

	workDir := t.TempDir()
	python := filepath.Join(workDir, "fake-python.sh")
	script := `#!/bin/sh
printf '%s' '{"error":"backend_unavailable","message":"bridge subprocess unavailable"}' >&2
exit 1
`
	if err := os.WriteFile(python, []byte(script), 0o755); err != nil {
		t.Fatalf("write failing fake python: %v", err)
	}

	return ml.NewBridge(ml.Config{
		Python:  python,
		WorkDir: workDir,
		Timeout: time.Second,
	})
}
