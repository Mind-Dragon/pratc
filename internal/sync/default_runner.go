package sync

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	gh "github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/repo"
)

type JobRecorder interface {
	MarkSyncJobComplete(jobID string, syncedAt time.Time) error
	MarkSyncJobFailed(jobID string, message string) error
}

type DefaultRunner struct {
	worker      Worker
	jobRecorder JobRecorder
	jobID       string
}

type dbJobRecorder struct {
	dbPath string
}

func NewDefaultRunner(jobRecorder JobRecorder, jobID string, cacheStore *cache.Store) *DefaultRunner {
	return NewRunner(defaultWorker(cacheStore), jobRecorder, jobID)
}

func NewDBJobRecorder(dbPath string) JobRecorder {
	return dbJobRecorder{dbPath: strings.TrimSpace(dbPath)}
}

func NewRunner(worker Worker, jobRecorder JobRecorder, jobID string) *DefaultRunner {
	return &DefaultRunner{
		worker:      worker,
		jobRecorder: jobRecorder,
		jobID:       strings.TrimSpace(jobID),
	}
}

func (r *DefaultRunner) Run(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
	cacheStore := r.worker.CacheStore
	metadataProcessed := 0
	metadataTotal := 0
	currentCursor := ""
	result, err := r.worker.SyncJob(ctx, repo, func(stage string, done, total int) {
		if stage == "metadata" {
			metadataProcessed = done
			if total > 0 {
				metadataTotal = total
			}
		} else if stage == "mirror_fetch" && metadataTotal <= 0 && metadataProcessed > 0 {
			metadataTotal = metadataProcessed
		}

		if emit != nil {
			emit("progress", map[string]any{
				"stage": stage,
				"done":  done,
				"total": total,
				"repo":  repo,
			})
		}

		if cacheStore == nil || r.jobID == "" {
			return
		}

		progress := cache.SyncProgress{ProcessedPRs: done, TotalPRs: total}
		progress.LastBudgetCheck = time.Now().UTC()
		switch stage {
		case "metadata":
			progress.TotalPRs = 0
		case "mirror_fetch":
			if metadataTotal > 0 {
				progress.TotalPRs = metadataTotal
			}
		case "complete":
			if metadataTotal > 0 {
				progress.TotalPRs = metadataTotal
			}
		}
		_ = cacheStore.UpdateSyncJobProgress(r.jobID, progress)
	}, func(cursor string, processed int) {
		currentCursor = cursor
		if cacheStore != nil && r.jobID != "" {
			_ = cacheStore.SaveCursor(r.jobID, cursor, processed, metadataTotal)
		}
	})
	if err != nil {
		if r.jobRecorder != nil && r.jobID != "" {
			_ = r.jobRecorder.MarkSyncJobFailed(r.jobID, err.Error())
		}
		return err
	}

	if cacheStore != nil && r.jobID != "" {
		finalTotal := metadataTotal
		if finalTotal <= 0 {
			finalTotal = metadataProcessed
		}
		if finalTotal <= 0 && result != nil {
			finalTotal = result.SyncedPRs
		}
		if finalTotal > 0 {
			_ = cacheStore.UpdateSyncJobProgress(r.jobID, cache.SyncProgress{
				ProcessedPRs: finalTotal,
				TotalPRs:     finalTotal,
			})
		}
	}

	if r.jobRecorder != nil && r.jobID != "" {
		if markErr := r.jobRecorder.MarkSyncJobComplete(r.jobID, time.Now().UTC()); markErr != nil {
			return fmt.Errorf("mark sync job complete: %w", markErr)
		}
	}

	_ = result
	_ = currentCursor
	return nil
}

func defaultWorker(cacheStore *cache.Store) Worker {
	return Worker{
		MirrorFactory: func(ctx context.Context, repoID string) (Mirror, error) {
			baseDir, err := repo.DefaultBaseDir()
			if err != nil {
				return nil, fmt.Errorf("default base dir: %w", err)
			}
			remoteURL := fmt.Sprintf("https://github.com/%s.git", repoID)
			return repo.OpenOrCreate(ctx, baseDir, repoID, remoteURL)
		},
		Metadata:   githubMetadataSource{client: gh.NewClient(gh.Config{Token: os.Getenv("GITHUB_TOKEN"), ReserveRequests: 200}), cacheStore: cacheStore},
		CacheStore: cacheStore,
		Now:        func() time.Time { return time.Now().UTC() },
	}
}

func (r dbJobRecorder) MarkSyncJobComplete(jobID string, syncedAt time.Time) error {
	if r.dbPath == "" {
		return nil
	}
	store, err := cache.Open(r.dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.MarkSyncJobComplete(jobID, syncedAt)
}

func (r dbJobRecorder) MarkSyncJobFailed(jobID string, message string) error {
	if r.dbPath == "" {
		return nil
	}
	store, err := cache.Open(r.dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.MarkSyncJobFailed(jobID, message)
}

type githubMetadataSource struct {
	client     *gh.Client
	cacheStore *cache.Store
}

func (g githubMetadataSource) SyncRepo(ctx context.Context, repoID string, progress func(done, total int), onCursor func(cursor string, processed int)) (MetadataSnapshot, error) {
	if g.client == nil {
		return MetadataSnapshot{}, fmt.Errorf("github client is required")
	}

	opts := gh.PullRequestListOptions{
		PerPage:  100,
		Progress: progress,
		OnCursor: onCursor,
	}

	if g.cacheStore != nil {
		if lastSync, err := g.cacheStore.LastSync(repoID); err == nil && !lastSync.IsZero() {
			opts.UpdatedSince = lastSync
		}
	}

	prs, err := g.client.FetchPullRequests(ctx, repoID, opts)
	if err != nil {
		return MetadataSnapshot{}, fmt.Errorf("fetch pull requests: %w", err)
	}

	if g.cacheStore != nil {
		for _, pr := range prs {
			if saveErr := g.cacheStore.UpsertPR(pr); saveErr != nil {
				return MetadataSnapshot{}, fmt.Errorf("save pull request %d: %w", pr.Number, saveErr)
			}
		}
	}

	openPRs := make([]int, 0, len(prs))
	for _, pr := range prs {
		openPRs = append(openPRs, pr.Number)
	}

	return MetadataSnapshot{
		OpenPRs:       openPRs,
		ClosedPRs:     nil,
		RemotePRHeads: map[int]string{},
		SyncedPRs:     len(prs),
	}, nil
}
