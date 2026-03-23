# AGENTS.md — internal/cmd

HTTP server and API routes for prATC.

## Route Map

| Method | Route | Handler |
|--------|-------|---------|
| GET | /healthz | Health check (also /api/health) |
| GET | /api/settings?repo= | Get settings |
| POST | /api/settings | Set: {scope, repo, key, value} |
| DELETE | /api/settings?scope=&repo=&key= | Delete setting |
| GET | /api/settings/export?scope=&repo= | Export YAML |
| POST | /api/settings/import | Import YAML (1MB limit) |
| GET | /api/repos/{o}/{n}/analyze | Analyze PRs |
| GET | /api/repos/{o}/{n}/cluster | Cluster PRs |
| GET | /api/repos/{o}/{n}/graph | Graph (?format=dot) |
| GET | /api/repos/{o}/{n}/plan | Plan (?target=N&mode=...) |
| POST | /api/repos/{o}/{n}/sync | Trigger sync (202) |
| GET | /api/repos/{o}/{n}/sync/stream | SSE sync events |

## Legacy Routes

Backward-compatible query-string style:
- GET /analyze?repo= /cluster?repo= /graph?repo=&format= /plan?repo=&target=

## Patterns

**Response helpers:**
- `writeHTTPJSON(w, status, payload)` — JSON with Content-Type
- `writeHTTPError(w, status, msg)` — `{"error": "..."}`

**Request guards:**
- `ensureGET(w, r)` / `ensureRepo(w, repo)` — Return bool, write status on fail
- `parseRepoActionPath(path)` — Parse `/api/repos/{owner}/{name}/{action}`

**Plan query params:**
- target: int >0, default 20
- mode: combination|permutation|with_replacement
- dry_run: bool, default true (safe)
- candidate_pool_cap: int 1..500, default 100
- score_min: float 0..100, stale_score_threshold: float 0..1

## Settings API

**Interface:** `settingsStore`
- Get(ctx, repo) → map[string]any
- Set/Delete/ValidateSet(ctx, scope, repo, key, value) error
- ExportYAML/ImportYAML(ctx, scope, repo, []byte) → []byte/error

**Scope:** `global` or `repo`. POST ?validateOnly=true validates without writing.

## Gotchas

1. **Route mismatch:** API uses `/api/settings?repo=` but web expects `/api/repos/{o}/{n}/settings/{scope}/{key}`. Align before v0.1.

2. **CORS hardcoded:** Allows `localhost:3000` only. Needs production config.

3. **Audit DB per call:** `logAuditEntry()` opens/closes SQLite each time. Pool if throughput increases.

4. **Plan dry_run:** Absent param = true. Must pass `dry_run=false` to disable.

5. **Import limit:** `http.MaxBytesReader(w, r.Body, 1<<20)` — 1MB max.

6. **Sync SSE nil check:** `repoSyncAPI` nil returns 500.

## Environment

- `PRATC_SETTINGS_DB` — Settings SQLite (default: ./pratc-settings.db)
- `PRATC_DB_PATH` — Audit SQLite (default: ~/.pratc/pratc.db)
