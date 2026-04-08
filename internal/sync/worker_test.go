package sync

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMirror struct {
	fetched   []int
	pruned    []int
	drift     map[int]string
	fetchErr  error
	pruneErr  error
	driftErr  error
	progressN int
}

func (f *fakeMirror) FetchAll(_ context.Context, openPRs []int, progress func(done, total int)) error {
	f.fetched = append([]int(nil), openPRs...)
	if progress != nil {
		progress(1, 2)
		progress(2, 2)
		f.progressN += 2
	}
	return f.fetchErr
}

func (f *fakeMirror) FetchAllWithSkipped(_ context.Context, openPRs []int, progress func(done, total int)) ([]int, error) {
	f.fetched = append([]int(nil), openPRs...)
	if progress != nil {
		progress(1, 2)
		progress(2, 2)
		f.progressN += 2
	}
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return []int{}, nil
}

func (f *fakeMirror) PruneClosedPRs(_ context.Context, closedPRs []int) error {
	f.pruned = append([]int(nil), closedPRs...)
	return f.pruneErr
}

func (f *fakeMirror) Drift(_ context.Context, _ map[int]string) (map[int]string, error) {
	return f.drift, f.driftErr
}

type fakeMetadata struct {
	snapshot MetadataSnapshot
	err      error
}

func (f fakeMetadata) SyncRepo(_ context.Context, _ string, progress func(done, total int), onCursor func(cursor string, processed int)) (MetadataSnapshot, error) {
	if progress != nil {
		progress(1, 3)
		progress(3, 3)
	}
	return f.snapshot, f.err
}

func TestSyncJobSuccess(t *testing.T) {
	t.Parallel()
	mirror := &fakeMirror{drift: map[int]string{99: "missing"}}
	worker := Worker{
		MirrorFactory: func(context.Context, string) (Mirror, error) { return mirror, nil },
		Metadata: fakeMetadata{snapshot: MetadataSnapshot{
			OpenPRs:       []int{1, 2},
			ClosedPRs:     []int{3},
			RemotePRHeads: map[int]string{1: "abc"},
			SyncedPRs:     2,
		}},
		Now: func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}

	progressStages := []string{}
	result, err := worker.SyncJob(context.Background(), "octo/repo", func(stage string, done, total int) {
		progressStages = append(progressStages, stage)
	}, nil)
	if err != nil {
		t.Fatalf("sync job failed: %v", err)
	}
	if result.SyncedPRs != 2 {
		t.Fatalf("expected SyncedPRs=2, got %d", result.SyncedPRs)
	}
	if len(mirror.fetched) != 2 || mirror.fetched[0] != 1 || mirror.fetched[1] != 2 {
		t.Fatalf("unexpected fetched PR list: %#v", mirror.fetched)
	}
	if len(mirror.pruned) != 1 || mirror.pruned[0] != 3 {
		t.Fatalf("unexpected pruned PR list: %#v", mirror.pruned)
	}
	if len(progressStages) == 0 {
		t.Fatalf("expected progress callback stages")
	}
}

func TestSyncJobPropagatesMetadataError(t *testing.T) {
	t.Parallel()
	worker := Worker{
		MirrorFactory: func(context.Context, string) (Mirror, error) { return &fakeMirror{}, nil },
		Metadata:      fakeMetadata{err: errors.New("boom")},
	}
	_, err := worker.SyncJob(context.Background(), "octo/repo", nil, nil)
	if err == nil {
		t.Fatalf("expected metadata error")
	}
}

func TestWatchRejectsInvalidInterval(t *testing.T) {
	t.Parallel()
	worker := Worker{}
	err := worker.Watch(context.Background(), "octo/repo", 0, nil, nil)
	if err == nil {
		t.Fatalf("expected interval validation error")
	}
}
