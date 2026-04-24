package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestHandleRepoActionActions(t *testing.T) {
	svc := app.NewService(app.Config{AllowForceCache: true, UseCacheFirst: true, IncludeReview: true})
	req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/actions?policy=advisory&dry_run=true", nil)
	rr := httptest.NewRecorder()

	handleRepoAction(rr, req, svc, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var plan types.ActionPlan
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if plan.Repo != "opencode-ai/opencode" {
		t.Fatalf("repo = %q", plan.Repo)
	}
	if len(plan.WorkItems) == 0 || len(plan.Lanes) == 0 {
		t.Fatalf("expected work items and lanes, got %d/%d", len(plan.WorkItems), len(plan.Lanes))
	}
	for _, intent := range plan.ActionIntents {
		if !intent.DryRun {
			t.Fatalf("advisory intent %s is not dry_run", intent.ID)
		}
	}
}

func TestHandleRepoActionActionsLaneFilter(t *testing.T) {
	svc := app.NewService(app.Config{AllowForceCache: true, UseCacheFirst: true, IncludeReview: true})
	req := httptest.NewRequest(http.MethodGet, "/api/repos/opencode-ai/opencode/actions?lane=focused_review", nil)
	rr := httptest.NewRecorder()

	handleRepoAction(rr, req, svc, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var plan types.ActionPlan
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, item := range plan.WorkItems {
		if item.Lane != types.ActionLaneFocusedReview {
			t.Fatalf("filtered response contains lane %s", item.Lane)
		}
	}
}

func TestHandleRepoActionActionsMethodNotAllowed(t *testing.T) {
	svc := app.NewService(app.Config{})
	req := httptest.NewRequest(http.MethodPost, "/api/repos/opencode-ai/opencode/actions", nil)
	rr := httptest.NewRecorder()

	handleRepoAction(rr, req, svc, nil)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}
