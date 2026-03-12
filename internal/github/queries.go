package github

import "time"

func buildPullRequestsQuery(owner string, repo string, opts PullRequestListOptions) (string, map[string]any) {
	variables := map[string]any{
		"owner":   owner,
		"repo":    repo,
		"perPage": normalizePageSize(opts.PerPage),
	}
	if opts.Cursor != "" {
		variables["cursor"] = opts.Cursor
	}
	if !opts.UpdatedSince.IsZero() {
		variables["updatedSince"] = opts.UpdatedSince.UTC().Format(time.RFC3339)
	}

	return `
query PullRequests($owner: String!, $repo: String!, $perPage: Int!, $cursor: String, $updatedSince: DateTime) {
  repository(owner: $owner, name: $repo) {
    pullRequests(first: $perPage, after: $cursor, states: OPEN, orderBy: {field: UPDATED_AT, direction: ASC}) {
      pageInfo { hasNextPage endCursor }
      nodes {
        id
        number
        title
        body
        url
        isDraft
        createdAt
        updatedAt
        additions
        deletions
        changedFiles
        mergeable
        baseRefName
        headRefName
        author { login }
        labels(first: 20) { nodes { name } }
        reviewDecision
        statusCheckRollup { state }
      }
    }
  }
}
`, variables
}

func buildFilesQuery(owner string, repo string, number int) (string, map[string]any) {
	return `
query PullRequestFiles($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      files(first: 100) {
        nodes { path }
      }
    }
  }
}
`, map[string]any{"owner": owner, "repo": repo, "number": number}
}

func buildReviewsQuery(owner string, repo string, number int) (string, map[string]any) {
	return `
query PullRequestReviews($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviews(first: 100) {
        nodes {
          state
          author { login }
        }
      }
    }
  }
}
`, map[string]any{"owner": owner, "repo": repo, "number": number}
}

func buildCIStatusQuery(owner string, repo string, number int) (string, map[string]any) {
	return `
query PullRequestCIStatus($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      statusCheckRollup { state }
    }
  }
}
`, map[string]any{"owner": owner, "repo": repo, "number": number}
}

func normalizePageSize(perPage int) int {
	if perPage <= 0 {
		return 100
	}
	if perPage > 100 {
		return 100
	}
	return perPage
}
