import React, { useState } from "react";
import type { GetServerSideProps } from "next";

import Layout from "../components/Layout";
import { fetchOmniPlan, fetchPlan } from "../lib/api";
import type { OmniPlanResponse, PlanResponse } from "../types/api";

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
  const [omniMode, setOmniMode] = useState(false);
  const [selector, setSelector] = useState("");
  const [omniResult, setOmniResult] = useState<OmniPlanResponse | null>(null);

  return (
    <Layout
      title="Merge Plan"
      eyebrow="Planner Output"
      description="Formula + graph recommendation set from the live API."
    >
      <div className="flex items-center gap-2 mb-4">
        <label className="text-sm font-medium">Omni-Batch Mode</label>
        <button
          onClick={() => setOmniMode(!omniMode)}
          className={`px-3 py-1 rounded text-sm ${omniMode ? "bg-blue-600 text-white" : "bg-gray-200"}`}
        >
          {omniMode ? "ON" : "OFF"}
        </button>
      </div>

      {omniMode && (
        <div className="mb-4 p-4 border rounded">
          <h3 className="text-sm font-semibold mb-2">Omni-Batch Selector</h3>
          <input
            type="text"
            value={selector}
            onChange={(e) => setSelector(e.target.value)}
            placeholder='e.g., "1-100 AND 50-200" or "1 OR 2 OR 3"'
            className="w-full px-3 py-2 border rounded mb-2"
          />
          <button
            onClick={async () => {
              try {
                const result = await fetchOmniPlan(plan?.repo ?? DEFAULT_REPO, selector, {
                  stageSize: 64,
                  target: 20,
                });
                if (!result) {
                  alert("Failed to fetch omni plan");
                  return;
                }
                setOmniResult(result);
              } catch (err) {
                alert(err instanceof Error ? err.message : "Failed to fetch omni plan");
              }
            }}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
          >
            Run Omni Batch
          </button>

          {omniResult && (
            <div className="mt-4">
              <h4 className="text-sm font-semibold mb-2">Stages</h4>
              <div className="grid grid-cols-4 gap-2 mb-4">
                {omniResult.stages.map((stage) => (
                  <div key={stage.stage} className="p-2 border rounded text-center">
                    <div className="text-xs text-gray-500">Stage {stage.stage}</div>
                    <div className="font-mono text-sm">{stage.matched} matched</div>
                    <div className="text-xs text-gray-400">{stage.selected} selected</div>
                  </div>
                ))}
              </div>
              <h4 className="text-sm font-semibold mb-2">Selected PRs ({omniResult.selected.length})</h4>
              <div className="flex flex-wrap gap-1">
                {omniResult.selected.map((pr) => (
                  <span key={pr} className="px-2 py-1 bg-blue-100 text-blue-700 rounded text-sm">
                    #{pr}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

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
