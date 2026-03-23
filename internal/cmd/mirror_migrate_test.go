package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunMirrorMigrate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (workspace string, baseDir string)
		wantOut string
		wantErr string
	}{
		{
			name: "migrated",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				workspace := t.TempDir()
				baseDir := filepath.Join(workspace, "cache", "repos")
				legacyPath, err := filepath.Abs(filepath.Join(workspace, ".pratc", "repos", "octo", "repo.git"))
				if err != nil {
					t.Fatalf("legacy abs path: %v", err)
				}
				if err := os.MkdirAll(legacyPath, 0o755); err != nil {
					t.Fatalf("create legacy mirror: %v", err)
				}
				if err := os.WriteFile(filepath.Join(legacyPath, "marker"), []byte("legacy"), 0o644); err != nil {
					t.Fatalf("write marker: %v", err)
				}
				return workspace, baseDir
			},
			wantOut: "Migrated legacy mirror for octo/repo",
		},
		{
			name: "no legacy mirror",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				workspace := t.TempDir()
				baseDir := filepath.Join(workspace, "cache", "repos")
				return workspace, baseDir
			},
			wantOut: "No legacy mirror found for octo/repo",
		},
		{
			name: "destination conflict",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				workspace := t.TempDir()
				baseDir := filepath.Join(workspace, "cache", "repos")
				legacyPath, err := filepath.Abs(filepath.Join(workspace, ".pratc", "repos", "octo", "repo.git"))
				if err != nil {
					t.Fatalf("legacy abs path: %v", err)
				}
				destPath := filepath.Join(baseDir, "octo", "repo.git")
				if err := os.MkdirAll(legacyPath, 0o755); err != nil {
					t.Fatalf("create legacy mirror: %v", err)
				}
				if err := os.MkdirAll(destPath, 0o755); err != nil {
					t.Fatalf("create destination mirror: %v", err)
				}
				return workspace, baseDir
			},
			wantOut: "Destination mirror already exists for octo/repo",
			wantErr: "destination mirror already exists",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workspace, baseDir := tc.setup(t)
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("getwd: %v", err)
			}
			if err := os.Chdir(workspace); err != nil {
				t.Fatalf("chdir workspace: %v", err)
			}
			defer func() {
				_ = os.Chdir(oldWd)
			}()

			cmd := &cobra.Command{}
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)

			err = runMirrorMigrate(cmd, "octo/repo", baseDir)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("runMirrorMigrate: %v", err)
			}
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
			}
			if !strings.Contains(stdout.String(), tc.wantOut) {
				t.Fatalf("stdout %q does not contain %q", stdout.String(), tc.wantOut)
			}
		})
	}
}
