package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jeffersonnunn/pratc/internal/cache"
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
	if cacheDir := os.Getenv("PRATC_CACHE_DIR"); cacheDir != "" {
		return filepath.Join(cacheDir, "repos"), nil
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
