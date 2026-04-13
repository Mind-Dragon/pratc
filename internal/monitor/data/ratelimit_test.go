package data

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockHTTPClient struct {
	handler func(w http.ResponseWriter, r *http.Request)
	doErr   error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.doErr != nil {
		return nil, m.doErr
	}
	rr := httptest.NewRecorder()
	m.handler(rr, req)
	return rr.Result(), nil
}

func TestRateLimitFetcher_Fetch_Success(t *testing.T) {
	resetTime := time.Now().Add(1 * time.Hour).Unix()
	var callCount int

	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if r.URL.Path != "/rate_limit" {
				t.Errorf("unexpected path: %s", r.URL.Path)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"rate": map[string]any{
					"limit":     5000,
					"remaining": 4500,
					"reset":     resetTime,
					"used":      500,
				},
			})
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()
	view, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if view.Remaining != 4500 {
		t.Errorf("expected Remaining=4500, got %d", view.Remaining)
	}
	if view.Total != 5000 {
		t.Errorf("expected Total=5000, got %d", view.Total)
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

func TestRateLimitFetcher_Cache(t *testing.T) {
	var callCount int

	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"rate": map[string]any{
					"limit":     5000,
					"remaining": 4500,
					"reset":     time.Now().Add(1 * time.Hour).Unix(),
					"used":      500,
				},
			})
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()

	_, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}

	_, err = fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("second fetch (cached) failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

func TestRateLimitFetcher_NoCacheOnError(t *testing.T) {
	var callCount int

	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			callCount++
			http.Error(w, "server error", http.StatusInternalServerError)
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()

	_, err := fetcher.Fetch(ctx)
	if err == nil {
		t.Error("expected error on first fetch")
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

func TestRateLimitFetcher_HTTPError(t *testing.T) {
	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = &mockHTTPClient{
		doErr: errors.New("network error"),
	}

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx)
	if err == nil {
		t.Error("expected error on network failure")
	}
}

func TestRateLimitFetcher_CacheExpiration(t *testing.T) {
	var callCount int

	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"rate": map[string]any{
					"limit":     5000,
					"remaining": 4500 - callCount*100,
					"reset":     time.Now().Add(1 * time.Hour).Unix(),
					"used":      500 + callCount*100,
				},
			})
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()

	view1, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if view1.Remaining != 4400 {
		t.Errorf("expected remaining 4400, got %d", view1.Remaining)
	}

	view2, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if view2.Remaining != 4400 {
		t.Errorf("expected cached value 4400, got %d", view2.Remaining)
	}

	if callCount != 1 {
		t.Errorf("expected 1 API call with cache, got %d", callCount)
	}

	fetcher.cacheUntil = time.Now().Add(-1 * time.Second)

	view3, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch after expiry failed: %v", err)
	}
	if view3.Remaining != 4300 {
		t.Errorf("expected fresh value 4300 after expiry, got %d", view3.Remaining)
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls after expiry, got %d", callCount)
	}
}

func TestRateLimitFetcher_StaleCacheFallback(t *testing.T) {
	var callCount int

	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"rate": map[string]any{
						"limit":     5000,
						"remaining": 4000,
						"reset":     time.Now().Add(1 * time.Hour).Unix(),
						"used":      1000,
					},
				})
			} else {
				http.Error(w, "server error", http.StatusInternalServerError)
			}
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()

	view1, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if view1.Remaining != 4000 {
		t.Errorf("expected remaining 4000, got %d", view1.Remaining)
	}

	fetcher.cacheUntil = time.Now().Add(-1 * time.Second)

	view2, err := fetcher.Fetch(ctx)
	if err == nil {
		t.Error("expected error when fetch fails with stale cache")
	}
	if view2.Remaining != 4000 {
		t.Errorf("expected stale cached value 4000, got %d", view2.Remaining)
	}
}

func TestRateLimitFetcher_InvalidJSON(t *testing.T) {
	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx)
	if err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestRateLimitFetcher_Non200Status(t *testing.T) {
	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx)
	if err == nil {
		t.Error("expected error on non-200 status")
	}
}

func TestRateLimitFetcher_EmptyResponse(t *testing.T) {
	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{}"))
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()
	view, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if view.Remaining != 0 || view.Total != 0 {
		t.Errorf("expected zero values for empty response, got remaining=%d total=%d", view.Remaining, view.Total)
	}
}

func TestRateLimitFetcher_RequestPath(t *testing.T) {
	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rate_limit" {
				t.Errorf("expected path /rate_limit, got %s", r.URL.Path)
			}
			if r.Method != http.MethodGet {
				t.Errorf("expected GET method, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"rate": map[string]any{
					"limit":     5000,
					"remaining": 4000,
					"reset":     time.Now().Add(1 * time.Hour).Unix(),
					"used":      1000,
				},
			})
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()
	fetcher.Fetch(ctx)
}

func TestRateLimitFetcher_ResetTime(t *testing.T) {
	resetTime := time.Now().Add(2 * time.Hour).Unix()

	mockClient := &mockHTTPClient{
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"rate": map[string]any{
					"limit":     5000,
					"remaining": 4000,
					"reset":     resetTime,
					"used":      1000,
				},
			})
		},
	}

	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = mockClient

	ctx := context.Background()
	view, err := fetcher.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	expectedReset := time.Unix(resetTime, 0)
	if !view.ResetTime.Equal(expectedReset) {
		t.Errorf("expected reset time %v, got %v", expectedReset, view.ResetTime)
	}
}
