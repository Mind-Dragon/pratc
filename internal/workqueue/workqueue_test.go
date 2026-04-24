package workqueue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func testQueue(t *testing.T, now time.Time) *Queue {
	t.Helper()
	q, err := OpenSQLite(t.TempDir()+"/queue.db", WithNow(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })
	return q
}

func sampleItem(id string) types.ActionWorkItem {
	return types.ActionWorkItem{
		ID:             id,
		PRNumber:       101,
		Lane:           types.ActionLaneFocusedReview,
		State:          types.ActionWorkItemStateClaimable,
		PriorityScore:  0.70,
		Confidence:     0.80,
		RiskFlags:      []string{"low"},
		ReasonTrail:    []string{"test"},
		EvidenceRefs:   []string{"fixture"},
		AllowedActions: []types.ActionKind{types.ActionKindComment},
		IdempotencyKey: "repo#101:comment",
	}
}

func TestQueueClaimReleaseHeartbeatExpire(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	q := testQueue(t, now)
	if err := q.Upsert(ctx, "owner/repo", sampleItem("wi-101")); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	claimed, err := q.Claim(ctx, "wi-101", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.State != types.ActionWorkItemStateClaimed || claimed.LeaseState.ClaimedBy != "worker-a" {
		t.Fatalf("claimed item = %+v", claimed)
	}

	heartbeat, err := q.Heartbeat(ctx, "wi-101", "worker-a", 20*time.Minute)
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if heartbeat.LeaseState.ExpiresAt <= claimed.LeaseState.ExpiresAt {
		t.Fatalf("heartbeat did not extend lease: before=%s after=%s", claimed.LeaseState.ExpiresAt, heartbeat.LeaseState.ExpiresAt)
	}

	if _, err := q.Release(ctx, "wi-101", "worker-a"); err != nil {
		t.Fatalf("release: %v", err)
	}
	released, err := q.Get(ctx, "wi-101")
	if err != nil {
		t.Fatalf("get released: %v", err)
	}
	if released.State != types.ActionWorkItemStateClaimable || released.LeaseState.ClaimedBy != "" {
		t.Fatalf("released item = %+v", released)
	}

	if _, err := q.Claim(ctx, "wi-101", "worker-a", time.Minute); err != nil {
		t.Fatalf("claim for expiry: %v", err)
	}
	q.SetNow(func() time.Time { return now.Add(2 * time.Minute) })
	expired, err := q.ExpireLeases(ctx)
	if err != nil {
		t.Fatalf("expire leases: %v", err)
	}
	if len(expired) != 1 || expired[0] != "wi-101" {
		t.Fatalf("expired = %#v", expired)
	}
	afterExpire, err := q.Get(ctx, "wi-101")
	if err != nil {
		t.Fatalf("get after expire: %v", err)
	}
	if afterExpire.State != types.ActionWorkItemStateClaimable {
		t.Fatalf("after expire state = %s", afterExpire.State)
	}
}

func TestQueueClaimRaceOnlyOneWorkerWins(t *testing.T) {
	ctx := context.Background()
	q := testQueue(t, time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC))
	if err := q.Upsert(ctx, "owner/repo", sampleItem("wi-race")); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, worker := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(worker string) {
			defer wg.Done()
			_, err := q.Claim(ctx, "wi-race", worker, 5*time.Minute)
			results <- err
		}(worker)
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful claims = %d, want 1", successes)
	}
}

func TestQueueSameWorkerClaimIsIdempotent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	q := testQueue(t, now)
	if err := q.Upsert(ctx, "owner/repo", sampleItem("wi-idempotent")); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	first, err := q.Claim(ctx, "wi-idempotent", "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	q.SetNow(func() time.Time { return now.Add(30 * time.Second) })
	second, err := q.Claim(ctx, "wi-idempotent", "worker-a", 5*time.Minute)
	if err != nil {
		t.Fatalf("second claim same worker: %v", err)
	}
	if second.LeaseState.ExpiresAt <= first.LeaseState.ExpiresAt {
		t.Fatalf("second claim did not extend lease")
	}
}

func TestQueueTransitionRejectsInvalidTransition(t *testing.T) {
	ctx := context.Background()
	q := testQueue(t, time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC))
	if err := q.Upsert(ctx, "owner/repo", sampleItem("wi-transition")); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := q.Transition(ctx, "wi-transition", "tester", types.ActionWorkItemStateClaimable, types.ActionWorkItemStateExecuted, "skip"); err == nil {
		t.Fatal("expected invalid transition error")
	}
	if _, err := q.Transition(ctx, "wi-transition", "tester", types.ActionWorkItemStateClaimable, types.ActionWorkItemStateClaimed, "claim"); err != nil {
		t.Fatalf("valid transition: %v", err)
	}
}

func TestQueueListClaimableByLane(t *testing.T) {
	ctx := context.Background()
	q := testQueue(t, time.Date(2026, 4, 24, 14, 0, 0, 0, time.UTC))
	one := sampleItem("wi-one")
	two := sampleItem("wi-two")
	two.Lane = types.ActionLaneHumanEscalate
	if err := q.Upsert(ctx, "owner/repo", one); err != nil {
		t.Fatalf("upsert one: %v", err)
	}
	if err := q.Upsert(ctx, "owner/repo", two); err != nil {
		t.Fatalf("upsert two: %v", err)
	}
	items, err := q.GetClaimable(ctx, "owner/repo", types.ActionLaneFocusedReview, 10)
	if err != nil {
		t.Fatalf("get claimable: %v", err)
	}
	if len(items) != 1 || items[0].ID != "wi-one" {
		t.Fatalf("items = %#v", items)
	}
}

func TestQueueEnqueueActionPlanMakesItemsClaimable(t *testing.T) {
	ctx := context.Background()
	q := testQueue(t, time.Date(2026, 4, 24, 15, 0, 0, 0, time.UTC))
	plan := types.ActionPlan{
		Repo: "owner/repo",
		WorkItems: []types.ActionWorkItem{
			sampleItem("wi-plan-one"),
			sampleItem("wi-plan-two"),
		},
	}
	plan.WorkItems[1].PRNumber = 102
	plan.WorkItems[1].Lane = types.ActionLaneDuplicateClose
	if err := q.EnqueueActionPlan(ctx, plan); err != nil {
		t.Fatalf("enqueue action plan: %v", err)
	}
	items, err := q.GetClaimable(ctx, "owner/repo", "", 10)
	if err != nil {
		t.Fatalf("get claimable: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("claimable items = %d, want 2", len(items))
	}
	claimed, err := q.Claim(ctx, "wi-plan-two", "worker-b", time.Minute)
	if err != nil {
		t.Fatalf("claim enqueued item: %v", err)
	}
	if claimed.PRNumber != 102 || claimed.Lane != types.ActionLaneDuplicateClose {
		t.Fatalf("claimed item drifted: %+v", claimed)
	}
}

func sampleProofBundle(workItemID string, prNumber int) types.ProofBundle {
	return types.ProofBundle{
		ID:           "proof-" + workItemID,
		WorkItemID:   workItemID,
		PRNumber:     prNumber,
		Summary:      "test proof",
		EvidenceRefs: []string{"evidence:test"},
		ArtifactRefs: []string{"artifact:test"},
		TestCommands: []string{"go test ./..."},
		TestResults:  []string{"PASS"},
		CreatedBy:    "worker-a",
		CreatedAt:    "2026-04-24T16:00:00Z",
	}
}

func TestQueueAttachProof(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	q := testQueue(t, now)

	// Create a work item and claim it
	item := sampleItem("wi-attach")
	if err := q.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	claimed, err := q.Claim(ctx, "wi-attach", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.State != types.ActionWorkItemStateClaimed {
		t.Fatalf("expected claimed state, got %s", claimed.State)
	}

	// Test invalid item (non-existent work item ID)
	bundle := sampleProofBundle("non-existent", 101)
	_, err = q.AttachProof(ctx, "non-existent", "worker-a", bundle)
	if err == nil {
		t.Fatal("expected error for invalid work item")
	}

	// Test wrong owner (worker ID mismatch)
	bundle = sampleProofBundle("wi-attach", 101)
	_, err = q.AttachProof(ctx, "wi-attach", "worker-b", bundle)
	if err == nil {
		t.Fatal("expected error for wrong owner")
	}

	// Test stale lease (simulate lease expiration)
	q.SetNow(func() time.Time { return now.Add(30 * time.Minute) }) // lease expired
	_, err = q.AttachProof(ctx, "wi-attach", "worker-a", bundle)
	if err == nil {
		t.Fatal("expected error for stale lease")
	}
	// Reset lease
	q.SetNow(func() time.Time { return now })

	// Test duplicate idempotent attach (attach same bundle twice)
	attached, err := q.AttachProof(ctx, "wi-attach", "worker-a", bundle)
	if err != nil {
		t.Fatalf("first attach failed: %v", err)
	}
	if attached.ID != bundle.ID {
		t.Fatalf("attached bundle ID mismatch: got %s want %s", attached.ID, bundle.ID)
	}
	// Verify ProofBundleRefs includes the proof ID
	itemAfterFirst, err := q.Get(ctx, "wi-attach")
	if err != nil {
		t.Fatalf("get work item after first attach: %v", err)
	}
	if len(itemAfterFirst.ProofBundleRefs) != 1 || itemAfterFirst.ProofBundleRefs[0] != bundle.ID {
		t.Fatalf("ProofBundleRefs after first attach = %v", itemAfterFirst.ProofBundleRefs)
	}

	// Attach same bundle again (should be idempotent)
	attached2, err := q.AttachProof(ctx, "wi-attach", "worker-a", bundle)
	if err != nil {
		t.Fatalf("second attach failed: %v", err)
	}
	if attached2.ID != bundle.ID {
		t.Fatalf("second attached bundle ID mismatch: got %s want %s", attached2.ID, bundle.ID)
	}
	// Verify ProofBundleRefs still has only one entry
	itemAfterSecond, err := q.Get(ctx, "wi-attach")
	if err != nil {
		t.Fatalf("get work item after second attach: %v", err)
	}
	if len(itemAfterSecond.ProofBundleRefs) != 1 || itemAfterSecond.ProofBundleRefs[0] != bundle.ID {
		t.Fatalf("ProofBundleRefs after second attach = %v", itemAfterSecond.ProofBundleRefs)
	}

	// Test successful attach transitions claimed -> preflighted
	// (Assuming that's the expected transition after proof attach)
	// Let's verify the state after attach (should be preflighted?)
	// According to task: "transition claimed -> preflighted once."
	// We'll check that the state changed to preflighted.
	itemAfterAttach, err := q.Get(ctx, "wi-attach")
	if err != nil {
		t.Fatalf("get work item after attach: %v", err)
	}
	if itemAfterAttach.State != types.ActionWorkItemStatePreflighted {
		t.Fatalf("expected state preflighted after proof attach, got %s", itemAfterAttach.State)
	}
}

func TestGetActionQueueStats(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	q := testQueue(t, now)

	// Create items in different lanes and states
	item1 := sampleItem("wi-stats-1")
	item1.Lane = types.ActionLaneFocusedReview
	item1.State = types.ActionWorkItemStateClaimable
	if err := q.Upsert(ctx, "owner/repo", item1); err != nil {
		t.Fatalf("upsert item1: %v", err)
	}

	item2 := sampleItem("wi-stats-2")
	item2.Lane = types.ActionLaneFocusedReview
	item2.State = types.ActionWorkItemStateClaimed
	if err := q.Upsert(ctx, "owner/repo", item2); err != nil {
		t.Fatalf("upsert item2: %v", err)
	}

	item3 := sampleItem("wi-stats-3")
	item3.Lane = types.ActionLaneHumanEscalate
	item3.State = types.ActionWorkItemStateClaimable
	if err := q.Upsert(ctx, "owner/repo", item3); err != nil {
		t.Fatalf("upsert item3: %v", err)
	}

	stats, err := q.GetActionQueueStats()
	if err != nil {
		t.Fatalf("GetActionQueueStats: %v", err)
	}

	// Verify the stats
	focusedReviewStats, ok := stats["focused_review"]
	if !ok {
		t.Fatal("focused_review lane not found in stats")
	}
	if focusedReviewStats["claimable"] != 1 {
		t.Fatalf("focused_review claimable count = %d, want 1", focusedReviewStats["claimable"])
	}
	if focusedReviewStats["claimed"] != 1 {
		t.Fatalf("focused_review claimed count = %d, want 1", focusedReviewStats["claimed"])
	}

	humanEscalateStats, ok := stats["human_escalate"]
	if !ok {
		t.Fatal("human_escalate lane not found in stats")
	}
	if humanEscalateStats["claimable"] != 1 {
		t.Fatalf("human_escalate claimable count = %d, want 1", humanEscalateStats["claimable"])
	}
}

func TestGetExecutorLedger(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	q := testQueue(t, now)

	// Create items and transitions
	item1 := sampleItem("wi-ledger-1")
	if err := q.Upsert(ctx, "owner/repo", item1); err != nil {
		t.Fatalf("upsert item1: %v", err)
	}

	// Claim the item to create a transition
	_, err := q.Claim(ctx, "wi-ledger-1", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Create a proof bundle
	bundle := sampleProofBundle("wi-ledger-1", 101)
	_, err = q.AttachProof(ctx, "wi-ledger-1", "worker-a", bundle)
	if err != nil {
		t.Fatalf("attach proof: %v", err)
	}

	// Get the ledger
	ledger, err := q.GetExecutorLedger(10)
	if err != nil {
		t.Fatalf("GetExecutorLedger: %v", err)
	}

	// Verify transitions
	if len(ledger.Transitions) == 0 {
		t.Fatal("expected at least one transition in ledger")
	}

	// Verify the transition details
	foundClaimTransition := false
	for _, tr := range ledger.Transitions {
		if tr.WorkItemID == "wi-ledger-1" && tr.ToState == "claimed" && tr.Actor == "worker-a" {
			foundClaimTransition = true
			if tr.FromState != "claimable" {
				t.Fatalf("expected from_state = claimable, got %s", tr.FromState)
			}
			if tr.Reason != "claim" {
				t.Fatalf("expected reason = claim, got %s", tr.Reason)
			}
		}
	}
	if !foundClaimTransition {
		t.Fatal("expected claim transition not found in ledger")
	}

	// Verify proof bundles
	if len(ledger.ProofBundles) == 0 {
		t.Fatal("expected at least one proof bundle in ledger")
	}

	foundBundle := false
	for _, b := range ledger.ProofBundles {
		if b.ID == "proof-wi-ledger-1" {
			foundBundle = true
			if b.WorkItemID != "wi-ledger-1" {
				t.Fatalf("proof bundle work item mismatch: got %s, want wi-ledger-1", b.WorkItemID)
			}
			if b.PRNumber != 101 {
				t.Fatalf("proof bundle PR number mismatch: got %d, want 101", b.PRNumber)
			}
			if b.Summary != "test proof" {
				t.Fatalf("proof bundle summary mismatch: got %s, want 'test proof'", b.Summary)
			}
			if len(b.TestCommands) != 1 || b.TestCommands[0] != "go test ./..." {
				t.Fatalf("proof bundle test commands mismatch: %v", b.TestCommands)
			}
		}
	}
	if !foundBundle {
		t.Fatal("expected proof bundle not found in ledger")
	}
}

func TestGetQueueSummary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	q := testQueue(t, now)

	// Create items in different states and lanes
	item1 := sampleItem("wi-summary-1")
	item1.Lane = types.ActionLaneFocusedReview
	item1.State = types.ActionWorkItemStateClaimable
	if err := q.Upsert(ctx, "owner/repo", item1); err != nil {
		t.Fatalf("upsert item1: %v", err)
	}

	item2 := sampleItem("wi-summary-2")
	item2.Lane = types.ActionLaneFocusedReview
	item2.State = types.ActionWorkItemStateClaimed
	if err := q.Upsert(ctx, "owner/repo", item2); err != nil {
		t.Fatalf("upsert item2: %v", err)
	}

	item3 := sampleItem("wi-summary-3")
	item3.Lane = types.ActionLaneHumanEscalate
	item3.State = types.ActionWorkItemStatePreflighted
	if err := q.Upsert(ctx, "owner/repo", item3); err != nil {
		t.Fatalf("upsert item3: %v", err)
	}

	// Claim item2 to set up lease
	_, err := q.Claim(ctx, "wi-summary-2", "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Expire the lease
	q.SetNow(func() time.Time { return now.Add(2 * time.Minute) })
	_, err = q.ExpireLeases(ctx)
	if err != nil {
		t.Fatalf("expire leases: %v", err)
	}

	// Reset time for summary
	q.SetNow(func() time.Time { return now })

	summary, err := q.GetQueueSummary()
	if err != nil {
		t.Fatalf("GetQueueSummary: %v", err)
	}

	// Verify total items
	if summary.TotalItems != 3 {
		t.Fatalf("total items = %d, want 3", summary.TotalItems)
	}

	// Verify by state
	if summary.ByState["claimable"] != 2 {
		t.Fatalf("claimable count = %d, want 2", summary.ByState["claimable"])
	}
	if summary.ByState["preflighted"] != 1 {
		t.Fatalf("preflighted count = %d, want 1", summary.ByState["preflighted"])
	}

	// Verify by lane
	if summary.ByLane["focused_review"] != 2 {
		t.Fatalf("focused_review count = %d, want 2", summary.ByLane["focused_review"])
	}
	if summary.ByLane["human_escalate"] != 1 {
		t.Fatalf("human_escalate count = %d, want 1", summary.ByLane["human_escalate"])
	}

	// Verify expired leases (item2 should have expired lease)
	if summary.ExpiredLeases != 1 {
		t.Fatalf("expired leases = %d, want 1", summary.ExpiredLeases)
	}
}
