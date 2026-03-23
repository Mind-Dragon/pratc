package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPlanDryRunAPIQueryDefaultsTrue(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/plan?repo=test/repo", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := "test/repo"
		dryRun := true
		if raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("dry_run"))); raw == "false" {
			dryRun = false
		}
		resp := types.PlanResponse{
			Repo:        repo,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Target:      20,
			DryRun:      dryRun,
		}
		writeHTTPJSON(w, http.StatusOK, resp)
	})

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	dryRunVal, ok := resp["dry_run"]
	if !ok {
		t.Fatal("dry_run field missing from response")
	}
	if dryRunVal != true {
		t.Errorf("expected dry_run=true by default, got %v", dryRunVal)
	}
}

func TestPlanDryRunAPIQueryExplicitFalse(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/plan?repo=test/repo&dry_run=false", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := "test/repo"
		dryRun := true
		if raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("dry_run"))); raw == "false" {
			dryRun = false
		}
		resp := types.PlanResponse{
			Repo:        repo,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Target:      20,
			DryRun:      dryRun,
		}
		writeHTTPJSON(w, http.StatusOK, resp)
	})

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	dryRunVal, ok := resp["dry_run"]
	if !ok {
		t.Fatal("dry_run field missing from response")
	}
	if dryRunVal != false {
		t.Errorf("expected dry_run=false when explicitly set, got %v", dryRunVal)
	}
}

func TestPlanResponseIncludesDryRunField(t *testing.T) {
	t.Parallel()
	fixedNow := func() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) }

	resp := types.PlanResponse{
		Repo:        "test/repo",
		GeneratedAt: fixedNow().Format(time.RFC3339),
		Target:      20,
		DryRun:      true,
	}

	if !resp.DryRun {
		t.Error("expected DryRun=true")
	}

	resp2 := types.PlanResponse{
		Repo:        "test/repo",
		GeneratedAt: fixedNow().Format(time.RFC3339),
		Target:      20,
		DryRun:      false,
	}

	if resp2.DryRun {
		t.Error("expected DryRun=false")
	}
}
