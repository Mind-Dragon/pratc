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
					"limit":      5000,
					"remaining":  4500,
					"reset":      resetTime,
					"used":       500,
				},
			})
		},
	}

	fetcher := NewRateLimitFetcher(nil)
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
					"limit":      5000,
					"remaining":  4500,
					"reset":      time.Now().Add(1 * time.Hour).Unix(),
					"used":       500,
				},
			})
		},
	}

	fetcher := NewRateLimitFetcher(nil)
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

	fetcher := NewRateLimitFetcher(nil)
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
	fetcher := NewRateLimitFetcher(nil)
	fetcher.httpClient = &mockHTTPClient{
		doErr: errors.New("network error"),
	}

	ctx := context.Background()
	_, err := fetcher.Fetch(ctx)
	if err == nil {
		t.Error("expected error on network failure")
	}
}
