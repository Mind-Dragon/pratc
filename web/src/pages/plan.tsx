import React from "react";
import type { GetServerSideProps } from "next";

import Layout from "../components/Layout";
import { fetchPlan } from "../lib/api";
import type { PlanResponse } from "../types/api";

const DEFAULT_REPO = "opencode-ai/opencode";

type PlanProps = {
  plan: PlanResponse | null;
};

export const getServerSideProps: GetServerSideProps<PlanProps> = async (context) => {
  const rawRepo = context.query.repo;
  const repo = typeof rawRepo === "string" && rawRepo.length > 0 ? rawRepo : DEFAULT_REPO;
  const rawTarget = context.query.target;
  const target = typeof rawTarget === "string" ? Number.parseInt(rawTarget, 10) : 20;
  const plan = await fetchPlan(repo, Number.isFinite(target) && target > 0 ? target : 20);
  return {
    props: {
      plan
    }
  };
};

export default function PlanPage({ plan }: PlanProps) {
  return (
    <Layout
      title="Merge Plan"
      eyebrow="Planner Output"
      description="Formula + graph recommendation set from the live API."
    >
      {!plan ? (
        <section className="hero-panel">
          <div>
            <p className="hero-kicker">API status</p>
            <h2>Disconnected</h2>
            <p>Unable to load merge plan payload.</p>
          </div>
        </section>
      ) : (
        <section className="cluster-section">
          <div className="section-heading">
            <div>
              <p className="hero-kicker">Strategy</p>
              <h3>{plan.strategy}</h3>
            </div>
            <p>
              target {plan.target} / pool {plan.candidatePoolSize}
            </p>
          </div>
          <div className="cluster-grid">
            {plan.ordering.map((item, index) => (
              <article className="cluster-card cluster-card--green" key={item.pr_number}>
                <div className="cluster-topline">
                  <span className="cluster-status">step {index + 1}</span>
                  <span className="cluster-count">score {item.score.toFixed(2)}</span>
                </div>
                <h4>#{item.pr_number} {item.title}</h4>
                <p>{item.rationale}</p>
                {item.conflict_warnings.length > 0 && (
                  <ul>
                    {item.conflict_warnings.map((warning) => (
                      <li key={warning}>{warning}</li>
                    ))}
                  </ul>
                )}
              </article>
            ))}
          </div>
        </section>
      )}
    </Layout>
  );
}
