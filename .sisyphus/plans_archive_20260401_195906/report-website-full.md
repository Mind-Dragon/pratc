# Report Website Full — Comprehensive PDF Reports

## TL;DR

> **Quick Summary**: Expand the minimal 2-page PDF (2.2 KB) into a full-featured multi-section report with real data, add CLI `report` command with PDF/JSON output, and wire the website to display the report.
>
> **Deliverables**:
> - PDF with all sections populated: Cover, Metrics, Conflicts Detail, Duplicates Detail, Overlaps Detail, Staleness Detail, Per-Cluster Summary, Recommendations
> - CLI `report` command: `pratc report --repo=owner/repo --format=pdf|json --output=<path>`
> - Website: report preview page with Download Report button wired to PDF API
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Wave 1 (data wiring) → Wave 2 (charts) → Wave 3 (CLI) → Wave 4 (website integration) → Final QA

---

## Context

### Original Request
User frustrated that PDF report is minimal (2.2 KB, 2 pages). Website is supposed to have a full report. The report is an important part of the website.

### Validated Assumptions (3 parallel explore agents)

**1. PDF Library**: `github.com/go-pdf/fpdf` v0.9.0
- All section methods are implemented (CoverSection, MetricsSection, PoolCompositionSection, ChartsSection, RecommendationsSection)
- ChartsSection only draws chart placeholders — no real chart rendering

**2. Data Model**: Types exist and Analyze populates them
- `DuplicateGroup`: CanonicalPRNumber, DuplicatePRNums, Similarity, Reason
- `ConflictPair`: SourcePR, TargetPR, ConflictType, FilesTouched, Severity, Reason
- `StalenessReport`: PRNumber, Score, Signals, Reasons, SupersededBy
- All returned in `AnalysisResponse` from `Analyze()`

**3. cmd/generate-report**: Builds successfully
- Uses `internal/report` package (not duplicated code)
- Hardcoded paths: reads `.sisyphus/evidence/task-5-cluster-full.json`, writes `.sisyphus/evidence/pratc-analysis-report.pdf`
- No flags, no CLI wiring — should be replaced by proper `report` CLI command

### Metis Review (Identified Gaps Addressed)
- **Chart rendering strategy**: gofpdf cannot render Vega-Lite directly — need PNG generation approach
- **Target page count**: Defined as 8-12 pages for "comprehensive"
- **CLI vs API parity**: Both should produce identical output
- **Data edge cases**: Empty sections get "No issues detected" message, not skip

---

## Work Objectives

### Core Objective
Produce a comprehensive PDF report (8-12 pages) with all available analysis data, wire it to CLI and web dashboard.

### Concrete Deliverables
- `internal/report/pdf.go`: All 5 PDFComposer sections fully populated with real data
- CLI `report` command: `pratc report --repo=owner/repo --format=pdf|json [--output=<path>]`
- API `GET /api/repos/{owner}/{repo}/report.pdf` returning full PDF
- Web dashboard with report preview metadata (page count, generatedAt, summary stats)
- Vega-Lite chart specs converted to PNG for PDF embedding

### Definition of Done
- `pratc report --repo=owner/repo --format=pdf --output=report.pdf` → exit 0, file > 100 KB, 8-12 pages
- `pdftotext report.pdf - | grep -q "Conflict Analysis"` → true
- `pdftotext report.pdf - | grep -q "Duplicate PRs"` → true
- `pdftotext report.pdf - | grep -q "Staleness Signals"` → true
- `pdftotext report.pdf - | grep -q "Cluster Summary"` → true
- Website "Download Report" button downloads valid PDF

### Must Have
- All AnalysisResponse fields rendered in PDF
- CLI `report` command wired to Cobra
- Chart rendering (bar charts for cluster sizes, staleness distribution)
- Graceful handling of empty sections ("No issues detected")
- API endpoint returns same PDF as CLI

### Must NOT Have
- Interactive charts on website (static preview only)
- Email delivery or scheduling
- Custom report templates
- Multiple output formats beyond PDF + JSON
- HTML export
- Historical/week-over-week comparison

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go tests in `internal/report/`, `cmd/pratc/`)
- **Automated tests**: Tests-after (add tests after each section is implemented)
- **Framework**: Go's built-in `testing` package

### QA Policy
Every task includes agent-executed QA scenarios. Evidence saved to `.sisyphus/evidence/report-website-full/`.

---

## Execution Strategy

```
Wave 1 (Foundation — data wiring, 7 tasks):
├── Task 1: Audit PDFComposer data flow (types → PDF sections)
├── Task 2: Expand PDFComposer with ConflictPair section
├── Task 3: Expand PDFComposer with DuplicateGroup section
├── Task 4: Expand PDFComposer with Overlaps section
├── Task 5: Expand PDFComposer with StalenessReport section
├── Task 6: Expand PDFComposer with per-cluster summary section
└── Task 7: Add "No issues detected" graceful handling

Wave 2 (Chart rendering — PNG generation, 4 tasks):
├── Task 8: Choose chart rendering approach (PNG from Vega-Lite or Go-native)
├── Task 9: Implement chart renderer for cluster bar chart
├── Task 10: Implement chart renderer for staleness distribution
├── Task 11: Implement chart renderer for conflict severity pie chart

Wave 3 (CLI + API wiring — 4 tasks):
├── Task 12: Create `pratc report` CLI command (Cobra)
├── Task 13: Wire report command to PDFComposer with real data
├── Task 14: Wire API endpoint to PDFComposer (handleReportPDF)
├── Task 15: Add JSON output format to report command

Wave 4 (Website integration — 3 tasks):
├── Task 16: Wire SyncStatusPanel Download Report to API
├── Task 17: Add report preview metadata to dashboard (page count, generatedAt)
└── Task 18: Consume Vega-Lite chart specs for web dashboard

Final Wave (2 parallel reviews):
├── F1: Plan compliance audit
└── F2: Code quality review + real QA
```

Critical Path: T1 → T2-T7 (all in Wave 1, can parallelize after T1) → T8 → T9-T11 → T12 → T13-15 → T16-18 → F1-F2
Max Concurrent: 7 (Wave 1), 4 (Wave 2), 4 (Wave 3), 3 (Wave 4)

---

## TODOs

- [x] 1. Audit PDFComposer data flow (types → PDF sections)

  **What to do**:
  - Read `internal/report/pdf.go` fully to trace how `ScalabilityMetrics` flows into each section
  - Read `internal/types/models.go` for `AnalysisResponse`, `ConflictPair`, `DuplicateGroup`, `StalenessReport`, `ClusterResults`
  - Read `internal/app/service.go` Analyze method to see what data is available
  - Document: which section currently receives which data, and which fields from AnalysisResponse are unconsumed
  - Check `cmd/generate-report/main.go` to see how it wires cluster JSON → PDF (this is the working reference)

  **Must NOT do**:
  - Don't modify any code yet — this is pure research

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Straight codebase reading and documentation — no implementation, no complex logic
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-7 once T1 findings are available)
  - **Parallel Group**: Wave 1 (first, unblocks all others)
  - **Blocks**: Tasks 2-7 (they need T1's data flow map)
  - **Blocked By**: None

  **References**:
  - `internal/report/pdf.go` — PDFComposer, section methods, ScalabilityMetrics usage
  - `internal/types/models.go` — AnalysisResponse, ConflictPair, DuplicateGroup, StalenessReport, ClusterResults
  - `internal/app/service.go` — Analyze() returns all reportable data
  - `cmd/generate-report/main.go` — reference implementation wiring cluster JSON → PDF

  **Acceptance Criteria**:
  - [ ] Written audit doc (can be in code comments or a separate doc): "Data flow: AnalysisResponse → PDF section"
  - [ ] List of which AnalysisResponse fields are currently consumed by PDF
  - [ ] List of which AnalysisResponse fields are NOT yet consumed

  **Commit**: NO

---

- [x] 2. Expand PDFComposer — ConflictPair section

  **What to do**:
  - In `internal/report/pdf.go`, add a `ConflictSection` type that renders ConflictPair data
  - Add `Render(*fpdf.Fpdf)` method to ConflictSection, following the same pattern as existing sections
  - Layout: section title "Conflict Analysis", then for each ConflictPair: PR numbers, conflict type, severity (color-coded if possible), files touched, brief reason
  - If `Conflicts` slice is empty: render "No conflicts detected." (graceful handling)
  - Update `NewPDFComposer` signature to accept `Conflicts []ConflictPair` — add to ScalabilityMetrics or as separate param
  - Update `cmd/generate-report/main.go` to pass Conflicts data to composer

  **Must NOT do**:
  - Don't add chart rendering here — that is Wave 2
  - Don't add page breaks mid-section without checking total page count

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Familiar Go PDF rendering pattern — follow existing section patterns
  > **Skills**: None required (straightforward Go following existing patterns)

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 3, 4, 5, 6, 7 after T1 completes)
  - **Parallel Group**: Wave 1
  - **Blocks**: Nothing (independent section)
  - **Blocked By**: T1

  **References**:
  - `internal/report/pdf.go` — existing section implementations to copy pattern from (MetricsSection, CoverSection)
  - `internal/types/models.go` — ConflictPair struct fields: SourcePR, TargetPR, ConflictType, FilesTouched, Severity, Reason
  - `cmd/generate-report/main.go` — how to wire new data through composer.AddSection()

  **Acceptance Criteria**:
  - [ ] `ConflictSection` type defined with `Render(*fpdf.Fpdf)` method
  - [ ] `NewPDFComposer` accepts Conflicts data
  - [ ] Empty conflicts renders "No conflicts detected."
  - [ ] `go build ./...` → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF with conflicts renders conflict section
    Tool: Bash
    Preconditions: DB has test repo with known conflicts
    Steps:
      1. cd /home/agent/pratc && ./bin/pratc report --repo=test/test --format=pdf --output=/tmp/conflict-test.pdf
      2. pdftotext /tmp/conflict-test.pdf - | grep -i "conflict"
    Expected Result: "Conflict Analysis" section present in PDF
    Evidence: .sisyphus/evidence/report-website-full/task-2-conflict-section.txt

  Scenario: PDF with no conflicts renders graceful message
    Tool: Bash
    Preconditions: Fresh test repo with no conflicts
    Steps:
      1. ./bin/pratc report --repo=test/empty --format=pdf --output=/tmp/no-conflict-test.pdf
      2. pdftotext /tmp/no-conflict-test.pdf - | grep -i "no conflict"
    Expected Result: "No conflicts detected." in PDF
    Evidence: .sisyphus/evidence/report-website-full/task-2-no-conflict-graceful.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add ConflictPair section to PDF composer`
  - Files: `internal/report/pdf.go`

---

- [x] 3. Expand PDFComposer — DuplicateGroup section

  **What to do**:
  - In `internal/report/pdf.go`, add a `DuplicatesSection` type
  - Render each DuplicateGroup: canonical PR number, list of duplicate PR numbers, similarity score, reason
  - If `Duplicates` is empty: render "No duplicate PRs detected."
  - Add to NewPDFComposer params

  **Must NOT do**:
  - Don't add chart rendering — Wave 2 only

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 4, 5, 6, 7 after T1)
  - **Blocks**: Nothing
  - **Blocked By**: T1

  **References**:
  - `internal/types/models.go` — DuplicateGroup: CanonicalPRNumber, DuplicatePRNums, Similarity, Reason

  **Acceptance Criteria**:
  - [ ] DuplicatesSection defined + Render implemented
  - [ ] NewPDFComposer accepts Duplicates data
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF renders duplicate PR section with groups
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/dup-test.pdf
      2. pdftotext /tmp/dup-test.pdf - | grep -i "duplicate"
    Expected Result: "Duplicate PRs" section present
    Evidence: .sisyphus/evidence/report-website-full/task-3-duplicate-section.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add DuplicateGroup section to PDF composer`

---

- [x] 4. Expand PDFComposer — Overlaps section

  **What to do**:
  - In `internal/report/pdf.go`, add an `OverlapsSection` type
  - Render each Overlap entry (similar structure to DuplicatesSection)
  - If empty: "No overlapping PRs detected."
  - Add to NewPDFComposer params

  **Must NOT do**:
  - Don't add chart rendering — Wave 2 only

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 3, 5, 6, 7)
  - **Blocks**: Nothing
  - **Blocked By**: T1

  **References**:
  - `internal/types/models.go` — Overlaps []DuplicateGroup (same shape as Duplicates)

  **Acceptance Criteria**:
  - [ ] OverlapsSection defined + Render implemented
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF renders overlaps section
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/overlap-test.pdf
      2. pdftotext /tmp/overlap-test.pdf - | grep -i "overlap"
    Expected Result: "Overlap Analysis" or "Overlapping PRs" section present
    Evidence: .sisyphus/evidence/report-website-full/task-4-overlap-section.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add Overlaps section to PDF composer`

---

- [x] 5. Expand PDFComposer — StalenessReport section

  **What to do**:
  - In `internal/report/pdf.go`, add a `StalenessSection` type
  - Render top N (e.g., top 20) stale PRs: PR number, staleness score, signals (e.g., "no activity 30+ days"), reasons
  - If empty: "No stale PRs detected."
  - Add to NewPDFComposer params

  **Must NOT do**:
  - Don't add chart rendering — Wave 2 only
  - Don't render all stale PRs if there are hundreds — cap at 20

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 3, 4, 6, 7)
  - **Blocks**: Nothing
  - **Blocked By**: T1

  **References**:
  - `internal/types/models.go` — StalenessReport: PRNumber, Score, Signals, Reasons, SupersededBy

  **Acceptance Criteria**:
  - [ ] StalenessSection defined + Render implemented
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF renders staleness section
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/stale-test.pdf
      2. pdftotext /tmp/stale-test.pdf - | grep -i "stale"
    Expected Result: "Staleness Signals" section present
    Evidence: .sisyphus/evidence/report-website-full/task-5-staleness-section.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add StalenessReport section to PDF composer`

---

- [x] 6. Expand PDFComposer — Per-cluster summary section

  **What to do**:
  - In `internal/report/pdf.go`, add a `ClusterSummarySection` type
  - Render: total cluster count, top 10 clusters by PR count (cluster label, PR count, brief description)
  - This consumes ClusterResults from Analyze
  - If no clusters: "No clusters identified."
  - Add to NewPDFComposer params

  **Must NOT do**:
  - Don't render all clusters — cap at top 10 by PR count
  - Don't add per-cluster charts here — Wave 2

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-5, 7)
  - **Blocks**: Nothing
  - **Blocked By**: T1

  **References**:
  - `internal/types/models.go` — ClusterResults struct (check exact name from T1 audit)

  **Acceptance Criteria**:
  - [ ] ClusterSummarySection defined + Render implemented
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF renders cluster summary
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/cluster-test.pdf
      2. pdftotext /tmp/cluster-test.pdf - | grep -i "cluster"
    Expected Result: "Cluster Summary" section present
    Evidence: .sisyphus/evidence/report-website-full/task-6-cluster-summary.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add cluster summary section to PDF composer`

---

- [x] 7. Add graceful "no issues" handling across all sections

  **What to do**:
  - Review all section Render methods (Cover, Metrics, Conflict, Duplicates, Overlaps, Staleness, Cluster, Recommendations)
  - Ensure every section that receives empty/nil data renders a graceful message instead of blank space
  - Check gofpdf page layout: ensure sections don't produce blank pages when data is empty
  - Verify PDF still opens and is valid even when all sections are empty

  **Must NOT do**:
  - Don't change section content — only add null/empty guards

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2-6)
  - **Blocks**: Nothing
  - **Blocked By**: T1

  **References**:
  - `internal/report/pdf.go` — all section Render methods

  **Acceptance Criteria**:
  - [ ] Every section has null/empty guard
  - [ ] PDF with empty data is valid (opens, has content, no crashes)
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF with completely empty data is valid
    Tool: Bash
    Preconditions: Use mock data with all empty slices
    Steps:
      1. Create a test invoking PDFComposer with all nil/empty data
      2. Verify PDF composes without panic
      3. Verify PDF is valid (file command returns "PDF")
    Expected Result: Valid PDF with graceful "no issues" messages
    Evidence: .sisyphus/evidence/report-website-full/task-7-empty-graceful.txt
  \`\`\`

  **Commit**: YES
  - Message: `fix(report): add graceful empty-state handling to all sections`

- [x] 8. Choose chart rendering approach for PDF

  **What to do**:
  - Research: gofpdf cannot natively render Vega-Lite or complex charts
  - Options to evaluate: (A) Go-native charting lib (go-chart, plotlib), (B) Generate PNG from Vega-Lite spec via external tool, (C) Static chart images from pre-defined templates
  - Decision: Pick the simplest approach that works with gofpdf's image embedding
  - Implement chartgo renderer's interface/struct that each chart task will use

  **Must NOT do**:
  - Don't pick a solution requiring external services ( wkhtmltopdf daemon, etc.)
  - Don't pick a solution requiring a browser runtime

  **Recommended Agent Profile**:
  > **Category**: `deep`
  > **Reason**: Needs genuine evaluation of trade-offs between approaches
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9, 10, 11 after this decision is made)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 9, 10, 11
  - **Blocked By**: Wave 1 complete

  **References**:
  - `go.mod` — existing dependencies (don't add incompatible ones)
  - `web/src/charts/*.json` — Vega-Lite specs (reference for what data is available)

  **Acceptance Criteria**:
  - [ ] Chart renderer interface defined
  - [ ] At least one working chart type (bar chart) renders into PNG
  - [ ] PNG is embeddable in gofpdf

  **Commit**: YES
  - Message: `feat(report): add chart rendering infrastructure`
  - Files: `internal/report/charts.go` (new file)

---

- [x] 9. Implement cluster size bar chart (PNG)

  **What to do**:
  - Consume ClusterResults: cluster label + PR count
  - Render bar chart: X-axis = cluster labels (truncated), Y-axis = PR count
  - Use chosen chart approach from T8
  - Embed in PDF via gofpdf's image embedding
  - Add ChartsSection to PDFComposer (or extend existing ChartsSection)

  **Must NOT do**:
  - Don't render all clusters if >20 — show top 20 by PR count
  - Don't use external services

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Follow chart renderer interface from T8, straightforward bar chart
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 8 done, T10, T11 can proceed)
  - **Blocks**: Nothing
  - **Blocked By**: T8

  **References**:
  - `internal/types/models.go` — ClusterResults for cluster labels and PR counts
  - `internal/report/pdf.go` — how sections embed images (existing pattern)

  **Acceptance Criteria**:
  - [ ] Bar chart PNG generated and embedded in test PDF
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF with clusters renders bar chart
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/chart-cluster.pdf
      2. Identify embedded image in PDF: pdftoppm /tmp/chart-cluster.pdf /tmp/chart-page -png -f 3 -l 3
      3. Check if image file exists: ls -la /tmp/chart-page*.png
    Expected Result: PNG file generated from PDF page 3
    Evidence: .sisyphus/evidence/report-website-full/task-9-cluster-chart.png
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add cluster size bar chart to PDF`

---

- [x] 10. Implement staleness distribution chart (PNG)

  **What to do**:
  - Consume StalenessReport data: distribution of staleness scores
  - Render: histogram or pie chart showing PR distribution by age/staleness bucket
  - Embed in PDF

  **Must NOT do**:
  - Don't require external services

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8 done, T9, T11)
  - **Blocks**: Nothing
  - **Blocked By**: T8

  **Acceptance Criteria**:
  - [ ] Staleness chart PNG embedded in test PDF
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF renders staleness distribution chart
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/chart-stale.pdf
      2. pdftoppm /tmp/chart-stale.pdf /tmp/stale-page -png -r 150
      3. Identify pages with images: file /tmp/stale-page*.png
    Expected Result: At least one PNG extracted from PDF with chart content
    Evidence: .sisyphus/evidence/report-website-full/task-10-staleness-chart.png
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add staleness distribution chart to PDF`

---

- [x] 11. Implement conflict severity pie chart (PNG)

  **What to do**:
  - Consume ConflictPair data: count by Severity (HIGH/MEDIUM/LOW)
  - Render pie or donut chart
  - Embed in PDF

  **Must NOT do**:
  - Don't require external services

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8 done, T9, T10)
  - **Blocks**: Nothing
  - **Blocked By**: T8

  **Acceptance Criteria**:
  - [ ] Conflict severity chart embedded in test PDF
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: PDF renders conflict severity chart
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/chart-conflict.pdf
      2. Extract any image: pdftoppm /tmp/chart-conflict.pdf /tmp/conf-page -png -r 150
      3. Verify: file /tmp/conf-page*.png | grep PNG
    Expected Result: PNG image extracted
    Evidence: .sisyphus/evidence/report-website-full/task-11-conflict-chart.png
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add conflict severity chart to PDF`

- [x] 12. Create `pratc report` CLI command (Cobra)

  **What to do**:
  - In `cmd/pratc/`, create `report.go` following the same pattern as `analyze.go`, `cluster.go`, etc.
  - Register with `init()` → `RegisterReportCommand(rootCmd)`
  - Flags: `--repo` (required), `--format` (pdf|json, default pdf), `--output` (default stdout for pdf, file for json)
  - Wire to service.Report() method (may need to create this)

  **Must NOT do**:
  - Don't create new packages — follow existing cmd/pratc/ structure exactly
  - Don't add flags beyond repo, format, output

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Follow existing Cobra command patterns in cmd/pratc/
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 13, 14, 15)
  - **Blocks**: Nothing
  - **Blocked By**: Wave 1+2 complete

  **References**:
  - `cmd/pratc/analyze.go` — reference for command structure, flags, service call
  - `cmd/pratc/cluster.go` — another reference
  - `cmd/pratc/root.go` — how commands are registered

  **Acceptance Criteria**:
  - [ ] `pratc report --help` shows usage
  - [ ] `pratc report --repo=owner/repo --format=pdf --output=report.pdf` runs without error
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: CLI report command exists and shows help
    Tool: Bash
    Steps:
      1. cd /home/agent/pratc && ./bin/pratc report --help
    Expected Result: Help text with --repo, --format, --output flags shown
    Evidence: .sisyphus/evidence/report-website-full/task-12-cli-help.txt

  Scenario: CLI report command produces PDF
    Tool: Bash
    Preconditions: DB has test repo data
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/cli-report.pdf
      2. file /tmp/cli-report.pdf
    Expected Result: "PDF document" in file output
    Evidence: .sisyphus/evidence/report-website-full/task-12-cli-pdf-output.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(cli): add report command with pdf/json output`

---

- [x] 13. Wire report CLI to PDFComposer with real data

  **What to do**:
  - In `internal/app/service.go`, add `Report(repo, format, output)` method
  - Method fetches latest analysis from DB (AnalyzeResult), then calls PDFComposer with all data
  - For PDF format: write to --output path (or stdout)
  - For JSON format: return same data as JSON (same fields as AnalysisResponse)
  - Read from warm cache or DB — do NOT re-run analyze

  **Must NOT do**:
  - Don't re-run analyze for report — use cached data
  - Don't change the AnalysisResponse type

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Follow existing service method patterns (Analyze, Cluster, Graph)
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 12, 14, 15)
  - **Blocks**: Nothing
  - **Blocked By**: Wave 1+2 complete

  **References**:
  - `internal/app/service.go` — existing service methods to copy pattern from
  - `internal/report/pdf.go` — NewPDFComposer signature
  - `internal/cache/` — how to read cached analysis results

  **Acceptance Criteria**:
  - [ ] `pratc report --repo=owner/repo` produces PDF > 100KB (with all sections)
  - [ ] `pratc report --repo=owner/repo --format=json` outputs valid JSON matching AnalysisResponse shape
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: CLI report produces comprehensive PDF
    Tool: Bash
    Preconditions: DB warm with facebook/react
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/full-report.pdf
      2. stat -c%s /tmp/full-report.pdf
      3. pdftotext /tmp/full-report.pdf - | wc -l
    Expected Result: File size > 100KB, text content > 500 lines
    Evidence: .sisyphus/evidence/report-website-full/task-13-full-pdf.txt

  Scenario: JSON output is valid
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=json | jq '. | keys'
    Expected Result: Keys include repo, generatedAt, conflicts, duplicates, overlaps, stalenessSignals, clusters
    Evidence: .sisyphus/evidence/report-website-full/task-13-json-output.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): wire service.Report() to PDF composer with full data`

---

- [x] 14. Wire API endpoint handleReportPDF to PDFComposer

  **What to do**:
  - In `internal/cmd/root.go`, update `handleReportPDF()` to call the same PDFComposer with full data
  - Ensure it uses the same code path as CLI report (share service.Report())
  - Test: `curl http://localhost:8080/api/repos/facebook/react/report.pdf -o /tmp/api-report.pdf`

  **Must NOT do**:
  - Don't duplicate the PDF composition logic — reuse service.Report()
  - Don't break existing API contracts

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Simple HTTP handler following existing patterns
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 12, 13, 15)
  - **Blocks**: Nothing
  - **Blocked By**: T13 (service method needs to exist first)

  **References**:
  - `internal/cmd/root.go` — existing handleReportPDF implementation
  - `internal/app/service.go` — Report() method to call

  **Acceptance Criteria**:
  - [ ] `curl http://localhost:8080/api/repos/facebook/react/report.pdf` returns valid PDF
  - [ ] API PDF matches CLI PDF byte-for-byte (same data)
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: API endpoint returns valid PDF
    Tool: Bash
    Preconditions: pratc serve running
    Steps:
      1. curl -sS http://localhost:8080/api/repos/facebook/react/report.pdf -o /tmp/api-report.pdf
      2. file /tmp/api-report.pdf
    Expected Result: "PDF document" output
    Evidence: .sisyphus/evidence/report-website-full/task-14-api-pdf.txt

  Scenario: API PDF and CLI PDF are identical
    Tool: Bash
    Preconditions: Both CLI and API produce PDF for same repo
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=pdf --output=/tmp/cli-report.pdf
      2. curl -sS http://localhost:8080/api/repos/facebook/react/report.pdf -o /tmp/api-report.pdf
      3. diff /tmp/cli-report.pdf /tmp/api-report.pdf
    Expected Result: No output (files identical)
    Evidence: .sisyphus/evidence/report-website-full/task-14-pdf-identical.txt
  \`\`\`

  **Commit**: YES
  - Message: `fix(api): wire handleReportPDF to full PDF composer`

---

- [x] 15. Add JSON output format to report command

  **What to do**:
  - `--format=json` already stubbed in T12
  - Ensure JSON output matches AnalysisResponse schema exactly
  - Pretty-print JSON for readability
  - Include: repo, generatedAt, counts, clusters, duplicates, overlaps, conflicts, stalenessSignals

  **Must NOT do**:
  - Don't change AnalysisResponse type
  - Don't add custom fields

  **Recommended Agent Profile**:
  > **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 12, 13, 14)
  - **Blocks**: Nothing
  - **Blocked By**: T12 (CLI command must exist)

  **References**:
  - `internal/types/models.go` — AnalysisResponse schema
  - `cmd/pratc/report.go` — T12's output formatting

  **Acceptance Criteria**:
  - [ ] `pratc report --repo=owner/repo --format=json` outputs valid JSON
  - [ ] All AnalysisResponse fields present
  - [ ] go build → PASS

  **QA Scenarios**:
  \`\`\`
  Scenario: JSON output complete and valid
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=facebook/react --format=json | jq 'keys'
    Expected Result: All keys: repo, generatedAt, counts, clusters, duplicates, overlaps, conflicts, stalenessSignals
    Evidence: .sisyphus/evidence/report-website-full/task-15-json-complete.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(report): add comprehensive JSON output format`

- [x] 16. Wire SyncStatusPanel Download Report button to API

  **What to do**:
  - Read `web/src/components/SyncStatusPanel.tsx` — current Download Report button implementation
  - Current: `window.location.href = /api/repos/{owner}/{repo}/report.pdf`
  - Verify it works: click Download Report → PDF downloads
  - If broken, fix the URL construction or add any missing props
  - Add loading state ("Generating report...") during download

  **Must NOT do**:
  - Don't change the API endpoint URL
  - Don't add new pages — just fix existing button wiring

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Simple fix to existing button, follows existing patterns
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 17, 18)
  - **Blocks**: Nothing
  - **Blocked By**: T14 (API must work first)

  **References**:
  - `web/src/components/SyncStatusPanel.tsx` — Download Report button
  - `web/src/lib/api.ts` — API client functions

  **Acceptance Criteria**:
  - [ ] Download Report button triggers PDF download
  - [ ] Browser receives valid PDF file
  - [ ] No console errors

  **QA Scenarios**:
  \`\`\`
  Scenario: Download Report button downloads PDF via Playwright
    Tool: Playwright
    Preconditions: web dev server running, pratc serve running
    Steps:
      1. Open http://localhost:3000
      2. Wait for SyncStatusPanel to load
      3. Click "Download Report" button
      4. Wait for download to complete
      5. Verify downloaded file is PDF
    Expected Result: PDF file saved to downloads folder
    Evidence: .sisyphus/evidence/report-website-full/task-16-download-btn.png
  \`\`\`

  **Commit**: YES
  - Message: `fix(web): wire Download Report button to API endpoint`

---

- [x] 17. Add report preview metadata to dashboard

  **What to do**:
  - Add to `web/src/pages/index.tsx`: fetch report metadata (page count, generatedAt, summary stats) alongside analysis data
  - Display: "Report generated at {timestamp}" and brief summary (total PRs, conflict count, duplicate count, cluster count)
  - This is metadata only — no actual PDF rendering in browser
  - Fetch via: GET /api/repos/{owner}/{repo}/report (new JSON endpoint returning report metadata)

  **Must NOT do**:
  - Don't render full PDF in browser
  - Don't add new pages — just extend existing index.tsx

  **Recommended Agent Profile**:
  > **Category**: `quick`
  > **Reason**: Small addition to existing dashboard page
  > **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 16, 18)
  - **Blocks**: Nothing
  - **Blocked By**: T15 (JSON output must exist first)

  **References**:
  - `web/src/pages/index.tsx` — existing dashboard structure
  - `web/src/components/SyncStatusPanel.tsx` — where to add metadata display

  **Acceptance Criteria**:
  - [ ] Dashboard shows report generation timestamp
  - [ ] Dashboard shows summary stats from latest analysis
  - [ ] No console errors on page load

  **QA Scenarios**:
  \`\`\`
  Scenario: Dashboard shows report metadata
    Tool: Playwright
    Preconditions: web dev server running, pratc serve running
    Steps:
      1. Open http://localhost:3000
      2. Wait for page to load
      3. Verify "Report generated" timestamp visible
      4. Verify PR count, cluster count visible
    Expected Result: Metadata visible in UI
    Evidence: .sisyphus/evidence/report-website-full/task-17-metadata-visible.png
  \`\`\`

  **Commit**: YES
  - Message: `feat(web): add report metadata to dashboard`

---

- [x] 18. Consume Vega-Lite chart specs for web dashboard

  **What to do**:
  - Read `web/src/charts/*.json` — all Vega-Lite chart specs
  - Find where these could be rendered: likely `web/src/pages/index.tsx` or a new `web/src/pages/report.tsx`
  - Use `vega-lite` npm package (or similar) to render charts server-side or static generation
  - Decision: Render as static images (not interactive) since this is a preview
  - Consume the same data the PDF charts use (ClusterResults, StalenessReport, ConflictPairs)

  **Must NOT do**:
  - Don't build interactive charts — static image preview only
  - Don't redesign chart specs — use existing ones from web/src/charts/

  **Recommended Agent Profile**:
  > **Category**: `visual-engineering`
  > **Reason**: Chart rendering in Next.js with Vega-Lite specs — requires frontend skills
  > **Skills**: [`vega-lite` or similar — find existing usage in web/]
  > **Skills Evaluated but Omitted**:
  > - `playwright`: not needed for static chart rendering

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 16, 17)
  - **Blocks**: Nothing
  - **Blocked By**: T15 (JSON output must exist first)

  **References**:
  - `web/src/charts/*.json` — Vega-Lite specs (cluster bar chart, staleness histogram, conflict severity)
  - `web/src/pages/index.tsx` — where to integrate charts
  - `web/package.json` — existing dependencies (check for vega/vega-lite)

  **Acceptance Criteria**:
  - [ ] Dashboard renders at least one chart from the Vega-Lite specs
  - [ ] Charts display real data from latest analysis
  - [ ] No runtime errors in browser console

  **QA Scenarios**:
  \`\`\`
  Scenario: Dashboard renders cluster chart from Vega-Lite spec
    Tool: Playwright
    Preconditions: web dev server running, chart data available
    Steps:
      1. Open http://localhost:3000
      2. Wait for charts to render (timeout: 10s)
      3. Take screenshot of chart area
    Expected Result: Screenshot shows bar chart with data
    Evidence: .sisyphus/evidence/report-website-full/task-18-chart-screenshot.png
  \`\`\`

  **Commit**: YES
  - Message: `feat(web): render Vega-Lite charts on dashboard`

---

- [ ] F1. **Plan Compliance Audit** — `oracle`

  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist.

  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

---

- [ ] F2. **Code Quality Review** — `unspecified-high`

  Run `go build ./...` + `go vet ./...` + `bun test` (web). Review changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.

  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

---

- [ ] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill if UI)

  Execute EVERY QA scenario from EVERY task. Test cross-task integration (CLI PDF = API PDF = Website download). Test edge cases: empty data, special characters in repo name.

  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

---

- [ ] F4. **Scope Fidelity Check** — `deep`

  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance.

  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **Wave 1** (Tasks 1-7): `feat(report): expand PDF sections with full data` — internal/report/pdf.go, internal/types/models.go (if types added)
- **Wave 2** (Tasks 8-11): `feat(report): add chart rendering to PDF` — internal/report/charts.go (new)
- **Wave 3** (Tasks 12-15): `feat(cli): add report command + API wiring` — cmd/pratc/report.go, internal/cmd/root.go, internal/app/service.go
- **Wave 4** (Tasks 16-18): `feat(web): integrate report in dashboard` — web/src/

---

## Success Criteria

### Verification Commands
```bash
# CLI report produces comprehensive PDF
./bin/pratc report --repo=facebook/react --format=pdf --output=report.pdf
stat -c%s report.pdf          # Expected: > 100KB
pdftotext report.pdf - | grep "Conflict Analysis"   # Expected: match
pdftotext report.pdf - | grep "Duplicate PRs"       # Expected: match
pdftotext report.pdf - | grep "Staleness Signals"  # Expected: match
pdftotext report.pdf - | grep "Cluster Summary"     # Expected: match

# JSON output complete
./bin/pratc report --repo=facebook/react --format=json | jq 'keys'
# Expected: repo, generatedAt, counts, clusters, duplicates, overlaps, conflicts, stalenessSignals

# API endpoint returns same PDF
curl -sS http://localhost:8080/api/repos/facebook/react/report.pdf -o api-report.pdf
diff report.pdf api-report.pdf   # Expected: no output (identical)

# Website download works
# Playwright: click Download Report → PDF saved

# Build passes
go build ./... && go vet ./...
```

### Final Checklist
- [ ] All Must Have present (all 5 PDF sections populated, CLI command, API endpoint, web wiring)
- [ ] All Must NOT Have absent (no interactive web charts, no scheduling, no email)
- [ ] All tests pass (`make test-go` + `make test-web`)
- [ ] Evidence files exist for all QA scenarios
- [ ] Plan archived at `.sisyphus/plans-archived-YYYYMMDD/`
