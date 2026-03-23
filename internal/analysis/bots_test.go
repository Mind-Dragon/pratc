package analysis

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestIsBotPR_AuthorPatterns(t *testing.T) {
	tests := []struct {
		name   string
		pr     types.PR
		expect bool
	}{
		{
			name:   "dependabot bot author",
			pr:     types.PR{Number: 1, Author: "dependabot[bot]", Title: "Bump golang.org/x/net from 0.17.0 to 0.20.0"},
			expect: true,
		},
		{
			name:   "renovate bot author",
			pr:     types.PR{Number: 2, Author: "renovate[bot]", Title: "Update dependency react to v18"},
			expect: true,
		},
		{
			name:   "github-actions bot author",
			pr:     types.PR{Number: 3, Author: "github-actions[bot]", Title: "Bump actions/checkout from 2 to 4"},
			expect: true,
		},
		{
			name:   "snyk-bot author",
			pr:     types.PR{Number: 4, Author: "snyk-bot", Title: "fix: Package.json vulnerability"},
			expect: true,
		},
		{
			name:   "regular human author",
			pr:     types.PR{Number: 5, Author: "johndoe", Title: "Add new feature"},
			expect: false,
		},
		{
			name:   "empty author with non-bot title",
			pr:     types.PR{Number: 6, Author: "", Title: "Add new feature"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBotPR(tt.pr)
			if got != tt.expect {
				t.Errorf("IsBotPR() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestIsBotPR_TitlePatterns(t *testing.T) {
	tests := []struct {
		name   string
		pr     types.PR
		expect bool
	}{
		{
			name:   "Bump prefix",
			pr:     types.PR{Number: 101, Author: "someuser", Title: "Bump lodash from 4.17.20 to 4.17.21"},
			expect: true,
		},
		{
			name:   "chore(deps) prefix",
			pr:     types.PR{Number: 102, Author: "someuser", Title: "chore(deps): update all dependencies"},
			expect: true,
		},
		{
			name:   "Update dependency prefix",
			pr:     types.PR{Number: 103, Author: "someuser", Title: "Update dependency @types/node to v20"},
			expect: true,
		},
		{
			name:   "regular feature title",
			pr:     types.PR{Number: 104, Author: "someuser", Title: "Add dark mode support"},
			expect: false,
		},
		{
			name:   "fix bug title",
			pr:     types.PR{Number: 105, Author: "someuser", Title: "Fix login page crash"},
			expect: false,
		},
		{
			name:   "lowercase bump",
			pr:     types.PR{Number: 106, Author: "someuser", Title: "bump lodash version"},
			expect: false,
		},
		{
			name:   "chore without deps",
			pr:     types.PR{Number: 107, Author: "someuser", Title: "chore: refactor utils"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBotPR(tt.pr)
			if got != tt.expect {
				t.Errorf("IsBotPR() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestDetectBots(t *testing.T) {
	prs := []types.PR{
		{Number: 1, Author: "dependabot[bot]", Title: "Bump golang.org/x/net"},
		{Number: 2, Author: "johndoe", Title: "Add new feature"},
		{Number: 3, Author: "renovate[bot]", Title: "Update dependency react"},
		{Number: 4, Author: "janedoe", Title: "Fix bug in auth"},
		{Number: 5, Author: "github-actions[bot]", Title: "Bump actions/cache"},
	}

	result := DetectBots(prs)

	if !result[0].IsBot {
		t.Error("PR 1 should be marked as bot")
	}
	if result[1].IsBot {
		t.Error("PR 2 should NOT be marked as bot")
	}
	if !result[2].IsBot {
		t.Error("PR 3 should be marked as bot")
	}
	if result[3].IsBot {
		t.Error("PR 4 should NOT be marked as bot")
	}
	if !result[4].IsBot {
		t.Error("PR 5 should be marked as bot")
	}
}

func TestDetectBots_EmptySlice(t *testing.T) {
	prs := []types.PR{}
	result := DetectBots(prs)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}
