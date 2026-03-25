package planning

import (
	"testing"
)

func TestEvaluator_Eval_SingleID(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 10}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name         string
		input        string
		wantSelected []int
		wantWarnings bool
	}{
		{"existing ID", "1", []int{1}, false},
		{"non-existing ID", "99", []int{}, true},
		{"ID at boundary", "10", []int{10}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvalString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !intSlicesEqual(result.SelectedIDs, tt.wantSelected) {
				t.Errorf("expected selected %v, got %v", tt.wantSelected, result.SelectedIDs)
			}

			hasWarnings := len(result.Warnings) > 0
			if hasWarnings != tt.wantWarnings {
				t.Errorf("expected warnings=%v, got %v", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestEvaluator_Eval_Range(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 8, 10, 15, 20}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name         string
		input        string
		wantSelected []int
		wantWarnings bool
	}{
		{"range all exist", "1-3", []int{1, 2, 3}, false},
		{"range some missing", "1-5", []int{1, 2, 3, 5}, true},
		{"range none exist", "11-14", []int{}, true},
		{"single element range", "10-10", []int{10}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvalString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !intSlicesEqual(result.SelectedIDs, tt.wantSelected) {
				t.Errorf("expected selected %v, got %v", tt.wantSelected, result.SelectedIDs)
			}

			hasWarnings := len(result.Warnings) > 0
			if hasWarnings != tt.wantWarnings {
				t.Errorf("expected warnings=%v, got warnings=%v", tt.wantWarnings, hasWarnings)
			}
		})
	}
}

func TestEvaluator_Eval_AND(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 8, 10}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name         string
		input        string
		wantSelected []int
	}{
		{"two existing IDs", "1 AND 2", []int{}},
		{"ID and range", "1 AND 2-5", []int{}},
		{"two ranges", "1-3 AND 5-10", []int{}},
		{"with non-existing", "1 AND 99", []int{}},
		{"same ID twice", "1 AND 1", []int{1}},
		{"overlapping ranges", "3-6 AND 5-8", []int{5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvalString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !intSlicesEqual(result.SelectedIDs, tt.wantSelected) {
				t.Errorf("expected selected %v, got %v", tt.wantSelected, result.SelectedIDs)
			}
		})
	}
}

func TestEvaluator_Eval_OR(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 8, 10}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name         string
		input        string
		wantSelected []int
	}{
		{"two existing IDs", "1 OR 2", []int{1, 2}},
		{"ID and range", "1 OR 8-10", []int{1, 8, 10}},
		{"two ranges", "1-3 OR 8-10", []int{1, 2, 3, 8, 10}},
		{"with non-existing", "1 OR 99", []int{1}},
		{"overlapping", "1-5 OR 3-8", []int{1, 2, 3, 5, 8}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvalString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !intSlicesEqual(result.SelectedIDs, tt.wantSelected) {
				t.Errorf("expected selected %v, got %v", tt.wantSelected, result.SelectedIDs)
			}
		})
	}
}

func TestEvaluator_Eval_Precedence(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 8, 10, 15, 20}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name         string
		input        string
		wantSelected []int
	}{
		{"or then and", "1 OR 2 AND 3", []int{1}},
		{"and then or", "1 AND 2 OR 3", []int{3}},
		{"range and or", "1-5 AND 3-7 OR 10", []int{3, 5, 10}},
		{"parens override", "(1 OR 2) AND 3", []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvalString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !intSlicesEqual(result.SelectedIDs, tt.wantSelected) {
				t.Errorf("expected selected %v, got %v", tt.wantSelected, result.SelectedIDs)
			}
		})
	}
}

func TestEvaluator_Eval_Complex(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 8, 10, 15, 20, 25, 30}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name         string
		input        string
		wantSelected []int
	}{
		{
			name:         "nested groups",
			input:        "(1 OR 2) AND (3 OR 5)",
			wantSelected: []int{},
		},
		{
			name:         "multiple OR with AND",
			input:        "1 OR 2 AND 3 OR 5",
			wantSelected: []int{1, 5},
		},
		{
			name:         "complex range",
			input:        "1-10 AND 5-15 OR 25",
			wantSelected: []int{5, 8, 10, 25},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvalString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !intSlicesEqual(result.SelectedIDs, tt.wantSelected) {
				t.Errorf("expected selected %v, got %v", tt.wantSelected, result.SelectedIDs)
			}
		})
	}
}

func TestEvaluator_Contains(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 10}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name      string
		input     string
		checkID   int
		wantMatch bool
	}{
		{"single ID match", "1", 1, true},
		{"single ID no match", "1", 2, false},
		{"range contains", "1-5", 3, true},
		{"range not contains", "1-5", 8, false},
		{"OR contains", "1 OR 5", 5, true},
		{"OR not contains", "1 OR 5", 3, false},
		{"AND both exist", "1 AND 5", 1, false},
		{"AND check second", "1 AND 5", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			got := eval.Contains(expr, tt.checkID)
			if got != tt.wantMatch {
				t.Errorf("expected Contains(%d)=%v, got %v", tt.checkID, tt.wantMatch, got)
			}
		})
	}
}

func TestEvaluator_Count(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 8, 10}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{"single ID", "5", 1},
		{"range", "1-5", 4},
		{"OR no overlap", "1 OR 10", 2},
		{"AND intersection", "1-5 AND 3-10", 2},
		{"empty result", "99", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			got := eval.Count(expr)
			if got != tt.wantCount {
				t.Errorf("expected count %d, got %d", tt.wantCount, got)
			}
		})
	}
}

func TestEvaluator_IsEmpty(t *testing.T) {
	availableIDs := []int{1, 2, 3}
	eval := NewEvaluator(availableIDs)

	tests := []struct {
		name      string
		input     string
		wantEmpty bool
	}{
		{"has results", "1", false},
		{"no results", "99", true},
		{"empty intersection", "1 AND 99", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			got := eval.IsEmpty(expr)
			if got != tt.wantEmpty {
				t.Errorf("expected IsEmpty=%v, got %v", tt.wantEmpty, got)
			}
		})
	}
}

func TestEvaluator_ToPredicate(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 10}
	eval := NewEvaluator(availableIDs)

	expr, err := Parse("1-3 OR 10")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	predicate := eval.ToPredicate(expr)

	tests := []struct {
		id        int
		wantMatch bool
	}{
		{1, true},
		{2, true},
		{3, true},
		{5, false},
		{10, true},
		{99, false},
	}

	for _, tt := range tests {
		t.Run("id-"+string(rune('0'+byte(tt.id))), func(t *testing.T) {
			got := predicate(tt.id)
			if got != tt.wantMatch {
				t.Errorf("expected predicate(%d)=%v, got %v", tt.id, tt.wantMatch, got)
			}
		})
	}
}

func TestEvaluator_EvalString_ParseError(t *testing.T) {
	availableIDs := []int{1, 2, 3}
	eval := NewEvaluator(availableIDs)

	_, err := eval.EvalString("1 @ 2")
	if err == nil {
		t.Fatalf("expected parse error but got none")
	}

	if !IsSelectorError(err, ErrSelectorUnexpectedToken) {
		t.Errorf("expected ErrSelectorUnexpectedToken, got %v", err)
	}
}

func TestEvaluator_Eval_EmptyExpression(t *testing.T) {
	eval := NewEvaluator([]int{1, 2, 3})

	result := eval.Eval(nil)
	if len(result.SelectedIDs) != 0 {
		t.Errorf("expected empty result for nil expression, got %v", result.SelectedIDs)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for nil expression")
	}
}

func TestSetOperations(t *testing.T) {
	t.Run("Intersection", func(t *testing.T) {
		a := map[int]struct{}{1: {}, 2: {}, 3: {}}
		b := map[int]struct{}{2: {}, 3: {}, 4: {}}
		c := map[int]struct{}{3: {}, 5: {}}

		// Two sets
		got := Intersection(a, b)
		want := []int{2, 3}
		if !intSlicesEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}

		// Three sets
		got = Intersection(a, b, c)
		want = []int{3}
		if !intSlicesEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}

		// Single set
		got = Intersection(a)
		want = []int{1, 2, 3}
		if !intSlicesEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}

		// No sets
		got = Intersection()
		if got != nil {
			t.Errorf("expected nil for empty intersection, got %v", got)
		}
	})

	t.Run("Union", func(t *testing.T) {
		a := map[int]struct{}{1: {}, 2: {}}
		b := map[int]struct{}{2: {}, 3: {}}
		c := map[int]struct{}{4: {}}

		// Multiple sets
		got := Union(a, b, c)
		want := []int{1, 2, 3, 4}
		if !intSlicesEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}

		// No sets
		got = Union()
		if got != nil {
			t.Errorf("expected nil for empty union, got %v", got)
		}
	})

	t.Run("Difference", func(t *testing.T) {
		a := map[int]struct{}{1: {}, 2: {}, 3: {}}
		b := map[int]struct{}{2: {}, 4: {}}

		got := Difference(a, b)
		want := []int{1, 3}
		if !intSlicesEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}

		// No overlap
		c := map[int]struct{}{1: {}}
		d := map[int]struct{}{2: {}}
		got = Difference(c, d)
		want = []int{1}
		if !intSlicesEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
	})
}

func TestNewEvaluator(t *testing.T) {
	t.Run("empty IDs", func(t *testing.T) {
		eval := NewEvaluator(nil)
		if eval == nil {
			t.Fatal("expected evaluator, got nil")
		}

		result, err := eval.EvalString("1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.SelectedIDs) != 0 {
			t.Errorf("expected empty result, got %v", result.SelectedIDs)
		}
	})

	t.Run("with IDs", func(t *testing.T) {
		eval := NewEvaluator([]int{1, 2, 3})
		result, err := eval.EvalString("1-3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !intSlicesEqual(result.SelectedIDs, []int{1, 2, 3}) {
			t.Errorf("expected [1,2,3], got %v", result.SelectedIDs)
		}
	})
}
