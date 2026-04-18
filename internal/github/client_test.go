package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPullRequestsQueryIncludesPaginationCursor(t *testing.T) {
	t.Parallel()

	cursor := "cursor-123"
	query, variables := buildPullRequestsQuery("owner", "repo", PullRequestListOptions{PerPage: 50, Cursor: cursor})
	if !strings.Contains(query, "pullRequests") {
		t.Fatalf("expected pullRequests query, got %q", query)
	}
	if variables["cursor"] != cursor {
		t.Fatalf("expected cursor variable %q, got %#v", cursor, variables["cursor"])
	}
}

func TestPullRequestsQueryIncludesUpdatedSince(t *testing.T) {
	t.Parallel()

	since := mustTime(t, "2026-03-12T10:00:00Z")
	_, variables := buildPullRequestsQuery("owner", "repo", PullRequestListOptions{UpdatedSince: since})
	if variables["updatedSince"] != since.Format(time.RFC3339) {
		t.Fatalf("expected updatedSince variable %q, got %#v", since.Format(time.RFC3339), variables["updatedSince"])
	}
}

func TestFetchPullRequestsPaginates(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var cursors []string
	var progressTotals []int
	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			req := decodeGraphQLRequest(t, r)
			mu.Lock()
			if cursor, _ := req.Variables["cursor"].(string); cursor != "" {
				cursors = append(cursors, cursor)
			} else {
				cursors = append(cursors, "")
			}
			mu.Unlock()
			if req.Variables["cursor"] == nil {
				return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequests": map[string]any{
								"totalCount": 2,
								"pageInfo":   map[string]any{"hasNextPage": true, "endCursor": "page-2"},
								"nodes": []map[string]any{
									samplePRNode(101, "First page", "2026-03-12T10:00:00Z"),
								},
							},
						},
					},
				})
			}

			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequests": map[string]any{
							"totalCount": 2,
							"pageInfo":   map[string]any{"hasNextPage": false, "endCursor": nil},
							"nodes": []map[string]any{
								samplePRNode(102, "Second page", "2026-03-12T11:00:00Z"),
							},
						},
					},
				},
			})
		}),
		ReserveRequests: 10,
	})
	prs, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{
		PerPage: 1,
		Progress: func(processed int, total int) {
			mu.Lock()
			progressTotals = append(progressTotals, total)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("fetch pull requests: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 prs, got %d", len(prs))
	}
	if prs[0].Number != 101 || prs[1].Number != 102 {
		t.Fatalf("unexpected prs: %+v", prs)
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "page-2" {
		t.Fatalf("unexpected cursors: %#v", cursors)
	}
	if len(progressTotals) == 0 || progressTotals[0] != 2 {
		t.Fatalf("expected progress total count to be surfaced, got %#v", progressTotals)
	}
}

func TestFetchPullRequestsUsesSnapshotCeilingForProgress(t *testing.T) {
	t.Parallel()

	var progressTotals []int
	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			req := decodeGraphQLRequest(t, r)
			if req.Variables["cursor"] == nil {
				return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequests": map[string]any{
								"totalCount": 2,
								"pageInfo":   map[string]any{"hasNextPage": true, "endCursor": "page-2"},
								"nodes": []map[string]any{
									samplePRNode(101, "First page", "2026-03-12T10:00:00Z"),
								},
							},
						},
					},
				})
			}
			t.Fatal("expected snapshot ceiling to stop before fetching a second page")
			return nil, nil
		}),
		ReserveRequests: 10,
	})

	prs, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{
		PerPage:         1,
		MaxPRs:          1,
		SnapshotCeiling: 1,
		Progress: func(processed int, total int) {
			progressTotals = append(progressTotals, total)
		},
	})
	if err != nil {
		t.Fatalf("fetch pull requests with snapshot ceiling: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected one PR, got %+v", prs)
	}
	if len(progressTotals) == 0 || progressTotals[0] != 1 {
		t.Fatalf("expected progress total to reflect snapshot ceiling, got %#v", progressTotals)
	}
}

func TestFetchPullRequestsStopsWhenOnPageReturnsError(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			req := decodeGraphQLRequest(t, r)
			if req.Variables["cursor"] == nil {
				return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequests": map[string]any{
								"totalCount": 2,
								"pageInfo":   map[string]any{"hasNextPage": true, "endCursor": "page-2"},
								"nodes": []map[string]any{
									samplePRNode(101, "First page", "2026-03-12T10:00:00Z"),
								},
							},
						},
					},
				})
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequests": map[string]any{
							"totalCount": 2,
							"pageInfo":   map[string]any{"hasNextPage": false, "endCursor": nil},
							"nodes": []map[string]any{
								samplePRNode(102, "Second page", "2026-03-12T11:00:00Z"),
							},
						},
					},
				},
			})
		}),
		ReserveRequests: 10,
	})

	called := 0
	_, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{
		PerPage: 1,
		OnPage: func(page []types.PR, cursor string) error {
			called++
			return fmt.Errorf("persist failed at %s", cursor)
		},
	})
	if err == nil {
		t.Fatal("expected fetch to fail when page persistence fails")
	}
	if !strings.Contains(err.Error(), "persist pull request page") {
		t.Fatalf("expected wrapped persistence error, got %v", err)
	}
	if called != 1 {
		t.Fatalf("expected OnPage to run once before aborting, got %d", called)
	}
}

func TestFetchPullRequestsIncludesUpdatedSince(t *testing.T) {
	t.Parallel()

	since := mustTime(t, "2026-03-12T12:00:00Z")
	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			req := decodeGraphQLRequest(t, r)
			if got := req.Variables["updatedSince"]; got != since.Format(time.RFC3339) {
				t.Fatalf("expected updatedSince %q, got %#v", since.Format(time.RFC3339), got)
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequests": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
							"nodes":    []map[string]any{},
						},
					},
				},
			})
		}),
		ReserveRequests: 10,
	})
	if _, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{UpdatedSince: since}); err != nil {
		t.Fatalf("fetch prs with updated since: %v", err)
	}
}

func TestRateLimitBackoff(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]string{
				"Content-Type":          "application/json",
				"X-RateLimit-Remaining": "5",
				"X-RateLimit-Reset":     fmtUnix(time.Now().Add(2 * time.Second)),
			}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequests": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
							"nodes":    []map[string]any{},
						},
					},
				},
			})
		}),
		ReserveRequests: 10,
		Sleep: func(d time.Duration) {
			sleeps = append(sleeps, d)
		},
	})

	if _, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{}); err != nil {
		t.Fatalf("fetch prs: %v", err)
	}
	if len(sleeps) == 0 {
		t.Fatal("expected at least one backoff sleep")
	}
}

func TestRateLimitExceededReturnsDescriptiveError(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Header: http.Header{
					"Retry-After": []string{"1"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":"rate limit exceeded"}`)),
			}, nil
		}),
		MaxSecondaryRetries: 0,
	})

	_, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("expected descriptive rate limit error, got %v", err)
	}
}

func TestRateLimitRetryCeilingEnforced(t *testing.T) {
	t.Parallel()

	var callCount int
	var mu sync.Mutex

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			// Always return 403 to trigger retries
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Header: http.Header{
					"Retry-After": []string{"1"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":"rate limited"}`)),
			}, nil
		}),
		MaxSecondaryRetries: 3,
		Sleep: func(time.Duration) {
			// Skip actual sleep in tests
		},
	})

	_, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{})
	if err == nil {
		t.Fatal("expected error after max retries")
	}

	mu.Lock()
	actualCalls := callCount
	mu.Unlock()

	// Should be called maxSecondaryRetries + 1 times (initial + retries)
	// With REST fallback, adds 2 more calls (initial + 1 retry)
	expectedCalls := 6 // 4 GraphQL (0,1,2,3) + 2 REST (initial + 1 retry)
	if actualCalls != expectedCalls {
		t.Fatalf("expected %d calls (initial + %d retries + REST fallback), got %d", expectedCalls, 3, actualCalls)
	}
}

func TestRateLimitExponentialBackoffWithJitter(t *testing.T) {
	t.Parallel()

	var sleepDurations []time.Duration
	var mu sync.Mutex
	var callCount int

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			callCount++
			attempt := callCount
			mu.Unlock()

			// Return 403 for first 3 attempts, success on 4th
			if attempt < 4 {
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header: http.Header{
						"Retry-After": []string{fmt.Sprintf("%d", attempt)}, // 1s, 2s, 3s
					},
					Body: io.NopCloser(strings.NewReader(`{"message":"rate limited"}`)),
				}, nil
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequests": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
							"nodes":    []map[string]any{},
						},
					},
				},
			})
		}),
		MaxSecondaryRetries: 5,
		Sleep: func(d time.Duration) {
			mu.Lock()
			sleepDurations = append(sleepDurations, d)
			mu.Unlock()
		},
	})

	_, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}

	mu.Lock()
	copies := make([]time.Duration, len(sleepDurations))
	copy(copies, sleepDurations)
	mu.Unlock()

	if len(copies) != 3 {
		t.Fatalf("expected 3 sleep calls (after attempts 0, 1, 2), got %d", len(copies))
	}

	// Verify exponential backoff: each sleep should be >= previous (with some variance for jitter)
	// The Retry-After header specifies 1s, 2s, 3s respectively
	// With jitter, actual sleep = baseDelay + random jitter
	for i := 1; i < len(copies); i++ {
		if copies[i] < copies[i-1] {
			t.Errorf("expected non-decreasing backoff durations, got %v then %v", copies[i-1], copies[i])
		}
	}

	// First sleep should be at least 1 second (from Retry-After: 1)
	if copies[0] < 1*time.Second {
		t.Errorf("expected first backoff >= 1s, got %v", copies[0])
	}
}

func TestReserveBudgetPauseBehavior(t *testing.T) {
	t.Parallel()

	var sleepDurations []time.Duration
	var mu sync.Mutex
	futureReset := time.Now().Add(5 * time.Second).Unix()

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]string{
				"Content-Type":          "application/json",
				"X-RateLimit-Remaining": "5", // Below default reserve of 200
				"X-RateLimit-Reset":     fmt.Sprintf("%d", futureReset),
			}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequests": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
							"nodes":    []map[string]any{},
						},
					},
				},
			})
		}),
		ReserveRequests: 100, // remaining (5) < reserve (100), should trigger pause
		Sleep: func(d time.Duration) {
			mu.Lock()
			sleepDurations = append(sleepDurations, d)
			mu.Unlock()
		},
	})

	_, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	mu.Lock()
	copies := make([]time.Duration, len(sleepDurations))
	copy(copies, sleepDurations)
	mu.Unlock()

	if len(copies) == 0 {
		t.Fatal("expected at least one pause for reserve budget, got none")
	}

	// Should have slept until reset (approximately 5 seconds)
	// Allow some tolerance for test execution time
	if copies[0] < 4*time.Second {
		t.Errorf("expected pause until reset (~5s), got %v", copies[0])
	}
}

func TestRateLimitDeterministicTerminalError(t *testing.T) {
	t.Parallel()

	var callCount int
	var mu sync.Mutex

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Header: http.Header{
					"Retry-After": []string{"2"},
				},
				Body: io.NopCloser(strings.NewReader(`{"message":"secondary rate limit exceeded"}`)),
			}, nil
		}),
		MaxSecondaryRetries: 2,
		Sleep: func(time.Duration) {
			// Skip actual sleep
		},
	})

	_, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{})

	// Verify error is returned and is descriptive
	if err == nil {
		t.Fatal("expected terminal error after max retries")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "rate limit") {
		t.Fatalf("expected 'rate limit' in error message, got %q", errMsg)
	}
	if !strings.Contains(errMsg, "retry after") {
		t.Fatalf("expected 'retry after' in error message, got %q", errMsg)
	}

	// Verify exact number of calls
	mu.Lock()
	actualCalls := callCount
	mu.Unlock()

	// 3 GraphQL attempts (0,1,2) + 2 REST attempts (initial + 1 retry)
	expectedCalls := 5
	if actualCalls != expectedCalls {
		t.Fatalf("expected exactly %d calls, got %d", expectedCalls, actualCalls)
	}
}

func TestRateLimitJitterVariance(t *testing.T) {
	t.Parallel()

	var sleepDurations []time.Duration
	var mu sync.Mutex
	attempts := 0

	// Test that jitter introduces variance by using a fixed base delay
	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Header: http.Header{
						"Retry-After": []string{"2"}, // Fixed 2 second base
					},
					Body: io.NopCloser(strings.NewReader(`{"message":"rate limited"}`)),
				}, nil
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequests": map[string]any{
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
							"nodes":    []map[string]any{},
						},
					},
				},
			})
		}),
		MaxSecondaryRetries: 5,
		Sleep: func(d time.Duration) {
			mu.Lock()
			sleepDurations = append(sleepDurations, d)
			mu.Unlock()
		},
	})

	_, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	mu.Lock()
	copies := make([]time.Duration, len(sleepDurations))
	copy(copies, sleepDurations)
	mu.Unlock()

	if len(copies) < 2 {
		t.Fatalf("expected at least 2 sleep calls, got %d", len(copies))
	}

	// All sleeps should be >= base delay (2s) due to jitter adding positive variance
	for i, d := range copies {
		if d < 2*time.Second {
			t.Errorf("sleep[%d] should be >= 2s (base delay), got %v", i, d)
		}
	}
}

func TestFetchPullRequestFiles(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			req := decodeGraphQLRequest(t, r)
			if !strings.Contains(req.Query, "files") {
				t.Fatalf("expected files query, got %q", req.Query)
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequest": map[string]any{
							"files": map[string]any{
								"nodes": []map[string]any{
									{"path": "internal/github/client.go"},
									{"path": "internal/cache/sqlite.go"},
								},
							},
						},
					},
				},
			})
		}),
	})
	files, err := client.FetchPullRequestFiles(context.Background(), "owner/repo", 101)
	if err != nil {
		t.Fatalf("fetch pull request files: %v", err)
	}
	if len(files) != 2 || files[0] != "internal/github/client.go" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestFetchPullRequestReviews(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequest": map[string]any{
							"reviews": map[string]any{
								"nodes": []map[string]any{
									{"state": "APPROVED", "author": map[string]any{"login": "octocat"}},
									{"state": "COMMENTED", "author": map[string]any{"login": "hubot"}},
								},
							},
						},
					},
				},
			})
		}),
	})
	reviews, err := client.FetchPullRequestReviews(context.Background(), "owner/repo", 101)
	if err != nil {
		t.Fatalf("fetch pull request reviews: %v", err)
	}
	if len(reviews) != 2 || reviews[0].State != "APPROVED" || reviews[0].Author != "octocat" {
		t.Fatalf("unexpected reviews: %+v", reviews)
	}
}

func TestFetchPullRequestCIStatus(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{
		BaseURL: "https://example.test",
		HTTPClient: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(t, http.StatusOK, map[string]string{"Content-Type": "application/json"}, map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"pullRequest": map[string]any{
							"statusCheckRollup": map[string]any{
								"state": "SUCCESS",
							},
						},
					},
				},
			})
		}),
	})
	state, err := client.FetchPullRequestCIStatus(context.Background(), "owner/repo", 101)
	if err != nil {
		t.Fatalf("fetch ci status: %v", err)
	}
	if state != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %q", state)
	}
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func decodeGraphQLRequest(t *testing.T, r *http.Request) graphQLRequest {
	t.Helper()

	var req graphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	return req
}

func jsonResponse(t *testing.T, status int, headers map[string]string, body map[string]any) (*http.Response, error) {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	header := make(http.Header, len(headers))
	for key, value := range headers {
		header.Set(key, value)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(string(payload))),
	}, nil
}

func samplePRNode(number int, title string, updatedAt string) map[string]any {
	return map[string]any{
		"id":                title,
		"number":            number,
		"title":             title,
		"body":              "Body",
		"url":               fmt.Sprintf(types.GitHubURLPrefix+"owner/repo/pull/%d", number),
		"isDraft":           false,
		"createdAt":         "2026-03-12T09:00:00Z",
		"updatedAt":         updatedAt,
		"additions":         10,
		"deletions":         2,
		"changedFiles":      1,
		"mergeable":         "MERGEABLE",
		"baseRefName":       "main",
		"headRefName":       fmt.Sprintf("feature/%d", number),
		"author":            map[string]any{"login": "octocat"},
		"labels":            map[string]any{"nodes": []map[string]any{{"name": "triage"}}},
		"reviewDecision":    "APPROVED",
		"statusCheckRollup": map[string]any{"state": "SUCCESS"},
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}

func fmtUnix(value time.Time) string {
	return strconv.FormatInt(value.Unix(), 10)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}
