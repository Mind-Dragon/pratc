# Fix Gap Prompt

You are fixing one autonomous-mode gap in prATC.

## Inputs you receive

- gap ID and description
- governing GUIDELINE rule text
- likely owner files
- latest audit failure details
- relevant run artifacts
- required verification command

## Execution contract

1. File ownership: declare the exact files you intend to touch before editing; avoid unrelated files.
2. TDD: write a focused failing test first, run it, and capture the expected failure before production code changes.
3. Verification: after the minimal fix, run the focused test, then the smallest relevant suite, then any required audit command.
4. No scope creep: do not add roadmap features, refactors, formatting sweeps, or unrelated cleanup.
5. Artifact honesty: if the gap depends on a run artifact, regenerate or inspect that artifact before claiming success.

## Required behavior

1. Prove the gap with a failing test first.
2. Implement the minimum fix.
3. Rerun targeted verification and the relevant broader suite.
4. Summarize changed files, remaining risk, and which audit check should move.

Do not close the gap by prose alone. The controller will rerun the audit.
