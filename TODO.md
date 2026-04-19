# prATC TODO — v1.5.0 Live Verification

## Goal
Verify the v1.5.0 triage engine against the full openclaw/openclaw corpus and close out the release.

## Scope
Active work only. Completed items live in CHANGELOG.md.

## Current status
- v1.5.0 code complete on main (a7e4e06). All BUG-1 through BUG-6 fixes landed.
- Full test suite green (27 packages, 0 failures).
- Overnight production run completed: `20260419-065654` (4,992 PRs, cap at 5,000).
- `hermes/security-quality-refactor` branch exists (137 files, 2371+/15701-) but is 0 commits ahead of main — already merged or rebased onto main.
- Version bumped to 1.5.0 in `internal/version/version.go`.

---

## Overnight Run Results (20260419-065654)

```
repo:           openclaw/openclaw
total PRs:      4,992 (capped at 5,000)
clusters:       69
duplicate groups: 9
overlap groups: 741
conflict pairs: 38,884
stale PRs:      215
garbage PRs:    8
plan selected:  20 / 3,071 candidates
truncation:     max_prs_cap (5,000)
precision mode: fast
```

### Observations
- Conflict pairs (38,884) still high. BUG-5 fix reduced from 92,911 but the 2+ shared-file threshold + noise file filter didn't cut deep enough for this repo.
- Duplicate groups (9) — working but low. Threshold fix (0.85) is correct; the corpus may genuinely have few duplicates, or file-overlap signaling needs tuning.
- Garbage PRs (8) — likely conservative. The classifier may need broader patterns.
- Truncation at 5,000 PRs — the repo has ~6,632 open PRs. The cap means the last ~1,640 were excluded. This is documented and explicit.

---

## Remaining for v1.5.0 close-out

### Task 1: Live verification of key metrics
- [ ] Confirm duplicate groups > 10 by running with `--max-prs=0` (no cap) or raising to 7,000
- [ ] Confirm conflict count < 5,000 after tuning — current 38,884 is still too high
- [ ] Confirm analysis completes in < 15 min on second run (intermediate cache should help)

### Task 2: Tune conflict noise filter
- [ ] Audit the top-20 most-conflicting files in the 38,884 pairs
- [ ] Add repo-specific noise patterns (this is a JS/TS monorepo — more lockfile/config noise)
- [ ] Consider raising the shared-file minimum from 2 to 3, or weighting by file depth

### Task 3: Tune garbage classifier
- [ ] Review the 8 garbage PRs — are they real garbage or false positives?
- [ ] Spot-check 20 PRs the classifier missed — what patterns should it catch?
- [ ] Expand abandoned/spam patterns if needed

### Task 4: Security refactor branch
- [ ] Decide: merge `hermes/security-quality-refactor` into main, or keep separate?
- [ ] Branch is 0 commits ahead of main — if already merged, delete the branch

### Task 5: Update docs for final state
- [x] Bump version.go to 1.5.0
- [x] Rewrite TODO.md
- [ ] Update ARCHITECTURE.md (schema v7, overnight results, version refs)
- [ ] Update GUIDELINE.md (threshold 0.85, pipeline reality)
- [ ] Update version1.4.2.md (mark shipped, point to v1.5)
- [ ] Update ROADMAP.md (v1.5 verification status)

---

## Exit criteria
- [ ] Full-corpus run (no cap) produces < 5,000 conflict pairs
- [ ] Full-corpus run completes in < 15 min on cached second pass
- [ ] Duplicate detection finds > 10 groups (or documented why the corpus has fewer)
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] All docs consistent with v1.5.0

## Post-v1.5 backlog
See ROADMAP.md v1.6+ for dashboard enhancements, evidence enrichment, multi-repo, and ML feedback.
