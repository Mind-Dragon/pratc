# Roadmap

prATC development roadmap for versions 1.4 through 1.7.

## Version 1.4 — Full-Corpus Triage Engine (Q2 2026)

### Goal
Turn prATC into a full-corpus decision engine that can account for every open PR in a large repository, peel away noise in layers, and produce an explainable now/future map for human attention.

### Phase 0 — Foundation (COMPLETED)

Work already shipped that v1.4 builds on top of.

#### Planning Integration (shipped)
- PoolSelector: weighted multi-component priority scoring (staleness, CI, security, cluster coherence, time decay)
- HierarchicalPlanner: 3-level cluster-based planning reducing O(C(n,k)) to O(C(clusters,c) × C(avg,s))
- PairwiseExecutor: sharded parallel conflict detection with early-exit
- TimeDecayWindow: exponential decay with protected lanes for security/urgent PRs
- PriorityWeights configuration with settings round-trip
- `--planning-strategy` CLI flag (formula or hierarchical)

#### Analyst PDF Report (partially shipped)
- Full PR analysis table with reason codes
- Spam/junk classification (trivial_change, bot_generated, spam_pattern, malformed, promotional, abandoned_no_activity)
- Duplicate detail list with similarity scores
- Category summary section with rollup counts
- Recommendations section (top-N inspect, close duplicates, close spam, re-engage)

#### Review Pipeline (shipped in v1.3)
- Advisory analyzers for security, reliability, performance, quality
- Evidence-backed review output with confidence scores
- PR buckets: merge_now, focused_review, duplicate/superseded, problematic, escalate

#### Known debt carried forward
- 5 pre-existing test failures on main: TestHandleAnalyze (x3), TestCorsMiddleware (x2) in internal/cmd/
- `maxPRs` default of 1000 in analyze command caps visible corpus
- `DefaultPoolCap = 100` hard-caps the candidate pool
- The detailed technical reference now lives in ARCHITECTURE.md; docs/techref.md has been removed.
- `pratc14a.md` remediation plan archived to `docs/archive/pratc14a-remediation.md`; relevant items absorbed into v1.4 workstreams

### Phase A — Corpus Coverage + Baseline Repair

Make the system account for every PR before any judgment is applied. Fix the known debt from Phase 0.

#### Deliverables
- fix the 5 pre-existing test failures (TestHandleAnalyze x3, TestCorsMiddleware x2)
- remove the hidden maxPRs=1000 default that silently truncates the corpus
- make DefaultPoolCap configurable or remove it as a hard gate on analysis coverage
- ingest the full PR set with no hidden truncation
- keep every PR represented in storage, analysis, and reporting
- preserve a reason trail for every decision made about a PR
- support repositories at 6,000+ PR scale without silently dropping work
- add a large-corpus fixture or benchmark case for 6,000+ PRs

### Phase B — Outer Peel

Remove the obvious noise before deeper reasoning consumes time.

#### Deliverables
- layer 1 (garbage): identify abandoned, malformed, empty, and low-signal PRs
- layer 2 (duplicates): detect repeated ideas, choose canonical, fold the rest
- layer 3 (obvious badness): classify spam, malware, junkware, structurally broken PRs
- make every discard reason visible and auditable
- never silently drop a PR — everything lands in a bucket with a reason

### Phase C — Substance Scoring

Score the remaining work against the things that actually matter.

#### Deliverables
- layer 4 (substance): score PRs for security, reliability, performance, and roadmap alignment
- extend existing review analyzers (internal/review/) to emit layer-4 scores
- assign a composite score and per-dimension reasons
- let low-scoring PRs fall out of the active queue into low_value bucket

### Phase D — Now vs Future Routing

Separate work that should happen now from work that belongs later.

#### Deliverables
- layer 5 (temporal routing): split work into current priorities and future priorities
- keep future items visible but out of the now queue
- treat "good, but later" as a distinct outcome, not a failure

### Phase E — Deep Judgment Layers

Apply deeper layers to decide what deserves human time.

#### Deliverables
- layer 6: confidence — do we know enough to trust the judgment?
- layer 7: dependency — is it blocked on something else?
- layer 8: blast radius — how much damage if this goes wrong?
- layer 9: leverage — does it unlock other work?
- layer 10: ownership — is there a real path to completion?
- layer 11: stability — is it settled enough to act on?
- layer 12: mergeability — can it land cleanly?
- layer 13: strategic weight — does it move the project in the right direction?
- layer 14: attention cost — how expensive is it for a human to understand?
- layer 15: reversibility — if we act and regret it, can we undo it?
- layer 16: signal quality — is this real signal, or noise with good packaging?

### Phase F — Report and Output

Compose the full-corpus report.

#### Deliverables
- executive summary
- ranked "do this now" section
- "defer to future" section
- duplicates section with canonicals and chains
- junk/noise section
- risk and quality scoring section
- full appendix covering every PR
- every PR has at least one reason code

### Guardrails
- no auto-merge
- no silent exclusion
- no hidden caps on analysis coverage
- no PR may vanish without an explanation
- every bucket must be defensible with reason codes

### Success Metrics
- every PR in the repo is accounted for
- every PR lands in a meaningful bucket
- the report answers "what matters now?" and "what can wait?"
- the system stays honest about uncertainty
- 6,000+ PRs are handled as a normal operating case, not an exception
- all tests on main are green before Phase B begins

---

## Version 1.5 — Dashboard Enhancements (Q3 2026)

### Goal
Transform the web dashboard from a monitoring tool into a full PR triage and merge execution interface.

### Deliverables

#### Triage Inbox
- Filterable PR list with analyzer findings displayed inline
- Bulk actions: approve for merge, request changes, mark as duplicate
- Keyboard shortcuts for rapid triage (vim-style navigation)

#### Merge Execution
- **Read-only merge planning** (no auto-merge)
- Generate merge instructions with conflict warnings
- Copy-to-clipboard merge commands for operators
- Integration with GitHub CLI for one-click merge execution

#### Real-Time Collaboration
- WebSocket-based live updates for multi-operator workflows
- Operator presence indicators ("Alice is reviewing PR #123")
- Comment threads on PRs (stored locally, not pushed to GitHub)

#### Mobile Responsiveness
- Full-featured mobile interface for on-call operators
- Push notifications for high-priority PRs (via Telegram integration)
- Offline mode with local cache

### Guardrails
- No auto-merge or auto-approve behavior (read-only planning only)
- All merge execution requires explicit operator confirmation
- No GitHub App or webhook expansion (manual trigger only)

### Success Metrics
- Triage time per PR: <30 seconds (current: ~2 minutes)
- Operator adoption: 80% of team using dashboard daily
- Merge conflict rate: <5% of merged PRs

---

## Version 1.6 — Evidence Enrichment (Q4 2026)

### Goal
Enhance analyzer evidence beyond metadata to include diff analysis, subsystem detection, and test coverage impact.

### Deliverables

#### Diff Analysis
- Parse unified diff to detect:
  - Files touched by subsystem (security/, auth/, api/, etc.)
  - Risky patterns (SQL queries, auth checks, crypto operations)
  - Test file changes (coverage impact estimation)
- Integrate with `internal/review/` analyzers for evidence-backed findings

#### Dependency Impact
- Detect changes to:
  - Public APIs (breaking change detection)
  - Shared libraries (impact on downstream consumers)
  - Configuration schemas (migration requirements)
- Cross-reference with dependency graph for downstream impact

#### Test Evidence
- Identify test files changed alongside source files
- Estimate coverage impact (lines changed in tested vs untested code)
- Flag PRs that modify production code without corresponding test changes

#### Evidence Presentation
- Structured evidence in review output:
  ```json
  {
    "analyzer_name": "security",
    "finding": "auth_bypass_risk",
    "confidence": 0.85,
    "evidence": [
      {"type": "diff_pattern", "pattern": "if user.isAdmin", "line": 142},
      {"type": "subsystem", "path": "src/auth/", "risk": "high"},
      {"type": "test_coverage", "changed": false, "coverage": 0.23}
    ]
  }
  ```

### Guardrails
- Evidence is advisory only — operators make final decisions
- No automated risk scoring or merge blocking
- Diff analysis is read-only (no code modification)

### Success Metrics
- Evidence coverage: 90% of high-risk PRs have diff-based evidence
- False positive rate: <10% of flagged risks are false alarms
- Operator trust: 85% of operators report "evidence is helpful"

---

## Version 1.7 — Multi-Repo + ML Feedback (Q1 2027)

### Goal
Extend prATC beyond single-repo operations to multi-repo aggregate analysis and learned improvements from operator decisions.

### Deliverables

#### Multi-Repo Support
- Aggregate analysis across multiple repositories
- Cross-repo dependency detection
- Unified merge planning for monorepo-style workflows

#### ML Feedback Loop
- Operator decisions as training signals
- Improve duplicate detection accuracy over time
- Personalized scoring based on team preferences

#### GitHub App Integration
- OAuth-based authentication (no PAT management)
- Webhook-triggered analysis (real-time PR updates)
- Status check integration (block merge on high-risk findings)

---

## Guardrails (All Versions)

The following principles apply to all future development:

1. **No auto-merge or auto-approve behavior** — prATC is advisory only
2. **No GitHub App, OAuth, or webhook expansion** — manual trigger only (unless explicitly approved for v1.7+)
3. **No claims of high-confidence merge safety from weak metadata-only signals** — evidence must be strong
4. **Non-commercial use only** — FSL-1.1-Apache-2.0 license (converts to Apache 2.0 after 2 years)
5. **Read-only by default** — all destructive operations require explicit opt-in

---

## How to Contribute

prATC welcomes contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

For roadmap discussions, open a GitHub Issue with the `roadmap` label.

For commercial licensing inquiries, contact: jefferson@heimdallstrategy.com
