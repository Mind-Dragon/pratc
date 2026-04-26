package e2e

import (
	"testing"
)

func TestE2EHarness_CommentAction(t *testing.T) {
	harness := NewE2EHarness()
	defer harness.Cleanup()

	err := harness.TestCommentAction()
	if err != nil {
		t.Fatalf("TestCommentAction failed: %v", err)
	}
}

func TestE2EHarness_LabelAction(t *testing.T) {
	harness := NewE2EHarness()
	defer harness.Cleanup()

	err := harness.TestLabelAction()
	if err != nil {
		t.Fatalf("TestLabelAction failed: %v", err)
	}
}

func TestE2EHarness_MergeAction(t *testing.T) {
	harness := NewE2EHarness()
	defer harness.Cleanup()

	err := harness.TestMergeAction()
	if err != nil {
		t.Fatalf("TestMergeAction failed: %v", err)
	}
}

func TestE2EHarness_AdvisoryModeNoWrites(t *testing.T) {
	harness := NewE2EHarness()
	defer harness.Cleanup()

	err := harness.TestAdvisoryModeNoWrites()
	if err != nil {
		t.Fatalf("TestAdvisoryModeNoWrites failed: %v", err)
	}
}
