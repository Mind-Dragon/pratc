# Comprehensive prATC Workspace Reinitialization and Documentation

## TL;DR
> **Summary**: Complete workspace reinitialization including archiving workflow artifacts, fixing broken tests via TDD approach, adding comprehensive AI-friendly documentation, and ensuring clean compilable state.
> **Deliverables**: Archived .sisyphus/ and stale docs, fixed test suite, MIT LICENSE, CONTRIBUTING.md, CHANGELOG.md, comprehensive API/database/UI documentation in pwd/docs
> **Effort**: Large
> **Parallel**: YES - 5 waves
> **Critical Path**: Archive → Test Fix → Documentation → Verification

## Context
### Original Request
User wants to reinitialize the prATC workspace completely:
1. Archive all items in `.sisyphus/` to `pwd/archives` (gitignored)
2. Archive all stale planning documents 
3. Add MIT license and standard documentation (CONTRIBUTING.md, CHANGELOG.md)
4. Fix broken tests using TDD approach (understand plan → create test → fix code → verify)
5. Replace OpenRouter references with MiniMax
6. Create comprehensive AI-friendly architecture, UI wiring, middleware, database schema, and API documentation in `pwd/docs`

### Interview Summary
- Archive location: `pwd/archives` (gitignored)
- Test fix approach: TDD - understand current implementation, update tests to match actual API, then ensure code works correctly
- OpenRouter replacement: Replace with "MiniMax" 
- Standard docs: MIT LICENSE, CONTRIBUTING.md, CHANGELOG.md, updated README.md
- AI documentation: Comprehensive architecture, UI wiring, middleware wiring, database schema, API docs in `pwd/docs`

### Metis Review (gaps addressed)
- Confirmed archive safety approach
- Clarified test fix strategy (TDD with understanding first)
- Defined exact documentation scope
- Identified compilation errors and their root cause
- Ensured no scope creep while maintaining comprehensive coverage

## Work Objectives
### Core Objective
Reinitialize the prATC workspace to a clean, well-documented, and fully functional state with comprehensive AI-friendly documentation.

### Deliverables
- Archived `.sisyphus/` workflow files in `archives/` directory
- Archived stale planning documents in `archives/` directory  
- Fixed test suite that passes completely
- MIT LICENSE file at root
- CONTRIBUTING.md and CHANGELOG.md files
- Updated README.md with current project status
- Comprehensive documentation in `docs/` covering:
  - Architecture overview
  - UI component wiring
  - Middleware flow
  - Database schema
  - API contracts
- Clean git workspace with no compilation errors
- All OpenRouter references replaced with MiniMax

### Definition of Done (verifiable conditions with commands)
- `ls archives/` shows archived `.sisyphus/` and stale docs directories
- `make test-go` exits with code 0 (all tests pass)
- `grep -r "openrouter" --exclude-dir=fixtures .` returns no results outside fixtures
- `cat LICENSE` shows MIT license text
- `ls docs/` shows comprehensive documentation structure
- `git status` shows clean working tree (no uncommitted changes)
- `make build` compiles successfully with binary at `./bin/pratc`

### Must Have
- Safety-first archiving (backup before deletion)
- TDD approach for test fixes (understand → test → fix → verify)
- AI-friendly documentation structure following 2026 best practices
- Comprehensive coverage of all system components
- Clean compilation and test execution

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- ❌ No deletion without archiving first
- ❌ No test deletion - only fixes
- ❌ No scope expansion beyond confirmed documentation list
- ❌ No "AI slop" - all documentation must be specific, actionable, and accurate
- ❌ No assumptions about undocumented behavior

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: TDD approach - understand current implementation, create correct tests, fix any implementation issues
- QA policy: Every task has agent-executed scenarios
- Evidence: .sisyphus/evidence/task-{N}-{slug}.{ext}

## Execution Strategy
### Parallel Execution Waves
> Target: 5-8 tasks per wave. Extract shared dependencies as Wave-1 tasks for max parallelism.

Wave 1: Archive Preparation and Safety
- Create archives/ directory and add to .gitignore
- Archive .sisyphus/ workflow files
- Archive stale planning documents

Wave 2: Test Understanding and Fixing (TDD)
- Analyze current Plan() implementation and PlanResponse structure
- Update service_test.go to match actual API
- Update dryrun_test.go to match actual PlanResponse structure  
- Verify all tests pass

Wave 3: Documentation Foundation
- Add MIT LICENSE
- Create CONTRIBUTING.md and CHANGELOG.md
- Update README.md with current project status

Wave 4: Comprehensive AI Documentation
- Create architecture documentation in docs/
- Create UI wiring documentation in docs/
- Create middleware documentation in docs/
- Create database schema documentation in docs/
- Create API contract documentation in docs/

Wave 5: Cleanup and Final Verification
- Replace OpenRouter references with MiniMax
- Verify clean compilation and test execution
- Final workspace verification

### Dependency Matrix (full, all tasks)
| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | None | 2, 3 |
| 2 | 1 | 6 |
| 3 | 1 | 7 |
| 4 | None | 5 |
| 5 | 4 | 6, 7 |
| 6 | 2, 5 | 8, 9 |
| 7 | 3, 5 | 10, 11, 12, 13, 14 |
| 8 | 6 | 15 |
| 9 | 6 | 15 |
| 10 | 7 | 15 |
| 11 | 7 | 15 |
| 12 | 7 | 15 |
| 13 | 7 | 15 |
| 14 | 7 | 15 |
| 15 | 8, 9, 10, 11, 12, 13, 14 | F1, F2, F3, F4 |

### Agent Dispatch Summary (wave → task count → categories)
- Wave 1: 3 tasks → filesystem, archive
- Wave 2: 3 tasks → testing, go
- Wave 3: 3 tasks → documentation, writing
- Wave 4: 5 tasks → documentation, writing, architecture
- Wave 5: 1 task → verification, cleanup

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [ ] 1. Create archives/ directory and add to .gitignore

  **What to do**: Create `archives/` directory at root and add it to `.gitignore` to ensure it's gitignored as requested.
  **Must NOT do**: Don't add anything else to .gitignore beyond the archives/ directory.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Simple filesystem operations
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper planning discipline
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: No testing required

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [2, 3] | Blocked By: []

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: User request specifies "pwd/archives git ignored"
  - External: Standard .gitignore pattern for archive directories

  **Acceptance Criteria** (agent-executable only):
  - [ ] `test -d archives/` returns true
  - [ ] `grep "archives/" .gitignore` returns match

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create archives directory and add to gitignore
    Tool: Bash
    Steps: mkdir archives && echo "archives/" >> .gitignore
    Expected: Directory exists and entry in .gitignore
    Evidence: .sisyphus/evidence/task-1-create-archives.txt

  Scenario: Verify archives directory is gitignored
    Tool: Bash
    Steps: git check-ignore archives/
    Expected: Returns "archives/" (confirms it's ignored)
    Evidence: .sisyphus/evidence/task-1-gitignore-verify.txt
  ```

  **Commit**: YES | Message: `chore: create archives directory` | Files: [archives/, .gitignore]

- [ ] 2. Archive .sisyphus/ workflow files

  **What to do**: Copy entire `.sisyphus/` directory to `archives/sisyphus/` preserving all subdirectories and files.
  **Must NOT do**: Don't delete the original `.sisyphus/` directory yet - keep it until final verification.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Filesystem copy operation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper archival process
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: No testing required

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [6] | Blocked By: [1]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: User request specifies "archive all items in .sisyphus"
  - External: Standard archival best practices

  **Acceptance Criteria** (agent-executable only):
  - [ ] `test -d archives/sisyphus/` returns true
  - [ ] `diff -r .sisyphus/ archives/sisyphus/` returns no differences

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Archive .sisyphus directory
    Tool: Bash
    Steps: cp -r .sisyphus/ archives/sisyphus/
    Expected: archives/sisyphus/ contains all original files
    Evidence: .sisyphus/evidence/task-2-archive-sisyphus.txt

  Scenario: Verify complete archive
    Tool: Bash
    Steps: find .sisyphus -type f | wc -l && find archives/sisyphus -type f | wc -l
    Expected: Both counts are identical (100 files)
    Evidence: .sisyphus/evidence/task-2-archive-count.txt
  ```

  **Commit**: YES | Message: `chore: archive sisyphus workflow files` | Files: [archives/sisyphus/]

- [ ] 3. Archive stale planning documents

  **What to do**: Copy stale planning documents (`pratc.md`, `prATC _ App Architecture Plan.md`, `plans/architecture-plan.md`, `plans/pr-review-analysis.md`) to `archives/planning/` directory.
  **Must NOT do**: Don't delete the original files yet - keep them until final verification.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Filesystem copy operation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper archival process
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: No testing required

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: [7] | Blocked By: [1]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: User request specifies "archive all state" and "stale planning documents"
  - External: From audit findings - 4 specific stale planning docs identified

  **Acceptance Criteria** (agent-executable only):
  - [ ] `test -d archives/planning/` returns true
  - [ ] All 4 stale planning documents exist in `archives/planning/`

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Archive stale planning documents
    Tool: Bash
    Steps: mkdir -p archives/planning && cp "pratc.md" "prATC _ App Architecture Plan.md" plans/architecture-plan.md plans/pr-review-analysis.md archives/planning/
    Expected: All 4 documents copied to archive location
    Evidence: .sisyphus/evidence/task-3-archive-planning.txt

  Scenario: Verify archive completeness
    Tool: Bash
    Steps: ls archives/planning/
    Expected: Shows all 4 planning document filenames
    Evidence: .sisyphus/evidence/task-3-archive-list.txt
  ```

  **Commit**: YES | Message: `chore: archive stale planning documents` | Files: [archives/planning/]

- [ ] 4. Analyze current Plan() implementation and PlanResponse structure

  **What to do**: Examine the actual `Plan()` function signature in `internal/app/service.go` and the `PlanResponse` struct in `internal/types/models.go` to understand the current API contract.
  **Must NOT do**: Don't make any code changes - only analysis.

  **Recommended Agent Profile**:
  - Category: `deep` — Reason: Requires deep understanding of Go API contracts
  - Skills: [`superpowers/systematic-debugging`, `superpowers/writing-plans`] — Why needed: Systematic analysis of broken tests
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Analysis phase only

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [5] | Blocked By: []

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/app/service.go:299` shows `func (s Service) Plan(ctx context.Context, repo string, target int, mode formula.Mode)`
  - API/Type: `internal/types/models.go:213-229` shows `PlanResponse` struct without `DryRun` field
  - Test: Current failing tests expect `dryRun bool` parameter and `DryRun` field

  **Acceptance Criteria** (agent-executable only):
  - [ ] Document shows Plan() takes 4 parameters (ctx, repo, target, mode)
  - [ ] Document shows PlanResponse has no DryRun field

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Analyze Plan function signature
    Tool: Bash
    Steps: grep -A 5 "func.*Plan(" internal/app/service.go
    Expected: Shows function with 4 parameters, no dryRun
    Evidence: .sisyphus/evidence/task-4-plan-signature.txt

  Scenario: Analyze PlanResponse structure
    Tool: Bash
    Steps: grep -A 20 "type PlanResponse struct" internal/types/models.go
    Expected: Shows struct fields, confirms no DryRun field
    Evidence: .sisyphus/evidence/task-4-planresponse-struct.txt
  ```

  **Commit**: NO | Message: `N/A` | Files: []

- [ ] 5. Update tests to match actual API implementation

  **What to do**: Update `internal/app/service_test.go` and `internal/cmd/dryrun_test.go` to match the actual `Plan()` function signature and `PlanResponse` struct structure. Remove the extra `dryRun` parameter call and remove `DryRun` field references.
  **Must NOT do**: Don't change the actual implementation - only fix the tests to match current reality.

  **Recommended Agent Profile**:
  - Category: `test-driven-development` — Reason: Classic TDD scenario - tests don't match implementation
  - Skills: [`superpowers/test-driven-development`, `superpowers/writing-plans`] — Why needed: Proper TDD discipline
  - Omitted: [`superpowers/systematic-debugging`] — Why not needed: Debugging already complete

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: [6, 7] | Blocked By: [4]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: `internal/app/service_test.go:108` currently calls `service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination, true)` - needs to remove the last `true`
  - Test: `internal/cmd/dryrun_test.go:29` currently includes `DryRun: dryRun,` in struct literal - needs removal
  - API/Type: Actual Plan() signature has 4 parameters, PlanResponse has no DryRun field

  **Acceptance Criteria** (agent-executable only):
  - [ ] `internal/app/service_test.go:108` calls Plan() with exactly 4 arguments
  - [ ] `internal/cmd/dryrun_test.go` has no references to `DryRun` field
  - [ ] `make test-go` compiles successfully

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Fix service_test.go Plan() call
    Tool: interactive_bash
    Steps: Use editor to remove extra true parameter from line 108
    Expected: Line 108 shows service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
    Evidence: .sisyphus/evidence/task-5-service-test-fix.txt

  Scenario: Fix dryrun_test.go DryRun references
    Tool: interactive_bash
    Steps: Use editor to remove all DryRun field assignments and accesses
    Expected: No DryRun references remain in file
    Evidence: .sisyphus/evidence/task-5-dryrun-test-fix.txt

  Scenario: Verify compilation success
    Tool: Bash
    Steps: make test-go
    Expected: Compiles without errors (may still have test failures, but no compilation errors)
    Evidence: .sisyphus/evidence/task-5-compilation-verify.txt
  ```

  **Commit**: YES | Message: `fix: align tests with actual Plan API` | Files: [internal/app/service_test.go, internal/cmd/dryrun_test.go]

- [ ] 6. Add MIT LICENSE and standard documentation files

  **What to do**: Create MIT LICENSE file at root, create CONTRIBUTING.md and CHANGELOG.md files with standard content appropriate for this project.
  **Must NOT do**: Don't include proprietary or inappropriate license text.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: Documentation creation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper documentation structure
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: No testing required

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [8, 9] | Blocked By: [2, 5]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: User specifically requested "MIT license, use standards for rest"
  - External: Standard MIT license template from opensource.org
  - External: Standard CONTRIBUTING.md and CHANGELOG.md templates

  **Acceptance Criteria** (agent-executable only):
  - [ ] `LICENSE` file exists with valid MIT license text
  - [ ] `CONTRIBUTING.md` file exists with contribution guidelines
  - [ ] `CHANGELOG.md` file exists with changelog structure

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create MIT LICENSE file
    Tool: Write
    Steps: Create LICENSE file with standard MIT text
    Expected: File contains Copyright notice and MIT license terms
    Evidence: .sisyphus/evidence/task-6-license.txt

  Scenario: Create CONTRIBUTING.md
    Tool: Write
    Steps: Create CONTRIBUTING.md with standard contribution guidelines
    Expected: File contains sections for bug reports, feature requests, pull requests
    Evidence: .sisyphus/evidence/task-6-contributing.txt

  Scenario: Create CHANGELOG.md
    Tool: Write
    Steps: Create CHANGELOG.md with standard changelog format
    Expected: File contains [Unreleased] section and version history structure
    Evidence: .sisyphus/evidence/task-6-changelog.txt
  ```

  **Commit**: YES | Message: `docs: add standard documentation files` | Files: [LICENSE, CONTRIBUTING.md, CHANGELOG.md]

- [ ] 7. Update README.md with current project status

  **What to do**: Update `README.md` to reflect current project status, remove outdated information, and ensure it accurately represents the current codebase state.
  **Must NOT do**: Don't remove important user-facing documentation.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: Documentation update
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper documentation updates
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: No testing required

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: [10, 11, 12, 13, 14] | Blocked By: [3, 5]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Current README.md contains one note about minimax-light replacing openrouter-light
  - External: Current project structure from AGENTS.md and actual codebase
  - Test: Ensure CLI commands, API routes, and Docker profiles are accurate

  **Acceptance Criteria** (agent-executable only):
  - [ ] `README.md` accurately reflects current CLI commands
  - [ ] `README.md` accurately reflects current API routes
  - [ ] `README.md` accurately reflects current Docker profiles (local-ml, minimax-light)

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Update README.md CLI section
    Tool: interactive_bash
    Steps: Verify and update CLI command documentation
    Expected: Commands match actual implementation in cmd/pratc/
    Evidence: .sisyphus/evidence/task-7-readme-cli.txt

  Scenario: Update README.md API section
    Tool: interactive_bash
    Steps: Verify and update API route documentation
    Expected: Routes match actual implementation in internal/cmd/
    Evidence: .sisyphus/evidence/task-7-readme-api.txt

  Scenario: Update README.md Docker section
    Tool: interactive_bash
    Steps: Verify and update Docker profile documentation
    Expected: Profiles match docker-compose.yml (local-ml, minimax-light)
    Evidence: .sisyphus/evidence/task-7-readme-docker.txt
  ```

  **Commit**: YES | Message: `docs: update README with current status` | Files: [README.md]

- [ ] 8. Create comprehensive architecture documentation

  **What to do**: Create detailed architecture documentation in `docs/architecture/` covering the overall system design, component interactions, and data flow.
  **Must NOT do**: Don't include speculative or undocumented features.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: Comprehensive documentation creation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper documentation structure
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Documentation task

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: [15] | Blocked By: [6, 9]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Follow n8n AGENTS.md structure for monorepos
  - External: Current system structure from AGENTS.md files
  - API/Type: Component interactions from internal/ package structure

  **Acceptance Criteria** (agent-executable only):
  - [ ] `docs/architecture/overview.md` exists with system overview
  - [ ] `docs/architecture/components.md` exists with component details
  - [ ] `docs/architecture/data-flow.md` exists with data flow diagrams

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create architecture overview
    Tool: Write
    Steps: Create docs/architecture/overview.md with system overview
    Expected: Covers Go CLI, Python ML service, Next.js dashboard
    Evidence: .sisyphus/evidence/task-8-architecture-overview.txt

  Scenario: Create component documentation
    Tool: Write
    Steps: Create docs/architecture/components.md with component details
    Expected: Details each major component and its responsibilities
    Evidence: .sisyphus/evidence/task-8-architecture-components.txt

  Scenario: Create data flow documentation
    Tool: Write
    Steps: Create docs/architecture/data-flow.md with data flow
    Expected: Shows how data moves between components
    Evidence: .sisyphus/evidence/task-8-architecture-dataflow.txt
  ```

  **Commit**: YES | Message: `docs: add comprehensive architecture documentation` | Files: [docs/architecture/]

- [ ] 9. Create UI wiring documentation

  **What to do**: Create detailed UI wiring documentation in `docs/ui/` covering React components, state management, and user interactions.
  **Must NOT do**: Don't include frontend frameworks not actually used (e.g., no Tailwind since it's not implemented).

  **Recommended Agent Profile**:
  - Category: `visual-engineering` — Reason: Frontend/UI documentation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper documentation structure
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Documentation task

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: [15] | Blocked By: [6, 8]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Current web uses Pages Router (not App Router) with single global CSS
  - External: Web structure from web/AGENTS.md and actual web/src/ structure
  - Test: Component interactions from web/src/pages/ and web/src/components/

  **Acceptance Criteria** (agent-executable only):
  - [ ] `docs/ui/component-tree.md` exists with component hierarchy
  - [ ] `docs/ui/state-management.md` exists with state flow
  - [ ] `docs/ui/user-interactions.md` exists with interaction patterns

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create UI component tree
    Tool: Write
    Steps: Create docs/ui/component-tree.md with component hierarchy
    Expected: Shows Pages Router structure and component relationships
    Evidence: .sisyphus/evidence/task-9-ui-components.txt

  Scenario: Create state management documentation
    Tool: Write
    Steps: Create docs/ui/state-management.md with state flow
    Expected: Details how state is managed across components
    Evidence: .sisyphus/evidence/task-9-ui-state.txt

  Scenario: Create user interaction documentation
    Tool: Write
    Steps: Create docs/ui/user-interactions.md with interaction patterns
    Expected: Documents key user flows and interactions
    Evidence: .sisyphus/evidence/task-9-ui-interactions.txt
  ```

  **Commit**: YES | Message: `docs: add comprehensive UI wiring documentation` | Files: [docs/ui/]

- [ ] 10. Create middleware wiring documentation

  **What to do**: Create detailed middleware documentation in `docs/middleware/` covering HTTP handlers, request/response flow, and middleware components.
  **Must NOT do**: Don't document middleware that doesn't exist.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: Backend documentation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper documentation structure
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Documentation task

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: [15] | Blocked By: [7, 8]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Current middleware in internal/cmd/ with CORS, settings, API routes
  - External: HTTP server structure from internal/cmd/AGENTS.md
  - API/Type: Route handlers and middleware from internal/cmd/root.go

  **Acceptance Criteria** (agent-executable only):
  - [ ] `docs/middleware/request-flow.md` exists with request processing flow
  - [ ] `docs/middleware/handlers.md` exists with handler documentation
  - [ ] `docs/middleware/error-handling.md` exists with error flow

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create middleware request flow
    Tool: Write
    Steps: Create docs/middleware/request-flow.md with request processing
    Expected: Shows how requests flow through middleware stack
    Evidence: .sisyphus/evidence/task-10-middleware-request.txt

  Scenario: Create handler documentation
    Tool: Write
    Steps: Create docs/middleware/handlers.md with handler details
    Expected: Documents each API handler and its responsibilities
    Evidence: .sisyphus/evidence/task-10-middleware-handlers.txt

  Scenario: Create error handling documentation
    Tool: Write
    Steps: Create docs/middleware/error-handling.md with error flow
    Expected: Details error handling and response formatting
    Evidence: .sisyphus/evidence/task-10-middleware-errors.txt
  ```

  **Commit**: YES | Message: `docs: add comprehensive middleware documentation` | Files: [docs/middleware/]

- [ ] 11. Create database schema documentation

  **What to do**: Create detailed database schema documentation in `docs/database/` covering SQLite tables, migrations, and data models.
  **Must NOT do**: Don't document tables that don't exist.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: Database documentation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper documentation structure
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Documentation task

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: [15] | Blocked By: [7, 8]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Current SQLite schema from internal/cache/sqlite.go and migrations
  - External: Required tables from AGENTS.md: pull_requests, pr_files, pr_reviews, ci_status, sync_jobs, sync_progress, merged_pr_index
  - API/Type: Data models from internal/types/models.go

  **Acceptance Criteria** (agent-executable only):
  - [ ] `docs/database/schema.md` exists with table definitions
  - [ ] `docs/database/migrations.md` exists with migration policy
  - [ ] `docs/database/relationships.md` exists with table relationships

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create database schema documentation
    Tool: Write
    Steps: Create docs/database/schema.md with table definitions
    Expected: Documents all required tables and their columns
    Evidence: .sisyphus/evidence/task-11-database-schema.txt

  Scenario: Create migration documentation
    Tool: Write
    Steps: Create docs/database/migrations.md with migration policy
    Expected: Details forward-only migration policy and schema_migrations table
    Evidence: .sisyphus/evidence/task-11-database-migrations.txt

  Scenario: Create relationship documentation
    Tool: Write
    Steps: Create docs/database/relationships.md with table relationships
    Expected: Shows how tables relate to each other
    Evidence: .sisyphus/evidence/task-11-database-relationships.txt
  ```

  **Commit**: YES | Message: `docs: add comprehensive database schema documentation` | Files: [docs/database/]

- [ ] 12. Create API contract documentation

  **What to do**: Create detailed API contract documentation in `docs/api/` covering all endpoints, request/response formats, and error handling.
  **Must NOT do**: Don't document endpoints that don't exist.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: API documentation
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper API documentation structure
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Documentation task

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: [15] | Blocked By: [7, 8]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Current API routes from internal/cmd/AGENTS.md
  - External: API contracts from contracts/ directory JSON schemas
  - API/Type: Response types from internal/types/models.go

  **Acceptance Criteria** (agent-executable only):
  - [ ] `docs/api/endpoints.md` exists with all API endpoints
  - [ ] `docs/api/request-formats.md` exists with request schemas
  - [ ] `docs/api/response-formats.md` exists with response schemas
  - [ ] `docs/api/error-handling.md` exists with error formats

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Create API endpoints documentation
    Tool: Write
    Steps: Create docs/api/endpoints.md with all endpoints
    Expected: Documents /healthz, /api/settings, /api/repos/{o}/{n}/analyze, etc.
    Evidence: .sisyphus/evidence/task-12-api-endpoints.txt

  Scenario: Create request format documentation
    Tool: Write
    Steps: Create docs/api/request-formats.md with request schemas
    Expected: Documents query parameters and request bodies
    Evidence: .sisyphus/evidence/task-12-api-requests.txt

  Scenario: Create response format documentation
    Tool: Write
    Steps: Create docs/api/response-formats.md with response schemas
    Expected: Documents all response types and their fields
    Evidence: .sisyphus/evidence/task-12-api-responses.txt

  Scenario: Create error handling documentation
    Tool: Write
    Steps: Create docs/api/error-handling.md with error formats
    Expected: Documents error response format {"error": "...", "message": "...", "status": "..."}
    Evidence: .sisyphus/evidence/task-12-api-errors.txt
  ```

  **Commit**: YES | Message: `docs: add comprehensive API contract documentation` | Files: [docs/api/]

- [ ] 13. Ensure AI-friendly documentation structure

  **What to do**: Ensure all documentation follows AI-friendly patterns with clear hierarchies, explicit parameter definitions, and structured markdown.
  **Must NOT do**: Don't use HTML or non-markdown formats.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: Documentation quality assurance
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures AI-friendly documentation standards
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Documentation review

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: [15] | Blocked By: [8, 9, 10, 11, 12]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: Follow n8n and Vercel Next.js AGENTS.md examples
  - External: 2026 AI-friendly documentation best practices (structured markdown, clear hierarchies)
  - Test: Ensure all documentation uses consistent heading levels and table formats

  **Acceptance Criteria** (agent-executable only):
  - [ ] All documentation files use structured markdown with clear H1/H2/H3 hierarchies
  - [ ] All API documentation uses tables for parameters and responses
  - [ ] All documentation includes negative rules ("Do NOT...") where applicable

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Verify markdown structure
    Tool: Bash
    Steps: Check all .md files for proper heading hierarchy
    Expected: Consistent H1→H2→H3 structure throughout
    Evidence: .sisyphus/evidence/task-13-markdown-structure.txt

  Scenario: Verify API parameter tables
    Tool: Bash
    Steps: Check API docs for parameter/response tables
    Expected: All parameters and responses documented in tables
    Evidence: .sisyphus/evidence/task-13-api-tables.txt

  Scenario: Verify negative rules
    Tool: Bash
    Steps: Check documentation for "Do NOT" or "MUST NOT" statements
    Expected: Clear guidance on what to avoid
    Evidence: .sisyphus/evidence/task-13-negative-rules.txt
  ```

  **Commit**: YES | Message: `docs: ensure AI-friendly documentation structure` | Files: [docs/]

- [ ] 14. Replace OpenRouter references with MiniMax

  **What to do**: Replace all OpenRouter references outside of fixture files with "MiniMax" as specified.
  **Must NOT do**: Don't modify fixture files containing legitimate OpenRouter mentions.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: Text replacement task
  - Skills: [`superpowers/writing-plans`] — Why needed: Ensures proper replacement scope
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Simple text replacement

  **Parallelization**: Can Parallel: YES | Wave 5 | Blocks: [15] | Blocked By: [7, 8, 9, 10, 11, 12, 13]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: User request specifies "Replace with MiniMax"
  - External: Current OpenRouter references in pratc.md, prATC _ App Architecture Plan.md, README.md
  - Test: Exclude fixtures/ directory from replacements

  **Acceptance Criteria** (agent-executable only):
  - [ ] `grep -r "openrouter" --exclude-dir=fixtures .` returns no results
  - [ ] All replacements use "MiniMax" (proper capitalization)

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Replace OpenRouter references
    Tool: Bash
    Steps: Replace openrouter/openrouter-light with MiniMax in non-fixture files
    Expected: All references updated to MiniMax
    Evidence: .sisyphus/evidence/task-14-replace-openrouter.txt

  Scenario: Verify no OpenRouter references remain
    Tool: Bash
    Steps: grep -r "openrouter" --exclude-dir=fixtures .
    Expected: No output (no matches found)
    Evidence: .sisyphus/evidence/task-14-verify-cleanup.txt
  ```

  **Commit**: YES | Message: `chore: replace OpenRouter references with MiniMax` | Files: [various .md files]

- [ ] 15. Final workspace verification

  **What to do**: Verify that the entire workspace is clean, compilable, and all tests pass. Ensure all deliverables are present and correct.
  **Must NOT do**: Don't make any additional changes - only verification.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: Comprehensive verification task
  - Skills: [`superpowers/verification-before-completion`, `superpowers/writing-plans`] — Why needed: Thorough verification discipline
  - Omitted: [`superpowers/test-driven-development`] — Why not needed: Verification phase

  **Parallelization**: Can Parallel: NO | Wave 5 | Blocks: [F1, F2, F3, F4] | Blocked By: [8, 9, 10, 11, 12, 13, 14]

  **References** (executor has NO interview context — be exhaustive):
  - Pattern: All Definition of Done criteria from plan
  - External: `make build && make test` should succeed completely
  - Test: `git status` should show clean working tree

  **Acceptance Criteria** (agent-executable only):
  - [ ] `make build` compiles successfully with binary at `./bin/pratc`
  - [ ] `make test` exits with code 0 (all tests pass)
  - [ ] `git status` shows clean working tree (no uncommitted changes)
  - [ ] All documentation directories exist and contain expected files

  **QA Scenarios** (MANDATORY — task incomplete without these):
  ```
  Scenario: Verify clean compilation
    Tool: Bash
    Steps: make build
    Expected: Exits with code 0, creates ./bin/pratc
    Evidence: .sisyphus/evidence/task-15-build-verify.txt

  Scenario: Verify all tests pass
    Tool: Bash
    Steps: make test
    Expected: Exits with code 0, all tests pass
    Evidence: .sisyphus/evidence/task-15-test-verify.txt

  Scenario: Verify clean git status
    Tool: Bash
    Steps: git status --porcelain
    Expected: No output (clean working tree)
    Evidence: .sisyphus/evidence/task-15-git-verify.txt

  Scenario: Verify documentation completeness
    Tool: Bash
    Steps: ls docs/
    Expected: Shows architecture/, ui/, middleware/, database/, api/ directories
    Evidence: .sisyphus/evidence/task-15-docs-verify.txt
  ```

  **Commit**: NO | Message: `N/A` | Files: []

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep
## Commit Strategy
- Each logical group of changes gets its own atomic commit
- Commit messages follow conventional commits format
- All commits reference the specific task they implement
- Final verification does not create a commit (only verification)

## Success Criteria
- All TODOs completed with passing acceptance criteria
- Final Verification Wave passes all 4 reviews
- User explicitly approves the final result
- Workspace is clean, compilable, well-documented, and ready for use