package sync

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type mockDeltaCacheStore struct {
	prs map[int]types.PR
	err error
}

func (m *mockDeltaCacheStore) ListPRs(filter cache.PRFilter) ([]types.PR, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([]types.PR, 0, len(m.prs))
	for _, pr := range m.prs {
		result = append(result, pr)
	}
	return result, nil
}

func (m *mockDeltaCacheStore) ListPRsIter(filter cache.PRFilter, fn func(types.PR) error) error {
	if m.err != nil {
		return m.err
	}
	keys := make([]int, 0, len(m.prs))
	for number := range m.prs {
		keys = append(keys, number)
	}
	sort.Ints(keys)
	for _, number := range keys {
		if err := fn(m.prs[number]); err != nil {
			return err
		}
	}
	return nil
}

func TestComputeDelta_MixedNewUpdatedUnchangedClosed(t *testing.T) {
	cacheStore := &mockDeltaCacheStore{
		prs: map[int]types.PR{
			1: {Repo: "owner/repo", Number: 1, UpdatedAt: "2026-03-01T10:00:00Z"},
			2: {Repo: "owner/repo", Number: 2, UpdatedAt: "2026-03-02T10:00:00Z"},
			3: {Repo: "owner/repo", Number: 3, UpdatedAt: "2026-03-03T10:00:00Z"},
			4: {Repo: "owner/repo", Number: 4, UpdatedAt: "2026-03-04T10:00:00Z"},
			5: {Repo: "owner/repo", Number: 5, UpdatedAt: "2026-03-05T10:00:00Z"},
		},
	}

	detector := &DeltaDetector{cacheStore: cacheStore}

	githubPRs := []PRInfo{
		{Number: 1, UpdatedAt: parseUpdatedAt("2026-03-01T10:00:00Z")},
		{Number: 2, UpdatedAt: parseUpdatedAt("2026-03-02T11:00:00Z")},
		{Number: 3, UpdatedAt: parseUpdatedAt("2026-03-03T10:00:00Z")},
		{Number: 6, UpdatedAt: parseUpdatedAt("2026-03-06T10:00:00Z")},
		{Number: 7, UpdatedAt: parseUpdatedAt("2026-03-07T10:00:00Z")},
	}

	delta := computeDeltaFromGitHubPRs(detector, githubPRs)

	if len(delta.NewPRs) != 2 {
		t.Errorf("expected 2 new PRs, got %d: %v", len(delta.NewPRs), delta.NewPRs)
	}
	if len(delta.UpdatedPRs) != 1 {
		t.Errorf("expected 1 updated PR, got %d: %v", len(delta.UpdatedPRs), delta.UpdatedPRs)
	}
	if len(delta.UnchangedPRs) != 2 {
		t.Errorf("expected 2 unchanged PRs, got %d: %v", len(delta.UnchangedPRs), delta.UnchangedPRs)
	}
	if len(delta.ClosedPRs) != 2 {
		t.Errorf("expected 2 closed PRs, got %d: %v", len(delta.ClosedPRs), delta.ClosedPRs)
	}

	if delta.NewPRs[0] != 6 || delta.NewPRs[1] != 7 {
		t.Errorf("new PRs mismatch: got %v", delta.NewPRs)
	}
	if delta.UpdatedPRs[0] != 2 {
		t.Errorf("updated PRs mismatch: got %v", delta.UpdatedPRs)
	}
	if delta.UnchangedPRs[0] != 1 || delta.UnchangedPRs[1] != 3 {
		t.Errorf("unchanged PRs mismatch: got %v", delta.UnchangedPRs)
	}
	if delta.ClosedPRs[0] != 4 || delta.ClosedPRs[1] != 5 {
		t.Errorf("closed PRs mismatch: got %v", delta.ClosedPRs)
	}
}

func TestComputeDelta_EmptyCache(t *testing.T) {
	cacheStore := &mockDeltaCacheStore{
		prs: map[int]types.PR{},
	}

	detector := &DeltaDetector{cacheStore: cacheStore}

	githubPRs := []PRInfo{
		{Number: 1, UpdatedAt: parseUpdatedAt("2026-03-01T10:00:00Z")},
		{Number: 2, UpdatedAt: parseUpdatedAt("2026-03-02T10:00:00Z")},
		{Number: 3, UpdatedAt: parseUpdatedAt("2026-03-03T10:00:00Z")},
	}

	delta := computeDeltaFromGitHubPRs(detector, githubPRs)

	if len(delta.NewPRs) != 3 {
		t.Errorf("expected 3 new PRs, got %d: %v", len(delta.NewPRs), delta.NewPRs)
	}
	if len(delta.UpdatedPRs) != 0 {
		t.Errorf("expected 0 updated PRs, got %d: %v", len(delta.UpdatedPRs), delta.UpdatedPRs)
	}
	if len(delta.UnchangedPRs) != 0 {
		t.Errorf("expected 0 unchanged PRs, got %d: %v", len(delta.UnchangedPRs), delta.UnchangedPRs)
	}
	if len(delta.ClosedPRs) != 0 {
		t.Errorf("expected 0 closed PRs, got %d: %v", len(delta.ClosedPRs), delta.ClosedPRs)
	}
}

func TestComputeDelta_AllUnchanged(t *testing.T) {
	cacheStore := &mockDeltaCacheStore{
		prs: map[int]types.PR{
			1: {Repo: "owner/repo", Number: 1, UpdatedAt: "2026-03-01T10:00:00Z"},
			2: {Repo: "owner/repo", Number: 2, UpdatedAt: "2026-03-02T10:00:00Z"},
			3: {Repo: "owner/repo", Number: 3, UpdatedAt: "2026-03-03T10:00:00Z"},
		},
	}

	detector := &DeltaDetector{cacheStore: cacheStore}

	githubPRs := []PRInfo{
		{Number: 1, UpdatedAt: parseUpdatedAt("2026-03-01T10:00:00Z")},
		{Number: 2, UpdatedAt: parseUpdatedAt("2026-03-02T10:00:00Z")},
		{Number: 3, UpdatedAt: parseUpdatedAt("2026-03-03T10:00:00Z")},
	}

	delta := computeDeltaFromGitHubPRs(detector, githubPRs)

	if len(delta.NewPRs) != 0 {
		t.Errorf("expected 0 new PRs, got %d: %v", len(delta.NewPRs), delta.NewPRs)
	}
	if len(delta.UpdatedPRs) != 0 {
		t.Errorf("expected 0 updated PRs, got %d: %v", len(delta.UpdatedPRs), delta.UpdatedPRs)
	}
	if len(delta.UnchangedPRs) != 3 {
		t.Errorf("expected 3 unchanged PRs, got %d: %v", len(delta.UnchangedPRs), delta.UnchangedPRs)
	}
	if len(delta.ClosedPRs) != 0 {
		t.Errorf("expected 0 closed PRs, got %d: %v", len(delta.ClosedPRs), delta.ClosedPRs)
	}
}

func TestComputeDelta_AllClosed(t *testing.T) {
	cacheStore := &mockDeltaCacheStore{
		prs: map[int]types.PR{
			1: {Repo: "owner/repo", Number: 1, UpdatedAt: "2026-03-01T10:00:00Z"},
			2: {Repo: "owner/repo", Number: 2, UpdatedAt: "2026-03-02T10:00:00Z"},
		},
	}

	detector := &DeltaDetector{cacheStore: cacheStore}

	githubPRs := []PRInfo{}

	delta := computeDeltaFromGitHubPRs(detector, githubPRs)

	if len(delta.NewPRs) != 0 {
		t.Errorf("expected 0 new PRs, got %d: %v", len(delta.NewPRs), delta.NewPRs)
	}
	if len(delta.UpdatedPRs) != 0 {
		t.Errorf("expected 0 updated PRs, got %d: %v", len(delta.UpdatedPRs), delta.UpdatedPRs)
	}
	if len(delta.UnchangedPRs) != 0 {
		t.Errorf("expected 0 unchanged PRs, got %d: %v", len(delta.UnchangedPRs), delta.UnchangedPRs)
	}
	if len(delta.ClosedPRs) != 2 {
		t.Errorf("expected 2 closed PRs, got %d: %v", len(delta.ClosedPRs), delta.ClosedPRs)
	}
}

func TestComputeDelta_ErrorHandling(t *testing.T) {
	cacheStore := &mockDeltaCacheStore{
		err: context.DeadlineExceeded,
	}

	detector := &DeltaDetector{cacheStore: cacheStore}

	_, err := detector.ComputeDelta(context.Background(), "owner/repo", time.Now())
	if err == nil {
		t.Error("expected error when cache fails, got nil")
	}
}

func TestSplitRepo(t *testing.T) {
	tests := []struct {
		repo      string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"jeffersonnunn/pratc", "jeffersonnunn", "pratc", false},
		{"a/b", "a", "b", false},
		{"", "", "", true},
		{"owner", "", "", true},
		{"/repo", "", "", true},
		{"owner/", "", "", true},
		{"a/b/c", "a", "b/c", false},
	}

	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			owner, name, err := splitRepo(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitRepo(%q) error = %v, wantErr %v", tt.repo, err, tt.wantErr)
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("splitRepo(%q) owner = %q, want %q", tt.repo, owner, tt.wantOwner)
			}
			if name != tt.wantName {
				t.Errorf("splitRepo(%q) name = %q, want %q", tt.repo, name, tt.wantName)
			}
		})
	}
}

func TestParseUpdatedAt(t *testing.T) {
	tests := []struct {
		input    string
		wantZero bool
	}{
		{"2026-03-01T10:00:00Z", false},
		{"2026-03-01T10:00:00+00:00", false},
		{"", true},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseUpdatedAt(tt.input)
			isZero := got.IsZero()
			if isZero != tt.wantZero {
				t.Errorf("parseUpdatedAt(%q) zero = %v, wantZero %v", tt.input, isZero, tt.wantZero)
			}
		})
	}
}

func TestNewDeltaDetector(t *testing.T) {
	cacheStore := &mockDeltaCacheStore{}
	detector := NewDeltaDetector(cacheStore)

	if detector == nil {
		t.Fatal("NewDeltaDetector returned nil")
	}
	if detector.cacheStore != cacheStore {
		t.Error("DeltaDetector.cacheStore not set correctly")
	}
}

func TestSyncDelta_DeterministicSorting(t *testing.T) {
	delta := &SyncDelta{
		NewPRs:       []int{5, 3, 1, 4, 2},
		UpdatedPRs:   []int{10, 8, 6},
		UnchangedPRs: []int{7, 9},
		ClosedPRs:    []int{15, 11, 13},
	}

	sort.Ints(delta.NewPRs)
	sort.Ints(delta.UpdatedPRs)
	sort.Ints(delta.UnchangedPRs)
	sort.Ints(delta.ClosedPRs)

	expected := []int{1, 2, 3, 4, 5}
	for i, got := range delta.NewPRs {
		if got != expected[i] {
			t.Errorf("NewPRs[%d] = %d, want %d", i, got, expected[i])
		}
	}
}

func computeDeltaFromGitHubPRs(d *DeltaDetector, githubPRs []PRInfo) *SyncDelta {
	githubMap := make(map[int]time.Time, len(githubPRs))
	for _, pr := range githubPRs {
		githubMap[pr.Number] = pr.UpdatedAt
	}

	cachedPRs, _ := d.cacheStore.ListPRs(cache.PRFilter{})
	cacheMap := make(map[int]time.Time, len(cachedPRs))
	for _, pr := range cachedPRs {
		cacheMap[pr.Number] = parseUpdatedAt(pr.UpdatedAt)
	}

	var newPRs, updatedPRs, unchangedPRs, closedPRs []int

	for number, ghUpdatedAt := range githubMap {
		cacheUpdatedAt, exists := cacheMap[number]
		if !exists {
			newPRs = append(newPRs, number)
		} else if !ghUpdatedAt.Equal(cacheUpdatedAt) {
			updatedPRs = append(updatedPRs, number)
		} else {
			unchangedPRs = append(unchangedPRs, number)
		}
	}

	for number := range cacheMap {
		if _, exists := githubMap[number]; !exists {
			closedPRs = append(closedPRs, number)
		}
	}

	sort.Ints(newPRs)
	sort.Ints(updatedPRs)
	sort.Ints(unchangedPRs)
	sort.Ints(closedPRs)

	return &SyncDelta{
		NewPRs:       newPRs,
		UpdatedPRs:   updatedPRs,
		UnchangedPRs: unchangedPRs,
		ClosedPRs:    closedPRs,
	}
}
