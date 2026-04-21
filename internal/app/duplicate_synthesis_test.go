package app

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestBuildDuplicateSynthesis_Basic verifies that synthesis plans are created
// for duplicate groups and that canonical nomination works.
func TestBuildDuplicateSynthesis_Basic(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 1, Title: "Add user auth", Author: "alice", Mergeable: "yes", IsDraft: false},
		{Number: 2, Title: "Add user authentication", Author: "bob", Mergeable: "yes", IsDraft: false},
		{Number: 3, Title: "Fix auth bug", Author: "carol", Mergeable: "no", IsDraft: false},
	}

	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 1, DuplicatePRNums: []int{2}, Similarity: 0.92, Reason: "high title/body similarity"},
	}

	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 1, Confidence: 0.85, SubstanceScore: 75, SignalQuality: "high"},
			{PRNumber: 2, Confidence: 0.70, SubstanceScore: 60, SignalQuality: "medium"},
			{PRNumber: 3, Confidence: 0.50, SubstanceScore: 40, SignalQuality: "low"},
		},
	}

	conflicts := []types.ConflictPair{}

	synthesis := buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, conflicts)

	if len(synthesis) != 1 {
		t.Fatalf("expected 1 synthesis plan, got %d", len(synthesis))
	}

	plan := synthesis[0]
	if plan.GroupType != "duplicate" {
		t.Errorf("expected group type 'duplicate', got %q", plan.GroupType)
	}
	if plan.OriginalCanonicalPR != 1 {
		t.Errorf("expected original canonical PR 1, got %d", plan.OriginalCanonicalPR)
	}
	if len(plan.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(plan.Candidates))
	}

	// PR 1 should be canonical (higher confidence and substance)
	if plan.NominatedCanonicalPR != 1 {
		t.Errorf("expected nominated canonical PR 1, got %d", plan.NominatedCanonicalPR)
	}
}

// TestBuildDuplicateSynthesis_Roles verifies that candidate roles are assigned correctly.
func TestBuildDuplicateSynthesis_Roles(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 10, Title: "Feature A", Author: "alice", Mergeable: "yes"},
		{Number: 11, Title: "Feature A copy", Author: "bob", Mergeable: "yes"},
		{Number: 12, Title: "Feature A variant", Author: "carol", Mergeable: "yes"},
		{Number: 13, Title: "Draft feature", Author: "dave", Mergeable: "yes", IsDraft: true},
	}

	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 10, DuplicatePRNums: []int{11, 12, 13}, Similarity: 0.88},
	}

	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 10, Confidence: 0.9, SubstanceScore: 90, SignalQuality: "high"},
			{PRNumber: 11, Confidence: 0.85, SubstanceScore: 80, SignalQuality: "high"},
			{PRNumber: 12, Confidence: 0.70, SubstanceScore: 60, SignalQuality: "medium"},
			{PRNumber: 13, Confidence: 0.60, SubstanceScore: 50, SignalQuality: "low"},
		},
	}

	conflicts := []types.ConflictPair{}

	synthesis := buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, conflicts)

	if len(synthesis) != 1 {
		t.Fatalf("expected 1 synthesis plan, got %d", len(synthesis))
	}

	plan := synthesis[0]

	// Find candidates by PR number
	candidatesByPR := make(map[int]types.DuplicateSynthesisCandidate)
	for _, c := range plan.Candidates {
		candidatesByPR[c.PRNumber] = c
	}

	// PR 10 should be canonical (highest score)
	if candidatesByPR[10].Role != "canonical" {
		t.Errorf("PR 10 expected role 'canonical', got %q", candidatesByPR[10].Role)
	}

	// PR 11 should be alternate (close second)
	if candidatesByPR[11].Role != "alternate" {
		t.Errorf("PR 11 expected role 'alternate', got %q", candidatesByPR[11].Role)
	}

	// PR 12 should be contributor
	if candidatesByPR[12].Role != "contributor" {
		t.Errorf("PR 12 expected role 'contributor', got %q", candidatesByPR[12].Role)
	}

	// Draft PR 13 should likely be excluded or contributor
	if candidatesByPR[13].Role != "excluded" && candidatesByPR[13].Role != "contributor" {
		t.Errorf("PR 13 (draft) expected role 'excluded' or 'contributor', got %q", candidatesByPR[13].Role)
	}
}

// TestBuildDuplicateSynthesis_ConflictFootprint verifies that conflict footprint affects scoring.
func TestBuildDuplicateSynthesis_ConflictFootprint(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 100, Title: "Auth changes", Author: "alice", Mergeable: "yes"},
		{Number: 101, Title: "Auth changes copy", Author: "bob", Mergeable: "yes"},
	}

	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 100, DuplicatePRNums: []int{101}, Similarity: 0.91},
	}

	// Same review quality for both
	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 100, Confidence: 0.8, SubstanceScore: 80, SignalQuality: "high"},
			{PRNumber: 101, Confidence: 0.8, SubstanceScore: 80, SignalQuality: "high"},
		},
	}

	// PR 101 has 5 conflicts, PR 100 has none
	conflicts := []types.ConflictPair{
		{SourcePR: 101, TargetPR: 200},
		{SourcePR: 101, TargetPR: 201},
		{SourcePR: 101, TargetPR: 202},
		{SourcePR: 101, TargetPR: 203},
		{SourcePR: 101, TargetPR: 204},
	}

	synthesis := buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, conflicts)

	if len(synthesis) != 1 {
		t.Fatalf("expected 1 synthesis plan, got %d", len(synthesis))
	}

	plan := synthesis[0]

	// PR 100 should be canonical (same quality but no conflicts)
	if plan.NominatedCanonicalPR != 100 {
		t.Errorf("PR 100 should be canonical due to no conflicts, got PR %d", plan.NominatedCanonicalPR)
	}
}

// TestBuildDuplicateSynthesis_MergeablePenalty verifies that non-mergeable PRs are penalized.
func TestBuildDuplicateSynthesis_MergeablePenalty(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 200, Title: "Feature", Author: "alice", Mergeable: "yes"},
		{Number: 201, Title: "Feature copy", Author: "bob", Mergeable: "conflicting"},
	}

	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 200, DuplicatePRNums: []int{201}, Similarity: 0.90},
	}

	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 200, Confidence: 0.8, SubstanceScore: 80},
			{PRNumber: 201, Confidence: 0.8, SubstanceScore: 80},
		},
	}

	conflicts := []types.ConflictPair{}

	synthesis := buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, conflicts)

	if len(synthesis) != 1 {
		t.Fatalf("expected 1 synthesis plan, got %d", len(synthesis))
	}

	plan := synthesis[0]

	// PR 200 should be canonical (mergeable)
	if plan.NominatedCanonicalPR != 200 {
		t.Errorf("PR 200 should be canonical (mergeable), got PR %d", plan.NominatedCanonicalPR)
	}
}

// TestBuildDuplicateSynthesis_Overlaps verifies that overlap groups are also processed.
func TestBuildDuplicateSynthesis_Overlaps(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 300, Title: "Feature implementation", Author: "alice"},
		{Number: 301, Title: "Related feature", Author: "bob"},
	}

	overlaps := []types.DuplicateGroup{
		{CanonicalPRNumber: 300, DuplicatePRNums: []int{301}, Similarity: 0.75, Reason: "partial overlap"},
	}

	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 300, Confidence: 0.8, SubstanceScore: 80},
			{PRNumber: 301, Confidence: 0.7, SubstanceScore: 70},
		},
	}

	conflicts := []types.ConflictPair{}

	synthesis := buildDuplicateSynthesis(nil, overlaps, prs, reviewPayload, conflicts)

	if len(synthesis) != 1 {
		t.Fatalf("expected 1 synthesis plan for overlaps, got %d", len(synthesis))
	}

	plan := synthesis[0]
	if plan.GroupType != "overlap" {
		t.Errorf("expected group type 'overlap', got %q", plan.GroupType)
	}
	if plan.NominatedCanonicalPR != 300 {
		t.Errorf("PR 300 should be canonical (higher confidence/substance), got PR %d", plan.NominatedCanonicalPR)
	}
}

// TestBuildDuplicateSynthesis_EmptyInput verifies that empty inputs don't panic.
func TestBuildDuplicateSynthesis_EmptyInput(t *testing.T) {
	t.Parallel()

	// Empty duplicates and overlaps
	synthesis := buildDuplicateSynthesis(nil, nil, []types.PR{}, &types.ReviewResponse{}, nil)
	if synthesis != nil {
		t.Errorf("expected nil synthesis for empty input, got %v", synthesis)
	}

	// Empty PRs
	prs := []types.PR{}
	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 1, DuplicatePRNums: []int{2}},
	}
	reviewPayload := &types.ReviewResponse{Results: []types.ReviewResult{}}
	synthesis = buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, nil)
	if synthesis != nil {
		t.Errorf("expected nil synthesis for empty PRs, got %v", synthesis)
	}
}

// TestBuildDuplicateSynthesis_SynthesisNotes verifies that synthesis notes are generated.
func TestBuildDuplicateSynthesis_SynthesisNotes(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 400, Title: "Feature", Author: "alice"},
		{Number: 401, Title: "Feature copy", Author: "bob"},
	}

	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 400, DuplicatePRNums: []int{401}, Similarity: 0.90},
	}

	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 400, Confidence: 0.8, SubstanceScore: 80},
			{PRNumber: 401, Confidence: 0.7, SubstanceScore: 70},
		},
	}

	conflicts := []types.ConflictPair{}

	synthesis := buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, conflicts)

	if len(synthesis) != 1 {
		t.Fatalf("expected 1 synthesis plan, got %d", len(synthesis))
	}

	plan := synthesis[0]

	if len(plan.SynthesisNotes) == 0 {
		t.Error("expected synthesis notes to be generated")
	}

	// Should contain canonical nomination note
	foundCanonicalNote := false
	for _, note := range plan.SynthesisNotes {
		if note != "" && len(note) > 0 {
			foundCanonicalNote = true
			break
		}
	}
	if !foundCanonicalNote {
		t.Error("expected at least one non-empty synthesis note")
	}
}

// TestBuildDuplicateSynthesis_ScoreOrdering verifies that candidates are sorted by score descending.
func TestBuildDuplicateSynthesis_ScoreOrdering(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 500, Title: "Low quality", Author: "alice"},
		{Number: 501, Title: "Medium quality", Author: "bob"},
		{Number: 502, Title: "High quality", Author: "carol"},
	}

	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 500, DuplicatePRNums: []int{501, 502}, Similarity: 0.88},
	}

	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 500, Confidence: 0.4, SubstanceScore: 40},
			{PRNumber: 501, Confidence: 0.7, SubstanceScore: 70},
			{PRNumber: 502, Confidence: 0.9, SubstanceScore: 90},
		},
	}

	conflicts := []types.ConflictPair{}

	synthesis := buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, conflicts)

	if len(synthesis) != 1 {
		t.Fatalf("expected 1 synthesis plan, got %d", len(synthesis))
	}

	plan := synthesis[0]

	// Candidates should be ordered by score (highest first)
	if len(plan.Candidates) != 3 {
		t.Errorf("expected 3 candidates, got %d", len(plan.Candidates))
	}

	if plan.Candidates[0].SynthesisScore < plan.Candidates[1].SynthesisScore {
		t.Error("candidates should be sorted by score descending")
	}
	if plan.Candidates[1].SynthesisScore < plan.Candidates[2].SynthesisScore {
		t.Error("candidates should be sorted by score descending")
	}

	// PR 502 (high quality) should be canonical
	if plan.NominatedCanonicalPR != 502 {
		t.Errorf("PR 502 should be canonical (highest quality), got PR %d", plan.NominatedCanonicalPR)
	}
}

// TestScoreSynthesisCandidate verifies the scoring function works correctly.
func TestScoreSynthesisCandidate(t *testing.T) {
	t.Parallel()

	pr := types.PR{
		Number:   1,
		Title:    "Test PR",
		Author:   "alice",
		Mergeable: "yes",
		IsDraft:  false,
	}

	review := types.ReviewResult{
		PRNumber:        1,
		Confidence:     0.85,
		SubstanceScore: 80,
		SignalQuality:  "high",
		AnalyzerFindings: []types.AnalyzerFinding{
			{AnalyzerName: "quality", Finding: "test coverage adequate"},
		},
	}

	candidate, score := scoreSynthesisCandidate(pr, review, 0)

	if score <= 0 || score > 1 {
		t.Errorf("expected score in (0, 1], got %.2f", score)
	}

	if candidate.PRNumber != 1 {
		t.Errorf("expected candidate PR number 1, got %d", candidate.PRNumber)
	}
	if candidate.Title != "Test PR" {
		t.Errorf("expected candidate title 'Test PR', got %q", candidate.Title)
	}
	if candidate.HasTestEvidence != true {
		t.Error("expected HasTestEvidence to be true (finding contains 'test')")
	}
	if candidate.SignalQuality != "high" {
		t.Errorf("expected signal quality 'high', got %q", candidate.SignalQuality)
	}
}

// TestScoreSynthesisCandidate_DraftPenalty verifies that draft PRs get a score penalty.
func TestScoreSynthesisCandidate_DraftPenalty(t *testing.T) {
	t.Parallel()

	prDraft := types.PR{Number: 1, Title: "Draft PR", Author: "alice", IsDraft: true}
	prReady := types.PR{Number: 2, Title: "Ready PR", Author: "bob", IsDraft: false}

	review := types.ReviewResult{
		PRNumber:     1,
		Confidence:   0.8,
		SubstanceScore: 80,
	}

	_, scoreDraft := scoreSynthesisCandidate(prDraft, review, 0)
	_, scoreReady := scoreSynthesisCandidate(prReady, review, 0)

	if scoreDraft >= scoreReady {
		t.Error("draft PR should have lower score than ready PR")
	}
}

// TestBuildDuplicateSynthesis_GroupID verifies that group IDs are stable and unique.
func TestBuildDuplicateSynthesis_GroupID(t *testing.T) {
	t.Parallel()

	prs := []types.PR{
		{Number: 1, Title: "PR 1"},
		{Number: 2, Title: "PR 2"},
	}

	duplicates := []types.DuplicateGroup{
		{CanonicalPRNumber: 1, DuplicatePRNums: []int{2}, Similarity: 0.90},
	}
	overlaps := []types.DuplicateGroup{
		{CanonicalPRNumber: 1, DuplicatePRNums: []int{2}, Similarity: 0.75},
	}

	reviewPayload := &types.ReviewResponse{
		Results: []types.ReviewResult{
			{PRNumber: 1, Confidence: 0.8, SubstanceScore: 80},
			{PRNumber: 2, Confidence: 0.7, SubstanceScore: 70},
		},
	}

	// Process duplicates
	synthDup := buildDuplicateSynthesis(duplicates, nil, prs, reviewPayload, nil)
	// Process overlaps
	synthOver := buildDuplicateSynthesis(nil, overlaps, prs, reviewPayload, nil)

	if len(synthDup) != 1 || len(synthOver) != 1 {
		t.Fatal("expected 1 synthesis plan each")
	}

	dupID := synthDup[0].GroupID
	overID := synthOver[0].GroupID

	if dupID == overID {
		t.Error("duplicate and overlap groups should have different group IDs")
	}

	if dupID == "" {
		t.Error("group ID should not be empty")
	}
}
