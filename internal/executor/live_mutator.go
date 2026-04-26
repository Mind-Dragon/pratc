package executor

import (
	"context"

	"github.com/jeffersonnunn/pratc/internal/github"
)

// LiveGitHubMutator wraps a real GitHub client and implements GitHubMutator.
type LiveGitHubMutator struct {
	client *github.Client
}

var _ GitHubMutator = (*LiveGitHubMutator)(nil)

// NewLiveGitHubMutator creates a mutator from a configured GitHub client.
func NewLiveGitHubMutator(client *github.Client) *LiveGitHubMutator {
	return &LiveGitHubMutator{client: client}
}

// GetPRState fetches live PR state and converts it to the executor PRState.
func (m *LiveGitHubMutator) GetPRState(ctx context.Context, repo string, prNumber int) (PRState, error) {
	pr, err := m.client.GetPR(ctx, repo, prNumber)
	if err != nil {
		return PRState{}, err
	}
	mergeable := false
	if pr.Mergeable != nil {
		mergeable = *pr.Mergeable
	}
	return PRState{
		Number:     prNumber,
		State:      pr.State,
		HeadSHA:    pr.HeadSHA,
		BaseBranch: pr.Base.Ref,
		Mergeable:  mergeable,
		CIStatus:   pr.CIStatus,
	}, nil
}

// GetHeadSHA returns the current head SHA of the PR.
func (m *LiveGitHubMutator) GetHeadSHA(ctx context.Context, repo string, prNumber int) (string, error) {
	pr, err := m.client.GetPR(ctx, repo, prNumber)
	if err != nil {
		return "", err
	}
	return pr.HeadSHA, nil
}

// GetBaseBranch returns the base branch name of the PR.
func (m *LiveGitHubMutator) GetBaseBranch(ctx context.Context, repo string, prNumber int) (string, error) {
	pr, err := m.client.GetPR(ctx, repo, prNumber)
	if err != nil {
		return "", err
	}
	return pr.Base.Ref, nil
}

// GetCIStatus returns the combined CI status string.
func (m *LiveGitHubMutator) GetCIStatus(ctx context.Context, repo string, prNumber int) (string, error) {
	pr, err := m.client.GetPR(ctx, repo, prNumber)
	if err != nil {
		return "", err
	}
	return pr.CIStatus, nil
}

// GetMergeable returns whether the PR is mergeable.
func (m *LiveGitHubMutator) GetMergeable(ctx context.Context, repo string, prNumber int) (bool, error) {
	pr, err := m.client.GetPR(ctx, repo, prNumber)
	if err != nil {
		return false, err
	}
	if pr.Mergeable == nil {
		return false, nil
	}
	return *pr.Mergeable, nil
}

// GetRequiredReviews returns true if branch protection requires reviews.
func (m *LiveGitHubMutator) GetRequiredReviews(ctx context.Context, repo string, prNumber int) (bool, error) {
	pr, err := m.client.GetPR(ctx, repo, prNumber)
	if err != nil {
		return false, err
	}
	return pr.RequiredReviews > 0, nil
}

// GetRateLimitRemaining returns remaining rate limit budget (stub).
func (m *LiveGitHubMutator) GetRateLimitRemaining(ctx context.Context) (int, error) {
	// TODO: query actual rate limit from github client if available
	return 1000, nil
}

// Merge performs the merge operation (live or dry-run).
func (m *LiveGitHubMutator) Merge(ctx context.Context, repo string, prNumber int, opts MergeOptions, dryRun bool) (MergeResult, error) {
	if dryRun {
		return MergeResult{Merged: false, SHA: "", AlreadyMerged: false}, nil
	}
	sha, err := m.client.Merge(ctx, repo, prNumber, opts.CommitTitle, opts.CommitMessage, opts.MergeMethod)
	if err != nil {
		return MergeResult{}, err
	}
	return MergeResult{Merged: true, SHA: sha, AlreadyMerged: false}, nil
}

// Close closes the PR.
func (m *LiveGitHubMutator) Close(ctx context.Context, repo string, prNumber int, reason string, dryRun bool) error {
	if dryRun {
		return nil
	}
	if reason != "" {
		if err := m.client.CreateComment(ctx, repo, prNumber, reason); err != nil {
			return err
		}
	}
	return m.client.Close(ctx, repo, prNumber)
}

// AddComment adds a comment to the PR.
func (m *LiveGitHubMutator) AddComment(ctx context.Context, repo string, prNumber int, body string, dryRun bool) error {
	if dryRun {
		return nil
	}
	return m.client.CreateComment(ctx, repo, prNumber, body)
}

// AddLabels adds labels to the PR.
func (m *LiveGitHubMutator) AddLabels(ctx context.Context, repo string, prNumber int, labels []string, dryRun bool) error {
	if dryRun {
		return nil
	}
	return m.client.AddLabels(ctx, repo, prNumber, labels)
}

// ApplyFix creates a branch with fixes (not implemented yet).
func (m *LiveGitHubMutator) ApplyFix(ctx context.Context, repo string, prNumber int, patch string, dryRun bool) (ApplyFixResult, error) {
	return ApplyFixResult{}, nil
}

// HasWritten returns whether any mutation has been performed (fake mutator pattern).
func (m *LiveGitHubMutator) HasWritten() bool {
	return false
}

// ResetWrites resets the mutation tracker (fake mutator pattern).
func (m *LiveGitHubMutator) ResetWrites() {}

// GetComments returns all comments on the PR.
func (m *LiveGitHubMutator) GetComments(ctx context.Context, repo string, prNumber int) ([]Comment, error) {
	raw, err := m.client.FetchComments(ctx, repo, prNumber)
	if err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(raw))
	for _, c := range raw {
		comments = append(comments, Comment{Body: c.Body})
	}
	return comments, nil
}

// GetLabels returns the current labels on the PR.
func (m *LiveGitHubMutator) GetLabels(ctx context.Context, repo string, prNumber int) ([]string, error) {
	return m.client.GetLabels(ctx, repo, prNumber)
}
