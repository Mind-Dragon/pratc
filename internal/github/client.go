package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Config struct {
	BaseURL             string
	HTTPClient          HTTPClient
	Token               string
	TokenSource         TokenSource
	ReserveRequests     int
	MaxSecondaryRetries int
	Now                 func() time.Time
	Sleep               func(time.Duration)
	Logger              *logger.Logger
	BudgetManager       *ratelimit.BudgetManager
}

type Client struct {
	baseURL             string
	httpClient          HTTPClient
	token               string // kept for backward compat; prefer tokenSource
	tokenSource         TokenSource
	reserveRequests     int
	maxSecondaryRetries int
	now                 func() time.Time
	sleep               func(time.Duration)
	log                 *logger.Logger
	budget              *ratelimit.BudgetManager
	etagCache           ETagCache
}

type RateLimitStatus struct {
	Remaining int
	ResetAt   time.Time
}

// commentDTO is used for comment payloads.
type commentDTO struct {
	Body string `json:"body"`
}


type PullRequestListOptions struct {
	PerPage         int
	Cursor          string
	UpdatedSince    time.Time
	MaxPRs          int
	SnapshotCeiling int
	Progress        func(processed int, total int)
	OnCursor        func(cursor string, processed int)
	OnPage          func(page []types.PR, cursor string) error
	Concurrency     int
}

type Review struct {
	State  string
	Author string
}

func intFromEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		maxIdleConns := 100
		if v := intFromEnv("PRATC_HTTP_MAX_IDLE", 0); v > 0 {
			maxIdleConns = v
		}
		maxIdleConnsPerHost := 10
		if v := intFromEnv("PRATC_HTTP_MAX_IDLE_PER_HOST", 0); v > 0 {
			maxIdleConnsPerHost = v
		}
		idleConnTimeout := 90 * time.Second
		if v := intFromEnv("PRATC_HTTP_IDLE_TIMEOUT", 0); v > 0 {
			idleConnTimeout = time.Duration(v) * time.Second
		}
		requestTimeout := 30 * time.Second
		if v := intFromEnv("PRATC_HTTP_TIMEOUT", 0); v > 0 {
			requestTimeout = time.Duration(v) * time.Second
		}

		transport := &http.Transport{
			MaxIdleConns:        maxIdleConns,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
			IdleConnTimeout:     idleConnTimeout,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
		}
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	sleep := cfg.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	reserve := cfg.ReserveRequests
	if reserve <= 0 {
		reserve = 200
	}
	maxRetries := cfg.MaxSecondaryRetries
	if maxRetries == 0 {
		maxRetries = 8
	}

	log := cfg.Logger
	if log == nil {
		log = logger.New("github")
	}

	tokenSrc := cfg.TokenSource
	if tokenSrc == nil {
		tokenSrc = &singleTokenSource{token: cfg.Token}
	}

	return &Client{
		baseURL:             baseURL,
		httpClient:          httpClient,
		token:               cfg.Token,
		tokenSource:         tokenSrc,
		reserveRequests:     reserve,
		maxSecondaryRetries: maxRetries,
		now:                 now,
		sleep:               sleep,
		log:                 log,
		budget:              cfg.BudgetManager,
	}
}

func (c *Client) FetchPullRequests(ctx context.Context, repo string, opts PullRequestListOptions) ([]types.PR, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	var prs []types.PR
	cursor := opts.Cursor

	for {
		query, variables := buildPullRequestsQuery(owner, name, PullRequestListOptions{
			PerPage:      opts.PerPage,
			Cursor:       cursor,
			UpdatedSince: opts.UpdatedSince,
		})

		var response struct {
			Data struct {
				Repository struct {
					PullRequests struct {
						TotalCount int `json:"totalCount"`
						PageInfo   struct {
							HasNextPage bool    `json:"hasNextPage"`
							EndCursor   *string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []pullRequestNode `json:"nodes"`
					} `json:"pullRequests"`
				} `json:"repository"`
			} `json:"data"`
		}

		if err := c.graphQL(ctx, query, variables, &response); err != nil {
			// If GraphQL fails with rate limit, fall back to REST
			if isRateLimitError(err) {
				c.log.Warn("graphQL rate limited, falling back to REST", "repo", repo, "err", err.Error())
				return c.FetchPullRequestsREST(ctx, repo, opts)
			}
			return nil, err
		}

		totalCount := response.Data.Repository.PullRequests.TotalCount
		snapshotCeiling := opts.SnapshotCeiling
		if snapshotCeiling <= 0 && totalCount > 0 {
			snapshotCeiling = totalCount
		}
		effectiveMax := opts.MaxPRs
		if snapshotCeiling > 0 && (effectiveMax <= 0 || snapshotCeiling < effectiveMax) {
			effectiveMax = snapshotCeiling
		}
		pagePRs := make([]types.PR, 0, len(response.Data.Repository.PullRequests.Nodes))
		for _, node := range response.Data.Repository.PullRequests.Nodes {
			pr := node.toPR(repo)
			if !opts.UpdatedSince.IsZero() {
				updatedAt, parseErr := time.Parse(time.RFC3339, pr.UpdatedAt)
				if parseErr == nil && updatedAt.Before(opts.UpdatedSince) {
					continue
				}
			}
			prs = append(prs, pr)
			pagePRs = append(pagePRs, pr)
			if opts.Progress != nil {
				progressTotal := totalCount
				if effectiveMax > 0 && effectiveMax < progressTotal {
					progressTotal = effectiveMax
				}
				opts.Progress(len(prs), progressTotal)
			}
			if effectiveMax > 0 && len(prs) >= effectiveMax {
				break
			}
		}
		if opts.OnPage != nil && len(pagePRs) > 0 {
			cursorCopy := cursor
			if response.Data.Repository.PullRequests.PageInfo.EndCursor != nil {
				cursorCopy = *response.Data.Repository.PullRequests.PageInfo.EndCursor
			}
			if err := opts.OnPage(pagePRs, cursorCopy); err != nil {
				return nil, fmt.Errorf("persist pull request page: %w", err)
			}
		}

		if (effectiveMax > 0 && len(prs) >= effectiveMax) || !response.Data.Repository.PullRequests.PageInfo.HasNextPage || response.Data.Repository.PullRequests.PageInfo.EndCursor == nil {
			break
		}
		cursor = *response.Data.Repository.PullRequests.PageInfo.EndCursor
		if opts.OnCursor != nil {
			opts.OnCursor(cursor, len(prs))
		}
	}

	return prs, nil
}

func (c *Client) FetchPullRequestFiles(ctx context.Context, repo string, number int) ([]string, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}
	query, variables := buildFilesQuery(owner, name, number)

	var response struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					Files struct {
						Nodes []struct {
							Path string `json:"path"`
						} `json:"nodes"`
					} `json:"files"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := c.graphQL(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	files := make([]string, 0, len(response.Data.Repository.PullRequest.Files.Nodes))
	for _, node := range response.Data.Repository.PullRequest.Files.Nodes {
		files = append(files, node.Path)
	}
	return files, nil
}

func (c *Client) FetchPullRequestReviews(ctx context.Context, repo string, number int) ([]Review, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}
	query, variables := buildReviewsQuery(owner, name, number)

	var response struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					Reviews struct {
						Nodes []struct {
							State  string `json:"state"`
							Author struct {
								Login string `json:"login"`
							} `json:"author"`
						} `json:"nodes"`
					} `json:"reviews"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := c.graphQL(ctx, query, variables, &response); err != nil {
		return nil, err
	}

	reviews := make([]Review, 0, len(response.Data.Repository.PullRequest.Reviews.Nodes))
	for _, node := range response.Data.Repository.PullRequest.Reviews.Nodes {
		reviews = append(reviews, Review{State: node.State, Author: node.Author.Login})
	}
	return reviews, nil
}

func (c *Client) FetchPullRequestCIStatus(ctx context.Context, repo string, number int) (string, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return "", err
	}
	query, variables := buildCIStatusQuery(owner, name, number)

	var response struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					StatusCheckRollup struct {
						State string `json:"state"`
					} `json:"statusCheckRollup"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := c.graphQL(ctx, query, variables, &response); err != nil {
		return "", err
	}

	return response.Data.Repository.PullRequest.StatusCheckRollup.State, nil
}

func (c *Client) RateLimitStatus() (RateLimitStatus, error) {
	if c.budget == nil {
		return RateLimitStatus{}, fmt.Errorf("rate limit budget manager is required")
	}
	return RateLimitStatus{
		Remaining: c.budget.Remaining(),
		ResetAt:   c.budget.ResetAt(),
	}, nil
}

// OpenPRCountResult holds the result of fetching open PR count.
type OpenPRCountResult struct {
	Count      int
	RateLimit  RateLimitStatus
	TotalCount int // same as Count, kept for API compatibility
}

// FetchOpenPRCount fetches the total count of open pull requests using GraphQL.
// It returns the count along with current rate limit status.
// This is a lightweight query that only fetches the totalCount, not any PR data.
func (c *Client) FetchOpenPRCount(ctx context.Context, repo string) (OpenPRCountResult, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return OpenPRCountResult{}, err
	}

	query, variables := buildOpenPRCountQuery(owner, name)

	var response struct {
		Data struct {
			Repository struct {
				PullRequests struct {
					TotalCount int `json:"totalCount"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := c.graphQL(ctx, query, variables, &response); err != nil {
		return OpenPRCountResult{}, err
	}

	// Get rate limit status from budget if available
	rlStatus := RateLimitStatus{}
	if c.budget != nil {
		rlStatus = RateLimitStatus{
			Remaining: c.budget.Remaining(),
			ResetAt:   c.budget.ResetAt(),
		}
	}

	return OpenPRCountResult{
		Count:      response.Data.Repository.PullRequests.TotalCount,
		RateLimit:  rlStatus,
		TotalCount: response.Data.Repository.PullRequests.TotalCount,
	}, nil
}

// FetchOpenPRCountWithToken is a standalone function that creates a temporary client
// with the given token and fetches the open PR count. This is useful for
// preflight checks that need to use token fallback.
func FetchOpenPRCountWithToken(ctx context.Context, token, repo string) (OpenPRCountResult, error) {
	client := NewClient(Config{Token: token})
	return client.FetchOpenPRCount(ctx, repo)
}

type PRFilesResult struct {
	PRNumber int
	Files    []string
	Err      error
}

type PRReviewsResult struct {
	PRNumber int
	Reviews  []Review
	Err      error
}

func (c *Client) FetchPullRequestFilesBatch(ctx context.Context, repo string, prNumbers []int, concurrency int) []PRFilesResult {
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > 20 {
		concurrency = 20
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	results := make([]PRFilesResult, len(prNumbers))

	for i, num := range prNumbers {
		wg.Add(1)
		go func(idx int, prNum int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			files, err := c.FetchPullRequestFiles(ctx, repo, prNum)
			results[idx] = PRFilesResult{PRNumber: prNum, Files: files, Err: err}
		}(i, num)
	}

	wg.Wait()
	return results
}

func (c *Client) FetchPullRequestReviewsBatch(ctx context.Context, repo string, prNumbers []int, concurrency int) []PRReviewsResult {
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > 20 {
		concurrency = 20
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	results := make([]PRReviewsResult, len(prNumbers))

	for i, num := range prNumbers {
		wg.Add(1)
		go func(idx int, prNum int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			reviews, err := c.FetchPullRequestReviews(ctx, repo, prNum)
			results[idx] = PRReviewsResult{PRNumber: prNum, Reviews: reviews, Err: err}
		}(i, num)
	}

	wg.Wait()
	return results
}

func (c *Client) graphQL(ctx context.Context, query string, variables map[string]any, dest any) error {
	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal graphql payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxSecondaryRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/graphql", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build graphql request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && attempt < c.maxSecondaryRetries {
				wait := transientBackoff(attempt)
				c.log.Warn("graphql transport error, retrying", "attempt", attempt+1, "max_retries", c.maxSecondaryRetries, "wait_seconds", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				lastErr = fmt.Errorf("perform graphql request: %w", err)
				continue
			}
			return fmt.Errorf("perform graphql request: %w", err)
		}

		retry, retryErr := c.handleRateLimit(resp, attempt)
		if retryErr != nil {
			_ = resp.Body.Close()
			return retryErr
		}
		if retry {
			_ = resp.Body.Close()
			continue
		}

		if isTransientHTTPStatus(resp.StatusCode) {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("github graphql request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
			if attempt < c.maxSecondaryRetries {
				wait := transientBackoff(attempt)
				c.log.Warn("graphql transient server error, retrying", "attempt", attempt+1, "max_retries", c.maxSecondaryRetries, "status", resp.StatusCode, "wait_seconds", wait.Seconds())
				c.sleep(wait)
				continue
			}
			return lastErr
		}

		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("github graphql request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
			return lastErr
		}

		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			_ = resp.Body.Close()
			return fmt.Errorf("decode graphql response: %w", err)
		}
		if err := resp.Body.Close(); err != nil {
			return fmt.Errorf("close graphql response body: %w", err)
		}
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("github graphql request failed after retries")
}

// isRateLimitError returns true if the error is a GitHub rate limit error.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "rate limit exceeded") ||
		strings.Contains(errStr, "rate limit exhausted") ||
		strings.Contains(errStr, "rate limit")
}

func addJitter(d time.Duration) time.Duration {
	return d + time.Duration(rand.Int63n(int64(d/4)))
}

func transientBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	base := time.Second << attempt
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	wait := addJitter(base)
	if wait > 30*time.Second {
		wait = 30 * time.Second
	}
	return wait
}

func isTransientTransportError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporarily") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe")
}

func isTransientHTTPStatus(status int) bool {
	switch status {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (c *Client) handleRateLimit(resp *http.Response, attempt int) (bool, error) {
	if resp.StatusCode == http.StatusForbidden {
		wait := retryAfter(resp.Header.Get("Retry-After"))
		if wait <= 0 {
			wait = untilReset(c.now(), resp.Header.Get("X-RateLimit-Reset"))
		}
		if wait <= 0 {
			wait = 2 * time.Second
		}

		resetEpoch, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)

		if attempt >= c.maxSecondaryRetries {
			c.log.Error("secondary rate limit exhausted", "retry_after", wait.Seconds(), "reset_epoch", resetEpoch, "attempt", attempt, "max_retries", c.maxSecondaryRetries)
			return false, fmt.Errorf("github rate limit exceeded; retry after %s", wait)
		}

		c.log.Info("secondary rate limit hit, retrying", "retry_after", wait.Seconds(), "reset_epoch", resetEpoch, "attempt", attempt+1, "max_retries", c.maxSecondaryRetries)
		c.sleep(addJitter(wait))
		return true, nil
	}

	remaining, err := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	if err == nil {
		c.log.Info("rate limit status checked", "remaining", remaining)

		if remaining < c.reserveRequests {
			if wait := untilReset(c.now(), resp.Header.Get("X-RateLimit-Reset")); wait > 0 {
				resetEpoch, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
				c.log.Error("rate limit exhausted, pausing until reset", "remaining", remaining, "reserve_requests", c.reserveRequests, "reset_epoch", resetEpoch, "duration_ms", wait.Milliseconds())
				c.sleep(addJitter(wait))
			}
		}
		if c.budget != nil && remaining >= 0 {
			resetEpoch, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
			c.budget.RecordResponse(remaining, resetEpoch)
		}
	}

	return false, nil
}

func splitRepo(repo string) (string, string, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}

func retryAfter(raw string) time.Duration {
	if raw == "" {
		return 0
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func untilReset(now time.Time, raw string) time.Duration {
	if raw == "" {
		return 0
	}
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	resetAt := time.Unix(epoch, 0)
	if !resetAt.After(now) {
		return 0
	}
	return resetAt.Sub(now)
}

type pullRequestNode struct {
	ID                string `json:"id"`
	Number            int    `json:"number"`
	Title             string `json:"title"`
	Body              string `json:"body"`
	URL               string `json:"url"`
	IsDraft           bool   `json:"isDraft"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
	Additions         int    `json:"additions"`
	Deletions         int    `json:"deletions"`
	ChangedFiles      int    `json:"changedFiles"`
	Mergeable         string `json:"mergeable"`
	BaseRefName       string `json:"baseRefName"`
	HeadRefName       string `json:"headRefName"`
	ReviewDecision    string `json:"reviewDecision"`
	StatusCheckRollup struct {
		State string `json:"state"`
	} `json:"statusCheckRollup"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
}

func (n pullRequestNode) toPR(repo string) types.PR {
	labels := make([]string, 0, len(n.Labels.Nodes))
	for _, label := range n.Labels.Nodes {
		labels = append(labels, label.Name)
	}

	return types.PR{
		ID:                n.ID,
		Repo:              repo,
		Number:            n.Number,
		Title:             n.Title,
		Body:              n.Body,
		URL:               n.URL,
		Author:            n.Author.Login,
		Labels:            labels,
		FilesChanged:      nil,
		ReviewStatus:      strings.ToLower(n.ReviewDecision),
		CIStatus:          strings.ToLower(n.StatusCheckRollup.State),
		Mergeable:         strings.ToLower(n.Mergeable),
		BaseBranch:        n.BaseRefName,
		HeadBranch:        n.HeadRefName,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
		IsDraft:           n.IsDraft,
		Additions:         n.Additions,
		Deletions:         n.Deletions,
		ChangedFilesCount: n.ChangedFiles,
		Provenance: map[string]string{
			"id":                  "live_api",
			"repo":                "live_api",
			"title":               "live_api",
			"body":                "live_api",
			"url":                 "live_api",
			"author":              "live_api",
			"labels":              "live_api",
			"review_status":       "live_api",
			"ci_status":           "live_api",
			"mergeable":           "live_api",
			"base_branch":         "live_api",
			"head_branch":         "live_api",
			"created_at":          "live_api",
			"updated_at":          "live_api",
			"is_draft":            "live_api",
			"additions":           "live_api",
			"deletions":           "live_api",
			"changed_files_count": "live_api",
		},
	}
}

// GetPR fetches a single pull request via REST.
func (c *Client) GetPR(ctx context.Context, repo string, number int) (*restPRNode, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.baseURL, owner, name, number)

	var retries int
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build get pr request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying", "url", url, "wait_seconds", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return nil, fmt.Errorf("perform rest request: %w", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("PR #%d not found", number)
		}
		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transient error", "url", url, "status", resp.StatusCode, "wait", wait.Seconds())
				c.sleep(wait)
				retries++
				continue
			}
			return nil, fmt.Errorf("github error %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("github error %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

	var pr restPRNode
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("decode pr: %w", err)
	}
	_ = resp.Body.Close()

	// Enrich: HeadSHA from Head.SHA
	pr.HeadSHA = pr.Head.SHA

	// Enrich: CI status via GraphQL
	ciStatus, err := c.FetchPullRequestCIStatus(ctx, repo, number)
	if err != nil {
		c.log.Warn("failed to fetch CI status", "repo", repo, "pr", number, "error", err.Error())
		pr.CIStatus = "unknown"
	} else {
		pr.CIStatus = ciStatus
	}

	// Enrich: RequiredReviews satisfaction (1 if satisfied, 0 otherwise)
	// Compute: required count from branch protection; approvals from reviews
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	} // already have from above
	requiredCount, err := c.getBranchProtectionRequiredCount(ctx, owner, name, pr.Base.Ref)
	if err != nil {
		c.log.Warn("failed to fetch branch protection", "repo", repo, "branch", pr.Base.Ref, "error", err.Error())
		// If we can't determine, assume satisfied to avoid blocking
		pr.RequiredReviews = 1
	} else if requiredCount == 0 {
		pr.RequiredReviews = 1
	} else {
		reviews, err := c.FetchPullRequestReviews(ctx, repo, number)
		if err != nil {
			c.log.Warn("failed to fetch reviews", "error", err.Error())
			pr.RequiredReviews = 0
		} else {
			approved := 0
			for _, r := range reviews {
				if r.State == "APPROVED" {
					approved++
				}
			}
			if approved >= requiredCount {
				pr.RequiredReviews = 1
			} else {
				pr.RequiredReviews = 0
			}
		}
	}

	return &pr, nil
	}
}

// Merge merges a pull request via the GitHub REST API.
func (c *Client) Merge(ctx context.Context, repo string, prNumber int, commitTitle, commitMessage, mergeMethod string) (string, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", c.baseURL, owner, name, prNumber)

	payload := map[string]any{
		"commit_title":  commitTitle,
		"commit_message": commitMessage,
		"merge_method":  mergeMethod,
	}
	if commitTitle == "" {
		payload["commit_title"] = "Merge pull request"
	}
	if mergeMethod == "" {
		payload["merge_method"] = "squash"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal merge payload: %w", err)
	}

	var retries int
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("build merge request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("Content-Type", "application/json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying merge", "url", url, "wait", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return "", fmt.Errorf("perform merge request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusConflict {
			// Already merged or not mergeable — read body for detail
			bodyBytes, _ := io.ReadAll(resp.Body)
			msg := strings.ToLower(string(bodyBytes))
			if strings.Contains(msg, "already merged") || strings.Contains(msg, "no commits between") {
				return "", nil // treat as success (no-op)
			}
			return "", fmt.Errorf("merge conflict: %s", strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest merge server error", "status", resp.StatusCode, "wait", wait.Seconds())
				c.sleep(wait)
				retries++
				continue
			}
			return "", fmt.Errorf("github merge failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("github merge failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

		// Success: response contains {"sha": "..."}
		var result struct {
			SHA string `json:"sha"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			// ignore decode error, we can still return success
			return "", nil
		}
		if result.SHA == "" && result.Message != "" {
			// GitHub sometimes returns {"message":"No commits between..."} — treat as merged
			return "", nil
		}
		return result.SHA, nil
	}
}

// Close closes a pull request via the GitHub REST API.
func (c *Client) Close(ctx context.Context, repo string, prNumber int) error {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.baseURL, owner, name, prNumber)

	payload := map[string]any{
		"state": "closed",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal close payload: %w", err)
	}

	var retries int
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build close request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("Content-Type", "application/json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying close", "url", url, "wait", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return fmt.Errorf("perform close request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest close server error", "status", resp.StatusCode, "wait", wait.Seconds())
				c.sleep(wait)
				retries++
				continue
			}
			return fmt.Errorf("github close failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("github close failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

		// Success - optionally decode to verify state
		var closedPR restPRNode
		if err := json.NewDecoder(resp.Body).Decode(&closedPR); err != nil {
			// ignore decode error — close succeeded even if we couldn't parse response
			return nil
		}
		if strings.ToLower(closedPR.State) != "closed" {
			c.log.Warn("close returned non-closed state", "state", closedPR.State)
		}
		return nil
	}
}

// CreateComment adds a comment to a pull request.
func (c *Client) CreateComment(ctx context.Context, repo string, prNumber int, body string) error {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL, owner, name, prNumber)

	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal comment payload: %w", err)
	}

	var retries int
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("build comment request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("Content-Type", "application/json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying comment", "url", url, "wait", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return fmt.Errorf("perform comment request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest comment server error", "status", resp.StatusCode, "wait", wait.Seconds())
				c.sleep(wait)
				retries++
				continue
			}
			return fmt.Errorf("github comment failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("github comment failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		// Success — optionally read response for created comment ID
		var commentResp struct{ ID int64 `json:"id"` }
		if err := json.NewDecoder(resp.Body).Decode(&commentResp); err != nil {
			// ignore, comment likely created
			return nil
		}
		_ = commentResp.ID
		return nil
	}
}

// AddLabels adds labels to a pull request.
func (c *Client) AddLabels(ctx context.Context, repo string, prNumber int, labels []string) error {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/labels", c.baseURL, owner, name, prNumber)

	payload := labels
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal labels payload: %w", err)
	}

	var retries int
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("build add-labels request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("Content-Type", "application/json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying add-labels", "url", url, "wait", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return fmt.Errorf("perform add-labels request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest add-labels server error", "status", resp.StatusCode, "wait", wait.Seconds())
				c.sleep(wait)
				retries++
				continue
			}
			return fmt.Errorf("github add-labels failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("github add-labels failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		// Success — response is an array of label objects
		return nil
	}
}

// GetRateLimit queries the GitHub REST rate limit endpoint.
func (c *Client) GetRateLimit(ctx context.Context) (int, error) {
	url := c.baseURL + "/rate_limit"

	var retries int
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return -1, fmt.Errorf("build rate limit request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying rate-limit", "url", url, "wait", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return -1, fmt.Errorf("perform rate limit request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest rate-limit server error", "status", resp.StatusCode, "wait", wait.Seconds())
				c.sleep(wait)
				retries++
				continue
			}
			return -1, fmt.Errorf("github rate-limit failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return -1, fmt.Errorf("github rate-limit failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

		var rate struct {
			Resources struct {
				Core struct {
					Remaining int `json:"remaining"`
					ResetAt   int64 `json:"reset"`
				} `json:"core"`
			} `json:"resources"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rate); err != nil {
			// Fall back to header-based if parsing fails
			remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
			return remaining, nil
		}
		return rate.Resources.Core.Remaining, nil
	}
}


// getBranchProtectionRequiredCount queries branch protection to get the number of
// required approving reviews for the given branch. Returns 0 if protection is not enabled.

// GetLabels fetches the current labels on a PR.
func (c *Client) GetLabels(ctx context.Context, repo string, prNumber int) ([]string, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/labels", c.baseURL, owner, name, prNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build get-labels request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform get-labels request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github get-labels failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var labelNodes []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&labelNodes); err != nil {
		return nil, fmt.Errorf("decode labels: %w", err)
	}
	labels := make([]string, 0, len(labelNodes))
	for _, n := range labelNodes {
		labels = append(labels, n.Name)
	}
	return labels, nil
}

// FetchComments retrieves all comment bodies on a PR.
func (c *Client) FetchComments(ctx context.Context, repo string, prNumber int) ([]commentDTO, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL, owner, name, prNumber)

	var all []commentDTO
	page := 1
	for {
		paged := fmt.Sprintf("%s?page=%d&per_page=100", url, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, paged, nil)
		if err != nil {
			return nil, fmt.Errorf("build fetch-comments request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("perform fetch-comments request: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("github fetch-comments failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		var pageComments []commentDTO
		if err := json.NewDecoder(resp.Body).Decode(&pageComments); err != nil {
			return nil, fmt.Errorf("decode comments: %w", err)
		}
		if len(pageComments) == 0 {
			break
		}
		all = append(all, pageComments...)
		if len(pageComments) < 100 {
			break
		}
		page++
		if page > 10 {
			c.log.Warn("fetch-comments capped at 10 pages", "total", len(all))
			break
		}
	}
	return all, nil
}

func (c *Client) getBranchProtectionRequiredCount(ctx context.Context, owner, repo, branch string) (int, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/branches/%s/protection", c.baseURL, owner, repo, branch)

	var retries int
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, fmt.Errorf("build branch protection request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if tok, tokErr := c.tokenSource.Token(ctx); tokErr == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTransientTransportError(err) && retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest transport error, retrying branch protection", "url", url, "wait", wait.Seconds(), "error", err.Error())
				c.sleep(wait)
				retries++
				continue
			}
			return 0, fmt.Errorf("perform branch protection request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			// Protection not enabled
			return 0, nil
		}
		if resp.StatusCode >= 500 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			if retries < c.maxSecondaryRetries {
				wait := transientBackoff(retries)
				c.log.Warn("rest branch protection server error", "status", resp.StatusCode, "wait", wait.Seconds())
				c.sleep(wait)
				retries++
				continue
			}
			return 0, fmt.Errorf("github branch protection failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return 0, fmt.Errorf("github branch protection failed %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}

		var protection struct {
			RequiredPullRequestReviews struct {
				RequiredApprovingReviewCount int `json:"required_approving_review_count"`
			} `json:"required_pull_request_reviews"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&protection); err != nil {
			return 0, fmt.Errorf("decode branch protection: %w", err)
		}
		return protection.RequiredPullRequestReviews.RequiredApprovingReviewCount, nil
	}
}
