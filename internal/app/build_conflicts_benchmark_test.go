package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestBuildConflictsMinimumSharedFiles(t *testing.T) {
	t.Parallel()
	prs := []types.PR{
		{
			Repo:         "owner/repo",
			Number:       1,
			Title:        "left",
			BaseBranch:   "main",
			HeadBranch:   "left",
			FilesChanged: []string{"src/a.go", "src/b.go", "README.md"},
			Mergeable:    "mergeable",
		},
		{
			Repo:         "owner/repo",
			Number:       2,
			Title:        "right",
			BaseBranch:   "main",
			HeadBranch:   "right",
			FilesChanged: []string{"src/a.go", "src/b.go", "README.md"},
			Mergeable:    "mergeable",
		},
	}

	withMinTwo := buildConflictsWithMinSignalFiles("owner/repo", prs, nil, 2)
	if len(withMinTwo) != 1 {
		t.Fatalf("min=2 conflicts=%d, want 1", len(withMinTwo))
	}

	withMinThree := buildConflictsWithMinSignalFiles("owner/repo", prs, nil, 3)
	if len(withMinThree) != 0 {
		t.Fatalf("min=3 conflicts=%d, want 0", len(withMinThree))
	}
}

func TestBuildConflictsFixtureThresholdComparison(t *testing.T) {
	prs, err := testutil.LoadFixturePRs()
	if err != nil {
		t.Fatalf("load fixture prs: %v", err)
	}
	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	withMinTwo := buildConflictsWithMinSignalFiles(manifest.Repo, prs, nil, 2)
	withMinThree := buildConflictsWithMinSignalFiles(manifest.Repo, prs, nil, 3)
	if len(withMinThree) > len(withMinTwo) {
		t.Fatalf("min=3 conflicts=%d exceeds min=2 conflicts=%d", len(withMinThree), len(withMinTwo))
	}

	t.Logf("fixture repo=%s prs=%d conflicts(min=2)=%d conflicts(min=3)=%d delta=%d", manifest.Repo, len(prs), len(withMinTwo), len(withMinThree), len(withMinTwo)-len(withMinThree))
}

func BenchmarkBuildConflictsCorpusScale(b *testing.B) {
	prs := syntheticConflictBenchmarkPRs("owner/repo", 6000)
	b.Logf("synthetic corpus prs=%d", len(prs))

	for _, minShared := range []int{2, 3} {
		conflicts := buildConflictsWithMinSignalFiles("owner/repo", prs, nil, minShared)
		b.Run(fmt.Sprintf("min_shared_%d", minShared), func(b *testing.B) {
			b.Logf("min_shared=%d conflicts=%d file_entries=%d", minShared, len(conflicts), totalConflictFileEntries(conflicts))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got := buildConflictsWithMinSignalFiles("owner/repo", prs, nil, minShared)
				if len(got) != len(conflicts) {
					b.Fatalf("conflict count=%d, want %d", len(got), len(conflicts))
				}
			}
		})
	}
}

func syntheticConflictBenchmarkPRs(repo string, count int) []types.PR {
	const cohortSize = 50

	prs := make([]types.PR, 0, count)
	now := fixedNow()
	for i := 1; i <= count; i++ {
		cohort := (i - 1) / cohortSize
		sharedFileCount := 2
		if cohort%2 == 1 {
			sharedFileCount = 3
		}
		files := make([]string, 0, sharedFileCount+2)
		for j := 0; j < sharedFileCount; j++ {
			files = append(files, fmt.Sprintf("services/service-%03d/shared-%d.go", cohort, j))
		}
		files = append(files,
			fmt.Sprintf("services/service-%03d/pr-%04d.go", cohort, i),
			fmt.Sprintf("docs/.generated/service-%03d-%04d.json", cohort, i),
		)
		prs = append(prs, types.PR{
			Repo:              repo,
			Number:            i,
			Title:             fmt.Sprintf("synthetic conflict %04d", i),
			Body:              fmt.Sprintf("cohort %03d", cohort),
			URL:               fmt.Sprintf("https://example.invalid/%s/pull/%d", repo, i),
			Author:            "benchmark-bot",
			FilesChanged:      files,
			CIStatus:          "success",
			Mergeable:         "mergeable",
			BaseBranch:        fmt.Sprintf("release/%03d", cohort),
			HeadBranch:        fmt.Sprintf("synthetic-%04d", i),
			CreatedAt:         now.Add(-time.Hour).Format(time.RFC3339),
			UpdatedAt:         now.Add(-time.Minute).Format(time.RFC3339),
			Additions:         10,
			Deletions:         5,
			ChangedFilesCount: len(files),
		})
	}
	return prs
}
