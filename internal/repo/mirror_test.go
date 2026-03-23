package repo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
