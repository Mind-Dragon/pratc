import React from "react";
import { useMonitorData } from "../../../hooks/useMonitorData";
import type { SyncJobView } from "../../../types/monitor";

function formatEta(nanoseconds: number): string {
  if (nanoseconds <= 0) return "—";
  const seconds = Math.floor(nanoseconds / 1e9);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}h ${remainingMinutes}m`;
}

function getStatusColor(status: string): string {
  switch (status.toLowerCase()) {
    case "in_progress":
    case "running":
    case "active":
      return "var(--cyan)";
    case "paused":
    case "waiting":
      return "var(--amber)";
    case "failed":
    case "error":
      return "var(--red)";
    case "queued":
    case "pending":
    default:
      return "rgba(224, 230, 241, 0.3)";
  }
}

function getStatusLabel(status: string): string {
  switch (status.toLowerCase()) {
    case "in_progress":
    case "running":
    case "active":
      return "Active";
    case "paused":
    case "waiting":
      return "Paused";
    case "failed":
    case "error":
      return "Failed";
    case "queued":
    case "pending":
      return "Queued";
    default:
      return status;
  }
}

interface JobRowProps {
  job: SyncJobView;
}

function JobRow({ job }: JobRowProps) {
  const progressPercent = Math.min(100, Math.max(0, Math.round(job.progress * 100)));
  const statusColor = getStatusColor(job.status);
  const statusLabel = getStatusLabel(job.status);

  return (
    <div
      className="job-row"
      style={{
        padding: "12px 14px",
        background: "rgba(224, 230, 241, 0.06)",
        borderRadius: 10,
        border: "1px solid var(--line)",
        display: "flex",
        alignItems: "center",
        gap: 12,
        marginBottom: 8,
      }}
    >
      <span
        className="job-status-pill"
        style={{
          minWidth: 72,
          padding: "5px 10px",
          borderRadius: 999,
          background: statusColor,
          color: job.status.toLowerCase().includes("fail") || job.status.toLowerCase().includes("error") ? "#0A0E27" : "var(--text)",
          fontSize: "0.72rem",
          fontWeight: 600,
          textTransform: "uppercase",
          letterSpacing: "0.06em",
          textAlign: "center",
        }}
      >
        {statusLabel}
      </span>

      <div
        className="job-repo"
        style={{
          flex: 1,
          minWidth: 0,
          overflow: "hidden",
        }}
      >
        <span
          style={{
            display: "block",
            fontSize: "0.85rem",
            fontWeight: 500,
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
          title={job.repo}
        >
          {job.repo}
        </span>
      </div>

      <div
        className="job-progress"
        style={{
          width: 140,
          minWidth: 140,
        }}
      >
        <div
          style={{
            width: "100%",
            height: 7,
            borderRadius: 999,
            background: "rgba(224, 230, 241, 0.08)",
            overflow: "hidden",
            position: "relative",
          }}
        >
          <div
            className="job-progress-bar"
            style={{
              width: `${progressPercent}%`,
              height: "100%",
              borderRadius: 999,
              background: `linear-gradient(90deg, ${statusColor}, rgba(255, 255, 255, 0.4))`,
              transition: "width 400ms ease",
              position: "relative",
              overflow: "hidden",
            }}
          >
            <div
              className="job-progress-bar--animation"
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                background:
                  job.status.toLowerCase().includes("progress") || job.status.toLowerCase().includes("running")
                    ? "linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.3), transparent)"
                    : "none",
                animation: job.status.toLowerCase().includes("progress") || job.status.toLowerCase().includes("running")
                  ? "progress-shimmer 1.5s infinite"
                  : "none",
              }}
            />
          </div>
        </div>
      </div>

      <div
        className="job-percentage"
        style={{
          width: 48,
          minWidth: 48,
          textAlign: "right",
          fontSize: "0.82rem",
          fontWeight: 600,
          color: "rgba(224, 230, 241, 0.75)",
        }}
      >
        {progressPercent}%
      </div>

      <div
        className="job-eta"
        style={{
          width: 70,
          minWidth: 70,
          textAlign: "right",
          fontSize: "0.78rem",
          color: "rgba(224, 230, 241, 0.65)",
        }}
      >
        {formatEta(job.eta)}
      </div>
    </div>
  );
}

export default function JobsPanel() {
  const { jobs, connected, error } = useMonitorData();

  const hasJobs = jobs && jobs.length > 0;

  return (
    <div
      className="jobs-panel hero-panel"
      style={{
        flexDirection: "column",
        gap: 12,
        padding: 18,
        minHeight: 280,
        maxHeight: 520,
      }}
    >
      <div
        className="jobs-panel__header"
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          width: "100%",
        }}
      >
        <h3
          style={{
            margin: 0,
            fontSize: "0.95rem",
            fontWeight: 600,
            fontFamily: "Georgia, 'Times New Roman', serif",
          }}
        >
          Sync Jobs
        </h3>
        <span
          className={`pill ${connected ? "pill--passing" : "pill--failing"}`}
          style={{ fontSize: "0.75rem" }}
        >
          {connected ? "Connected" : "Disconnected"}
        </span>
      </div>

      {error && (
        <p
          style={{
            margin: 0,
            fontSize: "0.82rem",
            color: "var(--red)",
          }}
        >
          {error}
        </p>
      )}

      {!hasJobs && (
        <div
          className="jobs-panel__empty"
          style={{
            flex: 1,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            padding: "40px 20px",
          }}
        >
        <p
          style={{
            margin: 0,
            fontSize: "0.88rem",
            color: "rgba(224, 230, 241, 0.55)",
            textAlign: "center",
          }}
        >
          No active jobs
        </p>
        </div>
      )}

      {hasJobs && (
        <div
          className="jobs-panel__list"
          style={{
            flex: 1,
            overflowY: "auto",
            width: "100%",
            paddingRight: 4,
          }}
        >
          {jobs.map((job) => (
            <JobRow key={job.id} job={job} />
          ))}
        </div>
      )}
    </div>
  );
}
