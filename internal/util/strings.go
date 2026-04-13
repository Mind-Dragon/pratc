// Package util provides shared string utilities for prATC.
package util

import (
	"regexp"
	"strings"
)

var wordSplit = regexp.MustCompile(`[^a-z0-9]+`)

// Tokenize splits a string on non-alphanumeric characters and lowercases the result.
// This provides consistent tokenization across all callers.
func Tokenize(s string) []string {
	if s == "" {
		return nil
	}
	parts := wordSplit.Split(strings.ToLower(s), -1)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// Jaccard computes the Jaccard similarity coefficient between two string slices.
// Both inputs are normalized (trimmed and lowercased) before comparison.
func Jaccard(left, right []string) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1
	}
	leftSet := make(map[string]struct{}, len(left))
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range left {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed != "" {
			leftSet[trimmed] = struct{}{}
		}
	}
	for _, value := range right {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed != "" {
			rightSet[trimmed] = struct{}{}
		}
	}

	intersection := 0.0
	union := make(map[string]struct{}, len(leftSet)+len(rightSet))
	for value := range leftSet {
		union[value] = struct{}{}
		if _, ok := rightSet[value]; ok {
			intersection++
		}
	}
	for value := range rightSet {
		union[value] = struct{}{}
	}
	if len(union) == 0 {
		return 0
	}

	return intersection / float64(len(union))
}
