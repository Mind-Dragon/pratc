# Multi-Repo Data Model Design for prATC v1.8

## Merged Recommendations

Summary: The additive v8 direction is right, but the v1.8 design needs stronger internal identity, migration, authorization, and cross-repo evidence semantics before implementation.

Must-fix before v1.8:
- Make the repository registry authoritative for new schema work, not just informational. Keep canonical `owner/name` strings for external compatibility, but add internal repository-ID references for new cross-repo and plan tables so referential integrity improves without rewriting legacy tables.
- Rewrite the migration section to match the real SQLite migration path in `internal/cache/sqlite.go`: idempotent DDL, Go-side backfill, conservative state derivation, and `schema_migrations` / `PRAGMA user_version` advancement only after successful completion. Explicitly require fresh DB, v6→v8, v7→v8, and restart-after-partial-migration tests.
- Use conservative sync-state backfill rules. Do not infer `ready` from incomplete historical evidence; default ambiguous repos to `unknown` or `degraded` until a successful modern sync establishes trustworthy state.
- Expand dependency modeling beyond repo-to-repo edges so the stated repo-A depends on artifact last changed in repo-B case is actually representable. Add normalized PR/artifact-level evidence and derived artifact lineage/ownership so the system can explain which artifact or contract created the dependency.
- Define authorization and privacy rules for all new cross-repo storage. Reading or creating cross-repo edges and multi-repo plans should require explicit repo-set authorization, and evidence payloads should be redacted or summarized when a caller lacks access to every participating repo.
- Specify multi-repo response contracts now, not later. Keeping single-repo contracts stable is good, but v1.8 also needs explicit `MultiRepoAnalysisResponse`, `MultiRepoGraphResponse`, and planning payload shapes so roadmap-level aggregate analysis is fully covered.

Nice-to-have after must-fix items:
- Broaden dependency heuristics beyond shared files/import paths/API overlap to include manifest version shifts, generated artifacts, submodules/subtrees, CI/workflow coupling, deployment/config ordering, and database rollout dependencies.
- Treat multi-repo plan records as first-class audit artifacts with stated retention, deletion, and visibility rules.
- Consider whether a single `app_install_id` on `repositories` is enough long term; it is acceptable for v1.8 if documented as a minimum model, but likely insufficient for richer installation history or multi-owner scenarios.

Counter-points / where to be careful:
- The audit is right that text-keyed new tables weaken authority, but a full repo-ID rewrite of legacy tables is still not warranted for v1.8. The better compromise is dual-layer modeling: legacy string keys remain, new tables use IDs where practical.
- The current heuristic approach remains appropriate. The gap is not that heuristics are wrong, but that the evidence model is too narrow to support the most important dependency cases safely and explainably.

## Audit (GPT 5.4)

### Strengths (2026-04-23)
- The design is directionally sound as an additive v8: keeping existing `repo TEXT`-keyed tables intact while adding a `repositories` registry fits prATC's current forward-only SQLite posture and avoids a risky repo-id rewrite in one release.
- It correctly recognizes that the real compatibility constraint is not just storage but response contracts. Calling for new multi-repo response types instead of overloading existing single-repo `AnalysisResponse`, `GraphResponse`, and `PlanResponse` aligns with the current `internal/types/models.go` shape.
- The document is right to treat cross-repo dependency detection as heuristic and explainable rather than pretending to produce semantic truth. Storing evidence and confidence is a good auditability choice.
- The privacy intent is mostly correct at a high level: raw PR/review/file/CI rows remain repo-scoped, while cross-repo artifacts are derived and minimal.
- The design aligns with `ROADMAP.md` at the headline level. v1.8 calls for multi-repo support, cross-repo dependency detection, and unified merge planning; this doc addresses all three directly.

### Weaknesses / Gaps (2026-04-23)
- The schema stops halfway between string-keyed and identity-keyed modeling. `repositories` gets an integer `id`, but new tables still use `source_repo TEXT`, `target_repo TEXT`, and `repo TEXT` rather than `repository_id` references. That preserves compatibility, but it means the registry is not actually authoritative and referential integrity remains weak.
- The migration story is optimistic relative to the current implementation in `internal/cache/sqlite.go`. The existing migration path is a single inline init routine that applies idempotent DDL plus conditional Go-side fixups. A realistic v8 must account for that exact pattern, including ordering, partial-failure behavior, and tests for fresh, N-1, and N-2 upgrades. As written, the doc does not spell out transaction boundaries or how `schema_migrations` / `PRAGMA user_version` are only advanced after backfill succeeds.
- Backfilling `repositories.sync_status` and `last_sync` from `sync_jobs`, `sync_progress`, `merged_pr_index`, and `audit_log` is plausible but lossy. The proposed inference could silently label repos `ready` even when cached data is stale, partial, or inherited from an interrupted older sync.
- Cross-repo detection is not sufficient yet if limited to shared files and import-path references. The doc adds API surface overlap, which helps, but it still misses several important cases: branch/ref pinning, package-version changes in manifests, generated-artifact lineage, Git submodules/subtrees, CI/workflow dependencies, deployment/config coupling, database/schema rollout ordering, and shared external resources where file paths differ entirely.
- The design does not fully handle the explicit case: a PR in repo A depends on a file last modified in repo B. The current proposal can detect concurrent/shared changed paths or namespace references, but it does not model cross-repo file lineage, default-branch provenance, or “last modified in repo B” lookups. `cross_repo_dependencies` is repo-to-repo, not PR-to-PR or file-to-file. There is no table that records which repo currently owns or most recently changed a shared artifact across the repo set.
- Privacy boundaries are underspecified operationally. `evidence_json` may still expose sensitive cross-repo file paths, import strings, schema names, or API symbols to callers authorized for one repo but not the other unless authorization is enforced at query time and evidence is redacted or filtered.
- Multi-repo plan storage is only partially isolated. `multi_repo_plan_runs.repo_set_json` and `multi_repo_plan_items` are useful, but the doc does not define access-control semantics, retention, or whether a user authorized for one repo can enumerate plan metadata that reveals another repo's participation.
- The doc says the `repositories` table prepares for GitHub App integration, but a single `app_install_id` on the repo row may be too narrow for future multi-owner / installation history / permission-scoped modeling.
- Alignment to the roadmap is good but incomplete. `ROADMAP.md` says “aggregate analysis across multiple repositories”; this doc is strongest on storage and planning, but weaker on how analysis/graph surfaces aggregate while preserving the existing single-repo contracts.

### Recommendations (2026-04-23)
- Keep the additive v8 direction, but explicitly define two layers: external canonical repo strings for compatibility and internal repository IDs for new tables. At minimum, add nullable `source_repository_id` / `target_repository_id` or use repo IDs directly in new tables while preserving repo strings in legacy tables.
- Make the migration plan match the real SQLite migration mechanism in `internal/cache/sqlite.go`: idempotent DDL, Go-side backfill, and version advancement only after successful completion. Call out required upgrade tests for fresh DB, v6→v8, and v7→v8, plus a restart-after-partial-migration test.
- Do not infer `ready` loosely during backfill. Prefer conservative states such as `unknown` or `degraded` when historical evidence is incomplete. Otherwise the registry will look more trustworthy than the source data actually is.
- Extend cross-repo dependency modeling beyond repo-to-repo edges. Add an optional normalized evidence layer for PR-level and artifact-level linkage, e.g. `cross_repo_dependency_items` keyed by dependency edge plus repo, PR number, artifact path, symbol, or manifest reference.
- Add explicit support for artifact lineage / ownership so the repo-A-on-repo-B-last-modified-file case is covered. A workable design would persist derived artifact fingerprints and last-seen owners from merged/default-branch state, then attach PR-level evidence showing that repo A changed or consumed an artifact whose latest authoritative upstream fingerprint came from repo B. Without that, the asked-for scenario is only weakly inferred or missed entirely.
- Tighten privacy semantics: require explicit repo-set authorization for creating or reading multi-repo edges/plans; define whether evidence is fully visible, summarized, or redacted per repo; and ensure cross-repo tables cannot be queried as an implicit wildcard side channel.
- Clarify how aggregate analysis and graph outputs map to the roadmap. If single-repo response types remain unchanged, define parallel `MultiRepoAnalysisResponse` and `MultiRepoGraphResponse` contracts now, not just the planning response.
- Treat `multi_repo_plan_runs` and `multi_repo_plan_items` as first-class audit artifacts: define retention, deletion behavior, and repo-scoped visibility rules so plan metadata itself does not become a privacy leak.

Status: DO NOT IMPLEMENT — design only
Target release: v1.8

## Purpose

prATC is currently modeled as a single logical repository per analysis/sync/planning run, even though many cache tables already carry a `repo` column. v1.8 extends that model into a first-class multi-repo system that can ingest, store, analyze, and plan across a repository set while preserving per-repo isolation.

This document proposes the data model, storage changes, migration path, and output contract needed to support:

- multiple repositories under one prATC instance
- cross-repo dependency detection
- unified merge planning across related repositories
- privacy boundaries that keep each repository's raw data separate

The current roadmap explicitly calls out multi-repo support, cross-repo dependency detection, and unified merge planning in v1.8. The current SQLite store is forward-only and already uses repo-scoped keys such as `(repo, number)` for `pull_requests`, `pr_files`, `pr_reviews`, `ci_status`, and `merged_pr_index`. That is a useful starting point, but it is not yet a complete multi-repo model.

## Current state summary

Grounded in the current codebase:

- `internal/cache/sqlite.go` uses schema version 7 and forward-only migrations.
- Existing core cache tables are repo-scoped by string key, not by repository identity row.
- `types.PR` stores `Repo string` and all primary response types (`AnalysisResponse`, `PlanResponse`, `GraphResponse`, etc.) assume a single `repo` string in the top-level payload.
- `MergePlanCandidate` and `ConflictPair` do not currently carry explicit repository identity on each item; they assume all items belong to the same repo.

This means the current system is multi-tenant in storage shape but still single-repo in product contract.

## Design goals

1. Make repositories first-class entities in the cache.
2. Keep existing per-repo tables valid and queryable during transition.
3. Support analysis of repository sets without collapsing all raw data into a shared namespace.
4. Detect cross-repo dependencies using explainable heuristics.
5. Produce a unified merge plan with per-repo sections and cross-repo conflict warnings.
6. Preserve forward-only SQLite migration discipline.
7. Keep repository privacy boundaries explicit.

## Proposed repository identity model

### New `repositories` table

Add a first-class repository registry table:

```sql
CREATE TABLE repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    last_sync TEXT NOT NULL DEFAULT '',
    sync_status TEXT NOT NULL DEFAULT 'never_synced',
    webhook_enabled INTEGER NOT NULL DEFAULT 0,
    app_install_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(owner, name)
);
```

Required requested fields:

- `id`
- `owner`
- `name`
- `last_sync`
- `sync_status`
- `webhook_enabled`
- `app_install_id`

Recommended supporting fields:

- `created_at`
- `updated_at`

Rationale:

- `owner` + `name` remain human-readable and stable for CLI/API use.
- `id` gives the database a compact internal key for joins and future expansion.
- `last_sync` and `sync_status` move repository-level operational state out of ad hoc inference.
- `webhook_enabled` and `app_install_id` prepare the model for GitHub App integration.

### Canonical repository string

The canonical external repository key remains `owner/name`, normalized to lowercase consistent with the existing v7 repo normalization migration.

Rules:

- storage canonical form: lowercase `owner/name`
- registry uniqueness: `UNIQUE(owner, name)` after normalization
- API/CLI presentation: preserve canonical lowercase form unless future UI wants display casing

## Relationship to current tables

Existing repo-scoped tables should remain in place for v1.8:

- `pull_requests`
- `pr_files`
- `pr_reviews`
- `ci_status`
- `sync_jobs`
- `sync_progress`
- `merged_pr_index`
- `duplicate_groups`
- `conflict_cache`
- `substance_cache`
- `audit_log`

These tables already partition data by `repo`. The new design does not require replacing that partitioning. Instead, the `repositories` table becomes the authoritative registry and operational metadata table.

## Proposed schema diff

### Current schema characteristics

Current cache schema version: 7

Current relevant characteristics:

- no `repositories` table
- top-level repository identity is stored only as repeated `repo TEXT` values across tables
- sync status is inferred from `sync_jobs` / `sync_progress`
- no first-class cross-repo dependency storage
- no first-class multi-repo plan artifact storage

### Proposed additions

#### 1) `repositories`

```sql
CREATE TABLE repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    last_sync TEXT NOT NULL DEFAULT '',
    sync_status TEXT NOT NULL DEFAULT 'never_synced',
    webhook_enabled INTEGER NOT NULL DEFAULT 0,
    app_install_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(owner, name)
);
CREATE INDEX idx_repositories_owner_name ON repositories(owner, name);
CREATE INDEX idx_repositories_sync_status ON repositories(sync_status);
```

#### 2) `cross_repo_dependencies`

This table stores explainable dependency/conflict edges discovered between repositories.

```sql
CREATE TABLE cross_repo_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_repo TEXT NOT NULL,
    target_repo TEXT NOT NULL,
    dependency_type TEXT NOT NULL,
    signal_strength REAL NOT NULL,
    evidence_json TEXT NOT NULL,
    detected_at TEXT NOT NULL,
    corpus_fingerprint TEXT NOT NULL DEFAULT '',
    UNIQUE(source_repo, target_repo, dependency_type, corpus_fingerprint)
);
CREATE INDEX idx_cross_repo_dep_source ON cross_repo_dependencies(source_repo);
CREATE INDEX idx_cross_repo_dep_target ON cross_repo_dependencies(target_repo);
```

`dependency_type` values:

- `shared_files`
- `import_path_reference`
- `api_surface_overlap`

`evidence_json` is an auditable JSON payload containing the concrete paths, import references, or symbol overlaps that justified the edge.

#### 3) `multi_repo_plan_runs`

Optional but recommended plan metadata table for auditable multi-repo planning.

```sql
CREATE TABLE multi_repo_plan_runs (
    id TEXT PRIMARY KEY,
    repo_set_json TEXT NOT NULL,
    target INTEGER NOT NULL,
    strategy TEXT NOT NULL,
    generated_at TEXT NOT NULL,
    warnings_json TEXT NOT NULL,
    conflicts_json TEXT NOT NULL
);
```

#### 4) `multi_repo_plan_items`

Optional but recommended normalized storage for selected items in a multi-repo plan.

```sql
CREATE TABLE multi_repo_plan_items (
    plan_id TEXT NOT NULL,
    repo TEXT NOT NULL,
    pr_number INTEGER NOT NULL,
    section_order INTEGER NOT NULL,
    item_order INTEGER NOT NULL,
    score REAL NOT NULL,
    rationale TEXT NOT NULL,
    PRIMARY KEY (plan_id, repo, pr_number)
);
CREATE INDEX idx_multi_repo_plan_items_repo ON multi_repo_plan_items(repo, item_order);
```

### Current vs proposed summary

Current:

- repeated repo strings in each table
- no repo registry
- no cross-repo edge storage
- single-repo plan output types

Proposed:

- add first-class repository registry
- keep existing repo-scoped tables unchanged for compatibility
- add explicit cross-repo edge table
- add optional multi-repo plan artifact tables
- extend response models to identify repo at item level

## Data model behavior

### Repository registry lifecycle

1. A repository is registered when the operator adds it explicitly or a sync is first requested.
2. A canonical `owner/name` entry is inserted into `repositories`.
3. `sync_status` transitions through states such as:
   - `never_synced`
   - `queued`
   - `syncing`
   - `ready`
   - `degraded`
   - `error`
4. `last_sync` is updated only after a successful completed sync.
5. `webhook_enabled` and `app_install_id` are updated by GitHub App installation management.

### Repository state authority

Repository-level operational truth should move to `repositories`, while per-run execution detail remains in `sync_jobs` and `sync_progress`.

- `repositories.sync_status` = latest coarse repository state
- `sync_jobs` = run history
- `sync_progress` = resumable cursor/checkpoint state

This avoids expensive joins just to ask “which repositories are ready?”

## Cross-repo dependency detection design

v1.8 requires cross-repo dependency detection across related repositories. Detection should remain heuristic, explainable, and auditable rather than pretending to be perfect semantic truth.

### Detection signals

#### 1) Shared files

Use when repositories mirror or vendor common assets or generated outputs.

Detection rule:

- compare normalized changed file paths across PRs in different repos
- focus on high-signal paths: OpenAPI specs, protobufs, shared schemas, SDK-generated clients, migrations, lockfiles, config contracts
- suppress common noise: docs, changelogs, generated build outputs unless configured otherwise

Evidence example:

```json
{
  "shared_paths": ["api/openapi.yaml", "proto/payment/v1/service.proto"],
  "match_count": 2,
  "suppressed_noise_matches": 5
}
```

Interpretation:

- likely coordinated change or mirrored artifact dependency

#### 2) Import paths

Use when one repository references packages, modules, or artifacts owned by another repository.

Detection rule:

- derive known import/module prefixes for each repository
- scan changed files and dependency manifests for references to another repo's import namespace
- examples: Go modules, npm package names, Python package imports, Terraform module sources, Git submodule references

Evidence example:

```json
{
  "import_references": [
    "github.com/acme/platform-contracts/api/v1",
    "@acme/contracts"
  ],
  "files": ["internal/client/client.go", "package.json"]
}
```

Interpretation:

- source repo likely depends on target repo’s API or package surface

#### 3) API surface overlap

Use when two repositories expose or consume the same external contract.

Detection rule:

- derive API signatures from changed artifacts: route definitions, protobuf services, GraphQL schema fields, OpenAPI paths, exported interface names
- compute overlap score across repositories
- flag only when overlap exceeds threshold and change types suggest coordination or conflict

Evidence example:

```json
{
  "overlapping_symbols": ["POST /v1/payments", "PaymentService.CreatePayment"],
  "source_change_type": "producer",
  "target_change_type": "consumer",
  "overlap_score": 0.83
}
```

Interpretation:

- likely producer/consumer coupling, version skew risk, or coordinated rollout requirement

### Scoring and confidence

Each detected edge should have:

- `dependency_type`
- `signal_strength` from 0.0 to 1.0
- structured evidence payload

Suggested weighting:

- shared files: medium confidence unless file class is schema/API critical
- import path references: high confidence for explicit module/package references
- API surface overlap: high confidence when symbol overlap and producer/consumer roles align

### Cross-repo conflict detection

Cross-repo conflict is not the same as code merge conflict. It means coordinated merge risk.

Flag when any of the following hold:

- two repos modify the same shared artifact lineage
- producer repo changes API surface while consumer repo still targets previous contract
- rollout ordering constraints exist (library first, consumer second)
- generated client/server repos show mismatched schema fingerprints

Conflict output should explain:

- affected repos
- affected PRs
- conflict type
- ordering recommendation
- confidence/evidence

## Unified merge plan output design

Current `PlanResponse` assumes one repo. v1.8 needs a multi-repo plan contract while keeping the existing single-repo response intact for backward compatibility.

### New multi-repo response shape

Proposed new response type conceptually:

```json
{
  "repo_set": ["acme/contracts", "acme/service", "acme/web"],
  "generatedAt": "2027-01-15T12:34:56Z",
  "target": 6,
  "strategy": "multi_repo_dependency_aware",
  "sections": [
    {
      "repo": "acme/contracts",
      "selected": [...],
      "ordering": [...],
      "rejections": [...],
      "warnings": [...]
    },
    {
      "repo": "acme/service",
      "selected": [...],
      "ordering": [...],
      "rejections": [...],
      "warnings": [...]
    }
  ],
  "cross_repo_conflicts": [
    {
      "source_repo": "acme/contracts",
      "source_pr": 101,
      "target_repo": "acme/service",
      "target_pr": 88,
      "conflict_type": "api_surface_overlap",
      "severity": "high",
      "recommended_order": [
        "acme/contracts#101",
        "acme/service#88"
      ],
      "reason": "consumer PR depends on producer API changes"
    }
  ],
  "warnings": []
}
```

### Design requirements

1. Per-repo sections remain self-contained.
2. Every selected PR item must be repo-qualified.
3. Cross-repo conflicts are emitted in a dedicated top-level section.
4. Ordering can express inter-repo sequencing constraints.
5. Single-repo `PlanResponse` remains available unchanged.

### Recommended type evolution

To support multi-repo planning without ambiguity:

- add `Repo string` to `MergePlanCandidate`
- add `SourceRepo` and `TargetRepo` to any multi-repo conflict payload
- introduce `MultiRepoPlanResponse` instead of overloading `PlanResponse`

This avoids breaking existing clients that assume a top-level `repo` field refers to all items.

## Privacy model

The v1.8 privacy requirement is: each repo's data stays separate.

### Storage boundaries

Raw repository data stays partitioned by repo:

- PR metadata remains stored with its originating `repo`
- file lists, reviews, CI, and merged history remain repo-scoped
- no raw cross-repo denormalized super-table should be introduced

### Cross-repo artifacts

Cross-repo artifacts may exist, but they must store only derived linkage metadata:

- dependency edge type
- confidence/signal strength
- evidence references
- plan ordering constraints

They should not duplicate entire PR bodies or unrelated repository data.

### Access boundaries

For future service/API enforcement, multi-repo operations should require explicit repository-set authorization.

Principles:

- single-repo operations only see one repo’s raw rows
- multi-repo operations are explicit, never implicit wildcard queries
- repository membership in a plan/analyze run is operator-selected or app-installation-scoped

### Operational interpretation

“Data stays separate” does not forbid cross-repo reasoning. It means:

- source-of-truth rows stay repo-local
- cross-repo outputs are derived, minimal, and auditable
- access to cross-repo views must be intentional

## Migration strategy

The cache layer already follows forward-only SQLite migration rules. v1.8 should continue that pattern.

### Migration approach

Add a new schema version, for example v8, with inline idempotent statements in `internal/cache/sqlite.go`.

Recommended v8 migration steps:

1. bump `supportedSchemaVersion` from 7 to 8
2. `CREATE TABLE IF NOT EXISTS repositories ...`
3. backfill `repositories` from distinct repo values found in existing tables
4. `CREATE TABLE IF NOT EXISTS cross_repo_dependencies ...`
5. optionally create `multi_repo_plan_runs` and `multi_repo_plan_items`
6. insert migration record into `schema_migrations`
7. set `PRAGMA user_version = 8`

### Backfill logic

Backfill sources:

- `pull_requests.repo`
- `sync_jobs.repo`
- `sync_progress.repo`
- `merged_pr_index.repo`
- `audit_log.repo` where non-empty

Backfill algorithm:

- gather distinct normalized `owner/name` values
- split repo string into `owner` and `name`
- insert rows with:
  - `last_sync` from best available source (`sync_progress.last_sync_at` preferred)
  - `sync_status` inferred from latest sync job/progress state, otherwise `ready` if historical data exists, else `never_synced`
  - `webhook_enabled = 0`
  - `app_install_id = NULL`
  - `created_at` and `updated_at` = migration execution time

### Why forward-only is sufficient

This change is additive. Existing tables and keys remain valid. The system can move forward without destructive transforms.

### Roll-forward safety

If migration partially succeeds, the next startup must be able to re-run idempotently.

Requirements:

- use `CREATE TABLE IF NOT EXISTS`
- use `INSERT OR IGNORE` / upsert for backfill
- never rewrite historical PR rows solely to attach numeric repo IDs in v8
- avoid introducing foreign keys that would block startup on incomplete backfill

## Compatibility strategy

### Cache compatibility

Preserve all existing `repo TEXT` columns in current tables for at least the full v1.8 cycle.

Why:

- low-risk migration
- minimal change to existing query paths
- preserves older tooling assumptions

### Type compatibility

Keep current single-repo responses unchanged.

Add new multi-repo-specific response types rather than changing:

- `AnalysisResponse`
- `PlanResponse`
- `GraphResponse`

This is safer than trying to overload a single `repo` field into an array.

## Non-goals

- no cross-repo auto-merge
- no assumption that all repos live in one monorepo or one package ecosystem
- no hard requirement for perfect semantic dependency inference
- no destructive replacement of existing repo-keyed tables with repo_id foreign keys in v1.8

## Recommended implementation sequencing

1. Add repository registry and migration.
2. Add repo set selection primitives in CLI/API.
3. Add cross-repo dependency detection pipeline and cache table.
4. Add multi-repo plan response types.
5. Add unified report rendering for multi-repo plans.
6. Optionally normalize more tables to `repo_id` in a later release once v8 is stable.

## Open questions

1. Should repository sets be ad hoc per request, or stored as named groups?
2. Should `app_install_id` live only on `repositories`, or also on a future installations table for multi-owner support?
3. Should cross-repo dependency evidence include raw snippets, or only references and fingerprints?
4. Should multi-repo planning target a global PR count or per-repo quotas?

## Recommendation

Adopt an additive v8 design:

- add a first-class `repositories` table
- keep existing repo-scoped tables intact
- add a derived `cross_repo_dependencies` table
- add a dedicated multi-repo plan response contract with per-repo sections and top-level cross-repo conflicts
- preserve repository privacy by keeping raw data repo-scoped and cross-repo artifacts minimal and derived

That design fits the current SQLite migration discipline, leverages the existing repo-keyed schema, and creates a clean foundation for GitHub App integration and the ML feedback work that follows.
