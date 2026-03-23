package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorsMiddlewareAllowedOrigin(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify CORS headers are present
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3000, got %q", origin)
	}

	methods := rr.Header().Get("Access-Control-Allow-Methods")
	if methods != "GET, POST, DELETE, OPTIONS" {
		t.Errorf("expected Access-Control-Allow-Methods=GET, POST, DELETE, OPTIONS, got %q", methods)
	}

	headers := rr.Header().Get("Access-Control-Allow-Headers")
	if headers != "Content-Type" {
		t.Errorf("expected Access-Control-Allow-Headers=Content-Type, got %q", headers)
	}
}

func TestCorsMiddlewarePreflightRequest(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS request")
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight, got %d", rr.Code)
	}

	// Verify CORS headers are present on preflight
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3000, got %q", origin)
	}

	methods := rr.Header().Get("Access-Control-Allow-Methods")
	if methods != "GET, POST, DELETE, OPTIONS" {
		t.Errorf("expected Access-Control-Allow-Methods=GET, POST, DELETE, OPTIONS, got %q", methods)
	}

	headers := rr.Header().Get("Access-Control-Allow-Headers")
	if headers != "Content-Type" {
		t.Errorf("expected Access-Control-Allow-Headers=Content-Type, got %q", headers)
	}
}

func TestCorsMiddlewareDisallowedOrigin(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	// Test with a disallowed origin - the middleware currently allows all requests
	// but sets the allowed origin header to localhost:3000
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.com")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// The middleware sets the allowed origin header regardless of the request origin
	// This is the current policy - it's a fixed-origin CORS policy
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3000 (fixed policy), got %q", origin)
	}

	// Browsers will reject this response because the origin doesn't match
	// This test documents the current middleware behavior
}

func TestCorsMiddlewarePostRequest(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/settings", nil)
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Fatal("handler should be called for POST request")
	}

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify CORS headers are present
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3000, got %q", origin)
	}
}

func TestCorsMiddlewareDeleteRequest(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings", nil)
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Fatal("handler should be called for DELETE request")
	}

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}

	// Verify CORS headers are present
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3000, got %q", origin)
	}
}
