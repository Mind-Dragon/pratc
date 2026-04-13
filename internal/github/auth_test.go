package github

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveTokenUsesEnvToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")

	token, err := ResolveToken(context.Background())
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "env-token" {
		t.Fatalf("ResolveToken() token = %q, want env-token", token)
	}
}

func TestResolveTokenUsesGHCLIWhenEnvMissing(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")

	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	contents := "#!/bin/sh\nif [ \"$1\" = auth ] && [ \"$2\" = token ]; then\n  printf '%s' gh-cli-token\n  exit 0\nfi\nexit 1\n"
	if runtime.GOOS == "windows" {
		script += ".cmd"
		contents = "@echo off\r\nif \"%1\"==\"auth\" if \"%2\"==\"token\" echo gh-cli-token\r\n"
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	token, err := ResolveToken(context.Background())
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "gh-cli-token" {
		t.Fatalf("ResolveToken() token = %q, want gh-cli-token", token)
	}
}

func TestResolveTokenErrorsWithoutAuth(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("PATH", t.TempDir())

	if _, err := ResolveToken(context.Background()); err == nil {
		t.Fatal("ResolveToken() error = nil, want auth error")
	}
}
