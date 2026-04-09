# Roadmap

## Near Term

- Finish the evidence-backed review pipeline so PRs can be ranked with analyzer findings, blockers, confidence, and next actions.
- Strengthen merged/open duplicate detection and supersession handling for large backlogs.
- Improve operator-facing review output across CLI, API, and dashboard surfaces.

## Upcoming

- Expand evidence enrichment beyond metadata so high-risk PRs can include diff, subsystem, dependency, and test evidence.
- Improve review bucket synthesis for `merge now`, `focused review`, `duplicate/superseded`, `problematic`, and `escalate` outcomes.
- Keep analyzer execution advisory-only, deterministic, timeout-bounded, and read-only.

## Guardrails

- No auto-merge or auto-approve behavior.
- No GitHub App, OAuth, or webhook expansion for this track.
- No claims of high-confidence merge safety from weak metadata-only signals.
