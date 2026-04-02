# OpenClaw 5k+ PR Analysis with Minimax + Voyage (No OpenRouter)

## TL;DR
> **Summary**: Execute a deterministic, staged dual-backend analysis of `openclaw/openclaw` (~5000+ PRs) using prATC with `ML_BACKEND=minimax` and `ML_BACKEND=voyage`, then compare outputs and reliability under identical frozen inputs.
> **Deliverables**:
> - Secret-safe runtime configuration (rotated keys, env-only injection)
> - Frozen snapshot evidence for fair backend comparison
> - Staged execution evidence (100 → 1k → 5k+)
> - Comparative report (quality, latency, failure profile)
> **Effort**: Medium
> **Parallel**: YES - 3 waves
> **Critical Path**: T1 → T2 → T3 → T4 → T7/T8 → T9

## Context
### Original Request
Run prATC analysis on `openclaw/openclaw` at 5k+ PR scale using Minimax and Voyage (explicitly no OpenRouter), and provide a concrete plan.

### Interview Summary
- Endpoint decision: `api.minimax.io` selected.
- Run strategy decision: dual-run comparison on the same frozen snapshot selected.
- Security constraint: API key posted in chat must be rotated and never stored in repo files.

### Metis Review (gaps addressed)
- Added mandatory secret-rotation gate before execution.
- Added snapshot-freeze and deterministic-manifest requirements.
- Added staged promotion gates and explicit retry/abort policy.
- Added acceptance thresholds for comparability and reliability.

## Work Objectives
### Core Objective
Produce a reproducible, auditable backend comparison for Minimax vs Voyage on `openclaw/openclaw` at 5k+ PR scale, with no OpenRouter usage.

### Deliverables
- Runtime config checklist for Minimax/Voyage only.
- Frozen input evidence proving both runs used identical data.
- Stage gate evidence files for 100, 1k, 5k+ runs per backend.
- Final comparison bundle with recommendation.

### Definition of Done (verifiable conditions with commands)
- `go build ./...` exits 0.
- `go test ./internal/ml ./internal/app ./internal/cmd -count=1` exits 0.
- `uv run pytest ml-service/tests -v` exits 0.
- `grep -R "openrouter" ml-service/src/pratc_ml/providers` returns no provider usage in active run config.
- Evidence set exists under `.sisyphus/evidence/openclaw-5k-minimax-voyage/`.

### Must Have
- Only `ML_BACKEND=minimax|voyage` for experiment runs.
- Minimax endpoint policy fixed to `api.minimax.io` (global) unless region override is explicitly documented.
- Same frozen dataset/snapshot for both backend runs.
- Deterministic, comparable output artifacts.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- No OpenRouter backend usage.
- No API keys in tracked files, logs, or markdown.
- No claim of comparability without frozen-input proof.
- No silent fallback between backends during a run.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: tests-after (existing Go + Python test suites)
- QA policy: Every task includes happy + failure scenario with evidence path
- Evidence root: `.sisyphus/evidence/openclaw-5k-minimax-voyage/`

## Execution Strategy
### Parallel Execution Waves
Wave 1: Security + experiment baseline (T1-T4)
Wave 2: Stage gates (T5-T8)
Wave 3: Full-run comparison + final bundle (T9-T10)

### Dependency Matrix (full, all tasks)
- T1 blocks all tasks.
- T2 blocks T3-T10.
- T3 blocks T5-T10.
- T4 blocks T5-T10.
- T5/T6 block T7/T8 respectively.
- T7 and T8 block T9.
- T9 blocks T10.

### Agent Dispatch Summary
- Wave 1: 4 tasks (unspecified-high/deep)
- Wave 2: 4 tasks (unspecified-high)
- Wave 3: 2 tasks (deep + writing)

## TODOs

- [x] 1. Rotate exposed Minimax secret and enforce runtime-only secret injection

  **What to do**: Revoke/rotate the posted Minimax key; define runbook using secret manager (`psst` or equivalent) so keys never touch repo files.
  **Must NOT do**: Commit key material to `.env`, markdown, JSON, or shell history artifacts.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: security-critical prerequisite.
  - Skills: [`verification-before-completion`] — verify no leaks.
  - Omitted: [`test-driven-development`] — no code feature implementation here.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T2-T10 | Blocked By: none

  **References**:
  - Pattern: `AGENTS.md` — secret handling guardrails (`psst SECRET_NAME -- <command>`)
  - Pattern: `ml-service/src/pratc_ml/providers/__init__.py` — env-driven keys

  **Acceptance Criteria**:
  - [ ] Old key is revoked/rotated externally and not reused.
  - [ ] Runbook created in evidence describing secure injection commands.

  **QA Scenarios**:
  ```
  Scenario: Secret injection happy path
    Tool: Bash
    Steps: run command with secret manager wrapper and confirm process starts without plaintext key in command text
    Expected: process receives key via env; no tracked file contains key
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-1-secret-injection.txt

  Scenario: Secret leak guard
    Tool: Bash
    Steps: search repository and evidence for exposed key patterns
    Expected: no matches found for key pattern
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-1-secret-leak-scan.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 2. Lock backend configuration contract (Minimax + Voyage, no OpenRouter)

  **What to do**: Document exact env matrix for both backends; enforce endpoint decision (`api.minimax.io` global, `api.minimaxi.com` region override only) and Voyage endpoint defaults.
  **Must NOT do**: Introduce OpenRouter vars or fallback paths in run matrix.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: configuration correctness and drift prevention.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T3-T10 | Blocked By: T1

  **References**:
  - API/Type: `ml-service/src/pratc_ml/providers/__init__.py:14-47`
  - API/Type: `ml-service/src/pratc_ml/providers/minimax.py:19-34`
  - API/Type: `ml-service/src/pratc_ml/providers/voyage.py`

  **Acceptance Criteria**:
  - [ ] Backend matrix written and versioned in evidence.
  - [ ] `ML_BACKEND=minimax` and `ML_BACKEND=voyage` both validate in smoke checks.

  **QA Scenarios**:
  ```
  Scenario: Minimax config validation
    Tool: Bash
    Steps: run python provider validation with ML_BACKEND=minimax and required env vars
    Expected: no BackendConfigError
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-2-minimax-config.txt

  Scenario: OpenRouter exclusion
    Tool: Bash
    Steps: execute run-matrix checks ensuring no OpenRouter vars required/used
    Expected: matrix includes only local|minimax|voyage
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-2-openrouter-excluded.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 3. Freeze experiment input snapshot for reproducible dual-run comparison

  **What to do**: Produce one frozen data state for `openclaw/openclaw` (sync checkpoint + DB/mirror hash evidence) and reuse it for both backend runs.
  **Must NOT do**: Refresh sync between paired minimax/voyage comparisons.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: reproducibility-critical execution control.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T5-T10 | Blocked By: T2

  **References**:
  - Pattern: `internal/cache/sqlite.go`
  - Pattern: `internal/sync/worker.go`
  - Pattern: `internal/repo/mirror.go`

  **Acceptance Criteria**:
  - [ ] Snapshot hash manifest exists and is reused by both backend runs.
  - [ ] Evidence proves identical PR universe for both backends.

  **QA Scenarios**:
  ```
  Scenario: Frozen snapshot happy path
    Tool: Bash
    Steps: compute and record DB/mirror hashes before runs
    Expected: same hashes referenced by both run manifests
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-3-snapshot-hashes.txt

  Scenario: Drift detection
    Tool: Bash
    Steps: intentionally trigger a resync and compare hashes
    Expected: drift detected and run blocked
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-3-drift-detection.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 4. Define deterministic output manifest and comparison schema

  **What to do**: Standardize per-run artifacts (config manifest, timings, status counts, canonicalized output digests).
  **Must NOT do**: Compare raw unsorted JSON outputs.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: comparability and reproducibility.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T5-T10 | Blocked By: T2

  **References**:
  - Pattern: `scripts/slo_benchmark.sh`
  - Pattern: `.sisyphus/evidence/task-12-pr-verification-bundle.md`

  **Acceptance Criteria**:
  - [ ] Manifest template is fixed before first stage run.
  - [ ] Canonical sort/digest rules are documented and used consistently.

  **QA Scenarios**:
  ```
  Scenario: Deterministic manifest generation
    Tool: Bash
    Steps: generate manifest twice from same input snapshot
    Expected: identical digests
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-4-manifest-determinism.txt

  Scenario: Non-canonical output rejection
    Tool: Bash
    Steps: feed unsorted output to comparator
    Expected: comparator rejects with explicit normalization error
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-4-manifest-rejection.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 5. Execute stage gate (100 PR) with Minimax

  **What to do**: Run 100-PR pilot on frozen snapshot using Minimax config and capture latency/error/retry evidence.
  **Must NOT do**: Promote to 1k if auth/endpoint/config errors are present.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: operational validation.
  - Skills: [`verification-before-completion`]
  - Omitted: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T7 | Blocked By: T3,T4

  **References**:
  - Pattern: `internal/ml/bridge.go`
  - Test: `ml-service/tests/test_overlap.py`

  **Acceptance Criteria**:
  - [ ] 100-PR run completes with zero fatal provider errors.
  - [ ] Retry behavior and timings are captured.

  **QA Scenarios**:
  ```
  Scenario: Minimax 100-PR pilot happy path
    Tool: Bash
    Steps: run analysis with ML_BACKEND=minimax on fixed 100 PR subset
    Expected: contract-valid output and complete evidence
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-5-minimax-100.txt

  Scenario: Minimax auth failure
    Tool: Bash
    Steps: run with invalid token
    Expected: deterministic configuration/auth error and no partial success claim
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-5-minimax-auth-failure.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 6. Execute stage gate (100 PR) with Voyage

  **What to do**: Run 100-PR pilot on same frozen snapshot using Voyage config and capture equivalent metrics.
  **Must NOT do**: Change subset or snapshot used by Minimax pilot.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: paired baseline validation.
  - Skills: [`verification-before-completion`]
  - Omitted: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8 | Blocked By: T3,T4

  **References**:
  - API/Type: `ml-service/src/pratc_ml/providers/voyage.py`

  **Acceptance Criteria**:
  - [ ] 100-PR run completes with contract-valid output.
  - [ ] Evidence format matches Minimax pilot for comparability.

  **QA Scenarios**:
  ```
  Scenario: Voyage 100-PR pilot happy path
    Tool: Bash
    Steps: run analysis with ML_BACKEND=voyage on same 100 PR subset
    Expected: contract-valid output and complete evidence
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-6-voyage-100.txt

  Scenario: Voyage quota/rate-limit handling
    Tool: Bash
    Steps: stress pilot with constrained rate budget
    Expected: bounded retries with explicit rate-limit evidence
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-6-voyage-rate-limit.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 7. Execute stage gate (1k PR) with Minimax

  **What to do**: Promote Minimax run to 1k PR subset after gate-100 pass.
  **Must NOT do**: Bypass gate conditions.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: scale stress check.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T9 | Blocked By: T5

  **References**:
  - SLO: `AGENTS.md` performance targets

  **Acceptance Criteria**:
  - [ ] 1k run completes within defined envelope and bounded failures.
  - [ ] Stage metrics persisted.

  **QA Scenarios**:
  ```
  Scenario: Minimax 1k stress run
    Tool: Bash
    Steps: run 1k subset with minimax and configured retries
    Expected: completed run with recorded timings/retries
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-7-minimax-1k.txt

  Scenario: Timeout guard
    Tool: Bash
    Steps: apply low timeout configuration
    Expected: graceful timeout failure with explicit abort reason
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-7-minimax-timeout.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 8. Execute stage gate (1k PR) with Voyage

  **What to do**: Promote Voyage run to 1k PR subset after gate-100 pass.
  **Must NOT do**: Change evaluation thresholds relative to Minimax run.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: paired scale check.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T9 | Blocked By: T6

  **References**:
  - API/Type: `ml-service/src/pratc_ml/duplicates.py`

  **Acceptance Criteria**:
  - [ ] 1k run completes with equivalent evidence schema.
  - [ ] No unbounded retry loop or silent fallback.

  **QA Scenarios**:
  ```
  Scenario: Voyage 1k stress run
    Tool: Bash
    Steps: run 1k subset with voyage and configured retries
    Expected: completed run and comparable metrics
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-8-voyage-1k.txt

  Scenario: Provider fallback prohibition
    Tool: Bash
    Steps: induce voyage failure condition
    Expected: run fails explicitly without switching backend
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-8-voyage-no-fallback.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 9. Execute full 5k+ paired runs and produce backend comparison report

  **What to do**: Run full dataset for Minimax and Voyage using the same frozen snapshot; compare output quality/reliability/performance.
  **Must NOT do**: Claim winner without side-by-side evidence.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: cross-backend analysis and decision-making.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T10 | Blocked By: T7,T8

  **References**:
  - Pattern: `scripts/slo_benchmark.sh`
  - Pattern: `.sisyphus/evidence/task-12-pr-verification-bundle.md`

  **Acceptance Criteria**:
  - [ ] Both full runs complete or fail with deterministic, classified reasons.
  - [ ] Comparison table includes: runtime, retries, failure codes, cluster/duplicate overlap metrics.

  **QA Scenarios**:
  ```
  Scenario: Full paired run happy path
    Tool: Bash
    Steps: run minimax full run then voyage full run on identical snapshot
    Expected: two complete manifests and diff report generated
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-9-full-paired-run.txt

  Scenario: One backend degraded
    Tool: Bash
    Steps: simulate provider degradation for one backend
    Expected: report marks backend as degraded with explicit reason and no false comparability claim
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-9-backend-degraded.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`

- [x] 10. Produce final execution bundle and recommendation decision

  **What to do**: Consolidate all stage evidence, risk notes, and recommendation (Minimax primary, Voyage primary, or dual strategy).
  **Must NOT do**: Omit unresolved risks or endpoint caveats.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: final technical narrative and decision memo.
  - Skills: []
  - Omitted: []

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: Final Wave | Blocked By: T9

  **References**:
  - External: `https://platform.minimax.io/docs/api-reference/api-overview`
  - External: `https://docs.voyageai.com/reference/embeddings-api`

  **Acceptance Criteria**:
  - [ ] Bundle includes config manifest, stage evidence index, comparison metrics, and recommendation.
  - [ ] Explicitly states OpenRouter excluded and secret-handling compliance.

  **QA Scenarios**:
  ```
  Scenario: Bundle completeness check
    Tool: Bash
    Steps: run checklist over expected artifact paths
    Expected: all required artifacts present and linked
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-10-bundle-completeness.txt

  Scenario: Decision traceability check
    Tool: Read
    Steps: inspect final recommendation section against comparison data
    Expected: recommendation traceable to measured metrics and risk notes
    Evidence: .sisyphus/evidence/openclaw-5k-minimax-voyage/task-10-decision-traceability.md
  ```

  **Commit**: NO | Message: `n/a` | Files: `.sisyphus/evidence/**`, `.sisyphus/status/**`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [x] F1. Plan Compliance Audit — oracle
- [x] F2. Code Quality Review — unspecified-high
- [x] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [x] F4. Scope Fidelity Check — deep

## Commit Strategy
- Keep evidence-only commits separate from runtime code changes.
- If endpoint/provider code adjustments are required, commit them before stage evidence commits.
- Suggested commit grouping:
  1. `docs(runbook): secret rotation and backend config matrix`
  2. `chore(experiment): snapshot freeze and deterministic manifest scaffolding`
  3. `test(experiment): staged 100/1k backend validation evidence`
  4. `test(experiment): full 5k paired-run and comparison bundle`

## Success Criteria
- Minimax and Voyage runs are executed against the same frozen OpenClaw snapshot.
- OpenRouter is fully excluded from configuration and execution.
- Stage gates (100, 1k, 5k+) have complete deterministic evidence.
- Final recommendation is evidence-backed and reproducible.
- Final Wave F1-F4 all APPROVE and user gives explicit final okay.
