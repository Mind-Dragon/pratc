package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

	_, err = s.db.Exec(`
		INSERT INTO pull_requests (
			id, repo, number, title, body, url, author, labels_json, files_changed_json,
			review_status, ci_status, mergeable, base_branch, head_branch, cluster_id,
			created_at, updated_at, is_draft, is_bot, additions, deletions, changed_files_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			changed_files_count = excluded.changed_files_count
	`,
		pr.ID, pr.Repo, pr.Number, pr.Title, pr.Body, pr.URL, pr.Author, string(labelsJSON), string(filesJSON),
		pr.ReviewStatus, pr.CIStatus, pr.Mergeable, pr.BaseBranch, pr.HeadBranch, pr.ClusterID,
		pr.CreatedAt, pr.UpdatedAt, pr.IsDraft, pr.IsBot, pr.Additions, pr.Deletions, pr.ChangedFilesCount,
	)
	if err != nil {
		return fmt.Errorf("upsert pull request %d: %w", pr.Number, err)
	}

	return nil
}

func (s *Store) ListPRs(filter PRFilter) ([]types.PR, error) {
	query := `
		SELECT
			id, repo, number, title, body, url, author, labels_json, files_changed_json,
			review_status, ci_status, mergeable, base_branch, head_branch, cluster_id,
			created_at, updated_at, is_draft, is_bot, additions, deletions, changed_files_count
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

	query += ` ORDER BY number ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pull requests: %w", err)
	}
	defer rows.Close()

	var prs []types.PR
	for rows.Next() {
		var pr types.PR
		var labelsJSON string
		var filesJSON string

		if err := rows.Scan(
			&pr.ID, &pr.Repo, &pr.Number, &pr.Title, &pr.Body, &pr.URL, &pr.Author, &labelsJSON, &filesJSON,
			&pr.ReviewStatus, &pr.CIStatus, &pr.Mergeable, &pr.BaseBranch, &pr.HeadBranch, &pr.ClusterID,
			&pr.CreatedAt, &pr.UpdatedAt, &pr.IsDraft, &pr.IsBot, &pr.Additions, &pr.Deletions, &pr.ChangedFilesCount,
		); err != nil {
			return nil, fmt.Errorf("scan pull request: %w", err)
		}

		if err := json.Unmarshal([]byte(labelsJSON), &pr.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
		if err := json.Unmarshal([]byte(filesJSON), &pr.FilesChanged); err != nil {
			return nil, fmt.Errorf("unmarshal files changed: %w", err)
		}

		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pull requests: %w", err)
	}

	return prs, nil
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

func (s *Store) CreateSyncJob(repo string) (SyncJob, error) {
	now := s.now().UTC()
	job := SyncJob{
		ID:        fmt.Sprintf("%s-%d", repo, now.UnixNano()),
		Repo:      repo,
		Status:    SyncJobStatusInProgress,
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
		INSERT INTO sync_progress (repo, job_id, cursor, processed_prs, total_prs, last_sync_at, updated_at)
		VALUES (?, ?, '', 0, 0, '', ?)
		ON CONFLICT(repo) DO UPDATE SET
			job_id = excluded.job_id,
			cursor = excluded.cursor,
			processed_prs = excluded.processed_prs,
			total_prs = excluded.total_prs,
			updated_at = excluded.updated_at
	`, repo, job.ID, now.Format(time.RFC3339))
	if err != nil {
		return SyncJob{}, fmt.Errorf("initialize sync progress: %w", err)
	}

	return job, nil
}

func (s *Store) UpdateSyncJobProgress(jobID string, progress SyncProgress) error {
	var repo string
	err := s.db.QueryRow(`SELECT repo FROM sync_jobs WHERE id = ?`, jobID).Scan(&repo)
	if err != nil {
		return fmt.Errorf("lookup sync job repo: %w", err)
	}

	now := s.now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
		UPDATE sync_jobs
		SET updated_at = ?
		WHERE id = ?
	`, now, jobID)
	if err != nil {
		return fmt.Errorf("touch sync job: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO sync_progress (repo, job_id, cursor, processed_prs, total_prs, last_sync_at, updated_at)
		VALUES (?, ?, ?, ?, ?, '', ?)
		ON CONFLICT(repo) DO UPDATE SET
			job_id = excluded.job_id,
			cursor = excluded.cursor,
			processed_prs = excluded.processed_prs,
			total_prs = excluded.total_prs,
			updated_at = excluded.updated_at
	`, repo, jobID, progress.Cursor, progress.ProcessedPRs, progress.TotalPRs, now)
	if err != nil {
		return fmt.Errorf("update sync progress: %w", err)
	}

	return nil
}

func (s *Store) GetSyncJob(jobID string) (SyncJob, error) {
	row := s.db.QueryRow(`
		SELECT
			j.id, j.repo, j.status, j.error_message, COALESCE(j.last_sync_at, ''), j.created_at, j.updated_at,
			COALESCE(p.cursor, ''), COALESCE(p.processed_prs, 0), COALESCE(p.total_prs, 0)
		FROM sync_jobs j
		LEFT JOIN sync_progress p ON p.job_id = j.id
		WHERE j.id = ?
	`, jobID)

	var job SyncJob
	var status string
	var lastSync string
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&job.ID, &job.Repo, &status, &job.Error, &lastSync, &createdAt, &updatedAt,
		&job.Progress.Cursor, &job.Progress.ProcessedPRs, &job.Progress.TotalPRs,
	); err != nil {
		return SyncJob{}, fmt.Errorf("get sync job: %w", err)
	}

	job.Status = SyncJobStatus(status)
	job.CreatedAt = parseOptionalTime(createdAt)
	job.UpdatedAt = parseOptionalTime(updatedAt)
	job.LastSyncAt = parseOptionalTime(lastSync)
	return job, nil
}

func (s *Store) ResumeSyncJob(repo string) (SyncJob, bool, error) {
	var jobID string
	err := s.db.QueryRow(`
		SELECT id
		FROM sync_jobs
		WHERE repo = ? AND status = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, repo, SyncJobStatusInProgress).Scan(&jobID)
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
	_, err := s.db.Exec(`
		UPDATE sync_jobs
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, SyncJobStatusFailed, message, s.now().UTC().Format(time.RFC3339), jobID)
	if err != nil {
		return fmt.Errorf("mark sync job failed: %w", err)
	}
	return nil
}

func (s *Store) init(ctx context.Context) error {
	const supportedSchemaVersion = 2

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
	}

	for _, stmt := range schema {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
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
