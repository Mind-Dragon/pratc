package executor

import (
	"context"
	"testing"
)

func TestVerifyMerge_PassesWhenMerged(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "merged", HeadSHA: "abc"})

	err := VerifyMerge(ctx, fake, "owner/repo", 101)
	if err != nil {
		t.Fatalf("VerifyMerge should pass when PR is merged: %v", err)
	}
}

func TestVerifyMerge_FailsWhenNotMerged(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", HeadSHA: "abc"})

	err := VerifyMerge(ctx, fake, "owner/repo", 101)
	if err == nil {
		t.Fatal("VerifyMerge should fail when PR is not merged")
	}
}

func TestVerifyClose_PassesWhenClosed(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "closed", HeadSHA: "abc"})

	err := VerifyClose(ctx, fake, "owner/repo", 101)
	if err != nil {
		t.Fatalf("VerifyClose should pass when PR is closed: %v", err)
	}
}

func TestVerifyClose_FailsWhenNotClosed(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", HeadSHA: "abc"})

	err := VerifyClose(ctx, fake, "owner/repo", 101)
	if err == nil {
		t.Fatal("VerifyClose should fail when PR is not closed")
	}
}

func TestVerifyComment_PassesWhenExists(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})
	fake.AddComment(ctx, "owner/repo", 101, "test comment", false)

	err := VerifyComment(ctx, fake, "owner/repo", 101, "test comment")
	if err != nil {
		t.Fatalf("VerifyComment should pass when comment exists: %v", err)
	}
}

func TestVerifyComment_FailsWhenNotExists(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})

	err := VerifyComment(ctx, fake, "owner/repo", 101, "test comment")
	if err == nil {
		t.Fatal("VerifyComment should fail when comment does not exist")
	}
}

func TestVerifyLabels_PassesWhenAllExist(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})
	fake.AddLabels(ctx, "owner/repo", 101, []string{"pratc-action", "bug"}, false)

	err := VerifyLabels(ctx, fake, "owner/repo", 101, []string{"pratc-action"})
	if err != nil {
		t.Fatalf("VerifyLabels should pass when labels exist: %v", err)
	}
}

func TestVerifyLabels_FailsWhenNotExists(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})

	err := VerifyLabels(ctx, fake, "owner/repo", 101, []string{"pratc-action"})
	if err == nil {
		t.Fatal("VerifyLabels should fail when label does not exist")
	}
}

func TestVerifyFixApplied_PassesWhenOpen(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true})

	err := VerifyFixApplied(ctx, fake, "owner/repo", 101)
	if err != nil {
		t.Fatalf("VerifyFixApplied should pass when PR is open: %v", err)
	}
}

func TestVerifyFixApplied_FailsWhenNotOpen(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "closed"})

	err := VerifyFixApplied(ctx, fake, "owner/repo", 101)
	if err == nil {
		t.Fatal("VerifyFixApplied should fail when PR is not open")
	}
}
