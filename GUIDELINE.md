# prATC Guideline

## Purpose

This document sets the operating rules for the v1.6.0 full-corpus triage engine.

The system does not get to ignore PRs. It does not get to hide uncertainty. It does not get to collapse the world into a single score and pretend that is enough.

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

8. The product surface is CLI + API + PDF.
   - CLI exists for humans operating the system.
   - API exists for AI systems consuming structured outputs.
   - PDF exists as the final human-facing report artifact.
   - Browser/dashboard surfaces are not part of the v1.6 product contract.

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

## Confidence

The system uses confidence scores to express how much it trusts its own judgment on a PR.

- Confidence is a float from 0.0 to 1.0.
- Scores below 0.5 mean the system is guessing.
- The existing `HighRiskConfidenceCap = 0.79` convention applies: high-risk findings should not claim confidence above 0.79 without strong evidence.
- Low-confidence bucket placements must be flagged for human review.
- The system must never present a low-confidence judgment as certainty.

## Non-negotiables
- No auto-merge.
- No silent exclusion.
- No opaque ranking without reasons.
- No pretending a truncated view is the whole corpus.
- No changing the meaning of a bucket without updating this guideline.
- No claiming certainty when confidence is low.

## What good looks like
- The full corpus is visible.
- The obvious junk is gone quickly.
- The duplicates are collapsed cleanly.
- The risky work is called out clearly.
- The future work is preserved without crowding the present.
- The report reads like a decision map, not a dump.

## Relationship to other documents
- **ROADMAP.md** defines what gets built and when.
- **ARCHITECTURE.md** defines the system shape and data flow.
- **This document** defines the rules the system must follow.
- **CHANGELOG.md** records what actually shipped in each version.
- If any document conflicts with this one on bucket definitions, layer ordering, or non-negotiables, this document wins.
