# prATC — App Architecture Plan

## System Overview

prATC is an AI-powered, self-hostable, and repo-agnostic platform designed to manage extremely large repositories with 5,500+ open pull requests. Targeted at solo maintainers, it applies combinatorial optimization—as in air traffic control—to PR triage, deduplication, and merge planning. The system is a polyglot monorepo with three major components: the `pratc` binary (Go CLI and optional HTTP API), a Python-based ML service invoked via subprocess and JSON, and a Next.js-powered web dashboard. The architecture is a monolith-style CLI that can optionally expose an HTTP API (`pratc serve`).

### Architecture Pattern

- **Type**: Monolith (polyglot, CLI-first) with optional web API and ML subprocesses
- **Frontend**: Next.js 14+ web dashboard (port 3000)
- **Backend API**: Go-based HTTP API (port 8080), part of `pratc serve`
- **Database**: SQLite (file-based, WAL mode)
- **Background Jobs**: In-process background sync worker for PR metadata
- **External Services**: GitHub GraphQL/REST, Python ML subprocess (local) or OpenRouter (hosted ML/AI), Weave CLI (optional)

### System Diagram

```
User (browser) → [Next.js (3000)] → REST → [Go HTTP API (8080)] → [SQLite DB]
                                              ↓           ↑
                                   Python ML subprocess  (JSON, stdin/stdout)
                                              ↓
                                    [GitHub API] ↔ [Background Worker]
                                              ↓
                                    [OpenRouter API] (optional)
                                              ↓
                                    [Weave CLI] (optional)
```

### Docker Compose Profiles

- **local-ml**: All ML/AI is local (sentence-transformers, HDBSCAN)
- **openrouter-light**: Hosted embeddings/reasoning via OpenRouter; minimal local ML dependencies

---

## Data Model

SQLite schema designed for scale and fast lookups:

### Entities

**PR**
- id: uuid (PK)
- number: int
- repo: text
- title: text
- body: text
- state: enum ('open', 'closed', 'merged')
- mergeable: enum ('mergeable', 'conflicting', 'unknown')
- base_branch: text
- head_branch: text
- author: text
- is_bot: bool
- is_draft: bool
- additions: int
- deletions: int
- changed_files: int
- ci_status: enum ('passing', 'failing', 'pending', 'unknown')
- review_decision: enum ('approved', 'changes_requested', 'review_required', 'none')
- created_at: timestamptz
- updated_at: timestamptz
- merged_at: timestamptz (nullable)
- cluster_id: text (nullable, FK→PRCluster.id)
- staleness_score: float (nullable)
- synced_at: timestamptz

**PRFile**
- id: uuid (PK)
- pr_id: uuid (FK→PR.id)
- filename: text
- status: enum ('added', 'modified', 'deleted', 'renamed')
- additions: int
- deletions: int

**PRCluster**
- id: uuid (PK)
- repo: text
- label: text
- centroid_embedding: blob (nullable)
- pr_count: int
- created_at: timestamptz

**SyncJob**
- id: uuid (PK)
- repo: text
- status: enum ('pending', 'running', 'completed', 'failed')
- cursor: text (nullable)
- current: int
- total: int (nullable)
- rate_limit_remaining: int
- rate_limit_reset_at: timestamptz (nullable)
- started_at: timestamptz
- completed_at: timestamptz (nullable)
- error: text (nullable)

**SchemaMigration**
- version: int (PK)
- name: text
- applied_at: timestamptz

**AuditLog**
- id: uuid (PK)
- action_type: text
- pr_number: int
- repo: text
- intent: text
- dry_run: bool
- created_at: timestamptz

### Relationships
- **PR → PRFile**: One-to-many
- **PR → PRCluster**: Many-to-one (via `cluster_id`)

### Indexes
- PR: (repo, state), (repo, base_branch), (repo, updated_at), (cluster_id)
- PRFile: (pr_id), (filename)
- SyncJob: (repo, status)

---

## API Design

All routes served from `pratc serve` (Go net/http) on port 8080. No authentication (localhost only). CORS restricts origins to localhost:3000.

### Routes

**Health**
```
GET /healthz
  Response: { status: "ok", version: string }

GET /api/health
  Response: { status: "healthy" }
```

**Repositories & Analysis**
```
GET /api/repos/:owner/:repo/analysis
  Response: { repo, generatedAt, counts, clusters, duplicates, overlaps, conflicts, stalenessSignals }
```

**Clusters**
```
GET /api/repos/:owner/:repo/clusters
  Response: { repo, generatedAt, model, thresholds, clusters[] }
```

**Graph**
```
GET /api/repos/:owner/:repo/graph
  Response: {
    nodes: [ { id, pr_number, cluster_id, lines_changed } ],
    edges: [ { source, target, type: 'dependency'|'conflict'|'duplicate', files: [] } ]
  }
```

**Plans**
```
GET /api/repos/:owner/:repo/plans?target=N&top=5&mode=combination|permutation|with_replacement&require_ci=bool&exclude_stale=bool&exclude_drafts=bool
  Response: {
    repo, generatedAt, target, candidatePoolSize, strategy,
    plans: [
      { rank, score, prs: [ { number, title, position, reason } ], warnings: [] },
      ...
    ]
  }
```

**PRs**
```
GET /api/repos/:owner/:repo/prs?page=N&per_page=100&cluster_id=X&base_branch=X
  Response: { total_count, prs: [...] }

GET /api/repos/:owner/:repo/prs/:number
  Response: { ...pr fields, files: [...], cluster: {...} }
```

**Sync**
```
POST /api/repos/:owner/:repo/sync
  Body: {}
  Response: { job_id, status, repo }

GET /api/repos/:owner/:repo/sync/status
  Response: { syncing: bool, job_id, progress: { current, total, eta_seconds }, rate_limit: { remaining, reset_at } }

GET /api/repos/:owner/:repo/sync/stream
  Response: Server-Sent Events, streaming progress updates
```

**Actions**
```
POST /api/repos/:owner/:repo/actions
  Body: { pr_number: int, action_type: "approve"|"close"|"skip", dry_run: bool }
  Response: { status: string, action_logged: bool }
```

**Error Format**
```json
{ "error": "string", "code": "string" }
```

---

## Auth & Permissions

- **Provider**: None (v0.1) — single-user, localhost only
- **GitHub PAT**: Handled by `psst` (secret manager) or `GITHUB_PAT` env var. Never stored in SQLite/config files.
- **All routes**: Public, CORS-restricted to localhost:3000.
- **Repo scoping**: All queries namespaced by `owner/repo`. No cross-repo data leaks.

---

## Third-Party Services

| Service | Purpose | Mode |
|---------|---------|------|
| GitHub GraphQL/REST API | Primary PR data source | Required |
| Python ML (sentence-transformers + HDBSCAN) | Local PR clustering & deduplication | local-ml profile |
| OpenRouter API | Hosted embeddings + GPT reasoning | openrouter-light profile |
| Weave CLI | Semantic conflict detection (tree-sitter) | Optional |

---

## Key Technical Decisions

- **Weave resolves 31/31 merge conflicts** vs Git’s 15/31 (tree-sitter AST-aware semantic merging)
- **HDBSCAN** chosen over k-means: no predefined cluster count, handles noise/outliers well at 5,500+ PR scale
- **SQLite WAL mode**: enables concurrent reads during background sync without blocking the API
- **Subprocess JSON bridge** (not HTTP): lower latency, no port management, simpler process lifecycle
- **Combinatorial modes**: combination (independent PRs), permutation (ordered dependency chains), with_replacement
