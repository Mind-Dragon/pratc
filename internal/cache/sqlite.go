package cache

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	_ "modernc.org/sqlite"
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	store := &Store{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
	}

	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) JournalMode() (string, error) {
	var mode string
	if err := s.db.QueryRow(`PRAGMA journal_mode;`).Scan(&mode); err != nil {
		return "", fmt.Errorf("query journal mode: %w", err)
	}
	return mode, nil
}

func (s *Store) UpsertPR(pr types.PR) error {
	labelsJSON, err := json.Marshal(pr.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}
	filesJSON, err := json.Marshal(pr.FilesChanged)
	if err != nil {
		return fmt.Errorf("marshal files changed: %w", err)
	}
	provenanceJSON, err := json.Marshal(pr.Provenance)
	if err != nil {
		return fmt.Errorf("marshal provenance: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO pull_requests (
			id, repo, number, title, body, url, author, labels_json, files_changed_json,
			review_status, ci_status, mergeable, base_branch, head_branch, cluster_id,
			created_at, updated_at, is_draft, is_bot, additions, deletions, changed_files_count, provenance_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo, number) DO UPDATE SET
			id = excluded.id,
			title = excluded.title,
			body = excluded.body,
			url = excluded.url,
			author = excluded.author,
			labels_json = excluded.labels_json,
			files_changed_json = excluded.files_changed_json,
			review_status = excluded.review_status,
			ci_status = excluded.ci_status,
			mergeable = excluded.mergeable,
			base_branch = excluded.base_branch,
			head_branch = excluded.head_branch,
			cluster_id = excluded.cluster_id,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			is_draft = excluded.is_draft,
			is_bot = excluded.is_bot,
			additions = excluded.additions,
			deletions = excluded.deletions,
			changed_files_count = excluded.changed_files_count,
			provenance_json = excluded.provenance_json
	`,
		pr.ID, pr.Repo, pr.Number, pr.Title, pr.Body, pr.URL, pr.Author, string(labelsJSON), string(filesJSON),
		pr.ReviewStatus, pr.CIStatus, pr.Mergeable, pr.BaseBranch, pr.HeadBranch, pr.ClusterID,
		pr.CreatedAt, pr.UpdatedAt, pr.IsDraft, pr.IsBot, pr.Additions, pr.Deletions, pr.ChangedFilesCount, string(provenanceJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert pull request %d: %w", pr.Number, err)
	}

	return nil
}

const defaultPRPageSize = 1000

func (s *Store) ListPRs(filter PRFilter) ([]types.PR, error) {
	var all []types.PR
	cursor := ""

	for {
		page, err := s.ListPRsPage(filter, cursor, defaultPRPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page.PRs...)
		if !page.HasMore {
			return all, nil
		}
		cursor = page.NextCursor
	}
}

func (s *Store) ListPRsPage(filter PRFilter, cursor string, limit int) (PRPage, error) {
	if limit <= 0 {
		return PRPage{}, fmt.Errorf("list pull requests page limit must be > 0")
	}

	query := `
		SELECT
			id, repo, number, title, body, url, author, labels_json, files_changed_json,
			review_status, ci_status, mergeable, base_branch, head_branch, cluster_id,
			created_at, updated_at, is_draft, is_bot, additions, deletions, changed_files_count, provenance_json
		FROM pull_requests
		WHERE repo = ?
	`
	args := []any{filter.Repo}

	if filter.BaseBranch != "" {
		query += ` AND base_branch = ?`
		args = append(args, filter.BaseBranch)
	}
	if filter.CIStatus != "" {
		query += ` AND ci_status = ?`
		args = append(args, filter.CIStatus)
	}
	if !filter.UpdatedSince.IsZero() {
		query += ` AND updated_at >= ?`
		args = append(args, filter.UpdatedSince.UTC().Format(time.RFC3339))
	}
	if cursor != "" {
		cursorNumber, err := strconv.Atoi(cursor)
		if err != nil {
			return PRPage{}, fmt.Errorf("parse cursor %q: %w", cursor, err)
		}
		query += ` AND number > ?`
		args = append(args, cursorNumber)
	}

	query += ` ORDER BY number ASC LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return PRPage{}, fmt.Errorf("query pull requests page: %w", err)
	}
	defer rows.Close()

	prs := make([]types.PR, 0, limit+1)
	for rows.Next() {
		var pr types.PR
		var labelsJSON string
		var filesJSON string
		var provenanceJSON string

		if err := rows.Scan(
			&pr.ID, &pr.Repo, &pr.Number, &pr.Title, &pr.Body, &pr.URL, &pr.Author, &labelsJSON, &filesJSON,
			&pr.ReviewStatus, &pr.CIStatus, &pr.Mergeable, &pr.BaseBranch, &pr.HeadBranch, &pr.ClusterID,
			&pr.CreatedAt, &pr.UpdatedAt, &pr.IsDraft, &pr.IsBot, &pr.Additions, &pr.Deletions, &pr.ChangedFilesCount, &provenanceJSON,
		); err != nil {
			return PRPage{}, fmt.Errorf("scan pull request: %w", err)
		}

		if err := json.Unmarshal([]byte(labelsJSON), &pr.Labels); err != nil {
			return PRPage{}, fmt.Errorf("unmarshal labels: %w", err)
		}
		if err := json.Unmarshal([]byte(filesJSON), &pr.FilesChanged); err != nil {
			return PRPage{}, fmt.Errorf("unmarshal files changed: %w", err)
		}
		if provenanceJSON != "" {
			if err := json.Unmarshal([]byte(provenanceJSON), &pr.Provenance); err != nil {
				return PRPage{}, fmt.Errorf("unmarshal provenance: %w", err)
			}
		}

		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		return PRPage{}, fmt.Errorf("iterate pull requests: %w", err)
	}

	page := PRPage{PRs: prs}
	if len(prs) > limit {
		page.HasMore = true
		page.PRs = prs[:limit]
		page.NextCursor = strconv.Itoa(page.PRs[len(page.PRs)-1].Number)
	}
	return page, nil
}

func (s *Store) ListPRsIter(filter PRFilter, fn func(types.PR) error) error {
	cursor := ""
	for {
		page, err := s.ListPRsPage(filter, cursor, defaultPRPageSize)
		if err != nil {
			return err
		}
		for _, pr := range page.PRs {
			if err := fn(pr); err != nil {
				return err
			}
		}
		if !page.HasMore {
			return nil
		}
		cursor = page.NextCursor
	}
}

func (s *Store) SetLastSync(repo string, syncedAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO sync_progress (repo, last_sync_at, cursor, processed_prs, total_prs, updated_at)
		VALUES (?, ?, '', 0, 0, ?)
		ON CONFLICT(repo) DO UPDATE SET
			last_sync_at = excluded.last_sync_at,
			updated_at = excluded.updated_at
	`, repo, syncedAt.UTC().Format(time.RFC3339), s.now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("set last sync: %w", err)
	}
	return nil
}

func (s *Store) LastSync(repo string) (time.Time, error) {
	var raw string
	err := s.db.QueryRow(`SELECT COALESCE(last_sync_at, '') FROM sync_progress WHERE repo = ?`, repo).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || raw == "" {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get last sync: %w", err)
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse last sync %q: %w", raw, err)
	}
	return parsed, nil
}

func (s *Store) ListAllRepos() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT repo FROM pull_requests ORDER BY repo`)
	if err != nil {
		return nil, fmt.Errorf("query repos: %w", err)
	}
	defer rows.Close()

	var repos []string
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repos: %w", err)
	}
	return repos, nil
}

func (s *Store) UpsertMergedPR(pr MergedPR) error {
	filesJSON, err := json.Marshal(pr.FilesTouched)
	if err != nil {
		return fmt.Errorf("marshal merged pr files: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO merged_pr_index (repo, number, merged_at, files_touched_json)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(repo, number) DO UPDATE SET
			merged_at = excluded.merged_at,
			files_touched_json = excluded.files_touched_json
	`, pr.Repo, pr.Number, pr.MergedAt.UTC().Format(time.RFC3339), string(filesJSON))
	if err != nil {
		return fmt.Errorf("upsert merged pr: %w", err)
	}
	return nil
}

func (s *Store) ListMergedPRs(repo string) ([]MergedPR, error) {
	rows, err := s.db.Query(`
		SELECT repo, number, merged_at, files_touched_json
		FROM merged_pr_index
		WHERE repo = ?
		ORDER BY merged_at DESC, number DESC
	`, repo)
	if err != nil {
		return nil, fmt.Errorf("query merged prs: %w", err)
	}
	defer rows.Close()

	var prs []MergedPR
	for rows.Next() {
		var pr MergedPR
		var mergedAt string
		var filesJSON string

		if err := rows.Scan(&pr.Repo, &pr.Number, &mergedAt, &filesJSON); err != nil {
			return nil, fmt.Errorf("scan merged pr: %w", err)
		}
		pr.MergedAt, err = time.Parse(time.RFC3339, mergedAt)
		if err != nil {
			return nil, fmt.Errorf("parse merged_at: %w", err)
		}
		if err := json.Unmarshal([]byte(filesJSON), &pr.FilesTouched); err != nil {
			return nil, fmt.Errorf("unmarshal files touched: %w", err)
		}

		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate merged prs: %w", err)
	}
	return prs, nil
}

func (s *Store) UpsertPRFiles(repo string, prNumber int, files []string) (err error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin pr files transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM pr_files WHERE repo = ? AND pr_number = ?`, repo, prNumber); err != nil {
		return fmt.Errorf("replace pr files: %w", err)
	}

	seen := make(map[string]struct{}, len(files))
	for _, path := range files {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		if _, err = tx.Exec(`INSERT INTO pr_files (repo, pr_number, path) VALUES (?, ?, ?)`, repo, prNumber, path); err != nil {
			return fmt.Errorf("insert pr file %q: %w", path, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit pr files transaction: %w", err)
	}

	return nil
}

func (s *Store) GetPRFiles(repo string, prNumber int) ([]string, bool, error) {
	rows, err := s.db.Query(`
		SELECT path
		FROM pr_files
		WHERE repo = ? AND pr_number = ?
		ORDER BY path ASC
	`, repo, prNumber)
	if err != nil {
		return nil, false, fmt.Errorf("query pr files: %w", err)
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, false, fmt.Errorf("scan pr file: %w", err)
		}
		files = append(files, path)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate pr files: %w", err)
	}
	if len(files) == 0 {
		return nil, false, nil
	}

	return files, true, nil
}

func (s *Store) ClearPRFiles(repo string, prNumber int) error {
	if _, err := s.db.Exec(`DELETE FROM pr_files WHERE repo = ? AND pr_number = ?`, repo, prNumber); err != nil {
		return fmt.Errorf("clear pr files: %w", err)
	}
	return nil
}

func (s *Store) CreateSyncJob(repo string) (SyncJob, error) {
	now := s.now().UTC()

	// Generate random bytes for unpredictable ID
	var randBytes [8]byte
	if _, err := rand.Read(randBytes[:]); err != nil {
		return SyncJob{}, fmt.Errorf("generate random ID: %w", err)
	}

	// Combine repo + timestamp + random bytes, hash for final ID
	hasher := fmt.Sprintf("%s-%d", repo, now.UnixNano())
	hashed := append([]byte(hasher), randBytes[:]...)
	sum := make([]byte, hex.EncodedLen(len(hashed)))
	hex.Encode(sum, hashed)

	job := SyncJob{
		ID:        fmt.Sprintf("%s-%s", repo, hex.EncodeToString(randBytes[:])),
		Repo:      repo,
		Status:    SyncJobStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.Exec(`
		INSERT INTO sync_jobs (id, repo, status, error_message, last_sync_at, created_at, updated_at)
		VALUES (?, ?, ?, '', '', ?, ?)
	`, job.ID, job.Repo, job.Status, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return SyncJob{}, fmt.Errorf("create sync job: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO sync_progress (repo, job_id, cursor, processed_prs, total_prs, snapshot_ceiling, next_scheduled_at, estimated_requests, last_sync_at, updated_at)
		VALUES (?, ?, '', 0, 0, 0, '', 0, '', ?)
		ON CONFLICT(repo) DO UPDATE SET
			job_id = excluded.job_id,
			cursor = excluded.cursor,
			processed_prs = excluded.processed_prs,
			total_prs = excluded.total_prs,
			snapshot_ceiling = excluded.snapshot_ceiling,
			next_scheduled_at = excluded.next_scheduled_at,
			estimated_requests = excluded.estimated_requests,
			updated_at = excluded.updated_at
	`, repo, job.ID, now.Format(time.RFC3339))
	if err != nil {
		return SyncJob{}, fmt.Errorf("initialize sync progress: %w", err)
	}

	return job, nil
}

func (s *Store) UpdateSyncJobProgress(jobID string, progress SyncProgress) (err error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin update sync job progress transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var repo string
	err = tx.QueryRow(`SELECT repo FROM sync_jobs WHERE id = ?`, jobID).Scan(&repo)
	if err != nil {
		return fmt.Errorf("lookup sync job repo: %w", err)
	}

	now := s.now().UTC().Format(time.RFC3339)
	lastBudgetCheck := ""
	if !progress.LastBudgetCheck.IsZero() {
		lastBudgetCheck = progress.LastBudgetCheck.UTC().Format(time.RFC3339)
	}

	// Transition job from queued to running when first progress is reported
	_, err = tx.Exec(`
		UPDATE sync_jobs
		SET status = CASE WHEN status = ? THEN ? ELSE status END,
		    updated_at = ?
		WHERE id = ?
	`, SyncJobStatusQueued, SyncJobStatusRunning, now, jobID)
	if err != nil {
		return fmt.Errorf("transition job to running: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO sync_progress (repo, job_id, cursor, processed_prs, total_prs, snapshot_ceiling, last_sync_at, last_budget_check, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)
		ON CONFLICT(repo) DO UPDATE SET
			job_id = excluded.job_id,
			cursor = excluded.cursor,
			processed_prs = excluded.processed_prs,
			total_prs = excluded.total_prs,
			snapshot_ceiling = CASE
				WHEN excluded.snapshot_ceiling > 0 THEN excluded.snapshot_ceiling
				ELSE snapshot_ceiling
			END,
			last_budget_check = excluded.last_budget_check,
			updated_at = excluded.updated_at
	`, repo, jobID, progress.Cursor, progress.ProcessedPRs, progress.TotalPRs, progress.SnapshotCeiling, lastBudgetCheck, now)
	if err != nil {
		return fmt.Errorf("update sync progress: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit update sync job progress transaction: %w", err)
	}

	return nil
}

func (s *Store) SaveCursor(repo string, cursor string, processedPRs int, totalPRs int) error {
	now := s.now().UTC().Format(time.RFC3339)
	estimatedRequests := 0
	if totalPRs > processedPRs {
		estimatedRequests = (totalPRs - processedPRs) * 3
	}
	_, err := s.db.Exec(`
		INSERT INTO sync_progress (repo, job_id, cursor, processed_prs, total_prs, snapshot_ceiling, estimated_requests, last_sync_at, updated_at)
		VALUES (?, '', ?, ?, ?, ?, ?, '', ?)
		ON CONFLICT(repo) DO UPDATE SET
			cursor = excluded.cursor,
			processed_prs = excluded.processed_prs,
			total_prs = excluded.total_prs,
			snapshot_ceiling = CASE
				WHEN excluded.snapshot_ceiling > 0 THEN excluded.snapshot_ceiling
				ELSE snapshot_ceiling
			END,
			estimated_requests = excluded.estimated_requests,
			updated_at = excluded.updated_at
	`, repo, cursor, processedPRs, totalPRs, totalPRs, estimatedRequests, now)
	if err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}
	return nil
}

func (s *Store) GetSyncProgress(repo string) (SyncProgress, bool, error) {
	row := s.db.QueryRow(`
		SELECT cursor, processed_prs, total_prs, COALESCE(snapshot_ceiling, 0), COALESCE(next_scheduled_at, ''), COALESCE(estimated_requests, 0),
		       COALESCE(scheduled_resume_at, ''), COALESCE(pause_reason, ''), COALESCE(last_budget_check, '')
		FROM sync_progress
		WHERE repo = ?
	`, repo)

	var progress SyncProgress
	var nextScheduledAt string
	var estimatedRequests int
	var scheduledResumeAt string
	var pauseReason string
	var lastBudgetCheck string
	if err := row.Scan(&progress.Cursor, &progress.ProcessedPRs, &progress.TotalPRs, &progress.SnapshotCeiling, &nextScheduledAt, &estimatedRequests, &scheduledResumeAt, &pauseReason, &lastBudgetCheck); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SyncProgress{}, false, nil
		}
		return SyncProgress{}, false, fmt.Errorf("get sync progress: %w", err)
	}
	progress.NextScheduledAt = parseOptionalTime(nextScheduledAt)
	progress.EstimatedRequests = estimatedRequests
	progress.ScheduledResumeAt = parseOptionalTime(scheduledResumeAt)
	progress.PauseReason = pauseReason
	progress.LastBudgetCheck = parseOptionalTime(lastBudgetCheck)
	return progress, true, nil
}

func (s *Store) GetSyncJob(jobID string) (SyncJob, error) {
	row := s.db.QueryRow(`
		SELECT
			j.id, j.repo, j.status, j.error_message, COALESCE(j.last_sync_at, ''), j.created_at, j.updated_at,
			COALESCE(p.cursor, ''), COALESCE(p.processed_prs, 0), COALESCE(p.total_prs, 0), COALESCE(p.snapshot_ceiling, 0),
			COALESCE(p.next_scheduled_at, ''), COALESCE(p.estimated_requests, 0),
			COALESCE(p.scheduled_resume_at, ''), COALESCE(p.pause_reason, ''), COALESCE(p.last_budget_check, '')
		FROM sync_jobs j
		LEFT JOIN sync_progress p ON p.job_id = j.id
		WHERE j.id = ?
	`, jobID)

	var job SyncJob
	var status string
	var lastSync string
	var createdAt string
	var updatedAt string
	var nextScheduledAt string
	var estimatedRequests int
	var scheduledResumeAt string
	var pauseReason string
	var lastBudgetCheck string
	if err := row.Scan(
		&job.ID, &job.Repo, &status, &job.Error, &lastSync, &createdAt, &updatedAt,
		&job.Progress.Cursor, &job.Progress.ProcessedPRs, &job.Progress.TotalPRs, &job.Progress.SnapshotCeiling,
		&nextScheduledAt, &estimatedRequests,
		&scheduledResumeAt, &pauseReason, &lastBudgetCheck,
	); err != nil {
		return SyncJob{}, fmt.Errorf("get sync job: %w", err)
	}

	job.Status = SyncJobStatus(status)
	job.CreatedAt = parseOptionalTime(createdAt)
	job.UpdatedAt = parseOptionalTime(updatedAt)
	job.LastSyncAt = parseOptionalTime(lastSync)
	job.Progress.NextScheduledAt = parseOptionalTime(nextScheduledAt)
	job.Progress.EstimatedRequests = estimatedRequests
	job.Progress.ScheduledResumeAt = parseOptionalTime(scheduledResumeAt)
	job.Progress.PauseReason = pauseReason
	job.Progress.LastBudgetCheck = parseOptionalTime(lastBudgetCheck)
	return job, nil
}

func (s *Store) ResumeSyncJob(repo string) (SyncJob, bool, error) {
	// Look for jobs in active states: queued, running, or resuming
	var jobID string
	err := s.db.QueryRow(`
		SELECT id FROM sync_jobs
		WHERE repo = ? AND status IN (?, ?, ?)
		ORDER BY updated_at DESC
		LIMIT 1
	`, repo, SyncJobStatusQueued, SyncJobStatusRunning, SyncJobStatusResuming).Scan(&jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncJob{}, false, nil
	}
	if err != nil {
		return SyncJob{}, false, fmt.Errorf("resume sync job: %w", err)
	}

	job, err := s.GetSyncJob(jobID)
	if err != nil {
		return SyncJob{}, false, err
	}
	return job, true, nil
}

// GetLatestSyncJob returns the most recent sync job for a repo, regardless of status.
// This is used by the status endpoint to report paused_rate_limit and failed states.
func (s *Store) GetLatestSyncJob(repo string) (SyncJob, bool, error) {
	var jobID string
	err := s.db.QueryRow(`
		SELECT id FROM sync_jobs
		WHERE repo = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, repo).Scan(&jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncJob{}, false, nil
	}
	if err != nil {
		return SyncJob{}, false, fmt.Errorf("get latest sync job: %w", err)
	}

	job, err := s.GetSyncJob(jobID)
	if err != nil {
		return SyncJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) MarkSyncJobComplete(jobID string, syncedAt time.Time) error {
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE sync_jobs
		SET status = ?, last_sync_at = ?, updated_at = ?
		WHERE id = ?
	`, SyncJobStatusCompleted, syncedAt.UTC().Format(time.RFC3339), now, jobID)
	if err != nil {
		return fmt.Errorf("mark sync job complete: %w", err)
	}

	var repo string
	if err := s.db.QueryRow(`SELECT repo FROM sync_jobs WHERE id = ?`, jobID).Scan(&repo); err != nil {
		return fmt.Errorf("lookup sync job repo after completion: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE sync_progress
		SET last_sync_at = ?, updated_at = ?
		WHERE repo = ?
	`, syncedAt.UTC().Format(time.RFC3339), now, repo)
	if err != nil {
		return fmt.Errorf("persist last sync after completion: %w", err)
	}

	return nil
}

func (s *Store) MarkSyncJobFailed(jobID string, message string) error {
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE sync_jobs
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, SyncJobStatusFailed, message, now, jobID)
	if err != nil {
		return fmt.Errorf("mark sync job failed: %w", err)
	}
	return nil
}

// CancelSyncJob transitions a job to the canceled terminal state.
func (s *Store) CancelSyncJob(jobID string) error {
	now := s.now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE sync_jobs
		SET status = ?, error_message = '', updated_at = ?
		WHERE id = ?
	`, SyncJobStatusCanceled, now, jobID)
	if err != nil {
		return fmt.Errorf("cancel sync job: %w", err)
	}
	return nil
}

func (s *Store) ResumeSyncJobByID(jobID string) error {
	var repo string
	if err := s.db.QueryRow(`SELECT repo FROM sync_jobs WHERE id = ?`, jobID).Scan(&repo); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no job found with ID %s", jobID)
		}
		return fmt.Errorf("lookup sync job repo for resume: %w", err)
	}

	return s.resumeSyncJob(jobID, repo)
}

func ResumeSyncJob(store *Store, repo string) (SyncJob, error) {
	if store == nil {
		return SyncJob{}, fmt.Errorf("resume sync job: store is required")
	}

	job, err := store.GetPausedSyncJobByRepo(repo)
	if err != nil {
		return SyncJob{}, fmt.Errorf("resume sync job: %w", err)
	}

	if err := store.resumeSyncJob(job.ID, repo); err != nil {
		return SyncJob{}, fmt.Errorf("resume sync job: %w", err)
	}

	resumed, err := store.GetSyncJob(job.ID)
	if err != nil {
		return SyncJob{}, fmt.Errorf("reload resumed sync job: %w", err)
	}

	return resumed, nil
}

func (s *Store) resumeSyncJob(jobID, repo string) error {
	now := s.now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin resume sync job transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var linkedJobID string
	if err := tx.QueryRow(`SELECT job_id FROM sync_progress WHERE repo = ?`, repo).Scan(&linkedJobID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("sync progress linkage missing for repo %q", repo)
		}
		return fmt.Errorf("lookup sync progress linkage for resume: %w", err)
	}
	if linkedJobID == "" {
		return fmt.Errorf("sync progress linkage missing for repo %q", repo)
	}

	result, err := tx.Exec(`
		UPDATE sync_jobs
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, SyncJobStatusResuming, "", now, jobID)
	if err != nil {
		return fmt.Errorf("resume sync job by ID: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no job found with ID %s", jobID)
	}

	if _, err = tx.Exec(`
		UPDATE sync_progress
		SET job_id = ?, next_scheduled_at = '', scheduled_resume_at = '', pause_reason = '', last_budget_check = '', updated_at = ?
		WHERE repo = ?
	`, jobID, now, repo); err != nil {
		return fmt.Errorf("clear paused sync fields: %w", err)
	}

	if rowsAffected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("check sync job rows affected: %w", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no job found with ID %s", jobID)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit resume sync job transaction: %w", err)
	}

	return nil
}

func (s *Store) PauseSyncJob(jobID string, nextScheduledAt time.Time, reason string) (err error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin pause sync job transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := s.now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(`
		UPDATE sync_jobs
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, SyncJobStatusPausedRateLimit, reason, now, jobID)
	if err != nil {
		return fmt.Errorf("pause sync job: %w", err)
	}

	var repo string
	if err := tx.QueryRow(`SELECT repo FROM sync_jobs WHERE id = ?`, jobID).Scan(&repo); err != nil {
		return fmt.Errorf("lookup sync job repo after pause: %w", err)
	}

	nextScheduled := ""
	if !nextScheduledAt.IsZero() {
		nextScheduled = nextScheduledAt.UTC().Format(time.RFC3339)
	}
	lastBudgetCheck := s.now().UTC().Format(time.RFC3339)

	_, err = tx.Exec(`
		UPDATE sync_progress
		SET next_scheduled_at = ?, scheduled_resume_at = ?, pause_reason = ?, last_budget_check = ?, updated_at = ?
		WHERE repo = ?
	`, nextScheduled, nextScheduled, reason, lastBudgetCheck, now, repo)
	if err != nil {
		return fmt.Errorf("persist next scheduled after pause: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit pause sync job transaction: %w", err)
	}

	return nil
}

func (s *Store) ListPausedSyncJobs() ([]SyncJob, error) {
	rows, err := s.db.Query(`
		SELECT
			j.id, j.repo, j.status, j.error_message, COALESCE(j.last_sync_at, ''), j.created_at, j.updated_at,
			COALESCE(p.cursor, ''), COALESCE(p.processed_prs, 0), COALESCE(p.total_prs, 0), COALESCE(p.snapshot_ceiling, 0),
			COALESCE(p.next_scheduled_at, ''), COALESCE(p.scheduled_resume_at, ''), COALESCE(p.pause_reason, ''), COALESCE(p.last_budget_check, '')
		FROM sync_jobs j
		LEFT JOIN sync_progress p ON p.repo = j.repo
	WHERE j.status IN (?, ?)
	ORDER BY j.updated_at ASC
	`, SyncJobStatusPaused, SyncJobStatusPausedRateLimit)
	if err != nil {
		return nil, fmt.Errorf("list paused sync jobs: %w", err)
	}
	defer rows.Close()

	var jobs []SyncJob
	for rows.Next() {
		var job SyncJob
		var status string
		var lastSync string
		var createdAt string
		var updatedAt string
		var nextScheduledStr string
		var scheduledResumeStr string
		var pauseReason string
		var lastBudgetCheckStr string
		if err := rows.Scan(
			&job.ID, &job.Repo, &status, &job.Error, &lastSync, &createdAt, &updatedAt,
			&job.Progress.Cursor, &job.Progress.ProcessedPRs, &job.Progress.TotalPRs, &job.Progress.SnapshotCeiling,
			&nextScheduledStr, &scheduledResumeStr, &pauseReason, &lastBudgetCheckStr,
		); err != nil {
			return nil, fmt.Errorf("scan paused sync job: %w", err)
		}

		job.Status = SyncJobStatus(status)
		job.CreatedAt = parseOptionalTime(createdAt)
		job.UpdatedAt = parseOptionalTime(updatedAt)
		job.LastSyncAt = parseOptionalTime(lastSync)
		job.Progress.NextScheduledAt = parseOptionalTime(nextScheduledStr)
		job.Progress.ScheduledResumeAt = parseOptionalTime(scheduledResumeStr)
		job.Progress.PauseReason = pauseReason
		job.Progress.LastBudgetCheck = parseOptionalTime(lastBudgetCheckStr)
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate paused sync jobs: %w", err)
	}

	return jobs, nil
}

// ListSyncJobs returns all sync jobs ordered by updated_at descending.
func (s *Store) ListSyncJobs() ([]SyncJob, error) {
	rows, err := s.db.Query(`
		SELECT
			j.id, j.repo, j.status, j.error_message, COALESCE(j.last_sync_at, ''), j.created_at, j.updated_at,
			COALESCE(p.cursor, ''), COALESCE(p.processed_prs, 0), COALESCE(p.total_prs, 0), COALESCE(p.snapshot_ceiling, 0),
			COALESCE(p.next_scheduled_at, ''), COALESCE(p.estimated_requests, 0),
			COALESCE(p.scheduled_resume_at, ''), COALESCE(p.pause_reason, ''), COALESCE(p.last_budget_check, '')
		FROM sync_jobs j
		LEFT JOIN sync_progress p ON p.job_id = j.id
		ORDER BY j.updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list sync jobs: %w", err)
	}
	defer rows.Close()

	var jobs []SyncJob
	for rows.Next() {
		var job SyncJob
		var status string
		var lastSync string
		var createdAt string
		var updatedAt string
		var nextScheduledStr string
		var estimatedRequests int
		var scheduledResumeStr string
		var pauseReason string
		var lastBudgetCheckStr string
		if err := rows.Scan(
			&job.ID, &job.Repo, &status, &job.Error, &lastSync, &createdAt, &updatedAt,
			&job.Progress.Cursor, &job.Progress.ProcessedPRs, &job.Progress.TotalPRs, &job.Progress.SnapshotCeiling,
			&nextScheduledStr, &estimatedRequests,
			&scheduledResumeStr, &pauseReason, &lastBudgetCheckStr,
		); err != nil {
			return nil, fmt.Errorf("scan sync job: %w", err)
		}

		job.Status = SyncJobStatus(status)
		job.CreatedAt = parseOptionalTime(createdAt)
		job.UpdatedAt = parseOptionalTime(updatedAt)
		job.LastSyncAt = parseOptionalTime(lastSync)
		job.Progress.NextScheduledAt = parseOptionalTime(nextScheduledStr)
		job.Progress.EstimatedRequests = estimatedRequests
		job.Progress.ScheduledResumeAt = parseOptionalTime(scheduledResumeStr)
		job.Progress.PauseReason = pauseReason
		job.Progress.LastBudgetCheck = parseOptionalTime(lastBudgetCheckStr)
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync jobs: %w", err)
	}

	return jobs, nil
}

// GetPausedSyncJobByRepo returns a paused sync job for the given repo, including scheduling information.
func (s *Store) GetPausedSyncJobByRepo(repo string) (SyncJob, error) {
	rows, err := s.db.Query(`
		SELECT j.id, j.repo, j.status, j.error_message, COALESCE(j.last_sync_at, ''), j.created_at, j.updated_at,
		       COALESCE(p.cursor, ''), COALESCE(p.processed_prs, 0), COALESCE(p.total_prs, 0), COALESCE(p.snapshot_ceiling, 0),
		       COALESCE(p.next_scheduled_at, ''), COALESCE(p.estimated_requests, 0),
		       COALESCE(p.scheduled_resume_at, ''), COALESCE(p.pause_reason, ''), COALESCE(p.last_budget_check, '')
		FROM sync_jobs j
		LEFT JOIN sync_progress p ON j.repo = p.repo
	WHERE j.repo = ? AND j.status IN (?, ?)
	ORDER BY j.updated_at DESC
	LIMIT 1
	`, repo, SyncJobStatusPaused, SyncJobStatusPausedRateLimit)
	if err != nil {
		return SyncJob{}, fmt.Errorf("get paused sync job by repo: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return SyncJob{}, fmt.Errorf("no paused sync job found for repo %q", repo)
	}

	var job SyncJob
	var status string
	var lastSync string
	var createdAt string
	var updatedAt string
	var nextScheduledAtStr string
	var estimatedRequests int
	var scheduledResumeAtStr string
	var pauseReason string
	var lastBudgetCheckStr string

	if err := rows.Scan(
		&job.ID, &job.Repo, &status, &job.Error, &lastSync, &createdAt, &updatedAt,
		&job.Progress.Cursor, &job.Progress.ProcessedPRs, &job.Progress.TotalPRs, &job.Progress.SnapshotCeiling,
		&nextScheduledAtStr, &estimatedRequests,
		&scheduledResumeAtStr, &pauseReason, &lastBudgetCheckStr,
	); err != nil {
		return SyncJob{}, fmt.Errorf("scan paused sync job: %w", err)
	}

	job.Status = SyncJobStatus(status)
	job.CreatedAt = parseOptionalTime(createdAt)
	job.UpdatedAt = parseOptionalTime(updatedAt)
	job.LastSyncAt = parseOptionalTime(lastSync)
	job.Progress.EstimatedRequests = estimatedRequests
	job.Progress.NextScheduledAt = parseOptionalTime(nextScheduledAtStr)
	job.Progress.ScheduledResumeAt = parseOptionalTime(scheduledResumeAtStr)
	job.Progress.PauseReason = pauseReason
	job.Progress.LastBudgetCheck = parseOptionalTime(lastBudgetCheckStr)

	if err := rows.Err(); err != nil {
		return SyncJob{}, fmt.Errorf("iterate paused sync job: %w", err)
	}

	return job, nil
}

func (s *Store) init(ctx context.Context) error {
	const supportedSchemaVersion = 7

	var currentVersion int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version;`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("query user_version: %w", err)
	}

	if currentVersion > supportedSchemaVersion {
		return fmt.Errorf("unsupported database schema version %d: binary supports up to version %d", currentVersion, supportedSchemaVersion)
	}

	pragmas := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA busy_timeout=5000;`,
		`PRAGMA foreign_keys=ON;`,
	}
	for _, stmt := range pragmas {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply pragma %q: %w", stmt, err)
		}
	}

	schema := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);`,
		`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
		 VALUES (1, 'baseline', '2026-03-12T00:00:00Z');`,
		`PRAGMA user_version = 1;`,
		`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
		 VALUES (2, 'audit_log', '2026-03-22T00:00:00Z');`,
		`PRAGMA user_version = 2;`,
		`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
		 VALUES (3, 'sync_progress_scheduling', '2026-04-02T00:00:00Z');`,
		`PRAGMA user_version = 3;`,
		`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
		 VALUES (4, 'field_provenance', '2026-04-12T00:00:00Z');`,
		`PRAGMA user_version = 4;`,
		`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
		 VALUES (5, 'sync_snapshot_ceiling', '2026-04-16T00:00:00Z');`,
		`PRAGMA user_version = 5;`,
		`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
		 VALUES (6, 'intermediate_cache', '2026-04-18T00:00:00Z');`,
		`PRAGMA user_version = 6;`,
		`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
		 VALUES (7, 'repo_name_normalization', '2026-04-18T00:00:00Z');`,
		`PRAGMA user_version = 7;`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL,
			action TEXT NOT NULL,
			repo TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp DESC);`,
		`CREATE TABLE IF NOT EXISTS pull_requests (
			id TEXT NOT NULL,
			repo TEXT NOT NULL,
			number INTEGER NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			url TEXT NOT NULL,
			author TEXT NOT NULL,
			labels_json TEXT NOT NULL,
			files_changed_json TEXT NOT NULL,
			review_status TEXT NOT NULL,
			ci_status TEXT NOT NULL,
			mergeable TEXT NOT NULL,
			base_branch TEXT NOT NULL,
			head_branch TEXT NOT NULL,
			cluster_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			is_draft INTEGER NOT NULL,
			is_bot INTEGER NOT NULL,
			additions INTEGER NOT NULL,
			deletions INTEGER NOT NULL,
			changed_files_count INTEGER NOT NULL,
			provenance_json TEXT NOT NULL DEFAULT '{}',
			PRIMARY KEY (repo, number)
		);`,
		`CREATE TABLE IF NOT EXISTS pr_files (
			repo TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			path TEXT NOT NULL,
			PRIMARY KEY (repo, pr_number, path)
		);`,
		`CREATE TABLE IF NOT EXISTS pr_reviews (
			repo TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			author TEXT NOT NULL,
			state TEXT NOT NULL,
			PRIMARY KEY (repo, pr_number, author, state)
		);`,
		`CREATE TABLE IF NOT EXISTS ci_status (
			repo TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			state TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (repo, pr_number)
		);`,
		`CREATE TABLE IF NOT EXISTS sync_jobs (
			id TEXT PRIMARY KEY,
			repo TEXT NOT NULL,
			status TEXT NOT NULL,
			error_message TEXT NOT NULL DEFAULT '',
			last_sync_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sync_progress (
			repo TEXT PRIMARY KEY,
			job_id TEXT NOT NULL DEFAULT '',
			cursor TEXT NOT NULL DEFAULT '',
			processed_prs INTEGER NOT NULL DEFAULT 0,
			total_prs INTEGER NOT NULL DEFAULT 0,
			snapshot_ceiling INTEGER NOT NULL DEFAULT 0,
			last_sync_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS merged_pr_index (
			repo TEXT NOT NULL,
			number INTEGER NOT NULL,
			merged_at TEXT NOT NULL,
			files_touched_json TEXT NOT NULL,
			PRIMARY KEY (repo, number)
		);`,
		`CREATE TABLE IF NOT EXISTS duplicate_groups (
			repo TEXT NOT NULL,
			canonical_pr INTEGER NOT NULL,
			duplicate_prs_json TEXT NOT NULL,
			similarity REAL NOT NULL,
			corpus_fingerprint TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (repo, canonical_pr, corpus_fingerprint)
		);`,
		`CREATE TABLE IF NOT EXISTS conflict_cache (
			repo TEXT NOT NULL,
			pr_a INTEGER NOT NULL,
			pr_b INTEGER NOT NULL,
			severity TEXT NOT NULL,
			conflict_type TEXT NOT NULL,
			shared_files_json TEXT NOT NULL,
			corpus_fingerprint TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (repo, pr_a, pr_b, corpus_fingerprint)
		);`,
		`CREATE TABLE IF NOT EXISTS substance_cache (
			repo TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			score INTEGER NOT NULL,
			corpus_fingerprint TEXT NOT NULL,
			computed_at TEXT NOT NULL,
			PRIMARY KEY (repo, pr_number, corpus_fingerprint)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_pull_requests_base_branch ON pull_requests(base_branch);`,
		`CREATE INDEX IF NOT EXISTS idx_pull_requests_ci_status ON pull_requests(ci_status);`,
		`CREATE INDEX IF NOT EXISTS idx_pull_requests_updated_at ON pull_requests(updated_at DESC);`,
	}

	for _, stmt := range schema {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}

	if err := s.addColumnIfNotExists("sync_progress", "next_scheduled_at", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("sync_progress", "estimated_requests", "INTEGER"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("sync_progress", "scheduled_resume_at", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("sync_progress", "pause_reason", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("sync_progress", "last_budget_check", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("sync_progress", "snapshot_ceiling", "INTEGER"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("pull_requests", "provenance_json", "TEXT NOT NULL DEFAULT '{}' "); err != nil {
		return err
	}

	// Migration v7: normalize repo names to lowercase.
	// For tables with (repo, number) unique constraints, delete rows from
	// mixed-case variants that conflict with existing lowercase rows before
	// normalizing. The lowercase entry is always kept (it has more data from
	// the primary cache).
	if currentVersion < 7 {
		// Tables with (repo, number) unique keys — deduplicate first
		dedupeTables := []struct {
			table string
			key   string // the non-repo part of the unique key
		}{
			{"pull_requests", "number"},
			{"pr_files", "pr_number"},
			{"pr_reviews", "pr_number"},
			{"ci_status", "pr_number"},
			{"merged_pr_index", "number"},
		}
		for _, dt := range dedupeTables {
			// Delete rows where LOWER(repo) matches an existing lowercase row
			_, _ = s.db.ExecContext(ctx, fmt.Sprintf(
				`DELETE FROM %s WHERE repo != LOWER(repo) AND EXISTS (
					SELECT 1 FROM %s t2 WHERE t2.repo = LOWER(%s.repo) AND t2.%s = %s.%s
				)`, dt.table, dt.table, dt.table, dt.key, dt.table, dt.key,
			))
			// Now normalize remaining rows
			_, err := s.db.ExecContext(ctx, fmt.Sprintf(
				`UPDATE %s SET repo = LOWER(repo) WHERE repo != LOWER(repo)`, dt.table,
			))
			if err != nil {
				return fmt.Errorf("migrate repo normalization on %s: %w", dt.table, err)
			}
		}

		// Tables without unique key conflicts — just lowercase
		simpleTables := []string{
			"audit_log",
			"sync_progress",
			"sync_jobs",
			"duplicate_groups",
			"conflict_cache",
			"substance_cache",
		}
		for _, table := range simpleTables {
			_, err := s.db.ExecContext(ctx, fmt.Sprintf(
				`UPDATE %s SET repo = LOWER(repo) WHERE repo != LOWER(repo)`, table,
			))
			if err != nil {
				return fmt.Errorf("migrate repo normalization on %s: %w", table, err)
			}
		}
	}

	return nil
}

func (s *Store) addColumnIfNotExists(table, column, colType string) error {
	var exists int
	err := s.db.QueryRow(
		`SELECT 1 FROM pragma_table_info(?) WHERE name = ?`, table, column,
	).Scan(&exists)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check column %s.%s: %w", table, column, err)
	}
	if exists == 1 {
		return nil
	}
	_, err = s.db.Exec(
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, colType),
	)
	if err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

func parseOptionalTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// CorpusFingerprint computes a deterministic hash of the PR corpus.
// It uses sorted PR numbers and count to produce a fingerprint that changes
// when the PR set changes, enabling cache invalidation.
func CorpusFingerprint(prs []types.PR) string {
	if len(prs) == 0 {
		return "empty"
	}
	// Extract and sort PR numbers
	numbers := make([]int, len(prs))
	for i, pr := range prs {
		numbers[i] = pr.Number
	}
	sort.Ints(numbers)

	// Build a string representation
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("count:%d,", len(prs)))
	for i, n := range numbers {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%d", n))
	}

	// Hash the string using a simple hash
	h := fnvHash(sb.String())
	return fmt.Sprintf("%x", h)
}

// fnvHash computes a 64-bit FNV-1a hash of the input string.
func fnvHash(s string) uint64 {
	const (
		fnvOffset uint64 = 14695981039346656037
		fnvPrime  uint64 = 1099511628211
	)
	var h uint64 = fnvOffset
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

// SaveDuplicateGroups saves duplicate groups to the cache, replacing any existing
// entries for the same repo and fingerprint.
func (s *Store) SaveDuplicateGroups(repo string, groups []types.DuplicateGroup, fingerprint string) error {
	if len(groups) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete existing entries for this repo and fingerprint
	if _, err := tx.Exec(`DELETE FROM duplicate_groups WHERE repo = ? AND corpus_fingerprint = ?`, repo, fingerprint); err != nil {
		return fmt.Errorf("delete existing duplicate groups: %w", err)
	}

	now := s.now().UTC().Format(time.RFC3339)
	for _, group := range groups {
		dupsJSON, err := json.Marshal(group.DuplicatePRNums)
		if err != nil {
			return fmt.Errorf("marshal duplicate prs: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO duplicate_groups (repo, canonical_pr, duplicate_prs_json, similarity, corpus_fingerprint, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, repo, group.CanonicalPRNumber, string(dupsJSON), group.Similarity, fingerprint, now); err != nil {
			return fmt.Errorf("insert duplicate group: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit duplicate groups: %w", err)
	}
	return nil
}

// LoadDuplicateGroups loads duplicate groups from the cache.
// Returns (groups, found, error). If no cache entry exists for the given
// repo and fingerprint, returns (nil, false, nil).
func (s *Store) LoadDuplicateGroups(repo string, fingerprint string) ([]types.DuplicateGroup, bool, error) {
	rows, err := s.db.Query(`
		SELECT canonical_pr, duplicate_prs_json, similarity
		FROM duplicate_groups
		WHERE repo = ? AND corpus_fingerprint = ?
		ORDER BY canonical_pr ASC
	`, repo, fingerprint)
	if err != nil {
		return nil, false, fmt.Errorf("query duplicate groups: %w", err)
	}
	defer rows.Close()

	var groups []types.DuplicateGroup
	for rows.Next() {
		var canonicalPR int
		var dupsJSON string
		var similarity float64
		if err := rows.Scan(&canonicalPR, &dupsJSON, &similarity); err != nil {
			return nil, false, fmt.Errorf("scan duplicate group: %w", err)
		}

		var dups []int
		if err := json.Unmarshal([]byte(dupsJSON), &dups); err != nil {
			return nil, false, fmt.Errorf("unmarshal duplicate prs: %w", err)
		}

		groups = append(groups, types.DuplicateGroup{
			CanonicalPRNumber: canonicalPR,
			DuplicatePRNums:   dups,
			Similarity:        similarity,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate duplicate groups: %w", err)
	}

	if len(groups) == 0 {
		return nil, false, nil
	}
	return groups, true, nil
}

// SaveConflictCache saves conflict pairs to the cache, replacing any existing
// entries for the same repo and fingerprint.
func (s *Store) SaveConflictCache(repo string, conflicts []types.ConflictPair, fingerprint string) error {
	if len(conflicts) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete existing entries for this repo and fingerprint
	if _, err := tx.Exec(`DELETE FROM conflict_cache WHERE repo = ? AND corpus_fingerprint = ?`, repo, fingerprint); err != nil {
		return fmt.Errorf("delete existing conflict cache: %w", err)
	}

	now := s.now().UTC().Format(time.RFC3339)
	for _, conflict := range conflicts {
		filesJSON, err := json.Marshal(conflict.FilesTouched)
		if err != nil {
			return fmt.Errorf("marshal shared files: %w", err)
		}

		// Store with ordered pr_a < pr_b for consistent primary key
		prA, prB := conflict.SourcePR, conflict.TargetPR
		if prA > prB {
			prA, prB = prB, prA
		}

		if _, err := tx.Exec(`
			INSERT INTO conflict_cache (repo, pr_a, pr_b, severity, conflict_type, shared_files_json, corpus_fingerprint, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, repo, prA, prB, conflict.Severity, conflict.ConflictType, string(filesJSON), fingerprint, now); err != nil {
			return fmt.Errorf("insert conflict cache: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit conflict cache: %w", err)
	}
	return nil
}

// LoadConflictCache loads conflict pairs from the cache.
// Returns (conflicts, found, error). If no cache entry exists for the given
// repo and fingerprint, returns (nil, false, nil).
func (s *Store) LoadConflictCache(repo string, fingerprint string) ([]types.ConflictPair, bool, error) {
	rows, err := s.db.Query(`
		SELECT pr_a, pr_b, severity, conflict_type, shared_files_json
		FROM conflict_cache
		WHERE repo = ? AND corpus_fingerprint = ?
		ORDER BY pr_a ASC, pr_b ASC
	`, repo, fingerprint)
	if err != nil {
		return nil, false, fmt.Errorf("query conflict cache: %w", err)
	}
	defer rows.Close()

	var conflicts []types.ConflictPair
	for rows.Next() {
		var prA, prB int
		var severity, conflictType string
		var filesJSON string
		if err := rows.Scan(&prA, &prB, &severity, &conflictType, &filesJSON); err != nil {
			return nil, false, fmt.Errorf("scan conflict cache: %w", err)
		}

		var files []string
		if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
			return nil, false, fmt.Errorf("unmarshal shared files: %w", err)
		}

		conflicts = append(conflicts, types.ConflictPair{
			SourcePR:     prA,
			TargetPR:     prB,
			Severity:     severity,
			ConflictType: conflictType,
			FilesTouched: files,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate conflict cache: %w", err)
	}

	if len(conflicts) == 0 {
		return nil, false, nil
	}
	return conflicts, true, nil
}

// SaveSubstanceCache saves substance scores to the cache, replacing any existing
// entries for the same repo and fingerprint.
func (s *Store) SaveSubstanceCache(repo string, scores map[int]int, fingerprint string) error {
	if len(scores) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete existing entries for this repo and fingerprint
	if _, err := tx.Exec(`DELETE FROM substance_cache WHERE repo = ? AND corpus_fingerprint = ?`, repo, fingerprint); err != nil {
		return fmt.Errorf("delete existing substance cache: %w", err)
	}

	now := s.now().UTC().Format(time.RFC3339)
	for prNumber, score := range scores {
		if _, err := tx.Exec(`
			INSERT INTO substance_cache (repo, pr_number, score, corpus_fingerprint, computed_at)
			VALUES (?, ?, ?, ?, ?)
		`, repo, prNumber, score, fingerprint, now); err != nil {
			return fmt.Errorf("insert substance cache: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit substance cache: %w", err)
	}
	return nil
}

// LoadSubstanceCache loads substance scores from the cache.
// Returns (scores, found, error). If no cache entry exists for the given
// repo and fingerprint, returns (nil, false, nil).
func (s *Store) LoadSubstanceCache(repo string, fingerprint string) (map[int]int, bool, error) {
	rows, err := s.db.Query(`
		SELECT pr_number, score
		FROM substance_cache
		WHERE repo = ? AND corpus_fingerprint = ?
		ORDER BY pr_number ASC
	`, repo, fingerprint)
	if err != nil {
		return nil, false, fmt.Errorf("query substance cache: %w", err)
	}
	defer rows.Close()

	scores := make(map[int]int)
	for rows.Next() {
		var prNumber, score int
		if err := rows.Scan(&prNumber, &score); err != nil {
			return nil, false, fmt.Errorf("scan substance cache: %w", err)
		}
		scores[prNumber] = score
	}

	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate substance cache: %w", err)
	}

	if len(scores) == 0 {
		return nil, false, nil
	}
	return scores, true, nil
}
