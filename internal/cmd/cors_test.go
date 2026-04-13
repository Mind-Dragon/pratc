package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestIsOriginAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		origin   string
		allowed  []string
		expected bool
	}{
		{
			name:     "origin in allowlist",
			origin:   "http://localhost:3000",
			allowed:  []string{"http://localhost:3000"},
			expected: true,
		},
		{
			name:     "origin not in allowlist",
			origin:   "http://evil.com",
			allowed:  []string{"http://localhost:3000"},
			expected: false,
		},
		{
			name:     "empty origin",
			origin:   "",
			allowed:  []string{"http://localhost:3000"},
			expected: false,
		},
		{
			name:     "multiple allowed origins - first match",
			origin:   "http://localhost:3000",
			allowed:  []string{"http://localhost:3000", "http://localhost:8080"},
			expected: true,
		},
		{
			name:     "multiple allowed origins - second match",
			origin:   "http://localhost:8080",
			allowed:  []string{"http://localhost:3000", "http://localhost:8080"},
			expected: true,
		},
		{
			name:     "multiple allowed origins - no match",
			origin:   "http://example.com",
			allowed:  []string{"http://localhost:3000", "http://localhost:8080"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOriginAllowed(tt.origin, tt.allowed)
			if result != tt.expected {
				t.Errorf("isOriginAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowed, result, tt.expected)
			}
		})
	}
}

func TestCorsAllowedOrigins(t *testing.T) {
	t.Parallel()

	// Save original env value
	origEnv := os.Getenv("PRATC_CORS_ALLOWED_ORIGINS")
	defer func() {
		if origEnv == "" {
			os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")
		} else {
			os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", origEnv)
		}
	}()

	tests := []struct {
		name          string
		envValue      string
		expectedLen   int
		expectedFirst string
	}{
		{
			name:          "default when env not set",
			envValue:      "",
			expectedLen:   1,
			expectedFirst: "http://localhost:3000",
		},
		{
			name:          "single origin",
			envValue:      "http://localhost:3000",
			expectedLen:   1,
			expectedFirst: "http://localhost:3000",
		},
		{
			name:          "multiple origins",
			envValue:      "http://localhost:3000,http://localhost:8080",
			expectedLen:   2,
			expectedFirst: "http://localhost:3000",
		},
		{
			name:          "multiple origins with spaces",
			envValue:      "http://localhost:3000, http://localhost:8080 ",
			expectedLen:   2,
			expectedFirst: "http://localhost:3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")
			} else {
				os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", tt.envValue)
			}
			origins := corsAllowedOrigins()
			if len(origins) != tt.expectedLen {
				t.Errorf("corsAllowedOrigins() returned %d origins, want %d", len(origins), tt.expectedLen)
			}
			if tt.expectedLen > 0 && origins[0] != tt.expectedFirst {
				t.Errorf("corsAllowedOrigins()[0] = %q, want %q", origins[0], tt.expectedFirst)
			}
		})
	}
}

func TestCorsMiddlewareAllowedOrigin(t *testing.T) {
	t.Parallel()

	// Set allowed origins for this test
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify CORS headers are present for allowed origin
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

	// Set allowed origins for this test - only localhost:3000 is allowed
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	// Test with a disallowed origin
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.com")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify CORS headers are NOT present for disallowed origin
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", origin)
	}
}

func TestCorsMiddlewareNoOriginHeader(t *testing.T) {
	t.Parallel()

	// Set allowed origins for this test
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	// Test without Origin header
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify CORS headers are NOT present when no Origin header
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin when no Origin header, got %q", origin)
	}
}

func TestCorsMiddlewarePreflightRequestAllowedOrigin(t *testing.T) {
	t.Parallel()

	// Set allowed origins for this test
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS request")
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight, got %d", rr.Code)
	}

	// Verify CORS headers are present on preflight for allowed origin
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

func TestCorsMiddlewarePreflightRequestDisallowedOrigin(t *testing.T) {
	t.Parallel()

	// Set allowed origins for this test
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS request")
	})

	corsHandler := corsMiddleware(handler)

	// Preflight with disallowed origin
	req := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.com")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for preflight, got %d", rr.Code)
	}

	// Verify CORS headers are NOT present for disallowed origin on preflight
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin on preflight, got %q", origin)
	}
}

func TestCorsMiddlewareMultipleAllowedOrigins(t *testing.T) {
	t.Parallel()

	// Set multiple allowed origins for this test
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:8080,https://app.example.com")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	tests := []struct {
		name        string
		origin      string
		expectCORS  bool
	}{
		{"localhost:3000 allowed", "http://localhost:3000", true},
		{"localhost:8080 allowed", "http://localhost:8080", true},
		{"app.example.com allowed", "https://app.example.com", true},
		{"evil.com not allowed", "http://evil.com", false},
		{"other.example.com not allowed", "https://other.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.Header.Set("Origin", tt.origin)
			rr := httptest.NewRecorder()

			corsHandler.ServeHTTP(rr, req)

			origin := rr.Header().Get("Access-Control-Allow-Origin")
			if tt.expectCORS && origin != tt.origin {
				t.Errorf("expected Access-Control-Allow-Origin=%q, got %q", tt.origin, origin)
			}
			if !tt.expectCORS && origin != "" {
				t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", origin)
			}
		})
	}
}

func TestCorsMiddlewarePostRequest(t *testing.T) {
	t.Parallel()

	// Set allowed origins for this test
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/settings", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Fatal("handler should be called for POST request")
	}

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify CORS headers are present for allowed origin
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3000, got %q", origin)
	}
}

func TestCorsMiddlewareDeleteRequest(t *testing.T) {
	t.Parallel()

	// Set allowed origins for this test
	os.Setenv("PRATC_CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("PRATC_CORS_ALLOWED_ORIGINS")

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	corsHandler := corsMiddleware(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Fatal("handler should be called for DELETE request")
	}

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}

	// Verify CORS headers are present for allowed origin
	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3000, got %q", origin)
	}
}
