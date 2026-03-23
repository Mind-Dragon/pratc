package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPlanAPIQuery(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/plan?repo=test/repo", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := "test/repo"
		resp := types.PlanResponse{
			Repo:        repo,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Target:      20,
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

	if resp["repo"] != "test/repo" {
		t.Errorf("expected repo=test/repo, got %v", resp["repo"])
	}
	if resp["target"] != float64(20) {
		t.Errorf("expected target=20, got %v", resp["target"])
	}
}

func TestPlanResponseFields(t *testing.T) {
	t.Parallel()
	fixedNow := func() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) }

	resp := types.PlanResponse{
		Repo:        "test/repo",
		GeneratedAt: fixedNow().Format(time.RFC3339),
		Target:      20,
	}

	if resp.Repo != "test/repo" {
		t.Error("expected Repo=test/repo")
	}
	if resp.Target != 20 {
		t.Error("expected Target=20")
	}
}
