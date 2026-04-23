package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// restPRNode is the GitHub REST API PR response shape.
type restPRNode struct {
	ID             int64  `json:"id"`
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	HTMLURL        string `json:"html_url"`
	Draft          bool   `json:"draft"`
	State          string `json:"state"`
	Locked         bool   `json:"locked"`
	ClosedAt       string `json:"closed_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
	ChangedFiles   int    `json:"changed_files"`
	Mergeable      *bool  `json:"mergeable"`
	MergeableState string `json:"mergeable_state"`

	User struct {
		Login string `json:"login"`
	} `json:"user"`

	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`

	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`

	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// FetchPullRequestsREST fetches open PRs using the GitHub REST API.
// This is a fallback when GraphQL is rate-limited.
func (c *Client) FetchPullRequestsREST(ctx context.Context, repo string, opts PullRequestListOptions) ([]types.PR, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	perPage := opts.PerPage
	if perPage <= 0 || perPage > 100 {
		perPage = 100
	}

	var allPRs []types.PR
	page := 1
	retries := 0
	for {
		url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&sort=updated&direction=asc&per_page=%d&page=%d",
			c.baseURL, owner, name, perPage, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build rest request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying", "url", url, "page", page, "wait_seconds", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return nil, fmt.Errorf("perform rest request: %w", err)
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			c.log.Warn("rest rate limited", "url", url, "body", string(bodyBytes))
			wait := retryAfter(resp.Header.Get("Retry-After"))
			if wait <= 0 {
				wait = untilReset(c.now(), resp.Header.Get("X-RateLimit-Reset"))
			}
			if wait <= 0 {
				wait = 60 * time.Second
			}
			c.log.Warn("rest rate limited", "url", url, "wait_seconds", wait.Seconds())
			c.sleep(wait)

			req.Header.Set("If-None-Match", resp.Header.Get("ETag"))
			resp2, err2 := c.httpClient.Do(req)
			if err2 != nil {
				return nil, fmt.Errorf("retry rest request: %w", err2)
			}
			resp = resp2
		}

		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			errMsg := strings.TrimSpace(string(bodyBytes))
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transient server error, retrying", "url", url, "status", resp.StatusCode, "wait_seconds", wait.Seconds(), "body", errMsg)
				c.sleep(wait)
				retries++
				continue
			}
			return nil, fmt.Errorf("github rest request failed with status %d: %s", resp.StatusCode, errMsg)
		}

		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			errMsg := strings.TrimSpace(string(bodyBytes))
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
				wait := retryAfter(resp.Header.Get("Retry-After"))
				if wait <= 0 {
					wait = untilReset(c.now(), resp.Header.Get("X-RateLimit-Reset"))
				}
				if wait <= 0 {
					wait = 60 * time.Second
				}
				return nil, fmt.Errorf("github rate limit exceeded; retry after %s", wait)
			}
			return nil, fmt.Errorf("github rest request failed with status %d: %s", resp.StatusCode, errMsg)
		}

		remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
		if remaining >= 0 && remaining < c.reserveRequests {
			c.log.Warn("rest api approaching rate limit", "remaining", remaining)
		}

		var prNodes []restPRNode
		if err := json.NewDecoder(resp.Body).Decode(&prNodes); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("decode rest response: %w", err)
		}
		_ = resp.Body.Close()

		if len(prNodes) == 0 {
			break
		}

		for _, node := range prNodes {
			pr := node.toPR(repo)
			if !opts.UpdatedSince.IsZero() {
				updatedAt, parseErr := time.Parse(time.RFC3339, pr.UpdatedAt)
				if parseErr == nil && updatedAt.Before(opts.UpdatedSince) {
					continue
				}
			}
			allPRs = append(allPRs, pr)

			if opts.MaxPRs > 0 && len(allPRs) >= opts.MaxPRs {
				if opts.OnPage != nil && len(allPRs) > 0 {
					_ = opts.OnPage(allPRs, "")
				}
				return allPRs, nil
			}
		}

		// GitHub REST returns empty page when exhausted
		if len(prNodes) < perPage {
			break
		}
		retries = 0
		page++

		// Safety cap on pages
		if page > 50 {
			c.log.Warn("rest fetch capped at 50 pages", "total_prs", len(allPRs))
			break
		}
	}

	if opts.OnPage != nil && len(allPRs) > 0 {
		_ = opts.OnPage(allPRs, "")
	}

	return allPRs, nil
}

// toPR converts a REST API node to a types.PR, marking provenance as REST.
func (n *restPRNode) toPR(repo string) types.PR {
	labels := make([]string, 0, len(n.Labels))
	for _, lbl := range n.Labels {
		labels = append(labels, lbl.Name)
	}

	var reviewStatus, ciStatus, mergeable string
	reviewStatus = "unknown"
	ciStatus = "unknown"
	mergeable = "unknown"

	if n.Mergeable != nil {
		if *n.Mergeable {
			mergeable = "mergeable"
		} else {
			mergeable = "conflicting"
		}
	}

	// Map mergeable_state to ci_status-like categories
	switch n.MergeableState {
	case "clean":
		ciStatus = "success"
	case "blocked":
		ciStatus = "failure"
	case "unstable":
		ciStatus = "error"
	case "pending":
		ciStatus = "pending"
	case "dirty":
		ciStatus = "failure"
	}

	return types.PR{
		ID:                fmt.Sprintf("rest-%d", n.ID),
		Repo:              repo,
		Number:            n.Number,
		Title:             n.Title,
		Body:              n.Body,
		URL:               n.HTMLURL,
		Author:            n.User.Login,
		Labels:            labels,
		FilesChanged:      nil,
		ReviewStatus:      reviewStatus,
		CIStatus:          ciStatus,
		Mergeable:         mergeable,
		BaseBranch:        n.Base.Ref,
		HeadBranch:        n.Head.Ref,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
		IsDraft:           n.Draft,
		Additions:         n.Additions,
		Deletions:         n.Deletions,
		ChangedFilesCount: n.ChangedFiles,
		Provenance: map[string]string{
			"id":                  "rest_api",
			"repo":                "rest_api",
			"title":               "rest_api",
			"body":                "rest_api",
			"url":                 "rest_api",
			"author":              "rest_api",
			"labels":              "rest_api",
			"review_status":       "rest_api",
			"ci_status":           "rest_api",
			"mergeable":           "rest_api",
			"base_branch":         "rest_api",
			"head_branch":         "rest_api",
			"created_at":          "rest_api",
			"updated_at":          "rest_api",
			"is_draft":            "rest_api",
			"additions":           "rest_api",
			"deletions":           "rest_api",
			"changed_files_count": "rest_api",
		},
	}
}
