package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSettingsStore struct {
	values map[string]any
	last   setSettingRequest
	err    error
}

func (f *fakeSettingsStore) Get(_ context.Context, _ string) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.values, nil
}

func (f *fakeSettingsStore) Set(_ context.Context, scope, repo, key string, value any) error {
	f.last = setSettingRequest{Scope: scope, Repo: repo, Key: key, Value: value}
	return f.err
}

func (f *fakeSettingsStore) Delete(_ context.Context, _, _, _ string) error {
	return f.err
}

func (f *fakeSettingsStore) ValidateSet(_ context.Context, _, _, _ string, _ any) error {
	return f.err
}

func (f *fakeSettingsStore) ExportYAML(_ context.Context, _, _ string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("max_prs: 1000\n"), nil
}

func (f *fakeSettingsStore) ImportYAML(_ context.Context, _, _ string, _ []byte) error {
	return f.err
}

func TestHandleSettingsGet(t *testing.T) {
	t.Parallel()
	store := &fakeSettingsStore{values: map[string]any{"max_prs": float64(1000)}}
	req := httptest.NewRequest(http.MethodGet, "/api/settings?repo=octo/repo", nil)
	rr := httptest.NewRecorder()

	handleSettings(rr, req, store)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["max_prs"] != float64(1000) {
		t.Fatalf("expected max_prs=1000, got %#v", payload["max_prs"])
	}
}

func TestHandleSettingsPostValidateOnly(t *testing.T) {
	t.Parallel()
	store := &fakeSettingsStore{}
	body := `{"scope":"global","repo":"","key":"max_prs","value":1000}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings?validateOnly=true", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handleSettings(rr, req, store)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "valid") {
		t.Fatalf("expected validation success response, got %s", rr.Body.String())
	}
}

func TestHandleSettingsPostPersists(t *testing.T) {
	t.Parallel()
	store := &fakeSettingsStore{}
	body := `{"scope":"global","repo":"","key":"duplicate_threshold","value":0.92}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handleSettings(rr, req, store)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if store.last.Key != "duplicate_threshold" {
		t.Fatalf("expected duplicate_threshold key, got %q", store.last.Key)
	}
}

func TestHandleExportAndImportSettings(t *testing.T) {
	t.Parallel()
	store := &fakeSettingsStore{}

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings/export?scope=global", nil)
	getRR := httptest.NewRecorder()
	handleExportSettings(getRR, getReq, store)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", getRR.Code)
	}
	if !strings.Contains(getRR.Body.String(), "max_prs") {
		t.Fatalf("expected yaml payload, got %s", getRR.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/settings/import?scope=global", bytes.NewBufferString("max_prs: 1000\n"))
	postRR := httptest.NewRecorder()
	handleImportSettings(postRR, postReq, store)
	if postRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", postRR.Code, postRR.Body.String())
	}
}

func TestHandleSettingsRejectsInvalidScope(t *testing.T) {
	t.Parallel()
	store := &fakeSettingsStore{}

	req := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{"scope":"invalid","repo":"","key":"duplicate_threshold","value":0.92}`))
	rr := httptest.NewRecorder()
	handleSettings(rr, req, store)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid scope, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/settings?scope=invalid&key=max_prs", nil)
	rr = httptest.NewRecorder()
	handleSettings(rr, req, store)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected delete invalid scope to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/settings/export?scope=invalid", nil)
	rr = httptest.NewRecorder()
	handleExportSettings(rr, req, store)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected export invalid scope to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/settings/import?scope=invalid", bytes.NewBufferString("max_prs: 1000\n"))
	rr = httptest.NewRecorder()
	handleImportSettings(rr, req, store)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected import invalid scope to return 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
