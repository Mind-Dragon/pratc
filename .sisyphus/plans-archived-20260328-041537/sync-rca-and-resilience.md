# prATC Sync Failure RCA & Resilience Plan

## TL;DR

> **Quick Summary**: Root cause analysis of sync failure (empty git mirror) and comprehensive plan to fix it by adding proper git mirror initialization, improving error handling, and ensuring the website stays running reliably.

> **Deliverables**:
> - RCA analysis document explaining the sync failure pattern
> - Git mirror initialization fix
> - Error handling and retry logic improvements
> - Health checks and monitoring enhancements

> **Estimated Effort**: Medium
> **Parallel Execution**: YES - N waves
> **Critical Path**: Task 1 (RCA) → Task 2 (Mirror Fix) → Task 3 (Error Handling) → Task 4 (Health Checks) → Task 5 (Testing)

---

## Context

### Original Request
User reported: "i tried to go to 100.112.201.95:7788 and it is not running" followed by "the web page does not load please rca why - review logs, and find out why and then do repair" and finally "create a plan to properly rca this and to ensure that the website stays running (doesn't crash) that the middle ware is resilient and so on"

### Interview Summary
**Key Discussions**:
- Fixed port 8080 conflict (AIMEM → prATC API server)
- Updated Next.js rewrite configuration to use environment variable
- Restarted API with GitHub token
- Identified sync failure: git mirror directory is empty, causing "fetch mirror refs" error

**Research Findings**:
- Current state: Dashboard serving, API proxy working, sync failing
- Root cause: `FetchAll` method expects mirror to exist but no initialization step (clone/init) occurs before `git fetch`
- No git history of recurring sync failures found yet (pending from background agents)

### Metis Review
**Identified Gaps** (to be addressed):
- [Gap 1]: Need to verify if sync failure pattern has occurred before in git history
- [Gap 2]: Need to understand current retry logic and error handling
- [Gap 3]: Need to assess health check implementation gaps
- [Gap 4]: Need to verify Go conventions compliance in proposed fixes

---

## Work Objectives

### Core Objective
Create a comprehensive RCA of the sync failure and implement fixes to ensure the website stays running reliably with resilient middleware.

### Concrete Deliverables
- `.sisyphus/plans/sync-rca-analysis.md` - Detailed root cause analysis document
- `internal/sync/mirror_init.go` - Git mirror initialization fix
- `internal/sync/retry.go` - Retry logic with exponential backoff
- `internal/cmd/health.go` - Enhanced health checks
- `internal/sync/*_test.go` - Tests for sync failures

### Definition of Done
- [ ] RCA analysis document explains root cause and fixes
- [ ] Git mirrors are initialized before fetch operations
- [ ] Error messages are clear and actionable
- [ ] Sync failures trigger retry logic
- [ ] Health checks verify sync status and API health
- [ ] All tests pass with fix applied

### Must Have
- Git mirror must be initialized (clone) before any fetch operations
- Error handling must wrap errors with context (fmt.Errorf with %w)
- Health checks must report sync status (OK/Failed)
- Sync failures must retry with exponential backoff

### Must NOT Have (Guardrails)
- No destructive down-migrations in SQLite schema
- No external dependencies beyond Go standard library
- No GitHub App or OAuth (out of scope for v0.1)
- No ML feedback loops (out of scope for v0.1)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go test framework, bun test for web)
- **Automated tests**: YES (TDD - RED-GREEN-REFACTOR)
- **Framework**: `go test -race -v ./...`
- **If TDD**: Each task follows RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios (see TODO template below).
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend/API**: Use Bash (curl) — Send requests, assert status + response fields
- **Sync/CLI**: Use Bash (bash script) — Run commands, validate output
- **Database**: Use Bash (sqlite3) — Query database, assert row counts

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Immediate - RCA foundation):
├── Task 1: Root Cause Analysis Investigation [deep]
├── Task 2: Error Handling Pattern Analysis [deep]
└── Task 3: Health Check Gap Analysis [deep]

Wave 2 (After Wave 1 - Git mirror fix):
├── Task 4: Git Mirror Initialization Fix [quick]
├── Task 5: Unit Tests for Mirror Initialization [deep]
└── Task 6: Integration Test for Sync Flow [unspecified-high]

Wave 3 (After Wave 2 - Error handling):
├── Task 7: Retry Logic Implementation [deep]
├── Task 8: Error Message Enhancement [quick]
├── Task 9: Unit Tests for Retry Logic [deep]
├── Task 10: Integration Test for Sync Failure Recovery [unspecified-high]

Wave 4 (After Wave 3 - Health checks):
├── Task 11: Health Check Endpoint Implementation [quick]
├── Task 12: Health Check Tests [deep]
├── Task 13: Health Check Integration Test [unspecified-high]

Wave 5 (After Wave 4 - integration):
├── Task 14: End-to-End Sync Flow Test [visual-engineering]
├── Task 15: Dashboard Integration Test [visual-engineering]
└── Task 16: Manual Verification Checklist [unspecified-high]

Wave FINAL (After ALL tasks - 4 parallel reviews, then user okay):
├── Task F1: Plan Compliance Audit (oracle)
├── Task F2: Code Quality Review (unspecified-high)
├── Task F3: Real Manual QA (unspecified-high)
└── Task F4: Scope Fidelity Check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 4 → Task 7 → Task 11 → Task 14 → Task 21 → F1-F4 → user okay
Parallel Speedup: ~85% faster than sequential
Max Concurrent: 3 (Waves 1 & 2)
```

### Dependency Matrix (abbreviated — show ALL tasks in your generated plan)

- **1-3**: — — 4-6, 1
- **4-6**: 1 — 7, 14
- **7-10**: 4, 6 — 11, 14
- **11-13**: 7, 10 — 14, 15
- **14-15**: 11, 13, 16 — F1-F4, 4

> This is abbreviated for reference. YOUR generated plan must include the FULL matrix for ALL tasks.

### Agent Dispatch Summary

- **1**: **3** — T1-T3 → `deep`
- **2**: **3** — T4 → `quick`, T5 → `deep`, T6 → `unspecified-high`
- **3**: **4** — T7 → `deep`, T8 → `quick`, T9 → `deep`, T10 → `unspecified-high`
- **4**: **4** — T11 → `quick`, T12 → `deep`, T13 → `unspecified-high`
- **5**: **3** — T14 → `visual-engineering`, T15 → `visual-engineering`, T16 → `unspecified-high`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

> Implementation + Test = ONE Task. Never separate.
> EVERY task MUST have: Recommended Agent Profile + Parallelization info + QA Scenarios.
> **A task WITHOUT QA Scenarios is INCOMPLETE. No exceptions.**

- [ ] 1. [Task Title]

  **What to do**:
  - [Clear implementation steps]
  - [Test cases to cover]

  **Must NOT do**:
  - [Specific exclusions from guardrails]

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `[visual-engineering | ultrabrain | artistry | quick | unspecified-low | unspecified-high | writing]`
    - Reason: [Why this category fits the task domain]
  - **Skills**: [`skill-1`, `skill-2`]
    - `skill-1`: [Why needed - domain overlap explanation]
    - `skill-2`: [Why needed - domain overlap explanation]
  - **Skills Evaluated but Omitted**:
    - `omitted-skill`: [Why domain doesn't overlap]

  **Parallelization**:
  - **Can Run In Parallel**: YES | NO
  - **Parallel Group**: Wave N (with Tasks X, Y) | Sequential
  - **Blocks**: [Tasks that depend on this task completing]
  - **Blocked By**: [Tasks this depends on] | None (can start immediately)

  **References** (CRITICAL - Be Exhaustive):

  > The executor has NO context from your interview. References are their ONLY guide.
  > Each reference must answer: "What should I look at and WHY?"

  **Pattern References** (existing code to follow):
  - `src/services/auth.ts:45-78` - Authentication flow pattern (JWT creation, refresh token handling)

  **API/Type References** (contracts to implement against):
  - `src/types/user.ts:UserDTO` - Response shape for user endpoints

  **Test References** (testing patterns to follow):
  - `src/__tests__/auth.test.ts:describe("login")` - Test structure and mocking patterns

  **External References** (libraries and frameworks):
  - Official docs: `https://zod.dev/?id=basic-usage` - Zod validation syntax

  **WHY Each Reference Matters** (explain the relevance):
  - Don't just list files - explain what pattern/information the executor should extract
  - Bad: `src/utils.ts` (vague, which utils? why?)
  - Good: `src/utils/validation.ts:sanitizeInput()` - Use this sanitization pattern for user input

  **Acceptance Criteria**:

  > **AGENT-EXECUTABLE VERIFICATION ONLY** — No human action permitted.
  > Every criterion MUST be verifiable by running a command or using a tool.

  **If TDD (tests enabled):**
  - [ ] Test file created: src/auth/login.test.ts
  - [ ] bun test src/auth/login.test.ts → PASS (3 tests, 0 failures)

  **QA Scenarios (MANDATORY — task is INCOMPLETE without these):**

  > **This is NOT optional. A task without QA scenarios WILL BE REJECTED.**
  >
  > Write scenario tests that verify the ACTUAL BEHAVIOR of what you built.
  > Minimum: 1 happy path + 1 failure/edge case per task.
  > Each scenario = exact tool + exact steps + exact assertions + evidence path.
  >
  > **The executing agent MUST run these scenarios after implementation.**
  > **The orchestrator WILL verify evidence files exist before marking task complete.**

  \`\`\`
  Scenario: [Happy path — what SHOULD work]
    Tool: [Playwright / interactive_bash / Bash (curl)]
    Preconditions: [Exact setup state]
    Steps:
      1. [Exact action — specific command/selector/endpoint, no vagueness]
      2. [Next action — with expected intermediate state]
      3. [Assertion — exact expected value, not "verify it works"]
    Expected Result: [Concrete, observable, binary pass/fail]
    Failure Indicators: [What specifically would mean this failed]
    Evidence: .sisyphus/evidence/task-{N}-{scenario-slug}.{ext}

  Scenario: [Failure/edge case — what SHOULD fail gracefully]
    Tool: [same format]
    Preconditions: [Invalid input / missing dependency / error state]
    Steps:
      1. [Trigger the error condition]
      2. [Assert error is handled correctly]
    Expected Result: [Graceful failure with correct error message/code]
    Evidence: .sisyphus/evidence/task-{N}-{scenario-slug}-error.{ext}
  \`\`\`

  > **Specificity requirements — every scenario MUST use:**
  > - **Selectors**: Specific CSS selectors (`.login-button`, not "the login button")
  > - **Data**: Concrete test data (`"test@example.com"`, not `"[email]"`)
  > - **Assertions**: Exact values (`text contains "Welcome back"`, not "verify it works")
  > - **Timing**: Wait conditions where relevant (`timeout: 10s`)
  > - **Negative**: At least ONE failure/error scenario per task
  >
  > **Anti-patterns (your scenario is INVALID if it looks like this):**
  > - ❌ "Verify it works correctly" — HOW? What does "correctly" mean?
  > - ❌ "Check the API returns data" — WHAT data? What fields? What values?
  > - ❌ "Test the component renders" — WHERE? What selector? What content?
  > - Any scenario without an evidence path

  **Evidence to Capture**:
  - [ ] Each evidence file named: task-{N}-{scenario-slug}.{ext}
  - [ ] Screenshots for UI, terminal output for CLI, response bodies for API

  **Commit**: YES | NO (groups with N)
  - Message: `type(scope): desc`
  - Files: `path/to/file`
  - Pre-commit: `test command`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `tsc --noEmit` + linter + `bun test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill if UI)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **1**: `type(scope): desc` — file.ts, npm test

---

## Success Criteria

### Verification Commands
```bash
command  # Expected: output
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass