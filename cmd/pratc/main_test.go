package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIHelpListsAllCommands(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	output, exitCode := runCommand(t, binary, "--help")
	if exitCode != 0 {
		t.Fatalf("pratc --help exit code = %d, want 0; output=%s", exitCode, output)
	}

	for _, command := range []string{"analyze", "cluster", "graph", "plan", "serve", "sync"} {
		if !strings.Contains(output, command) {
			t.Fatalf("help output missing command %q; output=%s", command, output)
		}
	}
}

func TestSyncWithoutRepoReturnsExitCodeTwo(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	output, exitCode := runCommand(t, binary, "sync")
	if exitCode != 2 {
		t.Fatalf("pratc sync exit code = %d, want 2; output=%s", exitCode, output)
	}

	if !strings.Contains(output, "required flag") {
		t.Fatalf("missing required flag error text; output=%s", output)
	}
}

func TestAnalyzeHelpShowsRepoFlag(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	output, exitCode := runCommand(t, binary, "analyze", "--help")
	if exitCode != 0 {
		t.Fatalf("pratc analyze --help exit code = %d, want 0; output=%s", exitCode, output)
	}

	if !strings.Contains(output, "--repo") {
		t.Fatalf("analyze help missing --repo flag; output=%s", output)
	}
}

func TestServeHelpShowsPortFlagDefault(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	output, exitCode := runCommand(t, binary, "serve", "--help")
	if exitCode != 0 {
		t.Fatalf("pratc serve --help exit code = %d, want 0; output=%s", exitCode, output)
	}

	if !strings.Contains(output, "--port") {
		t.Fatalf("serve help missing --port flag; output=%s", output)
	}
	if !strings.Contains(output, "default 7400") {
		t.Fatalf("serve help missing default 7400 text; output=%s", output)
	}
}

func TestAnalyzeWithoutRepoReturnsExitCodeTwo(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	output, exitCode := runCommand(t, binary, "analyze")
	if exitCode != 2 {
		t.Fatalf("pratc analyze exit code = %d, want 2; output=%s", exitCode, output)
	}

	if !strings.Contains(output, "required flag") {
		t.Fatalf("missing required flag error text; output=%s", output)
	}
}

func TestPlanInvalidTargetReturnsExitCodeTwo(t *testing.T) {
	t.Parallel()

	binary := buildBinary(t)
	output, exitCode := runCommand(t, binary, "plan", "--repo=owner/repo", "--target=abc")
	if exitCode != 2 {
		t.Fatalf("pratc plan --target=abc exit code = %d, want 2; output=%s", exitCode, output)
	}

	if !strings.Contains(output, "invalid") {
		t.Fatalf("missing invalid argument error text; output=%s", output)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "pratc")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	command := exec.Command("go", "build", "-o", binaryPath, "./cmd/pratc")
	command.Dir = repoRoot(t)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, output)
	}

	return binaryPath
}

func runCommand(t *testing.T, binary string, args ...string) (string, int) {
	t.Helper()

	command := exec.Command(binary, args...)
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0
	}

	exitError, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected command error type: %T", err)
	}

	return string(output), exitError.ExitCode()
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}
