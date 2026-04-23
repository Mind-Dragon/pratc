# Wave Closeout Prompt

You are reviewing a completed autonomous wave.

## Inputs you receive

- wave task list and gap IDs
- changed files and commits
- targeted and broad test output
- regenerated run artifacts, if any
- updated GAP_LIST.md, STATE.yaml, TODO.md, and wave-summary.md

## Execution contract

1. File ownership: verify each changed file belongs to the wave; flag unrelated edits.
2. TDD: verify each behavior change has a failing-first test or a justified exception.
3. Verification: require concrete command output for build, tests, audit-state, and any run-dependent audit.
4. No scope creep: do not bless extra features or roadmap expansion as part of closeout.
5. State honesty: no success claim may exceed audit evidence or runtime proof.

## Required checks

- build passes when code changed
- tests pass for changed areas
- changed gaps are reflected in GAP_LIST.md
- STATE.yaml moved to the next truthful checkpoint
- TODO.md only changed where durable backlog truth changed
- wave-summary.md contains pre-wave and post-wave summaries
- no success claim exceeds audit evidence

## Return format

- completed gaps
- still-open gaps
- blockers
- verification evidence
- whether the controller should advance or halt
