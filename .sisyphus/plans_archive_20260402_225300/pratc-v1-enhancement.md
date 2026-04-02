# prATC Enhancement Plan v1.0

## TL;DR

> **Quick Summary**: Implement GitHub token configuration command, set version to 1.0, add startup banner with author attribution, fix max-prs bug, enhance logger with DEBUG/WARN levels, and improve PR batch processing with proper git clone support.
> 
> **Deliverables**:
> - New `config` CLI command for secure GitHub token management
> - Version 1.0 with startup banner: "Github Pull Request Air Traffic Control v1.0 {DATE TIME STAMP} by Jefferson Nunn"
> - Fixed max-prs functionality (unlimited when -1 specified)
> - Enhanced logger with DEBUG/WARN methods
> - Improved PR batch processing with git clone support for main branch and full PR data
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 8 waves
> **Critical Path**: Config command → Token integration → Max-PRS fix → Batch processing

---

## Context

### Original Request
User wants to:
1. Setup 'config' command to import GitHub token (not auto-detect)
2. Set version to 1.0 
3. Add startup banner with "Github Pull Request Air Traffic Control v1.0 {DATE TIME STAMP} by Jefferson Nunn" + config location
4. Fix PR batch processing to pull in batches then run omni mode
5. Fix logger missing DEBUG/WARN levels (DEBUG=ERROR for now)
6. Review logs and make suggestions

### Key Findings from Exploration
- **Current state**: No config command, no version info, logger only has INFO/ERROR
- **Max-PRS bug**: `--max-prs=-1` defaults to 1000 instead of unlimited due to logic error in app/service.go
- **PR fetching**: Sequential GraphQL queries, no proper git clone support for PR content
- **Git operations**: Mirror exists but not integrated with analyze/cluster commands
- **Token handling**: Currently reads GITHUB_PAT/GH_TOKEN env vars, references psst pattern but doesn't implement it

### Research Insights
- Settings store uses SQLite at `pratc-settings.db` with global/repo scopes
- Command structure follows minimal stub pattern in cmd/pratc/ + registration in internal/cmd/root.go
- Logger uses Go slog with JSON output to stderr
- Omni mode exists as API endpoint but requires pre-synced PRs
- PR data structure includes body, title, files_changed (enriched separately)

---

## Work Objectives

### Core Objective
Transform prATC into a production-ready v1.0 system with proper configuration management, enhanced logging, fixed batch processing, and professional startup experience.

### Concrete Deliverables
- `pratc config` command with get/set/list/delete/export/import subcommands
- Version 1.0.0 with startup banner showing author and timestamp
- Fixed max-prs functionality working correctly with unlimited option
- Logger with Debug() and Warn() methods
- PR batch processing that clones git repos and fetches complete PR data including descriptions and code
- Integration between config command and GitHub client

### Definition of Done
- [ ] All commands work as documented
- [ ] `pratc config set --scope global github_token "token"` stores securely
- [ ] Startup banner displays correctly on every command
- [ ] `--max-prs=-1` fetches unlimited PRs without truncation
- [ ] Logger.Debug() and Logger.Warn() methods exist and work
- [ ] Git clone works for main branch and PR refs
- [ ] All tests pass: `make test`

### Must Have
- Secure GitHub token storage via config command
- Version 1.0 startup banner with Jefferson Nunn attribution
- Fixed max-prs behavior
- Complete PR data including body/description and file changes
- Backward compatibility with existing commands

### Must NOT Have (Guardrails)
- No automatic GitHub token detection - must use config command explicitly
- No breaking changes to existing CLI/API contracts
- No unencrypted token storage in plain text files
- No removal of existing INFO/ERROR logger methods
- No changes to core analysis logic beyond bug fixes

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go tests, Makefile)
- **Automated tests**: YES (Tests-after)
- **Framework**: go test -race -v ./...
- **If TDD**: Each task follows RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios with evidence capture.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation):
├── Task 1: Version and startup banner setup [quick]
├── Task 2: Logger enhancement with DEBUG/WARN [quick]
├── Task 3: Settings validator update for github_token [quick]
├── Task 4: Config command stub creation [quick]

Wave 2 (After Wave 1 — core config implementation):
├── Task 5: Config command implementation with subcommands [deep]
├── Task 6: GitHub token integration in service [deep]
├── Task 7: Max-PRS bug fix [quick]
├── Task 8: Config location display in startup banner [quick]

Wave 3 (After Wave 2 — PR batch processing):
├── Task 9: Git clone integration for main branch [deep]
├── Task 10: PR enrichment with complete data (body, files) [deep]
├── Task 11: Batch processing optimization [unspecified-high]
├── Task 12: Omni mode documentation and examples [writing]

Wave 4 (After Wave 3 — testing and validation):
├── Task 13: Comprehensive test suite for new features [unspecified-high]
├── Task 14: Integration testing with OpenClaw repo [unspecified-high]
├── Task 15: Documentation updates [writing]
└── Task 16: Build and release preparation [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay
```

### Dependency Matrix
- **1-4**: Independent foundation tasks
- **5**: Depends on 3, 4 (validator + stub)
- **6**: Depends on 5 (config command)
- **7**: Independent bug fix
- **8**: Depends on 1 (startup banner)
- **9-10**: Independent PR processing improvements
- **11**: Depends on 9, 10 (batch optimization)
- **12**: Independent documentation
- **13-14**: Depend on all implementation tasks
- **15**: Depends on all features
- **16**: Depends on version setup

---

## TODOs

- [x] 1. Version and startup banner setup

  **What to do**:
  - Create internal/version.go with Version = "1.0.0"
  - Modify ExecuteContext in root.go to print startup banner
  - Banner format: "Github Pull Request Air Traffic Control v1.0 {DATE TIME STAMP} by Jefferson Nunn"
  - Add config location display showing settings and cache paths

  **Must NOT do**:
  - Don't break existing command functionality
  - Don't add version flag yet (separate task if needed)

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Simple file creation and minor modifications to existing functions
  - **Skills**: [`git-master`]
    - `git-master`: For atomic commits and proper git workflow

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: Task 8 (config location display depends on this)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `cmd/pratc/main.go:12-17` - Main entry point calling ExecuteContext
  - `internal/cmd/root.go:43-56` - ExecuteContext function structure

  **API/Type References**:
  - `internal/types/models.go:4-27` - PR struct showing current data fields

  **WHY Each Reference Matters**:
  - Need to understand where to insert banner (ExecuteContext before rootCmd.ExecuteContext)
  - Understand current PR data structure to know what fields are available vs need enrichment

  **Acceptance Criteria**:
  - [ ] Version constant created in internal/version.go
  - [ ] Startup banner prints on every command execution
  - [ ] Banner includes correct timestamp format (RFC3339)
  - [ ] Banner shows Jefferson Nunn attribution exactly as specified

  **QA Scenarios**:
  ```
  Scenario: Startup banner displays correctly
    Tool: Bash
    Preconditions: Built pratc binary
    Steps:
      1. Run "/home/agent/pratc/bin/pratc --help"
      2. Capture stdout/stderr
      3. Assert output contains "Github Pull Request Air Traffic Control v1.0"
      4. Assert output contains "by Jefferson Nunn"
      5. Assert timestamp is in RFC3339 format
    Expected Result: Banner appears before help text
    Failure Indicators: Banner missing, wrong version, missing attribution
    Evidence: .sisyphus/evidence/task-1-banner.txt

  Scenario: Banner shows config locations
    Tool: Bash
    Preconditions: PRATC_SETTINGS_DB and PRATC_DB_PATH not set
    Steps:
      1. Run "/home/agent/pratc/bin/pratc analyze --repo=test/test --format=json"
      2. Capture stderr output
      3. Assert output contains "Using Config from:" with correct paths
    Expected Result: Shows default config paths
    Evidence: .sisyphus/evidence/task-1-config-locations.txt
  ```

  **Evidence to Capture**:
  - [ ] task-1-banner.txt - Banner output verification
  - [ ] task-1-config-locations.txt - Config path display

  **Commit**: YES (groups with 1)
  - Message: `feat(version): add v1.0 startup banner with author attribution`
  - Files: internal/version.go, internal/cmd/root.go
  - Pre-commit: make test

---

- [x] 2. Logger enhancement with DEBUG/WARN

  **What to do**:
  - Add Debug() and Warn() methods to Logger struct in logger.go
  - Maintain backward compatibility with existing Info() and Error() methods
  - Ensure DEBUG=ERROR behavior as requested (debug logs treated as errors initially)

  **Must NOT do**:
  - Don't break existing logger usage
  - Don't change log format or output destination

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Adding wrapper methods around existing slog.Logger
  - **Skills**: [`git-master`]
    - `git-master`: For precise method additions and testing

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: None directly, but enables better debugging in other tasks
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `internal/logger/logger.go:138-146` - Current Info() and Error() method implementations
  - `internal/logger/logger.go:29-33` - Logger struct definition

  **WHY Each Reference Matters**:
  - Need to follow exact same pattern as existing methods
  - Understand Logger struct to add methods correctly

  **Acceptance Criteria**:
  - [ ] Logger.Debug() method exists and compiles
  - [ ] Logger.Warn() method exists and compiles
  - [ ] Existing Info() and Error() methods unchanged
  - [ ] Debug logs output as ERROR level (DEBUG=ERROR requirement)

  **QA Scenarios**:
  ```
  Scenario: Logger.Debug and Logger.Warn methods work
    Tool: Bash
    Preconditions: Modified logger package
    Steps:
      1. Create test file that calls logger.New("test").Debug("debug msg")
      2. Create test file that calls logger.New("test").Warn("warn msg")  
      3. Compile and run test files
      4. Verify debug messages appear as ERROR level in output
    Expected Result: Both methods compile and execute without errors
    Failure Indicators: Compilation errors, runtime panics, wrong log levels
    Evidence: .sisyphus/evidence/task-2-logger-methods.txt

  Scenario: Backward compatibility maintained
    Tool: Bash
    Preconditions: Existing code using logger
    Steps:
      1. Run "make test" to ensure all existing tests pass
      2. Verify no logger-related test failures
    Expected Result: All tests pass as before
    Evidence: .sisyphus/evidence/task-2-backward-compat.txt
  ```

  **Evidence to Capture**:
  - [ ] task-2-logger-methods.txt - Method functionality verification
  - [ ] task-2-backward-compat.txt - Test compatibility verification

  **Commit**: YES (groups with 2)
  - Message: `feat(logger): add Debug and Warn methods with DEBUG=ERROR`
  - Files: internal/logger/logger.go
  - Pre-commit: make test

---

- [x] 3. Settings validator update for github_token

  **What to do**:
  - Add "github_token" to allowedKeys map in internal/settings/validator.go
  - Ensure token can be stored at global scope
  - Validate that token format matches GitHub PAT pattern

  **Must NOT do**:
  - Don't allow github_token at repo scope (security concern)
  - Don't store actual tokens in version control

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Simple addition to existing allowed keys map
  - **Skills**: [`git-master`]
    - `git-master`: For precise edits to validator

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Task 5 (config command needs this validation)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `internal/settings/validator.go:15-21` - Current allowedKeys map structure
  - `internal/settings/validator.go:23-35` - validateKey function logic

  **WHY Each Reference Matters**:
  - Need to follow exact same pattern as existing allowed keys
  - Understand validation logic to ensure proper scope handling

  **Acceptance Criteria**:
  - [ ] "github_token" added to allowedKeys map
  - [ ] Token can be set at global scope
  - [ ] Token rejected at repo scope (if implemented)
  - [ ] Validation passes for valid GitHub token format

  **QA Scenarios**:
  ```
  Scenario: github_token allowed in settings
    Tool: Bash
    Preconditions: Modified validator
    Steps:
      1. Run "go test ./internal/settings -v" 
      2. Verify no validation test failures
      3. Create test that validates github_token key
    Expected Result: github_token passes validation at global scope
    Failure Indicators: Validation errors, test failures
    Evidence: .sisyphus/evidence/task-3-validator.txt

  Scenario: Token format validation
    Tool: Bash
    Preconditions: Validator with github_token support
    Steps:
      1. Test with valid GitHub token format (ghp_...)
      2. Test with invalid format
      3. Verify validation behaves correctly
    Expected Result: Valid tokens accepted, invalid rejected
    Evidence: .sisyphus/evidence/task-3-token-validation.txt
  ```

  **Evidence to Capture**:
  - [ ] task-3-validator.txt - Validation test results
  - [ ] task-3-token-validation.txt - Token format validation

  **Commit**: YES (groups with 3)
  - Message: `feat(settings): allow github_token in global settings`
  - Files: internal/settings/validator.go
  - Pre-commit: make test

---

- [x] 4. Config command stub creation

  **What to do**:
  - Create cmd/pratc/config.go with init() function
  - Call cmd.RegisterConfigCommand() in init()
  - Ensure proper command registration pattern

  **Must NOT do**:
  - Don't implement actual command logic yet (separate task)
  - Don't break existing command registration

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Following established command stub pattern
  - **Skills**: [`git-master`]
    - `git-master`: For proper file creation and imports

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: Task 5 (config command implementation)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `cmd/pratc/analyze.go:1-6` - Command stub pattern example
  - `cmd/pratc/plan.go:1-6` - Another command stub example
  - `internal/cmd/root.go:1500+` - Register*Command function patterns

  **WHY Each Reference Matters**:
  - Must follow exact same pattern as existing commands
  - Understand import statements and function naming conventions

  **Acceptance Criteria**:
  - [ ] config.go file created with correct init() function
  - [ ] Proper import statement for internal/cmd package
  - [ ] Command appears in pratc --help output
  - [ ] No compilation errors

  **QA Scenarios**:
  ```
  Scenario: Config command stub registers correctly
    Tool: Bash
    Preconditions: Built pratc binary with config stub
    Steps:
      1. Run "/home/agent/pratc/bin/pratc --help"
      2. Verify "config" appears in available commands list
      3. Run "/home/agent/pratc/bin/pratc config --help"
      4. Verify command executes without "unknown command" error
    Expected Result: Config command recognized by CLI
    Failure Indicators: Unknown command error, compilation failures
    Evidence: .sisyphus/evidence/task-4-config-stub.txt
  ```

  **Evidence to Capture**:
  - [ ] task-4-config-stub.txt - Command registration verification

  **Commit**: YES (groups with 4)
  - Message: `feat(config): add config command stub`
  - Files: cmd/pratc/config.go
  - Pre-commit: make build

---

- [x] 5. Config command implementation with subcommands

  **What to do**:
  - Implement RegisterConfigCommand() in internal/cmd/root.go
  - Add subcommands: get, set, list, delete, export, import
  - Support --scope (global/repo) and --repo flags
  - Handle github_token specifically with secure storage
  - Follow mirror command structure as template

  **Must NOT do**:
  - Don't store tokens in plain text files
  - Don't break existing settings API

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `deep`
    - Reason: Complex command with multiple subcommands and validation logic
  - **Skills**: [`git-master`]
    - `git-master`: For complex command implementation and testing

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6, 7, 8)
  - **Blocks**: Task 6 (GitHub token integration)
  - **Blocked By**: Tasks 3, 4 (validator + stub)

  **References**:
  **Pattern References**:
  - `internal/cmd/root.go:1200-1300` - Mirror command implementation with subcommands
  - `internal/cmd/root.go:800-900` - Settings API implementation
  - `internal/settings/store.go:50-150` - Settings store interface methods

  **WHY Each Reference Matters**:
  - Mirror command shows exact subcommand pattern to follow
  - Settings API shows how to handle scope and repo parameters
  - Store interface shows available methods for get/set/list/delete

  **Acceptance Criteria**:
  - [ ] All subcommands work: get, set, list, delete, export, import
  - [ ] github_token can be set at global scope
  - [ ] Commands integrate with existing settings store
  - [ ] Proper error handling for invalid scopes/keys

  **QA Scenarios**:
  ```
  Scenario: Config set/get commands work
    Tool: Bash
    Preconditions: Built pratc with config command
    Steps:
      1. Run "pratc config set --scope global github_token test-token"
      2. Run "pratc config get --scope global github_token"
      3. Verify output returns "test-token"
      4. Run "pratc config list --scope global"
      5. Verify github_token appears in list
    Expected Result: Token stored and retrieved correctly
    Failure Indicators: Command errors, wrong output, missing keys
    Evidence: .sisyphus/evidence/task-5-config-commands.txt

  Scenario: Scope validation works
    Tool: Bash
    Preconditions: Config command implemented
    Steps:
      1. Run "pratc config set --scope repo --repo=test/test github_token token"
      2. Verify command rejects github_token at repo scope (security)
      3. Run "pratc config set --scope global invalid_key value"
      4. Verify command rejects invalid key
    Expected Result: Proper validation and error messages
    Evidence: .sisyphus/evidence/task-5-scope-validation.txt
  ```

  **Evidence to Capture**:
  - [ ] task-5-config-commands.txt - Command functionality
  - [ ] task-5-scope-validation.txt - Validation behavior

  **Commit**: YES (groups with 5)
  - Message: `feat(config): implement full config command with subcommands`
  - Files: internal/cmd/root.go
  - Pre-commit: make test

---

- [x] 6. GitHub token integration in service

  **What to do**:
  - Modify app/service.go to read github_token from settings store
  - Update GitHub client initialization to use stored token
  - Maintain backward compatibility with GITHUB_PAT/GH_TOKEN env vars
  - Add token priority: config store → GITHUB_PAT → GH_TOKEN → gh auth

  **Must NOT do**:
  - Don't break existing token resolution logic
  - Don't expose tokens in logs or error messages

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `deep`
    - Reason: Core service modification affecting authentication flow
  - **Skills**: [`git-master`]
    - `git-master`: For service-level changes and integration testing

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 7, 8)
  - **Blocks**: Task 14 (integration testing)
  - **Blocked By**: Task 5 (config command)

  **References**:
  **Pattern References**:
  - `internal/app/service.go:74-102` - Current token resolution logic
  - `internal/app/service.go:635` - Error message mentioning psst GITHUB_PAT
  - `internal/github/client.go:269` - Token usage in HTTP header

  **WHY Each Reference Matters**:
  - Need to understand current token flow to extend properly
  - Error message shows expected user guidance pattern
  - Client shows how token is actually used

  **Acceptance Criteria**:
  - [ ] Token from config store used when available
  - [ ] Backward compatibility maintained with env vars
  - [ ] Proper error messages when no token available
  - [ ] No token leakage in logs or errors

  **QA Scenarios**:
  ```
  Scenario: Config token takes precedence
    Tool: Bash
    Preconditions: Token set via config command, GITHUB_PAT also set
    Steps:
      1. Set token via "pratc config set --scope global github_token config-token"
      2. Set GITHUB_PAT=env-token
      3. Run "pratc analyze --repo=test/test --force-live"
      4. Verify config-token is used (mock GitHub API to verify)
    Expected Result: Config store token takes precedence over env var
    Failure Indicators: Wrong token used, authentication failures
    Evidence: .sisyphus/evidence/task-6-token-priority.txt

  Scenario: Backward compatibility maintained
    Tool: Bash
    Preconditions: No config token, only GITHUB_PAT set
    Steps:
      1. Ensure no github_token in config store
      2. Set GITHUB_PAT=env-token
      3. Run analyze command
      4. Verify it works as before
    Expected Result: Env var token still works
    Evidence: .sisyphus/evidence/task-6-backward-compat.txt
  ```

  **Evidence to Capture**:
  - [ ] task-6-token-priority.txt - Token precedence verification
  - [ ] task-6-backward-compat.txt - Compatibility verification

  **Commit**: YES (groups with 6)
  - Message: `feat(auth): integrate config command token with GitHub client`
  - Files: internal/app/service.go
  - Pre-commit: make test

---

- [x] 7. Max-PRS bug fix

  **What to do**:
  - Fix logic in app/service.go where maxPRs < 0 defaults to 1000
  - Change to: maxPRs == 0 means unlimited, maxPRs > 0 means cap
  - Update CLI flag documentation to reflect correct behavior
  - Ensure --max-prs=-1 works as unlimited

  **Must NOT do**:
  - Don't break existing behavior for positive max_prs values
  - Don't change API contract for max_prs parameter

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Simple logic fix in existing code
  - **Skills**: [`git-master`]
    - `git-master`: For precise bug fix and testing

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 8)
  - **Blocks**: None directly
  - **Blocked By**: None (independent bug fix)

  **References**:
  **Pattern References**:
  - `internal/app/service.go:106-109` - Current maxPRs default logic (bug location)
  - `internal/app/service.go:846-851` - applyIntakeControls truncation logic
  - `cmd/pratc/analyze.go:25` - CLI flag help text

  **WHY Each Reference Matters**:
  - Bug is in line 107: if maxPRs < 0 → sets to 1000
  - Need to update both service logic and CLI documentation
  - Truncation logic needs to handle unlimited case correctly

  **Acceptance Criteria**:
  - [ ] --max-prs=-1 fetches unlimited PRs without truncation
  - [ ] --max-prs=0 also works as unlimited (consistent API)
  - [ ] Positive values still cap correctly
  - [ ] CLI help text updated to reflect correct behavior

  **QA Scenarios**:
  ```
  Scenario: Max-PRS unlimited works
    Tool: Bash
    Preconditions: Small test repo with known PR count
    Steps:
      1. Run "pratc analyze --repo=test/test --max-prs=-1 --force-live"
      2. Count actual PRs returned vs expected
      3. Verify no truncation occurs
      4. Repeat with --max-prs=0
    Expected Result: All PRs returned, no truncation
    Failure Indicators: Truncation to 1000 PRs, wrong counts
    Evidence: .sisyphus/evidence/task-7-max-prs-unlimited.txt

  Scenario: Positive max_prs still works
    Tool: Bash
    Preconditions: Test repo with >10 PRs
    Steps:
      1. Run "pratc analyze --repo=test/test --max-prs=5 --force-live"
      2. Verify exactly 5 PRs returned
      3. Verify truncation metadata set correctly
    Expected Result: Correct capping behavior maintained
    Evidence: .sisyphus/evidence/task-7-max-prs-capped.txt
  ```

  **Evidence to Capture**:
  - [ ] task-7-max-prs-unlimited.txt - Unlimited behavior verification
  - [ ] task-7-max-prs-capped.txt - Capped behavior verification

  **Commit**: YES (groups with 7)
  - Message: `fix(analyze): fix max-prs bug, -1 now means unlimited`
  - Files: internal/app/service.go, cmd/pratc/analyze.go
  - Pre-commit: make test

---

- [x] 8. Config location display in startup banner

  **What to do**:
  - Enhance startup banner to show config file locations
  - Display PRATC_SETTINGS_DB path (settings) and PRATC_DB_PATH (cache)
  - Show actual paths being used (env var or default)
  - Format: "Using Config from: <settings_path> | <cache_path>"

  **Must NOT do**:
  - Don't expose sensitive information in paths
  - Don't break existing banner functionality

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Simple string formatting addition to existing banner
  - **Skills**: [`git-master`]
    - `git-master`: For precise banner enhancement

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7)
  - **Blocks**: None
  - **Blocked By**: Task 1 (startup banner setup)

  **References**:
  **Pattern References**:
  - `internal/cmd/root.go:81-88` - analyzeSyncDBPath function showing cache path logic
  - `internal/cmd/root.go:741` - openSettingsStore showing settings path logic
  - `internal/cmd/root.go:43-56` - ExecuteContext where banner should be added

  **WHY Each Reference Matters**:
  - Need to reuse existing path determination logic
  - Banner should be added in ExecuteContext before command execution
  - Paths should reflect actual runtime configuration

  **Acceptance Criteria**:
  - [ ] Banner shows both settings and cache paths
  - [ ] Paths reflect env vars when set, defaults when not
  - [ ] Format matches specification: "Using Config from: ..."
  - [ ] No path exposure issues (no tokens/secrets in paths)

  **QA Scenarios**:
  ```
  Scenario: Config paths displayed correctly
    Tool: Bash
    Preconditions: Default configuration (no env vars set)
    Steps:
      1. Run any pratc command (e.g., "pratc --help")
      2. Verify banner shows "Using Config from:" with correct default paths
      3. Set PRATC_SETTINGS_DB=/tmp/test.db
      4. Run command again
      5. Verify custom path shown
    Expected Result: Correct paths displayed based on configuration
    Failure Indicators: Wrong paths, missing paths, path exposure
    Evidence: .sisyphus/evidence/task-8-config-paths.txt
  ```

  **Evidence to Capture**:
  - [ ] task-8-config-paths.txt - Path display verification

  **Commit**: YES (groups with 8)
  - Message: `feat(config): display config file locations in startup banner`
  - Files: internal/cmd/root.go
  - Pre-commit: make test

---

- [x] 9. Git clone integration for main branch

  **What to do**:
  - Integrate repo/mirror.go git operations with analyze/cluster commands
  - Automatically clone main branch when analyzing repository
  - Fetch PR refs for open PRs during sync
  - Enable git-based file change detection

  **Must NOT do**:
  - Don't break existing non-git analysis mode
  - Don't require git for basic analysis

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `deep`
    - Reason: Complex integration between git operations and analysis pipeline
  - **Skills**: [`git-master`]
    - `git-master`: For git operations and integration

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 11, 12)
  - **Blocks**: Task 10 (PR enrichment)
  - **Blocked By**: None (can start after foundation)

  **References**:
  **Pattern References**:
  - `internal/repo/mirror.go:50-150` - Git mirror operations (OpenOrCreate, FetchAll)
  - `internal/sync/default_runner.go:80-120` - Sync runner using mirror
  - `internal/app/service.go:300-400` - Current analysis pipeline

  **WHY Each Reference Matters**:
  - Mirror already has all needed git operations
  - Sync runner shows how to integrate mirror with analysis
  - Need to understand current analysis flow to add git integration

  **Acceptance Criteria**:
  - [ ] Main branch cloned automatically during analysis
  - [ ] PR refs fetched for open PRs
  - [ ] Git-based file change detection enabled
  - [ ] Non-git mode still works as fallback

  **QA Scenarios**:
  ```
  Scenario: Git clone works for analysis
    Tool: Bash
    Preconditions: Valid GitHub repository
    Steps:
      1. Run "pratc analyze --repo=owner/repo --force-live"
      2. Verify .pratc/git/owner/repo directory created
      3. Verify main branch and PR refs fetched
      4. Verify analysis completes successfully
    Expected Result: Git operations succeed, analysis enhanced
    Failure Indicators: Git errors, failed analysis, missing refs
    Evidence: .sisyphus/evidence/task-9-git-clone.txt

  Scenario: Fallback to non-git mode
    Tool: Bash
    Preconditions: Repository with git disabled (mock)
    Steps:
      1. Mock git operations to fail
      2. Run analyze command
      3. Verify analysis still works with GraphQL-only data
    Expected Result: Graceful fallback to existing behavior
    Evidence: .sisyphus/evidence/task-9-git-fallback.txt
  ```

  **Evidence to Capture**:
  - [ ] task-9-git-clone.txt - Git clone verification
  - [ ] task-9-git-fallback.txt - Fallback verification

  **Commit**: YES (groups with 9)
  - Message: `feat(git): integrate git clone for main branch and PR refs`
  - Files: internal/app/service.go, internal/repo/mirror.go
  - Pre-commit: make test

---

- [x] 10. PR enrichment with complete data (body, files)

  **What to do**:
  - Enhance PR data to include full body/description and file changes
  - Use git diff for file changes when git clone available
  - Use GraphQL for body when git not available
  - Populate FilesChanged field in PR struct

  **Must NOT do**:
  - Don't break existing PR struct compatibility
  - Don't make enrichment mandatory (keep optional)

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `deep`
    - Reason: Data enrichment pipeline affecting core PR model
  - **Skills**: [`git-master`]
    - `git-master`: For git diff operations and data enrichment

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9, 11, 12)
  - **Blocks**: Task 11 (batch optimization)
  - **Blocked By**: Task 9 (git clone integration)

  **References**:
  **Pattern References**:
  - `internal/types/models.go:9` - PR.Body field (currently populated)
  - `internal/types/models.go:13` - PR.FilesChanged field (currently nil)
  - `internal/repo/mirror.go:200-250` - GetChangedFiles batch implementation
  - `internal/app/service.go:400-500` - enrichFromGraphQL function

  **WHY Each Reference Matters**:
  - PR struct already has fields, just need to populate them
  - Git mirror has batch file change detection ready to use
  - Existing enrichment function shows pattern to extend

  **Acceptance Criteria**:
  - [ ] PR.Body contains full description text
  - [ ] PR.FilesChanged contains actual changed files list
  - [ ] Git-based enrichment preferred when available
  - [ ] GraphQL fallback works when git not available

  **QA Scenarios**:
  ```
  Scenario: Complete PR data enriched
    Tool: Bash
    Preconditions: Repository with PRs containing descriptions and file changes
    Steps:
      1. Run "pratc analyze --repo=owner/repo --force-live"
      2. Examine JSON output for PR.Body field
      3. Examine PR.FilesChanged field
      4. Verify both contain actual data, not empty/nil
    Expected Result: Full PR data including description and file changes
    Failure Indicators: Empty body, nil files_changed, enrichment errors
    Evidence: .sisyphus/evidence/task-10-pr-enrichment.txt

  Scenario: Git vs GraphQL enrichment
    Tool: Bash
    Preconditions: Both git and GraphQL available
    Steps:
      1. Run analyze with git clone enabled
      2. Verify FilesChanged from git diff
      3. Disable git, run again
      4. Verify FilesChanged from GraphQL
    Expected Result: Appropriate enrichment method used based on availability
    Evidence: .sisyphus/evidence/task-10-enrichment-methods.txt
  ```

  **Evidence to Capture**:
  - [ ] task-10-pr-enrichment.txt - Data enrichment verification
  - [ ] task-10-enrichment-methods.txt - Method selection verification

  **Commit**: YES (groups with 10)
  - Message: `feat(pr): enrich PR data with full body and file changes`
  - Files: internal/app/service.go, internal/types/models.go
  - Pre-commit: make test

---

- [x] 11. Batch processing optimization

  **What to do**:
  - Optimize PR fetching with concurrent GraphQL queries
  - Increase git batch size beyond hardcoded 100
  - Implement proper cursor-based incremental sync
  - Add omni mode support for direct PR processing

  **Must NOT do**:
  - Don't break existing sequential processing
  - Don't exceed GitHub rate limits with concurrency

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `unspecified-high`
    - Reason: Performance optimization requiring careful rate limit management
  - **Skills**: [`git-master`]
    - `git-master`: For performance testing and optimization

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9, 10, 12)
  - **Blocks**: Task 14 (integration testing)
  - **Blocked By**: Tasks 9, 10 (git and enrichment)

  **References**:
  **Pattern References**:
  - `internal/github/client.go:100-200` - FetchPullRequests sequential pagination
  - `internal/repo/mirror.go:150-200` - FetchAllBatched with hardcoded 100
  - `internal/sync/default_runner.go:50-80` - SyncRepo without cursor support
  - `internal/app/stage_processor.go:30-80` - Omni batch processor

  **WHY Each Reference Matters**:
  - Current implementations are sequential or limited
  - Need to understand rate limiting to add safe concurrency
  - Omni processor shows batch processing patterns to extend

  **Acceptance Criteria**:
  - [ ] Concurrent PR fetching within rate limits
  - [ ] Configurable git batch size (not hardcoded 100)
  - [ ] Cursor-based incremental sync implemented
  - [ ] Omni mode works with enriched PR data

  **QA Scenarios**:
  ```
  Scenario: Batch processing performance improved
    Tool: Bash
    Preconditions: Large repository (>1000 PRs)
    Steps:
      1. Run analyze with old version, time execution
      2. Run analyze with new version, time execution  
      3. Compare performance improvement
      4. Verify no rate limit violations
    Expected Result: Significant performance improvement within rate limits
    Failure Indicators: Rate limit errors, slower performance, data corruption
    Evidence: .sisyphus/evidence/task-11-batch-performance.txt

  Scenario: Omni mode works with full data
    Tool: Interactive_bash
    Preconditions: Synced repository with enriched PRs
    Steps:
      1. Start tmux session
      2. Run "curl 'http://localhost:8080/api/repos/owner/repo/plan/omni?selector=1-100'"
      3. Verify response contains full PR data including body and files
      4. Verify batch processing works correctly
    Expected Result: Omni mode returns complete enriched data
    Evidence: .sisyphus/evidence/task-11-omni-full-data.txt
  ```

  **Evidence to Capture**:
  - [ ] task-11-batch-performance.txt - Performance verification
  - [ ] task-11-omni-full-data.txt - Omni mode verification

  **Commit**: YES (groups with 11)
  - Message: `feat(batch): optimize PR fetching and git operations with concurrency`
  - Files: internal/github/client.go, internal/repo/mirror.go, internal/sync/default_runner.go
  - Pre-commit: make test

---

- [x] 12. Omni mode documentation and examples

  **What to do**:
  - Document omni mode selector syntax and usage
  - Provide examples for common use cases (1-100, 50-150 AND 200-300)
  - Add CLI examples showing integration with config and analyze
  - Update README with v1.0 features

  **Must NOT do**:
  - Don't document incomplete or broken features
  - Don't provide incorrect selector syntax examples

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `writing`
    - Reason: Documentation and examples creation
  - **Skills**: []
    - No special skills needed for documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9, 10, 11)
  - **Blocks**: Task 15 (comprehensive documentation)
  - **Blocked By**: Task 11 (omni mode functionality)

  **References**:
  **Pattern References**:
  - `internal/planning/selector_parser.go:20-50` - Selector grammar rules
  - `internal/cmd/root.go:1400-1500` - handlePlanOmni API handler
  - `README.md:100-200` - Current documentation structure

  **WHY Each Reference Matters**:
  - Need accurate selector syntax from parser implementation
  - API handler shows actual endpoint behavior
  - README shows documentation style and structure

  **Acceptance Criteria**:
  - [ ] Omni mode selector syntax documented accurately
  - [ ] Working examples provided for common scenarios
  - [ ] Integration examples with config and analyze commands
  - [ ] README updated with v1.0 features

  **QA Scenarios**:
  ```
  Scenario: Documentation examples work
    Tool: Bash
    Preconditions: Built pratc with omni mode
    Steps:
      1. Copy example from documentation: "pratc config set..."
      2. Execute example commands
      3. Verify they work as documented
      4. Test selector examples via API
    Expected Result: All documented examples execute successfully
    Failure Indicators: Command errors, wrong syntax, broken examples
    Evidence: .sisyphus/evidence/task-12-doc-examples.txt
  ```

  **Evidence to Capture**:
  - [ ] task-12-doc-examples.txt - Example verification

  **Commit**: YES (groups with 12)
  - Message: `docs: add omni mode documentation and v1.0 examples`
  - Files: README.md, internal/cmd/root.go (API comments)
  - Pre-commit: none (documentation only)

---

- [x] 13. Comprehensive test suite for new features

  **What to do**:
  - Add unit tests for config command subcommands
  - Add integration tests for token priority and validation
  - Add tests for max-prs bug fix with various values
  - Add tests for logger Debug/Warn methods
  - Add tests for git clone and PR enrichment

  **Must NOT do**:
  - Don't skip testing edge cases
  - Don't break existing test suite

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `unspecified-high`
    - Reason: Comprehensive test coverage across multiple new features
  - **Skills**: [`git-master`]
    - `git-master`: For test creation and validation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 14, 15, 16)
  - **Blocks**: Final verification
  - **Blocked By**: All implementation tasks (1-12)

  **References**:
  **Pattern References**:
  - `internal/cmd/root_test.go:100-200` - Command testing patterns
  - `internal/app/service_test.go:50-150` - Service testing patterns
  - `internal/logger/logger_test.go:20-50` - Logger testing patterns
  - `Makefile:10-20` - Test command structure

  **WHY Each Reference Matters**:
  - Need to follow existing test patterns and conventions
  - Understand how to mock external dependencies (GitHub, git)
  - Ensure tests integrate with existing test infrastructure

  **Acceptance Criteria**:
  - [ ] All new features have corresponding unit tests
  - [ ] Integration tests cover end-to-end workflows
  - [ ] Edge cases tested (invalid tokens, wrong scopes, etc.)
  - [ ] All tests pass: make test

  **QA Scenarios**:
  ```
  Scenario: All tests pass
    Tool: Bash
    Preconditions: All implementation complete
    Steps:
      1. Run "make test"
      2. Verify all tests pass without failures
      3. Check test coverage for new code
    Expected Result: 100% test pass rate, good coverage
    Failure Indicators: Test failures, coverage gaps
    Evidence: .sisyphus/evidence/task-13-test-suite.txt
  ```

  **Evidence to Capture**:
  - [ ] task-13-test-suite.txt - Test execution results

  **Commit**: YES (groups with 13)
  - Message: `test: add comprehensive test suite for v1.0 features`
  - Files: Various _test.go files
  - Pre-commit: make test

---

- [x] 14. Integration testing with OpenClaw repo

  **What to do**:
  - Test complete workflow with OpenClaw/openclaw repository
  - Verify config command stores token correctly
  - Verify analyze works with unlimited PRs (--max-prs=-1)
  - Verify git clone and PR enrichment work at scale
  - Verify omni mode works with large PR sets

  **Must NOT do**:
  - Don't use real GitHub tokens in tests (mock instead)
  - Don't overwhelm GitHub API with excessive requests

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `unspecified-high`
    - Reason: Large-scale integration testing requiring careful resource management
  - **Skills**: [`git-master`]
    - `git-master`: For integration testing and debugging

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 13, 15, 16)
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 6, 7, 9, 11 (core functionality)

  **References**:
  **Pattern References**:
  - Previous debug logs showing OpenClaw has ~6646 PRs
  - `internal/testutil/fixture_loader.go:20-50` - Test fixture patterns
  - `fixtures/openclaw-sample-200.json` - Existing OpenClaw test data

  **WHY Each Reference Matters**:
  - OpenClaw is the primary scale target mentioned in AGENTS.md
  - Need to test at realistic scale (thousands of PRs)
  - Existing fixtures show expected data structure

  **Acceptance Criteria**:
  - [ ] Complete workflow tested with OpenClaw repository
  - [ ] Config command works end-to-end
  - [ ] Unlimited PR fetching works without truncation
  - [ ] Git clone and enrichment work at scale
  - [ ] Omni mode handles large PR sets efficiently

  **QA Scenarios**:
  ```
  Scenario: OpenClaw integration test
    Tool: Interactive_bash
    Preconditions: Mock GitHub API with OpenClaw-like data
    Steps:
      1. Start tmux session
      2. Set up mock GitHub with 1000+ PRs
      3. Run "pratc config set --scope global github_token mock-token"
      4. Run "pratc analyze --repo=openclaw/openclaw --max-prs=-1"
      5. Verify all PRs processed without truncation
      6. Verify git clone and enrichment completed
      7. Test omni mode with large selector
    Expected Result: Complete workflow succeeds at scale
    Failure Indicators: Truncation, git failures, performance issues
    Evidence: .sisyphus/evidence/task-14-opencalw-integration.txt
  ```

  **Evidence to Capture**:
  - [ ] task-14-opencalw-integration.txt - Integration test results

  **Commit**: NO (testing only)
  - Message: N/A (test execution only)

---

- [x] 15. Documentation updates

  **What to do**:
  - Update README with v1.0 features and commands
  - Document config command usage and examples
  - Document new startup banner and version info
  - Document fixed max-prs behavior
  - Document enhanced logger capabilities
  - Document PR enrichment and git integration

  **Must NOT do**:
  - Don't document features that aren't implemented
  - Don't provide outdated or incorrect information

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `writing`
    - Reason: Comprehensive documentation update
  - **Skills**: []
    - No special skills needed for documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 13, 14, 16)
  - **Blocks**: Final delivery
  - **Blocked By**: All features implemented (1-12)

  **References**:
  **Pattern References**:
  - `README.md:1-100` - Current documentation structure
  - `internal/cmd/root.go:30-50` - Command help text patterns
  - AGENTS.md documentation style

  **WHY Each Reference Matters**:
  - Need to maintain consistent documentation style
  - Command help text should match README examples
  - Follow existing documentation conventions

  **Acceptance Criteria**:
  - [ ] README updated with all v1.0 features
  - [ ] Config command fully documented with examples
  - [ ] Version and banner behavior documented
  - [ ] Max-PRS behavior correctly documented
  - [ ] Logger capabilities documented
  - [ ] PR enrichment and git features documented

  **QA Scenarios**:
  ```
  Scenario: Documentation accuracy
    Tool: Bash
    Preconditions: Updated README
    Steps:
      1. Follow README instructions step by step
      2. Verify all commands work as documented
      3. Verify all examples execute successfully
      4. Check for outdated or incorrect information
    Expected Result: Documentation matches actual behavior exactly
    Failure Indicators: Command errors, wrong syntax, missing features
    Evidence: .sisyphus/evidence/task-15-documentation.txt
  ```

  **Evidence to Capture**:
  - [ ] task-15-documentation.txt - Documentation verification

  **Commit**: YES (groups with 15)
  - Message: `docs: update README for v1.0 release`
  - Files: README.md
  - Pre-commit: none (documentation only)

---

- [x] 16. Build and release preparation

  **What to do**:
  - Update Makefile with version injection via ldflags
  - Ensure build process includes all new files
  - Verify binary works with all new features
  - Prepare release notes for v1.0

  **Must NOT do**:
  - Don't break existing build process
  - Don't include development/debug artifacts in release

  **Recommended Agent Profile**:
  > Select category + skills based on task domain. Justify each choice.
  - **Category**: `quick`
    - Reason: Build system updates and release preparation
  - **Skills**: [`git-master`]
    - `git-master`: For build system modifications

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 13, 14, 15)
  - **Blocks**: Final delivery
  - **Blocked By**: Version setup (Task 1)

  **References**:
  **Pattern References**:
  - `Makefile:5-15` - Current build command structure
  - `cmd/pratc/main.go:1-17` - Main entry point
  - `internal/version.go` - Version constant (from Task 1)

  **WHY Each Reference Matters**:
  - Need to understand current build process to enhance it
  - Version injection should use standard Go ldflags pattern
  - Ensure all new files are included in build

  **Acceptance Criteria**:
  - [ ] Makefile updated with version ldflags
  - [ ] Build process includes all new features
  - [ ] Binary works correctly with v1.0 features
  - [ ] Release notes prepared

  **QA Scenarios**:
  ```
  Scenario: Build and version verification
    Tool: Bash
    Preconditions: Updated Makefile
    Steps:
      1. Run "make build"
      2. Verify binary builds without errors
      3. Run "./bin/pratc --help"
      4. Verify startup banner shows v1.0
      5. Test all new commands
    Expected Result: Clean build with correct version and working features
    Failure Indicators: Build errors, wrong version, missing features
    Evidence: .sisyphus/evidence/task-16-build-release.txt
  ```

  **Evidence to Capture**:
  - [ ] task-16-build-release.txt - Build verification

  **Commit**: YES (groups with 16)
  - Message: `build: prepare v1.0 release with version injection`
  - Files: Makefile, internal/version.go
  - Pre-commit: make build

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `tsc --noEmit` + linter + `bun test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill if UI)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **1**: `feat(version): add v1.0 startup banner with author attribution` — internal/version.go, internal/cmd/root.go, npm test
- **2**: `feat(logger): add Debug and Warn methods with DEBUG=ERROR` — internal/logger/logger.go, npm test  
- **3**: `feat(settings): allow github_token in global settings` — internal/settings/validator.go, npm test
- **4**: `feat(config): add config command stub` — cmd/pratc/config.go, npm test
- **5**: `feat(config): implement full config command with subcommands` — internal/cmd/root.go, npm test
- **6**: `feat(auth): integrate config command token with GitHub client` — internal/app/service.go, npm test
- **7**: `fix(analyze): fix max-prs bug, -1 now means unlimited` — internal/app/service.go, cmd/pratc/analyze.go, npm test
- **8**: `feat(config): display config file locations in startup banner` — internal/cmd/root.go, npm test
- **9**: `feat(git): integrate git clone for main branch and PR refs` — internal/app/service.go, internal/repo/mirror.go, npm test
- **10**: `feat(pr): enrich PR data with full body and file changes` — internal/app/service.go, internal/types/models.go, npm test
- **11**: `feat(batch): optimize PR fetching and git operations with concurrency` — internal/github/client.go, internal/repo/mirror.go, internal/sync/default_runner.go, npm test
- **12**: `docs: add omni mode documentation and v1.0 examples` — README.md, internal/cmd/root.go, npm test
- **13**: `test: add comprehensive test suite for v1.0 features` — *_test.go files, npm test
- **14**: `chore: integration testing with OpenClaw repo` — test execution only, no commit
- **15**: `docs: update README for v1.0 release` — README.md, npm test
- **16**: `build: prepare v1.0 release with version injection` — Makefile, internal/version.go, npm test

---

## Success Criteria

### Verification Commands
```bash
# Test config command
pratc config set --scope global github_token test-token
pratc config get --scope global github_token

# Test version banner
pratc --help

# Test max-prs unlimited
pratc analyze --repo=test/test --max-prs=-1 --format=json

# Test logger methods
# (requires test file creation)

# Test build
make build && ./bin/pratc --help
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent  
- [ ] All tests pass
- [ ] Startup banner displays correctly
- [ ] Config command works end-to-end
- [ ] Max-PRS unlimited works
- [ ] Logger has Debug/Warn methods
- [ ] PR data includes full body and files
- [ ] Git clone works for main branch and PRs
- [ ] Omni mode works with enriched data
- [ ] Documentation updated and accurate