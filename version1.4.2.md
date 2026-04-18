# prATC v1.4.2 — Local-First Sync With Explicit Sync Ceilings

This is the shipped v1.4.2 operating model. It keeps the workflow honest about what was already synced, what still needs GitHub, and when the repo can stay entirely local.

If this file ever conflicts with `GUIDELINE.md` on bucket names, layer order, or non-negotiables, `GUIDELINE.md` wins.

## What v1.4.2 means

- Every open PR in the repo is accounted for.
- The system syncs once, then reuses the local snapshot for downstream phases.
- The workflow can skip a fresh GitHub sync when a completed snapshot already exists.
- A user-defined sync ceiling can cap how many PRs are fetched on the initial pass.
- The system peels away noise in layers instead of flattening everything into one score.
- Every decision carries a reason trail.
- The system distinguishes now, future, and blocked work.
- No PR disappears silently.
- No auto-merge, auto-approve, or write-back behavior is allowed.
- The report reads like a decision map, not a flat list.

## Locked vocabulary

Temporal buckets:
- now
- future
- blocked

Quality buckets:
- high_value
- merge_candidate
- needs_review
- re_engage
- low_value

Disposal buckets:
- duplicate
- junk
- stale

Risk buckets:
- security_risk
- reliability_risk
- performance_risk

Managed-service states:
- queued
- running
- paused_rate_limit
- resuming
- completed
- failed
- canceled

The bucket system is intentionally larger than the old v1.3 review categories. Historical v1.3 names may still appear in `ROADMAP.md` as Phase 0 context, but the active v1.4.2 operating model is the one above.

## What is already in place

- Planning integration is wired through the main app path.
- The review output sorts queue work by the current priority model.
- Cache listing supports caller-visible paged/streaming access.
- Bootstrap sync streams directly into the cache store.
- Review payloads, bucket counts, and decision trails are present in the API, dashboard, and report surfaces.
- The docs have a dedicated milestone summary and a single architecture document.
- The workflow can resume from SQLite instead of replaying the whole corpus.
- The sync path records a snapshot ceiling so paused or capped runs resume from the right boundary.
- The workflow can reuse a completed local snapshot instead of redownloading the same repo again.

These are foundation pieces. They are not the remaining finish line.

## Current working emphasis

The production flow is now managed-service oriented:

- sync once
- preserve the checkpoint locally
- resume paused jobs from SQLite
- keep explicit sync states visible to operators
- make rate-limit pauses restartable
- keep background runs supervision-friendly
- prefer cached/local data for analyze, cluster, graph, and plan after the first sync
- only hit GitHub again when the user explicitly asks for a refresh or the local snapshot is missing

That is the path for keeping a crashed session from losing track of an in-flight corpus run.

## Relationship to other docs

- `ROADMAP.md` says what v1.4.2 is and the order of work.
- `GUIDELINE.md` says what the system is allowed to do.
- `ARCHITECTURE.md` says how the system is shaped.
- `TODO.md` says what is still left to build.

This file exists so the milestone can be read quickly without losing the underlying contracts.
