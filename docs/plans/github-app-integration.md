# GitHub App Integration Design for prATC v1.8

Status: historical/auth design input. v2.0 action mutation policy is governed by `../../VERSION2.0.md`, `../../GUIDELINE.md`, and the central executor model.

## Merged Recommendations

Summary: The GitHub App direction is correct, but the design should separate operator login from repository access, tighten webhook/job ordering semantics, and make transition and uninstall behavior explicit before implementation.

Must-fix before v1.8:
- Split the auth model into two explicit flows. Operator onboarding/login may use device flow or another OAuth bootstrap, but repository access must come only from GitHub App installation tokens. The doc should clearly name the exact GitHub auth primitive used for operator bootstrap and state that it does not grant repository access by itself.
- Harden the backend-mediated device flow. Bind each in-flight login to a server-side session/nonce, enforce expiry and single-use completion, rate-limit polling, support revocation/logout, and ensure the CLI stores only a prATC session reference rather than provider tokens.
- Add webhook ordering, stale-event, and supersession rules. Jobs should be keyed by installation + repo + PR + head SHA (or equivalent generation), obsolete jobs should be ignored/cancelled, and only the latest known head SHA should publish final checks/status.
- Expand webhook operational semantics to include `ping`, out-of-order delivery handling, and explicit invalidation behavior for base-branch retargeting, uninstall, suspension, and repository removal.
- Define exact status/check mappings for both Checks API and Commit Status fallback. `neutral` and `skipped` are valid for Checks but not Commit Status, so fallback behavior must be explicitly mapped and documented.
- Make uninstall cleanup explicit: invalidate cached installation tokens, stop queued/running jobs for that installation, tombstone or clear repo-install mappings, disable future status posting, and define whether historical repo/PR data is retained for audit or purged for retention compliance.
- Clarify transition policy relative to the roadmap. PAT/`gh` should remain compatibility-only for existing operators, off by default for new setups, and documented as a temporary bridge rather than equal long-term auth.
- Reconcile advisory behavior with merge blocking. prATC can remain advisory as a product while still publishing failing checks that branch protection may enforce when an operator opts in.

Nice-to-have after must-fix items:
- Add a dedicated `github_app_installations` table once installation metadata, history, or per-installation state becomes important.
- Document migration and deprecation criteria for phasing out PAT/`gh` mode once GitHub App onboarding is stable.
- Separate lightweight CI/review-triggered recomputation from heavier corpus refresh paths more explicitly in the event matrix.

Counter-points / where to be careful:
- The audit is right that the current device-flow wording is too ambiguous, but keeping device flow as a CLI-friendly bootstrap can still be reasonable if the doc clearly demotes it to operator identity only.
- The existing backward-compatibility goal is still worth keeping. The issue is not that PAT fallback exists, but that the doc should describe it as transitional so it stays aligned with the roadmap.

## Audit (z.ai 5.1)

### Strengths — 2026-04-23
- The doc correctly pivots repository access toward GitHub App installation tokens, keeps the app private key server-side, and recommends short-lived token minting instead of persistent PAT-style credentials.
- The webhook receiver is sensibly split into a fast authenticated ingress path plus async processing, with HMAC verification, delivery ID tracking, and idempotent handling called out.
- The event list covers the core triggers needed for PR-centric recomputation: `pull_request`, review changes, CI/check changes, `push`, `installation`, and `installation_repositories`.
- The design preserves backward compatibility at a high level by explicitly keeping the existing `gh`/PAT path available for current operators while making GitHub App mode the preferred default.
- It broadly matches `ROADMAP.md` on the three headline items: OAuth-based onboarding, webhook-triggered analysis, and status/check reporting.

### Weaknesses / Gaps — 2026-04-23
- The OAuth device-flow description is the biggest design risk. GitHub App installation access is not obtained via device flow; device flow only establishes a user/operator identity. As written, the doc blurs OAuth App/user auth with GitHub App auth and does not specify whether prATC is using GitHub's OAuth App device flow, a separate OAuth broker, or GitHub App user-to-server authorization. That ambiguity matters for implementation and permissions.
- The backend-proxied device flow needs tighter security requirements. The design does not say how `device_code` polling is bound to the initiating CLI session, how poll abuse is rate-limited, how operator sessions are expired/revoked, or how CSRF/session fixation-style mixups are prevented when multiple logins are in flight.
- Webhook coverage is good but incomplete operationally. The doc does not mention `ping` handling, explicit out-of-order delivery handling, or stale-event suppression by installation/repo/PR/SHA version. `pull_request.edited` mentions title/body/base changes, but there is no explicit rule for base-branch retargeting invalidating prior analysis and status state.
- Uninstall/suspension handling is underspecified. `installation.deleted` only says to mark repositories disabled and stop work. It does not explicitly require purging any cached installation tokens, cancelling queued jobs for that installation, tombstoning or clearing repo-install mappings, removing webhook trust state, or defining what cached PR/repo data is retained vs deleted after uninstall.
- Status-state handling is not fully correct across both APIs. GitHub Commit Status supports `error`, `failure`, `pending`, and `success`; it does not support `neutral` or `skipped`. The doc proposes neutral/skipped states without defining how they map when Commit Status API is used as fallback.
- The status/check design is race-prone. Multiple webhook deliveries for the same PR can enqueue overlapping work, and the doc does not define supersession rules, per-SHA generation numbers, or how to prevent an older analysis from posting after a newer one. This is especially important for `synchronize`, CI status churn, and rapid review updates.
- Backward compatibility is conceptually present, but the transition plan is still thin. The doc says PAT users can continue unchanged, yet `ROADMAP.md` says "OAuth-based authentication (no PAT management)." That is reconcilable only if PAT/`gh` is clearly documented as temporary compatibility mode rather than ongoing first-class auth management.
- There is also a policy tension with the roadmap. `ROADMAP.md` says status checks should "block merge on high-risk findings," while the doc emphasizes advisory-only behavior and configurable failure/neutral mapping. The design needs a clearer statement that prATC itself remains advisory, but can publish failing checks that branch protection may enforce if an operator opts in.

### Recommendations — 2026-04-23
- Split the auth model into two explicit flows: (1) operator login/onboarding via a clearly identified OAuth mechanism, and (2) repository access strictly via GitHub App installation tokens. If GitHub device flow is retained, document it as user bootstrap only and name the exact GitHub auth primitive being used.
- Add concrete security controls for device flow: bind each device code to a server-side nonce/session, enforce expiry and single-use completion, rate-limit polling, audit approvals, support logout/session revocation, and never expose provider tokens back to the CLI when a prATC session reference will do.
- Expand webhook semantics to include `ping`, stale-delivery detection, and ordering/supersession rules. Process jobs keyed by installation + repo + PR + head SHA, cancel or ignore obsolete jobs, and only publish final status/check results for the latest known head SHA or generation.
- Define exact status/check state mapping for both APIs. For Checks API use `queued`/`in_progress`/`completed` with conclusions such as `success`, `failure`, `neutral`, `skipped`, or `cancelled`; for Commit Status fallback map unsupported neutral/skipped cases to an agreed safe value, likely `success` with explanatory text or `failure` only when policy explicitly gates merges.
- Make uninstall behavior explicit: on `installation.deleted`, immediately invalidate in-memory installation-token caches, cancel queued/running jobs for that installation, clear or tombstone `repositories.app_install_id`, disable future status posting, and define whether historical cache rows are retained for audit or purged for least-retention compliance.
- Clarify transition policy: PAT/`gh` fallback should remain readable as compatibility-only, off by default for new setups, and excluded from webhook/status features unless separately configured. Add migration criteria and a deprecation path so the design still aligns with the roadmap's direction away from PAT management.
- Reconcile the roadmap wording directly in this doc: note that prATC remains advisory as a product behavior, while v1.8 can still support merge blocking indirectly by publishing failing checks that repository branch protection rules enforce.

Status: DO NOT IMPLEMENT — design only
Target release: v1.8

## Purpose

prATC currently relies on PAT-style token resolution through the `gh` CLI and runtime token discovery logic in `internal/github/auth.go`. The v1.8 roadmap calls for:

- OAuth-based authentication instead of PAT management
- webhook-triggered analysis
- status check integration for PR feedback

This document defines the target design for moving prATC from PAT-only auth to a GitHub App + OAuth device flow model while preserving backward compatibility.

## Current state summary

Grounded in current code:

- `internal/github/auth.go` discovers accounts via `gh auth status` and resolves access tokens via `gh auth token`.
- `ResolveTokenForLogin` assumes the active gh account provides the token.
- Current auth model is runtime token passthrough, not installation-based auth.
- The project roadmap explicitly says GitHub App/OAuth/webhooks are a v1.8 feature, not current behavior.

So the current design is CLI-centric and user-token-centric. v1.8 must add app-centric and installation-centric auth without breaking existing operators.

## Design goals

1. Replace PAT-first workflows with GitHub App + OAuth device authorization for CLI onboarding.
2. Support repository installation and per-repo installation ownership.
3. Accept webhook events and trigger the minimum required recomputation.
4. Post commit status / check results back to pull requests.
5. Preserve PAT/gh-token fallback for existing deployments.
6. Minimize secret exposure and overbroad scopes.

## Authentication model overview

v1.8 introduces three related identities:

1. Operator identity
   - established by GitHub OAuth device flow in the CLI
   - used for login/session/bootstrap and installation selection

2. GitHub App identity
   - the prATC application registered in GitHub
   - used to mint installation access tokens

3. Installation identity
   - the per-org/per-user installation of the GitHub App
   - used for repository-scoped API access and webhook trust

Core principle:

- operators authenticate to prATC with OAuth device flow
- prATC accesses repository data through GitHub App installation tokens
- PAT remains an optional compatibility fallback, not the preferred path

## GitHub App registration and installation flow

### App registration

Operator/admin creates a GitHub App with:

- app name: `prATC` or deployment-specific variant
- callback URL for any web-based admin flow if needed later
- webhook URL: `https://<pratc-host>/webhooks/github`
- webhook secret
- permissions limited to the minimum set required
- event subscriptions for PR and repository activity

### Required repository permissions

Recommended minimum permissions:

- Pull requests: Read
- Contents: Read
- Metadata: Read
- Commit statuses: Write
- Checks: Write (optional but recommended over statuses for richer output)
- Issues: Read, if comments/labels are later used as signals
- Administration: none unless future install management requires it

### Installation flow

1. Operator runs CLI setup command, for example `pratc auth login` or similar future command.
2. CLI starts device code flow and authenticates the operator.
3. prATC backend exchanges the device code and stores an operator session token.
4. Backend lists installations visible to that operator for the registered GitHub App.
5. Operator selects one or more installations/orgs.
6. prATC records installation metadata and associates repositories to installation IDs.
7. Sync jobs and webhook processing use short-lived installation tokens minted on demand.

### Repository-to-installation mapping

Use the new multi-repo registry from the data-model design:

- `repositories.app_install_id` stores installation association
- one repo belongs to exactly one installation at a time
- installation may cover many repositories

If later needed, a separate `github_app_installations` table can hold installation metadata, but the requested v1.8 minimum can work with `app_install_id` on `repositories`.

## OAuth device code flow for CLI

The request explicitly calls for OAuth device code flow for CLI, replacing PAT.

### Why device code flow

- fits terminal-first operator experience
- avoids requiring a local callback listener
- works for headless/self-hosted environments
- lets operator authenticate interactively without pasting PATs

### Proposed flow

1. CLI requests device code from prATC backend.
2. Backend proxies or initiates GitHub OAuth device authorization.
3. CLI prints:
   - verification URL
   - user code
   - polling interval
4. Operator authorizes in browser.
5. CLI polls backend until complete.
6. Backend stores operator auth session and returns available installations.
7. CLI asks operator which installation(s) / repositories to enable.
8. CLI persists only a prATC session reference locally, not a long-lived GitHub PAT.

### Replacing PAT usage

Current PAT-era behavior:

- runtime asks `gh` for a token
- token is used directly for GitHub API calls

Proposed behavior:

- CLI uses device flow to bootstrap operator identity
- backend stores operator session securely
- backend mints installation tokens when sync/analyze actions need GitHub access
- CLI does not need raw PAT access for normal operation

### Local CLI storage

Store locally only:

- prATC server URL
- operator session identifier or encrypted refresh token reference
- selected installation/repository defaults

Do not store:

- GitHub App private key
- long-lived installation tokens
- plaintext PATs as part of the new default flow

## Webhook receiver endpoint design

### Endpoint

```text
POST /webhooks/github
```

### Verification

For every request:

- verify HMAC signature using the GitHub App webhook secret
- reject missing/invalid signature with 401
- log delivery ID for audit and replay protection
- process idempotently using GitHub delivery ID

### Required subscribed events

Recommended initial events:

- `pull_request`
- `pull_request_review`
- `check_suite`
- `check_run`
- `status`
- `push`
- `installation`
- `installation_repositories`

### Trigger behavior by event

#### `pull_request`

Actions to handle:

- `opened`
- `reopened`
- `synchronize`
- `edited`
- `ready_for_review`
- `converted_to_draft`
- `closed`

Triggers:

- `opened`, `reopened`, `synchronize`, `ready_for_review`: enqueue targeted repo refresh and PR re-analysis
- `edited`: refresh PR metadata only; full re-analysis only if title/body/base changed
- `converted_to_draft`: update cached PR state and downgrade active merge candidacy
- `closed`: mark PR inactive in cache and clear pending status jobs

#### `pull_request_review`

Actions:

- `submitted`
- `edited`
- `dismissed`

Triggers:

- refresh review state for affected PR
- recompute review-related routing or confidence if review status is used by planner
- update commit status/check summary if bucket recommendation changes

#### `check_suite`, `check_run`, `status`

Triggers:

- refresh CI state for affected PR head SHA
- re-run lightweight risk/status synthesis, not full corpus analysis unless configured
- update posted prATC status if risk gate outcome depends on CI signals

#### `push`

Triggers:

- for default branch pushes, update merged/base context and invalidate stale dependency or overlap caches if needed
- for PR head refs, usually covered by `pull_request.synchronize`; avoid duplicate heavy work

#### `installation`

Actions:

- `created`
- `deleted`
- `suspend`
- `unsuspend`

Triggers:

- create/update installation availability records
- mark repositories enabled/disabled accordingly
- stop webhook-triggered work for suspended/deleted installations

#### `installation_repositories`

Actions:

- `added`
- `removed`

Triggers:

- register newly added repositories
- disable access for removed repositories
- update `repositories.app_install_id`
- queue initial sync for newly added repositories if auto-sync is enabled

## Event processing model

Webhook handling should be split into two layers:

1. fast receiver
   - verify signature
   - persist raw event metadata
   - enqueue internal job
   - return 202 quickly

2. async processor
   - mint installation token
   - fetch any missing GitHub details
   - update cache
   - trigger targeted analysis/status publishing

This avoids long webhook latency and keeps retries safe.

## Status check API design

The request asks what gets posted back to PRs as commit status.

### Posting target

Preferred: GitHub Checks API
Fallback: Commit Status API

Why prefer Checks API:

- richer summary text
- actionable details link
- better UI on PRs

Why keep Commit Status fallback:

- simpler compatibility path
- some integrations or permissions may prefer statuses first

### Proposed check/status names

- `pratc/risk-review`
- `pratc/merge-readiness`
- `pratc/dependency-coordination` (only when multi-repo mode is active)

Initial v1.8 minimum can collapse into one combined signal:

- `pratc/analysis`

### Status states

Pending/in-progress:

- posted when analysis job is queued or running after webhook activity

Success:

- no blocking high-risk findings
- no unresolved cross-repo coordination blockers

Failure:

- high-risk findings requiring human intervention
- severe dependency conflict requiring ordered merge or blocking review

Neutral/skipped:

- insufficient data
- repo not enabled for full analysis
- draft PR when policy says not to enforce

### Proposed posted payload contents

For each PR head SHA, post:

- overall state: pending/success/failure/neutral
- short summary: bucket recommendation and confidence
- detail URL back to prATC report or API endpoint
- structured explanation in check run summary, for example:
  - review bucket/category
  - confidence
  - top reasons
  - duplicate/overlap warnings
  - cross-repo dependency warnings
  - advisory note: prATC remains advisory unless org policy separately treats failure as blocking

### Example decision mapping

- `merge_now`, high confidence, no blockers -> success
- `review_required` -> neutral or success with warning, depending on org policy
- `hold_for_dependency` -> failure if configured as gating
- `unknown_escalate` -> failure or neutral depending on enforcement mode

This policy should be configurable because roadmap guardrails say prATC is advisory by default.

## Backward compatibility with existing PAT auth

v1.8 should not force all users to migrate immediately.

### Compatibility modes

1. GitHub App mode
   - preferred default for new installs
   - uses OAuth device flow + installation tokens + webhooks

2. PAT/gh mode
   - compatibility mode for existing single-user operators
   - uses current `internal/github/auth.go` resolution path
   - no webhook trust or status posting unless separately configured

3. Hybrid mode
   - GitHub App for webhook/status operations
   - PAT/gh fallback for local CLI operations during transition

### Selection rules

Suggested auth resolution order:

- if repository has `app_install_id` and app credentials are configured, use GitHub App installation token
- else if operator session exists, use GitHub App flow
- else fallback to existing PAT/gh token resolution

### Migration strategy for operators

- existing PAT users can keep running unchanged
- setup flow should offer “migrate this repo to GitHub App” when app config is present
- once migrated, sync/status/webhooks prefer installation tokens

## Security model

### Secrets and credentials

Sensitive assets in the new model:

- GitHub App private key
- webhook secret
- operator OAuth session/refresh material
- short-lived installation tokens
- optional legacy PAT fallback

### Storage rules

Store securely server-side:

- GitHub App private key: encrypted at rest, file path or secret manager preferred
- webhook secret: encrypted config/secret store
- operator session tokens: encrypted at rest with expiry metadata
- installation tokens: do not persist long-term; cache in memory only if needed, with TTL shorter than token expiry

Do not store in SQLite plaintext:

- raw PATs
- GitHub App private key
- long-lived OAuth refresh secrets in plaintext
- installation access tokens beyond ephemeral cache

SQLite may store only references/metadata:

- installation ID
- operator account ID
- token expiry timestamps
- auth mode flags

### Scope minimization

Use only the minimum GitHub permissions required:

- read repo metadata/content/PR details
- write checks/statuses only if enabled
- no write permissions to contents, merges, or approvals in v1.8

### Token minting model

- mint installation token just-in-time for a job
- bind job work to the repository set authorized by the installation
- discard token after use
- refresh by re-minting instead of storing reusable long-lived tokens

### Webhook security

- verify HMAC signature on every delivery
- track delivery IDs to prevent replay duplication
- reject events for unknown installations
- ensure repository in payload matches authorized installation mapping

### Operator identity boundaries

Operator identity should not grant broader repository access than the selected installation provides.

That means:

- OAuth proves user identity
- installation grants repo access
- prATC acts on repos via installation permissions, not broad user PAT scopes

## Operational flow summary

### Initial setup

1. Admin configures GitHub App credentials and webhook secret.
2. Operator runs CLI login using device code flow.
3. Operator selects installation and repositories.
4. Repositories are registered with `app_install_id`.
5. prATC performs initial sync using installation tokens.

### Ongoing updates

1. GitHub sends webhook event.
2. prATC validates signature and enqueues work.
3. Worker mints installation token.
4. Worker refreshes affected cache data.
5. Worker reruns targeted analysis.
6. Worker posts updated check/status to PR head SHA.

## Non-goals

- historical v1.8 design did not allow auto-merge or auto-approve; v2.0 reopens mutations only through typed ActionIntents and the central executor
- no requirement to abandon PAT fallback immediately
- no broad GitHub write scope beyond statuses/checks in v1.8
- no assumption that every deployment exposes a public webhook endpoint on day one

## Recommended implementation sequencing

1. Add GitHub App config model and secure secret loading.
2. Add repository installation mapping.
3. Implement OAuth device flow for CLI login/bootstrap.
4. Implement installation token minting path.
5. Implement webhook receiver + queue.
6. Implement status/check posting.
7. Preserve PAT fallback and document migration path.

## Open questions

1. Should prATC post Checks API only, or both checks and commit statuses during transition?
2. Should device-flow operator login terminate locally at CLI, or always proxy through prATC server?
3. How should self-hosted/offline deployments handle webhook absence while still using GitHub App auth?
4. Should organization policy control whether `review_required` is neutral vs failing status?

## Recommendation

Adopt GitHub App as the primary repository access model, use OAuth device code flow for CLI operator onboarding, process webhook events asynchronously, and publish advisory status/check results back to PRs. Preserve existing PAT/gh token resolution as a compatibility fallback until operators migrate.
