package github

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// ETagCache interface for conditional request caching
type ETagCache interface {
	Get(key string) (etag string, data []byte, ok bool)
	Set(key string, etag string, data []byte) error
}

// MemoryETagCache implements ETagCache with in-memory storage
type MemoryETagCache struct {
	mu    sync.RWMutex
	cache map[string]etagEntry
}

type etagEntry struct {
	etag      string
	data      []byte
	timestamp time.Time
}

func NewMemoryETagCache() *MemoryETagCache {
	return &MemoryETagCache{
		cache: make(map[string]etagEntry),
	}
}

func (c *MemoryETagCache) Get(key string) (string, []byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.cache[key]
	if !ok {
		return "", nil, false
	}
	return entry.etag, entry.data, true
}

func (c *MemoryETagCache) Set(key string, etag string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = etagEntry{
		etag:      etag,
		data:      data,
		timestamp: time.Now(),
	}
	return nil
}

// cacheKey generates a cache key from GraphQL query and variables
func cacheKey(query string, variables map[string]any) string {
	h := sha256.New()
	h.Write([]byte(query))
	if variables != nil {
		vJSON, _ := json.Marshal(variables)
		h.Write(vJSON)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// FetchPullRequestsConditional fetches PRs with ETag conditional requests
// Returns (prs, fromCache, error)
func (c *Client) FetchPullRequestsConditional(ctx context.Context, repo string, opts PullRequestListOptions) ([]types.PR, bool, error) {
	// Build the query
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, false, err
	}

	query, variables := buildPullRequestsQuery(owner, name, opts)

	// Check ETag cache
	key := cacheKey(query, variables)
	var cachedETag string
	if c.etagCache != nil {
		if etag, data, ok := c.etagCache.Get(key); ok {
			cachedETag = etag
			// TODO: Make conditional request with If-None-Match header
			// For now, just proceed with normal request
			_ = data
			_ = cachedETag
		}
	}

	// Make the request
	// TODO: Add If-None-Match header when cachedETag is set
	// TODO: Handle 304 response by returning cached data

	// For now, use existing FetchPullRequests
	prs, err := c.FetchPullRequests(ctx, repo, opts)
	return prs, false, err
}
