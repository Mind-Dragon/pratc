package executor

import (
	"context"
	"fmt"
)

// AddCommentWithVerification adds a comment and verifies it was successfully added
func AddCommentWithVerification(ctx context.Context, mutator GitHubMutator, repo string, prNumber int, body string, dryRun bool) error {
	// Add the comment
	if err := mutator.AddComment(ctx, repo, prNumber, body, dryRun); err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}

	// In dry-run mode, skip verification
	if dryRun {
		return nil
	}

	// Verify the comment was added
	return VerifyComment(ctx, mutator, repo, prNumber, body)
}

// AddLabelsWithVerification adds labels and verifies they were successfully added
func AddLabelsWithVerification(ctx context.Context, mutator GitHubMutator, repo string, prNumber int, labels []string, dryRun bool) error {
	// Add the labels
	if err := mutator.AddLabels(ctx, repo, prNumber, labels, dryRun); err != nil {
		return fmt.Errorf("failed to add labels: %w", err)
	}

	// In dry-run mode, skip verification
	if dryRun {
		return nil
	}

	// Verify the labels were added
	return VerifyLabels(ctx, mutator, repo, prNumber, labels)
}
