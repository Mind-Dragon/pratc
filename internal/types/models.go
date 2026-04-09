package types

// PR is the shared pull request payload exchanged by the CLI, API, ML service, and web UI.
type PR struct {
	ID                string   `json:"id"`
	Repo              string   `json:"repo"`
	Number            int      `json:"number"`
	Title             string   `json:"title"`
	Body              string   `json:"body"`
	URL               string   `json:"url"`
	Author            string   `json:"author"`
	Labels            []string `json:"labels"`
	FilesChanged      []string `json:"files_changed"`
	ReviewStatus      string   `json:"review_status"`
	CIStatus          string   `json:"ci_status"`
	Mergeable         string   `json:"mergeable"`
	BaseBranch        string   `json:"base_branch"`
	HeadBranch        string   `json:"head_branch"`
	ClusterID         string   `json:"cluster_id"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	IsDraft           bool     `json:"is_draft"`
	IsBot             bool     `json:"is_bot"`
	Additions         int      `json:"additions"`
	Deletions         int      `json:"deletions"`
	ChangedFilesCount int      `json:"changed_files_count"`
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

type ConflictPair struct {
	SourcePR     int      `json:"source_pr"`
	TargetPR     int      `json:"target_pr"`
	ConflictType string   `json:"conflict_type"`
	FilesTouched []string `json:"files_touched"`
	Severity     string   `json:"severity"`
	Reason       string   `json:"reason"`
}

type StalenessReport struct {
	PRNumber     int      `json:"pr_number"`
	Score        float64  `json:"score"`
	Signals      []string `json:"signals"`
	Reasons      []string `json:"reasons"`
	SupersededBy []int    `json:"superseded_by"`
}

type MergePlanCandidate struct {
	PRNumber         int      `json:"pr_number"`
	Title            string   `json:"title"`
	Score            float64  `json:"score"`
	Rationale        string   `json:"rationale"`
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
	TotalPRs        int `json:"total_prs"`
	ClusterCount    int `json:"cluster_count"`
	DuplicateGroups int `json:"duplicate_groups"`
	OverlapGroups   int `json:"overlap_groups"`
	ConflictPairs   int `json:"conflict_pairs"`
	StalePRs        int `json:"stale_prs"`
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
	Repo                    string      `json:"repo"`
	GeneratedAt             string      `json:"generatedAt"`
	AnalysisTruncated       bool        `json:"analysis_truncated,omitempty"`
	TruncationReason        string      `json:"truncation_reason,omitempty"`
	MaxPRsApplied           int         `json:"max_prs_applied,omitempty"`
	PRWindow                *PRWindow   `json:"pr_window,omitempty"`
	PrecisionMode           string      `json:"precision_mode,omitempty"`
	DeepCandidateSubsetSize int         `json:"deep_candidate_subset_size,omitempty"`
	Model                   string      `json:"model"`
	Thresholds              Thresholds  `json:"thresholds"`
	Clusters                []PRCluster `json:"clusters"`
}

type DuplicateResponse struct {
	Repo        string           `json:"repo"`
	GeneratedAt string           `json:"generatedAt"`
	Duplicates  []DuplicateGroup `json:"duplicates"`
	Overlaps    []DuplicateGroup `json:"overlaps"`
}

type SemanticConflictResponse struct {
	Repo        string         `json:"repo"`
	GeneratedAt string         `json:"generatedAt"`
	Conflicts   []ConflictPair `json:"conflicts"`
}

type PRWindow struct {
	BeginningPRNumber int `json:"beginning_pr_number,omitempty"`
	EndingPRNumber    int `json:"ending_pr_number,omitempty"`
}

type OperationTelemetry struct {
	PoolStrategy     string         `json:"pool_strategy,omitempty"`
	PoolSizeBefore   int            `json:"pool_size_before,omitempty"`
	PoolSizeAfter    int            `json:"pool_size_after,omitempty"`
	GraphDeltaEdges  int            `json:"graph_delta_edges,omitempty"`
	DecayPolicy      string         `json:"decay_policy,omitempty"`
	PairwiseShards   int            `json:"pairwise_shards,omitempty"`
	StageLatenciesMS map[string]int `json:"stage_latencies_ms,omitempty"`
	StageDropCounts  map[string]int `json:"stage_drop_counts,omitempty"`
}

type AnalysisResponse struct {
	Repo                    string              `json:"repo"`
	GeneratedAt             string              `json:"generatedAt"`
	AnalysisTruncated       bool                `json:"analysis_truncated,omitempty"`
	TruncationReason        string              `json:"truncation_reason,omitempty"`
	MaxPRsApplied           int                 `json:"max_prs_applied,omitempty"`
	PRWindow                *PRWindow           `json:"pr_window,omitempty"`
	PrecisionMode           string              `json:"precision_mode,omitempty"`
	DeepCandidateSubsetSize int                 `json:"deep_candidate_subset_size,omitempty"`
	Counts                  Counts              `json:"counts"`
	PRs                     []PR                `json:"prs"`
	Clusters                []PRCluster         `json:"clusters"`
	Duplicates              []DuplicateGroup    `json:"duplicates"`
	Overlaps                []DuplicateGroup    `json:"overlaps"`
	Conflicts               []ConflictPair      `json:"conflicts"`
	StalenessSignals        []StalenessReport   `json:"stalenessSignals"`
	Telemetry               *OperationTelemetry `json:"telemetry,omitempty"`
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
	Status  string `json:"status"`
	Version string `json:"version"`
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
	// ReviewCategoryMergeSafe indicates the PR is safe to merge with minimal review.
	ReviewCategoryMergeSafe ReviewCategory = "merge_safe"
	// ReviewCategoryDuplicate indicates the PR is a duplicate of another PR.
	ReviewCategoryDuplicate ReviewCategory = "duplicate"
	// ReviewCategoryProblematic indicates the PR has issues that need attention.
	ReviewCategoryProblematic ReviewCategory = "problematic"
	// ReviewCategoryNeedsReview indicates the PR requires standard human review.
	ReviewCategoryNeedsReview ReviewCategory = "needs_review"
)
