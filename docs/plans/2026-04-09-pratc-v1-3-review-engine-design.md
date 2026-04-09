# PRATC v1.3 Review Engine Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:writing-plans to create the implementation plan for this design.

**Goal:** Turn PRATC from a backlog triage tool into an evidence-backed PR review engine that can rank, classify, and recommend actions for very large PR backlogs like OpenClaw.

**Architecture:** PRATC v1.3 will use a staged review pipeline: broad triage on all PRs, targeted evidence enrichment on prioritized PRs, specialized analyzers for security/reliability/performance/quality, synthesis into operator-facing buckets, and operational workflows for backlog handling. The system remains advisory-only.

**Tech Stack:** Go CLI/backend, existing Python ML bridge as optional augmentation, SQLite cache, existing web dashboard, existing PDF/report surfaces.

---

## 1. Problem Statement

The current PRATC implementation can organize PRs, cluster duplicates, and apply lightweight heuristic analyzers, but it does not yet perform sufficiently deep security, performance, reliability, or merge-safety analysis for a large repository like `openclaw/openclaw`.

Today, the system is best described as a **triage engine**. It is not yet a trustworthy **review engine**. The existing report structure can display categories, but many conclusions are still driven by metadata, labels, filenames, and CI state rather than evidence-rich analysis.

For OpenClaw-scale PR sets, PRATC v1.3 must help answer:

- Which PRs are safest to merge now?
- Which PRs are mergeable but require focused review?
- Which PRs are duplicates or superseded?
- Which PRs are noisy, suspicious, broken, or poor quality?
- Which PRs need escalation because evidence is incomplete or analyzers disagree?

---

## 2. Scope Decision

PRATC v1.3 targets **advisory-only, evidence-backed merge recommendations and risk reports**.

That means:

- PRATC may **recommend** merge actions.
- PRATC may **prioritize** PRs for review.
- PRATC may **quarantine** problematic PRs in reports.
- PRATC may **escalate** uncertain/high-risk PRs.

But PRATC v1.3 must **not**:

- auto-merge PRs
- auto-approve PRs
- post review decisions back to GitHub as actions
- introduce GitHub App / OAuth / webhook complexity

This keeps the system operationally safe while increasing trustworthiness.

---

## 3. Chosen Architecture

### Selected Option: Evidence-Backed Staged Review Engine

We considered:

1. **Heuristic-first triage engine only**  
   Fastest, but too weak for trustworthy merge recommendations.

2. **Evidence-backed staged review engine**  
   Recommended. Supports both scale and trust.

3. **Full agent swarm reviewer**  
   Potentially powerful, but too expensive/noisy/unstable for default operation across thousands of PRs.

### Why Option 2 wins

It preserves the strengths of the current system (scale, ranking, clustering) while creating a path to credible review decisions. It also fits the existing Go + Python hybrid architecture and lets the system remain useful even when optional ML/model-backed analysis is unavailable.

---

## 4. Five-Layer Review Pipeline

### Layer 1: Triage

Run on **all** open PRs.

Inputs:
- PR metadata
- labels
- author/bot signal
- draft status
- mergeability
- CI state
- changed file counts
- additions/deletions
- duplicate/overlap/cluster membership
- stale signals
- subsystem touched

Outputs:
- top merge candidates
- high-risk backlog
- likely duplicates
- likely noise/problematic PRs
- deep-review candidate list

Purpose:
This layer answers **what deserves attention first**, not what is safe to merge.

---

### Layer 2: Evidence Enrichment

Run only on prioritized PRs and high-risk/core-system PRs.

Required enrichment for baseline high-confidence review:
- changed files
- patch/diff evidence
- subsystem impact
- dependency/config changes
- test changes / missing test evidence
- ownership/blame context when useful

Additional enrichment for high-risk/core PRs:
- runtime / production evidence
- incident or error context
- performance-sensitive operational signals
- rollout/migration context

Purpose:
This layer prevents PRATC from making strong conclusions from filenames alone.

---

### Layer 3: Specialized Analyzers

PRATC v1.3 should include the following analyzers:

- **Security analyzer**
- **Reliability analyzer**
- **Performance analyzer**
- **Quality analyzer**
- **Duplicate/supersession engine**

Each analyzer must return:
- findings
- blockers
- evidence references
- confidence
- recommended next action

Purpose:
This layer provides depth. PRATC stops being just a sorter and becomes a review assistant.

---

### Layer 4: Synthesis

All analyzer and deterministic results are combined into final recommendations.

Target output buckets:

1. **Merge now**
2. **Merge after focused review**
3. **Duplicate / superseded**
4. **Problematic / quarantine**
5. **Unknown / escalate**

Each final decision must carry:
- final category
- final priority tier
- confidence
- blockers
- supporting evidence
- human next step

Purpose:
This layer translates raw findings into operational decisions.

---

### Layer 5: Operational Workflow

This layer is about making the outputs actionable for OpenClaw-scale repositories.

It should support:
- ranked queues for operators
- focused-review buckets by subsystem/risk class
- duplicate suppression workflows
- quarantine queues for broken/suspicious PRs
- escalation workflows for uncertain/core-risk PRs

Purpose:
This layer helps a human team actually work a giant PR backlog.

---

## 5. Evidence Model

### Tier A - Triage Evidence

Available on all PRs:
- title/body
- labels
- author / bot indicators
- CI state
- mergeability
- file counts
- additions/deletions
- cluster membership
- duplicate/overlap hints
- stale/draft state

Useful for:
- ranking
- suppression
- broad categorization

Not sufficient for:
- high-confidence merge recommendations

---

### Tier B - Review Evidence

Required by default for high-confidence recommendations:
- metadata
- CI
- diff/file evidence
- subsystem impact
- dependency/config changes
- test evidence or absence

Useful for:
- meaningful merge recommendations
- security/reliability/performance review

---

### Tier C - Runtime / Production Evidence

Required for **higher-risk / core-system PRs**.

Examples:
- error/incident correlation
- performance-sensitive runtime evidence
- rollout/migration context
- service criticality signals

Useful for:
- true high-confidence recommendations on core/risky changes

---

## 6. Confidence Model

Confidence must be tied to evidence quality.

### Rules

- **Low confidence**: metadata-only
- **Medium confidence**: metadata + CI + diff/file evidence
- **High confidence**: multiple independent signals agree
- **High confidence on high-risk/core PRs** requires runtime/production evidence

PRATC must never claim “safe to merge” with high confidence from filenames alone.

This is the key difference between a trustworthy review engine and a shallow classifier.

---

## 7. Analyzer Responsibilities

### Security Analyzer

Questions:
- Did this PR touch auth, permissions, secrets, credentials, tokens, config, or risky dependency surfaces?
- Does the diff suggest a security-sensitive change requiring review?

v1.3 requirement:
- move beyond filename-only heuristics for strong recommendations

---

### Reliability Analyzer

Questions:
- Is this PR likely to introduce failure handling, retry, rollout, or operational instability risk?
- Do CI, mergeability, and review churn suggest unstable change?

---

### Performance Analyzer

Questions:
- Does this PR touch likely hot paths?
- Does it suggest algorithmic, query, caching, or fanout risk?
- Does it need performance review or runtime evidence?

---

### Quality Analyzer

Questions:
- Is this PR too weak/noisy/under-explained to review effectively?
- Is it missing tests, issue linkage, or enough description?

---

### Duplicate / Supersession Engine

Questions:
- Is this PR a duplicate of another open PR?
- Is this PR duplicative of a merged PR?
- Is it superseded by a later/fresher PR?

This should be treated as a first-class review subsystem, not a side heuristic.

---

## 8. Ranking and Priority

Inside the operator-facing buckets, ranking should consider:

- blast radius
- confidence
- subsystem criticality
- review cost
- duplicate likelihood
- freshness/staleness
- test evidence quality

This ensures PRATC answers both:

1. **What should I look at first?**
2. **What is safest to merge next?**

Those are not always the same question.

---

## 9. Plugin / Analyzer Architecture

### v1.3 baseline

Use deterministic analyzers + evidence enrichment as the core system.

Optional ML/model-backed analyzers may augment the system, but:
- Go orchestrator remains source of truth
- Python analyzers remain optional
- no analyzer should be required for baseline correctness

### Required guarantees

- advisory-only
- read-only
- stateless
- timeout-bounded
- deterministic invocation order
- confidence and evidence required in output

---

## 10. What v1.3 Must Avoid

To remain within safe scope, v1.3 must not drift into:

- auto-merge behavior
- GitHub App / OAuth / webhook integration
- multi-repo orchestration UI
- agent-only or model-only decision making
- claiming certainty from weak evidence

---

## 11. Success Criteria

PRATC v1.3 succeeds if it can:

1. triage all open PRs at scale
2. enrich the right PRs with stronger evidence
3. classify PRs into actionable operator buckets
4. detect duplicates against both open and merged PRs
5. provide evidence-backed merge recommendations
6. escalate uncertain/high-risk/core PRs instead of bluffing

---

## 12. Recommended Next Step

The next step is to convert this design into a focused implementation plan for:

- deeper evidence enrichment
- stronger analyzer wiring into reports/dashboard/API
- confidence calibration
- backlog workflow outputs for OpenClaw-scale repositories

This should be done as a dedicated implementation plan rather than continuing to patch the current v1.0 system ad hoc.
