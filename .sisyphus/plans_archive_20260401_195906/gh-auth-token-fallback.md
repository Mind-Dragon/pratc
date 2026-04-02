# Add Automatic gh auth Token Fallback

## TL;DR
> Add automatic GitHub token retrieval from `gh auth token` command when env vars (GITHUB_PAT, GH_TOKEN) are not set. This improves UX by removing manual token export requirement.

## Work Objectives

### Core Objective
Automatically fetch GitHub token from `gh auth token` CLI when environment variables aren't set.

### Concrete Deliverables
- `internal/app/service.go` — Add `fetchTokenFromGHCLI()` helper function
- Token priority: GITHUB_PAT → GH_TOKEN → `gh auth token` → error
- Update error message to mention gh CLI auto-detection

### Definition of Done
- [ ] `./bin/pratc analyze --repo=owner/repo` works without exporting GH_TOKEN (if gh CLI is logged in)
- [ ] `go build ./...` passes
- [ ] Existing tests continue to pass
- [ ] Error message updated for clarity

## TODOs

- [x] 1. Add fetchTokenFromGHCLI() helper in internal/app/service.go

  **What to do**:
  - Add new function near top of file (after imports, before NewScalabilityAnalyzer):
  ```go
  // fetchTokenFromGHCLI attempts to get GitHub token from gh CLI
  func fetchTokenFromGHCLI() string {
      // Check if gh command exists
      path, err := exec.LookPath("gh")
      if err != nil {
          return ""
      }
      // Run gh auth token
      cmd := exec.Command(path, "auth", "token")
      output, err := cmd.Output()
      if err != nil {
          return ""
      }
      return strings.TrimSpace(string(output))
  }
  ```
  - Add `import "os/exec"` if not already present
  - Update token retrieval (lines 79-85) to call this function as third fallback
  - Update error message (line 557) to mention gh CLI is auto-detected

  **Must NOT do**:
  - Don't break existing env var priority
  - Don't add external dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Reason**: Simple helper function addition

  **Acceptance Criteria**:
  - [ ] gh CLI detected and token fetched when env vars missing
  - [ ] Empty string returned if gh not installed or not logged in
  - [ ] go build ./... → PASS

  **QA Scenarios**:
  ```
  Scenario: Token auto-fetched from gh CLI
    Tool: Bash
    Preconditions: gh CLI installed and logged in, no GH_TOKEN exported
    Steps:
      1. unset GH_TOKEN GITHUB_PAT GITHUB_TOKEN
      2. ./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts.total_prs'
    Expected Result: Returns 42 (auth works via gh CLI)
    Evidence: .sisyphus/evidence/gh-auth/task-1-auto-fetch.txt

  Scenario: Graceful fallback when gh not available
    Tool: Bash
    Preconditions: gh CLI not in PATH
    Steps:
      1. export PATH=/usr/bin:/bin (exclude gh location)
      2. ./bin/pratc analyze --repo=opencode-ai/opencode --format=json 2>&1 | head -3
    Expected Result: Error message mentions gh CLI requirement
    Evidence: .sisyphus/evidence/gh-auth/task-1-graceful-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(app): auto-fetch GitHub token from gh CLI when env vars not set`
  - Files: `internal/app/service.go`

---

## Success Criteria

### Verification Commands
```bash
# Test without env vars (gh CLI should be used)
unset GH_TOKEN GITHUB_PAT GITHUB_TOKEN
./bin/pratc analyze --repo=opencode-ai/opencode --format=json | jq '.counts.total_prs'
# Expected: 42

# Verify build
go build ./...

# Run tests
go test ./internal/app/...
```

### Final Checklist
- [ ] Token auto-fetched from gh CLI
- [ ] Env var priority preserved (GITHUB_PAT → GH_TOKEN → gh CLI)
- [ ] Graceful fallback when gh not available
- [ ] Error messages updated
- [ ] All tests pass
