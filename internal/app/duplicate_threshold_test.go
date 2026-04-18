package app

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestDuplicateThreshold_Reachable verifies that a PR pair with high file overlap
// (>0.8) and moderate title similarity (>0.05) produces a score >= DuplicateThreshold.
// This is a regression test for the bug where DuplicateThreshold was set to 0.90
// but the scoring formula could only produce a maximum of 0.85.
func TestDuplicateThreshold_Reachable(t *testing.T) {
	t.Parallel()

	// Two PRs with high file overlap (same 5 files) and identical titles
	// This should trigger the fileScore > 0.8 && titleScore > 0.05 boost
	// which produces a score above the threshold
	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 1,
			Title:  "Add user authentication with OAuth2",
			Body:   "Adds OAuth2 authentication to the app",
			FilesChanged: []string{
				"auth/user.go",
				"auth/oauth.go",
				"auth/session.go",
				"auth/token.go",
				"config.go",
			},
		},
		{
			Repo:   "owner/repo",
			Number: 2,
			Title:  "Add user authentication with OAuth2",
			Body:   "Adds OAuth2 authentication to the app",
			FilesChanged: []string{
				"auth/user.go",
				"auth/oauth.go",
				"auth/session.go",
				"auth/token.go",
				"config.go",
			},
		},
	}

	noop := func(string, int, int) {}

	dups, overlaps := classifyDuplicates(prs, nil, noop)

	// Score calculation for these PRs:
	// - titleScore = 1.0 (identical titles)
	// - fileScore = 1.0 (same 5 files)
	// - bodyScore = 1.0 (identical bodies)
	// - Base formula: 0.4*1.0 + 0.4*1.0 + 0.2*1.0 = 1.0
	// - With fileScore > 0.8 && titleScore > 0.05: score stays at 1.0
	// So the score is 1.0 which exceeds DuplicateThreshold of 0.85

	if len(dups) == 0 {
		t.Fatalf("DuplicateThreshold at 0.85 should be reachable; expected at least one duplicate group, got none")
	}
	if len(overlaps) != 0 {
		t.Fatalf("High similarity pair should be duplicate, not overlap: got %d overlaps", len(overlaps))
	}

	// Verify the duplicate group contains both PRs
	foundDup := false
	for _, dup := range dups {
		if dup.CanonicalPRNumber == 1 {
			foundDup = true
			has2 := false
			for _, n := range dup.DuplicatePRNums {
				if n == 2 {
					has2 = true
					break
				}
			}
			if !has2 {
				t.Fatalf("PR 2 should be in duplicate group with PR 1, got %v", dup.DuplicatePRNums)
			}
			if dup.Reason != "title/body/file similarity above duplicate threshold" {
				t.Fatalf("Expected reason 'title/body/file similarity above duplicate threshold', got %q", dup.Reason)
			}
			if dup.Similarity < types.DuplicateThreshold {
				t.Fatalf("Similarity score %f should meet or exceed DuplicateThreshold %f", dup.Similarity, types.DuplicateThreshold)
			}
		}
	}
	if !foundDup {
		t.Fatalf("Expected duplicate group with canonical PR 1")
	}
}

// TestDuplicateDetection_FindsGroup creates two PRs with same files and
// titles where one title is a literal substring of the other, then verifies
// they land in duplicate_groups (not just overlaps).
func TestDuplicateDetection_FindsGroup(t *testing.T) {
	t.Parallel()

	// Two PRs with identical files and titles where one title is a substring of the other
	// "Add feature" is a substring of "Add feature auth"
	prs := []types.PR{
		{
			Repo:   "owner/repo",
			Number: 10,
			Title:  "Add feature",
			Body:   "Adds a new feature",
			FilesChanged: []string{"feature.go", "feature_test.go"},
		},
		{
			Repo:   "owner/repo",
			Number: 11,
			Title:  "Add feature auth",
			Body:   "Adds a new feature",
			FilesChanged: []string{"feature.go", "feature_test.go"},
		},
	}

	noop := func(string, int, int) {}

	dups, overlaps := classifyDuplicates(prs, nil, noop)

	// With identical files (fileScore=1.0) and substring title relationship,
	// titleScore will be boosted to 0.80 via substring containment
	// fileScore > 0.8 && titleScore > 0.05 triggers: score = maxFloat(base, 0.85)
	// With non-empty bodies, the score exceeds 0.85 and meets the DuplicateThreshold

	if len(dups) == 0 {
		t.Fatalf("PRs with same files and substring titles should be detected as duplicates, not just overlaps")
	}
	if len(overlaps) != 0 {
		t.Fatalf("PRs exceeding duplicate threshold should not be classified as overlaps: got %d overlaps", len(overlaps))
	}

	// Verify the group structure
	var canonicalGroup *types.DuplicateGroup
	for _, dup := range dups {
		if dup.CanonicalPRNumber == 10 {
			canonicalGroup = &dup
			break
		}
	}

	if canonicalGroup == nil {
		t.Fatalf("Should have a duplicate group with canonical PR 10")
	}
	has11 := false
	for _, n := range canonicalGroup.DuplicatePRNums {
		if n == 11 {
			has11 = true
			break
		}
	}
	if !has11 {
		t.Fatalf("PR 11 should be listed as duplicate of PR 10, got %v", canonicalGroup.DuplicatePRNums)
	}
	if canonicalGroup.Similarity < types.DuplicateThreshold {
		t.Fatalf("Group similarity %f should meet DuplicateThreshold %f", canonicalGroup.Similarity, types.DuplicateThreshold)
	}
}