# prATC — App Architecture Plan

## System Overview

prATC is an AI-powered, self-hostable, and repo-agnostic platform designed to manage extremely large repositories with 5,500+ open pull requests. Targeted at solo maintainers, it applies combinatorial optimization—as in air traffic control—to PR triage, deduplication, and merge planning. The system is a polyglot monorepo with three major components: the `pratc` binary (Go CLI and optional HTTP API), a Python-based ML service invoked via subprocess and JSON, and a Next.js-powered web dashboard. The architecture is a monolith-style CLI that can optionally expose an HTTP API (`pratc serve`).

### Architecture Pattern

* **Type**: Monolith (polyglot, CLI-first) with optional web API and ML subprocesses

* **Key components**:

  * **Frontend**: Next.js 14+ web dashboard (port 3000)

  * **Backend API**: Go-based HTTP API (port 8080), part of `pratc serve`

  * **Database**: SQLite (file-based, WAL mode)

  * **Background Jobs**: In-process background sync worker for PR metadata

  * **External Services**: GitHub GraphQL/REST, Python ML subprocess (local) or OpenRouter (hosted ML/AI), Weave CLI (optional)

### System Diagram

* **Next.js Web Dashboard** (\[localhost:3000\])

  * Sends RESTful JSON requests to the **Go HTTP API** (\[localhost:8080\]) via browser fetch/XHR.

* **Go HTTP API** (`pratc serve`)

  * Provides all REST endpoints.

  * Reads/writes to **SQLite** cache on disk.

  * On-demand or background calls out to:

    * **Python ML service** (`pratc-ml`): Communicates via subprocess, JSON over stdin/stdout (not HTTP).

    * **GitHub GraphQL/REST API**: Sync worker fetches PR data incrementally, applies backoff/rate-limit logic, persists progress.

    * **OpenRouter API**: (Optional) For hosted embeddings and GPT-powered reasoning.

    * **Weave CLI**: (Optional) Invoked via shell for semantic conflict detection.

* **Docker Compose profiles**:

  * **local-ml**: All ML/AI is local (sentence-transformers, HDBSCAN).

  * **openrouter-light**: Hosted embeddings/reasoning via OpenRouter; minimal local ML dependencies.

---

```plain
User (browser) → \[Next.js (3000)\] → REST → \[Go HTTP API (8080)\] → \[SQLite DB\]
                                               ↓           ↑
                                    Python ML subprocess  (JSON, stdin/stdout)
                                               ↓
                                     \[GitHub API\] ↔ \[Background Worker\]
                                               ↓
                                     \[OpenRouter API\] (optional)
                                               ↓
                                     \[Weave CLI\] (optional)
```

---

## Data Model

SQLite schema is designed for scale and fast lookups:

### Entities

**PR**

* id: uuid (PK)

* number: int

* repo: text

* title: text

* body: text

* state: enum ('open', 'closed', 'merged')

* mergeable: enum ('mergeable', 'conflicting', 'unknown')

* base_branch: text

* head_branch: text

* author: text

* is_bot: bool

* is_draft: bool

* additions: int

* deletions: int

* changed_files: int

* ci_status: enum ('passing', 'failing', 'pending', 'unknown')

* review_decision: enum ('approved', 'changes_requested', 'review_required', 'none')

* created_at: timestamptz

* updated_at: timestamptz

* merged_at: timestamptz (nullable)

* cluster_id: text (nullable, FK→PRCluster.id)

* staleness_score: float (nullable)

* synced_at: timestamptz

**PRFile**

* id: uuid (PK)

* pr_id: uuid (FK→PR.id)

* filename: text

* status: enum ('added', 'modified', 'deleted', 'renamed')

* additions: int

* deletions: int

**PRCluster**

* id: uuid (PK)

* repo: text

* label: text

* centroid_embedding: blob (nullable)

* pr_count: int

* created_at: timestamptz

**SyncJob**

* id: uuid (PK)

* repo: text

* status: enum ('pending', 'running', 'completed', 'failed')

* cursor: text (nullable)

* current: int

* total: int (nullable)

* rate_limit_remaining: int

* rate_limit_reset_at: timestamptz (nullable)

* started_at: timestamptz

* completed_at: timestamptz (nullable)

* error: text (nullable)

**SchemaMigration**

* version: int (PK)

* name: text

* applied_at: timestamptz

**AuditLog**

* id: uuid (PK)

* action_type: text

* pr_number: int

* repo: text

* intent: text

* dry_run: bool

* created_at: timestamptz

### Relationships

* **PR → PRFile**: One-to-many

* **PR → PRCluster**: Many-to-one (via `cluster_id`)

### Indexes and Constraints

* PR: (repo, state), (repo, base_branch), (repo, updated_at), (cluster_id)

* PRFile: (pr_id), (filename)

* SyncJob: (repo, status)

* All tables have PK indices, with appropriate FKs.

* schema_migrations maintains a forward-only migration log. `PRAGMA user_version` must always match the latest migration.

## API Design

All routes are served from `pratc serve` (Go net/http) on port 8080. No authentication (localhost only). CORS restricts origins to localhost:3000.

### Routes

**Health**

```
GET /healthz
  Auth: public
  Response: { status: "ok", version: string }

```

`GET /api/health`  

`Auth: public`  

`Response: { status: "healthy" }`  

**Repositories & Analysis**

```
GET /api/repos/:owner/:repo/analysis
  Auth: public
  Response: { repo, generatedAt, counts, clusters, duplicates, overlaps, conflicts, stalenessSignals }
```

**Clusters**

```
GET /api/repos/:owner/:repo/clusters
  Auth: public
  Response: { repo, generatedAt, model, thresholds, clusters\[\] }
```

**Graph**

```
GET /api/repos/:owner/:repo/graph
  Auth: public
  Response: {
    nodes: \[ { id, pr_number, cluster_id, lines_changed } \],
    edges: \[ { source, target, type: 'dependency'|'conflict'|'duplicate', files: \[\] } \]
  }
```

**Plans**

```
GET /api/repos/:owner/:repo/plans?target=N&top=5&mode=combination|permutation|with_replacement&require_ci=bool&exclude_stale=bool&exclude_drafts=bool
  Auth: public
  Response: {
    repo, generatedAt, target, candidatePoolSize, strategy,
    plans: \[
      {
        rank, score,
        prs: \[ { number, title, position, reason } \],
        warnings: \[\]
      }, ...
    \]
  }
```

**PRs**

```
GET /api/repos/:owner/:repo/prs?page=N&per_page=100&cluster_id=X&base_branch=X
  Auth: public
  Response: { total_count, prs: \[...\] }   // paginated

```

`GET /api/repos/:owner/:repo/prs/:number`  

`Auth: public`  

`Response: { ...pr fields, files: [...], cluster: {...} }`  

**Sync**

```
POST /api/repos/:owner/:repo/sync
  Auth: public
  Body: { }
  Response: { job_id, status, repo }

```

`GET /api/repos/:owner/:repo/sync/status`  

`Auth: public`  

`Response: {`  

`syncing: bool, job_id, progress: { current, total, eta_seconds },`  

`rate_limit: { remaining, reset_at }`  

`}`

`GET /api/repos/:owner/:repo/sync/stream`  

`Auth: public`  

`Response: Server-Sent Events, streaming progress updates`  

**Actions**

```
POST /api/repos/:owner/:repo/actions
  Auth: public
  Body: { pr_number: int, action_type: "approve"|"close"|"skip", dry_run: bool }
  Response: { status: string, action_logged: bool }
```

**Error Format**All endpoints:

```json
{ "error": "string", "code": "string" }
```

**Rate Limiting:** None on localhost.  

**Pagination:** `page` and `per_page` query parameters; responses include `total_count`.  

**Exit Codes:** `2` for invalid args, `1` for runtime failure.

## Auth & Permissions

### Authentication

* **Provider**: None (v0.1)

* **User Management**: Single-user, localhost only.

* **GitHub PAT** handled by `psst` (secret manager) or `GITHUB_PAT` env var. Never stored in SQLite/config files; only passed at runtime.

* **Session Handling**: None.

### Authorization

* **Roles**: None (single maintainer)

* **Enforcement**: All routes are public, accessible from localhost:3000 only. CORS headers enforced globally by middleware.

* **Public Routes**: All.

### Multi-tenancy

* **Not applicable**; fully single-user/single-host.

* **Repo scoping**: All queries and API paths are namespaced with `owner/repo`. No cross-repo data leaks—strictly enforced at the query layer.

## Third-Party Services

### GitHub GraphQL/REST API

* **Purpose**: Primary source for PR metadata, files, reviews, and CI state.

* **SDK/Library**: Custom Go client (`internal/github/`) using stdlib net/http and raw GraphQL queries.

* **Config**: `GITHUB_PAT` env var, managed via `psst`.

* **Failure Handling**:  

  * Exponential backoff if `X-RateLimit-Remaining` < 10.

  * Progress for sync jobs persisted, so interrupted jobs can resume.

  * Fallback to REST endpoints for bulk fetch as a backup.

### Python ML Service (Local Mode)

* **Purpose**: Embedding via `sentence-transformers`, clustering with HDBSCAN, similarity with scikit-learn.

* **SDK/Library**: Subprocess, JSON over stdin/stdout (no HTTP or gRPC).

* **Config**: `ML_BACKEND=local`, `HF_HOME=/app/models` for model/cache volume.

* **Failure Handling**: If service fails, return degraded analysis—fallback to file-overlap clustering only, disabling NLP features.

### OpenRouter (Hosted Mode, Optional)

* **Purpose**: Hosted embeddings, GPT-5.4 for explanations.

* **SDK/Library**: HTTP client (Go).

* **Config**: `ML_BACKEND=openrouter`, `OPENROUTER_API_KEY`, `OPENROUTER_EMBED_MODEL`, `OPENROUTER_REASON_MODEL`.

* **Failure Handling**: Fail closed, log provider/model/timeout/retry/fallback. Ensure timeouts; never hang. Document fallback versus fast-failure logic.

### Weave CLI (Optional Dependency)

* **Purpose**: Tree-sitter-based semantic conflict detection (not merge resolution).

* **SDK/Library**: None; invoked via shell command.

* **Config**: Optional; if missing, system degrades to file-overlap heuristics.

## Frontend Architecture

### Tech Choices

* **Framework**: Next.js v14+ (App Router)

* **Language**: TypeScript (strict mode enabled)

* **Component Library**: shadcn/ui + Radix Primitives

* **Styling**: Tailwind CSS

* **State Management**: TanStack Query (for server state & polling sync), Zustand (for local component/UI state)

* **Specialized Libs**:  

  * **TanStack Table**: Infinite/virtualized table for PR list (5,000+ PRs)

  * **D3.js**: Force-directed graph in `/graph` (SVG for <500 nodes, Canvas for >500)

* **Testing**: Vitest + React Testing Library

* **E2E**: Playwright

### Page Structure

* `/` → Redirects to `/analysis`

* `/analysis` → Air traffic control panel (cluster cards, staleness heatmap, duplicates list)

* `/inbox` → Outlook-style 3-pane: Left = PR list (TanStack Table, 6 cols), Right = detail pane, Actions = Approve/Close/Skip

* `/graph` → Interactive D3.js graph: PR nodes (by size/cluster), edges (dependency, conflict, duplicate). Zoom, pan, tooltips, cluster filter.

* `/plan` → Merge plan UI: Config/formula settings (N, mode, constraints), timeline with conflict warnings, combinatorics formula stats, Markdown export

### Data Fetching Strategy

* **All pages**: Client components using fetch to Go API via `/web/src/lib/api.ts` typed wrapper (`NEXT_PUBLIC_API_URL`)

* **TanStack Query**: Polling every 10s during active sync; handles error/loading state globally (via isLoading/isError)

* **No SSR**: No server-side fetching—ensures UI is always reflecting the most recent local state

## Infrastructure & Deployment

### Hosting

* **Frontend**: Localhost:3000 using `bun run dev` (dev), or Docker Compose `pratc-web` service (prod)

* **Backend**: Localhost:8080 via `pratc serve` (dev), or Docker Compose `pratc-cli`

* **Database**: SQLite file at `./data/pratc.db`, volume-mounted in Docker Compose

### Docker Compose Setup

* `pratc-cli`:  

  * Built with `Dockerfile.cli` (Go multi-stage build + Python 3.11 runtime + UV sync for ML deps)

  * Runs `pratc serve` on 8080

  * Health check: `curl -f http://localhost:8080/api/health`

  * Volumes:  

    * `./data` → `/app/data` (SQLite DB)

    * `./models` → `/app/models` (Hugging Face model cache, local-ml only)

  * Env: `PRATC_DB_PATH=/app/data/pratc.db`, `HF_HOME=/app/models`

* `pratc-web`:  

  * Built with `Dockerfile.web` (Node 20 slim, bun install, next build/start)

  * Runs on 3000

  * Health check: `curl -f http://localhost:3000`

  * Env: `NEXT_PUBLIC_API_URL=http://pratc-cli:8080`

  * Depends on `pratc-cli`

* **Profiles**:  

  * **local-ml**: ML runs fully local w/model volume

  * **openrouter-light**: ML offloaded to OpenRouter—no model download, requires relevant API keys

### CI/CD Pipeline

* **No CI/CD in v0.1**: Purposefully omitted (no GitHub Actions implemented yet)

### Environments

* **Development**: Local machine, `make dev` brings up all services, seed DB from fixtures

* **Staging**: Not applicable

* **Production**: Docker Compose self-hosted deployment

* **Makefile Targets**:

  * `deps`, `verify-env`, `dev`, `build`, `test`, `test-go`, `test-python`, `test-web`, `lint`, `docker-build`, `docker-up`, `docker-down`, `docker-logs`, `clean`

### Environment Variables

* **Feature Flags**: None in v0.1

* **Secrets Handling**: Never commit GITHUB_PAT or OPENROUTER_API_KEY—managed via psst or .env (gitignored)

---

**This plan provides a fully actionable scaffold for prATC's initial v0.1 feature set, with explicit attention to system constraints, developer experience, and data and API surfaces defined for high-scale, solo-maintainer PR management.**