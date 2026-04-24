package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestActionsCommandRegistersFlags(t *testing.T) {
	command := newActionsCommand()
	if command.Use != "actions" {
		t.Fatalf("use = %q, want actions", command.Use)
	}
	for _, name := range []string{"repo", "policy", "lane", "format", "dry-run", "force-cache", "resync", "max-prs"} {
		if command.Flags().Lookup(name) == nil {
			t.Fatalf("missing flag --%s", name)
		}
	}
}

func TestActionsCommandOutputsAdvisoryActionPlanJSON(t *testing.T) {
	command := newActionsCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"--repo", "opencode-ai/opencode", "--policy", "advisory", "--format", "json", "--force-cache"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute actions command: %v\nstderr=%s", err, stderr.String())
	}

	var plan types.ActionPlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode command JSON: %v\nstdout=%s", err, stdout.String())
	}
	if plan.SchemaVersion != "2.0" {
		t.Fatalf("schema version = %q", plan.SchemaVersion)
	}
	if plan.Repo != "opencode-ai/opencode" {
		t.Fatalf("repo = %q", plan.Repo)
	}
	if len(plan.WorkItems) == 0 || len(plan.Lanes) == 0 {
		t.Fatalf("expected work items and lanes, got work_items=%d lanes=%d", len(plan.WorkItems), len(plan.Lanes))
	}
	for _, intent := range plan.ActionIntents {
		if !intent.DryRun {
			t.Fatalf("advisory intent %s is not dry_run", intent.ID)
		}
	}
}

func TestActionsCommandRejectsInvalidPolicy(t *testing.T) {
	command := newActionsCommand()
	command.SetArgs([]string{"--repo", "opencode-ai/opencode", "--policy", "root"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected invalid policy error")
	}
}
