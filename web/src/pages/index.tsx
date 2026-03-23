import React from "react";
import type { GetServerSideProps } from "next";

import Layout from "../components/Layout";
import { fetchAnalysis } from "../lib/api";
import type { AnalysisResponse } from "../types/api";

const DEFAULT_REPO = "opencode-ai/opencode";

type DashboardProps = {
  analysis: AnalysisResponse | null;
};

export const getServerSideProps: GetServerSideProps<DashboardProps> = async (context) => {
  const rawRepo = context.query.repo;
  const repo = typeof rawRepo === "string" && rawRepo.length > 0 ? rawRepo : DEFAULT_REPO;
  const analysis = await fetchAnalysis(repo);
  return {
    props: {
      analysis
    }
  };
};

export default function DashboardPage({ analysis }: DashboardProps) {
  if (!analysis) {
    return (
      <Layout
        title="prATC Dashboard"
        eyebrow="Air Traffic Control"
        description="Live API is unavailable. Start `pratc serve --port=8080` to load dashboard data."
      >
        <section className="hero-panel" aria-label="overview">
          <div>
            <p className="hero-kicker">API status</p>
            <h2>Disconnected</h2>
            <p>No analysis payload available.</p>
          </div>
          <div className="hero-badge">Read-only</div>
        </section>
      </Layout>
    );
  }

  const cards = [
    { label: "Open PRs", value: analysis.counts.total_prs.toLocaleString(), tone: "sky" },
    { label: "Clusters", value: analysis.counts.cluster_count.toString(), tone: "sand" },
    { label: "Duplicate Groups", value: analysis.counts.duplicate_groups.toString(), tone: "mint" },
    { label: "Stale PRs", value: analysis.counts.stale_prs.toString(), tone: "rose" }
  ] as const;

  return (
    <Layout
      title="prATC Dashboard"
      eyebrow="Air Traffic Control"
      description="Live PR analysis sourced from the local prATC API."
    >
      <section className="hero-panel" aria-label="overview">
        <div>
          <p className="hero-kicker">Repository focus</p>
          <h2>{analysis.repo}</h2>
          <p>
            Snapshot generated at <time dateTime={analysis.generatedAt}>{analysis.generatedAt}</time>.
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
          <p>{analysis.clusters.length} clusters surfaced in this snapshot.</p>
        </div>
        <div className="cluster-grid">
          {analysis.clusters.map((cluster) => (
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
