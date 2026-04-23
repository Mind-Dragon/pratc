package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/version"
	"github.com/spf13/cobra"
)

func TestWriteVersionIncludesMachineReadableBuildProvenance(t *testing.T) {
	oldVersion := version.Version
	oldBuildDate := version.BuildDate
	oldCommit := version.Commit
	oldDirty := version.Dirty
	t.Cleanup(func() {
		version.Version = oldVersion
		version.BuildDate = oldBuildDate
		version.Commit = oldCommit
		version.Dirty = oldDirty
	})

	version.Version = "1.7.0"
	version.BuildDate = "2026-04-23T22:00:00Z"
	version.Commit = "abcdef123456"
	version.Dirty = "false"

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := writeVersion(cmd); err != nil {
		t.Fatalf("writeVersion: %v", err)
	}

	output := out.String()
	for _, want := range []string{
		"version=1.7.0",
		"commit=abcdef123456",
		"build_date=2026-04-23T22:00:00Z",
		"dirty=false",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("version output %q missing %q", output, want)
		}
	}
}

func TestVersionCommandRegisteredOnRoot(t *testing.T) {
	cmd := &cobra.Command{Use: "pratc"}
	registerVersionCommand(cmd)

	if got, _, err := cmd.Find([]string{"version"}); err != nil || got == nil || got.Name() != "version" {
		t.Fatalf("version command not registered: cmd=%v err=%v", got, err)
	}
}
