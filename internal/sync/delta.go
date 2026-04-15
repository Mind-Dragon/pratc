package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	gh "github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type PRInfo struct {
	Number    int
	UpdatedAt time.Time
}

type SyncDelta struct {
	NewPRs       []int
	UpdatedPRs   []int
	UnchangedPRs []int
	ClosedPRs    []int
}

type DeltaDetector struct {
	cacheStore PRLister
}

type PRLister interface {
	ListPRs(filter cache.PRFilter) ([]types.PR, error)
	ListPRsIter(filter cache.PRFilter, fn func(types.PR) error) error
}

func NewDeltaDetector(cache PRLister) *DeltaDetector {
	return &DeltaDetector{
		cacheStore: cache,
	}
}

func (d *DeltaDetector) ComputeDelta(ctx context.Context, repo string, since time.Time) (*SyncDelta, error) {
	githubPRs, err := d.fetchGitHubPRList(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("fetch github pr list: %w", err)
	}

	cacheMap := make(map[int]time.Time)
	if err := d.cacheStore.ListPRsIter(cache.PRFilter{
		Repo:         repo,
		UpdatedSince: since,
	}, func(pr types.PR) error {
		cacheMap[pr.Number] = parseUpdatedAt(pr.UpdatedAt)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list cached prs: %w", err)
	}

	githubMap := make(map[int]time.Time, len(githubPRs))
	for _, pr := range githubPRs {
		githubMap[pr.Number] = pr.UpdatedAt
	}

	var newPRs, updatedPRs, unchangedPRs, closedPRs []int

	for number, ghUpdatedAt := range githubMap {
		cacheUpdatedAt, exists := cacheMap[number]
		if !exists {
			newPRs = append(newPRs, number)
		} else if !ghUpdatedAt.Equal(cacheUpdatedAt) {
			updatedPRs = append(updatedPRs, number)
		} else {
			unchangedPRs = append(unchangedPRs, number)
		}
	}

	for number := range cacheMap {
		if _, exists := githubMap[number]; !exists {
			closedPRs = append(closedPRs, number)
		}
	}

	sort.Ints(newPRs)
	sort.Ints(updatedPRs)
	sort.Ints(unchangedPRs)
	sort.Ints(closedPRs)

	return &SyncDelta{
		NewPRs:       newPRs,
		UpdatedPRs:   updatedPRs,
		UnchangedPRs: unchangedPRs,
		ClosedPRs:    closedPRs,
	}, nil
}

func (d *DeltaDetector) fetchGitHubPRList(ctx context.Context, repo string) ([]PRInfo, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	token, err := gh.ResolveToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve GitHub auth: %w", err)
	}
	baseURL := "https://api.github.com/graphql"

	var allPRs []PRInfo
	cursor := ""

	for {
		query, variables := buildDeltaQuery(owner, name, cursor, 100)

		payload := map[string]any{
			"query":     query,
			"variables": variables,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal graphql payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build graphql request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("perform graphql request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("github graphql request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var response struct {
			Data struct {
				Repository struct {
					PullRequests struct {
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []struct {
							Number    int    `json:"number"`
							UpdatedAt string `json:"updatedAt"`
						} `json:"nodes"`
					} `json:"pullRequests"`
				} `json:"repository"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("decode graphql response: %w", err)
		}

		for _, node := range response.Data.Repository.PullRequests.Nodes {
			updatedAt, err := time.Parse(time.RFC3339, node.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("parse updated_at for pr %d: %w", node.Number, err)
			}
			allPRs = append(allPRs, PRInfo{
				Number:    node.Number,
				UpdatedAt: updatedAt,
			})
		}

		if !response.Data.Repository.PullRequests.PageInfo.HasNextPage {
			break
		}
		cursor = response.Data.Repository.PullRequests.PageInfo.EndCursor
	}

	return allPRs, nil
}

func buildDeltaQuery(owner, repo, cursor string, perPage int) (string, map[string]any) {
	variables := map[string]any{
		"owner":   owner,
		"repo":    repo,
		"perPage": perPage,
	}

	varDecl := "$owner: String!, $repo: String!, $perPage: Int!"
	afterClause := ""

	if cursor != "" {
		varDecl += ", $cursor: String"
		afterClause = ", after: $cursor"
		variables["cursor"] = cursor
	}

	query := fmt.Sprintf(`query PullRequestDelta(%s) {
	  repository(owner: $owner, name: $repo) {
	    pullRequests(first: $perPage%s, states: OPEN, orderBy: {field: UPDATED_AT, direction: ASC}) {
	      pageInfo { hasNextPage endCursor }
	      nodes {
	        number
	        updatedAt
	      }
	    }
	  }
	}`, varDecl, afterClause)

	return query, variables
}

func splitRepo(repo string) (string, string, error) {
	var parts []string
	for i := 0; i < len(repo); i++ {
		if repo[i] == '/' {
			parts = []string{repo[:i], repo[i+1:]}
			break
		}
	}
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}

func parseUpdatedAt(updatedAt string) time.Time {
	if updatedAt == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}
