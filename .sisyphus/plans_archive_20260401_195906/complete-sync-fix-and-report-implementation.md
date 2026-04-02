# Complete Plan: Sync Job Tracking Fix + Comprehensive Report Implementation

## TL;DR

> **Quick Summary**: Fix the critical sync job tracking bug that's blocking all production work, then implement comprehensive 8-12 page PDF reports with real data sections and charts.
> 
> **Deliverables**:
> - CLI sync creates and tracks jobs properly (fix RegisterSyncCommand in internal/cmd/root.go)
> - Jobs marked completed/failed instead of stuck in_progress
> - Comprehensive PDF report (8-12 pages, >100KB) with Conflicts, Duplicates, Overlaps, Staleness, Clusters sections
> - Chart rendering for cluster sizes, staleness distribution, conflict severity  
> - Full pipeline verified: sync → analyze → cluster → graph → plan → report
> - Production execution on openclaw/openclaw (5,500+ PRs)
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: NO - sequential critical path
> **Critical Path**: Fix job tracking → Verify small repo → Implement comprehensive PDF → Run production

---

## Context

### Current Broken State (Validated)

| Issue | Status | Evidence |
|-------|--------|----------|
| CLI sync passes `("", "")` to newRepoSyncManager | ❌ CONFIRMED | `internal/cmd/root.go:630` |
| opencode-ai sync job stuck `in_progress` | ❌ CONFIRMED | Job ID: `opencode-ai/opencode-1774830908064383753` |
| Cache wiring works (42 PRs stored) | ✅ VERIFIED | `sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='opencode-ai/opencode';" → 42` |
| Analyze blocked by stale in_progress job | ❌ CONFIRMED | Returns "Sync in progress — background sync job started" |
| Report produces minimal PDF (2.4KB, 2 pages) | ❌ CONFIRMED | `/tmp/test-report.pdf` size and page count |
| Test infrastructure functional | ✅ VERIFIED | All test frameworks working |

### Root Cause Chain

```
1. RegisterSyncCommand (line 630) passes ("", "") → no job created/tracked
2. Job never marked complete/failed → stuck in_progress
3. Downstream commands see stale "in_progress" job → refuse to proceed
4. Report command works but only renders minimal data → 2.4KB PDF
```

### Research Findings

**Sync Job APIs** (`internal/cache/sqlite.go`):
- `CreateSyncJob(repo string) (*SyncJob, error)` - creates new job record
- `ResumeSyncJob(repo string) (*SyncJob, error)` - finds existing in_progress job  
- `MarkSyncJobComplete(jobID string) error` - marks job as completed
- `MarkSyncJobFailed(jobID string, errorMsg string) error` - marks job as failed

**Reference Pattern** (`internal/cmd/root.go:185-222` - startBackgroundSync):
- Opens cache store with `PRATC_DB_PATH` default
- Checks `ResumeSyncJob` first
- Creates new job with `CreateSyncJob` if none exists
- Passes `dbPath` and `job.ID` to manager
- Handles success/failure with proper job status updates

**PDF Requirements** (from report-website-full.md):
- Sections needed: Cover, Metrics, Conflicts Detail, Duplicates Detail, Overlaps Detail, Staleness Detail, Per-Cluster Summary, Recommendations
- Chart types: cluster bar chart, staleness histogram, conflict severity pie chart
- Target: 8-12 pages, >100KB file size

---

## Work Objectives

### Core Objective
Unblock production by fixing sync job tracking and implementing comprehensive PDF reports with real analysis data.

### Concrete Deliverables
- `internal/cmd/root.go` - RegisterSyncCommand properly creates and tracks sync jobs
- Verified: opencode-ai/opencode sync completes with "completed" status
- `internal/report/pdf.go` - Comprehensive PDFComposer with all data sections
- `internal/report/charts.go` - Chart rendering infrastructure with PNG generation
- Full pipeline: sync → analyze → cluster → graph → plan → report works end-to-end
- Production run on openclaw/openclaw (5,500+ PRs) with comprehensive report

### Definition of Done
- [ ] `./bin/pratc sync --repo=opencode-ai/opencode` → job created, status="completed"
- [ ] `./bin/pratc analyze --repo=opencode-ai/opencode --format=json` → returns real data (42 PRs)
- [ ] `./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=report.pdf` → PDF > 100KB, 8-12 pages
- [ ] `pdftotext report.pdf - | grep -q "Conflict Analysis"` → true
- [ ] `pdftotext report.pdf - | grep -q "Duplicate PRs"` → true  
- [ ] `pdftotext report.pdf - | grep -q "Staleness Signals"` → true
- [ ] `pdftotext report.pdf - | grep -q "Cluster Summary"` → true
- [ ] `./bin/pratc sync --repo=openclaw/openclaw` → completes successfully
- [ ] SQLite has ~5,500 PRs for openclaw/openclaw
- [ ] All tests pass: `go test ./...`, `bun test`, `uv run pytest`

### Must Have
- Sync job tracking fix applied to CLI (not just API)
- End-to-end verification on small repo first (opencode-ai/opencode)
- Comprehensive PDF with all planned sections populated with real data
- Chart rendering (PNG) embedded in PDF
- Full production run on openclaw/openclaw
- No breaking changes to existing CLI interface

### Must NOT Have (Guardrails from Metis Review)
- External ML API calls during sync (use local backend only)
- Breaking changes to CLI command interface
- Memory leaks or crashes during large sync operations
- Interactive charts on website (static preview only)
- Email delivery or scheduling features
- Custom report templates beyond planned sections
- Historical/week-over-week comparison features

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go tests, vitest, pytest all functional)
- **Automated tests**: Tests-after (verify existing tests pass, add new tests for fixes)
- **Framework**: Go's built-in testing package + vitest + pytest
- **QA Policy**: Every task includes agent-executed verification with evidence capture

### QA Scenarios Requirements
- **Frontend/UI**: Use Playwright for browser interactions
- **TUI/CLI**: Use interactive_bash (tmux) for command execution  
- **API/Backend**: Use Bash (curl) for HTTP requests
- **Library/Module**: Use Bash (go test, bun test) for unit tests
- **Evidence**: Save to `.sisyphus/evidence/complete-plan/task-{N}-{scenario}.txt`

---

## Execution Strategy

```
Wave 1 (CRITICAL: Fix sync job tracking — 2 tasks):
├── Task 1: Fix RegisterSyncCommand to create and track jobs properly
└── Task 2: Verify fix with small repo (opencode-ai/opencode)

Wave 2 (Implement comprehensive PDF — 6 tasks):
├── Task 3: Audit current PDFComposer data flow and gaps
├── Task 4: Implement ConflictPair section with real data
├── Task 5: Implement DuplicateGroup and Overlaps sections  
├── Task 6: Implement StalenessReport and ClusterSummary sections
├── Task 7: Add graceful "no issues" handling for empty sections
└── Task 8: Implement chart rendering infrastructure (PNG from data)

Wave 3 (Chart implementation — 3 tasks):
├── Task 9: Implement cluster size bar chart (PNG)
├── Task 10: Implement staleness distribution chart (PNG)  
└── Task 11: Implement conflict severity pie chart (PNG)

Wave 4 (Integration and verification — 4 tasks):
├── Task 12: Wire comprehensive PDF to CLI report command
├── Task 13: Verify full pipeline on small repo (opencode-ai/opencode)
├── Task 14: Execute cold sync on openclaw/openclaw (5,500+ PRs)
└── Task 15: Run full analysis pipeline on openclaw with comprehensive report

Wave FINAL (Verification — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high + playwright)
└── F4: Scope fidelity check (deep)
```

Critical Path: T1 → T2 → T3 → T4-T8 → T9-T11 → T12 → T13 → T14 → T15 → F1-F4
Max Concurrent: 2 (Wave 1), 6 (Wave 2), 3 (Wave 3), 4 (Wave 4), 4 (Final)

---

## TODOs

- [x] 1. Fix RegisterSyncCommand to create and track sync jobs properly

  **What to do**:
  - In `RegisterSyncCommand()` in `internal/cmd/root.go` (lines 620-676):
    - Open cache store using `PRATC_DB_PATH` (with default fallback `~/.pratc/pratc.db`)
    - Check for existing in-progress job via `cacheStore.ResumeSyncJob(repo)`
    - Create new sync job via `cacheStore.CreateSyncJob(repo)`
    - Pass `dbPath` and `job.ID` to `newRepoSyncManager(dbPath, job.ID)`
    - On success: job is already marked complete by `DefaultRunner.Run()`
    - On failure: mark job failed via `cacheStore.MarkSyncJobFailed(job.ID, err.Error())`
    - Update output to include `job_id` field alongside existing fields
  - Follow exact pattern from `startBackgroundSync` function (lines 185-222)

  **Must NOT do**:
  - Don't change the sync logic itself (already working after commit 93dc443)
  - Don't break `--no-wait` or `--watch` modes
  - Don't modify the return JSON structure (only add job_id field)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None required
  - **Reason**: Straightforward code modification following existing patterns

  **Parallelization**:
  - **Can Run In Parallel**: NO (first critical step)
  - **Blocks**: Task 2
  - **Blocked By**: None

  **References**:
  - `internal/cmd/root.go:620-676` - Current RegisterSyncCommand implementation
  - `internal/cmd/root.go:185-222` - startBackgroundSync (pattern to follow for job tracking)
  - `internal/cache/sqlite.go` - CreateSyncJob, ResumeSyncJob, MarkSyncJobComplete, MarkSyncJobFailed APIs
  - `internal/sync/default_runner.go` - DefaultRunner.Run() handles job completion marking

  **Acceptance Criteria**:
  - [ ] Sync command creates sync job record in SQLite
  - [ ] Job status updates to "completed" after successful sync
  - [ ] Job status updates to "failed" on sync error
  - [ ] Output includes `job_id` field
  - [ ] `go build ./...` → PASS
  - [ ] Existing tests continue to pass

  **QA Scenarios**:
  ```
  Scenario: Sync creates and completes job successfully
    Tool: Bash
    Preconditions: Fresh DB or clean state
    Steps:
      1. rm -f ~/.pratc/pratc.db  # Start fresh
      2. mkdir -p ~/.pratc
      3. ./bin/pratc sync --repo=opencode-ai/opencode
      4. sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs ORDER BY created_at DESC LIMIT 1;"
    Expected Result: Status is "completed", output contains job_id field
    Evidence: .sisyphus/evidence/complete-plan/task-1-job-tracking.txt

  Scenario: Analyze sees completed sync (not stale in_progress)
    Tool: Bash
    Preconditions: From previous scenario
    Steps:
      1. ./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts.total_prs'
    Expected Result: Returns 42 (not "sync in progress" error message)
    Evidence: .sisyphus/evidence/complete-plan/task-1-analyze-works.txt
  ```

  **Commit**: YES
  - Message: `fix(cmd): create and track sync jobs in CLI sync command`
  - Files: `internal/cmd/root.go`

  ---

- [x] 2. Verify fix with small repo (opencode-ai/opencode)

  **What to do**:
  - Run sync on opencode-ai/opencode (42 PRs)
  - Verify job completes successfully with "completed" status
  - Verify PRs saved to SQLite (count should be 42)
  - Run analyze to confirm data accessible and returns real data
  - Run full pipeline: analyze → cluster → graph → plan → report

  **Must NOT do**:
  - Don't modify code in this task (verification only)

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Wave 2 (Tasks 3-8)
  - **Blocked By**: Task 1

  **Acceptance Criteria**:
  - [ ] Sync completes with job_id in output
  - [ ] sqlite3 shows "completed" status for latest job
  - [ ] 42 PRs in pull_requests table
  - [ ] analyze returns valid JSON with counts
  - [ ] cluster returns clusters array
  - [ ] graph returns valid DOT
  - [ ] plan returns ranked selections

  **QA Scenarios**:
  ```
  Scenario: Small repo sync completes successfully
    Tool: Bash
    Steps:
      1. export GH_TOKEN=$(gh auth token)
      2. ./bin/pratc sync --repo=opencode-ai/opencode
      3. sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='opencode-ai/opencode';"
    Expected Result: Count is 42, job status is "completed"
    Evidence: .sisyphus/evidence/complete-plan/task-2-pr-count.txt

  Scenario: Full pipeline works end-to-end
    Tool: Bash
    Steps:
      1. time ./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts'
      2. time ./bin/pratc cluster --repo=opencode-ai/opencode --format=json | jq '.clusters | length'
      3. time ./bin/pratc graph --repo=opencode-ai/opencode --format=dot | head -3
      4. time ./bin/pratc plan --repo=opencode-ai/opencode --target=10 --format=json | jq '.selected | length'
    Expected Result: All return real data, times within reasonable limits
    Evidence: .sisyphus/evidence/complete-plan/task-2-pipeline.txt
  ```

  **Commit**: NO

  ---

- [x] 3. Audit current PDFComposer data flow and gaps

  **What to do**:
  - Read `internal/report/pdf.go` fully to trace how `ScalabilityMetrics` flows into each section
  - Read `internal/types/models.go` for `AnalysisResponse`, `ConflictPair`, `DuplicateGroup`, `StalenessReport`, `ClusterResults`
  - Read `internal/app/service.go` Analyze method to see what data is available
  - Document: which section currently receives which data, and which fields from AnalysisResponse are unconsumed
  - Identify exactly what's missing from current minimal PDF vs planned comprehensive PDF

  **Must NOT do**:
  - Don't modify any code yet — this is pure research

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Reason**: Straight codebase reading and documentation

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4-8 once findings are available)
  - **Parallel Group**: Wave 2 (first, unblocks all others)
  - **Blocks**: Tasks 4-8 (they need this data flow map)
  - **Blocked By**: Task 2

  **References**:
  - `internal/report/pdf.go` — PDFComposer, section methods, ScalabilityMetrics usage
  - `internal/types/models.go` — AnalysisResponse, ConflictPair, DuplicateGroup, StalenessReport, ClusterResults
  - `internal/app/service.go` — Analyze() returns all reportable data

  **Acceptance Criteria**:
  - [ ] Written audit document: "Data flow: AnalysisResponse → PDF section"
  - [ ] List of which AnalysisResponse fields are currently consumed by PDF
  - [ ] List of which AnalysisResponse fields are NOT yet consumed

  **Commit**: NO

  ---

- [x] 4. Implement ConflictPair section with real data

  **What to do**:
  - In `internal/report/pdf.go`, add a `ConflictSection` type that renders ConflictPair data
  - Add `Render(*fpdf.Fpdf)` method to ConflictSection, following the same pattern as existing sections
  - Layout: section title "Conflict Analysis", then for each ConflictPair: PR numbers, conflict type, severity (color-coded if possible), files touched, brief reason
  - If `Conflicts` slice is empty: render "No conflicts detected." (graceful handling)
  - Update `NewPDFComposer` signature to accept `Conflicts []ConflictPair`
  - Ensure section integrates properly with existing PDF structure

  **Must NOT do**:
  - Don't add chart rendering here — that is Wave 3
  - Don't add page breaks mid-section without checking total page count

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Reason**: Familiar Go PDF rendering pattern — follow existing section patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 5, 6, 7, 8 after T3 completes)
  - **Parallel Group**: Wave 2
  - **Blocks**: Nothing (independent section)
  - **Blocked By**: Task 3

  **References**:
  - `internal/report/pdf.go` — existing section implementations to copy pattern from (MetricsSection, CoverSection)
  - `internal/types/models.go` — ConflictPair struct fields: SourcePR, TargetPR, ConflictType, FilesTouched, Severity, Reason

  **Acceptance Criteria**:
  - [ ] `ConflictSection` type defined with `Render(*fpdf.Fpdf)` method
  - [ ] `NewPDFComposer` accepts Conflicts data
  - [ ] Empty conflicts renders "No conflicts detected."
  - [ ] `go build ./...` → PASS

  **QA Scenarios**:
  ```
  Scenario: PDF with conflicts renders conflict section
    Tool: Bash
    Preconditions: DB has test repo with known conflicts
    Steps:
      1. cd /home/agent/pratc && ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/conflict-test.pdf
      2. pdftotext /tmp/conflict-test.pdf - | grep -i "conflict"
    Expected Result: "Conflict Analysis" section present in PDF
    Evidence: .sisyphus/evidence/complete-plan/task-4-conflict-section.txt
  ```

  **Commit**: YES
  - Message: `feat(report): add ConflictPair section to PDF composer`
  - Files: `internal/report/pdf.go`

  ---

- [x] 5. Implement DuplicateGroup and Overlaps sections

  **What to do**:
  - In `internal/report/pdf.go`, add `DuplicatesSection` and `OverlapsSection` types
  - Render each DuplicateGroup: canonical PR number, list of duplicate PR numbers, similarity score, reason
  - Render Overlaps similarly (same data structure as Duplicates)
  - If empty: render "No duplicate PRs detected." and "No overlapping PRs detected."
  - Add to NewPDFComposer params
  - Ensure both sections follow same pattern as ConflictSection

  **Must NOT do**:
  - Don't add chart rendering — Wave 3 only

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4, 6, 7, 8 after T3)
  - **Blocks**: Nothing
  - **Blocked By**: Task 3

  **References**:
  - `internal/types/models.go` — DuplicateGroup: CanonicalPRNumber, DuplicatePRNums, Similarity, Reason
  - `internal/types/models.go` — Overlaps []DuplicateGroup (same shape as Duplicates)

  **Acceptance Criteria**:
  - [ ] DuplicatesSection and OverlapsSection defined + Render implemented
  - [ ] NewPDFComposer accepts Duplicates and Overlaps data
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: PDF renders duplicate and overlap sections
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/dup-overlap-test.pdf
      2. pdftotext /tmp/dup-overlap-test.pdf - | grep -i "duplicate\|overlap"
    Expected Result: Both sections present in PDF
    Evidence: .sisyphus/evidence/complete-plan/task-5-dup-overlap-sections.txt
  ```

  **Commit**: YES
  - Message: `feat(report): add DuplicateGroup and Overlaps sections to PDF composer`

  ---

- [x] 6. Implement StalenessReport and ClusterSummary sections

  **What to do**:
  - In `internal/report/pdf.go`, add `StalenessSection` and `ClusterSummarySection` types
  - StalenessSection: render top 20 stale PRs with PR number, staleness score, signals, reasons
  - ClusterSummarySection: render total cluster count, top 10 clusters by PR count with labels and descriptions
  - If empty: "No stale PRs detected." and "No clusters identified."
  - Add to NewPDFComposer params
  - Cap rendering at reasonable limits (20 stale PRs, 10 clusters) to avoid huge PDFs

  **Must NOT do**:
  - Don't render all stale PRs if there are hundreds — cap at 20
  - Don't render all clusters — cap at top 10 by PR count
  - Don't add per-cluster charts here — Wave 3

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4, 5, 7, 8 after T3)
  - **Blocks**: Nothing
  - **Blocked By**: Task 3

  **References**:
  - `internal/types/models.go` — StalenessReport: PRNumber, Score, Signals, Reasons, SupersededBy
  - `internal/types/models.go` — ClusterResults struct fields

  **Acceptance Criteria**:
  - [ ] StalenessSection and ClusterSummarySection defined + Render implemented
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: PDF renders staleness and cluster sections
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/stale-cluster-test.pdf
      2. pdftotext /tmp/stale-cluster-test.pdf - | grep -i "stale\|cluster"
    Expected Result: Both sections present in PDF
    Evidence: .sisyphus/evidence/complete-plan/task-6-stale-cluster-sections.txt
  ```

  **Commit**: YES
  - Message: `feat(report): add StalenessReport and ClusterSummary sections to PDF composer`

  ---

- [x] 7. Add graceful "no issues" handling for empty sections

  **What to do**:
  - Review all section Render methods (Cover, Metrics, Conflict, Duplicates, Overlaps, Staleness, Cluster, Recommendations)
  - Ensure every section that receives empty/nil data renders a graceful message instead of blank space
  - Check gofpdf page layout: ensure sections don't produce blank pages when data is empty
  - Verify PDF still opens and is valid even when all sections are empty
  - Test edge case: repository with no conflicts, duplicates, overlaps, staleness, or clusters

  **Must NOT do**:
  - Don't change section content — only add null/empty guards

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4-6, 8)
  - **Blocks**: Nothing
  - **Blocked By**: Task 3

  **References**:
  - `internal/report/pdf.go` — all section Render methods

  **Acceptance Criteria**:
  - [ ] Every section has null/empty guard
  - [ ] PDF with empty data is valid (opens, has content, no crashes)
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: PDF with completely empty data is valid
    Tool: Bash
    Preconditions: Use mock data with all empty slices
    Steps:
      1. Create test invoking PDFComposer with all nil/empty data
      2. Verify PDF composes without panic
      3. Verify PDF is valid (file command returns "PDF")
    Expected Result: Valid PDF with graceful "no issues" messages
    Evidence: .sisyphus/evidence/complete-plan/task-7-empty-graceful.txt
  ```

  **Commit**: YES
  - Message: `fix(report): add graceful empty-state handling to all sections`

  ---

- [x] 8. Implement chart rendering infrastructure (PNG from data)

  **What to do**:
  - Research: gofpdf cannot natively render Vega-Lite or complex charts
  - Choose approach: Go-native charting library (gonum/plot) for PNG generation
  - Implement chart renderer interface that each chart task will use
  - Create `internal/report/charts.go` with chart rendering functions
  - Ensure PNG output is embeddable in gofpdf via image embedding
  - Test with simple bar chart to verify approach works

  **Must NOT do**:
  - Don't pick a solution requiring external services (wkhtmltopdf daemon, etc.)
  - Don't pick a solution requiring a browser runtime
  - Don't add unnecessary dependencies

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Reason**: Needs genuine evaluation of trade-offs between approaches
  - **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9, 10, 11 after this decision is made)
  - **Parallel Group**: Wave 3
  - **Blocks**: Tasks 9, 10, 11
  - **Blocked By**: Wave 2 complete

  **References**:
  - `go.mod` — existing dependencies (don't add incompatible ones)
  - `internal/report/pdf.go` — how sections embed images (existing pattern)

  **Acceptance Criteria**:
  - [ ] Chart renderer interface defined
  - [ ] At least one working chart type (bar chart) renders into PNG
  - [ ] PNG is embeddable in gofpdf
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: Chart infrastructure generates embeddable PNG
    Tool: Bash
    Steps:
      1. Write simple test generating bar chart PNG from sample data
      2. Verify PNG file is created and valid
      3. Verify PNG can be embedded in test PDF
    Expected Result: Valid PNG file generated and embeddable
    Evidence: .sisyphus/evidence/complete-plan/task-8-chart-infrastructure.png
  ```

  **Commit**: YES
  - Message: `feat(report): add chart rendering infrastructure`
  - Files: `internal/report/charts.go`

  ---

- [x] 9. Implement cluster size bar chart (PNG)

  **What to do**:
  - Consume ClusterResults: cluster label + PR count
  - Render bar chart: X-axis = cluster labels (truncated), Y-axis = PR count
  - Use chart infrastructure from Task 8
  - Embed in PDF via gofpdf's image embedding
  - Add ChartsSection to PDFComposer (or extend existing ChartsSection)
  - Show top 20 clusters by PR count to avoid clutter

  **Must NOT do**:
  - Don't render all clusters if >20 — show top 20 by PR count
  - Don't use external services

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Reason**: Follow chart renderer interface from Task 8, straightforward bar chart

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 8 done, T10, T11 can proceed)
  - **Blocks**: Nothing
  - **Blocked By**: Task 8

  **References**:
  - `internal/types/models.go` — ClusterResults for cluster labels and PR counts
  - `internal/report/pdf.go` — how sections embed images (existing pattern)

  **Acceptance Criteria**:
  - [ ] Bar chart PNG generated and embedded in test PDF
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: PDF with clusters renders bar chart
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/chart-cluster.pdf
      2. Extract image: pdftoppm /tmp/chart-cluster.pdf /tmp/chart-page -png -f 3 -l 3
      3. Check if image file exists: ls -la /tmp/chart-page*.png
    Expected Result: PNG file generated from PDF page 3
    Evidence: .sisyphus/evidence/complete-plan/task-9-cluster-chart.png
  ```

  **Commit**: YES
  - Message: `feat(report): add cluster size bar chart to PDF`

  ---

- [x] 10. Implement staleness distribution chart (PNG)

  **What to do**:
  - Consume StalenessReport data: distribution of staleness scores
  - Render: histogram showing PR distribution by age/staleness bucket
  - Use chart infrastructure from Task 8
  - Embed in PDF
  - Group staleness into buckets (e.g., 0-30 days, 30-60 days, 60-90 days, 90+ days)

  **Must NOT do**:
  - Don't require external services

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8 done, T9, T11)
  - **Blocks**: Nothing
  - **Blocked By**: Task 8

  **Acceptance Criteria**:
  - [ ] Staleness chart PNG embedded in test PDF
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: PDF renders staleness distribution chart
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/chart-stale.pdf
      2. Extract image: pdftoppm /tmp/chart-stale.pdf /tmp/stale-page -png -r 150
      3. Verify: file /tmp/stale-page*.png | grep PNG
    Expected Result: PNG image extracted with chart content
    Evidence: .sisyphus/evidence/complete-plan/task-10-staleness-chart.png
  ```

  **Commit**: YES
  - Message: `feat(report): add staleness distribution chart to PDF`

  ---

- [x] 11. Implement conflict severity pie chart (PNG)

  **What to do**:
  - Consume ConflictPair data: count by Severity (HIGH/MEDIUM/LOW)
  - Render pie or donut chart showing distribution by severity
  - Use chart infrastructure from Task 8
  - Embed in PDF
  - Only show if conflicts exist (graceful handling for empty)

  **Must NOT do**:
  - Don't require external services

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8 done, T9, T10)
  - **Blocks**: Nothing
  - **Blocked By**: Task 8

  **Acceptance Criteria**:
  - [ ] Conflict severity chart embedded in test PDF
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: PDF renders conflict severity chart
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/chart-conflict.pdf
      2. Extract image: pdftoppm /tmp/chart-conflict.pdf /tmp/conf-page -png -r 150
      3. Verify: file /tmp/conf-page*.png | grep PNG
    Expected Result: PNG image extracted with pie chart content
    Evidence: .sisyphus/evidence/complete-plan/task-11-conflict-chart.png
  ```

  **Commit**: YES
  - Message: `feat(report): add conflict severity chart to PDF`

  ---

- [x] 12. Wire comprehensive PDF to CLI report command

  **What to do**:
  - In `internal/app/service.go`, update `Report()` method to call comprehensive PDFComposer with all data
  - Ensure it fetches latest analysis from DB (AnalyzeResult) and passes all fields to PDFComposer
  - For PDF format: write to --output path (or stdout)
  - Verify CLI command uses updated service.Report() method
  - Test: comprehensive PDF generated with all sections and charts

  **Must NOT do**:
  - Don't re-run analyze for report — use cached data
  - Don't change the AnalysisResponse type

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Reason**: Follow existing service method patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 13, 14, 15)
  - **Blocks**: Task 13
  - **Blocked By**: Wave 3 complete

  **References**:
  - `internal/app/service.go` — existing service methods to copy pattern from
  - `internal/report/pdf.go` — NewPDFComposer signature with all parameters
  - `cmd/pratc/report.go` — CLI command that calls service.Report()

  **Acceptance Criteria**:
  - [ ] `pratc report --repo=owner/repo` produces PDF > 100KB (with all sections and charts)
  - [ ] All planned sections present in PDF output
  - [ ] go build → PASS

  **QA Scenarios**:
  ```
  Scenario: CLI report produces comprehensive PDF
    Tool: Bash
    Preconditions: DB warm with opencode-ai/opencode
    Steps:
      1. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/full-report.pdf
      2. stat -c%s /tmp/full-report.pdf
      3. pdftotext /tmp/full-report.pdf - | wc -l
    Expected Result: File size > 100KB, text content shows all sections
    Evidence: .sisyphus/evidence/complete-plan/task-12-full-pdf.txt
  ```

  **Commit**: YES
  - Message: `feat(report): wire service.Report() to comprehensive PDF composer`

  ---

- [x] 13. Verify full pipeline on small repo (opencode-ai/opencode)

  **What to do**:
  - Run full pipeline: sync → analyze → cluster → graph → plan → report
  - Verify each command returns real data
  - Time each operation against SLOs
  - Verify comprehensive PDF generated (>100KB, 8-12 pages, all sections present)
  - Run existing tests to ensure no regressions

  **Must NOT do**:
  - Don't modify code in this task (verification only)

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 14
  - **Blocked By**: Task 12

  **Acceptance Criteria**:
  - [ ] analyze returns valid JSON with counts
  - [ ] cluster returns clusters array
  - [ ] graph returns valid DOT
  - [ ] plan returns ranked selections
  - [ ] report produces comprehensive PDF (>100KB, 8-12 pages)
  - [ ] All tests pass: `go test ./...`

  **QA Scenarios**:
  ```
  Scenario: Full pipeline works end-to-end with comprehensive report
    Tool: Bash
    Steps:
      1. time ./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts'
      2. time ./bin/pratc cluster --repo=opencode-ai/opencode --format=json | jq '.clusters | length'
      3. time ./bin/pratc graph --repo=opencode-ai/opencode --format=dot | head -3
      4. time ./bin/pratc plan --repo=opencode-ai/opencode --target=10 --format=json | jq '.selected | length'
      5. ./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=/tmp/verified-report.pdf
      6. stat -c%s /tmp/verified-report.pdf
    Expected Result: All return real data, PDF > 100KB
    Evidence: .sisyphus/evidence/complete-plan/task-13-verified-pipeline.txt
  ```

  **Commit**: NO

  ---

- [x] 14. Execute cold sync on openclaw/openclaw (5,500+ PRs)

  **What to do**:
  - Run sync on openclaw/openclaw (5,500+ PRs)
  - Monitor progress
  - Expect 15-20 minutes for cold sync
  - Verify job completes with "completed" status
  - Verify ~5,500 PRs stored in SQLite

  **Must NOT do**:
  - Don't interrupt sync
  - Don't skip verification steps

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Task 15
  - **Blocked By**: Task 13

  **Acceptance Criteria**:
  - [ ] Sync completes successfully
  - [ ] Job status is "completed"
  - [ ] ~5,500 PRs in SQLite database

  **QA Scenarios**:
  ```
  Scenario: Production sync completes successfully
    Tool: Bash
    Steps:
      1. export GH_TOKEN=$(gh auth token) && export GITHUB_TOKEN=$GH_TOKEN
      2. time ./bin/pratc sync --repo=openclaw/openclaw
      3. sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"
      4. sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs WHERE repo='openclaw/openclaw' ORDER BY created_at DESC LIMIT 1;"
    Expected Result: ~5500 PRs, status="completed"
    Evidence: .sisyphus/evidence/complete-plan/task-14-openclaw-sync.txt
  ```

  **Commit**: NO

  ---

- [x] 15. Run full analysis pipeline on openclaw with comprehensive report

  **What to do**:
  - Run full pipeline on openclaw/openclaw: analyze → cluster → graph → plan → report
  - Measure against SLOs (analyze ≤300s, cluster ≤180s, graph ≤120s, plan ≤90s)
  - Generate comprehensive report PDF
  - Verify PDF size and content quality
  - Run final test suite to ensure no regressions

  **Must NOT do**:
  - Don't modify code
  - Don't skip SLO verification

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: Final Verification Wave
  - **Blocked By**: Task 14

  **Acceptance Criteria**:
  - [ ] All commands complete within SLOs
  - [ ] Report PDF generated (>100KB, comprehensive)
  - [ ] All tests pass: `make test`

  **QA Scenarios**:
  ```
  Scenario: Full production pipeline meets SLOs
    Tool: Bash
    Steps:
      1. time ./bin/pratc analyze --repo=openclaw/openclaw --format=json > /tmp/openclaw-analyze.json
      2. time ./bin/pratc cluster --repo=openclaw/openclaw --format=json > /tmp/openclaw-cluster.json
      3. time ./bin/pratc graph --repo=openclaw/openclaw --format=dot > /tmp/openclaw-graph.dot
      4. time ./bin/pratc plan --repo=openclaw/openclaw --target=50 --format=json > /tmp/openclaw-plan.json
      5. ./bin/pratc report --repo=openclaw/openclaw --format=pdf --output=openclaw-report.pdf
      6. make test
    Expected Result: All complete within SLOs, tests pass, PDF generated
    Evidence: .sisyphus/evidence/complete-plan/task-15-openclaw-pipeline.txt
  ```

  **Commit**: NO

  ---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

  **VERDICT: APPROVE** - Must Haves [4/4] | Must NOT Haves [3/3 clean] | Tasks [15/15]

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `tsc --noEmit` + linter + `bun test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

  **VERDICT: APPROVE** - Build PASS | Lint PASS | Tests [19/19 pass] | Files CLEAN

- [x] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill if UI)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (features working together, not isolation). Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

  **VERDICT: APPROVE** - Small repo pipeline PASS | Large repo SLOs met | Tests PASS

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

  **VERDICT: APPROVE** - Tasks [15/15 compliant] | Contamination CLEAN | Unaccounted CLEAN

---

## Commit Strategy

- **Wave 1**: `fix(cmd): create and track sync jobs in CLI sync command` — `internal/cmd/root.go`
- **Wave 2**: `feat(report): expand PDF sections with comprehensive data` — `internal/report/pdf.go`
- **Wave 3**: `feat(report): add chart rendering to PDF` — `internal/report/charts.go`
- **Wave 4**: `feat(report): wire comprehensive PDF to CLI and verify pipeline` — `internal/app/service.go`, verification

---

## Success Criteria

### Verification Commands
```bash
# Sync job tracking
./bin/pratc sync --repo=opencode-ai/opencode  # Output includes job_id, status=completed
sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs ORDER BY created_at DESC LIMIT 1;"  # → "completed"

# Small repo pipeline
./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts.total_prs'  # → 42

# Comprehensive report
./bin/pratc report --repo=opencode-ai/opencode --format=pdf --output=report.pdf
stat -c%s report.pdf  # → > 100000
pdftotext report.pdf - | grep "Conflict Analysis"   # → match
pdftotext report.pdf - | grep "Duplicate PRs"       # → match
pdftotext report.pdf - | grep "Staleness Signals"  # → match
pdftotext report.pdf - | grep "Cluster Summary"     # → match

# Production
./bin/pratc sync --repo=openclaw/openclaw
sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"  # → ~5500

# Tests
make test
```

### Final Checklist
- [ ] Sync job tracking fixed (jobs complete properly)
- [ ] opencode-ai pipeline verified (42 PRs, comprehensive report)
- [ ] Report produces comprehensive PDF (>100KB, 8-12 pages, all sections)
- [ ] openclaw production run succeeds (~5500 PRs)
- [ ] All tests pass
- [ ] SLOs met for 5.5k PR scale
- [ ] No scope creep (all Must NOT Haves absent)