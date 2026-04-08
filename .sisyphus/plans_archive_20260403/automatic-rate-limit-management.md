# Automatic Rate Limit Management for Large Repositories

## TL;DR

> **Quick Summary**: Implement intelligent rate limit management that automatically spreads GitHub API calls over hours/days for large repositories like openclaw/openclaw (6,646 PRs), eliminating manual intervention and preventing rate limit exhaustion.
> 
> **Deliverables**:
> - Jitter implementation to prevent thundering herd
> - Persistent cursor updates during sync operations  
> - Adaptive chunk sizing based on rate limit budget
> - Automatic job pausing/resuming with scheduling
> - Background scheduler for paused jobs
> - Seamless CLI experience with progress estimation
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 6 waves
> **Critical Path**: RateLimitBudgetManager → SyncJobEnhancement → Scheduler

---

## Context

### Original Request
Repository owners like openclaw struggle with rate limits when processing large repositories (6,646+ PRs). Current prATC exhausts GitHub's 5,000/hour limit within minutes, requiring manual intervention and causing failed operations.

### Problem Analysis
- **Current State**: prATC has basic rate limit detection but no proactive management
- **Root Cause**: No budget tracking, no persistent state during long operations, no scheduling
- **Impact**: Large repos require manual chunking and monitoring, defeating automation goals

### Research Findings
- **Existing Infrastructure**: Cursor-based pagination, job tracking, resume capability all exist
- **Missing Pieces**: Jitter, persistent cursor updates, adaptive scheduling, budget management  
- **GitHub Limits**: 5,000 requests/hour, ~20K+ needed for full openclaw sync
- **Time Required**: 4+ hours minimum for large repos without intelligent spreading

---

## Work Objectives

### Core Objective
Enable prATC to automatically manage GitHub rate limits for repositories of any size, spreading API calls intelligently across available rate limit windows without user intervention.

### Concrete Deliverables
- `internal/telemetry/ratelimit/` package with budget management
- Enhanced sync job with pause/resume capabilities  
- Background scheduler for rate-limit-aware job execution
- CLI commands with automatic rate limit handling
- Web dashboard integration for monitoring

### Definition of Done
- [ ] All tests pass (`go test ./...`)
- [ ] Can process openclaw/openclaw (6,646 PRs) without manual intervention
- [ ] Estimated completion time displayed to user
- [ ] Survives process restarts and network interruptions
- [ ] Zero configuration required from repository owner

### Must Have
- Jitter implementation to prevent thundering herd
- Persistent cursor updates during sync operations
- Adaptive chunk sizing based on current rate limit budget
- Automatic job pausing when budget exhausted
- Background scheduler for resuming paused jobs

### Must NOT Have (Guardrails)
- Breaking changes to existing CLI interface
- Performance degradation for small repositories (<100 PRs)
- Complex configuration requirements for users
- Removal of existing rate limit functionality
- AI slop patterns: over-abstraction, excessive comments, generic naming

---## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (Go test framework, fixtures)
- **Automated tests**: TDD (each task includes test cases)
- **Framework**: `go test -race -v ./...`

### QA Policy
Every task MUST include agent-executed QA scenarios:

- **Backend**: Use Bash (curl) — Test CLI commands with real repositories
- **Library**: Use Bash (go test) — Unit and integration tests
- **Evidence**: `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`

---

## Execution Strategy

### Parallel Execution Waves

> Maximize throughput by grouping independent tasks into parallel waves.
> Each wave completes before the next begins.

```
Wave 1 (Foundation - Rate Limit Infrastructure):
├── Task 1: Add jitter to GitHub client backoff logic [quick]
├── Task 2: Create RateLimitBudgetManager package [quick]  
├── Task 3: Implement rate limit telemetry tracking [quick]
└── Task 4: Add budget-aware request estimation [quick]

Wave 2 (Sync Job Enhancement):
├── Task 5: Extend SyncProgress with scheduling fields [quick]
├── Task 6: Add "paused" job status and persistence [quick]
├── Task 7: Implement persistent cursor updates during sync [quick]
└── Task 8: Create adaptive chunk sizing logic [quick]

Wave 3 (Core Scheduling Logic):
├── Task 9: Implement automatic job pausing when budget exhausted [deep]
├── Task 10: Create background scheduler for paused jobs [deep]
├── Task 11: Add rate limit-aware sync runner [deep]
└── Task 12: Implement resume logic for paused jobs [deep]

Wave 4 (CLI Integration):
├── Task 13: Update sync command with auto-scheduling [quick]
├── Task 14: Add progress estimation to CLI output [quick]
├── Task 15: Update plan/analyze/graph commands for consistency [quick]
└── Task 16: Create comprehensive CLI integration tests [unspecified-high]

Wave 5 (Web Dashboard):
├── Task 17: Add sync job status API endpoints [quick]
├── Task 18: Create web dashboard sync monitoring UI [visual-engineering]
└── Task 19: Implement real-time progress SSE streaming [quick]

Wave FINAL (Verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA with openclaw/openclaw (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay
```

### Dependency Matrix

- **1-4**: Independent foundation tasks
- **5-8**: Build on existing sync infrastructure  
- **9-12**: Depend on 1-8 (need budget manager + enhanced sync)
- **13-16**: Depend on 9-12 (need core scheduling)
- **17-19**: Independent web layer
- **F1-F4**: Final verification after all implementation

---## TODOs

- [x] 1. Add jitter to GitHub client backoff logic

  **What to do**:
  - Modify `handleRateLimit` in `internal/github/client.go` to add 25% jitter
  - Update tests to verify jitter behavior
  - Ensure backward compatibility with existing rate limit logic

  **Must NOT do**:
  - Change existing retry logic or exponential backoff
  - Break existing rate limit functionality
  - Add complex configuration options

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Small, focused change to existing code
  - **Skills**: []
    - No external skills needed for this internal modification

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `internal/github/client.go:380-416` - Current `handleRateLimit` implementation
  - `internal/github/client_test.go:236-289` - Rate limit test patterns

  **API/Type References**:
  - `internal/types/models.go` - No changes needed

  **Test References**:
  - `internal/github/client_test.go:TestRateLimitJitterVariance` - Existing test that should pass after implementation

  **WHY Each Reference Matters**:
  - Need to understand current rate limit handling to add jitter without breaking existing logic
  - Tests show expected behavior and should continue passing

  **Acceptance Criteria**:
  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Jitter prevents thundering herd
    Tool: Bash (go test)
    Preconditions: GitHub client with rate limit configuration
    Steps:
      1. Create GitHub client with rate limit settings
      2. Simulate multiple 403 responses with same Retry-After header
      3. Verify sleep durations have variance (not identical)
      4. Confirm all retries complete within expected time bounds
    Expected Result: Sleep durations vary by up to 25% between calls
    Failure Indicators: Identical sleep durations across retries
    Evidence: .sisyphus/evidence/task-1-jitter-variance.txt

  Scenario: Backward compatibility maintained
    Tool: Bash (go test)
    Preconditions: Existing rate limit tests
    Steps:
      1. Run all existing rate limit tests
      2. Verify no test failures introduced
      3. Confirm exponential backoff still works correctly
    Expected Result: All existing tests pass, jitter doesn't break functionality
    Evidence: .sisyphus/evidence/task-1-backward-compat.txt
  ```

  **Evidence to Capture**:
  - [ ] Each evidence file named: task-1-{scenario-slug}.{ext}
  - [ ] Test output showing jitter variance
  - [ ] Test output showing backward compatibility

  **Commit**: YES | NO (groups with 1)
  - Message: `fix(github): add jitter to rate limit backoff to prevent thundering herd`
  - Files: `internal/github/client.go`, `internal/github/client_test.go`
  - Pre-commit: `go test ./internal/github/...`
## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `tsc --noEmit` + linter + `bun test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (`data/result/item/temp`).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill if UI)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `fix(github): add jitter to rate limit backoff`
- **Wave 2**: `feat(sync): enhance sync job with scheduling fields`
- **Wave 3**: `feat(scheduler): implement rate limit aware job scheduling`
- **Wave 4**: `feat(cli): add automatic rate limit management to commands`
- **Wave 5**: `feat(web): add sync monitoring dashboard`

---

## Success Criteria

### Verification Commands
```bash
# Test jitter implementation
go test ./internal/github/... -run TestRateLimitJitter

# Test full sync with large repository
./bin/pratc sync --repo openclaw/openclaw --max-prs=1000

# Verify CLI shows progress estimation
./bin/pratc sync --repo openclaw/openclaw --watch --interval=1h
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent  
- [ ] Can process 6,646 PR repository without manual intervention
- [ ] Estimated completion time displayed to user
- [ ] Survives process restarts
- [ ] Zero configuration required