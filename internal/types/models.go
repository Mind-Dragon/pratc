package types

// PR is the shared pull request payload exchanged by the CLI, API, ML service, and web UI.
type PR struct {
	ID                string            `json:"id"`
	Repo              string            `json:"repo"`
	Number            int               `json:"number"`
	Title             string            `json:"title"`
	Body              string            `json:"body"`
	URL               string            `json:"url"`
	Author            string            `json:"author"`
	Labels            []string          `json:"labels"`
	FilesChanged      []string          `json:"files_changed"`
	ReviewStatus      string            `json:"review_status"`
	CIStatus          string            `json:"ci_status"`
	Mergeable         string            `json:"mergeable"`
	BaseBranch        string            `json:"base_branch"`
	HeadBranch        string            `json:"head_branch"`
	ClusterID         string            `json:"cluster_id"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
	IsDraft           bool              `json:"is_draft"`
	IsBot             bool              `json:"is_bot"`
	Additions         int               `json:"additions"`
	Deletions         int               `json:"deletions"`
	ChangedFilesCount int               `json:"changed_files_count"`
	ReviewCount       int               `json:"review_count"`
	CommentCount      int               `json:"comment_count"`
	Provenance        map[string]string `json:"provenance,omitempty"`
	// Review fields: populated by the decision engine when IncludeReview is true.
	// These fields are derived from ReviewResult for each PR.
	Confidence     float64         `json:"confidence,omitempty"`
	Reasons        []string        `json:"reasons,omitempty"`
	SubstanceScore int             `json:"substance_score,omitempty"`
	TemporalBucket string          `json:"temporal_bucket,omitempty"`
	DecisionLayers []DecisionLayer `json:"decision_layers,omitempty"`
	Category       ReviewCategory  `json:"category,omitempty"`
	PriorityTier   PriorityTier    `json:"priority_tier,omitempty"`
	// IsCollapsedCanonical is true when this PR is the canonical representative
	// of a flattened duplicate group. SupersededPRs lists the PRs it replaces.
	IsCollapsedCanonical bool  `json:"is_collapsed_canonical,omitempty"`
	SupersededPRs        []int `json:"superseded_prs,omitempty"`
}

type PRCluster struct {
	ClusterID         string   `json:"cluster_id"`
	ClusterLabel      string   `json:"cluster_label"`
	Summary           string   `json:"summary"`
	PRIDs             []int    `json:"pr_ids"`
	HealthStatus      string   `json:"health_status"`
	AverageSimilarity float64  `json:"average_similarity"`
	SampleTitles      []string `json:"sample_titles"`
}

type DuplicateGroup struct {
	CanonicalPRNumber int     `json:"canonical_pr_number"`
	DuplicatePRNums   []int   `json:"duplicate_pr_numbers"`
	Similarity        float64 `json:"similarity"`
	Reason            string  `json:"reason"`
}

type DegradationMetadata struct {
	EmbeddingsUsed    bool   `json:"embeddings_used,omitempty"`
	FallbackReason    string `json:"fallback_reason,omitempty"`
	HeuristicFallback bool   `json:"heuristic_fallback,omitempty"`
}

// DuplicateSynthesisCandidate represents one PR within a duplicate/near-duplicate group
// evaluated for synthesis candidacy. It captures the signals used to rank candidates
// and the outcome of that evaluation.
type DuplicateSynthesisCandidate struct {
	// PRNumber identifies the PR this candidate represents.
	PRNumber int `json:"pr_number"`
	// Title is the PR title for quick reference.
	Title string `json:"title"`
	// Author is the PR author for follow-up routing.
	Author string `json:"author"`
	// Role describes this candidate's role within the group:
	// "canonical" - nominated best-of-group candidate for merge
	// "alternate" - viable alternative, supports canonical
	// "contributor" - partial but incomplete, not standalone material
	// "excluded" - not recommended for synthesis
	Role string `json:"role"`
	// SynthesisScore is a 0.0-1.0 composite score of how well this PR
	// represents the best material from the group for merge-by-bot use.
	// Higher is better.
	SynthesisScore float64 `json:"synthesis_score"`
	// Confidence is the review confidence in this candidate's quality assessment.
	Confidence float64 `json:"confidence"`
	// SubstanceScore is the PR's substance score (0-100).
	SubstanceScore int `json:"substance_score"`
	// Mergeable indicates whether the PR can merge cleanly: "yes", "no", or "unknown".
	Mergeable string `json:"mergeable"`
	// HasTestEvidence is true if analyzer findings include test-gap evidence.
	HasTestEvidence bool `json:"has_test_evidence"`
	// SubsystemTags lists the distinct subsystems implicated by analyzer findings.
	SubsystemTags []string `json:"subsystem_tags,omitempty"`
	// RiskyPatterns lists the distinct risky-pattern signals attached to this candidate.
	RiskyPatterns []string `json:"risky_patterns,omitempty"`
	// ConflictFootprint is the number of conflicts this PR has with other PRs
	// in the corpus. Lower is better for synthesis candidacy.
	ConflictFootprint int `json:"conflict_footprint"`
	// IsDraft is true if the PR is in draft state.
	IsDraft bool `json:"is_draft"`
	// SignalQuality describes the overall signal quality: "high", "medium", "low".
	SignalQuality string `json:"signal_quality"`
	// ScoringFactors breakdown the synthesis score components for transparency.
	ScoringFactors []string `json:"scoring_factors"`
	// Rationale explains why this candidate received its role and score.
	Rationale string `json:"rationale"`
}

// DuplicateSynthesisPlan captures the synthesis recommendation for a single
// duplicate/near-duplicate group. It is advisory-only: it nominates a canonical
// candidate and describes how a future bot could produce a merged artifact.
//
// v1.6 contract: no GitHub mutations are performed based on this artifact.
// It exists to make duplicate group resolution machine-readable and auditable.
type DuplicateSynthesisPlan struct {
	// GroupID is a stable identifier for this group derived from the canonical PR number
	// and the group's similarity threshold classification.
	GroupID string `json:"group_id"`
	// GroupType classifies the group: "duplicate" (≥DuplicateThreshold similarity)
	// or "overlap" (≥OverlapThreshold but below DuplicateThreshold).
	GroupType string `json:"group_type"`
	// OriginalCanonicalPR is the PR number that was originally selected as canonical
	// by the pair-ordering heuristic in classifyDuplicates.
	OriginalCanonicalPR int `json:"original_canonical_pr"`
	// NominatedCanonicalPR is the PR number nominated by synthesis scoring as the best
	// candidate for merge-by-bot use. May differ from OriginalCanonicalPR.
	NominatedCanonicalPR int `json:"nominated_canonical_pr"`
	// Similarity is the group's pairwise similarity score.
	Similarity float64 `json:"similarity"`
	// Reason is the human-readable reason for the group relationship.
	Reason string `json:"reason"`
	// Candidates contains all PRs evaluated in this group, ranked by synthesis score.
	Candidates []DuplicateSynthesisCandidate `json:"candidates"`
	// SynthesisNotes contains free-text guidance for a future merge bot,
	// including which alternates to preserve, what conflicts need resolution,
	// and what test coverage gaps exist.
	SynthesisNotes []string `json:"synthesis_notes"`
}

// GarbagePR represents a PR classified as junk by the outer peel (Layer 1).
type GarbagePR struct {
	PRNumber int    `json:"pr_number"`
	Reason   string `json:"reason"`
}

type ConflictPair struct {
	SourcePR     int      `json:"source_pr"`
	TargetPR     int      `json:"target_pr"`
	ConflictType string   `json:"conflict_type"`
	FilesTouched []string `json:"files_touched"`
	Severity     string   `json:"severity"`
	Reason       string   `json:"reason"`
}

// PRFile represents a single file changed in a pull request.
// It captures metadata about the change including additions, deletions, and patch content.
type PRFile struct {
	// Path is the file path relative to the repository root.
	Path string `json:"path"`
	// Status is the change status: "added", "removed", "modified", or "renamed".
	Status string `json:"status"`
	// Additions is the number of lines added in this file.
	Additions int `json:"additions"`
	// Deletions is the number of lines deleted in this file.
	Deletions int `json:"deletions"`
	// Patch is the unified diff patch for this file, if available.
	Patch string `json:"patch,omitempty"`
	// PreviousPath is the previous file path for renames, if applicable.
	PreviousPath string `json:"previous_path,omitempty"`
}

type StalenessReport struct {
	PRNumber     int      `json:"pr_number"`
	Score        float64  `json:"score"`
	Signals      []string `json:"signals"`
	Reasons      []string `json:"reasons"`
	SupersededBy []int    `json:"superseded_by"`
}

// CollapsedCorpus maps canonical PRs to their full superseded sets after
// chain-flattening of duplicate and overlap groups. It is built from
// DuplicateSynthesisPlan results and used by the planner to replace
// CollapsedCorpus maps canonical PRs to their full superseded sets after chain flattening.
type CollapsedCorpus struct {
	// CanonicalToSuperseded maps each canonical PR number to the PRs it supersedes.
	CanonicalToSuperseded map[int][]int `json:"canonical_to_superseded,omitempty"`
	// SupersededToCanonical maps each superseded PR back to its canonical.
	SupersededToCanonical map[int]int `json:"superseded_to_canonical,omitempty"`
	// CollapsedGroupCount is the number of groups after collapse.
	CollapsedGroupCount int `json:"collapsed_group_count"`
	// TotalSuperseded is the total number of PRs replaced by canonicals.
	TotalSuperseded int `json:"total_superseded"`
}

type MergePlanCandidate struct {
	PRNumber         int      `json:"pr_number"`
	Title            string   `json:"title"`
	Score            float64  `json:"score"`
	Rationale        string   `json:"rationale"`
	Reasons          []string `json:"reasons"`
	FilesTouched     []string `json:"files_touched"`
	ConflictWarnings []string `json:"conflict_warnings"`
}

type MergePlan struct {
	PlanID            string               `json:"plan_id"`
	Mode              string               `json:"mode"`
	FormulaExpression string               `json:"formula_expression"`
	Selected          []MergePlanCandidate `json:"selected"`
	Ordering          []MergePlanCandidate `json:"ordering"`
	TotalScore        float64              `json:"total_score"`
	Warnings          []string             `json:"warnings"`
}

type GraphNode struct {
	PRNumber  int    `json:"pr_number"`
	Title     string `json:"title"`
	ClusterID string `json:"cluster_id"`
	CIStatus  string `json:"ci_status"`
}

type GraphEdge struct {
	FromPR   int    `json:"from_pr"`
	ToPR     int    `json:"to_pr"`
	EdgeType string `json:"edge_type"`
	Reason   string `json:"reason"`
}

type Counts struct {
	TotalPRs                 int `json:"total_prs"`
	ClusterCount             int `json:"cluster_count"`
	DuplicateGroups          int `json:"duplicate_groups"`
	OverlapGroups            int `json:"overlap_groups"`
	ConflictPairs            int `json:"conflict_pairs"`
	StalePRs                 int `json:"stale_prs"`
	GarbagePRs               int `json:"garbage_prs"`
	CollapsedDuplicateGroups int `json:"collapsed_duplicate_groups"`
	CandidatePoolSize        int `json:"candidate_pool_size,omitempty"`
}

type Thresholds struct {
	Duplicate float64 `json:"duplicate"`
	Overlap   float64 `json:"overlap"`
}

type PlanRejection struct {
	PRNumber int    `json:"pr_number"`
	Reason   string `json:"reason"`
}

type ActionIntent struct {
	Action    string `json:"action"`
	PRNumber  int    `json:"pr_number"`
	DryRun    bool   `json:"dry_run"`
	CreatedAt string `json:"created_at"`
}

type ClusterRequest struct {
	Repo           string `json:"repo"`
	PRs            []PR   `json:"prs"`
	Model          string `json:"model"`
	MinClusterSize int    `json:"minClusterSize"`
}

type DuplicateDetectionRequest struct {
	Repo               string  `json:"repo"`
	PRs                []PR    `json:"prs"`
	DuplicateThreshold float64 `json:"duplicateThreshold"`
	OverlapThreshold   float64 `json:"overlapThreshold"`
}

type SemanticAnalysisRequest struct {
	Repo         string `json:"repo"`
	PRs          []PR   `json:"prs"`
	AnalysisMode string `json:"analysisMode"`
}

type ClusterResponse struct {
	Repo                    string               `json:"repo"`
	GeneratedAt             string               `json:"generatedAt"`
	AnalysisTruncated       bool                 `json:"analysis_truncated,omitempty"`
	TruncationReason        string               `json:"truncation_reason,omitempty"`
	MaxPRsApplied           int                  `json:"max_prs_applied,omitempty"`
	PRWindow                *PRWindow            `json:"pr_window,omitempty"`
	PrecisionMode           string               `json:"precision_mode,omitempty"`
	DeepCandidateSubsetSize int                  `json:"deep_candidate_subset_size,omitempty"`
	Model                   string               `json:"model"`
	Thresholds              Thresholds           `json:"thresholds"`
	Degradation             *DegradationMetadata `json:"degradation,omitempty"`
	Clusters                []PRCluster          `json:"clusters"`
}

type DuplicateResponse struct {
	Repo        string               `json:"repo"`
	GeneratedAt string               `json:"generatedAt"`
	Degradation *DegradationMetadata `json:"degradation,omitempty"`
	Duplicates  []DuplicateGroup     `json:"duplicates"`
	Overlaps    []DuplicateGroup     `json:"overlaps"`
}

type SemanticConflictResponse struct {
	Repo        string         `json:"repo"`
	GeneratedAt string         `json:"generatedAt"`
	Conflicts   []ConflictPair `json:"conflicts"`
}

type PRWindow struct {
	BeginningPRNumber int `json:"beginning_pr_number,omitempty"`
	EndingPRNumber    int `json:"ending_pr_number,omitempty"`
	SnapshotCeiling   int `json:"snapshot_ceiling,omitempty"`
}

type OperationTelemetry struct {
	PoolStrategy                    string         `json:"pool_strategy,omitempty"`
	PlanningStrategy                string         `json:"planning_strategy,omitempty"`
	PoolSizeBefore                  int            `json:"pool_size_before,omitempty"`
	PoolSizeAfter                   int            `json:"pool_size_after,omitempty"`
	GraphDeltaEdges                 int            `json:"graph_delta_edges,omitempty"`
	DecayPolicy                     string         `json:"decay_policy,omitempty"`
	PairwiseShards                  int            `json:"pairwise_shards,omitempty"`
	PairwiseEarlyExits              int            `json:"pairwise_early_exits,omitempty"`
	PairwiseWorkersActive           int            `json:"pairwise_workers_active,omitempty"`
	HierarchicalComplexityReduction float64        `json:"hierarchical_complexity_reduction,omitempty"`
	StageLatenciesMS                map[string]int `json:"stage_latencies_ms,omitempty"`
	StageDropCounts                 map[string]int `json:"stage_drop_counts,omitempty"`
}

type AnalysisResponse struct {
	Repo                    string               `json:"repo"`
	GeneratedAt             string               `json:"generatedAt"`
	AnalysisTruncated       bool                 `json:"analysis_truncated,omitempty"`
	TruncationReason        string               `json:"truncation_reason,omitempty"`
	MaxPRsApplied           int                  `json:"max_prs_applied,omitempty"`
	PRWindow                *PRWindow            `json:"pr_window,omitempty"`
	PrecisionMode           string               `json:"precision_mode,omitempty"`
	DeepCandidateSubsetSize int                  `json:"deep_candidate_subset_size,omitempty"`
	Counts                  Counts               `json:"counts"`
	PRs                     []PR                 `json:"prs"`
	Clusters                []PRCluster          `json:"clusters"`
	ClusterModel            string               `json:"cluster_model,omitempty"`
	ClusterDegradation      *DegradationMetadata `json:"cluster_degradation,omitempty"`
	DuplicateDegradation    *DegradationMetadata `json:"duplicate_degradation,omitempty"`
	Duplicates              []DuplicateGroup     `json:"duplicates"`
	Overlaps                []DuplicateGroup     `json:"overlaps"`
	Conflicts               []ConflictPair       `json:"conflicts"`
	StalenessSignals        []StalenessReport    `json:"stalenessSignals"`
	// GarbagePRs contains PRs classified as junk by the outer peel (Layer 1).
	// These PRs should be closed and do not enter the duplicate, conflict, or review pipeline.
	GarbagePRs []GarbagePR         `json:"garbagePRs,omitempty"`
	Telemetry  *OperationTelemetry `json:"telemetry,omitempty"`
	// ReviewPayload contains agentic review results for the analysis snapshot.
	// v1.3 pipelines populate this field by default so review buckets are first-class
	// output in the primary API and report surfaces.
	// The pointer is retained for compatibility with older callers.
	ReviewPayload *ReviewResponse `json:"review_payload,omitempty"`
	// DuplicateSynthesis contains synthesis plans for duplicate and near-duplicate groups.
	// Each plan nominates a canonical candidate and describes how a future bot could
	// produce a merged artifact from the group. This is advisory-only: no GitHub
	// mutations are performed based on this field.
	DuplicateSynthesis []DuplicateSynthesisPlan `json:"duplicate_synthesis,omitempty"`
	// CollapsedCorpus is the chain-flattened duplicate group mapping used by the planner.
	CollapsedCorpus CollapsedCorpus `json:"collapsed_corpus,omitempty"`
}

type GraphResponse struct {
	Repo        string              `json:"repo"`
	GeneratedAt string              `json:"generatedAt"`
	Nodes       []GraphNode         `json:"nodes"`
	Edges       []GraphEdge         `json:"edges"`
	DOT         string              `json:"dot"`
	Telemetry   *OperationTelemetry `json:"telemetry,omitempty"`
}

type PlanResponse struct {
	Repo                    string               `json:"repo"`
	GeneratedAt             string               `json:"generatedAt"`
	AnalysisTruncated       bool                 `json:"analysis_truncated,omitempty"`
	TruncationReason        string               `json:"truncation_reason,omitempty"`
	MaxPRsApplied           int                  `json:"max_prs_applied,omitempty"`
	PRWindow                *PRWindow            `json:"pr_window,omitempty"`
	PrecisionMode           string               `json:"precision_mode,omitempty"`
	DeepCandidateSubsetSize int                  `json:"deep_candidate_subset_size,omitempty"`
	Target                  int                  `json:"target"`
	CandidatePoolSize       int                  `json:"candidatePoolSize"`
	Strategy                string               `json:"strategy"`
	Selected                []MergePlanCandidate `json:"selected"`
	Ordering                []MergePlanCandidate `json:"ordering"`
	Rejections              []PlanRejection      `json:"rejections"`
	Telemetry               *OperationTelemetry  `json:"telemetry,omitempty"`
	// CollapsedCorpus is set when duplicate collapse was applied during planning.
	CollapsedCorpus *CollapsedCorpus `json:"collapsed_corpus,omitempty"`
}

// OmniPlanStage represents one processing stage in omni-batch mode.
type OmniPlanStage struct {
	Stage     int `json:"stage"`
	StageSize int `json:"stageSize"`
	Matched   int `json:"matched"`
	Selected  int `json:"selected"`
}

// OmniPlanResponse is the response for the omni-batch plan endpoint.
type OmniPlanResponse struct {
	Repo        string          `json:"repo"`
	GeneratedAt string          `json:"generatedAt"`
	Selector    string          `json:"selector"`
	Mode        string          `json:"mode"`
	StageCount  int             `json:"stageCount"`
	Stages      []OmniPlanStage `json:"stages"`
	Selected    []int           `json:"selected"`
	Ordering    []int           `json:"ordering"`
}

type HealthResponse struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	APIVersion string `json:"api_version"`
}

type AuditEntryResponse struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Repo      string `json:"repo"`
	Details   string `json:"details"`
}

type AuditResponse struct {
	GeneratedAt string               `json:"generatedAt"`
	Entries     []AuditEntryResponse `json:"entries"`
	Count       int                  `json:"count"`
}

// ReviewCategory represents the classification of a PR review for agentic review systems.
// It categorizes PRs based on their readiness for merge and potential issues.
type ReviewCategory string

const (
	// ReviewCategoryMergeNow indicates the PR is safe to merge with minimal review.
	ReviewCategoryMergeNow ReviewCategory = "merge_now"
	// ReviewCategoryMergeAfterFocusedReview indicates the PR requires focused review before merge.
	ReviewCategoryMergeAfterFocusedReview ReviewCategory = "merge_after_focused_review"
	// ReviewCategoryDuplicateSuperseded indicates the PR is a duplicate or superseded by another PR.
	ReviewCategoryDuplicateSuperseded ReviewCategory = "duplicate_superseded"
	// ReviewCategoryProblematicQuarantine indicates the PR has issues and should be quarantined.
	ReviewCategoryProblematicQuarantine ReviewCategory = "problematic_quarantine"
	// ReviewCategoryUnknownEscalate indicates the PR needs escalation due to insufficient evidence.
	ReviewCategoryUnknownEscalate ReviewCategory = "unknown_escalate"
)

// PriorityTier represents the urgency/priority level for PR review and merge.
// It helps triage PRs based on their readiness and business priority.
type PriorityTier string

const (
	// PriorityTierFastMerge indicates the PR is ready for immediate merge
	// (e.g., hotfixes, trivial changes, already-approved PRs).
	PriorityTierFastMerge PriorityTier = "fast_merge"
	// PriorityTierReviewRequired indicates the PR needs standard review
	// before merge (normal workflow PRs).
	PriorityTierReviewRequired PriorityTier = "review_required"
	// PriorityTierBlocked indicates the PR has blockers preventing merge
	// (e.g., conflicts, failing CI, requires rebase).
	PriorityTierBlocked PriorityTier = "blocked"
)

// ReviewCategoryCount tracks the count of PRs in a specific review category.
type ReviewCategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// BucketCount tracks the count of PRs in an operator-facing review bucket.
type BucketCount struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

// PriorityTierCount tracks the count of PRs in a specific priority tier.
type PriorityTierCount struct {
	Tier  string `json:"tier"`
	Count int    `json:"count"`
}

// ReviewResponse aggregates review results for all analyzed PRs.
// It provides categorized counts and priority tier distributions for agentic review systems.
type ReviewResponse struct {
	// TotalPRs is the total number of PRs included in this review.
	TotalPRs int `json:"total_prs"`
	// ReviewedPRs is the number of PRs that were successfully reviewed.
	ReviewedPRs int `json:"reviewed_prs"`
	// Categories contains counts of PRs by review category (merge_safe, duplicate, problematic, needs_review).
	Categories []ReviewCategoryCount `json:"categories"`
	// Buckets contains counts of PRs by operator-facing review buckets:
	// "now", "future", "duplicate", "junk", "blocked".
	Buckets []BucketCount `json:"buckets"`
	// RiskBuckets contains counts of PRs by risk buckets:
	// "security_risk", "reliability_risk", "performance_risk".
	RiskBuckets []BucketCount `json:"risk_buckets"`
	// PriorityTiers contains counts of PRs by priority tier (fast_merge, review_required, blocked).
	PriorityTiers []PriorityTierCount `json:"priority_tiers"`
	// Results contains individual review results for each PR.
	Results []ReviewResult `json:"results"`

	// Degradation markers (Option 2: partial-success with explicit errors).
	// Partial indicates that the review did not complete fully.
	Partial bool `json:"partial,omitempty"`
	// Errors contains top-level error messages from the review pipeline.
	Errors []string `json:"errors,omitempty"`
	// FailedPRs lists PR numbers that failed individual review.
	FailedPRs []int `json:"failed_prs,omitempty"`
}

// CodeLocation references a specific location in a source file.
// Used to pinpoint where an analyzer finding applies within the codebase.
type CodeLocation struct {
	// FilePath is the relative path to the file within the repository.
	FilePath string `json:"file_path"`
	// LineStart is the starting line number (1-indexed) of the relevant code.
	LineStart int `json:"line_start,omitempty"`
	// LineEnd is the ending line number (1-indexed) of the relevant code.
	// If only a single line is relevant, LineEnd equals LineStart.
	LineEnd int `json:"line_end,omitempty"`
	// ColumnStart is the starting column position (1-indexed), if known.
	ColumnStart int `json:"column_start,omitempty"`
	// ColumnEnd is the ending column position, if known.
	ColumnEnd int `json:"column_end,omitempty"`
	// Snippet is a short excerpt of the relevant code (up to ~200 chars).
	Snippet string `json:"snippet,omitempty"`
}

// DiffHunk represents a single hunk from a unified diff.
// It captures the before/after state of a code change with line numbers.
type DiffHunk struct {
	// OldPath is the file path before the change (e.g., "a/path/to/file.go").
	OldPath string `json:"old_path"`
	// NewPath is the file path after the change (e.g., "b/path/to/file.go").
	NewPath string `json:"new_path"`
	// OldStart is the starting line number in the old file (1-indexed).
	OldStart int `json:"old_start"`
	// OldLines is the number of lines in the old file hunk.
	OldLines int `json:"old_lines"`
	// NewStart is the starting line number in the new file (1-indexed).
	NewStart int `json:"new_start"`
	// NewLines is the number of lines in the new file hunk.
	NewLines int `json:"new_lines"`
	// Content is the actual diff content including +/- prefixes.
	Content string `json:"content"`
	// Section is the optional function/context header (e.g., "@@ -10,5 +10,7 @@ func Foo()").
	Section string `json:"section,omitempty"`
}

// AnalyzerFinding represents a single finding from an analyzer in the agentic review system.
// It captures the analyzer's output with version information for traceability.
type AnalyzerFinding struct {
	// AnalyzerName is the unique identifier for the analyzer that produced this finding.
	AnalyzerName string `json:"analyzer_name"`
	// AnalyzerVersion is the semantic version of the analyzer for reproducibility.
	AnalyzerVersion string `json:"analyzer_version"`
	// Finding is the human-readable description of what the analyzer discovered.
	Finding string `json:"finding"`
	// Confidence is the analyzer's confidence in this finding, ranging from 0.0 to 1.0.
	Confidence float64 `json:"confidence"`
	// Subsystem classifies which product or code subsystem the finding belongs to.
	Subsystem string `json:"subsystem,omitempty"`
	// SignalType classifies the evidence shape (e.g. risky_pattern, subsystem_tag, test_gap).
	SignalType string `json:"signal_type,omitempty"`
	// Location points to the specific code location this finding relates to, if applicable.
	Location *CodeLocation `json:"location,omitempty"`
	// DiffHunk contains the diff context for this finding, if available.
	DiffHunk *DiffHunk `json:"diff_hunk,omitempty"`
	// EvidenceHash is a SHA-256 hash of the evidence used for this finding (for deduplication).
	EvidenceHash string `json:"evidence_hash,omitempty"`
}

// DecisionLayer represents one gate in the 16-layer PR review funnel.
// It captures the gate number, name, cost tier, outcome, and whether
// the PR continued inward or exited the funnel at this gate.
//
// Gate journey semantics:
//   - Every PR starts at Gate 1 and progresses through Gate 16 in order.
//   - A PR may exit the funnel early at a terminal gate (junk, duplicate, blocked).
//   - Gates after an early exit have Continued=false and Terminal=false.
//   - The terminal gate has Terminal=true.
//   - PRs that pass all gates have Terminal=true at Gate 16.
type DecisionLayer struct {
	// Layer is the gate number, from 1 to 16.
	Layer int `json:"layer"`
	// Name is the human-readable gate name from GUIDELINE.md.
	Name string `json:"name"`
	// CostTier classifies the computational cost of this gate:
	// "cheap" for outer peel gates (1-3),
	// "medium" for substance gates (4-5),
	// "expensive" for deep judgment gates (6-16).
	CostTier string `json:"cost_tier"`
	// Bucket names the visible bucket or outcome associated with the gate, when relevant.
	Bucket string `json:"bucket,omitempty"`
	// Status describes whether the gate observed/clear/peeled/routed/judged the PR.
	Status string `json:"status"`
	// Reasons records the reason trail for this gate.
	Reasons []string `json:"reasons"`
	// Continued indicates the PR moved inward to the next gate.
	// false when the PR exited at this gate (Terminal=true) or after an early exit.
	Continued bool `json:"continued"`
	// Terminal indicates the PR exited the funnel at this gate.
	// When true, Continued is false and this gate records the exit reason.
	Terminal bool `json:"terminal"`
}

// ReviewResult represents the outcome of an agentic PR review.
// It aggregates findings from multiple analyzers to produce a final classification
// and priority recommendation for the PR.
type ReviewResult struct {
	// PRNumber identifies the PR this review result applies to.
	PRNumber int `json:"pr_number"`
	// Title carries the PR title for report surfaces and offline review packets.
	Title string `json:"title,omitempty"`
	// Author carries the PR author for analyst tables.
	Author string `json:"author,omitempty"`
	// ClusterID links the PR back to its cluster, when present.
	ClusterID string `json:"cluster_id,omitempty"`
	// ProblemType refines problematic classifications into concrete buckets like spam/broken/low_quality.
	ProblemType string `json:"problem_type,omitempty"`
	// Category classifies the PR based on its readiness for merge and potential issues.
	Category ReviewCategory `json:"category"`
	// PriorityTier indicates the urgency level for reviewing and merging this PR.
	PriorityTier PriorityTier `json:"priority_tier"`
	// Confidence is the overall confidence in this review result, ranging from 0.0 to 1.0.
	Confidence float64 `json:"confidence"`
	// Reasons is a list of reason codes explaining why this classification was assigned.
	Reasons []string `json:"reasons"`
	// DecisionLayers records the 16-layer decision ladder for this PR.
	DecisionLayers []DecisionLayer `json:"decision_layers,omitempty"`
	// Blockers lists the blocking issues that prevent merge or require follow-up.
	Blockers []string `json:"blockers"`
	// EvidenceReferences points to the evidence used to produce this review result.
	EvidenceReferences []string `json:"evidence_references"`
	// NextAction describes the next human action recommended for this PR.
	NextAction string `json:"next_action"`
	// SubstanceScore is a 0-100 composite score indicating the PR's overall substance.
	// Higher scores indicate more substance (file depth, test coverage, freshness, clean findings).
	SubstanceScore int `json:"substance_score"`
	// TemporalBucket assigns the PR to now/future/blocked based on substance score and readiness.
	TemporalBucket string `json:"temporal_bucket"`
	// Deep judgment layers (6-16)
	// BlastRadius: how much damage if this goes wrong (low/medium/high)
	BlastRadius string `json:"blast_radius,omitempty"`
	// Leverage: 0-1 score of how much other work this unblocks
	Leverage float64 `json:"leverage,omitempty"`
	// HasOwner: true if the author is active and the PR is not abandoned
	HasOwner bool `json:"has_owner"`
	// Mergeable: from GitHub API mergeable field
	Mergeable string `json:"mergeable,omitempty"`
	// StrategicWeight: 0-1 score of how much this moves the project forward
	StrategicWeight float64 `json:"strategic_weight,omitempty"`
	// AttentionCost: how expensive this PR is to review (low/medium/high)
	AttentionCost string `json:"attention_cost,omitempty"`
	// Reversible: true if the PR only touches tests, docs, or config
	Reversible bool `json:"reversible"`
	// SignalQuality: noise or signal based on composite of other layers
	SignalQuality string `json:"signal_quality,omitempty"`
	// AnalyzerFindings contains detailed output from each analyzer that contributed to this result.
	AnalyzerFindings []AnalyzerFinding `json:"analyzer_findings"`
	// ReclassifiedFrom records the original category if this PR was reclassified by the second-pass recovery.
	ReclassifiedFrom string `json:"reclassified_from,omitempty"`
	// ReclassificationReason describes why the PR was reclassified in the second-pass recovery.
	ReclassificationReason string `json:"reclassification_reason,omitempty"`
	// BatchTag is an optional tag for batch operations and tracking.
	BatchTag string `json:"batch_tag,omitempty"`
}

// AnalyzerMetadata provides metadata about an analyzer in the agentic review system.
// It describes the analyzer's identity, version, category, and confidence level.
type AnalyzerMetadata struct {
	// Name is the unique identifier for the analyzer.
	Name string `json:"name"`
	// Version is the semantic version of the analyzer for reproducibility.
	Version string `json:"version"`
	// Category classifies the analyzer by its purpose (security, reliability, performance, quality).
	Category string `json:"category"`
	// Confidence is the analyzer's confidence scale, ranging from 0.0 to 1.0.
	Confidence float64 `json:"confidence"`
}

// Shared constants for the codebase.
const (
	// PairwiseShardSize is the number of PRs processed per shard in pairwise comparison.
	PairwiseShardSize = 256

	// Deprecated: DefaultPoolCap is a legacy documentation-only cap retained for
	// backward reference. The active filter.Pipeline.BuildCandidatePool runtime
	// path does not enforce it; only explicit callers of filter.CapPool can apply
	// a hard limit.
	DefaultPoolCap = 64

	// SLO thresholds in milliseconds.
	AnalyzeSLOMS = 300000 // 300 seconds
	ClusterSLOMS = 180000 // 180 seconds
	GraphSLOMS   = 120000 // 120 seconds
	PlanSLOMS    = 90000  // 90 seconds
)
