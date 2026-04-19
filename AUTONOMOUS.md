# prATC Autonomous Testing Cycle

## Purpose

This document defines how prATC runs autonomous testing cycles using the TUI monitor as the live dashboard and Hermes subagent delegation as the execution engine. The cycle ingests a corpus, evaluates each GUIDELINE rule against real output, identifies gaps, dispatches subagents to fix them, and iterates until the triage engine produces output that satisfies every non-negotiable rule.

This is not a planning document. It is a runtime loop specification.

## Why this exists

The overnight run proved the pipeline runs end-to-end but the output does not satisfy GUIDELINE.md. Zero of 4,992 PRs have bucket assignments, reason trails, or confidence scores. The graph has 800K spurious dependency edges. Layers 4-16 are scaffolded but not computed. The PDF renders structurally correct sections filled with empty data.

Manual fix-and-recheck does not scale against 16 layers, 4,992 PRs, and a 28-minute pipeline. The autonomous cycle replaces that manual loop with a structured, repeatable, verifiable process.

## Architecture

```
┌─────────────────────────────────────────────────┐
│                TUI Monitor                       │
│  ┌──────────┬──────────┬──────────┐             │
│  │  Jobs    │ Timeline │  Rate    │  ← ws://    │
│  │  Panel   │  Panel   │  Limit   │  localhost  │
│  ├──────────┴──────────┴──────────┤             │
│  │       Console / Log Feed       │             │
│  └────────────────────────────────┘             │
│         ↑ SSE events from prATC serve           │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│           Hermes Controller Session              │
│                                                  │
│  1. Run pipeline (sync → analyze → report)       │
│  2. Audit output against GUIDELINE rules         │
│  3. Generate GAP_LIST for failing rules          │
│  4. Dispatch fix subagents per gap               │
│  5. Re-run pipeline on same corpus               │
│  6. Re-audit. If new gaps, loop to 3.            │
│  7. If all rules pass or budget exhausted, stop. │
│                                                  │
│  Uses: delegate_task for all implementation      │
│  Uses: TUI monitor for live observation          │
└─────────────────────────────────────────────────┘
```

## The cycle

### Phase 0: Bootstrap

1. Start `pratc serve` on port 7400 (or confirm it is running via `/healthz`).
2. Start the TUI monitor: `pratc monitor` in a separate tmux pane. The controller does not interact with the TUI directly — it watches via the console log feed and WebSocket events for live pipeline progress.
3. Confirm the corpus exists: check `projects/OpenClaw_OpenClaw/runs/<latest>/sync.json` or trigger a fresh `pratc sync --repo=openclaw/openclaw`.
4. Record the starting commit hash and test suite status: `go test ./...`.

### Phase 1: Run the pipeline

Execute the full workflow against the target corpus:

```bash
# Step 1: Sync (skip if already cached)
pratc sync --repo=openclaw/openclaw --use-cache-first

# Step 2: Analyze
pratc analyze --repo=openclaw/openclaw --use-cache-first --format=json

# Step 3: Cluster
pratc cluster --repo=openclaw/openclaw --format=json

# Step 4: Graph
pratc graph --repo=openclaw/openclaw --format=json

# Step 5: Plan
pratc plan --repo=openclaw/openclaw --target=20 --format=json

# Step 6: Report
pratc report --repo=openclaw/openclaw --input-dir=projects/OpenClaw_OpenClaw/runs/<latest>
```

Alternatively, if a `workflow` command exists that chains all steps, use that.

The TUI monitor shows live progress for each step. The controller watches for completion or failure.

### Phase 2: Audit against GUIDELINE rules

After the pipeline completes, run a programmatic audit of the output artifacts. The audit checks every rule from GUIDELINE.md against the actual data.

#### Rule audit matrix

Each rule maps to a concrete, checkable assertion against the output JSON:

| # | GUIDELINE rule | Audit check | Pass condition |
|---|----------------|-------------|----------------|
| 1 | Every PR must be accounted for | `len(analyze.json.prs) >= counts.total_prs - cap_delta` | No PRs silently dropped |
| 2a | Every decision needs a reason: bucket | Every PR in analyze.json has non-empty `bucket` field | 100% coverage |
| 2b | Every decision needs a reason: reason trail | Every PR has `review.reasons` or `review.bucket_reason` with length > 0 | 100% coverage |
| 2c | Every decision needs a reason: confidence | Every PR has `review.confidence` as float 0.0-1.0 | 100% coverage, none missing |
| 3 | Duplicates are collapsed | `counts.duplicate_groups > 0` OR corpus genuinely has < 10 | Reasonable for corpus |
| 4 | Garbage gets removed early | `counts.garbage_prs > 0` and garbage pass ran before duplicates | Order verified in logs |
| 5 | Future work stays visible | Some PRs have bucket `future` | Not zero unless corpus is small |
| 6 | Deeper judgment after obvious layers | Pipeline order: garbage → dup → badness → substance → routing → deep | Verified in code + logs |
| N1 | No auto-merge | No merge commands emitted | Binary search of output |
| N2 | No silent exclusion | `len(rejections)` + `len(selected)` + `len(garbage)` + `len(duplicates)` >= `total_prs` | Every PR accounted |
| N3 | No opaque ranking without reasons | Top-N selected PRs all have reason strings | 100% |
| N4 | No claiming certainty when confidence is low | No PR with confidence < 0.5 has bucket implying certainty | Cross-check |

#### Additional quantitative checks

| Check | Source | Pass condition |
|-------|--------|----------------|
| Conflict pairs < 5,000 | graph.json edges where `edge_type=conflicts_with` | Count < 5,000 |
| Dependency edges meaningful | graph.json edges where `edge_type=depends_on` | Reason is not "base branch X depends on head branch X" for > 50% |
| Analysis time < 15 min (cached) | timestamps in artifacts | `step-2-analyze` end - start < 900s on second run |
| Duplicate groups > 10 | analyze.json `counts.duplicate_groups` | > 10 or documented reason |
| Garbage classifier catches > 80% | Manual spot-check of 20 non-garbage PRs for false negatives | < 4 missed |
| PDF renders all 7 sections | report.pdf page count + section headers | Cover, summary, junk, dupes, now/review, future, appendix all present |
| Test suite green | `go test ./...` | 0 failures |

### Phase 3: Generate GAP_LIST

From the Phase 2 audit, produce a GAP_LIST: a prioritized list of failing rules, each with:
- The GUIDELINE rule number and text
- The actual vs expected value from the output
- The code location(s) most likely responsible
- A suggested fix approach

GAP_LIST format:

```
GAP-001: Rule 2a — bucket assignments missing
  Expected: 100% of PRs have non-empty bucket field
  Actual: 0/4992 PRs have bucket assignments
  Location: internal/app/service.go — Analyze() does not assign buckets
  Fix: Wire the review pipeline output into bucket assignment logic after layer 5

GAP-002: Rule N2 — conflict edges 804K trivial dependencies
  Expected: depends_on edges reflect real dependency (shared imports, API surface)
  Actual: 804,678 edges with reason "base branch 'main' depends on head branch 'main'"
  Location: internal/graph/graph.go — Build() creates edges for all same-target-branch PRs
  Fix: Remove same-branch dependency edges or require actual file-level dependency signal
```

### Phase 4: Dispatch fix subagents

For each gap in the GAP_LIST, dispatch a subagent:

**Rules for dispatch:**
- One gap per subagent (or group closely related gaps if they share the same file).
- Every subagent gets: the GAP_LIST entry, the relevant source code paths, the GUIDELINE rule text, and the TDD requirement.
- Every subagent must follow RED-GREEN-REFACTOR: write a failing test that proves the gap, then implement the fix, then verify the test passes.
- Subagents run against the same corpus (cached). They do not re-sync.

**Wave ordering:**

```
Wave 1: Data model gaps
  - Missing fields on types (bucket, confidence, reasons, deep judgment scores)
  - These are foundation — everything else depends on them

Wave 2: Core logic gaps
  - Bucket assignment in the pipeline
  - Dependency edge filtering
  - Garbage classifier tuning
  - Conflict noise tuning

Wave 3: Wiring gaps
  - Review output → analyze response
  - Deep judgment layer computation
  - PDF section population

Wave 4: Verification
  - Full pipeline re-run
  - GUIDELINE audit pass
  - Test suite
  - Doc sync
```

**Subagent context template:**

```
GOAL: Fix GAP-XXX: [description]

GUIDELINE RULE:
[RULE TEXT]

CURRENT BEHAVIOR:
[What the code does now — include file paths and line numbers]

EXPECTED BEHAVIOR:
[What the code should do to pass the audit check]

AUDIT ASSERTION:
[The specific check from Phase 2 that fails]

TDD REQUIREMENT:
1. Write a failing test that proves the gap exists
2. Verify the test fails: go test ./internal/.../ -run TestGapXXX -v
3. Implement the minimal fix
4. Verify the test passes
5. Verify no regressions: go test ./...

ENVIRONMENT:
- Working dir: /home/agent/pratc
- Build: go build ./...
- Test: go test ./...
- Corpus: projects/OpenClaw_OpenClaw/runs/20260419-065654/
- Cache: ~/.pratc/pratc.db
```

### Phase 5: Re-run and re-audit

After each wave completes:

1. `go build ./...` — confirm build
2. `go test ./...` — confirm test suite
3. Re-run the pipeline (steps 2-6 from Phase 1, skip sync if corpus unchanged)
4. Re-run the Phase 2 audit
5. If new gaps appear or old gaps resurface, add them to GAP_LIST and continue
6. If all rules pass, proceed to Phase 6

### Phase 6: Closeout

1. Final GUIDELINE audit: all rules pass.
2. Quantitative checks: conflicts < 5K, duplicates > 10, analysis < 15 min cached.
3. Test suite: `go test ./...` green.
4. Build: `go build ./...` clean.
5. Commit all changes with a descriptive message.
6. Update TODO.md, ROADMAP.md, CHANGELOG.md to reflect the verified state.
7. Verify the TUI monitor shows the completed run with honest status.

## Controller behavior rules

### What the controller does
- Orchestrates the cycle (Phase 0 through Phase 6)
- Runs the pipeline
- Runs the audit
- Generates the GAP_LIST
- Dispatches subagents
- Verifies subagent output
- Re-runs the audit
- Commits when green
- Updates docs

### What the controller does NOT do
- Implement code directly (all fixes go through subagents)
- Modify the GAP_LIST without running the audit
- Skip the TDD requirement for any gap
- Mark a gap as fixed without re-running the audit
- Proceed past a failing test suite

### Turn budget
- Controller: no hard limit (the cycle may need multiple iterations)
- Fix subagents: 300 turns each (enough for TDD cycle + debugging)
- Audit subagents: 100 turns each
- Doc sync subagent: 100 turns

### Stop conditions
The cycle stops when one of:
- All GUIDELINE rules pass the audit (SUCCESS)
- 3 full iterations without progress on any remaining gap (STALLED)
- The test suite is red and cannot be repaired in 2 waves (BLOCKED)
- Token or time budget exhausted (BUDGET)

On STALLED or BLOCKED, record the remaining gaps in TODO.md and report to the operator.

## TUI monitor integration

The TUI is not a control surface — it is an observation surface. The controller uses it to:

1. **Watch pipeline progress.** The Jobs panel shows sync/analyze/cluster/graph/plan/report steps. The controller can see when a step completes or fails without polling the filesystem.

2. **Monitor rate limits.** The RateLimit panel shows remaining GitHub API budget. If the budget drops below 200 during sync, the controller should pause and wait rather than dispatching pipeline steps that will fail.

3. **Read console logs.** The Console panel shows structured log entries. The controller can watch for error-level entries that indicate a pipeline step failed before the step officially exits.

4. **Observe timing.** The Timeline panel shows request activity. Long gaps indicate stalls. Bursts indicate active processing.

The controller connects to the same WebSocket (`ws://127.0.0.1:7400/sync/stream`) that the TUI uses. It can also read the server logs via `journalctl` if the service runs under systemd, or via the HTTP health endpoint.

## Audit script

The audit should be implemented as a standalone script at `scripts/audit-guideline.sh` (or `.py`) that:
1. Takes the run directory as an argument
2. Reads all artifact JSON files
3. Checks every rule in the matrix above
4. Prints PASS/FAIL for each rule with actual values
5. Exits 0 if all rules pass, 1 if any fail
6. Optionally writes results to `AUDIT_RESULTS.json` in the run directory

This makes the audit reproducible outside the controller session and usable in CI.

## File locations

```
pratc/
├── AUTONOMOUS.md              ← this document
├── scripts/
│   └── audit-guideline.py     ← the audit script (to be created)
├── projects/
│   └── OpenClaw_OpenClaw/
│       └── runs/
│           └── <timestamp>/
│               ├── AUDIT_RESULTS.json   ← audit output per run
│               ├── GAP_LIST.md          ← gaps found in this run
│               ├── analyze.json
│               ├── step-3-cluster.json
│               ├── step-4-graph.json
│               ├── step-5-plan.json
│               ├── report.pdf
│               └── sync.json
└── internal/
    └── ... (fixes land here)
```

## Known gaps from first audit (2026-04-19 run)

These are the starting gaps the first autonomous cycle should address:

| ID | Rule | Gap | Priority |
|----|------|-----|----------|
| G-001 | 2a | Zero bucket assignments on all 4,992 PRs | P0 |
| G-002 | 2b | Zero reason trails on all PRs | P0 |
| G-003 | 2c | Zero confidence scores on all PRs | P0 |
| G-004 | N2 | 804,678 trivial dependency edges | P1 |
| G-005 | N2 | 380,716 conflict edges (target: < 5,000) | P1 |
| G-006 | 5 | No temporal routing (now/future/blocked) | P1 |
| G-007 | — | Layers 6-16 not computed | P2 |
| G-008 | — | Layers 4-5 (substance score) not computed | P2 |
| G-009 | — | Duplicate groups only 9 (target: > 10) | P2 |
| G-010 | — | Garbage PRs only 8 (may be conservative) | P3 |

## Relationship to other documents
- **GUIDELINE.md** defines the rules this cycle audits against.
- **ARCHITECTURE.md** defines the system shape and data flow.
- **TODO.md** tracks the current work state including any gaps from the last cycle.
- **ROADMAP.md** defines version milestones.
- **This document** defines the autonomous testing process.
- If GUIDELINE.md changes, the audit matrix must be updated before the next cycle.
