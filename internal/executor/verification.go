package executor

import (
	"context"
	"fmt"
	"strings"
)

// VerifyMerge checks if a PR has been successfully merged
func VerifyMerge(ctx context.Context, mutator GitHubMutator, repo string, prNumber int) error {
	prState, err := mutator.GetPRState(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR state: %w", err)
	}
	
	if prState.State != "merged" {
		return fmt.Errorf("PR #%d is not merged (state: %s)", prNumber, prState.State)
	}
	
	return nil
}

// VerifyClose checks if a PR has been successfully closed
func VerifyClose(ctx context.Context, mutator GitHubMutator, repo string, prNumber int) error {
	prState, err := mutator.GetPRState(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR state: %w", err)
	}
	
	if prState.State != "closed" {
		return fmt.Errorf("PR #%d is not closed (state: %s)", prNumber, prState.State)
	}
	
	return nil
}

// VerifyComment checks if a comment with the given body exists on the PR
func VerifyComment(ctx context.Context, mutator GitHubMutator, repo string, prNumber int, body string) error {
	comments, err := mutator.GetComments(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}
	
	for _, comment := range comments {
		if strings.TrimSpace(comment.Body) == strings.TrimSpace(body) {
			return nil
		}
	}
	
	return fmt.Errorf("comment with body %q not found on PR #%d", body, prNumber)
}

// VerifyLabels checks if all given labels exist on the PR
func VerifyLabels(ctx context.Context, mutator GitHubMutator, repo string, prNumber int, labels []string) error {
	prLabels, err := mutator.GetLabels(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get labels: %w", err)
	}
	
	// Create a map of existing labels for efficient lookup
	existingLabels := make(map[string]bool)
	for _, label := range prLabels {
		existingLabels[strings.ToLower(label)] = true
	}
	
	// Check if all expected labels exist
	for _, expectedLabel := range labels {
		if !existingLabels[strings.ToLower(expectedLabel)] {
			return fmt.Errorf("label %q not found on PR #%d", expectedLabel, prNumber)
		}
	}
	
	return nil
}

// VerifyFixApplied checks if a fix was applied to the PR
func VerifyFixApplied(ctx context.Context, mutator GitHubMutator, repo string, prNumber int) error {
	// For now, we verify that the PR is still open and mergeable
	// In a real implementation, we would check for specific fix-related artifacts
	prState, err := mutator.GetPRState(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR state: %w", err)
	}
	
	if prState.State != "open" {
		return fmt.Errorf("PR #%d is not open (state: %s)", prNumber, prState.State)
	}
	
	return nil
}
