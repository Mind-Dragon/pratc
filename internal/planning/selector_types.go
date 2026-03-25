package planning

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Selector AST types for PR selection expressions.
// Supports: explicit IDs, ranges, AND, OR with deterministic precedence.
//
// Grammar:
//
//	expr       → orExpr
//	orExpr     → andExpr (OR andExpr)*
//	andExpr    → term (AND term)*
//	term       → '(' expr ')' | atomic
//	atomic     → ID | RANGE
//	ID         → integer (e.g., 123)
//	RANGE      → ID '-' ID (e.g., 100-200)
//
// Precedence (tightest to loosest):
// 1. Parentheses
// 2. AND (binds tighter)
// 3. OR (loosest)
//
// Examples:
//   - "123" → single PR
//   - "100-200" → PRs 100 through 200 inclusive
//   - "1 AND 2 OR 3" → (1 AND 2) OR 3 = {3} (since 1 AND 2 = ∅)
//   - "1 OR 2 AND 3" → 1 OR (2 AND 3) = {1, 2, 3} (since 2 AND 3 = {2,3} if both exist)
//   - "1-5 AND 3-7" → {3, 4, 5} (intersection)

// SelectExpr is the root of the selector AST.
type SelectExpr struct {
	Root SelectNode
}

// SelectNode represents a node in the selector AST.
type SelectNode interface {
	nodeType() string
}

// SelectOr represents an OR operation between child nodes.
type SelectOr struct {
	Children []SelectNode
}

func (n *SelectOr) nodeType() string { return "OR" }

// SelectAnd represents an AND operation between child nodes.
type SelectAnd struct {
	Children []SelectNode
}

func (n *SelectAnd) nodeType() string { return "AND" }

// SelectIDSet represents an explicit set of PR IDs.
type SelectIDSet struct {
	IDs []int
}

func (n *SelectIDSet) nodeType() string { return "IDSet" }

// SelectRange represents a range of PR IDs (inclusive).
type SelectRange struct {
	Start int
	End   int
}

func (n *SelectRange) nodeType() string { return "Range" }

// SelectorError represents a selector parsing or evaluation error.
type SelectorError struct {
	Code    SelectorErrorCode
	Message string
}

func (e *SelectorError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// SelectorErrorCode enumerates selector error codes.
type SelectorErrorCode string

const (
	ErrSelectorInvalidSyntax   SelectorErrorCode = "INVALID_SYNTAX"
	ErrSelectorEmptyExpression SelectorErrorCode = "EMPTY_EXPRESSION"
	ErrSelectorInvalidRange    SelectorErrorCode = "INVALID_RANGE"
	ErrSelectorInvalidID       SelectorErrorCode = "INVALID_ID"
	ErrSelectorUnexpectedToken SelectorErrorCode = "UNEXPECTED_TOKEN"
	ErrSelectorMismatchedParen SelectorErrorCode = "MISMATCHED_PAREN"
)

// NewSelectorError creates a new selector error with the given code and message.
func NewSelectorError(code SelectorErrorCode, format string, args ...any) *SelectorError {
	return &SelectorError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// IsSelectorError checks if err is a SelectorError with the given code.
func IsSelectorError(err error, code SelectorErrorCode) bool {
	var se *SelectorError
	if errors.As(err, &se) {
		return se.Code == code
	}
	return false
}

// ToHTTPError converts a selector error to an HTTP error payload.
// Returns status code 400 for all selector errors.
func (e *SelectorError) ToHTTPError() map[string]any {
	return map[string]any{
		"error":   string(e.Code),
		"message": e.Message,
		"status":  "400 Bad Request",
	}
}

// Expand returns the set of PR IDs represented by a node.
// This is used for debugging and testing.
func Expand(node SelectNode) map[int]struct{} {
	result := make(map[int]struct{})
	expandInto(node, result)
	return result
}

func expandInto(node SelectNode, acc map[int]struct{}) {
	switch n := node.(type) {
	case *SelectIDSet:
		for _, id := range n.IDs {
			acc[id] = struct{}{}
		}
	case *SelectRange:
		for id := n.Start; id <= n.End; id++ {
			acc[id] = struct{}{}
		}
	case *SelectAnd:
		if len(n.Children) == 0 {
			return
		}
		first := make(map[int]struct{})
		expandInto(n.Children[0], first)
		for _, child := range n.Children[1:] {
			childSet := make(map[int]struct{})
			expandInto(child, childSet)
			for id := range first {
				if _, ok := childSet[id]; !ok {
					delete(first, id)
				}
			}
		}
		for id := range first {
			acc[id] = struct{}{}
		}
	case *SelectOr:
		for _, child := range n.Children {
			expandInto(child, acc)
		}
	}
}

// String returns a string representation of the selector expression.
func (e *SelectExpr) String() string {
	if e.Root == nil {
		return "<empty>"
	}
	return nodeString(e.Root)
}

func nodeString(node SelectNode) string {
	switch n := node.(type) {
	case *SelectIDSet:
		if len(n.IDs) == 1 {
			return strconv.Itoa(n.IDs[0])
		}
		ids := make([]string, len(n.IDs))
		for i, id := range n.IDs {
			ids[i] = strconv.Itoa(id)
		}
		return "{" + strings.Join(ids, ",") + "}"
	case *SelectRange:
		if n.Start == n.End {
			return strconv.Itoa(n.Start)
		}
		return fmt.Sprintf("%d-%d", n.Start, n.End)
	case *SelectAnd:
		parts := make([]string, len(n.Children))
		for i, child := range n.Children {
			parts[i] = nodeString(child)
		}
		return "(" + strings.Join(parts, " AND ") + ")"
	case *SelectOr:
		parts := make([]string, len(n.Children))
		for i, child := range n.Children {
			parts[i] = nodeString(child)
		}
		return "(" + strings.Join(parts, " OR ") + ")"
	default:
		return "<unknown>"
	}
}

// AllIDs returns all unique PR IDs referenced in the expression (for validation).
func (e *SelectExpr) AllIDs() []int {
	if e.Root == nil {
		return nil
	}
	expanded := Expand(e.Root)
	ids := make([]int, 0, len(expanded))
	for id := range expanded {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// IDCount returns the total number of unique PR IDs in the expression.
func (e *SelectExpr) IDCount() int {
	if e.Root == nil {
		return 0
	}
	return len(Expand(e.Root))
}
