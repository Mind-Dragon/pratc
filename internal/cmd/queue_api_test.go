package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

func testQueue(t *testing.T, now time.Time) *workqueue.Queue {
	t.Helper()
	q, err := workqueue.OpenSQLite(t.TempDir()+"/queue.db", workqueue.WithNow(func() time.Time { return now }))
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

func TestQueueClaimRoute(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Enqueue a work item
	item := sampleItem("wi-claim")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Prepare claim request
	reqBody := claimRequest{
		WorkerID:   "worker-a",
		TTLSeconds: 300,
		ID:         "wi-claim",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueClaim(rr, req, queue, "owner/repo")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp claimResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if resp.WorkItem.ID != "wi-claim" {
		t.Errorf("claimed work item ID = %q, want %q", resp.WorkItem.ID, "wi-claim")
	}
	if resp.WorkItem.State != types.ActionWorkItemStateClaimed {
		t.Errorf("claimed work item state = %q, want %q", resp.WorkItem.State, types.ActionWorkItemStateClaimed)
	}
	if resp.WorkItem.LeaseState.ClaimedBy != "worker-a" {
		t.Errorf("claimed by = %q, want %q", resp.WorkItem.LeaseState.ClaimedBy, "worker-a")
	}
}

func TestQueueAPIRoutesRequireAuthAndSupportAPIPath(t *testing.T) {
	t.Setenv("PRATC_API_KEY", "test-key")
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)
	if err := queue.Upsert(ctx, "owner/repo", sampleItem("wi-api-claim")); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	mux := http.NewServeMux()
	registerQueueRoutes(mux, queue, "owner/repo")
	handler := authMiddleware(mux)

	// Test claim route auth
	payload, _ := json.Marshal(claimRequest{WorkerID: "worker-api", TTLSeconds: 300, ID: "wi-api-claim"})
	unauth := httptest.NewRequest(http.MethodPost, "/api/queue/claim", bytes.NewReader(payload))
	unauth.Header.Set("Content-Type", "application/json")
	unauthRR := httptest.NewRecorder()
	handler.ServeHTTP(unauthRR, unauth)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want 401; body=%s", unauthRR.Code, unauthRR.Body.String())
	}

	auth := httptest.NewRequest(http.MethodPost, "/api/queue/claim", bytes.NewReader(payload))
	auth.Header.Set("Content-Type", "application/json")
	auth.Header.Set("X-API-Key", "test-key")
	authRR := httptest.NewRecorder()
	handler.ServeHTTP(authRR, auth)
	if authRR.Code != http.StatusOK {
		t.Fatalf("auth status = %d, want 200; body=%s", authRR.Code, authRR.Body.String())
	}

	// Now test proof attach route auth (requires claimed work item)
	// Claim the work item via queue (since we have the queue instance)
	_, err := queue.Claim(ctx, "wi-api-claim", "worker-api", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim for proof attach: %v", err)
	}
	bundle := sampleProofBundle("wi-api-claim", 101)
	proofPayload, _ := json.Marshal(proofAttachRequest{
		WorkerID:    "worker-api",
		WorkItemID:  "wi-api-claim",
		ProofBundle: bundle,
	})
	unauthProof := httptest.NewRequest(http.MethodPost, "/api/queue/proof", bytes.NewReader(proofPayload))
	unauthProof.Header.Set("Content-Type", "application/json")
	unauthProofRR := httptest.NewRecorder()
	handler.ServeHTTP(unauthProofRR, unauthProof)
	if unauthProofRR.Code != http.StatusUnauthorized {
		t.Fatalf("unauth proof status = %d, want 401; body=%s", unauthProofRR.Code, unauthProofRR.Body.String())
	}

	authProof := httptest.NewRequest(http.MethodPost, "/api/queue/proof", bytes.NewReader(proofPayload))
	authProof.Header.Set("Content-Type", "application/json")
	authProof.Header.Set("X-API-Key", "test-key")
	authProofRR := httptest.NewRecorder()
	handler.ServeHTTP(authProofRR, authProof)
	if authProofRR.Code != http.StatusOK {
		t.Fatalf("auth proof status = %d, want 200; body=%s", authProofRR.Code, authProofRR.Body.String())
	}
}

func TestQueueClaimRouteWithoutID(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Enqueue a work item
	item := sampleItem("wi-claim2")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Prepare claim request without ID, using repo/lane filters
	reqBody := claimRequest{
		WorkerID:   "worker-b",
		TTLSeconds: 300,
		Repo:       "owner/repo",
		Lane:       "focused_review",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueClaim(rr, req, queue, "owner/repo")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp claimResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if resp.WorkItem.ID != "wi-claim2" {
		t.Errorf("claimed work item ID = %q, want %q", resp.WorkItem.ID, "wi-claim2")
	}
}

func TestQueueClaimRouteMissingAPIKey(t *testing.T) {
	// This test would require auth middleware, but our handler doesn't check API key.
	// The auth middleware is applied at server level; we can test that the handler
	// doesn't panic. For now, skip.
	t.Skip("auth middleware is applied at server level")
}

func TestQueueReleaseRoute(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Enqueue and claim a work item
	item := sampleItem("wi-release")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, err := queue.Claim(ctx, "wi-release", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Prepare release request
	reqBody := releaseRequest{
		WorkerID: "worker-a",
		ID:       "wi-release",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/release", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueRelease(rr, req, queue)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp releaseResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if resp.WorkItem.State != types.ActionWorkItemStateClaimable {
		t.Errorf("released work item state = %q, want %q", resp.WorkItem.State, types.ActionWorkItemStateClaimable)
	}
}

func TestQueueHeartbeatRoute(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Enqueue and claim a work item
	item := sampleItem("wi-heartbeat")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	claimed, err := queue.Claim(ctx, "wi-heartbeat", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	oldExpiry := claimed.LeaseState.ExpiresAt

	// Prepare heartbeat request
	reqBody := heartbeatRequest{
		WorkerID:   "worker-a",
		TTLSeconds: 1200,
		ID:         "wi-heartbeat",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueHeartbeat(rr, req, queue)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp heartbeatResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if resp.WorkItem.LeaseState.ExpiresAt <= oldExpiry {
		t.Errorf("heartbeat did not extend lease: old=%s new=%s", oldExpiry, resp.WorkItem.LeaseState.ExpiresAt)
	}
}

func TestQueueStatusRoute(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Enqueue two work items in different lanes
	item1 := sampleItem("wi-status1")
	item2 := sampleItem("wi-status2")
	item2.Lane = types.ActionLaneHumanEscalate
	if err := queue.Upsert(ctx, "owner/repo", item1); err != nil {
		t.Fatalf("upsert one: %v", err)
	}
	if err := queue.Upsert(ctx, "owner/repo", item2); err != nil {
		t.Fatalf("upsert two: %v", err)
	}

	// Claim one of them
	_, err := queue.Claim(ctx, "wi-status1", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Prepare status request
	req := httptest.NewRequest(http.MethodGet, "/queue/status?repo=owner/repo", nil)
	rr := httptest.NewRecorder()

	handleQueueStatus(rr, req, queue, "owner/repo")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp statusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if resp.Repo != "owner/repo" {
		t.Errorf("repo = %q, want %q", resp.Repo, "owner/repo")
	}
	// Expect claimable count for focused_review lane = 0 (since wi-status1 claimed)
	// and claimed count = 1
	focused := resp.Counts[types.ActionLaneFocusedReview]
	if focused.Claimable != 0 {
		t.Errorf("focused_review claimable = %d, want 0", focused.Claimable)
	}
	if focused.Claimed != 1 {
		t.Errorf("focused_review claimed = %d, want 1", focused.Claimed)
	}
	// Expect claimable count for human_escalate lane = 1 (unclaimed)
	human := resp.Counts[types.ActionLaneHumanEscalate]
	if human.Claimable != 1 {
		t.Errorf("human_escalate claimable = %d, want 1", human.Claimable)
	}
	if human.Claimed != 0 {
		t.Errorf("human_escalate claimed = %d, want 0", human.Claimed)
	}
}

func TestQueueStatusRouteExpiresLeases(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)
	if err := queue.Upsert(ctx, "owner/repo", sampleItem("wi-expired-status")); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := queue.Claim(ctx, "wi-expired-status", "worker-a", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}
	queue.SetNow(func() time.Time { return now.Add(2 * time.Minute) })

	req := httptest.NewRequest(http.MethodGet, "/queue/status?repo=owner/repo&lane=focused_review", nil)
	rr := httptest.NewRecorder()
	handleQueueStatus(rr, req, queue, "owner/repo")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp statusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	focused := resp.Counts[types.ActionLaneFocusedReview]
	if focused.Claimable != 1 || focused.Claimed != 0 {
		t.Fatalf("expired focused counts = %+v, want claimable=1 claimed=0", focused)
	}
}

func TestQueueRaceDisjointClaims(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Enqueue 10 items
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("wi-race-%d", i)
		item := sampleItem(id)
		if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	// Simulate 10 concurrent claim requests via HTTP handlers
	// We'll use a channel to collect results
	type result struct {
		id  string
		err error
	}
	results := make(chan result, 10)
	// We'll fire goroutines each claiming a different item (by ID) with different worker IDs
	// Since the queue's Claim method already ensures one-winner race, we can just test that
	// each worker can claim a distinct item (by picking different IDs).
	// Let's assign each worker a unique ID (they are claimable).
	for i := 1; i <= 10; i++ {
		go func(idx int) {
			id := fmt.Sprintf("wi-race-%d", idx)
			reqBody := claimRequest{
				WorkerID:   fmt.Sprintf("worker-%d", idx),
				TTLSeconds: 300,
				ID:         id,
			}
			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/queue/claim", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handleQueueClaim(rr, req, queue, "owner/repo")
			if rr.Code != http.StatusOK {
				results <- result{id: id, err: fmt.Errorf("status %d: %s", rr.Code, rr.Body.String())}
			} else {
				results <- result{id: id, err: nil}
			}
		}(i)
	}

	// Wait for all goroutines
	// We'll collect results (order not important)
	successCount := 0
	for i := 0; i < 10; i++ {
		res := <-results
		if res.err == nil {
			successCount++
		} else {
			t.Logf("failed claim for %s: %v", res.id, res.err)
		}
	}
	if successCount != 10 {
		t.Errorf("expected 10 successful disjoint claims, got %d", successCount)
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

func TestQueueProofAttachRoute(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Create a work item and claim it
	item := sampleItem("wi-attach")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, err := queue.Claim(ctx, "wi-attach", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Prepare proof attach request
	bundle := sampleProofBundle("wi-attach", 101)
	reqBody := proofAttachRequest{
		WorkerID:    "worker-a",
		WorkItemID:  "wi-attach",
		ProofBundle: bundle,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/proof", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueProof(rr, req, queue)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp proofAttachResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if resp.ProofBundle.ID != bundle.ID {
		t.Errorf("proof bundle ID = %q, want %q", resp.ProofBundle.ID, bundle.ID)
	}
	if resp.WorkItem.State != types.ActionWorkItemStatePreflighted {
		t.Errorf("work item state = %q, want %q", resp.WorkItem.State, types.ActionWorkItemStatePreflighted)
	}
	if len(resp.WorkItem.ProofBundleRefs) != 1 || resp.WorkItem.ProofBundleRefs[0] != bundle.ID {
		t.Errorf("proof bundle refs = %v, want [%s]", resp.WorkItem.ProofBundleRefs, bundle.ID)
	}
}

func TestQueueProofAttachRouteIdempotent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	// Create a work item, claim it, attach proof
	item := sampleItem("wi-attach2")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, err := queue.Claim(ctx, "wi-attach2", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	bundle := sampleProofBundle("wi-attach2", 101)
	_, err = queue.AttachProof(ctx, "wi-attach2", "worker-a", bundle)
	if err != nil {
		t.Fatalf("first attach: %v", err)
	}

	// Now try attaching same bundle via HTTP (should be idempotent)
	reqBody := proofAttachRequest{
		WorkerID:    "worker-a",
		WorkItemID:  "wi-attach2",
		ProofBundle: bundle,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/proof", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueProof(rr, req, queue)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp proofAttachResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if resp.ProofBundle.ID != bundle.ID {
		t.Errorf("proof bundle ID = %q, want %q", resp.ProofBundle.ID, bundle.ID)
	}
	// Ensure ProofBundleRefs still has only one entry
	if len(resp.WorkItem.ProofBundleRefs) != 1 || resp.WorkItem.ProofBundleRefs[0] != bundle.ID {
		t.Errorf("proof bundle refs = %v, want [%s]", resp.WorkItem.ProofBundleRefs, bundle.ID)
	}
}

func TestQueueProofAttachRouteInvalidItem(t *testing.T) {
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	bundle := sampleProofBundle("non-existent", 101)
	reqBody := proofAttachRequest{
		WorkerID:    "worker-a",
		WorkItemID:  "non-existent",
		ProofBundle: bundle,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/proof", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueProof(rr, req, queue)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestQueueProofAttachRouteWrongOwner(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	item := sampleItem("wi-attach3")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, err := queue.Claim(ctx, "wi-attach3", "worker-a", 10*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	bundle := sampleProofBundle("wi-attach3", 101)
	reqBody := proofAttachRequest{
		WorkerID:    "worker-b",
		WorkItemID:  "wi-attach3",
		ProofBundle: bundle,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/proof", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueProof(rr, req, queue)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestQueueProofAttachRouteStaleLease(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	queue := testQueue(t, now)

	item := sampleItem("wi-attach4")
	if err := queue.Upsert(ctx, "owner/repo", item); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	_, err := queue.Claim(ctx, "wi-attach4", "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	// Expire lease
	queue.SetNow(func() time.Time { return now.Add(2 * time.Minute) })

	bundle := sampleProofBundle("wi-attach4", 101)
	reqBody := proofAttachRequest{
		WorkerID:    "worker-a",
		WorkItemID:  "wi-attach4",
		ProofBundle: bundle,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/queue/proof", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handleQueueProof(rr, req, queue)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
	}
}
