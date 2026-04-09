"use client";

import React from "react";

import Layout from "../../components/Layout";
import JobsPanel from "./components/JobsPanel";
import TimelinePanel from "./components/TimelinePanel";
import RateLimitPanel from "./components/RateLimitPanel";
import ConsolePanel from "./components/ConsolePanel";

const DEFAULT_REPO = "opencode-ai/opencode";

export default function MonitorPage() {
  const repo = DEFAULT_REPO;

  return (
    <Layout
      title="prATC Monitor"
      eyebrow="Real-time Dashboard"
      description="Monitor sync jobs, rate limits, and system health"
    >
      <div className="monitor-layout">
        <div className="monitor-grid">
          <JobsPanel />
          <TimelinePanel />
          <RateLimitPanel />
        </div>
        <div className="monitor-console">
          <ConsolePanel />
        </div>
      </div>

      <style jsx>{`
        .monitor-layout {
          display: flex;
          flex-direction: column;
          gap: 24px;
          padding: 24px;
          min-height: calc(100vh - 280px);
        }

        .monitor-grid {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 24px;
        }

        .monitor-console {
          width: 100%;
        }

        /* Tablet: 768px - 1439px (2-column grid) */
        @media (max-width: 1439px) and (min-width: 768px) {
          .monitor-grid {
            grid-template-columns: repeat(2, 1fr);
          }
        }

        /* Mobile: <768px (stacked vertical) */
        @media (max-width: 767px) {
          .monitor-layout {
            padding: 16px;
            gap: 16px;
          }

          .monitor-grid {
            grid-template-columns: 1fr;
            gap: 16px;
          }

          .monitor-console {
            width: 100%;
          }
        }
      `}</style>
    </Layout>
  );
}
