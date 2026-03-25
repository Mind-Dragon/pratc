package app

import (
	"testing"
)

func TestBatchProcessor_StageCount(t *testing.T) {
	bp := NewBatchProcessor(StageConfig{StageSize: 64})

	tests := []struct {
		total int
		want  int
	}{
		{0, 1},
		{1, 1},
		{64, 1},
		{65, 2},
		{128, 2},
		{129, 3},
	}

	for _, tt := range tests {
		got := bp.StageCount(tt.total)
		if got != tt.want {
			t.Errorf("StageCount(%d) = %d, want %d", tt.total, got, tt.want)
		}
	}
}

func TestRecompose_Deduplicates(t *testing.T) {
	stages := []StageResult{
		{Stage: 1, OutputIDs: []int{1, 2, 3}},
		{Stage: 2, OutputIDs: []int{2, 3, 4, 5}},
	}
	got := Recompose(stages)
	want := []int{1, 2, 3, 4, 5}
	if !slicesEqual(got, want) {
		t.Errorf("Recompose = %v, want %v", got, want)
	}
}

func TestRecompose_Deterministic(t *testing.T) {
	stages := []StageResult{
		{Stage: 1, OutputIDs: []int{5, 3, 1}},
		{Stage: 2, OutputIDs: []int{2, 4}},
	}
	got1 := Recompose(stages)
	got2 := Recompose(stages)
	if !slicesEqual(got1, got2) {
		t.Errorf("Recompose not deterministic: got1=%v got2=%v", got1, got2)
	}
}

func TestDeduplicate(t *testing.T) {
	got := Deduplicate([]int{1, 2, 2, 3, 1, 4})
	want := []int{1, 2, 3, 4}
	if !slicesEqual(got, want) {
		t.Errorf("Deduplicate = %v, want %v", got, want)
	}
}

func TestBatchProcessor_LargeScale(t *testing.T) {
	bp := NewBatchProcessor(StageConfig{StageSize: 64})

	var allIDs []int
	for i := 1; i <= 6400; i++ {
		allIDs = append(allIDs, i)
	}

	stageCount := bp.StageCount(len(allIDs))
	wantStageCount := 100
	if stageCount != wantStageCount {
		t.Errorf("StageCount(6400) = %d, want %d", stageCount, wantStageCount)
	}

	var stages []StageResult
	for i := 0; i < 100; i++ {
		start := i * 64
		end := start + 64
		stageIDs := allIDs[start:end]
		stages = append(stages, StageResult{
			Stage:     i + 1,
			StageSize: 64,
			InputIDs:  stageIDs,
			OutputIDs: stageIDs,
			Rejected:  0,
		})
	}

	result := Recompose(stages)
	if len(result) != 6400 {
		t.Errorf("Recompose returned %d IDs, want 6400", len(result))
	}

	result2 := Recompose(stages)
	if len(result) != len(result2) {
		t.Errorf("Recompose not deterministic: got %d and %d", len(result), len(result2))
	}
}

func TestBatchProcessor_DedupeStress(t *testing.T) {
	stages := []StageResult{
		{Stage: 1, OutputIDs: []int{1, 2, 3, 100}},
		{Stage: 2, OutputIDs: []int{2, 3, 4, 200}},
		{Stage: 3, OutputIDs: []int{3, 4, 5, 300}},
	}
	result := Recompose(stages)
	want := []int{1, 2, 3, 4, 5, 100, 200, 300}
	if len(result) != 8 {
		t.Errorf("Recompose returned %d IDs, want 8", len(result))
	}
	if !slicesEqual(result, want) {
		t.Errorf("Recompose = %v, want %v", result, want)
	}
}

func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
