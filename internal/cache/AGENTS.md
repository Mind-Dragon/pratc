# AGENTS.md — internal/cache

**Package**: SQLite cache with forward-only migrations  
**Driver**: `modernc.org/sqlite` (pure Go, no CGO)

## Schema Version
Current: **7** (`supportedSchemaVersion` in `sqlite.go`)

Migrations:
- v1 (baseline) - 2026-03-12
- v2 (audit_log) - 2026-03-22
- v3 (sync_progress_scheduling) - 2026-04-02
- v4 (field_provenance) - 2026-04-12
- v5 (sync_snapshot_ceiling) - 2026-04-16
- v6 (intermediate_cache) - 2026-04-18
- v7 (repo_name_normalization) - 2026-04-18

## Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `pull_requests` | PR data | `(repo, number)` PK, `cluster_id`, JSON arrays for labels/files |
| `pr_files` | File associations | `(repo, pr_number, path)` PK |
| `pr_reviews` | Review states | `(repo, pr_number, author, state)` PK |
| `ci_status` | CI state per PR | `(repo, pr_number)` PK |
| `sync_jobs` | Job tracking | `id` PK, `status`, `error_message` |
| `sync_progress` | Cursor + counts | `repo` PK, `job_id`, `cursor`, `processed_prs` |
| `merged_pr_index` | Merged PR history | `(repo, number)` PK, `files_touched_json` |
| `schema_migrations` | Migration log | `version` PK, `name`, `applied_at` |
| `audit_log` | Audit trail | `id` PK, `timestamp`, `action`, `repo` |

## Key Methods (Store)

```go
Open(path string) (*Store, error)          // Opens DB, runs migrations, applies pragmas
UpsertPR(pr types.PR) error                 // INSERT ... ON CONFLICT DO UPDATE
ListPRs(filter PRFilter) ([]types.PR, error)

// Sync operations
CreateSyncJob(repo string) (SyncJob, error)
UpdateSyncJobProgress(jobID string, progress SyncProgress) error
MarkSyncJobComplete(jobID string, syncedAt time.Time) error
MarkSyncJobFailed(jobID string, message string) error
ResumeSyncJob(repo string) (SyncJob, bool, error)
SetLastSync(repo string, syncedAt time.Time) error
LastSync(repo string) (time.Time, error)

// Merged PR tracking
UpsertMergedPR(pr MergedPR) error
ListMergedPRs(repo string) ([]MergedPR, error)
```

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `PRATC_DB_PATH` | `~/.pratc/pratc.db` | Main cache database |
| `PRATC_SETTINGS_DB` | `~/.pratc/settings.db` | Settings database (separate, managed by `settings/store.go`) |

## Pragmas Applied on Open

```sql
PRAGMA journal_mode=WAL;        -- Write-ahead logging
PRAGMA busy_timeout=5000;       -- 5s timeout for locked DB
PRAGMA foreign_keys=ON;         -- Enforce FK constraints
```

## Migration System

Migrations are **inline** in `init()`, not separate files:

1. Check `PRAGMA user_version`
2. Fail fast if DB version > binary version
3. Apply `CREATE TABLE IF NOT EXISTS` statements (idempotent)
4. Insert into `schema_migrations`, update `PRAGMA user_version`

Current migrations: v1 (baseline), v2 (audit_log)

## Gotchas

- **Settings DB is separate**: Do not confuse `PRATC_DB_PATH` with `PRATC_SETTINGS_DB`
- **JSON arrays**: `labels` and `files_changed` stored as JSON strings, not normalized tables
- **Time storage**: All timestamps stored as RFC3339 strings in TEXT columns
- **No down migrations**: Forward-only. To rollback, restore from backup
- **WAL mode**: Journal files (`*.db-wal`, `*.db-shm`) appear alongside DB file
- **Busy timeout**: 5 seconds. Long queries may fail with "database is locked"
