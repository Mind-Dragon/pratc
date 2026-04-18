package app

import (
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// =============================================================================
// classifyGarbage tests
//
// Note: abandoned PR detection is handled by buildStaleness(), not classifyGarbage.
// classifyGarbage focuses on obviously bad PRs: empty, bot, spam, draft.
// =============================================================================

func TestClassifyGarbage_EmptyPR(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 1, Title: "Empty PR", Additions: 0, Deletions: 0, ChangedFilesCount: 0},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 1 {
		t.Fatalf("expected 1 garbage PR, got %d", len(garbage))
	}
	if garbage[0].PRNumber != 1 {
		t.Errorf("expected PRNumber 1, got %d", garbage[0].PRNumber)
	}
	if !strings.Contains(garbage[0].Reason, "empty PR") {
		t.Errorf("expected 'empty PR' in reason, got %q", garbage[0].Reason)
	}
}

func TestClassifyGarbage_BotPR(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 2, Title: "Bump deps", Author: "dependabot[bot]", IsBot: true, Additions: 10, Deletions: 5, ChangedFilesCount: 2},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 1 {
		t.Fatalf("expected 1 garbage PR, got %d", len(garbage))
	}
	if !strings.Contains(garbage[0].Reason, "bot-generated") {
		t.Errorf("expected 'bot-generated' in reason, got %q", garbage[0].Reason)
	}
}

func TestClassifyGarbage_EmptyTitle(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 3, Title: "   ", Additions: 10, Deletions: 5, ChangedFilesCount: 2},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 1 {
		t.Fatalf("expected 1 garbage PR, got %d", len(garbage))
	}
	if !strings.Contains(garbage[0].Reason, "empty title") {
		t.Errorf("expected 'empty title' in reason, got %q", garbage[0].Reason)
	}
}

func TestClassifyGarbage_ShortTitle(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 4, Title: "ab", Additions: 10, Deletions: 5, ChangedFilesCount: 2},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 1 {
		t.Fatalf("expected 1 garbage PR, got %d", len(garbage))
	}
	if !strings.Contains(garbage[0].Reason, "suspiciously short") {
		t.Errorf("expected 'suspiciously short' in reason, got %q", garbage[0].Reason)
	}
}

func TestClassifyGarbage_DraftMinimal(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 5, Title: "WIP draft", IsDraft: true, Additions: 1, Deletions: 1, ChangedFilesCount: 1},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 1 {
		t.Fatalf("expected 1 garbage PR, got %d", len(garbage))
	}
	if !strings.Contains(garbage[0].Reason, "draft with minimal changes") {
		t.Errorf("expected 'draft with minimal changes' in reason, got %q", garbage[0].Reason)
	}
}

func TestClassifyGarbage_NormalPRNotGarbage(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 10, Title: "Add user authentication", Author: "alice", Additions: 200, Deletions: 50, ChangedFilesCount: 10},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 0 {
		t.Fatalf("expected 0 garbage PRs, got %d: %v", len(garbage), garbage)
	}
}

func TestClassifyGarbage_WIPTitleNotSpam(t *testing.T) {
	t.Parallel()

	// "WIP" and "wip" are explicitly allowed as short titles
	prs := []types.PR{
		{Number: 6, Title: "WIP", Additions: 100, Deletions: 50, ChangedFilesCount: 5},
		{Number: 7, Title: "wip", Additions: 100, Deletions: 50, ChangedFilesCount: 5},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 0 {
		t.Fatalf("expected 0 garbage PRs for WIP titles, got %d: %v", len(garbage), garbage)
	}
}

func TestClassifyGarbage_MultipleReasons(t *testing.T) {
	t.Parallel()

	// PR that is both empty AND has empty title
	prs := []types.PR{
		{Number: 8, Title: "", Additions: 0, Deletions: 0, ChangedFilesCount: 0},
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 1 {
		t.Fatalf("expected 1 garbage PR, got %d", len(garbage))
	}
	if !strings.Contains(garbage[0].Reason, "empty PR") {
		t.Errorf("expected 'empty PR' in reason, got %q", garbage[0].Reason)
	}
	if !strings.Contains(garbage[0].Reason, "empty title") {
		t.Errorf("expected 'empty title' in reason, got %q", garbage[0].Reason)
	}
}

func TestClassifyGarbage_MixedBatch(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 1, Title: "Good feature", Additions: 100, Deletions: 50, ChangedFilesCount: 5},
		{Number: 2, Title: "", Additions: 0, Deletions: 0, ChangedFilesCount: 0},    // empty title + empty PR
		{Number: 3, Title: "Bot update", IsBot: true, Additions: 10, ChangedFilesCount: 1},
		{Number: 4, Title: "Another good PR", Additions: 50, Deletions: 20, ChangedFilesCount: 3},
		{Number: 5, Title: "x", Additions: 10, Deletions: 5, ChangedFilesCount: 1}, // short title
	}

	garbage := classifyGarbage(prs)

	if len(garbage) != 3 {
		t.Fatalf("expected 3 garbage PRs, got %d", len(garbage))
	}

	garbageNumbers := make(map[int]bool)
	for _, g := range garbage {
		garbageNumbers[g.PRNumber] = true
	}

	for _, want := range []int{2, 3, 5} {
		if !garbageNumbers[want] {
			t.Errorf("expected PR %d to be classified as garbage", want)
		}
	}
	if garbageNumbers[1] {
		t.Error("PR 1 should NOT be garbage")
	}
	if garbageNumbers[4] {
		t.Error("PR 4 should NOT be garbage")
	}
}
