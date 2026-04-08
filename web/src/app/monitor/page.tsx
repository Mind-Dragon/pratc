"use client";

import React from "react";

import Layout from "../../components/Layout";

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
        <section className="monitor-zone" aria-label="Jobs">
          <div className="zone-header">
            <h2>Jobs</h2>
            <p className="zone-description">Active and queued sync jobs</p>
          </div>
          <div className="zone-content-placeholder">
            <p>Job queue will appear here</p>
          </div>
        </section>

        <section className="monitor-zone" aria-label="Timeline">
          <div className="zone-header">
            <h2>Timeline</h2>
            <p className="zone-description">Sync history and scheduled operations</p>
          </div>
          <div className="zone-content-placeholder">
            <p>Timeline will appear here</p>
          </div>
        </section>

        <section className="monitor-zone" aria-label="Rate Limit">
          <div className="zone-header">
            <h2>Rate Limit</h2>
            <p className="zone-description">GitHub API quota and budget status</p>
          </div>
          <div className="zone-content-placeholder">
            <p>Rate limit metrics will appear here</p>
          </div>
        </section>
      </div>

      <style jsx>{`
        .monitor-layout {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 24px;
          padding: 24px;
          min-height: calc(100vh - 280px);
        }

        .monitor-zone {
          background: var(--panel);
          border: 1px solid var(--line);
          border-radius: 12px;
          padding: 20px;
          box-shadow: var(--shadow);
        }

        .zone-header {
          margin-bottom: 16px;
          padding-bottom: 12px;
          border-bottom: 1px solid var(--line);
        }

        .zone-header h2 {
          margin: 0;
          font-size: 1.25rem;
          color: var(--ink);
        }

        .zone-description {
          margin: 4px 0 0;
          font-size: 0.875rem;
          color: rgba(23, 34, 53, 0.6);
        }

        .zone-content-placeholder {
          display: flex;
          align-items: center;
          justify-content: center;
          min-height: 200px;
          background: rgba(185, 215, 242, 0.08);
          border-radius: 8px;
          border: 2px dashed var(--line);
        }

        .zone-content-placeholder p {
          margin: 0;
          color: rgba(23, 34, 53, 0.5);
          font-size: 0.95rem;
        }
      `}</style>
    </Layout>
  );
}
