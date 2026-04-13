package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type Mirror interface {
	FetchAll(ctx context.Context, openPRs []int, progress func(done, total int)) error
	FetchAllWithSkipped(ctx context.Context, openPRs []int, progress func(done, total int)) ([]int, error)
	PruneClosedPRs(ctx context.Context, closedPRs []int) error
	Drift(ctx context.Context, remoteByPR map[int]string) (map[int]string, error)
}

type MetadataSource interface {
	SyncRepo(ctx context.Context, repo string, progress func(done, total int), onCursor func(cursor string, processed int)) (MetadataSnapshot, error)
}

type BootstrapSource interface {
	Bootstrap(ctx context.Context, repo string) ([]types.PR, error)
}

type MirrorFactory func(ctx context.Context, repo string) (Mirror, error)

type MetadataSnapshot struct {
	OpenPRs       []int
	ClosedPRs     []int
	RemotePRHeads map[int]string
	SyncedPRs     int
}

type SyncResult struct {
	Repo          string            `json:"repo"`
	SyncedPRs     int               `json:"synced_prs"`
	SkippedPRs    []int             `json:"skipped_prs"`
	DriftDetected map[int]string    `json:"drift_detected"`
	GeneratedAt   string            `json:"generated_at"`
	Progress      map[string][2]int `json:"progress"`
}

type Worker struct {
	MirrorFactory MirrorFactory
	Metadata      MetadataSource
	Bootstrap     BootstrapSource
	CacheStore    *cache.Store
	Budget        *ratelimit.BudgetManager
	Now           func() time.Time
}

func (w Worker) SyncJob(ctx context.Context, repo string, progress func(stage string, done, total int), onCursor func(cursor string, processed int)) (*SyncResult, error) {
	if w.MirrorFactory == nil {
		return nil, fmt.Errorf("mirror factory is required")
	}
	if w.Metadata == nil {
		return nil, fmt.Errorf("metadata source is required")
	}
	now := w.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	if w.Bootstrap != nil && w.CacheStore != nil {
		progress, ok, err := w.CacheStore.GetSyncProgress(repo)
		if err != nil {
			return nil, fmt.Errorf("load sync progress: %w", err)
		}
		if !ok || (progress.Cursor == "" && progress.ProcessedPRs == 0) {
			bootstrapPRS, err := w.Bootstrap.Bootstrap(ctx, repo)
			if err != nil {
				return nil, fmt.Errorf("bootstrap sync data: %w", err)
			}
			for _, pr := range bootstrapPRS {
				if err := w.CacheStore.UpsertPR(pr); err != nil {
					return nil, fmt.Errorf("persist bootstrap pr %d: %w", pr.Number, err)
				}
			}
		}
	}

	snapshot, err := w.Metadata.SyncRepo(ctx, repo, func(done, total int) {
		if progress != nil {
			progress("metadata", done, total)
		}
	}, onCursor)
	if err != nil {
		return nil, fmt.Errorf("sync metadata: %w", err)
	}

	mirror, err := w.MirrorFactory(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("open mirror: %w", err)
	}

	skipped, err := mirror.FetchAllWithSkipped(ctx, snapshot.OpenPRs, func(done, total int) {
		if progress != nil {
			progress("mirror_fetch", done, total)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("fetch mirror refs: %w", err)
	}

	if err := mirror.PruneClosedPRs(ctx, snapshot.ClosedPRs); err != nil {
		return nil, fmt.Errorf("prune closed PR refs: %w", err)
	}

	drift, err := mirror.Drift(ctx, snapshot.RemotePRHeads)
	if err != nil {
		return nil, fmt.Errorf("compute drift: %w", err)
	}

	result := &SyncResult{
		Repo:          repo,
		SyncedPRs:     snapshot.SyncedPRs,
		SkippedPRs:    append([]int{}, skipped...),
		DriftDetected: drift,
		GeneratedAt:   now().Format(time.RFC3339),
		Progress:      map[string][2]int{},
	}
	if progress != nil {
		progress("complete", 1, 1)
	}
	return result, nil
}

func (w Worker) ResumeSyncJob(ctx context.Context, repo string, progress func(stage string, done, total int), onCursor func(cursor string, processed int)) (*SyncResult, error) {
	if w.CacheStore == nil {
		return nil, fmt.Errorf("cache store is required")
	}

	if _, err := cache.ResumeSyncJob(w.CacheStore, repo); err != nil {
		return nil, fmt.Errorf("resume sync job: %w", err)
	}

	return w.SyncJob(ctx, repo, progress, onCursor)
}

func (w Worker) Watch(ctx context.Context, repo string, interval time.Duration, progress func(stage string, done, total int), onCursor func(cursor string, processed int)) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if _, err := w.SyncJob(ctx, repo, progress, onCursor); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
