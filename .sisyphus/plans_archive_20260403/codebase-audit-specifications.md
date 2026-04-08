# Codebase Audit & Specification Generation

## TL;DR

> **Quick Summary**: Systematic audit of the entire prATC codebase to generate AI-native specifications covering architecture, API contracts, and code standards for agentic understanding and reference.
> 
> **Deliverables**: Comprehensive documentation in `~/.docs/pratc/` covering all system components with machine-readable structure optimized for AI consumption.
> - Architecture specifications (system boundaries, data flow, component interactions)
> - API contracts (CLI, HTTP, IPC, external integrations)
> - Code standards (Go conventions, Python patterns, TypeScript practices)
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Foundation → Core Components → Integrations → Verification

---

## Context

### Original Request
Audit the entire prATC codebase and write detailed specifications (architecture, API contracts, code standards) into git-ignored ~/.docs directory in AI-native format for agentic understanding/reference.

### Requirements Confirmed
- **Scope**: Full prATC system including Go CLI, Python ML service, Next.js dashboard
- **Specification Types**: Architecture, API contracts, code standards  
- **Purpose**: Agentic understanding and reference
- **Format**: AI-native (structured, machine-readable, context-rich)
- **Location**: Git-ignored `~/.docs/pratc/`

### Research Findings
From AGENTS.md analysis:
- **Structure**: Multi-language system (Go backend, Python ML, TypeScript frontend)
- **Key Components**: CLI commands, HTTP API, Go↔Python IPC, GitHub integration, SQLite cache
- **Cross-cutting**: Shared types (snake_case JSON), config flow, rate-limiting policies
- **Standards**: Go error wrapping, table-driven tests, stable sorting, functional options

---

## Work Objectives

### Core Objective
Generate comprehensive, AI-native specifications that enable complete system understanding for autonomous agents working on the prATC codebase.

### Concrete Deliverables
- `~/.docs/pratc/architecture/` - System architecture specifications
- `~/.docs/pratc/api/` - All API contract specifications  
- `~/.docs/pratc/standards/` - Code standards and conventions
- `~/.docs/pratc/components/` - Individual component specifications
- `~/.docs/pratc/integrations/` - External integration specifications

### Definition of Done
- [ ] All specifications generated and saved to `~/.docs/pratc/`
- [ ] Directory structure mirrors codebase organization
- [ ] All specifications use AI-native format (structured YAML/JSON + rich markdown)
- [ ] Complete coverage of all system components confirmed

### Must Have
- Machine-readable structure optimized for AI parsing
- Complete coverage of Go, Python, and TypeScript components
- Explicit contract definitions with examples
- Cross-references between related specifications
- Version compatibility information

### Must NOT Have (Guardrails)
- Human-readable prose without structured data
- Incomplete or partial component coverage
- Assumptions about undocumented behavior
- Git-tracked files (must be in git-ignored location)
- Duplicate or redundant information

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: N/A (documentation generation)
- **Automated tests**: None (specification generation)
- **Framework**: N/A

### QA Policy
Every task MUST include agent-executed QA scenarios to verify specification completeness and accuracy.

- **File Verification**: Use Bash to confirm file existence, structure, and content
- **Coverage Verification**: Use grep/find to ensure all components are documented
- **Format Verification**: Validate AI-native structure with schema checks

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation + discovery):
├── Task 1: Create docs directory structure [quick]
├── Task 2: Audit codebase structure and components [ultrabrain]
├── Task 3: Define AI-native specification format [ultrabrain]
└── Task 4: Document shared types and cross-cutting concerns [ultrabrain]

Wave 2 (After Wave 1 — core backend components):
├── Task 5: Document Go CLI architecture and commands [ultrabrain]
├── Task 6: Document internal Go packages and services [ultrabrain]
├── Task 7: Document SQLite cache and migrations [ultrabrain]
├── Task 8: Document GitHub client and rate-limiting [ultrabrain]
└── Task 9: Document Go conventions and standards [ultrabrain]

Wave 3 (After Wave 1 — ML and frontend):
├── Task 10: Document Python ML service architecture [ultrabrain]
├── Task 11: Document Go↔Python IPC contracts [ultrabrain]
├── Task 12: Document Next.js dashboard architecture [visual-engineering]
├── Task 13: Document TypeScript types and conventions [visual-engineering]
└── Task 14: Document web build and test processes [visual-engineering]

Wave 4 (After Waves 2-3 — integrations + verification):
├── Task 15: Document system-wide API contracts [ultrabrain]
├── Task 16: Document external integrations (GitHub, Docker) [ultrabrain]
├── Task 17: Document performance SLOs and thresholds [ultrabrain]
├── Task 18: Generate cross-component relationship maps [ultrabrain]
├── Task 19: Verify specification completeness [ultrabrain]
└── Task 20: Final format validation and cleanup [quick]

Critical Path: Task 1 → Task 2 → Task 5 → Task 15 → Task 19 → Task 20
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 5 (Wave 2)
```

### Dependency Matrix
- **1-4**: — — 5-14, 1
- **5-9**: 1-4 — 15-18, 2  
- **10-14**: 1-4 — 15-18, 3
- **15-20**: 5-14 — 4

---

## TODOs

- [x] 1. Create documentation directory structure

  **What to do**:
  - Create git-ignored `~/.docs/pratc/` directory
  - Set up subdirectories: architecture, api, standards, components, integrations
  - Create README explaining the documentation structure and purpose

  **Must NOT do**:
  - Create any git-tracked documentation files
  - Use non-AI-native directory naming

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple directory creation and basic file setup
  - **Skills**: []
    - No specialized skills needed for basic filesystem operations

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: All subsequent tasks
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - AGENTS.md:25-40 - Project structure documentation pattern

  **WHY Each Reference Matters**:
  - AGENTS.md shows the existing documentation approach and structure expectations

  **Acceptance Criteria**:
  - [ ] Directory `~/.docs/pratc/` exists
  - [ ] Subdirectories created: architecture, api, standards, components, integrations
  - [ ] README.md file created with basic structure explanation

  **QA Scenarios**:

  ```
  Scenario: Verify directory structure exists
    Tool: Bash
    Preconditions: None
    Steps:
      1. ls -la ~/.docs/pratc/
      2. Verify output contains: architecture/, api/, standards/, components/, integrations/, README.md
    Expected Result: All required directories and files exist
    Evidence: .sisyphus/evidence/task-1-dir-structure.txt

  Scenario: Verify git ignore status
    Tool: Bash  
    Preconditions: Working directory is /home/agent/pratc
    Steps:
      1. git check-ignore ~/.docs/pratc/
      2. Verify command returns the path (indicating it's ignored)
    Expected Result: ~/.docs/pratc/ is git-ignored
    Evidence: .sisyphus/evidence/task-1-git-ignore.txt
  ```

  **Evidence to Capture**:
  - [ ] Directory listing evidence
  - [ ] Git ignore verification evidence

  **Commit**: NO

- [x] 2. Audit codebase structure and components

  **What to do**:
  - Systematically map all components in prATC codebase
  - Identify Go packages, Python modules, TypeScript components
  - Document entry points, main functions, and component boundaries
  - Create component inventory with responsibilities and relationships

  **Must NOT do**:
  - Skip any component or module
  - Make assumptions about undocumented components

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires deep analysis of complex multi-language codebase structure
  - **Skills**: []
    - Core reasoning sufficient for structural analysis

  **Parallelization**:
  - **Can Run In Parallel**: YES  
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: Tasks 5-14 (all component-specific documentation)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - AGENTS.md:25-40 - Existing project structure documentation
  - cmd/pratc/*.go - CLI command entry points
  - internal/app/service.go - Core service facade
  - ml-service/src/pratc_ml/ - Python ML modules
  - web/src/ - Next.js dashboard structure

  **WHY Each Reference Matters**:
  - AGENTS.md provides the authoritative structure overview
  - Entry point files show actual command wiring and initialization
  - Service files reveal core business logic organization

  **Acceptance Criteria**:
  - [ ] Complete component inventory generated
  - [ ] All Go packages documented with responsibilities
  - [ ] All Python modules documented with responsibilities  
  - [ ] All TypeScript components documented with responsibilities
  - [ ] Component relationships mapped

  **QA Scenarios**:

  ```
  Scenario: Verify complete Go package coverage
    Tool: Bash
    Preconditions: None
    Steps:
      1. find /home/agent/pratc/internal -name "*.go" -type f | grep -v test | wc -l
      2. Compare count against documented Go packages in component inventory
    Expected Result: All Go source files accounted for in inventory
    Evidence: .sisyphus/evidence/task-2-go-coverage.txt

  Scenario: Verify complete Python module coverage
    Tool: Bash
    Preconditions: None
    Steps:
      1. find /home/agent/pratc/ml-service -name "*.py" -type f | grep -v __pycache__ | wc -l
      2. Compare count against documented Python modules in component inventory
    Expected Result: All Python source files accounted for in inventory
    Evidence: .sisyphus/evidence/task-2-python-coverage.txt
  ```

  **Evidence to Capture**:
  - [ ] Component inventory evidence
  - [ ] Coverage verification evidence

  **Commit**: NO

- [x] 3. Define AI-native specification format

  **What to do**:
  - Design structured format optimized for AI consumption
  - Create templates for architecture, API, and standards specifications
  - Define metadata schema for cross-referencing and versioning
  - Document format guidelines for consistent specification generation

  **Must NOT do**:
  - Use unstructured or purely human-readable formats
  - Create inconsistent templates across specification types

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires designing optimal machine-readable structures for AI processing
  - **Skills**: []
    - Core reasoning sufficient for format design

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: All specification generation tasks (5-20)
  - **Blocked By**: None (can start immediately)

  **References**:
  **External References**:
  - OpenAPI specification format - API contract structure inspiration
  - Architecture Decision Records (ADR) - Architecture documentation patterns

  **WHY Each Reference Matters**:
  - OpenAPI provides proven machine-readable API specification patterns
  - ADRs provide structured architecture documentation approaches

  **Acceptance Criteria**:
  - [ ] AI-native format specification document created
  - [ ] Templates defined for all specification types (architecture, API, standards)
  - [ ] Metadata schema includes component references and version info
  - [ ] Format guidelines document created

  **QA Scenarios**:

  ```
  Scenario: Verify format structure validity
    Tool: Bash
    Preconditions: Format specification file exists at ~/.docs/pratc/format-spec.yaml
    Steps:
      1. Check that format spec contains required sections: metadata, structure, examples
      2. Verify templates exist for architecture, api, and standards types
    Expected Result: Format specification is complete and well-structured
    Evidence: .sisyphus/evidence/task-3-format-valid.txt

  Scenario: Verify machine-readability
    Tool: Bash
    Preconditions: Format specification file exists
    Steps:
      1. Attempt to parse format spec as YAML/JSON
      2. Verify no syntax errors in structured sections
    Expected Result: Format specification is parseable by machines
    Evidence: .sisyphus/evidence/task-3-machine-readable.txt
  ```

  **Evidence to Capture**:
  - [ ] Format specification evidence
  - [ ] Parsing validation evidence

  **Commit**: NO

- [x] 4. Document shared types and cross-cutting concerns

  **What to do**:
  - Document shared type definitions across Go, Python, TypeScript
  - Map configuration flow from env vars through all layers
  - Document cross-cutting patterns (error handling, logging, testing)
  - Create specification for shared contracts and conventions

  **Must NOT do**:
  - Document types in isolation without cross-language mapping
  - Miss any shared concern or cross-cutting pattern

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding complex cross-language type consistency and patterns
  - **Skills**: []
    - Core reasoning sufficient for cross-cutting concern analysis

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: Tasks 5-14 (component documentation depends on shared context)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - internal/types/models.go - Go type definitions
  - ml-service/src/pratc_ml/models.py - Python type definitions  
  - web/src/types/api.ts - TypeScript type definitions
  - AGENTS.md:70-85 - Cross-cutting patterns documentation
  - internal/cmd/root.go - Configuration flow entry point

  **WHY Each Reference Matters**:
  - Type definition files show the actual shared contracts
  - AGENTS.md documents the intended cross-cutting patterns
  - Configuration files show the actual flow implementation

  **Acceptance Criteria**:
  - [ ] Shared types specification created with cross-language mappings
  - [ ] Configuration flow documented end-to-end
  - [ ] Cross-cutting patterns documented (error handling, testing, etc.)
  - [ ] Shared contracts specification created

  **QA Scenarios**:

  ```
  Scenario: Verify type consistency across languages
    Tool: Bash
    Preconditions: Shared types specification exists
    Steps:
      1. Extract type names from Go models.go
      2. Verify each type is documented in shared types spec with Python/TS equivalents
    Expected Result: All shared types have cross-language documentation
    Evidence: .sisyphus/evidence/task-4-type-consistency.txt

  Scenario: Verify configuration flow completeness
    Tool: Bash
    Preconditions: Configuration flow documentation exists
    Steps:
      1. Trace env var → Go → Python → React flow in documentation
      2. Verify all connection points are documented
    Expected Result: Complete end-to-end configuration flow documented
    Evidence: .sisyphus/evidence/task-4-config-flow.txt
  ```

  **Evidence to Capture**:
  - [ ] Type consistency evidence
  - [ ] Configuration flow evidence

  **Commit**: NO

- [x] 5. Document Go CLI architecture and commands

  **What to do**:
  - Document CLI command structure and wiring
  - Map all available commands and their options
  - Document command execution flow and error handling
  - Create specification for CLI interface contracts

  **Must NOT do**:
  - Miss any CLI command or option
  - Document incorrect command behavior

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires deep understanding of Go CLI architecture and cobra command structure
  - **Skills**: []
    - Core reasoning sufficient for CLI documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6, 7, 8, 9)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - cmd/pratc/*.go - CLI command implementations
  - AGENTS.md:130-145 - CLI command documentation
  - internal/cmd/root.go - HTTP server and API routes

  **WHY Each Reference Matters**:
  - Command files show actual implementation details
  - AGENTS.md provides intended behavior documentation
  - Root.go shows HTTP API integration points

  **Acceptance Criteria**:
  - [ ] Complete CLI command specification created
  - [ ] All command options and flags documented
  - [ ] Command execution flow documented
  - [ ] Error handling behavior specified

  **QA Scenarios**:

  ```
  Scenario: Verify CLI command completeness
    Tool: Bash
    Preconditions: CLI specification exists
    Steps:
      1. Extract command names from cmd/pratc/*.go files
      2. Verify each command is documented in CLI specification
    Expected Result: All CLI commands documented with complete options
    Evidence: .sisyphus/evidence/task-5-cli-complete.txt

  Scenario: Verify command contract accuracy
    Tool: Bash
    Preconditions: CLI specification exists
    Steps:
      1. Run pratc --help and compare output against documented commands
      2. Verify all documented options match actual implementation
    Expected Result: CLI specification matches actual behavior
    Evidence: .sisyphus/evidence/task-5-cli-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] CLI command completeness evidence
  - [ ] Command contract accuracy evidence

  **Commit**: NO

- [x] 6. Document internal Go packages and services

  **What to do**:
  - Document all internal Go packages (app, planning, formula, filter, etc.)
  - Map service layer architecture and business logic flow
  - Document package responsibilities and interfaces
  - Create specifications for core algorithms and data structures

  **Must NOT do**:
  - Skip any internal package or service
  - Document incorrect package responsibilities

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires deep analysis of complex Go service architecture and algorithms
  - **Skills**: []
    - Core reasoning sufficient for service documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 7, 8, 9)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - internal/app/service.go - Core service facade
  - internal/planning/ - Pool selection and planning logic
  - internal/formula/ - Combinatorial engine
  - internal/filter/ - Pre-filter pipeline
  - AGENTS.md:55-70 - Internal package documentation

  **WHY Each Reference Matters**:
  - Service files show actual business logic implementation
  - Planning/formula packages contain core algorithms
  - AGENTS.md provides high-level package descriptions

  **Acceptance Criteria**:
  - [ ] Complete internal package specification created
  - [ ] Service layer architecture documented
  - [ ] Package interfaces and responsibilities specified
  - [ ] Core algorithm specifications created

  **QA Scenarios**:

  ```
  Scenario: Verify internal package coverage
    Tool: Bash
    Preconditions: Internal package specification exists
    Steps:
      1. List all directories in internal/
      2. Verify each package is documented in specification
    Expected Result: All internal packages documented with responsibilities
    Evidence: .sisyphus/evidence/task-6-packages-complete.txt

  Scenario: Verify service layer accuracy
    Tool: Bash
    Preconditions: Service layer specification exists
    Steps:
      1. Review service.go method signatures and behavior
      2. Verify specification matches actual service interface
    Expected Result: Service specification matches implementation
    Evidence: .sisyphus/evidence/task-6-service-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Package coverage evidence
  - [ ] Service accuracy evidence

  **Commit**: NO

- [x] 7. Document SQLite cache and migrations

  **What to do**:
  - Document SQLite database schema and table structure
  - Map migration policy and versioning strategy
  - Document required tables and their relationships
  - Create specification for cache operations and constraints

  **Must NOT do**:
  - Miss any required database tables or columns
  - Document incorrect migration policies

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding of database schema design and migration strategies
  - **Skills**: []
    - Core reasoning sufficient for database documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 8, 9)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - internal/cache/sqlite.go - SQLite implementation and migrations
  - AGENTS.md:165-175 - SQLite migration policy
  - fixtures/ - Test data showing expected schema

  **WHY Each Reference Matters**:
  - SQLite.go shows actual implementation details
  - AGENTS.md documents migration policy requirements
  - Fixtures show real-world data examples

  **Acceptance Criteria**:
  - [ ] Complete SQLite schema specification created
  - [ ] Migration policy documented with versioning strategy
  - [ ] Required tables and relationships specified
  - [ ] Cache operation constraints documented

  **QA Scenarios**:

  ```
  Scenario: Verify schema completeness
    Tool: Bash
    Preconditions: SQLite schema specification exists
    Steps:
      1. Extract table definitions from sqlite.go
      2. Verify all tables documented in schema specification
    Expected Result: Complete SQLite schema documented
    Evidence: .sisyphus/evidence/task-7-schema-complete.txt

  Scenario: Verify migration policy accuracy
    Tool: Bash
    Preconditions: Migration policy specification exists
    Steps:
      1. Compare against AGENTS.md migration requirements
      2. Verify forward-only, idempotent policies documented
    Expected Result: Migration policy matches documented requirements
    Evidence: .sisyphus/evidence/task-7-migration-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Schema completeness evidence
  - [ ] Migration policy accuracy evidence

  **Commit**: NO

- [x] 8. Document GitHub client and rate-limiting

  **What to do**:
  - Document GitHub GraphQL client implementation
  - Map rate-limiting policies and retry strategies
  - Document authentication flow and token handling
  - Create specification for GitHub API interactions and constraints

  **Must NOT do**:
  - Miss any rate-limiting policy details
  - Document incorrect authentication handling

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding of external API integration patterns and rate-limiting strategies
  - **Skills**: []
    - Core reasoning sufficient for API integration documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7, 9)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - internal/github/client.go - GitHub client implementation
  - AGENTS.md:177-185 - GitHub rate-limit policy
  - internal/ml/bridge.go - Go→Python IPC (for auth context)

  **WHY Each Reference Matters**:
  - Client.go shows actual GitHub integration implementation
  - AGENTS.md documents rate-limiting requirements
  - ML bridge shows authentication flow through system

  **Acceptance Criteria**:
  - [ ] Complete GitHub client specification created
  - [ ] Rate-limiting policies documented with retry strategies
  - [ ] Authentication flow documented end-to-end
  - [ ] GitHub API constraints and limitations specified

  **QA Scenarios**:

  ```
  Scenario: Verify rate-limit policy completeness
    Tool: Bash
    Preconditions: Rate-limit specification exists
    Steps:
      1. Extract rate-limit constants and logic from client.go
      2. Verify all policies documented (reserve threshold, backoff, retries)
    Expected Result: Complete rate-limiting policy documented
    Evidence: .sisyphus/evidence/task-8-rate-limit-complete.txt

  Scenario: Verify authentication flow accuracy
    Tool: Bash
    Preconditions: Authentication specification exists
    Steps:
      1. Trace token handling from env var through GitHub client
      2. Verify flow matches actual implementation
    Expected Result: Authentication flow accurately documented
    Evidence: .sisyphus/evidence/task-8-auth-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Rate-limit completeness evidence
  - [ ] Authentication accuracy evidence

  **Commit**: NO

- [x] 9. Document Go conventions and standards

  **What to do**:
  - Document Go error wrapping conventions
  - Map testing patterns and table-driven test structure
  - Document interface design and constructor patterns
  - Create specification for Go coding standards and best practices

  **Must NOT do**:
  - Miss any Go convention or standard
  - Document incorrect testing patterns

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding of Go-specific conventions and idioms
  - **Skills**: []
    - Core reasoning sufficient for standards documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7, 8)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - AGENTS.md:190-200 - Go conventions documentation
  - internal/testutil/ - Test fixture patterns
  - cmd/pratc/init.go - Constructor and interface patterns

  **WHY Each Reference Matters**:
  - AGENTS.md provides authoritative Go standards
  - Test utilities show actual testing patterns
  - Command files show constructor and interface usage

  **Acceptance Criteria**:
  - [ ] Complete Go conventions specification created
  - [ ] Error wrapping patterns documented
  - [ ] Testing patterns and structures specified
  - [ ] Interface and constructor patterns documented

  **QA Scenarios**:

  ```
  Scenario: Verify Go conventions completeness
    Tool: Bash
    Preconditions: Go conventions specification exists
    Steps:
      1. Extract conventions from AGENTS.md Go section
      2. Verify all documented conventions included in specification
    Expected Result: Complete Go standards documented
    Evidence: .sisyphus/evidence/task-9-go-conventions-complete.txt

  Scenario: Verify testing pattern accuracy
    Tool: Bash
    Preconditions: Testing specification exists
    Steps:
      1. Review test files for table-driven patterns
      2. Verify specification matches actual test structure
    Expected Result: Testing patterns accurately documented
    Evidence: .sisyphus/evidence/task-9-testing-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Conventions completeness evidence
  - [ ] Testing accuracy evidence

  **Commit**: NO

- [x] 10. Document Python ML service architecture

  **What to do**:
  - Document Python ML service structure and modules
  - Map clustering, duplicates, overlap algorithms
  - Document ML provider integration patterns
  - Create specification for Python service architecture and contracts

  **Must NOT do**:
  - Skip any Python module or algorithm
  - Document incorrect ML service behavior

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding of ML service architecture and Python patterns
  - **Skills**: []
    - Core reasoning sufficient for ML service documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 11, 12, 13, 14)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - ml-service/src/pratc_ml/ - Python ML modules
  - ml-service/src/pratc_ml/models.py - Pydantic models with bootstrap fallback
  - AGENTS.md:42-48 - ML service documentation
  - internal/ml/bridge.go - Go→Python IPC integration

  **WHY Each Reference Matters**:
  - ML source shows actual algorithm implementations
  - Models.py shows type consistency with Go/TS
  - AGENTS.md provides high-level ML service description
  - Bridge.go shows integration points

  **Acceptance Criteria**:
  - [ ] Complete Python ML service specification created
  - [ ] ML algorithms documented with inputs/outputs
  - [ ] Provider integration patterns specified
  - [ ] Service architecture and module responsibilities documented

  **QA Scenarios**:

  ```
  Scenario: Verify ML module coverage
    Tool: Bash
    Preconditions: ML service specification exists
    Steps:
      1. List Python modules in ml-service/src/pratc_ml/
      2. Verify each module documented in specification
    Expected Result: Complete ML service architecture documented
    Evidence: .sisyphus/evidence/task-10-ml-modules-complete.txt

  Scenario: Verify algorithm contract accuracy
    Tool: Bash
    Preconditions: Algorithm specification exists
    Steps:
      1. Review ML function signatures and behavior
      2. Verify specification matches actual contracts
    Expected Result: ML algorithm contracts accurately documented
    Evidence: .sisyphus/evidence/task-10-algorithm-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] ML module coverage evidence
  - [ ] Algorithm contract accuracy evidence

  **Commit**: NO

- [x] 11. Document Go↔Python IPC contracts

  **What to do**:
  - Document JSON stdin/stdout IPC protocol
  - Map action types (health, cluster, duplicates, overlap)
  - Document request/response payload structures
  - Create specification for cross-language communication contracts

  **Must NOT do**:
  - Miss any IPC action type or payload field
  - Document incorrect payload structures

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding of cross-language IPC protocols and JSON contracts
  - **Skills**: []
    - Core reasoning sufficient for IPC documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 12, 13, 14)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - internal/ml/bridge.go - Go→Python bridge implementation
  - ml-service/src/pratc_ml/ - Python IPC handling
  - AGENTS.md:80-85 - Go↔Python IPC documentation

  **WHY Each Reference Matters**:
  - Bridge.go shows actual IPC implementation
  - Python code shows request/response handling
  - AGENTS.md documents intended IPC behavior

  **Acceptance Criteria**:
  - [ ] Complete IPC protocol specification created
  - [ ] All action types documented with payloads
  - [ ] Request/response structures specified
  - [ ] Error handling and timeout behavior documented

  **QA Scenarios**:

  ```
  Scenario: Verify IPC action completeness
    Tool: Bash
    Preconditions: IPC specification exists
    Steps:
      1. Extract action types from bridge.go and Python code
      2. Verify all actions documented in specification
    Expected Result: Complete IPC protocol documented
    Evidence: .sisyphus/evidence/task-11-ipc-actions-complete.txt

  Scenario: Verify payload structure accuracy
    Tool: Bash
    Preconditions: Payload specification exists
    Steps:
      1. Review actual JSON payloads in code
      2. Verify specification matches actual structures
    Expected Result: IPC payload structures accurately documented
    Evidence: .sisyphus/evidence/task-11-payload-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] IPC action completeness evidence
  - [ ] Payload structure accuracy evidence

  **Commit**: NO

- [x] 12. Document Next.js dashboard architecture

  **What to do**:
  - Document Next.js App Router structure and page organization
  - Map component hierarchy and data flow patterns
  - Document API route integration with Go backend
  - Create specification for frontend architecture and state management

  **Must NOT do**:
  - Skip any major dashboard component or route
  - Document incorrect data flow patterns

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Requires understanding of frontend architecture, component hierarchies, and UI patterns
  - **Skills**: []
    - Core reasoning sufficient for frontend documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 11, 13, 14)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - web/src/ - Next.js dashboard source
  - web/src/pages/ - Page routing structure
  - web/src/components/ - Component organization
  - AGENTS.md:42-48 - Web dashboard documentation

  **WHY Each Reference Matters**:
  - Web source shows actual frontend implementation
  - Page structure reveals routing and navigation patterns
  - AGENTS.md provides high-level dashboard description

  **Acceptance Criteria**:
  - [ ] Complete dashboard architecture specification created
  - [ ] Component hierarchy and relationships documented
  - [ ] Data flow patterns specified (API calls, state management)
  - [ ] Page routing and navigation documented

  **QA Scenarios**:

  ```
  Scenario: Verify dashboard component coverage
    Tool: Bash
    Preconditions: Dashboard specification exists
    Steps:
      1. List major components in web/src/components/
      2. Verify each component documented in specification
    Expected Result: Complete dashboard architecture documented
    Evidence: .sisyphus/evidence/task-12-dashboard-complete.txt

  Scenario: Verify API integration accuracy
    Tool: Bash
    Preconditions: API integration specification exists
    Steps:
      1. Review API calls in web/src/lib/
      2. Verify specification matches actual integration patterns
    Expected Result: API integration accurately documented
    Evidence: .sisyphus/evidence/task-12-api-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Component coverage evidence
  - [ ] API integration accuracy evidence

  **Commit**: NO

- [x] 13. Document TypeScript types and conventions

  **What to do**:
  - Document shared TypeScript types and interfaces
  - Map type consistency with Go/Python counterparts
  - Document frontend-specific type patterns and utilities
  - Create specification for TypeScript coding standards

  **Must NOT do**:
  - Miss any shared type definitions
  - Document incorrect type mappings

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Requires understanding of TypeScript type systems and frontend patterns
  - **Skills**: []
    - Core reasoning sufficient for type documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 11, 12, 14)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - web/src/types/api.ts - Shared API types
  - internal/types/models.go - Go type definitions
  - ml-service/src/pratc_ml/models.py - Python type definitions
  - AGENTS.md:70-75 - Type consistency documentation

  **WHY Each Reference Matters**:
  - API types show frontend type definitions
  - Go/Python files show cross-language consistency requirements
  - AGENTS.md documents type consistency expectations

  **Acceptance Criteria**:
  - [ ] Complete TypeScript types specification created
  - [ ] Cross-language type mappings documented
  - [ ] Frontend-specific type patterns specified
  - [ ] TypeScript conventions documented

  **QA Scenarios**:

  ```
  Scenario: Verify type consistency completeness
    Tool: Bash
    Preconditions: Type specification exists
    Steps:
      1. Extract type names from web/src/types/api.ts
      2. Verify each type has cross-language mapping documented
    Expected Result: Complete type consistency documented
    Evidence: .sisyphus/evidence/task-13-types-complete.txt

  Scenario: Verify frontend type accuracy
    Tool: Bash
    Preconditions: Frontend type specification exists
    Steps:
      1. Review actual TypeScript types in web/src/
      2. Verify specification matches actual type definitions
    Expected Result: TypeScript types accurately documented
    Evidence: .sisyphus/evidence/task-13-frontend-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Type consistency evidence
  - [ ] Frontend type accuracy evidence

  **Commit**: NO

- [x] 14. Document web build and test processes

  **What to do**:
  - Document Next.js build configuration and optimization
  - Map testing setup and patterns (Vitest, Playwright)
  - Document development workflow and debugging setup
  - Create specification for web build and test standards

  **Must NOT do**:
  - Miss any build configuration details
  - Document incorrect test patterns

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Requires understanding of frontend build systems and testing frameworks
  - **Skills**: []
    - Core reasoning sufficient for build/test documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 11, 12, 13)
  - **Blocks**: Task 15 (system-wide API contracts)
  - **Blocked By**: Tasks 1, 2, 3, 4 (foundation tasks)

  **References**:
  **Pattern References**:
  - web/package.json - Build scripts and dependencies
  - web/vitest.config.ts - Test configuration
  - AGENTS.md:125-130 - Web test documentation
  - makefile - Build commands

  **WHY Each Reference Matters**:
  - Package.json shows actual build scripts
  - Vitest config shows testing setup
  - AGENTS.md documents test requirements
  - Makefile shows integrated build process

  **Acceptance Criteria**:
  - [ ] Complete web build specification created
  - [ ] Testing setup and patterns documented
  - [ ] Development workflow documented
  - [ ] Build optimization strategies specified

  **QA Scenarios**:

  ```
  Scenario: Verify build process completeness
    Tool: Bash
    Preconditions: Build specification exists
    Steps:
      1. Extract build commands from web/package.json and makefile
      2. Verify all build steps documented in specification
    Expected Result: Complete build process documented
    Evidence: .sisyphus/evidence/task-14-build-complete.txt

  Scenario: Verify test setup accuracy
    Tool: Bash
    Preconditions: Test specification exists
    Steps:
      1. Review vitest.config.ts and test files
      2. Verify specification matches actual test setup
    Expected Result: Test setup accurately documented
    Evidence: .sisyphus/evidence/task-14-test-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Build process completeness evidence
  - [ ] Test setup accuracy evidence

  **Commit**: NO

- [x] 15. Document system-wide API contracts

  **What to do**:
  - Document CLI command contracts and output formats
  - Map HTTP API routes and response structures
  - Document IPC contracts between Go and Python
  - Create unified specification for all system APIs

  **Must NOT do**:
  - Miss any API endpoint or contract
  - Document incorrect response formats

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires synthesizing cross-component API contracts into unified specification
  - **Skills**: []
    - Core reasoning sufficient for API contract documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 16, 17, 18, 19, 20)
  - **Blocks**: Task 19 (verification)
  - **Blocked By**: Tasks 5-14 (all component documentation)

  **References**:
  **Pattern References**:
  - cmd/pratc/*.go - CLI command implementations
  - internal/cmd/root.go - HTTP API routes
  - internal/ml/bridge.go - IPC contracts
  - AGENTS.md:130-160 - API contract documentation
  - README.md - CLI and API examples

  **WHY Each Reference Matters**:
  - Command files show actual CLI contracts
  - Root.go shows HTTP API structure
  - Bridge.go shows IPC contracts
  - AGENTS.md and README provide contract specifications

  **Acceptance Criteria**:
  - [ ] Complete CLI API specification created
  - [ ] HTTP API routes and responses documented
  - [ ] IPC contracts specified with payloads
  - [ ] Unified API contract specification created

  **QA Scenarios**:

  ```
  Scenario: Verify CLI contract completeness
    Tool: Bash
    Preconditions: CLI API specification exists
    Steps:
      1. Run pratc --help and compare against documented commands
      2. Verify all CLI contracts match specification
    Expected Result: Complete CLI API contracts documented
    Evidence: .sisyphus/evidence/task-15-cli-complete.txt

  Scenario: Verify HTTP API accuracy
    Tool: Bash
    Preconditions: HTTP API specification exists
    Steps:
      1. Review root.go API routes
      2. Verify specification matches actual routes and responses
    Expected Result: HTTP API contracts accurately documented
    Evidence: .sisyphus/evidence/task-15-http-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] CLI contract completeness evidence
  - [ ] HTTP API accuracy evidence

  **Commit**: NO

- [x] 16. Document external integrations (GitHub, Docker)

  **What to do**:
  - Document GitHub API integration patterns and constraints
  - Map Docker Compose profiles and container orchestration
  - Document environment variable configuration and secrets handling
  - Create specification for external integration contracts

  **Must NOT do**:
  - Miss any external integration point
  - Document incorrect integration patterns

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding of external system integration patterns and deployment configurations
  - **Skills**: []
    - Core reasoning sufficient for integration documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 15, 17, 18, 19, 20)
  - **Blocks**: Task 19 (verification)
  - **Blocked By**: Tasks 5-14 (all component documentation)

  **References**:
  **Pattern References**:
  - docker-compose.yml - Docker Compose configuration
  - internal/github/client.go - GitHub integration
  - AGENTS.md:205-220 - External integration documentation
  - README.md - Docker profiles and env vars

  **WHY Each Reference Matters**:
  - Docker Compose shows actual deployment configuration
  - GitHub client shows API integration details
  - AGENTS.md documents integration requirements
  - README shows user-facing integration instructions

  **Acceptance Criteria**:
  - [ ] Complete GitHub integration specification created
  - [ ] Docker Compose profiles documented with services
  - [ ] Environment configuration documented
  - [ ] External integration contracts specified

  **QA Scenarios**:

  ```
  Scenario: Verify Docker profile completeness
    Tool: Bash
    Preconditions: Docker specification exists
    Steps:
      1. Extract profiles from docker-compose.yml
      2. Verify all profiles documented in specification
    Expected Result: Complete Docker integration documented
    Evidence: .sisyphus/evidence/task-16-docker-complete.txt

  Scenario: Verify GitHub integration accuracy
    Tool: Bash
    Preconditions: GitHub specification exists
    Steps:
      1. Review GitHub client implementation
      2. Verify specification matches actual integration patterns
    Expected Result: GitHub integration accurately documented
    Evidence: .sisyphus/evidence/task-16-github-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Docker profile completeness evidence
  - [ ] GitHub integration accuracy evidence

  **Commit**: NO

- [x] 17. Document performance SLOs and thresholds

  **What to do**:
  - Document performance SLOs for all operations (analyze, cluster, graph, plan)
  - Map threshold values (duplicateThreshold=0.90, overlapThreshold=0.70)
  - Document resource constraints and scaling expectations
  - Create specification for performance requirements and limits

  **Must NOT do**:
  - Miss any performance SLO or threshold
  - Document incorrect threshold values

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires understanding of performance requirements and system constraints
  - **Skills**: []
    - Core reasoning sufficient for performance documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 15, 16, 18, 19, 20)
  - **Blocks**: Task 19 (verification)
  - **Blocked By**: Tasks 5-14 (all component documentation)

  **References**:
  **Pattern References**:
  - AGENTS.md:187-195 - Performance SLOs documentation
  - internal/app/service.go - Threshold definitions
  - README.md - Performance examples

  **WHY Each Reference Matters**:
  - AGENTS.md provides authoritative SLO documentation
  - Service.go shows actual threshold implementations
  - README shows real-world performance expectations

  **Acceptance Criteria**:
  - [ ] Complete performance SLO specification created
  - [ ] All threshold values documented with purposes
  - [ ] Resource constraints specified (memory, time)
  - [ ] Scaling expectations documented

  **QA Scenarios**:

  ```
  Scenario: Verify SLO completeness
    Tool: Bash
    Preconditions: Performance specification exists
    Steps:
      1. Extract SLOs from AGENTS.md
      2. Verify all SLOs documented in specification
    Expected Result: Complete performance requirements documented
    Evidence: .sisyphus/evidence/task-17-slo-complete.txt

  Scenario: Verify threshold accuracy
    Tool: Bash
    Preconditions: Threshold specification exists
    Steps:
      1. Extract threshold values from service.go
      2. Verify specification matches actual values
    Expected Result: Threshold values accurately documented
    Evidence: .sisyphus/evidence/task-17-threshold-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] SLO completeness evidence
  - [ ] Threshold accuracy evidence

  **Commit**: NO

- [x] 18. Generate cross-component relationship maps

  **What to do**:
  - Create dependency graphs showing component relationships
  - Map data flow between Go backend, Python ML, and TypeScript frontend
  - Document configuration flow from env vars through all layers
  - Generate visual and structured relationship specifications

  **Must NOT do**:
  - Miss any major component relationship
  - Create inaccurate dependency mappings

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires synthesizing cross-component relationships into comprehensive maps
  - **Skills**: []
    - Core reasoning sufficient for relationship mapping

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 15, 16, 17, 19, 20)
  - **Blocks**: Task 19 (verification)
  - **Blocked By**: Tasks 5-14 (all component documentation)

  **References**:
  **Pattern References**:
  - internal/graph/ - Dependency graph implementation
  - internal/cmd/root.go - Configuration flow
  - internal/ml/bridge.go - Go→Python data flow
  - web/src/lib/ - Frontend→backend API calls

  **WHY Each Reference Matters**:
  - Graph package shows actual dependency analysis
  - Configuration files show env var flow
  - Bridge.go shows cross-language data flow
  - Frontend lib shows API consumption patterns

  **Acceptance Criteria**:
  - [ ] Complete component dependency map created
  - [ ] Data flow diagrams generated for all layers
  - [ ] Configuration flow documented end-to-end
  - [ ] Relationship specifications in AI-native format

  **QA Scenarios**:

  ```
  Scenario: Verify dependency map completeness
    Tool: Bash
    Preconditions: Dependency map exists
    Steps:
      1. List all major components from previous specifications
      2. Verify all relationships documented in dependency map
    Expected Result: Complete component relationships mapped
    Evidence: .sisyphus/evidence/task-18-dependencies-complete.txt

  Scenario: Verify data flow accuracy
    Tool: Bash
    Preconditions: Data flow specification exists
    Steps:
      1. Trace data flow from Go through Python to frontend
      2. Verify specification matches actual implementation
    Expected Result: Data flow accurately documented
    Evidence: .sisyphus/evidence/task-18-dataflow-accurate.txt
  ```

  **Evidence to Capture**:
  - [ ] Dependency map completeness evidence
  - [ ] Data flow accuracy evidence

  **Commit**: NO

- [x] 19. Verify specification completeness

  **What to do**:
  - Audit all generated specifications for completeness
  - Verify coverage of all prATC components and features
  - Check AI-native format consistency across all specifications
  - Validate that all requirements from original request are met

  **Must NOT do**:
  - Skip any specification file during verification
  - Accept incomplete or inconsistent specifications

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires comprehensive audit and validation of all generated specifications
  - **Skills**: []
    - Core reasoning sufficient for completeness verification

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 15, 16, 17, 18, 20)
  - **Blocks**: Task 20 (final validation)
  - **Blocked By**: Tasks 15-18 (integration documentation)

  **References**:
  **Pattern References**:
  - ~/.docs/pratc/ - All generated specifications
  - Original requirements from user request
  - AGENTS.md - Complete system reference

  **WHY Each Reference Matters**:
  - Generated specs are the audit target
  - Original requirements define success criteria
  - AGENTS.md provides complete system baseline

  **Acceptance Criteria**:
  - [ ] All specifications audited for completeness
  - [ ] 100% prATC component coverage confirmed
  - [ ] AI-native format consistency verified
  - [ ] All original requirements satisfied

  **QA Scenarios**:

  ```
  Scenario: Verify component coverage completeness
    Tool: Bash
    Preconditions: All specifications generated
    Steps:
      1. Compare component list from Task 2 against documented components
      2. Verify 100% coverage achieved
    Expected Result: Complete prATC system documented
    Evidence: .sisyphus/evidence/task-19-coverage-complete.txt

  Scenario: Verify AI-native format consistency
    Tool: Bash
    Preconditions: All specifications generated
    Steps:
      1. Check all spec files for consistent AI-native structure
      2. Verify machine-readable formats used throughout
    Expected Result: Consistent AI-native format across all specifications
    Evidence: .sisyphus/evidence/task-19-format-consistent.txt
  ```

  **Evidence to Capture**:
  - [ ] Component coverage evidence
  - [ ] Format consistency evidence

  **Commit**: NO

- [x] 20. Final format validation and cleanup

  **What to do**:
  - Validate all specifications use correct AI-native format
  - Clean up any redundant or duplicate content
  - Ensure consistent naming and structure across all files
  - Generate final README with navigation and usage instructions

  **Must NOT do**:
  - Leave inconsistent formatting or structure
  - Include redundant or conflicting information

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Final cleanup and validation tasks that are straightforward
  - **Skills**: []
    - No specialized skills needed for cleanup tasks

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 15, 16, 17, 18, 19)
  - **Blocks**: Final Verification Wave
  - **Blocked By**: Task 19 (completeness verification)

  **References**:
  **Pattern References**:
  - ~/.docs/pratc/format-spec.yaml - AI-native format specification
  - All generated specification files

  **WHY Each Reference Matters**:
  - Format spec defines the correct structure
  - Generated files need final validation against this standard

  **Acceptance Criteria**:
  - [ ] All specifications validated against AI-native format
  - [ ] Redundant content removed
  - [ ] Consistent naming and structure applied
  - [ ] Final README with navigation created

  **QA Scenarios**:

  ```
  Scenario: Verify final format compliance
    Tool: Bash
    Preconditions: All specifications cleaned up
    Steps:
      1. Validate all spec files against format-spec.yaml
      2. Verify no format violations exist
    Expected Result: All specifications compliant with AI-native format
    Evidence: .sisyphus/evidence/task-20-format-compliant.txt

  Scenario: Verify README completeness
    Tool: Bash
    Preconditions: Final README created
    Steps:
      1. Check README includes navigation to all specification sections
      2. Verify usage instructions are clear and complete
    Expected Result: Comprehensive README with proper navigation
    Evidence: .sisyphus/evidence/task-20-readme-complete.txt
  ```

  **Evidence to Capture**:
  - [ ] Format compliance evidence
  - [ ] README completeness evidence

  **Commit**: NO

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [x] F1. **Plan Compliance Audit** — `oracle` — **APPROVE**
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [5/5] | Must NOT Have [5/5] | Tasks [20/20] | VERDICT: APPROVE`

- [x] F2. **Code Quality Review** — `unspecified-high` — **PASS**
  Run `tsc --noEmit` + linter + `bun test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Format [PASS] | Files [21/21 clean] | Parse Errors [0] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill if UI) — **PASS**
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [51/58 pass] | Integration [PASS] | Edge Cases [PASS tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep` — **APPROVE**
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [2/2 compliant] | Contamination [CLEAN] | Unaccounted [CLEAN] | VERDICT`

---

## Commit Strategy

- **1**: `docs(pratc): add comprehensive AI-native specifications` — ~/.docs/pratc/, npm test

---

## Success Criteria

### Verification Commands
```bash
ls -la ~/.docs/pratc/  # Expected: architecture/, api/, standards/, components/, integrations/
find ~/.docs/pratc -name "*.yaml" | wc -l  # Expected: > 20 specification files
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Complete coverage of prATC system components
- [ ] AI-native format used consistently
- [ ] Git-ignored location confirmed