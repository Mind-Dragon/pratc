# prATC Internal Roadmap

Internal strategic notes for where the product gets genuinely strange and powerful after the v1.6 pipeline-first reset.

Status: historical strategy note. The active execution plan is now `VERSION2.0.md`. Concepts here that survived into the current plan: continuous state, diff-grounded evidence, PR memory, backlog surgery, and operator-grade report/dashboard surfaces.

## Grounded take from the repo

- The real product already lives in the pipeline plus durable artifacts.
- `serve` already gives us the API surface.
- The standalone review endpoint is still not implemented anyway.
- The browser dashboard (`web/`) is not the active product surface; v2.0 dashboard work is TUI-first.

If the goal is a 24/7 system that eats PRs and processes them, the center of gravity should be:

`sync → analyze → review → action-plan/dashboard → persistent state → executor → repeat`

Not:

`sync → dashboard → vibes`

---

## Five things that would take prATC from “wow, triage” to “IT DOES WHAT?!”

### 1. Continuous ingest + incremental re-analysis

**Difficulty:** medium  
**Impact:** huge

#### What it is
- prATC runs as a long-lived daemon/service
- continuously watches one repo or a configured set
- pulls new/changed PRs on a schedule
- only reprocesses changed PRs plus affected neighbors
- keeps a warm analysis state all day

#### Why it matters
Right now prATC feels like a powerful batch engine.  
This turns it into a living control system.

#### The “IT DOES WHAT?!” moment
“You mean it just stays on, notices backlog drift, and keeps the corpus current without me re-running a giant workflow?”

#### Notes
This is very aligned with the intended 2.0 direction.

---

### 2. Diff-grounded evidence, not just metadata heuristics

**Difficulty:** hard  
**Impact:** massive

#### What it is
- analyzers inspect actual diff hunks, not just filenames, labels, or counts
- security finds removed auth checks, token handling, secret exposure, and dangerous config changes
- reliability finds error-path changes, retries removed, and timeout handling changed
- performance finds new nested loops, hot-path query changes, and cache bypasses
- quality finds broad refactors with weak explanation and no tests

#### Why it matters
This is the single biggest jump in credibility.

Right now 1.x mostly says:

> this PR looks risky because of where it touches

The next leap is:

> this PR is risky because here is the exact changed hunk and why it matters

#### The “IT DOES WHAT?!” moment
“It read the patch and surfaced the likely bad line.”

---

### 3. PR memory and longitudinal judgment

**Difficulty:** hard  
**Impact:** huge

#### What it is
For every PR, keep evolving state:
- when it entered
- how its confidence changed over time
- whether it got rebased, went stale, recovered, or got superseded
- whether other PRs converged on the same idea
- whether CI stabilized or stayed flaky
- whether an author tends to land clean work in this repo

#### Why it matters
A PR is not a static object.  
The really interesting system judges trajectory, not just snapshot state.

This would let prATC say things like:
- “this PR was junk three weeks ago and is now mergeable”
- “this author’s last 8 PRs in this subsystem landed clean”
- “this backlog cluster is converging toward PR #412 as canonical”

#### The “IT DOES WHAT?!” moment
“It remembers the life story of the backlog.”

---

### 4. Report packet becomes live TUI action dashboard

**Difficulty:** medium  
**Impact:** huge

#### What it is
Make the report concepts the product surface, not just a static PDF. The PDF remains the snapshot packet; the TUI becomes the live dashboard.

For each run, emit:
- executive summary
- now / future / blocked queues
- canonical duplicate chains
- high-risk patch evidence
- action lanes and queue state
- executor/preflight state
- proof bundle status
- “if I wanted to reduce backlog fastest, close these 40”
- “if I wanted to unlock merge flow, resolve these 6 conflict hubs”
- “this run changed these judgments since last run”

#### Why it matters
The PDF becomes the snapshot packet; the TUI becomes the live battle board. If the TUI exposes the same reasoning with queue/executor state, nobody will miss the browser dashboard.

#### The “IT DOES WHAT?!” moment
“It hands me a battle plan, not a report.”

---

### 5. Backlog surgery: canonicalization + collapse recommendations

**Difficulty:** medium-hard  
**Impact:** absurdly high

#### What it is
Not just “these are duplicates.” Actually:
- identify idea clusters
- choose canonical PR
- list superseded PRs
- propose closure order
- propose merge order
- propose rebase order
- identify hub PRs that unblock many others

Basically, turn a PR backlog into a dependency-managed cleanup plan.

#### Why it matters
This is where prATC becomes weirdly powerful. Most tools stop at review. This starts doing backlog compression.

#### The “IT DOES WHAT?!” moment
“It told me how to collapse 600 open PRs into 80 meaningful work items.”

---

## Honest ranking for what to do next

If we want maximum holy hell per unit effort:

1. Diff-grounded evidence
2. Report as operator packet
3. Continuous ingest + incremental analysis
4. Backlog surgery / canonicalization
5. Longitudinal PR memory

---

## Relationship to current plan

The v1.6 reset prepared the foundation. `VERSION2.0.md` is now the active plan for:
- full-corpus ActionPlan
- action lanes
- TUI dashboard
- swarm work queue
- proof bundles
- centralized executor
- guarded/autonomous GitHub actions with live preflight
