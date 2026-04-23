# ML Feedback Loop Design for prATC v1.8

## Quick Review

This doc does not yet include an external audit section, but the design is directionally solid and ready for a deeper review focused on implementation risk and privacy rigor.

Needs deeper audit attention:
- Schema and lifecycle validation: confirm the proposed `operator_decisions` minimum table is sufficient for append-only auditability, replay protection, and future expansion without forcing a breaking migration.
- Capture semantics: verify exactly where operator actions enter the system, how duplicate submissions or automated actions are excluded, and whether decision typing is precise enough for bucket overrides, rejections, duplicate corrections, and plan overrides.
- ML boundary design: review whether the proposed local-only versus externalized fields are strict enough, especially around free-text reasons, file paths, stable sample IDs, and repo-derived feature leakage.
- Training-loop safety: confirm that v1.8 should remain batch/retrain-oriented rather than online weight mutation, and define what immediate local effects are allowed without making model behavior unstable or opaque.
- Contract alignment: verify how a new `feedback` action would fit the current Go↔Python bridge contracts, batching/export semantics, audit logging, and failure handling when the ML side is unavailable.

Status: DO NOT IMPLEMENT — design only
Target release: v1.8

## Purpose

prATC currently has an ML bridge (`internal/ml/bridge.go`) that delegates clustering, duplicate detection, and optional analysis to a Python subprocess over JSON stdin/stdout. The roadmap for v1.8 adds an ML feedback loop so operator decisions become training signals.

This document defines how operator overrides and rejections should be captured, stored, validated, and converted into ML signals while preserving privacy boundaries.

## Current state summary

Grounded in current code:

- `internal/review/orchestrator.go` is the judgment entry point for analyzer aggregation and review categorization.
- The orchestrator produces a final `ReviewResult` derived from deterministic logic plus analyzer findings.
- `internal/ml/bridge.go` currently supports actions `cluster`, `duplicates`, and `analyze` through subprocess JSON IPC.
- There is no current feedback capture path, no operator decision table, and no ML-specific persistence for overrides.

So v1.8 needs to add feedback capture around the existing decision pipeline rather than replacing the review orchestrator.

## Design goals

1. Capture operator overrides as explicit, structured feedback.
2. Store that feedback durably in SQLite with full auditability.
3. Distinguish local/private metadata from shareable ML signals.
4. Feed validated feedback into duplicate detection and scoring improvement loops.
5. Keep the online path safe: operators stay in control, ML learns asynchronously.

## What counts as feedback

The roadmap requirement specifically calls out operator bucket changes and rejections.

For v1.8, treat the following as first-class feedback events:

- bucket/category override
  - example: `review_required` changed to `merge_now`
- explicit rejection of a recommendation
  - example: operator rejects prATC’s nominated canonical duplicate
- duplicate/overlap correction
  - example: operator says two PRs are not duplicates, or marks previously separate PRs as duplicates
- dependency/merge-plan override
  - example: operator changes merge order due to domain knowledge

The required minimum is bucket changes and rejections, but the storage format should leave room for more decision types.

## Feedback capture points

### Primary capture point: post-judgment override path

`internal/review/orchestrator.go` remains the system’s judgment producer. Feedback should be captured after the system has produced a recommendation and a human/operator has taken an explicit action to change or reject it.

This means feedback does not originate inside the analyzer logic itself. It originates in an operator action layer that compares:

- system recommendation
- operator final decision

### Capture scenarios

#### 1) Review bucket/category override

System output:

- category/bucket
- confidence
- reasons
- analyzer findings

Operator action:

- changes bucket/category
- optionally provides reason

Captured signal:

- old bucket
- new bucket
- decision type `bucket_override`
- rationale text

#### 2) Recommendation rejection

System output:

- merge plan candidate
- duplicate canonical selection
- dependency ordering recommendation

Operator action:

- rejects recommendation without necessarily selecting a replacement

Captured signal:

- decision type `rejection`
- target artifact type
- old state recorded, new state may be empty or special value such as `rejected`

#### 3) Duplicate/overlap correction

System output:

- duplicate group or overlap group

Operator action:

- confirms, rejects, or reassigns grouping

Captured signal:

- decision type such as `duplicate_override`
- pair/group identifiers
- previous classification vs operator classification

## SQLite feedback format

The user requested this table design:

- `operator_decisions: id, repo, pr_number, decision_type, old_bucket, new_bucket, reason, operator_id, timestamp`

That should be the v1.8 minimum table.

### Proposed table

```sql
CREATE TABLE operator_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo TEXT NOT NULL,
    pr_number INTEGER NOT NULL,
    decision_type TEXT NOT NULL,
    old_bucket TEXT NOT NULL DEFAULT '',
    new_bucket TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    operator_id TEXT NOT NULL,
    timestamp TEXT NOT NULL
);
CREATE INDEX idx_operator_decisions_repo_pr ON operator_decisions(repo, pr_number, timestamp DESC);
CREATE INDEX idx_operator_decisions_type ON operator_decisions(decision_type, timestamp DESC);
CREATE INDEX idx_operator_decisions_operator ON operator_decisions(operator_id, timestamp DESC);
```

### Column semantics

- `id`: stable local primary key
- `repo`: canonical `owner/name` repository key
- `pr_number`: PR identifier within that repo
- `decision_type`: kind of override or rejection
- `old_bucket`: original prATC bucket/classification, if applicable
- `new_bucket`: operator-selected replacement bucket/classification, if applicable
- `reason`: optional human explanation
- `operator_id`: local operator identifier
- `timestamp`: RFC3339 time of decision capture

### Allowed `decision_type` values

Minimum initial enum:

- `bucket_override`
- `rejection`
- `duplicate_override`
- `plan_override`

The required v1.8 implementation can start with the first two.

### Why keep the minimum table narrow

The request explicitly wants a small, auditable SQLite design. Keeping it narrow has benefits:

- easy to migrate
- easy to inspect manually
- clear privacy boundaries
- enough information to start supervised learning and error analysis

### Recommended optional extension

If richer feedback is needed later, add a sidecar table rather than bloating the core table:

```sql
CREATE TABLE operator_decision_artifacts (
    decision_id INTEGER NOT NULL,
    artifact_type TEXT NOT NULL,
    artifact_json TEXT NOT NULL,
    PRIMARY KEY (decision_id, artifact_type)
);
```

This allows extra structured context for duplicate or plan corrections without changing the minimum v1.8 table contract.

## Feedback lifecycle

The user requested this lifecycle: capture → validate → store → signal ML service.

### 1) Capture

Trigger when an operator explicitly changes or rejects a prATC recommendation.

Captured fields should include:

- repo
- pr_number
- decision_type
- original system bucket or recommendation
- operator final bucket or outcome
- free-text reason if provided
- operator identity
- timestamp

Note: if the UI/API does not currently require a reason, v1.8 should treat `reason` as optional but highly recommended.

### 2) Validate

Before storing the event, validate:

- `repo` exists and is authorized
- `pr_number` exists in cache for that repo
- `decision_type` is known
- `old_bucket` matches one of the allowed review categories when bucket-based
- `new_bucket` matches one of the allowed review categories when bucket-based
- `operator_id` is present and authenticated
- duplicate submission protection if the exact same operator action is replayed rapidly

Validation should also ensure the event is operator-authored, not inferred from background automation.

### 3) Store

Write to `operator_decisions` within a single transaction.

Properties:

- append-only
- no in-place mutation of historical feedback rows
- if correction is needed, insert a new decision row rather than rewriting history

Why append-only:

- preserves auditability
- enables time-based retraining windows
- makes operator behavior explainable over time

### 4) Signal ML service

After durable local storage, emit a derived ML feedback payload.

Two signaling modes:

- synchronous best-effort enqueue of a local event
- asynchronous batching and export to ML service

Recommended default:

- store locally first
- then asynchronously batch/export to ML service
- never block the operator action on remote ML availability

## How feedback improves scoring and duplicate detection

The request asks how feedback should feed into scoring/duplicate detection, specifically retrain signal vs online learning.

### Recommendation: retrain signal first, online learning later

For v1.8, use feedback primarily as a retraining signal, not as direct online model mutation.

Why:

- safer and easier to audit
- avoids unstable behavior from immediate weight changes
- fits current subprocess ML architecture, which is batch-oriented JSON IPC
- lets operators trust that overrides do not instantly perturb all recommendations

### Retrain-signal path

Use stored operator decisions to build labeled datasets for periodic retraining or threshold tuning.

Examples:

#### Bucket overrides -> scoring calibration

If operators frequently change:

- `review_required` -> `merge_now`

then likely causes include:

- risk scoring too conservative
- analyzer confidence penalties too strong
- certain subsystem heuristics overweighted

Retraining/tuning output could adjust:

- confidence calibration
- per-team thresholding
- feature weights used for priority/bucket assignment

#### Rejections of duplicate recommendations -> duplicate model tuning

If operators reject duplicate groups often, retraining can improve:

- similarity thresholds
- candidate generation behavior
- importance of file overlap vs title/body semantics
- subsystem-aware duplicate suppression

### Online learning in v1.8

Avoid direct online model updates in the request path.

What is acceptable in v1.8:

- online counters
- team-local threshold adaptation suggestions
- drift dashboards

What is not recommended for v1.8:

- mutating model weights immediately after each operator decision
- changing duplicate thresholds silently during a live review session

### Suggested loop split

1. local immediate effects
   - show operator-adjusted outcome in UI/API
   - update analytics counters
   - maybe recompute same PR recommendation if needed

2. asynchronous learning effects
   - batch export decisions
   - retrain or re-score offline
   - deploy new model/threshold version explicitly

## Privacy boundaries

The user explicitly requested this split:

Local only:

- `operator_id`
- repo names
- PR numbers

Sent externally:

- feature vectors
- decision diffs

This should be the hard design boundary.

### Data that stays local

Never send to external ML service by default:

- `operator_id`
- canonical repository names
- PR numbers
- raw PR URLs
- raw auth/session identifiers
- free-form private reasons unless explicitly scrubbed/allowed

### Data that may be sent externally

Send only derived ML payloads such as:

- feature vectors representing the PR/recommendation state
- original predicted class/bucket encoded as model label
- operator-corrected class/bucket encoded as target label
- decision diff metadata such as `predicted=review_required`, `actual=merge_now`
- duplicate similarity feature vectors and corrected labels

### Scrubbing rules

Before external signaling:

- replace repo with anonymized stable cohort or omit entirely
- replace PR number with local ephemeral sample ID
- strip free-text reason unless transformed into safe structured tags
- remove raw file paths unless hashed/featurized and policy allows it

### Why this boundary fits prATC

prATC is self-hostable and repo-agnostic. Operators may run it on private codebases. The feedback loop must therefore preserve local operational identity and repository identity while still enabling ML improvement via derived features.

## Suggested feedback payload to ML service

Current ML bridge accepts actions `cluster`, `duplicates`, and `analyze`. v1.8 should add a new conceptual action:

- `feedback`

Example payload shape:

```json
{
  "action": "feedback",
  "samples": [
    {
      "sample_id": "local-12345",
      "decision_type": "bucket_override",
      "predicted_bucket": "review_required",
      "actual_bucket": "merge_now",
      "feature_vector": {
        "substance_score": 71,
        "ci_green": 1,
        "files_changed_count": 3,
        "duplicate_cluster_size": 0,
        "analyzer_confidence_mean": 0.64
      },
      "decision_diff": {
        "changed": true,
        "magnitude": "one_step_less_severe"
      },
      "timestamp": "2027-01-15T12:34:56Z"
    }
  ]
}
```

Notably absent:

- repo name
- PR number
- operator_id

## Local feature extraction strategy

Feature extraction should happen locally before signaling.

Potential local features:

- review category predicted by orchestrator
- confidence
- analyzer disagreement count
- substance score
- changed files count
- additions/deletions magnitude
- bot/draft flags
- duplicate/overlap scores
- conflict count
- CI state encoded numerically
- temporal bucket / staleness score

These features already largely exist in current review/analyze data structures or can be derived from them.

## Interaction with the current ML bridge

Current bridge characteristics:

- JSON-over-stdin/stdout
- subprocess timeout
- batch action model

This suggests the simplest v1.8 addition is a new batched feedback action rather than a streaming online learner.

Recommended bridge evolution:

- add a `feedback` action to the Python CLI
- accept batches of derived feature/decision samples
- return acknowledgement plus any optional calibration suggestions
- keep it best-effort; local persistence remains the source of truth

## Audit and observability

Feedback systems are risky without visibility. Minimum observability should include:

- count of captured operator decisions by type
- percentage of recommendations overridden
- top bucket transitions
- duplicate recommendation rejection rate
- age of oldest unsent feedback batch
- last successful ML feedback export time

This is important both for model quality and to detect operator frustration with prATC recommendations.

## Failure handling

### If local store fails

- reject the override API action or mark it incomplete
- do not pretend the feedback was captured

### If ML export fails

- keep local decision row
- retry asynchronously
- surface export lag in telemetry
- never discard operator history due to remote ML outage

### If payload validation fails

- do not send externally
- store local audit row only if operator action itself was valid
- emit diagnostic log or admin-visible error

## Migration strategy

Add a forward-only SQLite migration, for example v9 if v8 is used by the multi-repo work.

Migration steps:

1. create `operator_decisions`
2. add indexes
3. optionally create `operator_decision_artifacts`
4. insert new `schema_migrations` row
5. bump `PRAGMA user_version`

This is additive and does not require rewriting existing PR or review rows.

## Non-goals

- no fully automated online weight updates in the critical request path
- no sending operator identity or repo identity to external ML by default
- no replacing the review orchestrator with an opaque ML-only classifier
- no hidden policy changes based on feedback without explicit deployment/versioning

## Recommended implementation sequencing

1. Add operator decision capture API/path after recommendation output.
2. Add SQLite migration for `operator_decisions`.
3. Add validation and append-only persistence.
4. Add local feature extraction for feedback samples.
5. Add asynchronous `feedback` action to ML bridge.
6. Add retraining/calibration job based on accumulated decisions.
7. Consider online threshold adaptation only after auditability is proven.

## Open questions

1. Should `reason` remain free text only, or also support structured reason codes?
2. Should duplicate/plan overrides share the same minimum table, or require the optional artifact sidecar immediately?
3. What minimum number of decisions is needed before retraining is allowed?
4. Should teams be able to opt out of external ML signaling entirely while keeping local feedback analytics?

## Recommendation

Use an append-only local `operator_decisions` table as the source of truth, capture overrides only after explicit operator action, validate then store locally first, and send only derived feature vectors plus decision diffs to the ML service. Treat feedback as a retraining/calibration signal in v1.8, not as immediate online model mutation.
