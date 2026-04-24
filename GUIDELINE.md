# prATC Guideline

## Purpose

This document sets the operating rules for the v1.x full-corpus triage engine and the v2.0 action engine.

The system does not get to ignore PRs. It does not get to hide uncertainty. It does not get to collapse the world into a single score and pretend that is enough. In v2.0, it also does not get to mutate GitHub directly from a vague recommendation: every action must be typed, preflighted, auditable, and policy-allowed.

## Core rules

1. Every PR must be accounted for.
   - If a PR is in the repo, it must enter the pipeline.
   - No hidden cap may silently remove PRs from consideration.
   - If something is deferred, discarded, or folded into something else, that decision must be visible.

2. Every PR follows the same funnel.
   - The 16-gate ladder is not optional metadata or a best-effort summary.
   - Every non-garbage PR must traverse the same ordered gates.
   - A PR may exit the funnel early, but it must still record where it exited and why.

3. Every decision needs a reason.
   - Each PR gets a bucket, a score, and a reason trail.
   - Unknowns and weak confidence must be recorded, not erased.
   - A human or agent should be able to ask, "why did this land here?" and get a straight answer.

4. Duplicates are not separate problems.
   - If two PRs are the same idea, one becomes canonical and the rest become linked duplicates.
   - Duplicate handling should reduce noise, not hide information.
   - The system should advance toward synthesis-ready duplicate groups, not stop at mere detection.

5. Garbage gets removed early.
   - Obvious spam, junk, malformed PRs, and low-integrity noise should not be allowed to consume deep review.
   - Bad actors and obvious trash should be classified quickly and visibly.
   - The outer peel should remove broad junk cheaply before the inner layers spend real CPU.

6. Future work stays visible.
   - A PR can be good and still not belong in the current pile.
   - "Later" is a valid outcome.
   - Deferral is not failure.

7. The cost of judgment must rise inward.
   - Outer layers should be cheap, broad, and aggressive.
   - Inner layers should be more expensive, more precise, and reserved for surviving PRs.
   - The system must not spend diff-level or synthesis-level effort on obvious trash.

8. The product surface is CLI + API + TUI + PDF.
   - CLI exists for humans operating the system.
   - API exists for AI systems and swarm workers consuming structured outputs.
   - TUI exists as the live terminal dashboard and operator control surface for v2.0 action lanes, queue leases, executor state, and audit stream.
   - PDF exists as a point-in-time human-facing snapshot artifact, not the live control plane.
   - Browser/dashboard surfaces are not part of the active v2.0 product contract unless explicitly revived later.

9. Action decisions are separate from bucket decisions.
   - A bucket describes what the PR is.
   - An action lane describes what should happen next.
   - An ActionIntent describes a possible mutation or non-mutating operation.
   - No ActionIntent may execute without policy approval, live preflight, reason trail, evidence refs, idempotency, and audit ledger write.

## The 16-layer decision ladder

Every PR is processed as a funnel.

- Every PR enters at Gate 1.
- Each gate emits an explicit outcome.
- A PR either exits at a gate with a recorded reason or advances inward to the next gate.
- Gate order is fixed in v1.6 and must run from cheaper outer judgment to more expensive inner judgment.
- The ladder is therefore both a decision model and a cost-control model.

### Outer peel
1. Garbage — is this worth looking at at all?
2. Duplicates — is this really the same as something else?
3. Obvious badness — is it junk, spam, malware, or structurally broken?

### Substance
4. Substance score — does this change appear substantial enough to justify deeper work?
5. Now vs future — does this belong in current priorities, or later?

### Truthfulness and context
6. Confidence — do we know enough to trust the judgment?
7. Dependency — is it blocked on something else?
8. Blast radius — how much damage if this goes wrong?
9. Leverage — does it unlock other work?
10. Ownership — is there a real path to completion?

### Readiness and strategy
11. Stability — is it settled enough to act on?
12. Mergeability — can it land cleanly?
13. Strategic weight — does it move the project in the right direction?
14. Attention cost — how expensive is it for a human or agent to understand?
15. Reversibility — if we act and regret it, can we undo it safely?
16. Signal quality — is this real signal, or noise with good packaging?

## Buckets

Every PR must land in at least one bucket. A PR may hold multiple buckets when they don't conflict (e.g., `now` + `high_value`, or `future` + `security_risk`).

### Temporal buckets (mutually exclusive)
- `now` — act on this in the current cycle
- `future` — valid work, but belongs to a later priority window
- `blocked` — cannot proceed until a dependency is resolved

### Quality buckets
- `high_value` — strategically important, likely merge candidate
- `merge_candidate` — ready or near-ready to land
- `needs_review` — substance is there but needs human eyes
- `re_engage` — valid PR with an inactive author; worth reviving
- `low_value` — not harmful, but not worth active attention

### Disposal buckets
- `duplicate` — same idea as another PR; linked to a canonical
- `junk` — spam, malware, malformed, promotional, abandoned noise
- `stale` — has not seen meaningful activity in a long time

### Risk buckets (can be combined with any temporal or quality bucket)
- `security_risk` — touches security-sensitive code or patterns
- `reliability_risk` — may affect system stability
- `performance_risk` — may degrade performance

### Bucket rules
- A PR may move between buckets as better information arrives, but it may not disappear.
- Temporal buckets are mutually exclusive: a PR is `now`, `future`, or `blocked`, never two at once.
- Risk buckets are additive: a PR can be `now` + `security_risk` + `reliability_risk`.
- Disposal buckets are terminal unless overridden by a human: once `junk`, it stays `junk` unless someone says otherwise.
- Every bucket assignment must carry a reason code.

## Action lanes

Every PR must land in exactly one primary action lane before it can become swarm work. Risk buckets remain additive.

- `fast_merge` — clean, low-risk PR that can be merged after live preflight.
- `fix_and_merge` — valid PR that a swarm can repair, test, and resubmit for executor approval.
- `duplicate_close` — duplicate or superseded PR that should be closed or commented with a canonical link.
- `reject_or_close` — invalid, junk, unsafe, or structurally broken PR that should be closed/rejected with a visible reason.
- `focused_review` — meaningful PR that needs deeper agent/human review before an action can be chosen.
- `future_or_reengage` — valid work that belongs later or needs author/product re-engagement.
- `human_escalate` — PR that cannot be safely acted on by automation.

### Action lane rules

- A blocked PR cannot emit a merge ActionIntent.
- A high-risk PR cannot land in `fast_merge` without explicit human override recorded in the ledger.
- A duplicate-close action must name the canonical PR and duplicate group.
- A reject/close action must carry stronger confidence than ordinary routing.
- A fix-and-merge action must produce a proof bundle before executor approval.
- A human-escalate item remains visible and claimable for review, but not executable.

## Policy profiles

- `advisory` — default; produces plans, lanes, dashboard data, and reports with zero GitHub writes.
- `guarded` — allows non-destructive actions such as comments and labels; no merge or close.
- `autonomous` — allows merge/close/comment/label only through typed ActionIntents that pass live preflight and audit.

Policy profile is part of the output contract. A caller must never infer mutation permission from bucket names or scores.

## Confidence

The system uses confidence scores to express how much it trusts its own judgment on a PR.

- Confidence is a float from 0.0 to 1.0.
- Scores below 0.5 mean the system is guessing.
- The existing `HighRiskConfidenceCap = 0.79` convention applies: high-risk findings should not claim confidence above 0.79 without strong evidence.
- Low-confidence bucket placements must be flagged for human review.
- The system must never present a low-confidence judgment as certainty.

## ML honesty and fallback behavior

ML-backed judgments must describe what actually happened, not the best-case path the system hoped to use.

- If embeddings, provider calls, or ML subprocess execution fail, the system must say so and must name the fallback path that was used.
- No silent degradation: a heuristic or rules-based fallback may keep the pipeline running, but it must not be presented as if the embedding-backed path succeeded.
- Any output that relies on ML-style similarity or clustering must be attributable to the backend and mode that produced it: at minimum the backend class, the model when one was actually used, and whether the result came from embeddings, local heuristics, or a non-ML Go path.
- `heuristic-fallback` is a distinct mode, not a cosmetic alias for a real embedding model.
- Local heuristic similarity, cache-first duplicate detection, and embedding-backed similarity are allowed to coexist, but they must not be described as equivalent evidence.
- If the system falls back from one mode to another, the reason for that downgrade should be visible in logs, telemetry, or the reason trail; operators should not have to infer it from missing fields.
- The Go orchestrator remains the source of truth for pipeline decisions. Optional Python analyzers and embedding providers may enrich results, but they do not justify stronger claims than the evidence supports.
- The system must not claim online learning, automatic retraining, feedback-loop improvement, or operator-decision training unless that behavior actually exists in the shipped runtime path and is documented in architecture and contracts.
- Planned ML feedback work belongs in roadmap or design documents until it is implemented; reports and user-facing outputs must describe current behavior, not intended future behavior.

## Non-negotiables
- No unaudited GitHub mutation.
- No direct swarm-worker-to-GitHub mutation path.
- No silent exclusion.
- No opaque ranking without reasons.
- No pretending a truncated view is the whole corpus.
- No changing the meaning of a bucket or action lane without updating this guideline.
- No claiming certainty when confidence is low.
- No merge/close action without live preflight, idempotency key, and post-action verification.
- Advisory mode must perform zero writes.

## What good looks like
- The full corpus is visible.
- The obvious junk is gone quickly.
- The duplicates are collapsed cleanly.
- The risky work is called out clearly.
- The future work is preserved without crowding the present.
- The report reads like a decision map, not a dump.
- The TUI shows the live version of that decision map.
- The ActionPlan gives a swarm multiple safe work queues, not one top-20 list.
- The executor can explain every action before and after it acts.

## Relationship to other documents
- **ROADMAP.md** defines what gets built and when.
- **ARCHITECTURE.md** defines the system shape and data flow.
- **VERSION2.0.md** defines the v2.0 action-engine execution plan.
- **This document** defines the rules the system must follow.
- **CHANGELOG.md** records what actually shipped in each version.
- If any document conflicts with this one on bucket definitions, action lanes, policy profiles, layer ordering, or non-negotiables, this document wins.
