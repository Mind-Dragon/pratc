import React from "react";
import Layout from "../components/Layout";
import type { PR } from "../types/api";

const mockPRs: PR[] = [
  {
    id: "PR_kwDOAA1",
    repo: "opencode-ai/opencode",
    number: 2181,
    title: "chore(deps): bump next to 15.2.2",
    body: "Routine dependency refresh.",
    url: "https://github.com/opencode-ai/opencode/pull/2181",
    author: "dependabot[bot]",
    labels: ["dependencies", "bot"],
    files_changed: ["web/package.json", "web/bun.lockb"],
    review_status: "pending",
    ci_status: "passing",
    mergeable: "mergeable",
    base_branch: "main",
    head_branch: "dependabot/npm_and_yarn/next-15.2.2",
    cluster_id: "deps-01",
    created_at: "2026-03-11T15:30:00Z",
    updated_at: "2026-03-12T08:00:00Z",
    is_draft: false,
    is_bot: true,
    additions: 12,
    deletions: 4,
    changed_files_count: 2
  },
  {
    id: "PR_kwDOAA2",
    repo: "opencode-ai/opencode",
    number: 441,
    title: "Refine candidate pool scoring",
    body: "Tweaks planner heuristics ahead of wider formula integration.",
    url: "https://github.com/opencode-ai/opencode/pull/441",
    author: "jeffersonwarrior",
    labels: ["planner", "needs-review"],
    files_changed: ["internal/planner/scorer.go"],
    review_status: "changes_requested",
    ci_status: "failing",
    mergeable: "conflicting",
    base_branch: "main",
    head_branch: "planner/scoring-refactor",
    cluster_id: "planner-07",
    created_at: "2026-03-05T09:10:00Z",
    updated_at: "2026-03-12T12:20:00Z",
    is_draft: false,
    is_bot: false,
    additions: 140,
    deletions: 52,
    changed_files_count: 1
  },
  {
    id: "PR_kwDOAA3",
    repo: "opencode-ai/opencode",
    number: 511,
    title: "Simplify merge ordering heuristics",
    body: "Removes a scoring branch to make plan output more predictable.",
    url: "https://github.com/opencode-ai/opencode/pull/511",
    author: "octocat",
    labels: ["planner"],
    files_changed: ["internal/planner/ordering.go", "internal/planner/plan.go"],
    review_status: "approved",
    ci_status: "passing",
    mergeable: "mergeable",
    base_branch: "main",
    head_branch: "planner/order-simplification",
    cluster_id: "planner-07",
    created_at: "2026-03-02T10:45:00Z",
    updated_at: "2026-03-12T09:05:00Z",
    is_draft: false,
    is_bot: false,
    additions: 88,
    deletions: 34,
    changed_files_count: 2
  }
];

export default function TriagePage() {
  return (
    <Layout
      title="Triage Inbox"
      eyebrow="Sequential Review"
      description="Outlook-style triage shell with mock rows only. Action wiring remains dry-run and future-task scoped."
    >
      <section className="triage-shell">
        <div className="triage-table-panel">
          <div className="section-heading">
            <div>
              <p className="hero-kicker">Queue</p>
              <h3>Review lane</h3>
            </div>
            <p>{mockPRs.length} mock PRs ready for triage.</p>
          </div>
          <table className="triage-table">
            <thead>
              <tr>
                <th>#</th>
                <th>Title</th>
                <th>Author</th>
                <th>Status</th>
                <th>Cluster</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {mockPRs.map((pr) => (
                <tr key={pr.id}>
                  <td>#{pr.number}</td>
                  <td>{pr.title}</td>
                  <td>{pr.author}</td>
                  <td>
                    <span className={`pill pill--${pr.ci_status}`}>{pr.ci_status}</span>
                  </td>
                  <td>{pr.cluster_id}</td>
                  <td>
                    <div className="action-row">
                      <button type="button">Approve</button>
                      <button type="button">Close</button>
                      <button type="button">Skip</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <aside className="triage-detail">
          <p className="hero-kicker">Selected PR</p>
          <h3>#{mockPRs[0].number}</h3>
          <p>{mockPRs[0].title}</p>
          <dl>
            <div>
              <dt>Repository</dt>
              <dd>{mockPRs[0].repo}</dd>
            </div>
            <div>
              <dt>Review</dt>
              <dd>{mockPRs[0].review_status}</dd>
            </div>
            <div>
              <dt>Changed files</dt>
              <dd>{mockPRs[0].changed_files_count}</dd>
            </div>
            <div>
              <dt>Branch</dt>
              <dd>{mockPRs[0].head_branch}</dd>
            </div>
          </dl>
        </aside>
      </section>
    </Layout>
  );
}
