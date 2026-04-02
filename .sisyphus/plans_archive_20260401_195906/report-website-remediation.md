# Report Website Full — Corrected Remediation Plan

> **CORRECTION NOTICE**: Initial audit was WRONG. Chart renderers, Download Report button, Vega-Lite charts, and ChartPanel ALL EXIST. The ONLY verified gap is the missing evidence directory.

## TL;DR

> **Quick Summary**: The report-website-full implementation is COMPLETE. Only issue: evidence directory doesn't exist. Need to create evidence by running verification tests.
> 
> **Deliverables**:
> - Create `.sisyphus/evidence/report-website-full/` directory
> - Run verification tests and save evidence
> - Verify all components work together
> 
> **Estimated Effort**: Low
> **Parallel Execution**: YES — 2 waves
> **Critical Path**: Create evidence directory → Run all QA scenarios → Final audit

---

## Context

### Original Audit Was WRONG

My initial audit claimed chart renderers were missing. This was INCORRECT.

**What ACTUALLY exists:**

| Component | Status | File |
|-----------|--------|------|
| ChartRenderer interface | ✅ EXISTS | `internal/report/chart.go:19-25` |
| BarChartRenderer | ✅ EXISTS | `internal/report/chart.go:28-151` |
| ClusterDistributionChart() | ✅ EXISTS | `internal/report/chart.go:191-232` |
| StalenessDistributionChart() | ✅ EXISTS | `internal/report/chart.go:235-267` |
| ConflictSeverityChart() | ✅ EXISTS | `internal/report/chart.go:274-310` |
| ChartsSection in PDF | ✅ EXISTS | `internal/report/pdf.go:685-746` |
| Download Report button | ✅ EXISTS | `web/src/components/SyncStatusPanel.tsx:293-306` |
| Download loading state | ✅ EXISTS | `"Generating report…"` |
| Vega-Lite chart specs | ✅ EXISTS | `web/src/charts/*.json` (5 files) |
| ChartPanel component | ✅ EXISTS | `web/src/components/ChartPanel.tsx` |
| Charts API endpoint | ✅ EXISTS | `internal/cmd/root.go:920-921` |
| handleChartStalenessPNG | ✅ EXISTS | `internal/cmd/root.go:980` |

**What ACTUALLY is missing:**

| Component | Status | Impact |
|-----------|--------|--------|
| Evidence directory | ❌ MISSING | Cannot verify plan compliance |
| Evidence files | ❌ MISSING | No proof of QA execution |

### This Plan's Scope
- **INCLUDE**: Create evidence directory, run verification, document findings
- **EXCLUDE**: Implementation work (everything is already built)

---

## Work Objectives

### Core Objective
Run verification tests and create evidence files to prove the implementation works.

### Concrete Deliverables
- `.sisyphus/evidence/report-website-full/` directory created
- Evidence files for all verification scenarios
- Final audit showing implementation is complete

### Definition of Done
- Evidence directory exists with 10+ files
- All verification tests pass (or failures documented)
- Final audit shows 100% compliance

### Must Have
- Evidence directory created
- Verification tests executed
- Results documented

### Must NOT Have
- New implementation (everything is done)
- Skipped verification steps

---

## Execution Strategy

```
Wave 1 (Setup + Quick Verification — 4 tasks):
├── Task 1: Create evidence directory
├── Task 2: Run Go tests for report package
├── Task 3: Verify CLI report command works
└── Task 4: Verify build passes

Wave 2 (Integration Verification — 3 tasks):
├── Task 5: Verify PDF contains charts (pdftoppm)
├── Task 6: Verify web components exist and are wired
└── Task 7: Create final evidence summary

Final Wave (1 review):
└── F1: Final compliance audit
```

Critical Path: T1 → T2-T4 → T5-T7 → F1

---

## TODOs

- [ ] 1. Create evidence directory

  **What to do**:
  - Create `.sisyphus/evidence/report-website-full/` directory
  - This is the ONLY missing piece identified

  **Must NOT do**:
  - Don't implement anything new

  **Acceptance Criteria**:
  - [ ] Directory exists: `ls -la .sisyphus/evidence/report-website-full/`

  **QA Scenarios**:
  ```
  Scenario: Evidence directory created
    Tool: Bash
    Steps:
      1. mkdir -p .sisyphus/evidence/report-website-full/
      2. ls -la .sisyphus/evidence/
    Expected Result: Directory exists
    Evidence: .sisyphus/evidence/report-website-full/task-1-dir-created.txt
  ```

  **Commit**: NO

  ---

- [ ] 2. Run Go tests for report package

  **What to do**:
  - Run `go test -v ./internal/report/...`
  - Save test output to evidence directory
  - Verify all tests pass

  **Must NOT do**:
  - Don't modify tests (they should pass)

  **Acceptance Criteria**:
  - [ ] All tests pass
  - [ ] Evidence file created with test output

  **QA Scenarios**:
  ```
  Scenario: Report package tests pass
    Tool: Bash
    Steps:
      1. go test -v ./internal/report/... 2>&1 | tee .sisyphus/evidence/report-website-full/task-2-go-tests.txt
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/report-website-full/task-2-go-tests.txt
  ```

  **Commit**: NO

  ---

- [ ] 3. Verify CLI report command works

  **What to do**:
  - Run `./bin/pratc report --help`
  - Save output to evidence directory
  - Verify command exists and shows correct flags

  **Acceptance Criteria**:
  - [ ] Command exists
  - [ ] Shows --repo, --format, --output flags
  - [ ] Evidence file created

  **QA Scenarios**:
  ```
  Scenario: CLI report command works
    Tool: Bash
    Steps:
      1. ./bin/pratc report --help 2>&1 | tee .sisyphus/evidence/report-website-full/task-3-cli-help.txt
    Expected Result: Help text shows all required flags
    Evidence: .sisyphus/evidence/report-website-full/task-3-cli-help.txt
  ```

  **Commit**: NO

  ---

- [ ] 4. Verify build passes

  **What to do**:
  - Run `go build ./...`
  - Run `go vet ./...`
  - Save output to evidence directory

  **Acceptance Criteria**:
  - [ ] Build passes
  - [ ] Vet passes
  - [ ] Evidence file created

  **QA Scenarios**:
  ```
  Scenario: Build and vet pass
    Tool: Bash
    Steps:
      1. go build ./... 2>&1 | tee .sisyphus/evidence/report-website-full/task-4-build.txt
      2. go vet ./... 2>&1 | tee -a .sisyphus/evidence/report-website-full/task-4-build.txt
    Expected Result: No errors
    Evidence: .sisyphus/evidence/report-website-full/task-4-build.txt
  ```

  **Commit**: NO

  ---

- [ ] 5. Verify PDF contains charts

  **What to do**:
  - Generate a test PDF (using fixture data or mock)
  - Use pdftoppm to extract pages
  - Verify PNG images exist in PDF
  - Save evidence

  **Acceptance Criteria**:
  - [ ] PDF can be generated
  - [ ] PDF contains embedded images
  - [ ] Evidence file created

  **QA Scenarios**:
  ```
  Scenario: PDF contains embedded chart images
    Tool: Bash
    Steps:
      1. Check if pdftoppm is available: which pdftoppm
      2. If available, extract images from any test PDF
      3. Verify PNG files are created
    Expected Result: Chart images extracted from PDF
    Evidence: .sisyphus/evidence/report-website-full/task-5-pdf-charts.txt
  ```

  **Commit**: NO

  ---

- [ ] 6. Verify web components exist and are wired

  **What to do**:
  - Verify SyncStatusPanel has Download Report button
  - Verify ChartPanel component exists
  - Verify index.tsx uses ChartPanel with chartSrc
  - Run web tests if available
  - Save evidence

  **Acceptance Criteria**:
  - [ ] Download Report button code verified
  - [ ] ChartPanel verified in index.tsx
  - [ ] Evidence file created

  **QA Scenarios**:
  ```
  Scenario: Web components verified
    Tool: Bash + Grep
    Steps:
      1. grep -n "Download Report" web/src/components/SyncStatusPanel.tsx | tee .sisyphus/evidence/report-website-full/task-6-web-components.txt
      2. grep -n "ChartPanel" web/src/pages/index.tsx | tee -a .sisyphus/evidence/report-website-full/task-6-web-components.txt
      3. ls web/src/charts/*.json | wc -l | tee -a .sisyphus/evidence/report-website-full/task-6-web-components.txt
    Expected Result: All components exist
    Evidence: .sisyphus/evidence/report-website-full/task-6-web-components.txt
  ```

  **Commit**: NO

  ---

- [ ] 7. Create final evidence summary

  **What to do**:
  - List all evidence files created
  - Count them and verify >= 6 files
  - Create summary markdown file
  - Mark all implementation items as complete in original plan

  **Acceptance Criteria**:
  - [ ] Summary file created
  - [ ] 6+ evidence files exist
  - [ ] Original plan checkboxes verified

  **QA Scenarios**:
  ```
  Scenario: Evidence summary created
    Tool: Bash
    Steps:
      1. ls -la .sisyphus/evidence/report-website-full/ | tee .sisyphus/evidence/report-website-full/task-7-evidence-summary.txt
      2. wc -l .sisyphus/evidence/report-website-full/task-7-evidence-summary.txt
    Expected Result: Summary with 6+ files
    Evidence: .sisyphus/evidence/report-website-full/task-7-evidence-summary.txt
  ```

  **Commit**: NO

  ---

## Final Verification Wave (MANDATORY)

- [ ] F1. **Final Compliance Audit** — `oracle`
  
  Read the original report-website-full plan. Verify all 18 tasks are complete by checking code exists.
  
  Output: `Tasks Complete [18/18] | Evidence [6+/N] | VERDICT: APPROVE/REJECT`

---

## Success Criteria

### Verification Commands
```bash
# All should pass
go build ./... && go vet ./...
go test ./internal/report/...
./bin/pratc report --help
ls .sisyphus/evidence/report-website-full/ | wc -l  # Should be 6+
```

### Final Checklist
- [ ] Evidence directory exists with 6+ files
- [ ] All Go tests pass
- [ ] Build passes
- [ ] Web components verified
- [ ] Original plan shows 18/18 tasks complete
