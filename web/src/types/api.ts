export interface PR {
  id: string;
  repo: string;
  number: number;
  title: string;
  body: string;
  url: string;
  author: string;
  labels: string[];
  files_changed: string[];
  review_status: string;
  ci_status: string;
  mergeable: string;
  base_branch: string;
  head_branch: string;
  cluster_id: string;
  created_at: string;
  updated_at: string;
  is_draft: boolean;
  is_bot: boolean;
  additions: number;
  deletions: number;
  changed_files_count: number;
  provenance?: Record<string, string>;
  is_collapsed_canonical?: boolean;
  superseded_prs?: number[];
}

export interface PRCluster {
  cluster_id: string;
  cluster_label: string;
  summary: string;
  pr_ids: number[];
  health_status: string;
  average_similarity: number;
  sample_titles: string[];
}

export interface DuplicateGroup {
  canonical_pr_number: number;
  duplicate_pr_numbers: number[];
  similarity: number;
  reason: string;
}

export interface ConflictPair {
  source_pr: number;
  target_pr: number;
  conflict_type: string;
  files_touched: string[];
  severity: string;
  reason: string;
}

export interface PRFile {
  path: string;
  status: string;
  additions: number;
  deletions: number;
  patch?: string;
  previous_path?: string;
}

export interface StalenessReport {
  pr_number: number;
  score: number;
  signals: string[];
  reasons: string[];
  superseded_by: number[];
}

export interface MergePlanCandidate {
  pr_number: number;
  title: string;
  score: number;
  rationale: string;
  files_touched: string[];
  conflict_warnings: string[];
}

export interface MergePlan {
  plan_id: string;
  mode: string;
  formula_expression: string;
  selected: MergePlanCandidate[];
  ordering: MergePlanCandidate[];
  total_score: number;
  warnings: string[];
}

export interface GraphNode {
  pr_number: number;
  title: string;
  cluster_id: string;
  ci_status: string;
}

export interface GraphEdge {
  from_pr: number;
  to_pr: number;
  edge_type: string;
  reason: string;
}

export interface Counts {
  total_prs: number;
  cluster_count: number;
  duplicate_groups: number;
  overlap_groups: number;
  conflict_pairs: number;
  stale_prs: number;
  collapsed_duplicate_groups?: number;
}

export interface CollapsedCorpus {
  canonical_to_superseded?: Record<number, number[]>;
  superseded_to_canonical?: Record<number, number>;
  collapsed_group_count: number;
  total_superseded: number;
}

export interface Thresholds {
  duplicate: number;
  overlap: number;
}

export interface PlanRejection {
  pr_number: number;
  reason: string;
}

export interface OmniPlanStage {
  stage: number;
  stageSize: number;
  matched: number;
  selected: number;
}

export interface OmniPlanResponse {
  repo: string;
  generatedAt: string;
  selector: string;
  mode: string;
  stageCount: number;
  stages: OmniPlanStage[];
  selected: number[];
  ordering: number[];
}

export interface ActionIntent {
  action: string;
  pr_number: number;
  dry_run: boolean;
  created_at: string;
}

export interface ClusterRequest {
  repo: string;
  prs: PR[];
  model: string;
  minClusterSize: number;
}

export interface DuplicateDetectionRequest {
  repo: string;
  prs: PR[];
  duplicateThreshold: number;
  overlapThreshold: number;
}

export interface SemanticAnalysisRequest {
  repo: string;
  prs: PR[];
  analysisMode: string;
}

export interface ClusterResponse {
  repo: string;
  generatedAt: string;
  model: string;
  thresholds: Thresholds;
  clusters: PRCluster[];
}

export interface DuplicateResponse {
  repo: string;
  generatedAt: string;
  duplicates: DuplicateGroup[];
  overlaps: DuplicateGroup[];
}

export interface SemanticConflictResponse {
  repo: string;
  generatedAt: string;
  conflicts: ConflictPair[];
}

export interface AnalysisResponse {
  repo: string;
  generatedAt: string;
  counts: Counts;
  prs: PR[];
  clusters: PRCluster[];
  duplicates: DuplicateGroup[];
  overlaps: DuplicateGroup[];
  conflicts: ConflictPair[];
  stalenessSignals: StalenessReport[];
  collapsed_corpus?: CollapsedCorpus;
}

export interface GraphResponse {
  repo: string;
  generatedAt: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
  dot: string;
}

export interface PlanResponse {
  repo: string;
  generatedAt: string;
  target: number;
  candidatePoolSize: number;
  strategy: string;
  selected: MergePlanCandidate[];
  ordering: MergePlanCandidate[];
  rejections: PlanRejection[];
  collapsed_corpus?: CollapsedCorpus;
}

export interface HealthResponse {
  status: string;
  version: string;
}

export type ReviewCategory = "merge_now" | "merge_after_focused_review" | "duplicate_superseded" | "problematic_quarantine" | "unknown_escalate";

export type PriorityTier = "fast_merge" | "review_required" | "blocked";

export interface CodeLocation {
  file_path: string;
  line_start?: number;
  line_end?: number;
  column_start?: number;
  column_end?: number;
  snippet?: string;
}

export interface DiffHunk {
  old_path: string;
  new_path: string;
  old_start: number;
  old_lines: number;
  new_start: number;
  new_lines: number;
  content: string;
  section?: string;
}

export interface AnalyzerFinding {
  analyzer_name: string;
  analyzer_version: string;
  finding: string;
  confidence: number;
  location?: CodeLocation;
  diff_hunk?: DiffHunk;
  evidence_hash?: string;
}

export interface AnalyzerMetadata {
  name: string;
  version: string;
  category: string;
  confidence: number;
}

export interface DecisionLayer {
  layer: number;
  name: string;
  cost_tier: string;
  bucket: string;
  status: string;
  reasons: string[];
  continued: boolean;
  terminal: boolean;
}

export interface ReviewResult {
  category: ReviewCategory;
  priority_tier: PriorityTier;
  confidence: number;
  reasons: string[];
  decision_layers: DecisionLayer[];
  blockers: string[];
  evidence_references: string[];
  next_action: string;
  analyzer_findings: AnalyzerFinding[];
}

export interface ReviewCategoryCount {
  category: ReviewCategory;
  count: number;
}

export interface PriorityTierCount {
  tier: PriorityTier;
  count: number;
}

export interface BucketCount {
  bucket: string;
  count: number;
}

export interface ReviewResponse {
  total_prs: number;
  reviewed_prs: number;
  categories: ReviewCategoryCount[];
  buckets: BucketCount[];
  risk_buckets: BucketCount[];
  priority_tiers: PriorityTierCount[];
  results: ReviewResult[];
}

export interface AnalysisResponse {
  repo: string;
  generatedAt: string;
  counts: Counts;
  clusters: PRCluster[];
  duplicates: DuplicateGroup[];
  overlaps: DuplicateGroup[];
  conflicts: ConflictPair[];
  stalenessSignals: StalenessReport[];
  review_payload: ReviewResponse;
  collapsed_corpus?: CollapsedCorpus;
}
