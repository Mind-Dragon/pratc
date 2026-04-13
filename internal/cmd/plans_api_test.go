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
			var errorResponse map[string]string
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
