package planning

import (
	"sort"
)

// Evaluator evaluates selector expressions against a set of available PRs.
type Evaluator struct {
	availableIDs map[int]struct{}
}

// NewEvaluator creates a new evaluator with the given available PR IDs.
func NewEvaluator(availableIDs []int) *Evaluator {
	set := make(map[int]struct{}, len(availableIDs))
	for _, id := range availableIDs {
		set[id] = struct{}{}
	}
	return &Evaluator{availableIDs: set}
}

// EvalResult contains the result of evaluating a selector expression.
type EvalResult struct {
	// SelectedIDs is the sorted list of selected PR IDs.
	SelectedIDs []int
	// Warnings contains warnings about IDs in the expression that don't exist.
	Warnings []string
	// Expression is the original expression that was evaluated.
	Expression string
}

// Eval evaluates a selector expression and returns the selected PR IDs.
// IDs in the expression that don't exist in the available set generate warnings
// but do not cause evaluation to fail.
func (e *Evaluator) Eval(expr *SelectExpr) *EvalResult {
	if expr == nil || expr.Root == nil {
		return &EvalResult{
			SelectedIDs: nil,
			Warnings:    []string{"empty expression"},
			Expression:  "",
		}
	}

	// Expand the expression to get all referenced IDs
	referencedIDs := Expand(expr.Root)

	// Filter to only available IDs and collect warnings
	var warnings []string
	selectedMap := make(map[int]struct{})

	for id := range referencedIDs {
		if _, ok := e.availableIDs[id]; ok {
			selectedMap[id] = struct{}{}
		} else {
			warnings = append(warnings, "PR #"+itoa(id)+" not found")
		}
	}

	// Convert to sorted slice for deterministic output
	selectedIDs := make([]int, 0, len(selectedMap))
	for id := range selectedMap {
		selectedIDs = append(selectedIDs, id)
	}
	sort.Ints(selectedIDs)

	// Sort warnings for determinism
	sort.Strings(warnings)

	return &EvalResult{
		SelectedIDs: selectedIDs,
		Warnings:    warnings,
		Expression:  expr.String(),
	}
}

// EvalString parses and evaluates a selector expression string.
// This is a convenience method that combines Parse and Eval.
func (e *Evaluator) EvalString(input string) (*EvalResult, error) {
	expr, err := Parse(input)
	if err != nil {
		return nil, err
	}
	return e.Eval(expr), nil
}

// Contains checks if a selector expression matches a specific PR ID.
// Returns true if the ID would be selected by the expression.
func (e *Evaluator) Contains(expr *SelectExpr, prID int) bool {
	if expr == nil || expr.Root == nil {
		return false
	}

	// Check if the ID is in the expanded set
	expanded := Expand(expr.Root)
	_, ok := expanded[prID]
	return ok
}

// Count returns the number of PRs that would be selected by the expression.
// This counts only available PRs, not all referenced IDs.
func (e *Evaluator) Count(expr *SelectExpr) int {
	if expr == nil || expr.Root == nil {
		return 0
	}

	result := e.Eval(expr)
	return len(result.SelectedIDs)
}

// IsEmpty returns true if the expression selects no PRs.
func (e *Evaluator) IsEmpty(expr *SelectExpr) bool {
	return e.Count(expr) == 0
}

// ToPredicate converts an evaluator to a predicate function.
// The returned function can be used to test if a PR ID matches the expression.
func (e *Evaluator) ToPredicate(expr *SelectExpr) func(int) bool {
	expanded := Expand(expr.Root)
	return func(id int) bool {
		_, ok := expanded[id]
		return ok && e.isAvailable(id)
	}
}

func (e *Evaluator) isAvailable(id int) bool {
	_, ok := e.availableIDs[id]
	return ok
}

// itoa converts an integer to string without allocations for small numbers.
// This is a simple optimization for the hot path.
func itoa(n int) string {
	if n < 10 {
		return string('0' + byte(n))
	}
	if n < 100 {
		return string('0'+byte(n/10)) + string('0'+byte(n%10))
	}
	return string(rune('0'+byte(n/100))) + string('0'+byte((n/10)%10)) + string('0'+byte(n%10))
}

// Intersection computes the intersection of multiple ID sets.
// Returns a sorted slice of IDs present in all sets.
func Intersection(sets ...map[int]struct{}) []int {
	if len(sets) == 0 {
		return nil
	}
	if len(sets) == 1 {
		return mapKeys(sets[0])
	}

	// Start with the first set
	result := make(map[int]struct{})
	for id := range sets[0] {
		result[id] = struct{}{}
	}

	// Intersect with remaining sets
	for _, set := range sets[1:] {
		for id := range result {
			if _, ok := set[id]; !ok {
				delete(result, id)
			}
		}
	}

	return mapKeys(result)
}

// Union computes the union of multiple ID sets.
// Returns a sorted slice of IDs present in any set.
func Union(sets ...map[int]struct{}) []int {
	if len(sets) == 0 {
		return nil
	}

	result := make(map[int]struct{})
	for _, set := range sets {
		for id := range set {
			result[id] = struct{}{}
		}
	}

	return mapKeys(result)
}

// Difference computes the set difference (a - b).
// Returns a sorted slice of IDs in a but not in b.
func Difference(a, b map[int]struct{}) []int {
	result := make(map[int]struct{})
	for id := range a {
		if _, ok := b[id]; !ok {
			result[id] = struct{}{}
		}
	}
	return mapKeys(result)
}

func mapKeys(m map[int]struct{}) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
