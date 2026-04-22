package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware401IsStructuredJSON(t *testing.T) {
	t.Setenv("PRATC_API_KEY", "secret-key")

	handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got == "" {
		t.Fatal("expected Content-Type header on unauthorized response")
	}
}
