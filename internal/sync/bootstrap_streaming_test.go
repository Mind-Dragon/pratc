package sync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type testStreamingBootstrapSource struct {
	prs      []types.PR
	streamed bool
}

func (s *testStreamingBootstrapSource) Bootstrap(ctx context.Context, repo string) ([]types.PR, error) {
	return nil, fmt.Errorf("Bootstrap should not be used when StreamBootstrap is available")
}

func (s *testStreamingBootstrapSource) StreamBootstrap(ctx context.Context, repo string, emit func(types.PR) error) error {
	s.streamed = true
	for _, pr := range s.prs {
		if err := emit(pr); err != nil {
			return err
		}
	}
	return nil
}

type noopMirror struct{}

func (noopMirror) FetchAllWithSkipped(ctx context.Context, openPRs []int, progress func(done, total int)) ([]int, error) {
	return nil, nil
}

func (noopMirror) FetchAll(ctx context.Context, openPRs []int, progress func(done, total int)) error {
	return nil
}

func (noopMirror) PruneClosedPRs(ctx context.Context, closedPRs []int) error {
	return nil
}

func (noopMirror) Drift(ctx context.Context, remoteByPR map[int]string) (map[int]string, error) {
	return map[int]string{}, nil
}

type staticMetadataSource struct {
	snapshot MetadataSnapshot
}

func (s staticMetadataSource) SyncRepo(ctx context.Context, repo string, progress func(done, total int), onCursor func(cursor string, processed int)) (MetadataSnapshot, error) {
	return s.snapshot, nil
}

func TestWorkerSyncJobUsesStreamingBootstrap(t *testing.T) {
	t.Parallel()

	store, err := cache.Open("file::bootstrap-streaming?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	bootstrap := &testStreamingBootstrapSource{prs: make([]types.PR, 0, 200)}
	for i := 1; i <= 200; i++ {
		bootstrap.prs = append(bootstrap.prs, types.PR{
			Repo:         "owner/repo",
			Number:       i,
			Title:        fmt.Sprintf("PR %d", i),
			URL:          types.GitHubURLPrefix + "owner/repo/pull/" + fmt.Sprint(i),
			Author:       "octocat",
			BaseBranch:   "main",
			HeadBranch:   "feature",
			CIStatus:     "success",
			ReviewStatus: "approved",
			Mergeable:    "mergeable",
			CreatedAt:    time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		})
	}

	worker := Worker{
		Bootstrap: bootstrap,
		Metadata: staticMetadataSource{snapshot: MetadataSnapshot{
			SyncedPRs:     200,
			OpenPRs:       nil,
			ClosedPRs:     nil,
			RemotePRHeads: map[int]string{},
		}},
		MirrorFactory: func(ctx context.Context, repo string) (Mirror, error) {
			return noopMirror{}, nil
		},
		CacheStore: store,
		Now:        func() time.Time { return time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC) },
	}

	result, err := worker.SyncJob(context.Background(), "owner/repo", nil, nil)
	if err != nil {
		t.Fatalf("sync job: %v", err)
	}
	if !bootstrap.streamed {
		t.Fatal("expected streaming bootstrap path to be used")
	}
	if result.SyncedPRs != 200 {
		t.Fatalf("synced_prs = %d, want 200", result.SyncedPRs)
	}
	prs, err := store.ListPRs(cache.PRFilter{Repo: "owner/repo"})
	if err != nil {
		t.Fatalf("list prs: %v", err)
	}
	if len(prs) != 200 {
		t.Fatalf("expected 200 bootstrap PRs in cache, got %d", len(prs))
	}
}
