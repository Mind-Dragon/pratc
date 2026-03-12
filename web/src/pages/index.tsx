import React from "react";
import Layout from "../components/Layout";
import type { AnalysisResponse } from "../types/api";

const mockAnalysis: AnalysisResponse = {
  repo: "opencode-ai/opencode",
  generatedAt: "2026-03-12T18:00:00Z",
  counts: {
    total_prs: 5674,
    cluster_count: 128,
    duplicate_groups: 42,
    overlap_groups: 67,
    conflict_pairs: 113,
    stale_prs: 804
  },
  prs: [],
  clusters: [
    {
      cluster_id: "deps-01",
      cluster_label: "Dependency refresh lane",
      summary: "Routine package updates with low merge risk and high batching potential.",
      pr_ids: [2181, 2184, 2192],
      health_status: "green",
      average_similarity: 0.93,
      sample_titles: [
        "chore(deps): bump next to 15.2",
        "chore(deps): update vitest",
        "chore(deps): refresh bun lockfile"
      ]
    },
    {
      cluster_id: "planner-07",
      cluster_label: "Planner logic conflicts",
      summary: "Competing merge-plan changes touching the same scoring and ordering paths.",
      pr_ids: [441, 452, 480, 511],
      health_status: "red",
      average_similarity: 0.81,
      sample_titles: [
        "Refine candidate pool scoring",
        "Add conflict weighting to planner",
        "Simplify merge ordering heuristics"
      ]
    }
  ],
  duplicates: [],
  overlaps: [],
  conflicts: [],
  stalenessSignals: []
};

const cards = [
  {
    label: "Open PRs",
    value: mockAnalysis.counts.total_prs.toLocaleString(),
    tone: "sky"
  },
  {
    label: "Clusters",
    value: mockAnalysis.counts.cluster_count.toString(),
    tone: "sand"
  },
  {
    label: "Duplicate Groups",
    value: mockAnalysis.counts.duplicate_groups.toString(),
    tone: "mint"
  },
  {
    label: "Stale PRs",
    value: mockAnalysis.counts.stale_prs.toString(),
    tone: "rose"
  }
] as const;

export default function DashboardPage() {
  return (
    <Layout
      title="prATC Dashboard"
      eyebrow="Air Traffic Control"
      description="Static scaffold for the ATC overview. Live API wiring lands in later dashboard tasks."
    >
      <section className="hero-panel" aria-label="overview">
        <div>
          <p className="hero-kicker">Repository focus</p>
          <h2>{mockAnalysis.repo}</h2>
          <p>
            Snapshot generated at <time dateTime={mockAnalysis.generatedAt}>{mockAnalysis.generatedAt}</time>.
          </p>
        </div>
        <div className="hero-badge">Dry run only</div>
      </section>

      <section className="stats-grid" aria-label="summary cards">
        {cards.map((card) => (
          <article className={`stat-card stat-card--${card.tone}`} key={card.label}>
            <span>{card.label}</span>
            <strong>{card.value}</strong>
          </article>
        ))}
      </section>

      <section className="cluster-section" aria-label="cluster preview">
        <div className="section-heading">
          <div>
            <p className="hero-kicker">Cluster lanes</p>
            <h3>Priority lanes</h3>
          </div>
          <p>{mockAnalysis.clusters.length} clusters surfaced in this mock snapshot.</p>
        </div>
        <div className="cluster-grid">
          {mockAnalysis.clusters.map((cluster) => (
            <article className={`cluster-card cluster-card--${cluster.health_status}`} key={cluster.cluster_id}>
              <div className="cluster-topline">
                <span className="cluster-status">{cluster.health_status}</span>
                <span className="cluster-count">{cluster.pr_ids.length} PRs</span>
              </div>
              <h4>{cluster.cluster_label}</h4>
              <p>{cluster.summary}</p>
              <ul>
                {cluster.sample_titles.map((title) => (
                  <li key={title}>{title}</li>
                ))}
              </ul>
            </article>
          ))}
        </div>
      </section>
    </Layout>
  );
}
