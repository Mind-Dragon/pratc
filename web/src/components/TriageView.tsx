import React, { useMemo, useRef, useState, useCallback } from "react";
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  createColumnHelper,
  type Row,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";

import Layout from "./Layout";
import type { AnalysisResponse, PR, ActionIntent } from "../types/api";

type TriageViewProps = {
  analysis: AnalysisResponse | null;
  title?: string;
  eyebrow?: string;
  description?: string;
  emptyMessage?: string;
};

type ActionResult = {
  action: string;
  prNumber: number;
  success: boolean;
  message: string;
  timestamp: string;
};

type PRWithMeta = PR & { _index: number };

const columnHelper = createColumnHelper<PRWithMeta>();

function ActionFeedback({
  result,
  onDismiss,
}: {
  result: ActionResult | null;
  onDismiss: () => void;
}) {
  if (!result) return null;

  const isSuccess = result.success;
  return (
    <div
      role="alert"
      className={`action-feedback action-feedback--${isSuccess ? "success" : "failure"}`}
    >
      <div className="action-feedback-content">
        <span className={`action-feedback-icon ${isSuccess ? "icon-success" : "icon-failure"}`}>
          {isSuccess ? "✓" : "✗"}
        </span>
        <span className="action-feedback-message">{result.message}</span>
      </div>
      <button
        type="button"
        className="action-feedback-dismiss"
        onClick={onDismiss}
        aria-label="Dismiss notification"
      >
        ×
      </button>
    </div>
  );
}

function ActionButtons({
  pr,
  onAction,
  disabled,
}: {
  pr: PR;
  onAction: (action: "approve" | "close" | "skip", pr: PR) => void;
  disabled?: boolean;
}) {
  return (
    <div className="action-row">
      <button
        type="button"
        className="action-btn action-btn--approve"
        onClick={() => onAction("approve", pr)}
        disabled={disabled}
        aria-label={`Approve PR #${pr.number}`}
      >
        Approve
      </button>
      <button
        type="button"
        className="action-btn action-btn--close"
        onClick={() => onAction("close", pr)}
        disabled={disabled}
        aria-label={`Close PR #${pr.number}`}
      >
        Close
      </button>
      <button
        type="button"
        className="action-btn action-btn--skip"
        onClick={() => onAction("skip", pr)}
        disabled={disabled}
        aria-label={`Skip PR #${pr.number}`}
      >
        Skip
      </button>
    </div>
  );
}

export default function TriageView({
  analysis,
  title = "Triage Inbox",
  eyebrow = "Sequential Review",
  description = "Outlook-style sequential triage using live analysis data (read-only actions).",
  emptyMessage = "No triage data available.",
}: TriageViewProps) {
  const prsRaw = analysis?.prs ?? [];
  const prs = useMemo<PRWithMeta[]>(() => prsRaw.map((pr, idx) => ({ ...pr, _index: idx })), [prsRaw]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [feedback, setFeedback] = useState<ActionResult | null>(null);
  const [intentLog, setIntentLog] = useState<ActionIntent[]>([]);
  const [actionInProgress, setActionInProgress] = useState<string | null>(null);
  const tableContainerRef = useRef<HTMLDivElement>(null);

  const selected = useMemo<PR | null>(() => {
    if (prs.length === 0) {
      return null;
    }
    if (selectedIndex < 0 || selectedIndex >= prs.length) {
      return prs[0];
    }
    return prs[selectedIndex];
  }, [prs, selectedIndex]);

  const review = analysis?.review_payload ?? null;
  const reviewBuckets = review
    ? [
        { label: "now", value: review.buckets.find((bucket) => bucket.bucket === "now")?.count ?? 0, tone: "mint" },
        { label: "future", value: review.buckets.find((bucket) => bucket.bucket === "future")?.count ?? 0, tone: "sky" },
        { label: "duplicate", value: review.buckets.find((bucket) => bucket.bucket === "duplicate")?.count ?? 0, tone: "sand" },
        { label: "junk", value: review.buckets.find((bucket) => bucket.bucket === "junk")?.count ?? 0, tone: "rose" },
        { label: "blocked", value: review.buckets.find((bucket) => bucket.bucket === "blocked")?.count ?? 0, tone: "sand" }
      ] as const
    : [];

  const priorityTiers = review
    ? [
        { label: "fast_merge", value: review.priority_tiers.find((tier) => tier.tier === "fast_merge")?.count ?? 0, tone: "mint" },
        { label: "review_required", value: review.priority_tiers.find((tier) => tier.tier === "review_required")?.count ?? 0, tone: "sky" },
        { label: "blocked", value: review.priority_tiers.find((tier) => tier.tier === "blocked")?.count ?? 0, tone: "rose" }
      ] as const
    : [];

  // Log action intent (intent-only, no real mutation)
  const logIntent = useCallback(
    (action: string, prNumber: number): ActionIntent => {
      const intent: ActionIntent = {
        action,
        pr_number: prNumber,
        dry_run: true,
        created_at: new Date().toISOString(),
      };
      setIntentLog((prev) => [...prev, intent]);
      return intent;
    },
    []
  );

  const handleAction = useCallback(
    async (action: "approve" | "close" | "skip", pr: PR) => {
      if (actionInProgress) return;

      setActionInProgress(action);
      setFeedback(null);

      // Simulate async operation with deterministic result
      await new Promise((resolve) => setTimeout(resolve, 50));

      // Intent-only: log the action (no real GitHub mutation)
      logIntent(action, pr.number);

      // Deterministic feedback: always succeed for valid actions
      const success = true;
      const messages: Record<string, string> = {
        approve: `Intent logged: would approve PR #${pr.number}`,
        close: `Intent logged: would close PR #${pr.number}`,
        skip: `Intent logged: skipped PR #${pr.number}`,
      };

      const result: ActionResult = {
        action,
        prNumber: pr.number,
        success,
        message: messages[action],
        timestamp: new Date().toISOString(),
      };

      setFeedback(result);
      setActionInProgress(null);

      setTimeout(() => setFeedback(null), 3000);
    },
    [actionInProgress, logIntent]
  );

  const dismissFeedback = useCallback(() => setFeedback(null), []);

  const columns = useMemo(
    () => [
      columnHelper.accessor("number", {
        header: "#",
        cell: (info) => `#${info.getValue()}`,
        size: 60,
      }),
      columnHelper.accessor("title", {
        header: "Title",
        cell: (info) => info.getValue(),
        size: 300,
      }),
      columnHelper.accessor("author", {
        header: "Author",
        cell: (info) => info.getValue(),
        size: 100,
      }),
      columnHelper.accessor("ci_status", {
        header: "Status",
        cell: (info) => (
          <span className={`pill pill--${info.getValue()}`}>{info.getValue()}</span>
        ),
        size: 80,
      }),
      columnHelper.accessor("cluster_id", {
        header: "Cluster",
        cell: (info) => info.getValue() || "unclustered",
        size: 100,
      }),
      columnHelper.display({
        id: "actions",
        header: "Actions",
        cell: ({ row }) => (
          <ActionButtons
            pr={row.original}
            onAction={handleAction}
            disabled={actionInProgress !== null}
          />
        ),
        size: 200,
      }),
    ],
    [handleAction, actionInProgress]
  );

  const table = useReactTable({
    data: prs,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  const { rows } = table.getRowModel();
  const rowVirtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => tableContainerRef.current,
    estimateSize: () => 52,
    overscan: 10,
  });

  const virtualRows = rowVirtualizer.getVirtualItems();
  const totalSize = rowVirtualizer.getTotalSize();
  const useVirtualization = virtualRows.length > 0 || rows.length === 0;

  if (!analysis) {
    return (
      <Layout
        title={title}
        eyebrow={eyebrow}
        description="Live API is unavailable. Start `pratc serve --port=8080` to load triage data."
      >
        <section className="triage-shell">
          <div className="triage-table-panel">
            <p>{emptyMessage}</p>
          </div>
        </section>
      </Layout>
    );
  }

  return (
    <Layout title={title} eyebrow={eyebrow} description={description}>
      <section className="triage-shell">
        <div className="triage-table-panel">
          <div className="section-heading">
            <div>
              <p className="hero-kicker">Queue</p>
              <h3>Review lane</h3>
            </div>
            <p>{prs.length} PRs ready for triage.</p>
          </div>

          <ActionFeedback result={feedback} onDismiss={dismissFeedback} />

          {review ? (
            <section className="cluster-section" aria-label="review summary">
              <div className="section-heading">
                <div>
                  <p className="hero-kicker">Review engine</p>
                  <h3>Bucket distribution</h3>
                </div>
                <p>
                  {review.reviewed_prs.toLocaleString()} reviewed PRs out of {review.total_prs.toLocaleString()} total.
                </p>
              </div>
              <div className="stats-grid">
                {reviewBuckets.map((card) => (
                  <article className={`stat-card stat-card--${card.tone}`} key={card.label}>
                    <span>{card.label}</span>
                    <strong>{card.value}</strong>
                  </article>
                ))}
              </div>
              <div className="stats-grid" style={{ marginTop: 12 }}>
                {priorityTiers.map((card) => (
                  <article className={`stat-card stat-card--${card.tone}`} key={card.label}>
                    <span>{card.label}</span>
                    <strong>{card.value}</strong>
                  </article>
                ))}
              </div>
            </section>
          ) : null}

          <div
            ref={tableContainerRef}
            className="triage-table-container"
            style={{ maxHeight: "500px", overflow: "auto" }}
          >
            <table className="triage-table" style={{ display: "grid" }}>
              <thead style={{ display: "grid", position: "sticky", top: 0, zIndex: 1, background: "var(--panel)" }}>
                {table.getHeaderGroups().map((headerGroup) => (
                  <tr key={headerGroup.id} style={{ display: "flex", width: "100%" }}>
                    {headerGroup.headers.map((header) => (
                      <th
                        key={header.id}
                        style={{
                          width: header.getSize(),
                          flexShrink: 0,
                        }}
                      >
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody
                style={
                  useVirtualization
                    ? {
                        display: "grid",
                        height: `${totalSize}px`,
                        position: "relative",
                      }
                    : undefined
                }
              >
                {useVirtualization
                  ? virtualRows.map((virtualRow) => {
                      const row = rows[virtualRow.index] as Row<PRWithMeta>;
                      return (
                        <tr
                          key={row.id}
                          data-index={virtualRow.index}
                          onClick={() => setSelectedIndex(virtualRow.index)}
                          className={`triage-row ${selectedIndex === virtualRow.index ? "triage-row--selected" : ""}`}
                          style={{
                            position: "absolute",
                            top: 0,
                            left: 0,
                            width: "100%",
                            height: `${virtualRow.size}px`,
                            transform: `translateY(${virtualRow.start}px)`,
                            display: "flex",
                          }}
                        >
                          {row.getVisibleCells().map((cell) => (
                            <td
                              key={cell.id}
                              style={{
                                width: cell.column.getSize(),
                                flexShrink: 0,
                              }}
                            >
                              {flexRender(cell.column.columnDef.cell, cell.getContext())}
                            </td>
                          ))}
                        </tr>
                      );
                    })
                  : rows.map((row, index) => (
                      <tr
                        key={row.id}
                        data-index={index}
                        onClick={() => setSelectedIndex(index)}
                        className={`triage-row ${selectedIndex === index ? "triage-row--selected" : ""}`}
                      >
                        {row.getVisibleCells().map((cell) => (
                          <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>
                        ))}
                      </tr>
                    ))}
              </tbody>
            </table>
          </div>

          {intentLog.length > 0 && (
            <div className="intent-log">
              <p className="hero-kicker">Intent Log</p>
              <ul className="intent-log-list">
                {intentLog.slice(-5).map((intent, idx) => (
                  <li key={idx} className="intent-log-item">
                    <span className="intent-action">{intent.action}</span>
                    <span className="intent-pr">#{intent.pr_number}</span>
                    <span className="intent-time">{new Date(intent.created_at).toLocaleTimeString()}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>

        {selected && (
          <aside className="triage-detail">
            <p className="hero-kicker">Selected PR</p>
            <h3>#{selected.number}</h3>
            <p>{selected.title}</p>
            <dl>
              <div>
                <dt>Repository</dt>
                <dd>{selected.repo}</dd>
              </div>
              <div>
                <dt>Review</dt>
                <dd>{selected.review_status}</dd>
              </div>
              <div>
                <dt>Changed files</dt>
                <dd>{selected.changed_files_count}</dd>
              </div>
              <div>
                <dt>Branch</dt>
                <dd>{selected.head_branch}</dd>
              </div>
            </dl>
          </aside>
        )}
      </section>
    </Layout>
  );
}
