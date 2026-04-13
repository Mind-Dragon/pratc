package repo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

func TestLegacyMirrorPath(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	got, err := LegacyMirrorPath(workspace, "octo/repo")
	if err != nil {
		t.Fatalf("legacy mirror path: %v", err)
	}

	want := filepath.Join(workspace, ".pratc", "repos", "octo", "repo.git")
	if got != want {
		t.Fatalf("unexpected legacy path: got %q want %q", got, want)
	}
}

func TestDefaultBaseDirPrefersPrimaryCacheDir(t *testing.T) {
	primaryDir := t.TempDir()
	secondaryDir := t.TempDir()
	t.Setenv("PRATC_CACHE_DIR", primaryDir)
	t.Setenv("PRATC_SECONDARY_CACHE_DIR", secondaryDir)

	baseDir, err := DefaultBaseDir()
	if err != nil {
		t.Fatalf("default base dir: %v", err)
	}
	want := filepath.Join(primaryDir, "repos")
	if baseDir != want {
		t.Fatalf("default base dir should prefer PRATC_CACHE_DIR, got %q want %q", baseDir, want)
	}
}

func TestDefaultBaseDirUsesSecondaryCacheDirWhenPrimaryUnset(t *testing.T) {
	secondaryDir := t.TempDir()
	t.Setenv("PRATC_CACHE_DIR", "")
	t.Setenv("PRATC_SECONDARY_CACHE_DIR", secondaryDir)

	baseDir, err := DefaultBaseDir()
	if err != nil {
		t.Fatalf("default base dir: %v", err)
	}
	want := filepath.Join(secondaryDir, "repos")
	if baseDir != want {
		t.Fatalf("default base dir should prefer PRATC_SECONDARY_CACHE_DIR, got %q want %q", baseDir, want)
	}
}

func TestPlanLegacyMirrorMigration(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	newBaseDir := filepath.Join(workspace, "cache", "repos")
	oldPath, err := LegacyMirrorPath(workspace, "octo/repo")
	if err != nil {
		t.Fatalf("legacy mirror path: %v", err)
	}
	newPath, err := MirrorPath(newBaseDir, "octo/repo")
	if err != nil {
		t.Fatalf("mirror path: %v", err)
	}
	if err := os.MkdirAll(oldPath, 0o755); err != nil {
		t.Fatalf("create old mirror: %v", err)
	}

	plan, err := PlanLegacyMirrorMigration(workspace, newBaseDir, "octo/repo")
	if err != nil {
		t.Fatalf("plan migration: %v", err)
	}
	if !plan.ShouldMigrate {
		t.Fatalf("expected migration plan to require move")
	}
	if plan.Source != oldPath {
		t.Fatalf("unexpected source: got %q want %q", plan.Source, oldPath)
	}
	if plan.Destination != newPath {
		t.Fatalf("unexpected destination: got %q want %q", plan.Destination, newPath)
	}
}

func TestMigrateLegacyMirror(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(t *testing.T, workspace, newBaseDir string) (oldPath, newPath string)
		wantErr       bool
		wantOldExists bool
		wantNewExists bool
	}{
		{
			name: "success",
			setup: func(t *testing.T, workspace, newBaseDir string) (string, string) {
				t.Helper()
				oldPath, err := LegacyMirrorPath(workspace, "octo/repo")
				if err != nil {
					t.Fatalf("legacy mirror path: %v", err)
				}
				newPath, err := MirrorPath(newBaseDir, "octo/repo")
				if err != nil {
					t.Fatalf("mirror path: %v", err)
				}
				if err := os.MkdirAll(oldPath, 0o755); err != nil {
					t.Fatalf("create old mirror: %v", err)
				}
				if err := os.WriteFile(filepath.Join(oldPath, "marker"), []byte("legacy"), 0o644); err != nil {
					t.Fatalf("write legacy marker: %v", err)
				}
				return oldPath, newPath
			},
			wantOldExists: false,
			wantNewExists: true,
		},
		{
			name: "missing old path no-op",
			setup: func(t *testing.T, workspace, newBaseDir string) (string, string) {
				t.Helper()
				oldPath, err := LegacyMirrorPath(workspace, "octo/repo")
				if err != nil {
					t.Fatalf("legacy mirror path: %v", err)
				}
				newPath, err := MirrorPath(newBaseDir, "octo/repo")
				if err != nil {
					t.Fatalf("mirror path: %v", err)
				}
				return oldPath, newPath
			},
			wantOldExists: false,
			wantNewExists: false,
		},
		{
			name: "destination exists",
			setup: func(t *testing.T, workspace, newBaseDir string) (string, string) {
				t.Helper()
				oldPath, err := LegacyMirrorPath(workspace, "octo/repo")
				if err != nil {
					t.Fatalf("legacy mirror path: %v", err)
				}
				newPath, err := MirrorPath(newBaseDir, "octo/repo")
				if err != nil {
					t.Fatalf("mirror path: %v", err)
				}
				if err := os.MkdirAll(oldPath, 0o755); err != nil {
					t.Fatalf("create old mirror: %v", err)
				}
				if err := os.MkdirAll(newPath, 0o755); err != nil {
					t.Fatalf("create new mirror: %v", err)
				}
				return oldPath, newPath
			},
			wantErr:       true,
			wantOldExists: true,
			wantNewExists: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			workspace := t.TempDir()
			newBaseDir := filepath.Join(workspace, "cache", "repos")
			oldPath, newPath := tc.setup(t, workspace, newBaseDir)

			err := MigrateLegacyMirror(workspace, newBaseDir, "octo/repo")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected migration error")
				}
			} else if err != nil {
				t.Fatalf("migrate legacy mirror: %v", err)
			}

			_, oldErr := os.Stat(oldPath)
			if got := !os.IsNotExist(oldErr); got != tc.wantOldExists {
				t.Fatalf("old path exists = %v, want %v (err=%v)", got, tc.wantOldExists, oldErr)
			}

			_, newErr := os.Stat(newPath)
			if got := !os.IsNotExist(newErr); got != tc.wantNewExists {
				t.Fatalf("new path exists = %v, want %v (err=%v)", got, tc.wantNewExists, newErr)
			}
		})
	}
}

func TestMirrorPathRejectsTraversal(t *testing.T) {
	t.Parallel()
	_, err := MirrorPath(t.TempDir(), "../etc/passwd")
	if err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}
}

func TestOpenOrCreateAndFetchMain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	workspace := t.TempDir()
	remote := createRemoteWithMain(t, workspace)

	mirror, err := OpenOrCreate(ctx, filepath.Join(workspace, "mirrors"), "octo/repo", remote)
	if err != nil {
		t.Fatalf("open mirror: %v", err)
	}

	progressCalls := 0
	if err := mirror.FetchAll(ctx, nil, func(done, total int) {
		progressCalls++
	}); err != nil {
		t.Fatalf("fetch all: %v", err)
	}
	if progressCalls == 0 {
		t.Fatalf("expected progress callback to be called")
	}

	sha, err := mirror.RefSHA(ctx, "refs/heads/main")
	if err != nil {
		t.Fatalf("ref sha for main: %v", err)
	}
	if len(sha) != 40 {
		t.Fatalf("expected SHA length 40, got %q", sha)
	}
}

func TestPruneClosedPRsDeletesPRRef(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	workspace := t.TempDir()
	remote := createRemoteWithMain(t, workspace)

	mirror, err := OpenOrCreate(ctx, filepath.Join(workspace, "mirrors"), "octo/repo", remote)
	if err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	if err := mirror.FetchAll(ctx, nil, nil); err != nil {
		t.Fatalf("fetch main: %v", err)
	}

	mainSHA, err := mirror.RefSHA(ctx, "refs/heads/main")
	if err != nil {
		t.Fatalf("read main sha: %v", err)
	}
	if _, err := mirror.runner.Run(ctx, "update-ref", "refs/pr/42/head", mainSHA); err != nil {
		t.Fatalf("create PR ref: %v", err)
	}

	if err := mirror.PruneClosedPRs(ctx, []int{42}); err != nil {
		t.Fatalf("prune closed PRs: %v", err)
	}

	_, err = mirror.RefSHA(ctx, "refs/pr/42/head")
	if err == nil {
		t.Fatalf("expected deleted PR ref to be missing")
	}
}

func TestGetChangedFilesUsesCacheHitWithoutGitDiff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMirrorTestStore(t)
	const repo = "octo/repo"

	if err := store.UpsertPRFiles(repo, 7, []string{"z.go", "a.go"}); err != nil {
		t.Fatalf("seed cached pr files: %v", err)
	}

	mirror := NewMirrorWithCache(filepath.Join(t.TempDir(), "octo", "repo.git"), store)
	files, err := mirror.GetChangedFiles(ctx, 7, "main")
	if err != nil {
		t.Fatalf("get changed files from cache: %v", err)
	}

	want := []string{"a.go", "z.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("unexpected cached files: got %v want %v", files, want)
	}
}

func TestGetChangedFilesCacheHitLatency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMirrorTestStore(t)
	const repo = "octo/repo"
	const prNumber = 7
	const iterations = 2000

	if err := store.UpsertPRFiles(repo, prNumber, []string{"z.go", "a.go"}); err != nil {
		t.Fatalf("seed cached pr files: %v", err)
	}

	mirror := NewMirrorWithCache(filepath.Join(t.TempDir(), "octo", "repo.git"), store)
	mirror.runner = failingRunner{t: t}

	if _, err := mirror.GetChangedFiles(ctx, prNumber, "main"); err != nil {
		t.Fatalf("warm cache-hit lookup: %v", err)
	}

	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		files, err := mirror.GetChangedFiles(ctx, prNumber, "main")
		if err != nil {
			t.Fatalf("cache-hit lookup %d: %v", i, err)
		}
		if !reflect.DeepEqual(files, []string{"a.go", "z.go"}) {
			t.Fatalf("unexpected cached files on iteration %d: %v", i, files)
		}
		durations = append(durations, time.Since(start))
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	mean := total / time.Duration(len(durations))
	median := durations[len(durations)/2]

	t.Logf("cache-hit file-list retrieval: mean=%s median=%s iterations=%d", mean, median, iterations)

	const threshold = 100 * time.Millisecond
	if mean >= threshold {
		t.Fatalf("mean cache-hit retrieval %s exceeds %s", mean, threshold)
	}
	if median >= threshold {
		t.Fatalf("median cache-hit retrieval %s exceeds %s", median, threshold)
	}
}

func BenchmarkGetChangedFilesCacheHit(b *testing.B) {
	ctx := context.Background()
	store := newMirrorTestStore(b)
	const repo = "octo/repo"
	const prNumber = 7

	if err := store.UpsertPRFiles(repo, prNumber, []string{"z.go", "a.go"}); err != nil {
		b.Fatalf("seed cached pr files: %v", err)
	}

	mirror := NewMirrorWithCache(filepath.Join(b.TempDir(), "octo", "repo.git"), store)
	mirror.runner = failingRunner{t: b}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files, err := mirror.GetChangedFiles(ctx, prNumber, "main")
		if err != nil {
			b.Fatalf("cache-hit lookup %d: %v", i, err)
		}
		if !reflect.DeepEqual(files, []string{"a.go", "z.go"}) {
			b.Fatalf("unexpected cached files on iteration %d: %v", i, files)
		}
	}
}

func TestGetChangedFilesBatchPopulatesCacheOnMiss(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMirrorGitRepo(t)
	store := newMirrorTestStore(t)
	mirror := NewMirrorWithCache(repo.bareDir, store)

	repo.createPR(t, 1, map[string]string{"alpha.txt": "alpha v1\n"})
	repo.createPR(t, 2, map[string]string{"beta.txt": "beta v1\n"})

	results, err := mirror.GetChangedFilesBatch(ctx, []int{1, 2}, "main", 2)
	if err != nil {
		t.Fatalf("get changed files batch: %v", err)
	}

	got := map[int][]string{}
	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("batch result for pr %d: %v", result.PRNumber, result.Err)
		}
		got[result.PRNumber] = result.Files
	}

	if !reflect.DeepEqual(got[1], []string{"alpha.txt"}) {
		t.Fatalf("unexpected files for pr 1: %v", got[1])
	}
	if !reflect.DeepEqual(got[2], []string{"beta.txt"}) {
		t.Fatalf("unexpected files for pr 2: %v", got[2])
	}

	for _, tc := range []struct {
		pr   int
		want []string
	}{
		{pr: 1, want: []string{"alpha.txt"}},
		{pr: 2, want: []string{"beta.txt"}},
	} {
		files, found, err := store.GetPRFiles(repo.repo, tc.pr)
		if err != nil {
			t.Fatalf("get cached files for pr %d: %v", tc.pr, err)
		}
		if !found {
			t.Fatalf("expected cached files for pr %d", tc.pr)
		}
		if !reflect.DeepEqual(files, tc.want) {
			t.Fatalf("unexpected cached files for pr %d: got %v want %v", tc.pr, files, tc.want)
		}
	}
}

func TestGetChangedFilesBatch1000PRsCompletesWithinOneMinute(t *testing.T) {
	ctx := context.Background()
	repo := newMirrorGitRepo(t)
	openPRs := make([]int, 1000)
	for i := range openPRs {
		openPRs[i] = i + 1
	}

	repo.createPR(t, 1, map[string]string{"changed.txt": "changed\n"})
	sha := gitOutput(t, repo.sourceDir, "rev-parse", "HEAD")
	repo.seedPullRefsToSHA(t, openPRs, sha)

	store := newMirrorTestStore(t)
	mirror := NewMirrorWithCache(repo.bareDir, store)

	start := time.Now()
	results, err := mirror.GetChangedFilesBatch(ctx, openPRs, "main", 10)
	if err != nil {
		t.Fatalf("get changed files batch: %v", err)
	}
	elapsed := time.Since(start)

	if len(results) != len(openPRs) {
		t.Fatalf("unexpected result count: got %d want %d", len(results), len(openPRs))
	}
	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("batch result for pr %d: %v", result.PRNumber, result.Err)
		}
		if !reflect.DeepEqual(result.Files, []string{"changed.txt"}) {
			t.Fatalf("unexpected files for pr %d: %v", result.PRNumber, result.Files)
		}
	}

	t.Logf("1000-pr extraction: elapsed=%s results=%d workers=%d", elapsed, len(results), 10)
	if elapsed >= time.Minute {
		t.Fatalf("1000-pr extraction took %s, want < 1m", elapsed)
	}
}

func TestGetChangedFilesRefreshesAfterCacheClear(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMirrorGitRepo(t)
	store := newMirrorTestStore(t)
	mirror := NewMirrorWithCache(repo.bareDir, store)

	repo.createPR(t, 1, map[string]string{"alpha.txt": "alpha v1\n"})

	files, err := mirror.GetChangedFiles(ctx, 1, "main")
	if err != nil {
		t.Fatalf("initial changed files: %v", err)
	}
	if !reflect.DeepEqual(files, []string{"alpha.txt"}) {
		t.Fatalf("unexpected initial files: %v", files)
	}

	repo.updatePR(t, 1, map[string]string{"alpha.txt": "alpha v1\n", "beta.txt": "beta v2\n"})
	if err := store.ClearPRFiles(repo.repo, 1); err != nil {
		t.Fatalf("clear cached files: %v", err)
	}

	files, err = mirror.GetChangedFiles(ctx, 1, "main")
	if err != nil {
		t.Fatalf("refreshed changed files: %v", err)
	}
	want := []string{"alpha.txt", "beta.txt"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("unexpected refreshed files: got %v want %v", files, want)
	}

	cached, found, err := store.GetPRFiles(repo.repo, 1)
	if err != nil {
		t.Fatalf("read refreshed cache: %v", err)
	}
	if !found {
		t.Fatal("expected refreshed cache entry")
	}
	if !reflect.DeepEqual(cached, want) {
		t.Fatalf("unexpected refreshed cache contents: got %v want %v", cached, want)
	}
}

func TestFetchAllBatched1000PRsCompletesWithinTwoMinutes(t *testing.T) {
	ctx := context.Background()
	repo := newMirrorGitRepo(t)
	openPRs := make([]int, 1000)
	for i := range openPRs {
		openPRs[i] = i + 1
	}
	repo.seedPullRefs(t, openPRs)

	mirror, err := OpenOrCreate(ctx, filepath.Join(t.TempDir(), "mirrors"), repo.repo, repo.bareDir)
	if err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	runner := &countingRunner{runner: mirror.runner}
	mirror.runner = runner

	start := time.Now()
	if err := mirror.FetchAllBatched(ctx, openPRs, 100, nil); err != nil {
		t.Fatalf("fetch 1000 PR refs in batches: %v", err)
	}
	elapsed := time.Since(start)

	expectedFetches := (len(openPRs) + 100) / 100
	t.Logf("1000-pr sync: elapsed=%s fetches=%d refspecs=%d batches=%d", elapsed, runner.fetches, len(openPRs)+1, expectedFetches)

	if runner.fetches != expectedFetches {
		t.Fatalf("unexpected fetch count: got %d want %d", runner.fetches, expectedFetches)
	}
	if elapsed >= 2*time.Minute {
		t.Fatalf("1000-pr sync took %s, want < 2m", elapsed)
	}
}

func createRemoteWithMain(t *testing.T, root string) string {
	t.Helper()
	workRepo := filepath.Join(root, "source")
	bareRemote := filepath.Join(root, "remote.git")

	runGit(t, root, "init", "--initial-branch=main", "source")
	runGit(t, workRepo, "config", "user.email", "test@example.com")
	runGit(t, workRepo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(workRepo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, workRepo, "add", "README.md")
	runGit(t, workRepo, "commit", "-m", "init")
	runGit(t, root, "clone", "--bare", workRepo, bareRemote)
	return bareRemote
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func runGitWithInput(t *testing.T, dir string, stdin string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

type mirrorGitRepo struct {
	root      string
	sourceDir string
	bareDir   string
	repo      string
}

func newMirrorGitRepo(t *testing.T) *mirrorGitRepo {
	t.Helper()

	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	bareDir := filepath.Join(root, "octo", "repo.git")

	runGit(t, root, "init", "--initial-branch=main", "source")
	runGit(t, sourceDir, "config", "user.email", "test@example.com")
	runGit(t, sourceDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("write main file: %v", err)
	}
	runGit(t, sourceDir, "add", "README.md")
	runGit(t, sourceDir, "commit", "-m", "main")
	if err := os.MkdirAll(filepath.Dir(bareDir), 0o755); err != nil {
		t.Fatalf("create bare repo parent: %v", err)
	}
	runGit(t, root, "clone", "--bare", sourceDir, bareDir)
	runGit(t, sourceDir, "remote", "add", "origin", bareDir)
	runGit(t, sourceDir, "push", "origin", "main:refs/heads/main")

	return &mirrorGitRepo{
		root:      root,
		sourceDir: sourceDir,
		bareDir:   bareDir,
		repo:      "octo/repo",
	}
}

func (r *mirrorGitRepo) createPR(t *testing.T, prNumber int, files map[string]string) {
	t.Helper()

	branch := fmt.Sprintf("pr-%d", prNumber)
	runGit(t, r.sourceDir, "checkout", "-B", branch, "main")
	r.writeFiles(t, files)
	runGit(t, r.sourceDir, "add", "--all")
	runGit(t, r.sourceDir, "commit", "-m", fmt.Sprintf("pr %d create", prNumber))
	runGit(t, r.sourceDir, "push", "origin", fmt.Sprintf("HEAD:refs/pull/%d/head", prNumber))
}

func (r *mirrorGitRepo) updatePR(t *testing.T, prNumber int, files map[string]string) {
	t.Helper()

	branch := fmt.Sprintf("pr-%d", prNumber)
	runGit(t, r.sourceDir, "checkout", branch)
	r.writeFiles(t, files)
	runGit(t, r.sourceDir, "add", "--all")
	runGit(t, r.sourceDir, "commit", "-m", fmt.Sprintf("pr %d update", prNumber))
	runGit(t, r.sourceDir, "push", "origin", fmt.Sprintf("HEAD:refs/pull/%d/head", prNumber))
}

func (r *mirrorGitRepo) seedPullRefs(t *testing.T, prs []int) {
	t.Helper()

	sha := gitOutput(t, r.sourceDir, "rev-parse", "main")
	r.seedPullRefsToSHA(t, prs, sha)
}

func (r *mirrorGitRepo) seedPullRefsToSHA(t *testing.T, prs []int, sha string) {
	t.Helper()

	var stdin strings.Builder
	for _, pr := range prs {
		fmt.Fprintf(&stdin, "update refs/pull/%d/head %s\n", pr, sha)
	}
	runGitWithInput(t, r.bareDir, stdin.String(), "update-ref", "--stdin")
}

func (r *mirrorGitRepo) writeFiles(t *testing.T, files map[string]string) {
	t.Helper()

	for path, content := range files {
		fullPath := filepath.Join(r.sourceDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("create parent dirs for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func newMirrorTestStore(t testing.TB) *cache.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := cache.Open(path)
	if err != nil {
		t.Fatalf("open cache store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close cache store: %v", err)
		}
	})
	return store
}

type failingRunner struct {
	t testing.TB
}

func (r failingRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	r.t.Helper()
	r.t.Fatalf("unexpected git command on cache-hit path: %s", strings.Join(args, " "))
	return nil, nil
}

type countingRunner struct {
	runner  commandRunner
	fetches int
}

func (r *countingRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	if len(args) > 0 && args[0] == "fetch" {
		r.fetches++
	}
	return r.runner.Run(ctx, args...)
}
