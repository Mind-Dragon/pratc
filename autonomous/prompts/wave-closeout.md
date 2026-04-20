# Wave Closeout Prompt

You are reviewing a completed autonomous wave.

Required checks:
- build passes
- tests pass
- changed gaps are reflected in GAP_LIST.md
- STATE.yaml moved to the next truthful checkpoint
- TODO.md only changed where durable backlog truth changed
- no success claim exceeds audit evidence

Return a short closeout note with:
- completed gaps
- still-open gaps
- blockers
- whether the controller should advance or halt
