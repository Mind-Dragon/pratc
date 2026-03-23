package filter

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestFilterDraft(t *testing.T) {
	tests := []struct {
		name string
		pr   types.PR
		want bool
	}{
		{
			name: "draft PR rejected",
			pr:   types.PR{Number: 1, IsDraft: true},
			want: true,
		},
		{
			name: "non-draft PR accepted",
			pr:   types.PR{Number: 2, IsDraft: false},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterDraft(tt.pr)
			if got != tt.want {
				t.Errorf("FilterDraft() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterMergeConflict(t *testing.T) {
	tests := []struct {
		name string
		pr   types.PR
		want bool
	}{
		{
			name: "conflicting PR rejected",
			pr:   types.PR{Number: 1, Mergeable: "conflicting"},
			want: true,
		},
		{
			name: "mergeable PR accepted",
			pr:   types.PR{Number: 2, Mergeable: "mergeable"},
			want: false,
		},
		{
			name: "unknown mergeable status accepted",
			pr:   types.PR{Number: 3, Mergeable: "unknown"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterMergeConflict(tt.pr)
			if got != tt.want {
				t.Errorf("FilterMergeConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterCIFailure(t *testing.T) {
	tests := []struct {
		name string
		pr   types.PR
		want bool
	}{
		{
			name: "CI failure rejected",
			pr:   types.PR{Number: 1, CIStatus: "failure"},
			want: true,
		},
		{
			name: "CI success accepted",
			pr:   types.PR{Number: 2, CIStatus: "success"},
			want: false,
		},
		{
			name: "CI pending accepted",
			pr:   types.PR{Number: 3, CIStatus: "pending"},
			want: false,
		},
		{
			name: "CI unknown accepted",
			pr:   types.PR{Number: 4, CIStatus: "unknown"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterCIFailure(tt.pr)
			if got != tt.want {
				t.Errorf("FilterCIFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyFilters_Sequential(t *testing.T) {
	prs := []types.PR{
		{Number: 1, IsDraft: false, Mergeable: "mergeable", CIStatus: "success"},
		{Number: 2, IsDraft: true, Mergeable: "mergeable", CIStatus: "success"},
		{Number: 3, IsDraft: false, Mergeable: "conflicting", CIStatus: "success"},
		{Number: 4, IsDraft: false, Mergeable: "mergeable", CIStatus: "failure"},
		{Number: 5, IsDraft: false, Mergeable: "mergeable", CIStatus: "success"},
	}

	pool, rejections := ApplyFilters(prs, true)

	if len(pool) != 2 {
		t.Errorf("ApplyFilters() pool size = %d, want 2", len(pool))
	}

	if len(rejections) != 3 {
		t.Errorf("ApplyFilters() rejections = %d, want 3", len(rejections))
	}

	rejectionReasons := make(map[int]string)
	for _, r := range rejections {
		rejectionReasons[r.PRNumber] = r.Reason
	}

	if reason, ok := rejectionReasons[2]; !ok || reason != "draft" {
		t.Errorf("PR 2 rejection = %q, want \"draft\"", reason)
	}
	if reason, ok := rejectionReasons[3]; !ok || reason != "merge conflict" {
		t.Errorf("PR 3 rejection = %q, want \"merge conflict\"", reason)
	}
	if reason, ok := rejectionReasons[4]; !ok || reason != "ci failure" {
		t.Errorf("PR 4 rejection = %q, want \"ci failure\"", reason)
	}
}

func TestFilterBot(t *testing.T) {
	tests := []struct {
		name string
		pr   types.PR
		want bool
	}{
		{
			name: "bot PR rejected",
			pr:   types.PR{Number: 1, IsBot: true},
			want: true,
		},
		{
			name: "human PR accepted",
			pr:   types.PR{Number: 2, IsBot: false},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterBot(tt.pr)
			if got != tt.want {
				t.Errorf("FilterBot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyFilters_BotExclusion(t *testing.T) {
	prs := []types.PR{
		{Number: 1, IsDraft: false, Mergeable: "mergeable", CIStatus: "success", IsBot: false},
		{Number: 2, IsDraft: false, Mergeable: "mergeable", CIStatus: "success", IsBot: true},
		{Number: 3, IsDraft: false, Mergeable: "mergeable", CIStatus: "success", IsBot: false},
		{Number: 4, IsDraft: false, Mergeable: "mergeable", CIStatus: "success", IsBot: true},
		{Number: 5, IsDraft: false, Mergeable: "mergeable", CIStatus: "success", IsBot: false},
	}

	t.Run("bots excluded by default", func(t *testing.T) {
		pool, rejections := ApplyFilters(prs, false)
		if len(pool) != 3 {
			t.Errorf("pool size = %d, want 3", len(pool))
		}
		if len(rejections) != 2 {
			t.Errorf("rejections = %d, want 2", len(rejections))
		}
		for _, r := range rejections {
			if r.Reason != "bot pr" {
				t.Errorf("rejection reason = %q, want \"bot pr\"", r.Reason)
			}
		}
	})

	t.Run("bots included when flag is true", func(t *testing.T) {
		pool, rejections := ApplyFilters(prs, true)
		if len(pool) != 5 {
			t.Errorf("pool size = %d, want 5", len(pool))
		}
		if len(rejections) != 0 {
			t.Errorf("rejections = %d, want 0", len(rejections))
		}
	})
}
