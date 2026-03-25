package planning

import (
	"testing"
)

func TestParse_SingleID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIDs []int
		wantErr bool
		errCode SelectorErrorCode
	}{
		{"single digit", "1", []int{1}, false, ""},
		{"multi digit", "123", []int{123}, false, ""},
		{"with spaces", "  42  ", []int{42}, false, ""},
		{"large number", "9999", []int{9999}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.errCode != "" {
					if !IsSelectorError(err, tt.errCode) {
						t.Errorf("expected error code %s, got %v", tt.errCode, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotIDs := expr.AllIDs()
			if !intSlicesEqual(gotIDs, tt.wantIDs) {
				t.Errorf("expected IDs %v, got %v", tt.wantIDs, gotIDs)
			}
		})
	}
}

func TestParse_Range(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIDs []int
		wantErr bool
		errCode SelectorErrorCode
	}{
		{"simple range", "1-5", []int{1, 2, 3, 4, 5}, false, ""},
		{"single element range", "5-5", []int{5}, false, ""},
		{"large range", "100-105", []int{100, 101, 102, 103, 104, 105}, false, ""},
		{"with spaces", "  10 - 20  ", []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, false, ""},
		{"invalid range reversed", "10-5", nil, true, ErrSelectorInvalidRange},
		{"missing end", "10-", nil, true, ErrSelectorInvalidSyntax},
		{"missing start", "-10", nil, true, ErrSelectorInvalidSyntax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.errCode != "" {
					if !IsSelectorError(err, tt.errCode) {
						t.Errorf("expected error code %s, got %v", tt.errCode, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotIDs := expr.AllIDs()
			if !intSlicesEqual(gotIDs, tt.wantIDs) {
				t.Errorf("expected IDs %v, got %v", tt.wantIDs, gotIDs)
			}
		})
	}
}

func TestParse_AND(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIDs []int
		wantErr bool
	}{
		// AND computes intersection: "1 AND 2" = {1} ∩ {2} = ∅
		{"two IDs disjoint", "1 AND 2", []int{}, false},
		// "1 AND 1" = {1} ∩ {1} = {1}
		{"two same IDs", "1 AND 1", []int{1}, false},
		// "1 AND 5-10" = {1} ∩ {5,6,7,8,9,10} = ∅
		{"ID and range disjoint", "1 AND 5-10", []int{}, false},
		// "3 AND 5-10" = {3} ∩ {5,6,7,8,9,10} = ∅
		{"ID in range", "5 AND 5-10", []int{5}, false},
		// "1-3 AND 5-7" = {1,2,3} ∩ {5,6,7} = ∅
		{"two ranges disjoint", "1-3 AND 5-7", []int{}, false},
		// "3-6 AND 5-7" = {3,4,5,6} ∩ {5,6,7} = {5,6}
		{"two ranges overlapping", "3-6 AND 5-8", []int{5, 6}, false},
		// "1 AND 2 AND 3" = {1} ∩ {2} ∩ {3} = ∅
		{"multiple AND disjoint", "1 AND 2 AND 3", []int{}, false},
		// "5 AND 5 AND 5" = {5} ∩ {5} ∩ {5} = {5}
		{"multiple AND same", "5 AND 5 AND 5", []int{5}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotIDs := expr.AllIDs()
			if !intSlicesEqual(gotIDs, tt.wantIDs) {
				t.Errorf("expected IDs %v, got %v", tt.wantIDs, gotIDs)
			}
		})
	}
}

func TestParse_OR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIDs []int
		wantErr bool
	}{
		{"two IDs", "1 OR 2", []int{1, 2}, false},
		{"ID and range", "1 OR 5-7", []int{1, 5, 6, 7}, false},
		{"two ranges", "1-3 OR 5-7", []int{1, 2, 3, 5, 6, 7}, false},
		{"multiple OR", "1 OR 2 OR 3", []int{1, 2, 3}, false},
		{"overlapping ranges", "1-5 OR 3-7", []int{1, 2, 3, 4, 5, 6, 7}, false},
		{"case insensitive", "1 or 2", []int{1, 2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotIDs := expr.AllIDs()
			if !intSlicesEqual(gotIDs, tt.wantIDs) {
				t.Errorf("expected IDs %v, got %v", tt.wantIDs, gotIDs)
			}
		})
	}
}

func TestParse_Precedence_AND_binds_tighter_than_OR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIDs []int
	}{
		{"or then and", "1 OR 2 AND 3", []int{1}},
		{"and then or", "1 AND 2 OR 3", []int{3}},
		{"complex precedence", "1 OR 2 AND 3 OR 4", []int{1, 4}},
		// "1-5 AND 3-7 OR 10" = "(1-5 AND 3-7) OR 10" = {3, 4, 5, 10}
		{"range and or", "1-5 AND 3-7 OR 10", []int{3, 4, 5, 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotIDs := expr.AllIDs()
			if !intSlicesEqual(gotIDs, tt.wantIDs) {
				t.Errorf("expected IDs %v, got %v", tt.wantIDs, gotIDs)
			}
		})
	}
}

func TestParse_Parentheses(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIDs []int
		wantErr bool
		errCode SelectorErrorCode
	}{
		{"override precedence", "(1 OR 2) AND 3", []int{}, false, ""},
		{"nested parens", "((1))", []int{1}, false, ""},
		{"complex grouping", "(1 OR 2) AND (3 OR 4)", []int{}, false, ""},
		{"parens with ranges", "(1-5) AND (3-7)", []int{3, 4, 5}, false, ""},
		{"mismatched open paren", "(1 OR 2", nil, true, ErrSelectorMismatchedParen},
		{"mismatched close paren", "1 OR 2)", nil, true, ErrSelectorUnexpectedToken},
		{"empty parens", "()", nil, true, ErrSelectorInvalidSyntax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.errCode != "" {
					if !IsSelectorError(err, tt.errCode) {
						t.Errorf("expected error code %s, got %v", tt.errCode, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotIDs := expr.AllIDs()
			if !intSlicesEqual(gotIDs, tt.wantIDs) {
				t.Errorf("expected IDs %v, got %v", tt.wantIDs, gotIDs)
			}
		})
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errCode SelectorErrorCode
	}{
		{"empty string", "", true, ErrSelectorEmptyExpression},
		{"whitespace only", "   ", true, ErrSelectorEmptyExpression},
		{"invalid token", "1 @ 2", true, ErrSelectorUnexpectedToken},
		{"missing operand after AND", "1 AND", true, ErrSelectorInvalidSyntax},
		{"missing operand after OR", "1 OR", true, ErrSelectorInvalidSyntax},
		{"double minus", "1--2", true, ErrSelectorInvalidSyntax},
		{"non-numeric ID", "abc", true, ErrSelectorInvalidID},
		{"partial range", "1-2-3", true, ErrSelectorUnexpectedToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error but got none")
			}

			if tt.errCode != "" {
				if !IsSelectorError(err, tt.errCode) {
					t.Errorf("expected error code %s, got %v", tt.errCode, err)
				}
			}
		})
	}
}

func TestParseAndValidate(t *testing.T) {
	availableIDs := []int{1, 2, 3, 5, 8, 10, 15, 20}

	tests := []struct {
		name         string
		input        string
		wantIDs      []int
		wantWarnings []string
		wantErr      bool
	}{
		{
			name:         "all IDs exist",
			input:        "1 OR 2 OR 3",
			wantIDs:      []int{1, 2, 3},
			wantWarnings: nil,
			wantErr:      false,
		},
		{
			name:         "some IDs missing",
			input:        "1 OR 4 OR 7",
			wantIDs:      []int{1, 4, 7},
			wantWarnings: []string{"PR #4 not found", "PR #7 not found"},
			wantErr:      false,
		},
		{
			name:         "range with some missing",
			input:        "1-5",
			wantIDs:      []int{1, 2, 3, 4, 5},
			wantWarnings: []string{"PR #4 not found"},
			wantErr:      false,
		},
		{
			name:         "parse error",
			input:        "1 @ 2",
			wantIDs:      nil,
			wantWarnings: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, warnings, err := ParseAndValidate(tt.input, availableIDs)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			gotIDs := expr.AllIDs()
			if !intSlicesEqual(gotIDs, tt.wantIDs) {
				t.Errorf("expected IDs %v, got %v", tt.wantIDs, gotIDs)
			}

			if !stringSlicesEqual(warnings, tt.wantWarnings) {
				t.Errorf("expected warnings %v, got %v", tt.wantWarnings, warnings)
			}
		})
	}
}

func TestSelectExpr_String(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single ID", "123", "123"},
		{"range", "10-20", "10-20"},
		{"AND expression", "1 AND 2", "(1 AND 2)"},
		{"OR expression", "1 OR 2", "(1 OR 2)"},
		{"complex", "1 OR 2 AND 3", "(1 OR (2 AND 3))"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			got := expr.String()
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestSelectExpr_IDCount(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{"single ID", "123", 1},
		{"range", "10-15", 6},
		{"OR (no overlap)", "1 OR 5", 2},
		{"OR (with overlap)", "1-5 OR 3-7", 7},
		{"AND (intersection)", "1-5 AND 3-7", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			got := expr.IDCount()
			if got != tt.wantCount {
				t.Errorf("expected count %d, got %d", tt.wantCount, got)
			}
		})
	}
}

// Helper functions

func intSlicesEqual(a, b []int) bool {
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

func stringSlicesEqual(a, b []string) bool {
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
