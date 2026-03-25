package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/spf13/cobra"
)

func TestWriteMirrorListIncludesSizeAndLastSync(t *testing.T) {
	tempDir := t.TempDir()
	baseDir := filepath.Join(tempDir, "repos")
	repoPath := filepath.Join(baseDir, "owner", "repo.git")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir mirror: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "objects.bin"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("seed mirror file: %v", err)
	}

	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	wantSync := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := store.SetLastSync("owner/repo", wantSync); err != nil {
		_ = store.Close()
		t.Fatalf("seed last sync: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := writeMirrorList(cmd, baseDir); err != nil {
		t.Fatalf("writeMirrorList: %v", err)
	}

	wantLine := fmt.Sprintf("owner/repo\t%s\t3\t%s", repoPath, wantSync.UTC().Format(time.RFC3339))
	output := out.String()
	for _, want := range []string{"REPO\tPATH\tSIZE\tLAST_SYNC", wantLine} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
}

func TestWriteMirrorInfoIncludesDiskUsage(t *testing.T) {
	tempDir := t.TempDir()
	baseDir := filepath.Join(tempDir, "repos")
	repoPath := filepath.Join(baseDir, "owner", "repo.git")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir mirror: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "objects.bin"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("seed mirror file: %v", err)
	}

	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	wantSync := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := store.SetLastSync("owner/repo", wantSync); err != nil {
		_ = store.Close()
		t.Fatalf("seed last sync: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := writeMirrorInfo(cmd, baseDir, "owner/repo"); err != nil {
		t.Fatalf("writeMirrorInfo: %v", err)
	}

	output := out.String()
	for _, want := range []string{
		"Repository: owner/repo",
		fmt.Sprintf("Path: %s", repoPath),
		"Exists: true",
		"Disk usage: 3 bytes",
		"Last sync: 2024-01-02T03:04:05Z",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
	if !strings.Contains(output, "Last modified:") {
		t.Fatalf("expected last modified line, got %q", output)
	}
}
