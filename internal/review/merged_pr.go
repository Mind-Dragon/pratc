package review

import (
	"strings"
	"time"
)

// MergedPRRecord represents a normalized merged PR for comparison with open PRs.
// It contains the essential fields needed for duplicate detection and similarity
// analysis between merged PR history and currently open PRs.
type MergedPRRecord struct {
	// PRNumber is the pull request number.
	PRNumber int `json:"pr_number"`

	// Title is the pull request title (normalized for comparison).
	Title string `json:"title"`

	// Body is the pull request description/body (normalized for comparison).
	Body string `json:"body"`

	// FilesChanged is the list of files modified by this PR.
	FilesChanged []string `json:"files_changed"`

	// MergedAt is when the PR was merged.
	MergedAt time.Time `json:"merged_at"`

	// Repo is the full repository name (e.g., "owner/repo").
	Repo string `json:"repo"`
}

// NormalizeTitle returns a normalized version of the title for comparison.
// It converts to lowercase, trims whitespace, and collapses multiple spaces.
func (m MergedPRRecord) NormalizeTitle() string {
	return normalizeString(m.Title)
}

// NormalizeBody returns a normalized version of the body for comparison.
// It converts to lowercase, trims whitespace, and collapses multiple spaces.
func (m MergedPRRecord) NormalizeBody() string {
	return normalizeString(m.Body)
}

// FileSet returns the files changed as a map for efficient set operations.
// Useful for quick intersection checks when comparing with open PRs.
func (m MergedPRRecord) FileSet() map[string]struct{} {
	set := make(map[string]struct{}, len(m.FilesChanged))
	for _, f := range m.FilesChanged {
		set[f] = struct{}{}
	}
	return set
}

// HasFileOverlap returns true if this merged PR touches any of the given files.
func (m MergedPRRecord) HasFileOverlap(files []string) bool {
	if len(m.FilesChanged) == 0 || len(files) == 0 {
		return false
	}
	fileSet := m.FileSet()
	for _, f := range files {
		if _, ok := fileSet[f]; ok {
			return true
		}
	}
	return false
}

// FileOverlapCount returns the number of files that overlap between this
// merged PR and the given file list.
func (m MergedPRRecord) FileOverlapCount(files []string) int {
	if len(m.FilesChanged) == 0 || len(files) == 0 {
		return 0
	}
	fileSet := m.FileSet()
	count := 0
	for _, f := range files {
		if _, ok := fileSet[f]; ok {
			count++
		}
	}
	return count
}

// Key returns a unique identifier for this merged PR record.
// Format: "owner/repo#123"
func (m MergedPRRecord) Key() string {
	return m.Repo + "#" + string(rune('0'+m.PRNumber))
}

// String returns a human-readable representation of the merged PR record.
func (m MergedPRRecord) String() string {
	return m.Repo + "#" + string(rune('0'+m.PRNumber)) + ": " + m.Title
}

// normalizeString performs string normalization for comparison purposes.
// It converts to lowercase, trims whitespace, and collapses multiple spaces.
func normalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	// Collapse multiple spaces into single space
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// MergedPRRecordFromCache creates a MergedPRRecord from cache.MergedPR data.
// This is a convenience function for converting from the cache layer to
// the review layer representation.
func MergedPRRecordFromCache(repo string, number int, mergedAt time.Time, files []string) MergedPRRecord {
	return MergedPRRecord{
		PRNumber:     number,
		Title:        "", // Title not stored in cache.MergedPR, populated separately
		Body:         "", // Body not stored in cache.MergedPR, populated separately
		FilesChanged: files,
		MergedAt:     mergedAt,
		Repo:         repo,
	}
}
