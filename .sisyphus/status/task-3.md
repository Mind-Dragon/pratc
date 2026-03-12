task id: 3
title: GitHub API client + SQLite cache (TDD)
status: in_progress
owner: Builder 2
branch/worktree: current branch
dependencies:
  - 1
changed files:
  - internal/github/
  - internal/cache/
evidence paths:
  - .sisyphus/evidence/task-3-cache-tests.txt
  - .sisyphus/evidence/task-3-rate-limit-tests.txt
tests run:
  - pending (RED phase starting)
merge commit:
residual risks:
  - Must use httptest mock server for tests (no real API calls)
  - Must follow rate-limit policy per AGENTS.md

notes:
  - Critical path: T0 → T1 → T3 → T13 → T14 → T18 → T19 → T20
  - Task 3 is the foundational block for CLI commands
  - Started implementation planning on 2026-03-12 after coordinator assignment confirmation
