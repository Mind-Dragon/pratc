package cmd

import (
	"github.com/jeffersonnunn/pratc/internal/github"
)

// discoverTokensFn is the token discovery function used by attemptTokenFallback.
// It defaults to github.DiscoverTokens but can be overridden in tests.
var discoverTokensFn = github.DiscoverTokens
