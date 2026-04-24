package types

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGoAndPythonProduceIdenticalJSON(t *testing.T) {
	t.Parallel()

	report := sampleAnalysisResponse()

	goJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal go payload: %v", err)
	}

	pythonPath, ok := findPython()
	if !ok {
		t.Skip("python interpreter not available")
	}

	repoRoot := repoRoot(t)
	pythonFile := filepath.Join(repoRoot, "ml-service", "src", "pratc_ml", "models.py")
	cmd := exec.Command(pythonPath, pythonFile)
	cmd.Env = append(os.Environ(), "PRATC_SAMPLE_ANALYSIS_JSON="+string(goJSON))
	cmd.Dir = repoRoot

	pythonJSON, err := cmd.Output()
	if err != nil {
		t.Fatalf("run python serializer: %v", err)
	}

	if !bytes.Equal(normalizeJSON(t, goJSON), normalizeJSON(t, pythonJSON)) {
		t.Fatalf("go and python json differ\ngo=%s\npython=%s", normalizeJSON(t, goJSON), normalizeJSON(t, pythonJSON))
	}
}

func TestJSONSchemaFilesCoverTaskContracts(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	requiredSchemas := map[string][]string{
		"analysis-response.json":           {"repo", "generatedAt", "counts", "clusters", "duplicates", "overlaps", "conflicts", "stalenessSignals"},
		"cluster-response.json":            {"repo", "generatedAt", "model", "thresholds", "clusters"},
		"duplicate-response.json":          {"repo", "generatedAt", "duplicates", "overlaps"},
		"semantic-conflict-response.json":  {"repo", "generatedAt", "conflicts"},
		"graph-response.json":              {"repo", "generatedAt", "nodes", "edges", "dot"},
		"plan-response.json":               {"repo", "generatedAt", "target", "candidatePoolSize", "strategy", "selected", "ordering", "rejections"},
		"health-response.json":             {"status", "version"},
		"cluster-request.json":             {"repo", "prs", "model", "minClusterSize"},
		"duplicate-detection-request.json": {"repo", "prs", "duplicateThreshold", "overlapThreshold"},
		"semantic-analysis-request.json":   {"repo", "prs", "analysisMode"},
	}

	for file, fields := range requiredSchemas {
		file := file
		fields := fields
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(repoRoot, "contracts", file))
			if err != nil {
				t.Fatalf("read schema %s: %v", file, err)
			}

			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatalf("parse schema %s: %v", file, err)
			}

			properties, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("schema %s missing properties", file)
			}

			required, ok := schema["required"].([]any)
			if !ok {
				t.Fatalf("schema %s missing required list", file)
			}

			for _, field := range fields {
				if _, ok := properties[field]; !ok {
					t.Fatalf("schema %s missing property %s", file, field)
				}
				if !contains(required, field) {
					t.Fatalf("schema %s missing required entry %s", file, field)
				}
			}
		})
	}
}

func TestTypeScriptInterfacesMirrorCanonicalFields(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(repoRoot, "web", "src", "types", "api.ts"))
	if err != nil {
		t.Fatalf("read typescript file: %v", err)
	}

	for _, token := range []string{
		"export interface PR",
		"files_changed: string[];",
		"provenance?: Record<string, string>;",
		"cluster_id: string;",
		"export interface PRCluster",
		"cluster_label: string;",
		"health_status: string;",
		"export interface AnalysisResponse",
		"generatedAt: string;",
		"review_payload?: ReviewResponse;",
		"export interface ReviewResponse",
		"export interface AnalyzerFinding",
		"analyzer_name: string;",
		"subsystem?: string;",
		"signal_type?: string;",
		"evidence_hash?: string;",
		"export interface BucketCount",
		"buckets: BucketCount[];",
		"export interface ReviewResult",
		"blockers: string[];",
		"evidence_references: string[];",
		"next_action: string;",
		"stalenessSignals: StalenessReport[];",
		"export interface PlanResponse",
		"candidatePoolSize: number;",
		"export interface CollapsedCorpus",
		"collapsed_corpus?: CollapsedCorpus;",
		"is_collapsed_canonical?: boolean;",
		"superseded_prs?: number[];",
	} {
		if !bytes.Contains(raw, []byte(token)) {
			t.Fatalf("expected token %q in api.ts", token)
		}
	}
}

func normalizeJSON(t *testing.T, raw []byte) []byte {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}

	normalized, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-encode json: %v", err)
	}

	return normalized
}

func contains(items []any, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func findPython() (string, bool) {
	candidates := []string{"python3", "/opt/homebrew/bin/python3.14", "/opt/homebrew/bin/python3.13", "/opt/homebrew/bin/python3.11"}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	return root
}

func sampleAnalysisResponse() AnalysisResponse {
	pr := PR{
		ID:                "PR_kwDOAA",
		Repo:              "owner/repo",
		Number:            101,
		Title:             "Reduce planner conflicts",
		Body:              "Introduces a smaller candidate pool.",
		URL:               GitHubURLPrefix + "owner/repo/pull/101",
		Author:            "octocat",
		Labels:            []string{"planner", "ci"},
		FilesChanged:      []string{"internal/planner/plan.go", "internal/types/models.go"},
		ReviewStatus:      "approved",
		CIStatus:          "passing",
		Mergeable:         "mergeable",
		BaseBranch:        "main",
		HeadBranch:        "planner/reduce-conflicts",
		ClusterID:         "cluster-1",
		CreatedAt:         "2026-03-12T10:00:00Z",
		UpdatedAt:         "2026-03-12T11:00:00Z",
		IsDraft:           false,
		IsBot:             false,
		Additions:         42,
		Deletions:         7,
		ChangedFilesCount: 2,
	}

	cluster := PRCluster{
		ClusterID:         "cluster-1",
		ClusterLabel:      "planner conflicts",
		Summary:           "Planner-focused PRs that reduce merge contention.",
		PRIDs:             []int{101},
		HealthStatus:      "green",
		AverageSimilarity: 0.94,
		SampleTitles:      []string{"Reduce planner conflicts"},
	}

	dup := DuplicateGroup{
		CanonicalPRNumber: 101,
		DuplicatePRNums:   []int{104},
		Similarity:        0.93,
		Reason:            "Title and file overlap exceed duplicate threshold.",
	}

	conflict := ConflictPair{
		SourcePR:     101,
		TargetPR:     102,
		ConflictType: "file_overlap",
		FilesTouched: []string{"internal/planner/plan.go"},
		Severity:     "medium",
		Reason:       "Both PRs modify the planner ordering logic.",
	}

	stale := StalenessReport{
		PRNumber:     103,
		Score:        88,
		Signals:      []string{"superseded", "inactive"},
		Reasons:      []string{"A merged PR already touched the same files.", "No updates in 45 days."},
		SupersededBy: []int{99},
	}

	return AnalysisResponse{
		Repo:        "owner/repo",
		GeneratedAt: "2026-03-12T12:00:00Z",
		Counts: Counts{
			TotalPRs:        3,
			ClusterCount:    1,
			DuplicateGroups: 1,
			OverlapGroups:   1,
			ConflictPairs:   1,
			StalePRs:        1,
		},
		PRs:              []PR{pr},
		Clusters:         []PRCluster{cluster},
		Duplicates:       []DuplicateGroup{dup},
		Overlaps:         []DuplicateGroup{dup},
		Conflicts:        []ConflictPair{conflict},
		StalenessSignals: []StalenessReport{stale},
	}
}

func TestSampleAnalysisResponseContainsExpectedKeys(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(sampleAnalysisResponse())
	if err != nil {
		t.Fatalf("marshal sample response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode sample response: %v", err)
	}

	want := []string{"repo", "generatedAt", "counts", "prs", "clusters", "duplicates", "overlaps", "conflicts", "stalenessSignals"}
	for _, key := range want {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing key %s", key)
		}
	}

	prs, ok := decoded["prs"].([]any)
	if !ok || len(prs) != 1 {
		t.Fatalf("unexpected prs payload: %#v", decoded["prs"])
	}
}

func TestReviewPayloadOmittedWhenNil(t *testing.T) {
	t.Parallel()

	// Test that ReviewPayload is omitted when nil (backward compatibility)
	respWithoutReview := AnalysisResponse{
		Repo:          "owner/repo",
		GeneratedAt:   "2026-04-09T12:00:00Z",
		Counts:        Counts{TotalPRs: 1},
		PRs:           []PR{},
		ReviewPayload: nil, // Explicitly nil - should be omitted
	}

	raw, err := json.Marshal(respWithoutReview)
	if err != nil {
		t.Fatalf("marshal response without review: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify review_payload is NOT present when nil
	if _, exists := decoded["review_payload"]; exists {
		t.Fatalf("review_payload should be omitted when nil, but was present in: %s", string(raw))
	}

	// Verify other fields ARE present
	requiredFields := []string{"repo", "generatedAt", "counts", "prs"}
	for _, field := range requiredFields {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("required field %s missing from response", field)
		}
	}

	// Test that ReviewPayload IS included when set (verify omitempty doesn't always omit)
	respWithReview := AnalysisResponse{
		Repo:        "owner/repo",
		GeneratedAt: "2026-04-09T12:00:00Z",
		Counts:      Counts{TotalPRs: 1},
		PRs:         []PR{},
		ReviewPayload: &ReviewResponse{
			TotalPRs:      1,
			ReviewedPRs:   1,
			Categories:    []ReviewCategoryCount{},
			PriorityTiers: []PriorityTierCount{},
			Results: []ReviewResult{{
				Category:           ReviewCategoryMergeNow,
				PriorityTier:       PriorityTierFastMerge,
				Confidence:         0.98,
				Reasons:            []string{"high_confidence"},
				Blockers:           []string{},
				EvidenceReferences: []string{"diff:internal/planner/plan.go"},
				NextAction:         "merge_now",
				AnalyzerFindings: []AnalyzerFinding{{
					AnalyzerName:    "security",
					AnalyzerVersion: "1.0.0",
					Finding:         "no issues",
					Confidence:      0.99,
				}},
			}},
		},
	}

	rawWithReview, err := json.Marshal(respWithReview)
	if err != nil {
		t.Fatalf("marshal response with review: %v", err)
	}

	var decodedWithReview map[string]any
	if err := json.Unmarshal(rawWithReview, &decodedWithReview); err != nil {
		t.Fatalf("decode response with review: %v", err)
	}

	// Verify review_payload IS present when set
	if _, exists := decodedWithReview["review_payload"]; !exists {
		t.Fatalf("review_payload should be present when set, but was missing in: %s", string(rawWithReview))
	}

	reviewPayload, ok := decodedWithReview["review_payload"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected review_payload structure: %#v", decodedWithReview["review_payload"])
	}

	results, ok := reviewPayload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected review results payload: %#v", reviewPayload["results"])
	}

	first, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first review result payload: %#v", results[0])
	}

	for _, field := range []string{"blockers", "evidence_references", "next_action"} {
		if _, exists := first[field]; !exists {
			t.Fatalf("review result field %s should be present when set, but was missing in: %s", field, string(rawWithReview))
		}
	}
}

func TestActionConstantsContract(t *testing.T) {
	t.Parallel()

	lanes := []ActionLane{
		ActionLaneFastMerge,
		ActionLaneFixAndMerge,
		ActionLaneDuplicateClose,
		ActionLaneRejectOrClose,
		ActionLaneFocusedReview,
		ActionLaneFutureOrReengage,
		ActionLaneHumanEscalate,
	}
	wantLanes := []string{
		"fast_merge",
		"fix_and_merge",
		"duplicate_close",
		"reject_or_close",
		"focused_review",
		"future_or_reengage",
		"human_escalate",
	}
	for i, lane := range lanes {
		if string(lane) != wantLanes[i] {
			t.Fatalf("lane %d = %q, want %q", i, lane, wantLanes[i])
		}
	}

	profiles := []PolicyProfile{
		PolicyProfileAdvisory,
		PolicyProfileGuarded,
		PolicyProfileAutonomous,
	}
	wantProfiles := []string{"advisory", "guarded", "autonomous"}
	for i, profile := range profiles {
		if string(profile) != wantProfiles[i] {
			t.Fatalf("policy profile %d = %q, want %q", i, profile, wantProfiles[i])
		}
	}
	if DefaultPolicyProfile != PolicyProfileAdvisory {
		t.Fatalf("default policy profile = %q, want %q", DefaultPolicyProfile, PolicyProfileAdvisory)
	}

	states := []ActionWorkItemState{
		ActionWorkItemStateProposed,
		ActionWorkItemStateClaimable,
		ActionWorkItemStateClaimed,
		ActionWorkItemStatePreflighted,
		ActionWorkItemStatePatched,
		ActionWorkItemStateTested,
		ActionWorkItemStateApprovedForExecution,
		ActionWorkItemStateExecuted,
		ActionWorkItemStateVerified,
		ActionWorkItemStateFailed,
		ActionWorkItemStateEscalated,
		ActionWorkItemStateCanceled,
	}
	wantStates := []string{
		"proposed",
		"claimable",
		"claimed",
		"preflighted",
		"patched",
		"tested",
		"approved_for_execution",
		"executed",
		"verified",
		"failed",
		"escalated",
		"canceled",
	}
	for i, state := range states {
		if string(state) != wantStates[i] {
			t.Fatalf("work item state %d = %q, want %q", i, state, wantStates[i])
		}
	}

	kinds := []ActionKind{
		ActionKindMerge,
		ActionKindClose,
		ActionKindComment,
		ActionKindLabel,
		ActionKindRequestChanges,
		ActionKindApplyFix,
	}
	wantKinds := []string{"merge", "close", "comment", "label", "request_changes", "apply_fix"}
	for i, kind := range kinds {
		if string(kind) != wantKinds[i] {
			t.Fatalf("action kind %d = %q, want %q", i, kind, wantKinds[i])
		}
	}
}

func TestActionPlanJSONContract(t *testing.T) {
	t.Parallel()

	preflight := ActionPreflight{
		Check:        "pr_still_open",
		Status:       "passed",
		Reason:       "PR #101 is open",
		EvidenceRefs: []string{"github:pr/101#state"},
		Required:     true,
		CheckedAt:    "2026-04-24T08:02:00Z",
	}
	plan := ActionPlan{
		SchemaVersion: "2.0",
		RunID:         "run-123",
		Repo:          "owner/repo",
		PolicyProfile: PolicyProfileAdvisory,
		GeneratedAt:   "2026-04-24T08:00:00Z",
		CorpusSnapshot: ActionCorpusSnapshot{
			TotalPRs:          1,
			HeadSHAIndexed:    true,
			AnalysisTruncated: false,
			MaxPRsApplied:     0,
		},
		Lanes: []ActionLaneSummary{{
			Lane:        ActionLaneFastMerge,
			Count:       1,
			WorkItemIDs: []string{"wi-101"},
		}},
		WorkItems: []ActionWorkItem{{
			ID:                      "wi-101",
			PRNumber:                101,
			Lane:                    ActionLaneFastMerge,
			State:                   ActionWorkItemStateProposed,
			PriorityScore:           0.91,
			Confidence:              0.95,
			RiskFlags:               []string{"low_risk"},
			ReasonTrail:             []string{"ci_green", "mergeable_clean"},
			EvidenceRefs:            []string{"github:pr/101"},
			RequiredPreflightChecks: []ActionPreflight{preflight},
			IdempotencyKey:          "owner/repo#101:merge:abc123",
			LeaseState: ActionLease{
				ClaimedBy: "worker-1",
				ClaimedAt: "2026-04-24T08:03:00Z",
				ExpiresAt: "2026-04-24T08:33:00Z",
			},
			AllowedActions:  []ActionKind{ActionKindMerge},
			BlockedReasons:  []string{},
			ProofBundleRefs: []string{"proof-101"},
		}},
		ActionIntents: []ActionIntent{{
			ID:             "intent-101-merge",
			Action:         ActionKindMerge,
			PRNumber:       101,
			Lane:           ActionLaneFastMerge,
			DryRun:         true,
			PolicyProfile:  PolicyProfileAdvisory,
			Confidence:     0.95,
			RiskFlags:      []string{"low_risk"},
			Reasons:        []string{"ci_green", "mergeable_clean"},
			EvidenceRefs:   []string{"github:pr/101"},
			Preconditions:  []ActionPreflight{preflight},
			IdempotencyKey: "owner/repo#101:merge:abc123",
			CreatedAt:      "2026-04-24T08:01:00Z",
			Payload:        map[string]any{"merge_method": "squash"},
		}},
		Audit: ActionPlanAudit{
			Checks: []ActionPlanAuditCheck{{
				Name:         "lane_coverage",
				Status:       "passed",
				Reason:       "every PR has one primary action lane",
				EvidenceRefs: []string{"run:run-123"},
				CheckedAt:    "2026-04-24T08:04:00Z",
			}},
		},
	}

	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal action plan: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode action plan: %v", err)
	}
	assertExactJSONKeys(t, decoded, []string{
		"schema_version",
		"run_id",
		"repo",
		"policy_profile",
		"generated_at",
		"corpus_snapshot",
		"lanes",
		"work_items",
		"action_intents",
		"audit",
	})

	corpus, ok := decoded["corpus_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected corpus_snapshot payload: %#v", decoded["corpus_snapshot"])
	}
	assertExactJSONKeys(t, corpus, []string{"total_prs", "head_sha_indexed", "analysis_truncated", "max_prs_applied"})
	if corpus["head_sha_indexed"] != true {
		t.Fatalf("head_sha_indexed = %#v, want true", corpus["head_sha_indexed"])
	}

	workItems, ok := decoded["work_items"].([]any)
	if !ok || len(workItems) != 1 {
		t.Fatalf("unexpected work_items payload: %#v", decoded["work_items"])
	}
	workItem, ok := workItems[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected work item payload: %#v", workItems[0])
	}
	assertExactJSONKeys(t, workItem, []string{
		"id",
		"pr_number",
		"lane",
		"state",
		"priority_score",
		"confidence",
		"risk_flags",
		"reason_trail",
		"evidence_refs",
		"required_preflight_checks",
		"idempotency_key",
		"lease_state",
		"allowed_actions",
		"blocked_reasons",
		"proof_bundle_refs",
	})

	intents, ok := decoded["action_intents"].([]any)
	if !ok || len(intents) != 1 {
		t.Fatalf("unexpected action_intents payload: %#v", decoded["action_intents"])
	}
	intent, ok := intents[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected action intent payload: %#v", intents[0])
	}
	assertExactJSONKeys(t, intent, []string{
		"id",
		"action",
		"pr_number",
		"lane",
		"dry_run",
		"policy_profile",
		"confidence",
		"risk_flags",
		"reasons",
		"evidence_refs",
		"preconditions",
		"idempotency_key",
		"created_at",
		"payload",
	})
	if intent["action"] != string(ActionKindMerge) {
		t.Fatalf("action = %#v, want %q", intent["action"], ActionKindMerge)
	}
	if intent["dry_run"] != true {
		t.Fatalf("dry_run = %#v, want true", intent["dry_run"])
	}
}

func assertExactJSONKeys(t *testing.T, decoded map[string]any, want []string) {
	t.Helper()

	if len(decoded) != len(want) {
		t.Fatalf("keys = %#v, want exactly %#v", decoded, want)
	}
	for _, key := range want {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing key %s from %#v", key, decoded)
		}
	}
}
