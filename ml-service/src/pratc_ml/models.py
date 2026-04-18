from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass, field, fields, is_dataclass
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


    class Thresholds(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        duplicate: float
        overlap: float


    class PlanRejection(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        pr_number: int
        reason: str


    class ActionIntent(BaseModel):
        model_config = ConfigDict(populate_by_name=True)

        action: str
        pr_number: int
        dry_run: bool
        created_at: str


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
            kwargs[item.name] = _coerce_value(item.type, value[item.name])
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


    @dataclass
    class Thresholds(_BootstrapModel):
        duplicate: float
        overlap: float


    @dataclass
    class PlanRejection(_BootstrapModel):
        pr_number: int
        reason: str


    @dataclass
    class ActionIntent(_BootstrapModel):
        action: str
        pr_number: int
        dry_run: bool
        created_at: str


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


    @dataclass
    class HealthResponse(_BootstrapModel):
        status: str
        version: str


def _payload_to_json(payload: AnalysisResponse) -> str:
    if BaseModel is not None:
        return payload.model_dump_json(by_alias=True, exclude_none=True)
    return json.dumps(payload.model_dump(), separators=(",", ":"), sort_keys=False)


if __name__ == "__main__":
    raw = os.environ.get("PRATC_SAMPLE_ANALYSIS_JSON")
    if not raw:
        raise SystemExit("PRATC_SAMPLE_ANALYSIS_JSON is required")
    payload = AnalysisResponse.model_validate(json.loads(raw))
    print(_payload_to_json(payload))
