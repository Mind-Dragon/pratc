package github

import (
	"fmt"
	"time"
)

func buildPullRequestsQuery(owner string, repo string, opts PullRequestListOptions) (string, map[string]any) {
	variables := map[string]any{
		"owner":   owner,
		"repo":    repo,
		"perPage": normalizePageSize(opts.PerPage),
	}
	useCursor := opts.Cursor != ""
	useUpdatedSince := !opts.UpdatedSince.IsZero()
	if useCursor {
		variables["cursor"] = opts.Cursor
	}
	if useUpdatedSince {
		variables["updatedSince"] = opts.UpdatedSince.UTC().Format(time.RFC3339)
	}

	varDecl := "$owner: String!, $repo: String!, $perPage: Int!"
	afterClause := ""
	if useCursor {
		varDecl += ", $cursor: String"
		afterClause = ", after: $cursor"
	}
	if useUpdatedSince {
		varDecl += ", $updatedSince: DateTime"
	}

	query := fmt.Sprintf(`query PullRequests(%s) {
  repository(owner: $owner, name: $repo) {
    pullRequests(first: $perPage%s, states: OPEN, orderBy: {field: UPDATED_AT, direction: ASC}) {
      totalCount
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
	}`, varDecl, afterClause)

	return query, variables
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

// buildOpenPRCountQuery returns a lightweight GraphQL query that fetches only
// the totalCount of open pull requests, without fetching any PR nodes.
func buildOpenPRCountQuery(owner string, repo string) (string, map[string]any) {
	return `query OpenPRCount($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
    pullRequests(states: OPEN, first: 1) {
      totalCount
    }
  }
}
`, map[string]any{"owner": owner, "repo": repo}
}
