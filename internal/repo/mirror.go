package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
)

var repoPartPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Mirror struct {
	gitDir    string
	repo      string
	runner    commandRunner
	cache     *cache.Store
	BatchSize int
}

func WithBatchSize(size int) func(*Mirror) {
	return func(m *Mirror) {
		if size > 0 {
			m.BatchSize = size
		}
	}
}

type commandRunner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type gitRunner struct {
	gitDir string
}

func (g gitRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{"--git-dir", g.gitDir}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func NewMirror(gitDir string) *Mirror {
	return &Mirror{
		gitDir: gitDir,
		repo:   repoFromGitDir(gitDir),
		runner: gitRunner{gitDir: gitDir},
	}
}

func NewMirrorWithCache(gitDir string, cacheStore *cache.Store) *Mirror {
	m := NewMirror(gitDir)
	m.cache = cacheStore
	return m
}

func (m *Mirror) SetPRFilesCache(cacheStore *cache.Store) {
	m.cache = cacheStore
}

func DefaultBaseDir() (string, error) {
	// PRATC_BASE_DIR takes absolute precedence
	if baseDir := os.Getenv("PRATC_BASE_DIR"); baseDir != "" {
		return filepath.Join(baseDir, "repos"), nil
	}
	if cacheDir := os.Getenv("PRATC_CACHE_DIR"); cacheDir != "" {
		return filepath.Join(cacheDir, "repos"), nil
	}
	if secondaryDir := os.Getenv("PRATC_SECONDARY_CACHE_DIR"); secondaryDir != "" {
		return filepath.Join(secondaryDir, "repos"), nil
	}
	// Build mnt paths from configurable prefix
	mntPrefix := os.Getenv("PRATC_MNT_PATH_PREFIX")
	if mntPrefix == "" {
		mntPrefix = "/mnt/clawdata"
	}
	for _, idx := range []string{"2", "1"} {
		candidate := filepath.Join(mntPrefix+idx, "pratc")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Join(candidate, "repos"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "pratc", "repos"), nil
}

func MirrorPath(baseDir, repo string) (string, error) {
	owner, name, err := parseRepo(repo)
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, owner, name+".git"), nil
}

func LegacyMirrorBaseDir(root string) string {
	return filepath.Join(root, ".pratc", "repos")
}

func LegacyMirrorPath(root, repo string) (string, error) {
	owner, name, err := parseRepo(repo)
	if err != nil {
		return "", err
	}
	return filepath.Join(LegacyMirrorBaseDir(root), owner, name+".git"), nil
}

type LegacyMirrorMigrationPlan struct {
	Source        string
	Destination   string
	ShouldMigrate bool
}

func PlanLegacyMirrorMigration(root, newBaseDir, repo string) (*LegacyMirrorMigrationPlan, error) {
	source, err := LegacyMirrorPath(root, repo)
	if err != nil {
		return nil, err
	}
	destination, err := MirrorPath(newBaseDir, repo)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return &LegacyMirrorMigrationPlan{Source: source, Destination: destination}, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat legacy mirror: %w", err)
	}

	if _, err := os.Stat(destination); err == nil {
		return nil, fmt.Errorf("destination mirror already exists: %s", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat destination mirror: %w", err)
	}

	return &LegacyMirrorMigrationPlan{Source: source, Destination: destination, ShouldMigrate: true}, nil
}

func MigrateLegacyMirror(root, newBaseDir, repo string) error {
	plan, err := PlanLegacyMirrorMigration(root, newBaseDir, repo)
	if err != nil {
		return err
	}
	if !plan.ShouldMigrate {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(plan.Destination), 0o755); err != nil {
		return fmt.Errorf("create destination parent: %w", err)
	}
	if err := os.Rename(plan.Source, plan.Destination); err != nil {
		return fmt.Errorf("move legacy mirror: %w", err)
	}
	return nil
}

func OpenOrCreate(ctx context.Context, baseDir, repo, remoteURL string, opts ...func(*Mirror)) (*Mirror, error) {
	if strings.TrimSpace(remoteURL) == "" {
		return nil, errors.New("remote URL is required")
	}
	gitDir, err := MirrorPath(baseDir, repo)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(gitDir), 0o755); err != nil {
		return nil, fmt.Errorf("create parent directory: %w", err)
	}

	if _, err := os.Stat(gitDir); errors.Is(err, os.ErrNotExist) {
		cmd := exec.CommandContext(ctx, "git", "init", "--bare", gitDir)
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return nil, fmt.Errorf("initialize bare repo: %w: %s", runErr, strings.TrimSpace(string(out)))
		}
	} else if err != nil {
		return nil, fmt.Errorf("stat mirror path: %w", err)
	}

	m := &Mirror{gitDir: gitDir, repo: repo, runner: gitRunner{gitDir: gitDir}, BatchSize: 100}
	for _, opt := range opts {
		opt(m)
	}
	if _, err := m.runner.Run(ctx, "remote", "remove", "origin"); err != nil {
		_ = err
	}
	if _, err := m.runner.Run(ctx, "remote", "add", "origin", remoteURL); err != nil {
		return nil, err
	}
	return m, nil
}

func BuildRefspecs(openPRs []int) []string {
	refspecs := []string{"refs/heads/main:refs/heads/main"}
	for _, number := range openPRs {
		refspecs = append(refspecs, fmt.Sprintf("refs/pull/%d/head:refs/pr/%d/head", number, number))
	}
	return refspecs
}

func (m *Mirror) FetchAll(ctx context.Context, openPRs []int, progress func(done, total int)) error {
	batchSize := m.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	return m.FetchAllBatched(ctx, openPRs, batchSize, progress)
}

func (m *Mirror) FetchAllWithSkipped(ctx context.Context, openPRs []int, progress func(done, total int)) ([]int, error) {
	total := len(openPRs)
	skipped := make([]int, 0)
	for i, number := range openPRs {
		refspec := fmt.Sprintf("refs/pull/%d/head:refs/pr/%d/head", number, number)
		if _, err := m.runner.Run(ctx, "fetch", "--prune", "--filter=blob:none", "origin", refspec); err != nil {
			skipped = append(skipped, number)
		}
		if progress != nil {
			progress(i+1, total)
		}
	}
	return skipped, nil
}

func (m *Mirror) FetchAllBatched(ctx context.Context, openPRs []int, maxRefsPerFetch int, progress func(done, total int)) error {
	refspecs := BuildRefspecs(openPRs)
	total := len(refspecs)

	for i := 0; i < len(refspecs); i += maxRefsPerFetch {
		end := i + maxRefsPerFetch
		if end > len(refspecs) {
			end = len(refspecs)
		}
		batch := refspecs[i:end]

		args := []string{"fetch", "--prune", "--filter=blob:none", "origin"}
		for _, refspec := range batch {
			args = append(args, refspec)
		}

		if _, err := m.runner.Run(ctx, args...); err != nil {
			return err
		}

		if progress != nil {
			progress(end, total)
		}
	}
	return nil
}

func (m *Mirror) PruneClosedPRs(ctx context.Context, closedPRs []int) error {
	for _, number := range closedPRs {
		if _, err := m.runner.Run(ctx, "update-ref", "-d", fmt.Sprintf("refs/pr/%d/head", number)); err != nil {
			if strings.Contains(err.Error(), "cannot lock ref") || strings.Contains(err.Error(), "not a valid ref") {
				continue
			}
			return err
		}
	}
	return nil
}

func (m *Mirror) RefSHA(ctx context.Context, ref string) (string, error) {
	out, err := m.runner.Run(ctx, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (m *Mirror) Drift(ctx context.Context, remoteByPR map[int]string) (map[int]string, error) {
	drift := map[int]string{}
	for number, remoteSHA := range remoteByPR {
		localSHA, err := m.RefSHA(ctx, fmt.Sprintf("refs/pr/%d/head", number))
		if err != nil {
			drift[number] = "missing"
			continue
		}
		if localSHA != strings.TrimSpace(remoteSHA) {
			drift[number] = localSHA
		}
	}
	return drift, nil
}

func (m *Mirror) GetChangedFiles(ctx context.Context, prNumber int, baseBranch string) ([]string, error) {
	if m.cache != nil && m.repo != "" {
		if files, found, err := m.cache.GetPRFiles(m.repo, prNumber); err == nil && found {
			return files, nil
		}
	}

	if baseBranch == "" {
		baseBranch = "main"
	}
	prRef := fmt.Sprintf("refs/pull/%d/head", prNumber)
	baseRef := fmt.Sprintf("refs/heads/%s", baseBranch)

	out, err := m.runner.Run(ctx, "merge-base", baseRef, prRef)
	if err != nil {
		return nil, fmt.Errorf("git merge-base: %w", err)
	}
	mergeBase := strings.TrimSpace(string(out))
	if mergeBase == "" {
		return nil, fmt.Errorf("empty merge-base for PR %d", prNumber)
	}

	diffOut, err := m.runner.Run(ctx, "diff", "--name-only", mergeBase, prRef)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(diffOut)), "\n")
	var result []string
	for _, file := range files {
		if file != "" {
			result = append(result, file)
		}
	}

	if m.cache != nil && m.repo != "" {
		_ = m.cache.UpsertPRFiles(m.repo, prNumber, result)
	}
	return result, nil
}

type PRFiles struct {
	PRNumber int
	Files    []string
	Err      error
}

// GetDiffPatch returns the full diff patch data for a PR, including file metadata
// (additions, deletions, status) and parsed diff hunks. This is used by the
// review analyzers to emit findings with concrete code/diff evidence.
func (m *Mirror) GetDiffPatch(ctx context.Context, prNumber int, baseBranch string) ([]types.PRFile, []types.DiffHunk, error) {
	if baseBranch == "" {
		baseBranch = "main"
	}
	prRef := fmt.Sprintf("refs/pull/%d/head", prNumber)
	baseRef := fmt.Sprintf("refs/heads/%s", baseBranch)

	mergeBase, err := m.runOutput(ctx, "merge-base", baseRef, prRef)
	if err != nil {
		return nil, nil, fmt.Errorf("git merge-base: %w", err)
	}

	// Get diff with patch content
	out, err := m.runner.Run(ctx, "diff", "-p", "--stat", mergeBase, prRef)
	if err != nil {
		return nil, nil, fmt.Errorf("git diff -p: %w", err)
	}

	return parseDiffOutput(string(out), prNumber)
}

// parseDiffOutput parses unified diff output into PRFile and DiffHunk slices.
// The input is the output of `git diff -p --stat`.
func parseDiffOutput(output string, prNumber int) ([]types.PRFile, []types.DiffHunk, error) {
	var files []types.PRFile
	var hunks []types.DiffHunk

	// Split into file sections by "diff --git"
	sections := strings.Split(output, "diff --git a/")
	if len(sections) < 2 {
		return files, hunks, nil
	}

	for _, section := range sections[1:] {
		lines := strings.Split(section, "\n")
		if len(lines) < 2 {
			continue
		}

		// Parse the "a/path b/path" header line
		// The split removed "diff --git a/" so we need to add it back
		headerLine := "a/" + lines[0]
		oldPath, newPath, additions, deletions := parseDiffHeader(headerLine)

		// Collect patch content (everything between the header and the next diff)
		var patchLines []string
		var currentHunk *types.DiffHunk
		var hunkLines []string

		for i := 1; i < len(lines); i++ {
			line := lines[i]

			// Skip empty lines at the start of patch
			if len(patchLines) == 0 && strings.TrimSpace(line) == "" {
				continue
			}

			// Check if we've hit the next diff section
			if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "Binary files") {
				// Save any pending hunk
				if currentHunk != nil && len(hunkLines) > 0 {
					currentHunk.Content = strings.Join(hunkLines, "\n")
					hunks = append(hunks, *currentHunk)
					currentHunk = nil
					hunkLines = nil
				}
				// This line starts the next file, so put it back for next section
				// by NOT adding it to patchLines
				break
			}

			patchLines = append(patchLines, line)

			// Parse hunk headers
			if strings.HasPrefix(line, "@@") {
				// Save previous hunk if exists
				if currentHunk != nil && len(hunkLines) > 0 {
					currentHunk.Content = strings.Join(hunkLines, "\n")
					hunks = append(hunks, *currentHunk)
				}

				// Parse new hunk header
				hdr := parseHunkHeader(line)
				if hdr != nil {
					currentHunk = &types.DiffHunk{
						OldPath:  oldPath,
						NewPath:  newPath,
						OldStart: hdr.OldStart,
						OldLines: hdr.OldLines,
						NewStart: hdr.NewStart,
						NewLines: hdr.NewLines,
						Section:  hdr.Section,
					}
					hunkLines = []string{line}
				}
			} else if currentHunk != nil {
				hunkLines = append(hunkLines, line)
			}
		}

		// Save last hunk of this file
		if currentHunk != nil && len(hunkLines) > 0 {
			currentHunk.Content = strings.Join(hunkLines, "\n")
			hunks = append(hunks, *currentHunk)
		}

		// Determine status from patch lines
		status := "modified"
		for _, pl := range patchLines {
			if strings.HasPrefix(pl, "new file") {
				status = "added"
				break
			}
			if strings.HasPrefix(pl, "deleted file") {
				status = "removed"
				break
			}
		}

		files = append(files, types.PRFile{
			Path:      newPath,
			Status:    status,
			Additions: additions,
			Deletions: deletions,
			Patch:     strings.Join(patchLines, "\n"),
		})
	}

	return files, hunks, nil
}

// diffHeader holds parsed header information from a unified diff.
type diffHeader struct {
	OldPath  string
	NewPath  string
	Additions int
	Deletions int
}

// parseDiffHeader parses a "a/path b/path" diff header line and stat info.
// Example: "a/internal/auth.go b/internal/auth.go (new)"
func parseDiffHeader(headerLine string) (oldPath, newPath string, additions, deletions int) {
	// Format: "a/path b/path" or "a/path b/path (new)" or "a/path b/path (deleted)"
	parts := strings.Fields(headerLine)
	if len(parts) < 2 {
		return "", "", 0, 0
	}

	oldPath = parts[0]
	newPath = parts[1]

	// Try to extract additions/deletions from trailing stat
	// Example: "... (new), 42 insertions(+), 0 deletions(-)"
	addRe := regexp.MustCompile(`(\d+) insertion`)
	delRe := regexp.MustCompile(`(\d+) deletion`)
	if matches := addRe.FindStringSubmatch(headerLine); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			additions = n
		}
	}
	if matches := delRe.FindStringSubmatch(headerLine); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			deletions = n
		}
	}

	return oldPath, newPath, additions, deletions
}

// hunkHeader holds parsed hunk header information.
type hunkHeader struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Section  string
}

// parseHunkHeader parses a hunk header line.
// Format: "@@ -oldStart,oldLines +newStart,newLines @@ optional section header"
func parseHunkHeader(line string) *hunkHeader {
	// Format: "@@ -10,5 +20,7 @@ func Foo()"
	re := regexp.MustCompile(`@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@ ?(.*)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 4 {
		// Try simpler format without line counts
		re2 := regexp.MustCompile(`@@ -(\d+) \+(\d+) @@ ?(.*)`)
		matches2 := re2.FindStringSubmatch(line)
		if len(matches2) < 3 {
			return nil
		}
		oldStart, _ := strconv.Atoi(matches2[1])
		newStart, _ := strconv.Atoi(matches2[2])
		section := strings.TrimSpace(matches2[3])
		return &hunkHeader{
			OldStart: oldStart,
			OldLines: 1,
			NewStart: newStart,
			NewLines: 1,
			Section:  section,
		}
	}

	oldStart, _ := strconv.Atoi(matches[1])
	oldLines := 1
	if matches[2] != "" {
		if n, err := strconv.Atoi(matches[2]); err == nil {
			oldLines = n
		}
	}
	newStart, _ := strconv.Atoi(matches[3])
	newLines := 1
	if matches[4] != "" {
		if n, err := strconv.Atoi(matches[4]); err == nil {
			newLines = n
		}
	}
	section := ""
	if len(matches) > 5 {
		section = strings.TrimSpace(matches[5])
	}

	return &hunkHeader{
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
		Section:  section,
	}
}

func (m *Mirror) GetChangedFilesBatch(ctx context.Context, prNumbers []int, baseBranch string, workers int) ([]PRFiles, error) {
	if workers <= 0 {
		workers = 10
	}

	jobs := make(chan int, len(prNumbers))
	results := make(chan PRFiles, len(prNumbers))

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for prNumber := range jobs {
				files, err := m.GetChangedFiles(ctx, prNumber, baseBranch)
				results <- PRFiles{PRNumber: prNumber, Files: files, Err: err}
			}
		}()
	}

	for _, n := range prNumbers {
		jobs <- n
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var prFilesList []PRFiles
	for result := range results {
		prFilesList = append(prFilesList, result)
	}
	return prFilesList, nil
}

func parseRepo(repo string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(repo), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/repo", repo)
	}
	owner := parts[0]
	name := parts[1]
	if owner == "" || name == "" {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/repo", repo)
	}
	if !repoPartPattern.MatchString(owner) || !repoPartPattern.MatchString(name) {
		return "", "", fmt.Errorf("invalid repo %q, owner/repo contains unsupported characters", repo)
	}
	if strings.Contains(owner, "..") || strings.Contains(name, "..") {
		return "", "", fmt.Errorf("invalid repo %q, path traversal detected", repo)
	}
	return owner, name, nil
}

func repoFromGitDir(gitDir string) string {
	name := filepath.Base(filepath.Clean(gitDir))
	if !strings.HasSuffix(name, ".git") {
		return ""
	}
	parent := filepath.Base(filepath.Dir(filepath.Clean(gitDir)))
	owner := strings.TrimSpace(parent)
	repo := strings.TrimSuffix(name, ".git")
	if owner == "" || repo == "" {
		return ""
	}
	if !repoPartPattern.MatchString(owner) || !repoPartPattern.MatchString(repo) {
		return ""
	}
	return owner + "/" + repo
}

// DiffStats holds the additions and deletions counts for a PR.
type DiffStats struct {
	PRNumber  int
	Additions int
	Deletions int
	Files     int
}

// GetDiffStats returns addition/deletion/file counts for a PR using git diff --stat.
func (m *Mirror) GetDiffStats(ctx context.Context, prNumber int, baseBranch string) (*DiffStats, error) {
	if baseBranch == "" {
		baseBranch = "main"
	}
	prRef := fmt.Sprintf("refs/pull/%d/head", prNumber)
	baseRef := fmt.Sprintf("refs/heads/%s", baseBranch)

	mergeBase, err := m.runOutput(ctx, "merge-base", baseRef, prRef)
	if err != nil {
		return nil, fmt.Errorf("git merge-base: %w", err)
	}
	mergeBase = strings.TrimSpace(mergeBase)

	// git diff --stat for additions/deletions summary
	out, err := m.runner.Run(ctx, "diff", "--stat", mergeBase, prRef)
	if err != nil {
		return nil, fmt.Errorf("git diff --stat: %w", err)
	}

	stats := parseDiffStat(strings.TrimSpace(string(out)), prNumber)
	return stats, nil
}

// GetDiffStatsBatch returns diff stats for multiple PRs concurrently.
func (m *Mirror) GetDiffStatsBatch(ctx context.Context, prNumbers []int, baseBranch string, workers int) ([]*DiffStats, error) {
	if workers <= 0 {
		workers = 10
	}

	results := make([]*DiffStats, len(prNumbers))
	errors := make([]error, len(prNumbers))
	var wg sync.WaitGroup

	for i, prNumber := range prNumbers {
		wg.Add(1)
		go func(idx int, num int) {
			defer wg.Done()
			stats, err := m.GetDiffStats(ctx, num, baseBranch)
			results[idx] = stats
			errors[idx] = err
		}(i, prNumber)
	}
	wg.Wait()

	// Return first error if any
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

// CommitInfo holds commit metadata from git log.
type CommitInfo struct {
	SHA     string
	Author  string
	Date    string
	Subject string
	Body    string
}

// GetCommitLog returns commit history for a PR using git log.
func (m *Mirror) GetCommitLog(ctx context.Context, prNumber int, limit int) ([]CommitInfo, error) {
	if limit <= 0 {
		limit = 50
	}
	prRef := fmt.Sprintf("refs/pull/%d/head", prNumber)

	// Use git log with custom format to get commit info
	format := "%H%n%an%n%ad%n%s%n%b<COMMIT_DELIM>"
	args := []string{"log", "--format=" + format, "--date=short", "-n", fmt.Sprintf("%d", limit), prRef}

	out, err := m.runner.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	return parseCommitLog(strings.TrimSpace(string(out))), nil
}

// runOutput is a helper to run git and return trimmed output.
func (m *Mirror) runOutput(ctx context.Context, args ...string) (string, error) {
	out, err := m.runner.Run(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// parseDiffStat parses git diff --stat output.
// Example: " 0 files changed, 100 insertions(+), 50 deletions(-)"
func parseDiffStat(output string, prNumber int) *DiffStats {
	stats := &DiffStats{PRNumber: prNumber}

	// Parse insertion/deletion counts
	addRe := regexp.MustCompile(`(\d+) insertion`)
	delRe := regexp.MustCompile(`(\d+) deletion`)
	fileRe := regexp.MustCompile(`(\d+) file`)

	if matches := addRe.FindStringSubmatch(output); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			stats.Additions = n
		}
	}
	if matches := delRe.FindStringSubmatch(output); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			stats.Deletions = n
		}
	}
	if matches := fileRe.FindStringSubmatch(output); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			stats.Files = n
		}
	}

	return stats
}

// parseCommitLog parses git log output with custom delimiter.
func parseCommitLog(output string) []CommitInfo {
	const delim = "<COMMIT_DELIM>"
	parts := strings.Split(output, delim)
	var commits []CommitInfo

	for _, part := range parts {
		lines := strings.Split(strings.TrimSpace(part), "\n")
		if len(lines) < 4 {
			continue
		}
		commits = append(commits, CommitInfo{
			SHA:     lines[0],
			Author:  lines[1],
			Date:    lines[2],
			Subject: lines[3],
			Body:    strings.Join(lines[4:], "\n"),
		})
	}
	return commits
}
