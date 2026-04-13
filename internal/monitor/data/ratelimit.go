package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const rateLimitCacheTTL = 10 * time.Second

type RateLimitFetcher struct {
	httpClient HTTPClient
	token      string
	cached     *RateLimitView
	cacheUntil time.Time
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewRateLimitFetcher(token string) *RateLimitFetcher {
	return &RateLimitFetcher{
		httpClient: http.DefaultClient,
		token:      strings.TrimSpace(token),
	}
}

func (f *RateLimitFetcher) Fetch(ctx context.Context) (RateLimitView, error) {
	now := time.Now()

	if f.cached != nil && now.Before(f.cacheUntil) {
		return *f.cached, nil
	}

	view, err := f.fetchFromGitHub(ctx)
	if err != nil {
		if f.cached != nil {
			return *f.cached, fmt.Errorf("rate limit fetch failed, using stale cache: %w", err)
		}
		return RateLimitView{}, fmt.Errorf("rate limit fetch failed: %w", err)
	}

	f.cached = &view
	f.cacheUntil = now.Add(rateLimitCacheTTL)

	return view, nil
}

func (f *RateLimitFetcher) fetchFromGitHub(ctx context.Context) (RateLimitView, error) {
	baseURL := "https://api.github.com"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/rate_limit", nil)
	if err != nil {
		return RateLimitView{}, fmt.Errorf("create rate limit request: %w", err)
	}

	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return RateLimitView{}, fmt.Errorf("perform rate limit request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return RateLimitView{}, fmt.Errorf("rate limit endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var rateLimitResp rateLimitResponse
	if err := json.NewDecoder(resp.Body).Decode(&rateLimitResp); err != nil {
		return RateLimitView{}, fmt.Errorf("decode rate limit response: %w", err)
	}

	resetTime := time.Unix(rateLimitResp.Rate.Reset, 0)

	return RateLimitView{
		Remaining:    rateLimitResp.Rate.Remaining,
		Total:        rateLimitResp.Rate.Limit,
		ResetTime:    resetTime,
		UsageHistory: nil,
	}, nil
}

type rateLimitResponse struct {
	Rate struct {
		Limit     int   `json:"limit"`
		Remaining int   `json:"remaining"`
		Reset     int64 `json:"reset"`
		Used      int   `json:"used"`
	} `json:"rate"`
}
