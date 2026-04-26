package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMergeSendsMethodTitleMessageAndSHA(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var payload map[string]any
	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPut || r.URL.Path != "/repos/owner/repo/pulls/301/merge" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			mu.Lock()
			defer mu.Unlock()
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{"sha": "merge-sha"})
		}),
	})

	sha, err := client.Merge(context.Background(), "owner/repo", 301, "title", "message", "rebase", "abc123")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if sha != "merge-sha" {
		t.Fatalf("sha = %q", sha)
	}
	mu.Lock()
	defer mu.Unlock()
	if payload["merge_method"] != "rebase" || payload["commit_title"] != "title" || payload["commit_message"] != "message" || payload["sha"] != "abc123" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestMergeRejectsUnsupportedMethod(t *testing.T) {
	t.Parallel()
	client := NewClient(Config{BaseURL: "https://example.test"})
	if _, err := client.Merge(context.Background(), "owner/repo", 1, "", "", "octopus", ""); err == nil || !strings.Contains(err.Error(), "unsupported merge method") {
		t.Fatalf("expected unsupported merge method error, got %v", err)
	}
}

func TestMergeAlreadyMergedConflictIsIdempotent(t *testing.T) {
	t.Parallel()
	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusConflict, map[string]string{"Content-Type": "application/json"}, map[string]any{"message": "Pull Request is already merged"})
		}),
	})
	sha, err := client.Merge(context.Background(), "owner/repo", 301, "", "", "squash", "")
	if err != nil {
		t.Fatalf("already merged conflict should be idempotent: %v", err)
	}
	if sha != "" {
		t.Fatalf("sha = %q, want empty idempotent sha", sha)
	}
}

func TestMergeRetriesTransientServerError(t *testing.T) {
	t.Parallel()
	calls := 0
	var sleeps []time.Duration
	client := NewClient(Config{
		BaseURL:             "https://example.test",
		MaxSecondaryRetries: 1,
		Sleep: func(d time.Duration) {
			sleeps = append(sleeps, d)
		},
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return jsonResponse(t, http.StatusBadGateway, map[string]string{"Content-Type": "application/json"}, map[string]any{"message": "temporary"})
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{"sha": "retry-sha"})
		}),
	})
	sha, err := client.Merge(context.Background(), "owner/repo", 301, "", "", "squash", "")
	if err != nil {
		t.Fatalf("merge retry: %v", err)
	}
	if sha != "retry-sha" || calls != 2 || len(sleeps) != 1 {
		t.Fatalf("sha=%q calls=%d sleeps=%d", sha, calls, len(sleeps))
	}
}

func TestMergeDoesNotRetryNonRetryableClientError(t *testing.T) {
	t.Parallel()
	calls := 0
	client := NewClient(Config{
		BaseURL:             "https://example.test",
		MaxSecondaryRetries: 2,
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return jsonResponse(t, http.StatusUnprocessableEntity, map[string]string{"Content-Type": "application/json"}, map[string]any{"message": "sha does not match"})
		}),
	})
	if _, err := client.Merge(context.Background(), "owner/repo", 301, "", "", "squash", "bad-sha"); err == nil {
		t.Fatal("expected non-retryable 422 error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestCloseCreateCommentAndAddLabelsRetryTransientServerErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		run  func(context.Context, *Client) error
	}{
		{name: "close", run: func(ctx context.Context, c *Client) error { return c.Close(ctx, "owner/repo", 301) }},
		{name: "comment", run: func(ctx context.Context, c *Client) error { return c.CreateComment(ctx, "owner/repo", 301, "body") }},
		{name: "labels", run: func(ctx context.Context, c *Client) error {
			return c.AddLabels(ctx, "owner/repo", 301, []string{"pratc"})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var sleeps []time.Duration
			client := NewClient(Config{
				BaseURL:             "https://example.test",
				MaxSecondaryRetries: 1,
				Sleep: func(d time.Duration) {
					sleeps = append(sleeps, d)
				},
				HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if calls == 1 {
						return jsonResponse(t, http.StatusServiceUnavailable, map[string]string{"Content-Type": "application/json"}, map[string]any{"message": "temporary"})
					}
					return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{"state": "closed", "id": 123})
				}),
			})
			if err := tc.run(context.Background(), client); err != nil {
				t.Fatalf("%s retry: %v", tc.name, err)
			}
			if calls != 2 || len(sleeps) != 1 {
				t.Fatalf("calls=%d sleeps=%d", calls, len(sleeps))
			}
		})
	}
}
