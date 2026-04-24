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
