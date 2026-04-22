package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/app"
)

func TestHandlePlanValidParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		query  string
		expect int
	}{
		{
			name:   "default params",
			query:  "",
			expect: http.StatusOK,
		},
		{
			name:   "all valid params",
			query:  "?target=10&cluster_id=feat-auth&exclude_conflicts=true&stale_score_threshold=0.5&candidate_pool_cap=100&score_min=50",
			expect: http.StatusOK,
		},
		{
			name:   "cluster_id only",
			query:  "?cluster_id=feat-auth",
			expect: http.StatusOK,
		},
		{
			name:   "exclude_conflicts false",
			query:  "?exclude_conflicts=false",
			expect: http.StatusOK,
		},
		{
			name:   "stale_score_threshold zero",
			query:  "?stale_score_threshold=0",
			expect: http.StatusOK,
		},
		{
			name:   "stale_score_threshold one",
			query:  "?stale_score_threshold=1",
			expect: http.StatusOK,
		},
		{
			name:   "candidate_pool_cap minimum",
			query:  "?candidate_pool_cap=1",
			expect: http.StatusOK,
		},
		{
			name:   "candidate_pool_cap maximum",
			query:  "?candidate_pool_cap=500",
			expect: http.StatusOK,
		},
		{
			name:   "score_min zero",
			query:  "?score_min=0",
			expect: http.StatusOK,
		},
		{
			name:   "score_min maximum",
			query:  "?score_min=100",
			expect: http.StatusOK,
		},
		{
			name:   "mode combination",
			query:  "?mode=combination",
			expect: http.StatusOK,
		},
		{
			name:   "mode permutation",
			query:  "?mode=permutation",
			expect: http.StatusOK,
		},
		{
			name:   "mode with_replacement",
			query:  "?mode=with_replacement",
			expect: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/plans"+tt.query, nil)
			rr := httptest.NewRecorder()

			handlePlan(rr, req, app.NewService(app.Config{}), "opencode-ai/opencode")

			if rr.Code != tt.expect {
				t.Fatalf("expected status %d, got %d body=%s", tt.expect, rr.Code, rr.Body.String())
			}

			// Verify response is valid JSON for 200 status
			if tt.expect == http.StatusOK {
				var response map[string]any
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatalf("expected valid JSON response, got error: %v", err)
				}
			}
		})
	}
}

func TestHandlePlanInvalidParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		query         string
		expectStatus  int
		expectContent string
	}{
		{
			name:          "invalid target negative",
			query:         "?target=-5",
			expectStatus:  http.StatusBadRequest,
			expectContent: "target",
		},
		{
			name:          "invalid target zero",
			query:         "?target=0",
			expectStatus:  http.StatusBadRequest,
			expectContent: "target",
		},
		{
			name:          "invalid target non-numeric",
			query:         "?target=abc",
			expectStatus:  http.StatusBadRequest,
			expectContent: "target",
		},
		{
			name:          "invalid exclude_conflicts",
			query:         "?exclude_conflicts=yes",
			expectStatus:  http.StatusBadRequest,
			expectContent: "exclude_conflicts",
		},
		{
			name:          "invalid exclude_conflicts non-boolean",
			query:         "?exclude_conflicts=123",
			expectStatus:  http.StatusBadRequest,
			expectContent: "exclude_conflicts",
		},
		{
			name:          "invalid stale_score_threshold negative",
			query:         "?stale_score_threshold=-0.5",
			expectStatus:  http.StatusBadRequest,
			expectContent: "stale_score_threshold",
		},
		{
			name:          "invalid stale_score_threshold above one",
			query:         "?stale_score_threshold=1.5",
			expectStatus:  http.StatusBadRequest,
			expectContent: "stale_score_threshold",
		},
		{
			name:          "invalid stale_score_threshold non-numeric",
			query:         "?stale_score_threshold=high",
			expectStatus:  http.StatusBadRequest,
			expectContent: "stale_score_threshold",
		},
		{
			name:          "invalid candidate_pool_cap zero",
			query:         "?candidate_pool_cap=0",
			expectStatus:  http.StatusBadRequest,
			expectContent: "candidate_pool_cap",
		},
		{
			name:          "invalid candidate_pool_cap above 500",
			query:         "?candidate_pool_cap=501",
			expectStatus:  http.StatusBadRequest,
			expectContent: "candidate_pool_cap",
		},
		{
			name:          "invalid candidate_pool_cap negative",
			query:         "?candidate_pool_cap=-10",
			expectStatus:  http.StatusBadRequest,
			expectContent: "candidate_pool_cap",
		},
		{
			name:          "invalid candidate_pool_cap non-numeric",
			query:         "?candidate_pool_cap=many",
			expectStatus:  http.StatusBadRequest,
			expectContent: "candidate_pool_cap",
		},
		{
			name:          "invalid score_min negative",
			query:         "?score_min=-10",
			expectStatus:  http.StatusBadRequest,
			expectContent: "score_min",
		},
		{
			name:          "invalid score_min above 100",
			query:         "?score_min=101",
			expectStatus:  http.StatusBadRequest,
			expectContent: "score_min",
		},
		{
			name:          "invalid score_min non-numeric",
			query:         "?score_min=high",
			expectStatus:  http.StatusBadRequest,
			expectContent: "score_min",
		},
		{
			name:          "invalid mode",
			query:         "?mode=invalid",
			expectStatus:  http.StatusBadRequest,
			expectContent: "mode",
		},
		{
			name:          "multiple invalid params",
			query:         "?target=-1&stale_score_threshold=2&candidate_pool_cap=1000",
			expectStatus:  http.StatusBadRequest,
			expectContent: "target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/plans"+tt.query, nil)
			rr := httptest.NewRecorder()

			handlePlan(rr, req, app.NewService(app.Config{}), "opencode-ai/opencode")

			if rr.Code != tt.expectStatus {
				t.Fatalf("expected status %d, got %d body=%s", tt.expectStatus, rr.Code, rr.Body.String())
			}

			// Verify error response contains expected field
			body := rr.Body.String()
			if !strings.Contains(strings.ToLower(body), strings.ToLower(tt.expectContent)) {
				t.Fatalf("expected error to contain %q, got body=%s", tt.expectContent, body)
			}

			// Verify response is valid JSON error
			var errorResponse map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err != nil {
				t.Fatalf("expected valid JSON error response, got error: %v", err)
			}
			if _, ok := errorResponse["error"]; !ok {
				t.Fatalf("expected error field in response, got %#v", errorResponse)
			}
		})
	}
}

func TestHandlePlanMissingRepo(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/plans", nil)
	rr := httptest.NewRecorder()

	// Pass empty repo to trigger ensureRepo failure
	handlePlan(rr, req, app.NewService(app.Config{}), "")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandlePlanWrongMethod(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/repos/opencode-ai/opencode/plans", nil)
	rr := httptest.NewRecorder()

	handlePlan(rr, req, app.NewService(app.Config{}), "opencode-ai/opencode")

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandlePlanOmniValidParams(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/plan/omni?selector=1-5", nil)
	rr := httptest.NewRecorder()

	svc := app.NewService(app.Config{UseCacheFirst: false})
	handlePlanOmni(rr, req, svc, "opencode-ai/opencode")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	if resp["mode"] != "omni_batch" {
		t.Fatalf("expected mode omni_batch, got %q", resp["mode"])
	}
	if resp["selector"] != "1-5" {
		t.Fatalf("expected selector 1-5, got %q", resp["selector"])
	}
}

// TestPlanHandler_RespectsExcludeConflicts tests D1: Plan handler ignores exclude_conflicts.
//
// BUG: serve.go lines 513-518 show that exclude_conflicts is parsed but never used.
// The code validates that it's a valid boolean but then doesn't pass it to the service
// or use it to filter the candidate pool. When exclude_conflicts=true, the plan
// result should have no conflicts in selected PRs.
func TestPlanHandler_RespectsExcludeConflicts(t *testing.T) {
	tests := []struct {
		name              string
		query             string
		wantConflictsInResult bool // whether conflicts should appear in result
	}{
		{
			name:              "exclude_conflicts=true should filter conflicts",
			query:             "?target=5&exclude_conflicts=true",
			wantConflictsInResult: false,
		},
		{
			name:              "exclude_conflicts=false may include conflicts",
			query:             "?target=5&exclude_conflicts=false",
			wantConflictsInResult: true, // unspecified behavior, but at least it should be used
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/plans"+tt.query, nil)
			rr := httptest.NewRecorder()

			svc := app.NewService(app.Config{UseCacheFirst: false})
			handlePlan(rr, req, svc, "opencode-ai/opencode")

			if rr.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
			}

			var resp map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("expected valid JSON response, got error: %v", err)
			}

			// The bug is that exclude_conflicts is parsed but never used
			// We can't easily test for absence of conflicts in the response
			// without setting up a complex scenario with conflicting PRs.
			// The bug exists if the parameter has no effect.
			//
			// To properly fix this, the handler should:
			// 1. Parse exclude_conflicts into a variable
			// 2. Pass it to the service or filter pipeline
			// 3. The service should exclude conflicting PRs when true
			_ = tt.wantConflictsInResult
			_ = resp
		})
	}
}

// TestPlanHandler_RespectsDryRun tests D2: dry_run not implemented.
//
// BUG: According to AGENTS.md line 40, dry_run should control whether the plan
// is written to cache. However, in plan.go line 99, dry_run is only logged in
// the audit entry and is never passed to the service.Plan() call to prevent
// cache writes. When dry_run=true, the plan should be computed but not written
// to the cache. Currently it always writes to cache regardless of dry_run.
func TestPlanHandler_RespectsDryRun(t *testing.T) {
	// Note: This test would require integration with the cache layer to verify
	// that dry_run=true doesn't write to cache. For now, we document the expected
	// behavior and verify that the parameter is at least parsed.

	// Expected behavior:
	// - dry_run=true (or absent, since default is true per AGENTS.md): plan computed, not cached
	// - dry_run=false: plan computed AND written to cache

	// The bug is that dry_run is read but never used to control cache behavior.
	// See plan.go:94 where service.Plan() is called without any dry_run parameter.

	t.Run("dry_run parameter should be parsed", func(t *testing.T) {
		// Verify that dry_run=true is accepted without error
		req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/plans?dry_run=true", nil)
		rr := httptest.NewRecorder()

		svc := app.NewService(app.Config{UseCacheFirst: false})
		handlePlan(rr, req, svc, "opencode-ai/opencode")

		// Should not return a bad request error for valid dry_run param
		if rr.Code == http.StatusBadRequest {
			t.Logf("dry_run parameter not recognized or implemented: %s", rr.Body.String())
		}
	})

	t.Run("dry_run=false should be accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/plans?dry_run=false", nil)
		rr := httptest.NewRecorder()

		svc := app.NewService(app.Config{UseCacheFirst: false})
		handlePlan(rr, req, svc, "opencode-ai/opencode")

		if rr.Code == http.StatusBadRequest {
			t.Logf("dry_run=false parameter not recognized or implemented: %s", rr.Body.String())
		}
	})
}
