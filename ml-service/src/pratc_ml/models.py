from __future__ import annotations

import json
import os
from dataclasses import MISSING, asdict, dataclass, field, fields, is_dataclass
from typing import Any, TypeVar, get_args, get_origin

try:
    from pydantic import BaseModel, ConfigDict
except Exception:  # pragma: no cover - bootstrap fallback until pyproject lands
    BaseModel = None
    ConfigDict = dict


if BaseModel is not None:

    class PR(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        id: str
        repo: str
        number: int
        title: str
        body: str
        url: str
        author: str
        labels: list[str]
        files_changed: list[str]
        review_status: str
        ci_status: str
        mergeable: str
        base_branch: str
        head_branch: str
        cluster_id: str
        created_at: str
        updated_at: str
        is_draft: bool
        is_bot: bool
        additions: int
        deletions: int
        changed_files_count: int
        review_count: int
        comment_count: int


    class PRCluster(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        cluster_id: str
        cluster_label: str
        summary: str
        pr_ids: list[int]
        health_status: str
        average_similarity: float
        sample_titles: list[str]


    class DuplicateGroup(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        canonical_pr_number: int
        duplicate_pr_numbers: list[int]
        similarity: float
        reason: str


    class GarbagePR(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        pr_number: int
        reason: str


    class ConflictPair(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        source_pr: int
        target_pr: int
        conflict_type: str
        files_touched: list[str]
        severity: str
        reason: str


    class StalenessReport(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        pr_number: int
        score: float
        signals: list[str]
        reasons: list[str]
        superseded_by: list[int]


    class MergePlanCandidate(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        pr_number: int
        title: str
        score: float
        rationale: str
        files_touched: list[str]
        conflict_warnings: list[str]


    class MergePlan(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        plan_id: str
        mode: str
        formula_expression: str
        selected: list[MergePlanCandidate]
        ordering: list[MergePlanCandidate]
        total_score: float
        warnings: list[str]


    class GraphNode(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        pr_number: int
        title: str
        cluster_id: str
        ci_status: str


    class GraphEdge(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        from_pr: int
        to_pr: int
        edge_type: str
        reason: str


    class Counts(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        total_prs: int
        cluster_count: int
        duplicate_groups: int
        overlap_groups: int
        conflict_pairs: int
        stale_prs: int
        garbage_prs: int = 0
        collapsed_duplicate_groups: int = 0


    class CollapsedCorpus(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        canonical_to_superseded: dict[int, list[int]] | None = None
        superseded_to_canonical: dict[int, int] | None = None
        collapsed_group_count: int = 0
        total_superseded: int = 0


    class DuplicateSynthesisCandidate(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        pr_number: int
        title: str
        author: str
        role: str
        synthesis_score: float
        confidence: float = 0.0
        substance_score: int = 0
        mergeable: str = "unknown"
        has_test_evidence: bool = False
        subsystem_tags: list[str] = []
        risky_patterns: list[str] = []
        conflict_footprint: int = 0
        is_draft: bool = False
        signal_quality: str = "medium"
        scoring_factors: list[str] = []
        rationale: str = ""


    class DuplicateSynthesisPlan(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        group_id: str
        group_type: str
        original_canonical_pr: int
        nominated_canonical_pr: int
        similarity: float
        reason: str
        candidates: list[DuplicateSynthesisCandidate] = []
        synthesis_notes: list[str] = []


    class Thresholds(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        duplicate: float
        overlap: float


    class PlanRejection(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        pr_number: int
        reason: str


    class ActionPreflight(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        check: str
        status: str
        reason: str
        evidence_refs: list[str] | None = None
        required: bool = False
        checked_at: str | None = None


    class ProofBundle(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        id: str
        work_item_id: str
        pr_number: int
        summary: str
        evidence_refs: list[str] | None = None
        artifact_refs: list[str] | None = None
        test_commands: list[str] | None = None
        test_results: list[str] | None = None
        created_by: str | None = None
        created_at: str | None = None


    class ActionLease(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        claimed_by: str | None = None
        claimed_at: str | None = None
        expires_at: str | None = None


    class ActionWorkItem(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        id: str
        pr_number: int
        lane: str
        state: str
        priority_score: float
        confidence: float
        risk_flags: list[str] | None = None
        reason_trail: list[str] | None = None
        evidence_refs: list[str] | None = None
        required_preflight_checks: list[ActionPreflight] | None = None
        idempotency_key: str = ""
        lease_state: ActionLease | None = None
        allowed_actions: list[str] | None = None
        blocked_reasons: list[str] | None = None
        proof_bundle_refs: list[str] | None = None


    class ActionIntent(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        id: str = ""
        action: str
        pr_number: int
        lane: str = ""
        dry_run: bool
        policy_profile: str = "advisory"
        confidence: float = 0.0
        risk_flags: list[str] | None = None
        reasons: list[str] | None = None
        evidence_refs: list[str] | None = None
        preconditions: list[ActionPreflight] | None = None
        idempotency_key: str = ""
        created_at: str
        payload: dict[str, Any] | None = None


    class ActionLaneSummary(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        lane: str
        count: int
        work_item_ids: list[str] | None = None


    class ActionCorpusSnapshot(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        total_prs: int = 0
        head_sha_indexed: bool = False
        analysis_truncated: bool = False
        max_prs_applied: int = 0


    class ActionPlanAuditCheck(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        name: str
        status: str
        reason: str | None = None
        evidence_refs: list[str] | None = None
        checked_at: str | None = None


    class ActionPlanAudit(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        checks: list[ActionPlanAuditCheck] = []
        warnings: list[str] | None = None
        errors: list[str] | None = None


    class ActionPlan(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        schema_version: str = ""
        run_id: str = ""
        repo: str
        policy_profile: str = "advisory"
        generated_at: str = ""
        corpus_snapshot: ActionCorpusSnapshot | None = None
        lanes: list[ActionLaneSummary] = []
        work_items: list[ActionWorkItem] = []
        action_intents: list[ActionIntent] = []
        audit: ActionPlanAudit | None = None


    class ClusterRequest(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        prs: list[PR]
        model: str
        minClusterSize: int


    class DuplicateDetectionRequest(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        prs: list[PR]
        duplicateThreshold: float
        overlapThreshold: float


    class SemanticAnalysisRequest(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        prs: list[PR]
        analysisMode: str


    class ClusterResponse(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        generatedAt: str
        model: str
        thresholds: Thresholds
        clusters: list[PRCluster]


    class DuplicateResponse(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        generatedAt: str
        duplicates: list[DuplicateGroup]
        overlaps: list[DuplicateGroup]


    class SemanticConflictResponse(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        generatedAt: str
        conflicts: list[ConflictPair]


    class AnalysisResponse(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        generatedAt: str
        counts: Counts
        prs: list[PR]
        clusters: list[PRCluster]
        duplicates: list[DuplicateGroup]
        overlaps: list[DuplicateGroup]
        conflicts: list[ConflictPair]
        stalenessSignals: list[StalenessReport]
        garbagePRs: list[GarbagePR] | None = None
        collapsed_corpus: CollapsedCorpus | None = None
        duplicate_synthesis: list[DuplicateSynthesisPlan] | None = None


    class GraphResponse(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        generatedAt: str
        nodes: list[GraphNode]
        edges: list[GraphEdge]
        dot: str


    class PlanResponse(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        repo: str
        generatedAt: str
        target: int
        candidatePoolSize: int
        strategy: str
        selected: list[MergePlanCandidate]
        ordering: list[MergePlanCandidate]
        rejections: list[PlanRejection]
        collapsed_corpus: CollapsedCorpus | None = None


    class HealthResponse(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        status: str
        version: str


else:
    T = TypeVar("T")

    class _BootstrapModel:
        def model_dump(self) -> dict[str, Any]:
            return _dump(self)

        @classmethod
        def model_validate(cls: type[T], value: Any) -> T:
            return _coerce_dataclass(cls, value)


    def _coerce_value(annotation: Any, value: Any) -> Any:
        origin = get_origin(annotation)
        if origin is list:
            inner = get_args(annotation)[0]
            return [_coerce_value(inner, item) for item in value]
        if isinstance(annotation, type) and is_dataclass(annotation):
            return _coerce_dataclass(annotation, value)
        return value


    def _coerce_dataclass(cls: type[T], value: Any) -> T:
        kwargs = {}
        for item in fields(cls):
            if item.name in value:
                kwargs[item.name] = _coerce_value(item.type, value[item.name])
            else:
                # Field is missing - use default if available
                if item.default_factory is not MISSING:
                    kwargs[item.name] = item.default_factory()
                elif item.default is not MISSING:
                    kwargs[item.name] = item.default
                else:
                    # No default available - raise KeyError
                    raise KeyError(item.name)
        return cls(**kwargs)


    def _dump(value: Any) -> Any:
        if is_dataclass(value):
            return {key: _dump(item) for key, item in asdict(value).items()}
        if isinstance(value, list):
            return [_dump(item) for item in value]
        return value


    @dataclass
    class PR(_BootstrapModel):
        id: str
        repo: str
        number: int
        title: str
        body: str
        url: str
        author: str
        labels: list[str] = field(default_factory=list)
        files_changed: list[str] = field(default_factory=list)
        review_status: str = ""
        ci_status: str = ""
        mergeable: str = ""
        base_branch: str = ""
        head_branch: str = ""
        cluster_id: str = ""
        created_at: str = ""
        updated_at: str = ""
        is_draft: bool = False
        is_bot: bool = False
        additions: int = 0
        deletions: int = 0
        changed_files_count: int = 0
        review_count: int = 0
        comment_count: int = 0


    @dataclass
    class PRCluster(_BootstrapModel):
        cluster_id: str
        cluster_label: str
        summary: str
        pr_ids: list[int] = field(default_factory=list)
        health_status: str = ""
        average_similarity: float = 0.0
        sample_titles: list[str] = field(default_factory=list)


    @dataclass
    class DuplicateGroup(_BootstrapModel):
        canonical_pr_number: int
        duplicate_pr_numbers: list[int] = field(default_factory=list)
        similarity: float = 0.0
        reason: str = ""


    @dataclass
    class GarbagePR(_BootstrapModel):
        pr_number: int
        reason: str = ""


    @dataclass
    class ConflictPair(_BootstrapModel):
        source_pr: int
        target_pr: int
        conflict_type: str
        files_touched: list[str] = field(default_factory=list)
        severity: str = ""
        reason: str = ""


    @dataclass
    class StalenessReport(_BootstrapModel):
        pr_number: int
        score: float
        signals: list[str] = field(default_factory=list)
        reasons: list[str] = field(default_factory=list)
        superseded_by: list[int] = field(default_factory=list)


    @dataclass
    class MergePlanCandidate(_BootstrapModel):
        pr_number: int
        title: str
        score: float
        rationale: str
        files_touched: list[str] = field(default_factory=list)
        conflict_warnings: list[str] = field(default_factory=list)


    @dataclass
    class MergePlan(_BootstrapModel):
        plan_id: str
        mode: str
        formula_expression: str
        selected: list[MergePlanCandidate] = field(default_factory=list)
        ordering: list[MergePlanCandidate] = field(default_factory=list)
        total_score: float = 0.0
        warnings: list[str] = field(default_factory=list)


    @dataclass
    class GraphNode(_BootstrapModel):
        pr_number: int
        title: str
        cluster_id: str
        ci_status: str


    @dataclass
    class GraphEdge(_BootstrapModel):
        from_pr: int
        to_pr: int
        edge_type: str
        reason: str


    @dataclass
    class Counts(_BootstrapModel):
        total_prs: int
        cluster_count: int
        duplicate_groups: int
        overlap_groups: int
        conflict_pairs: int
        stale_prs: int
        garbage_prs: int = 0
        collapsed_duplicate_groups: int = 0


    @dataclass
    class CollapsedCorpus(_BootstrapModel):
        canonical_to_superseded: dict[int, list[int]] = field(default_factory=dict)
        superseded_to_canonical: dict[int, int] = field(default_factory=dict)
        collapsed_group_count: int = 0
        total_superseded: int = 0


    @dataclass
    class DuplicateSynthesisCandidate(_BootstrapModel):
        pr_number: int
        title: str
        author: str
        role: str
        synthesis_score: float
        confidence: float = 0.0
        substance_score: int = 0
        mergeable: str = "unknown"
        has_test_evidence: bool = False
        subsystem_tags: list[str] = field(default_factory=list)
        risky_patterns: list[str] = field(default_factory=list)
        conflict_footprint: int = 0
        is_draft: bool = False
        signal_quality: str = "medium"
        scoring_factors: list[str] = field(default_factory=list)
        rationale: str = ""


    @dataclass
    class DuplicateSynthesisPlan(_BootstrapModel):
        group_id: str
        group_type: str
        original_canonical_pr: int
        nominated_canonical_pr: int
        similarity: float
        reason: str
        candidates: list[DuplicateSynthesisCandidate] = field(default_factory=list)
        synthesis_notes: list[str] = field(default_factory=list)


    @dataclass
    class Thresholds(_BootstrapModel):
        duplicate: float
        overlap: float


    @dataclass
    class PlanRejection(_BootstrapModel):
        pr_number: int
        reason: str


    @dataclass
    class ActionPreflight(_BootstrapModel):
        check: str
        status: str
        reason: str
        evidence_refs: list[str] = field(default_factory=list)
        required: bool = False
        checked_at: str = ""


    @dataclass
    class ProofBundle(_BootstrapModel):
        id: str
        work_item_id: str
        pr_number: int
        summary: str
        evidence_refs: list[str] = field(default_factory=list)
        artifact_refs: list[str] = field(default_factory=list)
        test_commands: list[str] = field(default_factory=list)
        test_results: list[str] = field(default_factory=list)
        created_by: str = ""
        created_at: str = ""


    @dataclass
    class ActionLease(_BootstrapModel):
        claimed_by: str = ""
        claimed_at: str = ""
        expires_at: str = ""


    @dataclass
    class ActionWorkItem(_BootstrapModel):
        id: str
        pr_number: int
        lane: str
        state: str
        priority_score: float
        confidence: float
        risk_flags: list[str] = field(default_factory=list)
        reason_trail: list[str] = field(default_factory=list)
        evidence_refs: list[str] = field(default_factory=list)
        required_preflight_checks: list[ActionPreflight] = field(default_factory=list)
        idempotency_key: str = ""
        lease_state: ActionLease = field(default_factory=ActionLease)
        allowed_actions: list[str] = field(default_factory=list)
        blocked_reasons: list[str] = field(default_factory=list)
        proof_bundle_refs: list[str] = field(default_factory=list)


    @dataclass
    class ActionIntent(_BootstrapModel):
        action: str
        pr_number: int
        dry_run: bool
        created_at: str
        id: str = ""
        lane: str = ""
        policy_profile: str = "advisory"
        confidence: float = 0.0
        risk_flags: list[str] = field(default_factory=list)
        reasons: list[str] = field(default_factory=list)
        evidence_refs: list[str] = field(default_factory=list)
        preconditions: list[ActionPreflight] = field(default_factory=list)
        idempotency_key: str = ""
        payload: dict[str, Any] = field(default_factory=dict)


    @dataclass
    class ActionLaneSummary(_BootstrapModel):
        lane: str
        count: int
        work_item_ids: list[str] = field(default_factory=list)


    @dataclass
    class ActionCorpusSnapshot(_BootstrapModel):
        total_prs: int = 0
        head_sha_indexed: bool = False
        analysis_truncated: bool = False
        max_prs_applied: int = 0


    @dataclass
    class ActionPlanAuditCheck(_BootstrapModel):
        name: str
        status: str
        reason: str = ""
        evidence_refs: list[str] = field(default_factory=list)
        checked_at: str = ""


    @dataclass
    class ActionPlanAudit(_BootstrapModel):
        checks: list[ActionPlanAuditCheck] = field(default_factory=list)
        warnings: list[str] = field(default_factory=list)
        errors: list[str] = field(default_factory=list)


    @dataclass
    class ActionPlan(_BootstrapModel):
        repo: str
        schema_version: str = ""
        run_id: str = ""
        policy_profile: str = "advisory"
        generated_at: str = ""
        corpus_snapshot: ActionCorpusSnapshot = field(default_factory=ActionCorpusSnapshot)
        lanes: list[ActionLaneSummary] = field(default_factory=list)
        work_items: list[ActionWorkItem] = field(default_factory=list)
        action_intents: list[ActionIntent] = field(default_factory=list)
        audit: ActionPlanAudit = field(default_factory=ActionPlanAudit)


    @dataclass
    class ClusterRequest(_BootstrapModel):
        repo: str
        prs: list[PR] = field(default_factory=list)
        model: str = ""
        minClusterSize: int = 0


    @dataclass
    class DuplicateDetectionRequest(_BootstrapModel):
        repo: str
        prs: list[PR] = field(default_factory=list)
        duplicateThreshold: float = 0.0
        overlapThreshold: float = 0.0


    @dataclass
    class SemanticAnalysisRequest(_BootstrapModel):
        repo: str
        prs: list[PR] = field(default_factory=list)
        analysisMode: str = ""


    @dataclass
    class ClusterResponse(_BootstrapModel):
        repo: str
        generatedAt: str
        model: str
        thresholds: Thresholds
        clusters: list[PRCluster] = field(default_factory=list)


    @dataclass
    class DuplicateResponse(_BootstrapModel):
        repo: str
        generatedAt: str
        duplicates: list[DuplicateGroup] = field(default_factory=list)
        overlaps: list[DuplicateGroup] = field(default_factory=list)


    @dataclass
    class SemanticConflictResponse(_BootstrapModel):
        repo: str
        generatedAt: str
        conflicts: list[ConflictPair] = field(default_factory=list)


    @dataclass
    class AnalysisResponse(_BootstrapModel):
        repo: str
        generatedAt: str
        counts: Counts
        prs: list[PR] = field(default_factory=list)
        clusters: list[PRCluster] = field(default_factory=list)
        duplicates: list[DuplicateGroup] = field(default_factory=list)
        overlaps: list[DuplicateGroup] = field(default_factory=list)
        conflicts: list[ConflictPair] = field(default_factory=list)
        stalenessSignals: list[StalenessReport] = field(default_factory=list)
        garbagePRs: list[GarbagePR] = field(default_factory=list)
        collapsed_corpus: CollapsedCorpus = field(default_factory=CollapsedCorpus)
        duplicate_synthesis: list[DuplicateSynthesisPlan] = field(default_factory=list)


    @dataclass
    class GraphResponse(_BootstrapModel):
        repo: str
        generatedAt: str
        nodes: list[GraphNode] = field(default_factory=list)
        edges: list[GraphEdge] = field(default_factory=list)
        dot: str = ""


    @dataclass
    class PlanResponse(_BootstrapModel):
        repo: str
        generatedAt: str
        target: int
        candidatePoolSize: int
        strategy: str
        selected: list[MergePlanCandidate] = field(default_factory=list)
        ordering: list[MergePlanCandidate] = field(default_factory=list)
        rejections: list[PlanRejection] = field(default_factory=list)
        collapsed_corpus: CollapsedCorpus = field(default_factory=CollapsedCorpus)


    @dataclass
    class HealthResponse(_BootstrapModel):
        status: str
        version: str


def _payload_to_json(payload: Any) -> str:
    if BaseModel is not None:
        return payload.model_dump_json(by_alias=True, exclude_none=True)
    return json.dumps(payload.model_dump(), separators=(",", ":"), sort_keys=False)


if __name__ == "__main__":
    action_plan_raw = os.environ.get("PRATC_SAMPLE_ACTIONPLAN_JSON")
    if action_plan_raw:
        payload = ActionPlan.model_validate(json.loads(action_plan_raw))
        print(_payload_to_json(payload))
        raise SystemExit(0)

    raw = os.environ.get("PRATC_SAMPLE_ANALYSIS_JSON")
    if not raw:
        raise SystemExit("PRATC_SAMPLE_ANALYSIS_JSON or PRATC_SAMPLE_ACTIONPLAN_JSON is required")
    payload = AnalysisResponse.model_validate(json.loads(raw))
    print(_payload_to_json(payload))
