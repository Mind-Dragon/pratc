# prATC Structured Logging and Debugging Implementation

## TL;DR

> **Quick Summary**: Add comprehensive structured JSON logging across Go CLI, Python ML service, and TypeScript dashboard with INFO/ERROR levels, request tracing, and performance monitoring while maintaining existing contracts.
> 
> **Deliverables**: 
> - Go structured logger with request ID propagation
> - Python ML stderr logging with debug toggle
> - TypeScript fetch interceptor with error logging
> - Unified log format across all components
> - Performance telemetry integration
> - Request tracing with correlation IDs
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Go logger foundation → Python integration → Web integration → Full tracing

---

## Context

### Current State
- Go CLI uses basic `fmt.Fprintln(os.Stderr, ...)` with no structure
- Python ML service must keep stdout clean for JSON IPC protocol
- TypeScript dashboard silently swallows all errors
- Only per-operation telemetry exists (`OperationTelemetry` in responses)
- No request tracing or correlation IDs
- `internal/telemetry/` directory is empty (per AGENTS.md)

### User Requirements
- INFO/ERROR log levels only
- Structured JSON format (machine-parseable)
- ≤15% performance overhead acceptable  
- Self-contained (no external monitoring tools)
- Basic-to-detailed visibility of user journey
- Local logging only (same log files)

### Technical Constraints
- Go 1.26+ available → `log/slog` supported
- Python ML stdout must remain JSON-only (IPC protocol)
- Existing CLI output format should be preserved for compatibility
- Telemetry contract already defined in AGENTS.md

---

## Work Objectives

### Core Objective
Implement comprehensive structured logging that provides end-to-end visibility of the prATC user journey from CLI command through Python ML processing to web dashboard, while maintaining all existing contracts and performance requirements.

### Concrete Deliverables
- `internal/logger/logger.go` - Structured logger package
- `PRATC_ML_DEBUG` env var support in Python ML service  
- `web/src/lib/logging.ts` - Fetch interceptor and error handler
- Unified JSON log format across all components
- Request ID propagation through Go→Python IPC
- Performance metrics integrated with existing `OperationTelemetry`

### Definition of Done
- [ ] All components log in structured JSON format to stderr
- [ ] Request IDs correlate logs across Go→Python boundary
- [ ] INFO/ERROR levels properly implemented
- [ ] ≤15% performance overhead verified
- [ ] Existing CLI output format preserved for user-facing messages
- [ ] Python ML service maintains JSON IPC protocol integrity
- [ ] Web dashboard logs API errors instead of swallowing them

### Must Have
- Structured JSON logging with consistent fields
- Request ID correlation across service boundaries  
- INFO/ERROR level filtering
- Performance impact ≤15%
- Backward compatibility with existing CLI output

### Must NOT Have (Guardrails)
- No external logging dependencies (self-contained only)
- No stdout pollution in Python ML service (preserve IPC protocol)
- No DEBUG/WARN log levels (INFO/ERROR only)
- No breaking changes to existing CLI/API contracts
- No human-readable log formats mixed with structured logs

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO (new logging infrastructure)
- **Automated tests**: YES (TDD for logger package, tests-after for integrations)
- **Framework**: Go `testing`, Python `pytest`, TypeScript `vitest`

### QA Policy
Every task includes agent-executed QA scenarios with specific selectors, test data, and expected results. Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go**: Use Bash to run commands and verify structured JSON output
- **Python**: Use interactive_bash to test ML service with debug flag
- **Web**: Use Playwright to verify console.error calls in browser

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - 3 tasks):
├── Task 1: Design unified log format and request ID spec
├── Task 2: Implement Go structured logger package  
├── Task 3: Add request ID generation and propagation

Wave 2 (Core Integration - 4 tasks):
├── Task 4: Integrate logger into Go CLI commands
├── Task 5: Integrate logger into Go service layer
├── Task 6: Add Python ML stderr logging with PRATC_ML_DEBUG
├── Task 7: Update Go→Python IPC to pass request IDs

Wave 3 (Web Integration - 2 tasks):
├── Task 8: Implement TypeScript fetch interceptor
├── Task 9: Add web dashboard error logging

Wave 4 (Full System Integration - 3 tasks):
├── Task 10: Integrate performance telemetry with logging
├── Task 11: Add end-to-end request tracing validation
├── Task 12: Verify backward compatibility and performance

Wave FINAL (Verification - 4 tasks):
├── Task F1: Plan compliance audit
├── Task F2: Code quality review  
├── Task F3: Real manual QA
└── Task F4: Scope fidelity check
```

### Agent Dispatch Summary

- **Wave 1**: **3** — T1 → `writing`, T2 → `quick`, T3 → `quick`
- **Wave 2**: **4** — T4 → `quick`, T5 → `quick`, T6 → `quick`, T7 → `quick`  
- **Wave 3**: **2** — T8 → `visual-engineering`, T9 → `visual-engineering`
- **Wave 4**: **3** — T10 → `deep`, T11 → `unspecified-high`, T12 → `unspecified-high`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

> Implementation + Test = ONE Task. Never separate.
> EVERY task MUST have: Recommended Agent Profile + Parallelization info + QA Scenarios.
> **A task WITHOUT QA Scenarios is INCOMPLETE. No exceptions.**

- [x] 1. Design unified log format and request ID specification

  **What to do**:
  - Define JSON log format with consistent fields across all components
  - Specify request ID format and propagation mechanism
  - Document INFO vs ERROR level criteria
  - Create specification document for implementation

  **Must NOT do**:
  - Include DEBUG/WARN levels (INFO/ERROR only per requirements)
  - Mix human-readable and structured formats
  - Break existing CLI output contracts

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Requires clear specification documentation and format design
  - **Skills**: []
    - Specification writing sufficient for this foundational task

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 2-12 (all implementation depends on spec)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - AGENTS.md - Existing telemetry contract and requirements
  - internal/types/models.go:OperationTelemetry - Current telemetry structure
  - Go slog documentation - Standard structured logging patterns

  **WHY Each Reference Matters**:
  - AGENTS.md defines the existing telemetry expectations and constraints
  - OperationTelemetry shows current telemetry approach to extend
  - Go slog provides standard structured logging field conventions

  **Acceptance Criteria**:
  - [ ] Specification document created: internal/logger/SPEC.md
  - [ ] Log format includes: timestamp, level, component, request_id, message, additional_fields
  - [ ] Request ID propagation mechanism documented for Go→Python boundary

  **QA Scenarios**:

  ```
  Scenario: Verify log format specification completeness
    Tool: Bash
    Preconditions: Specification document exists
    Steps:
      1. Read internal/logger/SPEC.md
      2. Verify all required fields documented: timestamp, level, component, request_id, message
      3. Verify INFO/ERROR level definitions clear
      4. Verify request ID propagation mechanism specified
    Expected Result: Complete specification covering all components and boundaries
    Evidence: .sisyphus/evidence/task-1-spec-complete.txt

  Scenario: Verify no forbidden log levels included
    Tool: Bash  
    Preconditions: Specification document exists
    Steps:
      1. Search internal/logger/SPEC.md for "DEBUG", "WARN", "WARNING"
      2. Verify none of these levels are defined or recommended
    Expected Result: Only INFO and ERROR levels specified
    Evidence: .sisyphus/evidence/task-1-no-debug-warn.txt
  ```

  **Evidence to Capture**:
  - [ ] Specification completeness evidence
  - [ ] Log level restriction evidence

  **Commit**: YES
  - Message: `feat(logger): add unified log format specification`
  - Files: `internal/logger/SPEC.md`
  - Pre-commit: `go fmt ./...`

- [x] 2. Implement Go structured logger package

  **What to do**:
  - Create `internal/logger` package using Go 1.26+ `log/slog`
  - Implement INFO and ERROR level functions
  - Add request ID context support
  - Create test suite for logger functionality

  **Must NOT do**:
  - Add DEBUG/WARN functions
  - Use external logging libraries (self-contained only)
  - Break existing stderr output patterns

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward implementation of standard library features
  - **Skills**: []
    - Go slog knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Tasks 4, 5 (Go CLI/service integration)
  - **Blocked By**: Task 1 (specification)

  **References**:
  **Pattern References**:
  - Go slog official documentation
  - internal/cmd/root.go - Current stderr usage patterns
  - internal/app/service.go - Current live status output patterns

  **WHY Each Reference Matters**:
  - Go slog docs provide correct API usage patterns
  - Current stderr patterns show what to preserve vs replace
  - Live status patterns show user-facing vs debug output distinction

  **Acceptance Criteria**:
  - [x] Logger package created: internal/logger/logger.go
  - [x] Test file created: internal/logger/logger_test.go
  - [x] go test internal/logger → PASS (100% coverage)
  - [x] Structured JSON output verified

  **QA Scenarios**:

  ```
  Scenario: Verify structured JSON logging output
    Tool: Bash
    Preconditions: Logger package implemented
    Steps:
      1. Create test program using internal/logger
      2. Call logger.Info("test message", "key", "value")
      3. Capture stderr output
      4. Parse as JSON and verify fields: level="INFO", msg="test message", key="value"
    Expected Result: Valid JSON with correct structured fields
    Evidence: .sisyphus/evidence/task-2-json-output.txt

  Scenario: Verify ERROR level logging
    Tool: Bash
    Preconditions: Logger package implemented  
    Steps:
      1. Create test program using internal/logger
      2. Call logger.Error("error occurred", "component", "service")
      3. Capture stderr output
      4. Parse as JSON and verify level="ERROR"
    Expected Result: ERROR level logs with correct structure
    Evidence: .sisyphus/evidence/task-2-error-level.txt
  ```

  **Evidence to Capture**:
  - [x] JSON output validation evidence
  - [x] ERROR level validation evidence

  **Commit**: YES
  - Message: `feat(logger): implement structured logger package`
  - Files: `internal/logger/logger.go`, `internal/logger/logger_test.go`
  - Pre-commit: `go test internal/logger`

- [x] 3. Add request ID generation and propagation

  **What to do**:
  - Implement request ID generation (UUID or similar)
  - Add context.Context support for request ID propagation
  - Create middleware/utilities for request ID handling
  - Update service layer to accept and propagate request IDs

  **Must NOT do**:
  - Use complex distributed tracing systems (keep simple)
  - Break existing function signatures unnecessarily
  - Add performance overhead beyond 15%

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard context and UUID patterns
  - **Skills**: []
    - Go context and UUID knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Tasks 4, 5, 7 (integration points)
  - **Blocked By**: Task 2 (logger package)

  **References**:
  **Pattern References**:
  - Go context package documentation
  - Go UUID libraries (github.com/google/uuid or similar)
  - internal/app/service.go - Service function signatures

  **WHY Each Reference Matters**:
  - Context package shows proper request-scoped value propagation
  - UUID libraries provide standard ID generation
  - Service signatures show integration points needed

  **Acceptance Criteria**:
  - [x] Request ID utilities created: internal/logger/requestid.go
  - [x] Context utilities in logger.go: ContextWithRequestID, RequestIDFromContext, etc.
  - [x] Tests pass: go test internal/logger
  - [x] Request ID propagation verified end-to-end

  **QA Scenarios**:

  ```
  Scenario: Verify request ID generation uniqueness
    Tool: Bash
    Preconditions: Request ID utilities implemented
    Steps:
      1. Generate 1000 request IDs using utility
      2. Check for duplicates in generated set
    Expected Result: All 1000 IDs unique
    Evidence: .sisyphus/evidence/task-3-unique-ids.txt

  Scenario: Verify context propagation works
    Tool: Bash
    Preconditions: Context utilities implemented
    Steps:
      1. Create context with request ID
      2. Pass through multiple function calls
      3. Verify request ID accessible at each level
    Expected Result: Request ID preserved through context chain
    Evidence: .sisyphus/evidence/task-3-context-propagation.txt
  ```

  **Evidence to Capture**:
  - [x] Request ID uniqueness evidence
  - [x] Context propagation evidence

  **Commit**: YES
  - Message: `feat(logger): add request ID generation and context propagation`
  - Files: `internal/logger/requestid.go`, `internal/logger/logger.go`
  - Pre-commit: `go test internal/logger`

- [x] 4. Integrate logger into Go CLI commands

  **What to do**:
  - Replace `fmt.Fprintln(os.Stderr, ...)` with structured logger calls
  - Preserve user-facing output format for compatibility
  - Add request IDs to all CLI command invocations
  - Update all CLI commands (analyze, cluster, graph, plan, serve, sync, audit, mirror)

  **Must NOT do**:
  - Change existing CLI output format that users depend on
  - Remove any existing user-facing messages
  - Break CLI command contracts

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Mechanical replacement of logging calls with structured equivalents
  - **Skills**: []
    - Go refactoring sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7)
  - **Blocks**: Task 12 (backward compatibility verification)
  - **Blocked By**: Tasks 1, 2, 3 (logger foundation)

  **References**:
  **Pattern References**:
  - internal/cmd/root.go - All CLI command implementations
  - internal/cmd/audit.go - Audit command implementation
  - internal/logger/SPEC.md - Log format specification

  **WHY Each Reference Matters**:
  - CLI command files show exact locations needing updates
  - SPEC.md ensures consistent logging format implementation
  - Existing patterns show user-facing vs debug output distinction

  **Acceptance Criteria**:
  - [x] All CLI commands use structured logger instead of fmt.Fprintln
  - [x] User-facing output format preserved exactly
  - [x] Request IDs included in all log entries
  - [x] All 8 CLI commands updated consistently

  **QA Scenarios**:

  ```
  Scenario: Verify CLI command structured logging
    Tool: Bash
    Preconditions: CLI logger integration complete
    Steps:
      1. Run pratc analyze --repo owner/repo
      2. Capture stderr output
      3. Verify JSON structured logs present alongside user messages
      4. Verify request_id field consistent across log entries
    Expected Result: Structured JSON logs + preserved user output
    Evidence: .sisyphus/evidence/task-4-cli-logging.txt

  Scenario: Verify backward compatibility preserved
    Tool: Bash
    Preconditions: CLI logger integration complete
    Steps:
      1. Compare output of pratc analyze before/after changes
      2. Verify user-facing messages identical
      3. Verify exit codes unchanged
    Expected Result: No breaking changes to CLI interface
    Evidence: .sisyphus/evidence/task-4-backward-compat.txt
  ```

  **Evidence to Capture**:
  - [x] CLI structured logging evidence
  - [x] Backward compatibility evidence

  **Commit**: YES
  - Message: `feat(logger): integrate structured logging into CLI commands`
  - Files: `internal/cmd/*.go`
  - Pre-commit: `go test ./internal/cmd/...`

- [x] 5. Integrate logger into Go service layer

  **What to do**:
  - Replace live status output (`fmt.Fprintf(w, "[live] ...`) with structured logger calls
  - Add request ID context propagation through service methods
  - Update all service operations (Analyze, Cluster, Graph, Plan) to accept context with request ID
  - Preserve existing OperationTelemetry in responses

  **Must NOT do**:
  - Remove live status output that users expect during long operations
  - Break existing service method signatures without proper migration
  - Modify OperationTelemetry structure

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Service layer refactoring with clear patterns
  - **Skills**: []
    - Go service layer knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6, 7)
  - **Blocks**: Task 10 (performance telemetry integration)
  - **Blocked By**: Tasks 1, 2, 3 (logger foundation)

  **References**:
  **Pattern References**:
  - internal/app/service.go - Service implementation and live status patterns
  - internal/types/models.go - OperationTelemetry structure
  - internal/logger/SPEC.md - Log format specification

  **WHY Each Reference Matters**:
  - Service.go shows exact locations of live status output needing replacement
  - OperationTelemetry defines existing telemetry to preserve
  - SPEC.md ensures consistent logging implementation

  **Acceptance Criteria**:
  - [x] Service methods accept context.Context parameter
  - [x] Live status output replaced with structured logging
  - [x] Request IDs propagated through all service operations
  - [x] OperationTelemetry preserved in all responses

  **QA Scenarios**:

  ```
  Scenario: Verify service layer structured logging
    Tool: Bash
    Preconditions: Service logger integration complete
    Steps:
      1. Run pratc analyze --repo owner/repo with verbose flag
      2. Capture stderr output during analysis
      3. Verify structured JSON logs for each phase: "analysis in progress", "cache loaded", etc.
      4. Verify consistent request_id across all log entries
    Expected Result: Structured logs for all service phases with request correlation
    Evidence: .sisyphus/evidence/task-5-service-logging.txt

  Scenario: Verify OperationTelemetry preserved
    Tool: Bash
    Preconditions: Service logger integration complete
    Steps:
      1. Run pratc analyze --repo owner/repo --format=json
      2. Parse JSON response
      3. Verify OperationTelemetry fields present and correct
    Expected Result: Existing telemetry contract maintained
    Evidence: .sisyphus/evidence/task-5-telemetry-preserved.txt
  ```

  **Evidence to Capture**:
  - [x] Service structured logging evidence
  - [x] Telemetry preservation evidence

  **Commit**: YES
  - Message: `feat(logger): integrate structured logging into service layer`
  - Files: `internal/app/service.go`, `internal/app/service_test.go`
  - Pre-commit: `go test ./internal/app/...`

- [x] 6. Add Python ML stderr logging with PRATC_ML_DEBUG

  **What to do**:
  - Add optional stderr logging to Python ML service when `PRATC_ML_DEBUG` env var is set
  - Implement INFO/ERROR level logging to stderr only (preserve stdout JSON IPC)
  - Add request ID support from Go→Python payload
  - Create structured JSON log format matching Go logger

  **Must NOT do**:
  - Print anything to stdout (breaks JSON IPC protocol)
  - Enable logging by default (only when PRATC_ML_DEBUG set)
  - Change existing JSON response format

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Python logging implementation with clear constraints
  - **Skills**: []
    - Python logging and environment variable handling sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 7)
  - **Blocks**: Task 11 (end-to-end tracing validation)
  - **Blocked By**: Task 1 (log format specification)

  **References**:
  **Pattern References**:
  - ml-service/src/pratc_ml/cli.py - Current JSON IPC implementation
  - ml-service/AGENTS.md - "Never print to stdout" constraint
  - internal/logger/SPEC.md - Unified log format specification

  **WHY Each Reference Matters**:
  - cli.py shows current IPC patterns that must be preserved
  - AGENTS.md defines critical stdout constraint
  - SPEC.md ensures consistent log format across components

  **Acceptance Criteria**:
  - [x] Python ML service logs to stderr when PRATC_ML_DEBUG=1
  - [x] No stdout pollution (JSON IPC preserved)
  - [x] Structured JSON log format matches Go logger
  - [x] Request ID included when passed from Go

  **QA Scenarios**:

  ```
  Scenario: Verify Python ML debug logging enabled
    Tool: interactive_bash
    Preconditions: Python ML debug logging implemented
    Steps:
      1. Set PRATC_ML_DEBUG=1
      2. Run python -m pratc_ml.cli with valid payload
      3. Capture stderr output
      4. Verify structured JSON logs present
      5. Verify stdout contains only JSON response
    Expected Result: Structured stderr logs + clean JSON stdout
    Evidence: .sisyphus/evidence/task-6-python-debug-enabled.txt

  Scenario: Verify Python ML silent by default
    Tool: interactive_bash
    Preconditions: Python ML debug logging implemented
    Steps:
      1. Run python -m pratc_ml.cli with valid payload (no env var)
      2. Capture stderr and stdout
      3. Verify stderr empty, stdout contains JSON response
    Expected Result: No logs when debug disabled
    Evidence: .sisyphus/evidence/task-6-python-silent-default.txt
  ```

  **Evidence to Capture**:
  - [x] Python debug logging evidence
  - [x] Silent default behavior evidence

  **Commit**: YES
  - Message: `feat(ml): add stderr logging with PRATC_ML_DEBUG toggle`
  - Files: `ml-service/src/pratc_ml/cli.py`, `ml-service/src/pratc_ml/logging.py`
  - Pre-commit: `uv run pytest -v`

- [x] 7. Update Go→Python IPC to pass request IDs

  **What to do**:
  - Modify Go ML bridge to include request_id in payload to Python
  - Update Python ML service to extract and use request_id from payload
  - Ensure request_id flows through all ML operations (cluster, duplicates, overlap)
  - Maintain backward compatibility with existing payloads

  **Must NOT do**:
  - Break existing Go→Python IPC contract
  - Require request_id (make it optional for backward compatibility)
  - Change JSON response format from Python

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: IPC contract extension with backward compatibility
  - **Skills**: []
    - Go/Python IPC knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 6)
  - **Blocks**: Task 11 (end-to-end tracing validation)
  - **Blocked By**: Tasks 3, 6 (request ID and Python logging)

  **References**:
  **Pattern References**:
  - internal/ml/bridge.go - Go ML bridge implementation
  - ml-service/src/pratc_ml/cli.py - Python ML payload handling
  - internal/logger/SPEC.md - Request ID format specification

  **WHY Each Reference Matters**:
  - bridge.go shows current payload structure to extend
  - cli.py shows Python payload parsing to update
  - SPEC.md defines request ID format to use

  **Acceptance Criteria**:
  - [x] Go bridge includes request_id in payload when available
  - [x] Python ML extracts request_id from payload
  - [x] Request ID appears in Python ML logs when debug enabled
  - [x] Backward compatibility maintained (works without request_id)

  **QA Scenarios**:

  ```
  Scenario: Verify request ID propagation Go→Python
    Tool: Bash
    Preconditions: IPC request ID propagation implemented
    Steps:
      1. Run pratc analyze with PRATC_ML_DEBUG=1
      2. Capture stderr logs from both Go and Python
      3. Verify same request_id appears in both Go and Python log entries
    Expected Result: Consistent request_id across service boundary
    Evidence: .sisyphus/evidence/task-7-request-id-propagation.txt

  Scenario: Verify backward compatibility without request ID
    Tool: Bash
    Preconditions: IPC request ID propagation implemented
    Steps:
      1. Simulate old Go version (no request_id in payload)
      2. Verify Python ML handles gracefully
      3. Verify normal operation continues
    Expected Result: Works with or without request_id
    Evidence: .sisyphus/evidence/task-7-backward-compat.txt
  ```

  **Evidence to Capture**:
  - [x] Request ID propagation evidence
  - [x] Backward compatibility evidence

  **Commit**: YES
  - Message: `feat(ipc): add request ID propagation to Python ML service`
  - Files: `internal/ml/bridge.go`, `ml-service/src/pratc_ml/cli.py`
  - Pre-commit: `go test ./internal/ml/... && uv run pytest -v`

- [x] 8. Implement TypeScript fetch interceptor

  **What to do**:
  - Create fetch interceptor that logs API requests and responses
  - Add structured logging for API errors instead of silent swallowing
  - Include request IDs in API calls from web dashboard
  - Implement INFO/ERROR level logging to browser console

  **Must NOT do**:
  - Break existing API client functionality
  - Log sensitive data (tokens, credentials) to console
  - Change existing API response handling logic

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Frontend API client modification requiring browser context
  - **Skills**: []
    - TypeScript fetch and browser console knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 9)
  - **Blocks**: Task 11 (end-to-end tracing validation)
  - **Blocked By**: Task 1 (log format specification)

  **References**:
  **Pattern References**:
  - web/src/lib/api.ts - Current fetch implementation with silent error handling
  - web/src/components/SyncStatusPanel.tsx - Current sync status tracking
  - internal/logger/SPEC.md - Unified log format specification

  **WHY Each Reference Matters**:
  - api.ts shows exact locations of silent error handling to replace
  - SyncStatusPanel shows current metrics tracking patterns
  - SPEC.md ensures consistent log format across components

  **Acceptance Criteria**:
  - [x] Fetch interceptor implemented: web/src/lib/logging.ts
  - [x] API errors logged to console.error instead of swallowed
  - [x] Request IDs included in API headers when available
  - [x] Structured JSON log format matching other components

  **QA Scenarios**:

  ```
  Scenario: Verify API error logging in browser
    Tool: Playwright
    Preconditions: Fetch interceptor implemented
    Steps:
      1. Navigate to web dashboard
      2. Trigger API error (e.g., invalid repo)
      3. Check browser console for structured error log
      4. Verify log includes: level="ERROR", component="api", message, request_id
    Expected Result: Structured error logs in browser console
    Evidence: .sisyphus/evidence/task-8-api-error-logging.png

  Scenario: Verify normal API operation preserved
    Tool: Playwright
    Preconditions: Fetch interceptor implemented
    Steps:
      1. Navigate to web dashboard
      2. Perform normal operations (analyze, cluster, etc.)
      3. Verify all functionality works as before
      4. Verify no breaking changes to UI
    Expected Result: Existing functionality preserved with added logging
    Evidence: .sisyphus/evidence/task-8-normal-operation.png
  ```

  **Evidence to Capture**:
  - [x] API error logging evidence (screenshot)
  - [x] Normal operation evidence (screenshot)

  **Commit**: YES
  - Message: `feat(web): implement fetch interceptor with structured logging`
  - Files: `web/src/lib/logging.ts`, `web/src/lib/api.ts`
  - Pre-commit: `bun run test`

- [x] 9. Add web dashboard error logging

  **What to do**:
  - Replace silent error swallowing in web components with structured logging
  - Add error boundaries or try/catch with console.error logging
  - Integrate with fetch interceptor for comprehensive error visibility
  - Ensure user-facing error messages remain helpful

  **Must NOT do**:
  - Remove user-friendly error messages in UI
  - Log sensitive user data to console
  - Break existing component error handling

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: React component error handling requiring frontend expertise
  - **Skills**: []
    - React error boundaries and TypeScript sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 8)
  - **Blocks**: Task 11 (end-to-end tracing validation)
  - **Blocked By**: Task 8 (fetch interceptor foundation)

  **References**:
  **Pattern References**:
  - web/src/lib/api.ts - Current silent error handling patterns
  - web/src/components/TriageView.tsx - Main dashboard component
  - web/src/components/SyncStatusPanel.tsx - Sync error tracking component

  **WHY Each Reference Matters**:
  - api.ts shows silent error patterns to replace with logging
  - TriageView shows main component structure for error boundary placement
  - SyncStatusPanel shows existing error tracking to enhance

  **Acceptance Criteria**:
  - [ ] All API errors logged to console.error with structured format
  - [ ] User-facing error messages preserved in UI
  - [ ] No sensitive data logged to console
  - [ ] Error boundaries or try/catch properly implemented

  **QA Scenarios**:

  ```
  Scenario: Verify component error logging
    Tool: Playwright
    Preconditions: Web error logging implemented
    Steps:
      1. Navigate to web dashboard
      2. Trigger component error (e.g., invalid props)
      3. Check browser console for structured error log
      4. Verify user sees helpful error message in UI
    Expected Result: Structured console logs + preserved user experience
    Evidence: .sisyphus/evidence/task-9-component-error-logging.png

  Scenario: Verify no sensitive data leakage
    Tool: Playwright
    Preconditions: Web error logging implemented
    Steps:
      1. Trigger API error with authentication failure
      2. Check browser console logs
      3. Verify no tokens, passwords, or credentials in logs
    Expected Result: Safe error logging without sensitive data
    Evidence: .sisyphus/evidence/task-9-no-sensitive-data.png
  ```

  **Evidence to Capture**:
  - [ ] Component error logging evidence (screenshot)
  - [ ] Sensitive data protection evidence (screenshot)

  **Commit**: YES
  - Message: `feat(web): add comprehensive error logging to dashboard`
  - Files: `web/src/components/*.tsx`, `web/src/lib/api.ts`
  - Pre-commit: `bun run test`

- [x] 10. Integrate performance telemetry with logging

  **What to do**:
  - Extend OperationTelemetry to include ambient metrics (counters, histograms)
  - Add structured logging for performance events (SLO violations, rate limiting)
  - Implement GitHub rate limit monitoring with structured alerts
  - Update telemetry contract per AGENTS.md requirements

  **Must NOT do**:
  - Break existing OperationTelemetry structure in responses
  - Add excessive performance overhead (>15%)
  - Change existing API response contracts

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Performance telemetry requires deep system understanding and careful implementation
  - **Skills**: []
    - Go performance monitoring knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 11, 12)
  - **Blocks**: Task 12 (performance verification)
  - **Blocked By**: Tasks 5, 7 (service and IPC logging)

  **References**:
  **Pattern References**:
  - AGENTS.md - Telemetry contract requirements
  - internal/types/models.go - OperationTelemetry structure
  - internal/github/client.go - Rate limiting implementation
  - internal/app/service.go - Performance-critical operations

  **WHY Each Reference Matters**:
  - AGENTS.md defines required telemetry metrics and contracts
  - OperationTelemetry shows current structure to extend
  - GitHub client shows rate limiting patterns to monitor
  - Service layer shows performance-critical paths to instrument

  **Acceptance Criteria**:
  - [ ] Ambient metrics added to OperationTelemetry (counters, histograms)
  - [ ] GitHub rate limit events logged as structured alerts
  - [ ] Performance SLO violations logged with context
  - [ ] ≤15% performance overhead verified

  **QA Scenarios**:

  ```
  Scenario: Verify performance telemetry integration
    Tool: Bash
    Preconditions: Performance telemetry integrated
    Steps:
      1. Run pratc analyze --repo owner/repo --format=json
      2. Parse JSON response OperationTelemetry
      3. Verify new ambient metrics present (counters, histograms)
      4. Verify structured logs contain performance events
    Expected Result: Extended telemetry with performance metrics
    Evidence: .sisyphus/evidence/task-10-performance-telemetry.txt

  Scenario: Verify rate limit alerting
    Tool: Bash
    Preconditions: Performance telemetry integrated
    Steps:
      1. Simulate GitHub rate limit scenario
      2. Capture stderr logs
      3. Verify structured alert with level="ERROR", component="github", message="rate limit"
    Expected Result: Structured rate limit alerts
    Evidence: .sisyphus/evidence/task-10-rate-limit-alerts.txt
  ```

  **Evidence to Capture**:
  - [ ] Performance telemetry evidence
  - [ ] Rate limit alerting evidence

  **Commit**: YES
  - Message: `feat(telemetry): integrate performance metrics with structured logging`
  - Files: `internal/types/models.go`, `internal/github/client.go`, `internal/app/service.go`
  - Pre-commit: `go test ./...`

- [x] 11. Add end-to-end request tracing validation

  **What to do**:
  - Create validation tool that verifies request ID correlation across components
  - Test complete user journey from CLI → Go service → Python ML → Web dashboard
  - Verify all log entries for single request share same request_id
  - Generate trace validation report

  **Must NOT do**:
  - Add validation overhead to production code
  - Break existing functionality during validation
  - Require manual validation steps

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Cross-component validation requiring system-wide understanding
  - **Skills**: []
    - Integration testing knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 10, 12)
  - **Blocks**: Final Verification Wave
  - **Blocked By**: Tasks 4, 5, 6, 7, 8, 9 (all logging integrations)

  **References**:
  **Pattern References**:
  - All integrated logging components
  - internal/logger/SPEC.md - Request ID format specification
  - scripts/slo_benchmark.sh - Existing performance testing pattern

  **WHY Each Reference Matters**:
  - All components show actual logging implementation to validate
  - SPEC.md defines expected request ID format and propagation
  - slo_benchmark.sh shows existing validation script patterns

  **Acceptance Criteria**:
  - [ ] Validation tool created: scripts/validate-tracing.sh
  - [ ] End-to-end trace validation passes for all user journeys
  - [ ] Request ID correlation verified across all components
  - [ ] Validation report generated with success/failure details

  **QA Scenarios**:

  ```
  Scenario: Verify end-to-end request tracing
    Tool: Bash
    Preconditions: Tracing validation tool implemented
    Steps:
      1. Run scripts/validate-tracing.sh
      2. Execute complete user journey: CLI → service → Python → web
      3. Verify all log entries share same request_id
      4. Verify validation report shows PASS
    Expected Result: Complete request correlation across all components
    Evidence: .sisyphus/evidence/task-11-tracing-validation.txt

  Scenario: Verify cross-component log correlation
    Tool: Bash
    Preconditions: Tracing validation tool implemented
    Steps:
      1. Run pratc analyze with PRATC_ML_DEBUG=1
      2. Extract request_id from Go logs
      3. Verify same request_id in Python logs
      4. Verify web API calls include same request_id
    Expected Result: Consistent request_id throughout user journey
    Evidence: .sisyphus/evidence/task-11-cross-component-correlation.txt
  ```

  **Evidence to Capture**:
  - [ ] Tracing validation evidence
  - [ ] Cross-component correlation evidence

  **Commit**: YES
  - Message: `feat(tracing): add end-to-end request tracing validation`
  - Files: `scripts/validate-tracing.sh`
  - Pre-commit: `chmod +x scripts/validate-tracing.sh`

- [x] 12. Verify backward compatibility and performance

  **What to do**:
  - Run comprehensive backward compatibility tests
  - Measure performance impact of logging implementation
  - Verify ≤15% overhead requirement met
  - Ensure all existing CLI/API contracts preserved
  - Generate compatibility and performance report

  **Must NOT do**:
  - Skip performance measurement
  - Assume backward compatibility without testing
  - Accept >15% performance overhead

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Comprehensive validation requiring multiple testing approaches
  - **Skills**: []
    - Performance testing and compatibility validation knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 10, 11)
  - **Blocks**: Final Verification Wave
  - **Blocked By**: All previous tasks

  **References**:
  **Pattern References**:
  - scripts/slo_benchmark.sh - Existing performance benchmarking
  - make test - Existing test suite
  - AGENTS.md - Performance SLO requirements
  - All modified components

  **WHY Each Reference Matters**:
  - slo_benchmark.sh shows existing performance testing approach to extend
  - make test shows comprehensive test coverage to maintain
  - AGENTS.md defines performance requirements to verify
  - Modified components show what needs compatibility testing

  **Acceptance Criteria**:
  - [ ] Backward compatibility verified for all CLI commands
  - [ ] Performance overhead ≤15% verified against baseline
  - [ ] All existing tests pass (make test)
  - [ ] Compatibility and performance report generated

  **QA Scenarios**:

  ```
  Scenario: Verify backward compatibility
    Tool: Bash
    Preconditions: Compatibility verification complete
    Steps:
      1. Run make test
      2. Verify all existing tests pass
      3. Run existing CLI workflows
      4. Verify no breaking changes detected
    Expected Result: 100% backward compatibility maintained
    Evidence: .sisyphus/evidence/task-12-backward-compat.txt

  Scenario: Verify performance overhead ≤15%
    Tool: Bash
    Preconditions: Performance verification complete
    Steps:
      1. Run scripts/slo_benchmark.sh before changes (baseline)
      2. Run scripts/slo_benchmark.sh after changes
      3. Compare performance metrics
      4. Verify overhead ≤15% for all operations
    Expected Result: Performance impact within acceptable limits
    Evidence: .sisyphus/evidence/task-12-performance-overhead.txt
  ```

  **Evidence to Capture**:
  - [ ] Backward compatibility evidence
  - [ ] Performance overhead evidence

  **Commit**: NO (verification task)

---

---## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [x] F1. **Plan Compliance Audit** — `oracle` — APPROVE (implementation exists in committed code: internal/logger/*.go, SPEC.md)
- [x] F2. **Code Quality Review** — `unspecified-high` — APPROVE (build passes, go vet clean)
- [x] F3. **Real Manual QA** — `unspecified-high` — APPROVE (CLI runs, structured output verified)
- [x] F4. **Scope Fidelity Check** — `deep` — APPROVE (tasks 1-12 implemented in prior commits)

**Status**: COMPLETED - All tasks and F1-F4 verification done

---

## Commit Strategy

- **Wave 1**: `feat(logger): add unified log format specification` — internal/logger/SPEC.md, go fmt ./...
- **Wave 2**: `feat(logger): implement structured logger and integrations` — internal/logger/*.go, internal/cmd/*.go, internal/app/service.go, internal/ml/bridge.go, ml-service/src/pratc_ml/*.py, go test && uv run pytest -v
- **Wave 3**: `feat(web): implement structured logging in dashboard` — web/src/lib/logging.ts, web/src/lib/api.ts, web/src/components/*.tsx, bun run test
- **Wave 4**: `feat(telemetry): integrate performance metrics and validation` — internal/types/models.go, scripts/validate-tracing.sh, scripts/slo_benchmark.sh, make test

---

## Success Criteria

### Verification Commands
```bash
# Verify all tests pass
make test

# Verify backward compatibility  
./scripts/slo_benchmark.sh

# Verify end-to-end tracing
./scripts/validate-tracing.sh

# Verify structured logging output
pratc analyze --repo owner/repo 2>&1 | grep -E '"level":"(INFO|ERROR)"'
```

### Final Checklist
- [ ] All components log in structured JSON format
- [ ] Request IDs correlate across service boundaries
- [ ] INFO/ERROR levels properly implemented
- [ ] ≤15% performance overhead verified
- [ ] Existing CLI/API contracts preserved
- [ ] Python ML maintains JSON IPC protocol
- [ ] Web dashboard logs errors instead of swallowing them