# prATC Database Schema Guide

**Last Updated:** 2026-03-23  
**System Version:** v0.1  
**Driver:** modernc.org/sqlite (pure Go, no CGO)  
**Current Schema Version:** 2

## Table of Contents

1. [Overview](#overview)
2. [Database Configuration](#database-configuration)
3. [Table Reference](#table-reference)
4. [Entity Relationship Diagram](#entity-relationship-diagram)
5. [Schema Details](#schema-details)
6. [Migration System](#migration-system)
7. [Query Patterns](#query-patterns)
8. [Performance Considerations](#performance-considerations)
9. [Gotchas and Warnings](#gotchas-and-warnings)

---

## Overview

prATC uses SQLite for persistence with two separate databases:

1. **Main cache** (`PRATC_DB_PATH`) — PR data, sync jobs, audit log
2. **Settings** (`PRATC_SETTINGS_DB`) — Configuration settings (managed by `internal/settings/`)

This document covers the main cache database.

### Design Principles

- **Forward-only migrations** — No down-migrations, fix forward only
- **Idempotent schema** — `CREATE TABLE IF NOT EXISTS` for safety
- **JSON arrays as TEXT** — `labels` and `files_changed` stored as JSON strings
- **RFC3339 timestamps** — All times stored as TEXT in RFC3339 format
- **WAL mode** — Write-ahead logging for better concurrency

---

## Database Configuration

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PRATC_DB_PATH` | `~/.pratc/pratc.db` | Main cache database |
| `PRATC_SETTINGS_DB` | `./pratc-settings.db` | Settings database |

### Connection Pragmas

Applied automatically on `cache.Open()`:

```sql
PRAGMA journal_mode=WAL;        -- Write-ahead logging
PRAGMA busy_timeout=5000;       -- 5 second timeout for locked DB
PRAGMA foreign_keys=ON;         -- Enforce foreign key constraints
PRAGMA user_version=2;          -- Schema version (migration tracking)
```

### WAL Mode

Write-ahead logging creates companion files:
- `{dbname}.db-wal` — WAL journal
- `{dbname}.db-shm` — Shared memory file

**Note:** These files must be preserved alongside the main DB file.

---

## Table Reference

| Table | Purpose | Row Count (5.5k PRs) |
|-------|---------|---------------------|
| `pull_requests` | PR metadata | ~5,500 |
| `pr_files` | File associations | ~50,000 |
| `pr_reviews` | Review states | ~15,000 |
| `ci_status` | CI state per PR | ~5,500 |
| `sync_jobs` | Background sync jobs | ~100 |
| `sync_progress` | Cursor + progress tracking | ~10 |
| `merged_pr_index` | Merged PR history | ~50,000 |
| `schema_migrations` | Migration history | 2 |
| `audit_log` | Audit trail | Variable |

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Entity Relationships                               │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│  pull_requests  │◄───────►│    pr_files     │         │   pr_reviews    │
│                 │  1:N    │                 │         │                 │
│  PK: (repo,     │         │  PK: (repo,     │         │  PK: (repo,     │
│      number)    │         │      pr_number, │         │      pr_number, │
│                 │         │      path)      │         │      author,    │
│  - title        │         │                 │         │      state)     │
│  - author       │         │  - path         │         │                 │
│  - labels (JSON)│         └─────────────────┘         │  - state        │
│  - files (JSON) │                                   └─────────────────┘
│  - cluster_id   │
│  - is_draft     │         ┌─────────────────┐         ┌─────────────────┐
│  - is_bot       │◄───────►│   ci_status     │         │  sync_jobs      │
│  - created_at   │  1:1    │                 │         │                 │
│  - updated_at   │         │  PK: (repo,     │         │  PK: id         │
└─────────────────┘         │      pr_number) │         │                 │
                            │                 │         │  - repo         │
                            │  - status       │         │  - status       │
                            │  - conclusion   │         │  - cursor       │
                            └─────────────────┘         │  - error_msg    │
                                                        │  - created_at   │
                                                        │  - updated_at   │
                                                        └─────────────────┘
┌─────────────────┐         ┌─────────────────┐
│  sync_progress  │         │ merged_pr_index │
│                 │         │                 │
│  PK: repo       │         │  PK: (repo,     │
│                 │         │      number)    │
│  - job_id       │         │                 │
│  - cursor       │         │  - merged_at    │
│  - processed_prs│         │  - files (JSON) │
│  - total_prs    │         │  - author       │
│  - status       │         │  - title        │
└─────────────────┘         └─────────────────┘

┌─────────────────┐         ┌─────────────────┐
│ schema_migr.    │         │   audit_log     │
│                 │         │                 │
│  PK: version    │         │  PK: id         │
│                 │         │                 │
│  - name         │         │  - timestamp    │
│  - applied_at   │         │  - action       │
└─────────────────┘         │  - repo         │
                            │  - details      │
                            └─────────────────┘
```

---

## Schema Details

### pull_requests

Main table for PR metadata.

```sql
CREATE TABLE IF NOT EXISTS pull_requests (
  repo TEXT NOT NULL,
  number INTEGER NOT NULL,
  title TEXT NOT NULL,
  body TEXT,
  url TEXT NOT NULL,
  author TEXT NOT NULL,
  labels TEXT,              -- JSON array: ["bug", "enhancement"]
  files_changed TEXT,       -- JSON array: ["src/main.go", "src/utils.go"]
  review_status TEXT,
  ci_status TEXT,
  mergeable TEXT,
  base_branch TEXT,
  head_branch TEXT,
  cluster_id TEXT,
  created_at TEXT,          -- RFC3339
  updated_at TEXT,          -- RFC3339
  is_draft INTEGER,         -- 0 or 1
  is_bot INTEGER,           -- 0 or 1
  additions INTEGER,
  deletions INTEGER,
  changed_files_count INTEGER,
  PRIMARY KEY (repo, number)
);

CREATE INDEX IF NOT EXISTS idx_prs_cluster ON pull_requests(repo, cluster_id);
CREATE INDEX IF NOT EXISTS idx_prs_author ON pull_requests(repo, author);
CREATE INDEX IF NOT EXISTS idx_prs_status ON pull_requests(repo, review_status, ci_status);
```

**Key Go type mapping:**
```go
type PR struct {
  Repo              string   `json:"repo"`
  Number            int      `json:"number"`
  Title             string   `json:"title"`
  Labels            []string `json:"labels"`       // Stored as JSON TEXT
  FilesChanged      []string `json:"files_changed"` // Stored as JSON TEXT
  // ... other fields
}
```

### pr_files

Normalized file associations (alternative to JSON array in pull_requests).

```sql
CREATE TABLE IF NOT EXISTS pr_files (
  repo TEXT NOT NULL,
  pr_number INTEGER NOT NULL,
  path TEXT NOT NULL,
  PRIMARY KEY (repo, pr_number, path),
  FOREIGN KEY (repo, pr_number) REFERENCES pull_requests(repo, number)
);

CREATE INDEX IF NOT EXISTS idx_files_path ON pr_files(path);
```

**Note:** Currently both `pr_files` table and `files_changed` JSON exist. The JSON is used for quick access; the table enables file-based queries.

### pr_reviews

Review states per PR per reviewer.

```sql
CREATE TABLE IF NOT EXISTS pr_reviews (
  repo TEXT NOT NULL,
  pr_number INTEGER NOT NULL,
  author TEXT NOT NULL,
  state TEXT NOT NULL,      -- APPROVED, CHANGES_REQUESTED, COMMENTED
  submitted_at TEXT,        -- RFC3339
  body TEXT,
  PRIMARY KEY (repo, pr_number, author, state)
);
```

### ci_status

CI status per PR (one row per PR).

```sql
CREATE TABLE IF NOT EXISTS ci_status (
  repo TEXT NOT NULL,
  pr_number INTEGER NOT NULL,
  status TEXT,              -- PENDING, SUCCESS, FAILURE
  conclusion TEXT,          -- SUCCESS, FAILURE, CANCELLED, etc.
  check_runs TEXT,          -- JSON array of check details
  updated_at TEXT,          -- RFC3339
  PRIMARY KEY (repo, pr_number)
);
```

### sync_jobs

Background sync job tracking.

```sql
CREATE TABLE IF NOT EXISTS sync_jobs (
  id TEXT PRIMARY KEY,
  repo TEXT NOT NULL,
  status TEXT NOT NULL,     -- pending, running, completed, failed
  cursor TEXT,              -- GitHub GraphQL cursor (opaque)
  error_message TEXT,
  created_at TEXT,          -- RFC3339
  updated_at TEXT,          -- RFC3339
  completed_at TEXT         -- RFC3339
);

CREATE INDEX IF NOT EXISTS idx_sync_repo ON sync_jobs(repo);
CREATE INDEX IF NOT EXISTS idx_sync_status ON sync_jobs(status);
```

### sync_progress

Progress tracking for incremental sync.

```sql
CREATE TABLE IF NOT EXISTS sync_progress (
  repo TEXT PRIMARY KEY,
  job_id TEXT,
  cursor TEXT,              -- Resume cursor
  processed_prs INTEGER,
  total_prs INTEGER,
  status TEXT               -- in_progress, completed, failed
);
```

**Resume flow:**
1. Check `sync_progress` for existing cursor
2. If cursor exists, resume from that point
3. If no cursor, start fresh sync
4. Update cursor after each page

### merged_pr_index

History of merged PRs for conflict prediction.

```sql
CREATE TABLE IF NOT EXISTS merged_pr_index (
  repo TEXT NOT NULL,
  number INTEGER NOT NULL,
  merged_at TEXT,           -- RFC3339
  files_touched TEXT,       -- JSON array of files
  author TEXT,
  title TEXT,
  PRIMARY KEY (repo, number)
);

CREATE INDEX IF NOT EXISTS idx_merged_time ON merged_pr_index(merged_at);
```

### schema_migrations

Migration tracking (forward-only).

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT,
  applied_at TEXT           -- RFC3339
);
```

**Current versions:**
- v1: Baseline schema
- v2: Added audit_log table

### audit_log

Audit trail for operations.

```sql
CREATE TABLE IF NOT EXISTS audit_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp TEXT NOT NULL,  -- RFC3339
  action TEXT NOT NULL,     -- analyze, cluster, plan, sync, etc.
  repo TEXT,
  details TEXT              -- JSON with operation details
);

CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_repo ON audit_log(repo);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
```

---

## Migration System

### Philosophy

- **Forward-only** — Never migrate down
- **Idempotent** — `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ADD COLUMN`
- **Version tracking** — `schema_migrations` table + `PRAGMA user_version`
- **Fail fast** — Binary refuses to start if DB version > binary version

### Migration Code Structure

Migrations are inline in `init()`, not separate files:

```go
func init() {
  migrations = []migration{
    {version: 1, name: "baseline", fn: migrateV1},
    {version: 2, name: "add_audit_log", fn: migrateV2},
  }
}

func applyMigrations(db *sql.DB) error {
  var userVersion int
  _ = db.QueryRow("PRAGMA user_version").Scan(&userVersion)
  
  // Fail if DB is newer than binary
  if userVersion > supportedSchemaVersion {
    return fmt.Errorf("database schema v%d is newer than binary v%d",
      userVersion, supportedSchemaVersion)
  }
  
  // Apply pending migrations
  for _, m := range migrations {
    if m.version > userVersion {
      if err := m.fn(db); err != nil {
        return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
      }
      // Record migration
      _, _ = db.Exec(
        "INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
        m.version, m.name, time.Now().UTC().Format(time.RFC3339),
      )
      // Update pragma
      _, _ = db.Exec(fmt.Sprintf("PRAGMA user_version=%d", m.version))
    }
  }
  
  return nil
}
```

### Current Migrations

**v1 — Baseline:**
```sql
CREATE TABLE IF NOT EXISTS pull_requests (...);
CREATE TABLE IF NOT EXISTS pr_files (...);
CREATE TABLE IF NOT EXISTS pr_reviews (...);
CREATE TABLE IF NOT EXISTS ci_status (...);
CREATE TABLE IF NOT EXISTS sync_jobs (...);
CREATE TABLE IF NOT EXISTS sync_progress (...);
CREATE TABLE IF NOT EXISTS merged_pr_index (...);
CREATE TABLE IF NOT EXISTS schema_migrations (...);
```

**v2 — Add audit_log:**
```sql
CREATE TABLE IF NOT EXISTS audit_log (...);
CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_repo ON audit_log(repo);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
```

### Testing Migrations

Verify upgrade paths:
- Fresh DB → v2
- v1 DB → v2
- v0 (pre-migration) → v2

---

## Query Patterns

### List PRs with Filter

```go
// From cache/store.go
func (s *Store) ListPRs(filter PRFilter) ([]types.PR, error) {
  query := `
    SELECT repo, number, title, body, url, author,
           labels, files_changed, review_status, ci_status,
           mergeable, base_branch, head_branch, cluster_id,
           created_at, updated_at, is_draft, is_bot,
           additions, deletions, changed_files_count
    FROM pull_requests
    WHERE repo = ?
  `
  args := []interface{}{filter.Repo}
  
  if filter.Author != "" {
    query += " AND author = ?"
    args = append(args, filter.Author)
  }
  if filter.ClusterID != "" {
    query += " AND cluster_id = ?"
    args = append(args, filter.ClusterID)
  }
  if !filter.IncludeDrafts {
    query += " AND is_draft = 0"
  }
  if !filter.IncludeBots {
    query += " AND is_bot = 0"
  }
  
  rows, err := s.db.Query(query, args...)
  // ... scan rows
}
```

### Upsert PR

```go
func (s *Store) UpsertPR(pr types.PR) error {
  labelsJSON, _ := json.Marshal(pr.Labels)
  filesJSON, _ := json.Marshal(pr.FilesChanged)
  
  _, err := s.db.Exec(`
    INSERT INTO pull_requests (
      repo, number, title, body, url, author,
      labels, files_changed, review_status, ci_status,
      mergeable, base_branch, head_branch, cluster_id,
      created_at, updated_at, is_draft, is_bot,
      additions, deletions, changed_files_count
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(repo, number) DO UPDATE SET
      title=excluded.title,
      body=excluded.body,
      -- ... other fields
      updated_at=excluded.updated_at
  `, pr.Repo, pr.Number, /* ... */)
  
  return err
}
```

### Resume Sync

```go
func (s *Store) ResumeSyncJob(repo string) (SyncJob, bool, error) {
  var progress SyncProgress
  err := s.db.QueryRow(`
    SELECT job_id, cursor, processed_prs, total_prs, status
    FROM sync_progress
    WHERE repo = ?
  `, repo).Scan(&progress.JobID, &progress.Cursor, /* ... */)
  
  if err == sql.ErrNoRows {
    return SyncJob{}, false, nil  // No existing sync
  }
  
  // Resume from cursor
  return SyncJob{Cursor: progress.Cursor}, true, nil
}
```

### File-based Conflict Detection

```sql
-- Find PRs touching the same files
SELECT a.repo, a.pr_number, b.pr_number, a.path
FROM pr_files a
JOIN pr_files b ON a.path = b.path
  AND a.repo = b.repo
  AND a.pr_number < b.pr_number
WHERE a.repo = ?
```

---

## Performance Considerations

### Indexing Strategy

**Current indexes:**
- `idx_prs_cluster` — Cluster lookup
- `idx_prs_author` — Author filtering
- `idx_prs_status` — Status filtering
- `idx_files_path` — File conflict detection
- `idx_sync_repo` — Sync job queries
- `idx_audit_time/repo/action` — Audit log queries

### Query Optimization

**For 5,500 PRs:**
- List queries: < 100ms
- Upsert batch: < 1s for 100 PRs
- File conflict query: < 500ms

**Tips:**
- Use `PRAGMA optimize` periodically
- Vacuum after large deletes: `VACUUM;`
- Analyze after schema changes: `ANALYZE;`

### WAL Mode Tuning

**Checkpoint settings (consider for high write volume):**
```sql
PRAGMA wal_autocheckpoint=1000;  -- Checkpoint every 1000 pages
```

### Connection Pool

Currently no connection pooling (single connection per Store). For higher concurrency, consider:

```go
db.SetMaxOpenConns(10)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(time.Hour)
```

---

## Gotchas and Warnings

### 1. Settings DB is Separate

Do not confuse `PRATC_DB_PATH` with `PRATC_SETTINGS_DB`:
- `PRATC_DB_PATH`: PR data, sync jobs, audit log
- `PRATC_SETTINGS_DB`: Configuration settings only

### 2. JSON Arrays as TEXT

Labels and files are stored as JSON strings:

```go
// Store
labelsJSON, _ := json.Marshal(pr.Labels)
db.Exec("INSERT ... labels", string(labelsJSON))

// Retrieve
var labelsJSON string
db.QueryRow("SELECT labels ...").Scan(&labelsJSON)
json.Unmarshal([]byte(labelsJSON), &pr.Labels)
```

### 3. Boolean as INTEGER

SQLite has no native boolean; use 0/1:

```sql
is_draft INTEGER,  -- 0 = false, 1 = true
```

```go
var isDraft int
db.QueryRow("SELECT is_draft ...").Scan(&isDraft)
pr.IsDraft = isDraft == 1
```

### 4. Time Storage

All timestamps stored as RFC3339 TEXT:

```go
// Store
db.Exec("INSERT ... created_at", pr.CreatedAt.Format(time.RFC3339))

// Retrieve
var createdAt string
db.QueryRow("SELECT created_at ...").Scan(&createdAt)
t, _ := time.Parse(time.RFC3339, createdAt)
```

### 5. No Down Migrations

To "undo" a migration, create a new forward migration:

```go
// Instead of down-migrating v2, create v3:
func migrateV3(db *sql.DB) error {
  // Drop audit_log table (if needed)
  _, err := db.Exec("DROP TABLE IF EXISTS audit_log")
  return err
}
```

### 6. WAL Files Must Be Preserved

The `.db-wal` and `.db-shm` files are part of the database. Never delete them while the database is in use.

### 7. Busy Timeout

Default 5 seconds. Long queries may fail with "database is locked":

```sql
-- Check busy timeout
PRAGMA busy_timeout;  -- Returns 5000 (milliseconds)
```

### 8. Foreign Keys

Foreign keys are enforced (`PRAGMA foreign_keys=ON`), but SQLite foreign keys have limitations:
- No `ON UPDATE CASCADE` for composite keys
- Deferred constraints not supported in all cases

---

## Schema Version History

| Version | Date | Changes |
|---------|------|---------|
| 1 | Initial | Baseline schema with all core tables |
| 2 | 2026-03 | Added audit_log table with indexes |

---

## SQL Reference

### Useful Queries

**Count PRs per cluster:**
```sql
SELECT cluster_id, COUNT(*) as count
FROM pull_requests
WHERE repo = 'owner/repo'
GROUP BY cluster_id;
```

**Find stale PRs (>30 days):**
```sql
SELECT number, title, updated_at
FROM pull_requests
WHERE repo = 'owner/repo'
  AND datetime(updated_at) < datetime('now', '-30 days');
```

**Top authors by PR count:**
```sql
SELECT author, COUNT(*) as pr_count
FROM pull_requests
WHERE repo = 'owner/repo'
GROUP BY author
ORDER BY pr_count DESC
LIMIT 10;
```

**Recent sync jobs:**
```sql
SELECT * FROM sync_jobs
WHERE repo = 'owner/repo'
ORDER BY created_at DESC
LIMIT 10;
```

**Audit log summary:**
```sql
SELECT action, COUNT(*) as count,
       MIN(timestamp) as first_occurrence,
       MAX(timestamp) as last_occurrence
FROM audit_log
WHERE repo = 'owner/repo'
GROUP BY action;
```
