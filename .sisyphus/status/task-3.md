task id: 3
title: GitHub API client + SQLite cache (TDD)
status: code_complete
owner: Builder 2
branch/worktree: current branch
dependencies:
  - 1
changed files:
  - internal/cache/models.go
  - internal/cache/sqlite.go
  - internal/cache/sqlite_test.go
  - internal/github/client.go
  - internal/github/queries.go
  - internal/github/client_test.go
  - go.mod
  - go.sum
evidence paths:
  - .sisyphus/evidence/task-3-cache-tests.txt
  - .sisyphus/evidence/task-3-rate-limit-tests.txt
tests run:
  - go test -race -v ./internal/cache/...
  - go test -race -v ./internal/github/...
merge commit:
residual risks:
  - Not yet merged by coordinator; full mainline verification still pending
  - GraphQL query uses client-side updated-at filtering until server-side search integration is added

notes:
  - Critical path: T0 → T1 → T3 → T13 → T14 → T18 → T19 → T20
  - Task 3 is the foundational block for CLI commands
  - Started implementation planning on 2026-03-12 after coordinator assignment confirmation
  - Completed implementation and package-level validation on 2026-03-12
