# prATC v1.4.2.x Active Operations Backlog

## Goal

Keep the production triage service honest: every open PR is eventually captured, rate-limit pauses resume from the last checkpoint, and the operator can tell what is happening without guessing.

## Source of truth

- `ROADMAP.md` defines the milestone sequence and release-line intent.
- `GUIDELINE.md` defines the allowed bucket vocabulary and non-negotiables.
- `ARCHITECTURE.md` defines the system shape and data flow.
- `version1.4.2.md` defines the shipped v1.4.2 operating model.
- `version1.4.2.md` defines the managed-service follow-on for resumable sync and explicit states.

## Already shipped

- Full-corpus analysis no longer depends on a hidden `maxPRs=1000` default.
- Cache listing supports caller-visible paging/streaming access.
- The 16-layer decision ladder exists in code, tests, and report output.
- Review payloads, operator buckets, risk buckets, and priority tiers are wired through API, web, and PDF surfaces.
- The rate-limit budget manager and resumable sync state exist for long-running runs.
- Persistent project run artifacts live under `projects/<repo>/runs/<timestamp>/`.

These are foundations. They do not belong in the active backlog.

## Remaining work

### 0. Efficiency pass (COMPLETED)

The local-first sync workflow is now shipped:
- the workflow reuses a completed local snapshot after the first sync unless `--refresh-sync` is set
- downstream phases consume the SQLite snapshot and run artifacts instead of re-downloading the same corpus
- initial syncs can be capped with `--sync-max-prs` so the first pass only fetches the requested PR ceiling
- the sync path stores and honors the captured snapshot ceiling so resumed work stays bounded

### 1. Managed-service hardening

**Goal:** make background runs restartable and observable enough to survive session crashes.

**Work items:**
- Keep the workflow/service path explicit about current sync state and resume metadata.
- Confirm sync resumes cleanly after rate-limit pauses and other transient interruptions.
- Keep health, status, and run-manifest output aligned so a background run can be supervised without opening the process manually.
- Preserve the last checkpoint when the session dies and the worker restarts.

### 2. Corpus-coverage regression guardrails

**Goal:** keep the corpus path honest on large repositories.

**Work items:**
- Keep a 6,000+ PR smoke/benchmark in place.
- Keep corpus caps explicit and configurable only where they are intentionally part of the contract.
- Guard against hidden truncation reappearing in CLI or planning paths.

### 3. Doc synchronization

**Goal:** keep the milestone docs aligned with the live codebase.

**Work items:**
- Keep `README.md`, `ROADMAP.md`, `version1.4.2.md`, and `CHANGELOG.md` aligned with the current release line.
- Keep the active backlog free of shipped v1.4.2 items.
- Move any new implementation debt into the appropriate minor-release or ops doc instead of re-opening v1.4.2.

## Done when

- No hidden cap silently shrinks the corpus.
- A large-repo run either captures every open PR or reports the exact blocking reason.
- Rate-limit pauses resume from the last checkpoint.
- The service can be supervised as a managed background process without losing visibility.
