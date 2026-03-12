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
								"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "page-2"},
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
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
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
	prs, err := client.FetchPullRequests(context.Background(), "owner/repo", PullRequestListOptions{PerPage: 1})
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
		"url":               fmt.Sprintf("https://github.com/owner/repo/pull/%d", number),
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
