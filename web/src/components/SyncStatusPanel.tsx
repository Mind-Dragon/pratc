import React, { useCallback, useEffect, useRef, useState } from "react";

const DEFAULT_REPO = "opencode-ai/opencode";

type SyncStatus = "idle" | "syncing" | "error" | "complete";

type SyncPhase = "mirroring" | "metadata" | "clustering" | "done";

interface SyncProgress {
  processed: number;
  total: number;
  eta_seconds: number;
  phase?: SyncPhase;
  prs_synced?: number;
  errors?: number;
}

interface RepoStats {
  cache_size: number;
  total_prs?: number;
}

interface SyncStatusResponse {
  repo: string;
  last_sync: string | null;
  pr_count: number;
  status: string;
  in_progress: boolean;
  progress_percent: number;
}

interface ScalabilityMetrics {
  totalPRs: number | null;
  syncDuration: number | null;
  throughput: number | null;
  errorCount: number;
}

interface DriftEntry {
  pr_number: number;
  local_sha: string;
  remote_sha: string;
}

function apiBaseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_PRATC_API_URL;
  if (configured && configured.trim().length > 0) {
    return configured.replace(/\/$/, "");
  }
  return "http://localhost:8080";
}

function repoPath(repo: string): string {
  const [owner, name] = repo.split("/");
  if (owner && name) {
    return `/api/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}`;
  }
  return `/api/repos/${encodeURIComponent(repo)}`;
}

export default function SyncStatusPanel() {
  const [status, setStatus] = useState<SyncStatus>("idle");
  const [phase, setPhase] = useState<SyncPhase | null>(null);
  const [progress, setProgress] = useState<SyncProgress | null>(null);
  const [driftEntries, setDriftEntries] = useState<DriftEntry[]>([]);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [lastSyncAt, setLastSyncAt] = useState<string | null>(null);
  const [lastSyncedCount, setLastSyncedCount] = useState(0);
  const [lastSyncDuration, setLastSyncDuration] = useState<number | null>(null);
  const [cacheSize, setCacheSize] = useState<number | null>(null);
  const [totalPRs, setTotalPRs] = useState<number | null>(null);
  const [errorCount, setErrorCount] = useState(0);
  const [syncStartTime, setSyncStartTime] = useState<number | null>(null);
  const [showDrift, setShowDrift] = useState(false);
  const [showMetrics, setShowMetrics] = useState(false);
  const [repo] = useState(DEFAULT_REPO);
  const eventSourceRef = useRef<EventSource | null>(null);
  const statusRef = useRef<SyncStatus>(status);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  statusRef.current = status;

  const connectSSE = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const url = `${apiBaseUrl()}${repoPath(repo)}/sync/stream`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.addEventListener("progress", (e) => {
      try {
        const data = JSON.parse(e.data) as SyncProgress;
        setProgress(data);
        setLastSyncedCount(data.prs_synced ?? data.processed);
        setStatus("syncing");
        if (data.phase) {
          setPhase(data.phase);
        }
        if (data.errors !== undefined) {
          setErrorCount(data.errors);
        }
      } catch {}
    });

    es.addEventListener("complete", () => {
      setStatus("complete");
      setPhase("done");
      setLastSyncAt(new Date().toLocaleTimeString());
      setProgress(null);
      if (syncStartTime) {
        const duration = (Date.now() - syncStartTime) / 1000;
        setLastSyncDuration(duration);
      }
      es.close();
      eventSourceRef.current = null;
      fetchStats();
    });

    es.addEventListener("drift_detected", (e) => {
      try {
        const data = JSON.parse(e.data) as DriftEntry;
        setDriftEntries((prev) => [...prev, data]);
      } catch {}
    });

    es.addEventListener("error", (e) => {
      if (e instanceof MessageEvent) {
        try {
          const data = JSON.parse(e.data);
          setErrorMsg(data.message ?? "Sync failed");
          setStatus("error");
        } catch {
          setStatus("idle");
        }
      }
      es.close();
      eventSourceRef.current = null;
    });

    es.onerror = () => {
      if (statusRef.current === "syncing") {
        setStatus("error");
        setErrorMsg("Connection lost");
      }
      es.close();
      eventSourceRef.current = null;
    };
  }, [repo, syncStartTime]);

  const fetchStats = useCallback(async () => {
    try {
      const response = await fetch(`${apiBaseUrl()}${repoPath(repo)}/stats`);
      if (response.ok) {
        const data = (await response.json()) as RepoStats;
        setCacheSize(data.cache_size ?? 0);
        if (data.total_prs !== undefined) {
          setTotalPRs(data.total_prs);
        }
      }
    } catch {
    }
  }, [repo]);

  const fetchSyncStatus = useCallback(async () => {
    try {
      const response = await fetch(`${apiBaseUrl()}${repoPath(repo)}/sync/status`);
      if (response.ok) {
        const data = (await response.json()) as SyncStatusResponse;
        if (data.last_sync) {
          setLastSyncAt(data.last_sync);
        }
        if (data.pr_count !== undefined && data.pr_count > 0) {
          setLastSyncedCount(data.pr_count);
        }
        if (data.in_progress && statusRef.current !== "syncing") {
          setStatus("syncing");
        }
      }
    } catch {
    }
  }, [repo]);

  const startSync = useCallback(async () => {
    setStatus("syncing");
    setPhase(null);
    setErrorMsg(null);
    setErrorCount(0);
    setDriftEntries([]);
    setProgress(null);
    setSyncStartTime(Date.now());

    try {
      const response = await fetch(`${apiBaseUrl()}${repoPath(repo)}/sync`, { method: "POST" });
      if (!response.ok) {
        setErrorMsg(`Sync request failed: ${response.status}`);
        setStatus("error");
        return;
      }
      connectSSE();
    } catch {
      setErrorMsg("Unable to reach API server");
      setStatus("error");
    }
  }, [repo, connectSSE]);

  useEffect(() => {
    fetchStats();
    fetchSyncStatus();
    connectSSE();
    pollingRef.current = setInterval(fetchSyncStatus, 30000);
    return () => {
      eventSourceRef.current?.close();
      eventSourceRef.current = null;
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
      }
    };
  }, [connectSSE, fetchStats, fetchSyncStatus]);

  const pct = progress ? Math.round((progress.processed / progress.total) * 100) : 0;
  const eta = progress?.eta_seconds
    ? progress.eta_seconds >= 60
      ? `${Math.floor(progress.eta_seconds / 60)}m ${Math.round(progress.eta_seconds % 60)}s`
      : `${Math.round(progress.eta_seconds)}s`
    : null;

  const throughput = lastSyncDuration && lastSyncDuration > 0 && lastSyncedCount > 0
    ? (lastSyncedCount / lastSyncDuration) * 60
    : null;

  const phaseLabel = phase
    ? phase === "mirroring"
      ? "Fetching refs"
      : phase === "metadata"
        ? "Syncing metadata"
        : phase === "clustering"
          ? "Building clusters"
          : phase === "done"
            ? "Complete"
            : phase
    : null;

  return (
    <div className="sync-panel hero-panel" style={{ flexDirection: "column", gap: 12, padding: 18 }}>
      <div className="sync-panel__header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span
            className={`pill ${status === "syncing" ? "pill--passing" : status === "error" ? "pill--failing" : "cluster-status"}`}
          >
            {status === "syncing" ? "Syncing" : status === "error" ? "Error" : status === "complete" ? "Up to date" : "Idle"}
          </span>
          {driftEntries.length > 0 && (
            <button
              type="button"
              onClick={() => setShowDrift(!showDrift)}
              style={{ fontSize: "0.82rem", color: "var(--danger)", background: "none", border: "none", cursor: "pointer", padding: 0 }}
            >
              {driftEntries.length} drift {showDrift ? "▲" : "▼"}
            </button>
          )}
        </div>
        <button
          type="button"
          disabled={status === "syncing"}
          onClick={startSync}
          style={{
            border: "1px solid var(--line)",
            borderRadius: 999,
            padding: "6px 14px",
            background: status === "syncing" ? "rgba(23,34,53,0.08)" : "rgba(255,255,255,0.8)",
            opacity: status === "syncing" ? 0.5 : 1,
            cursor: status === "syncing" ? "wait" : "pointer",
          }}
        >
          {status === "syncing" ? "Syncing..." : "Sync Now"}
        </button>
        <button
          type="button"
          onClick={() => {
            if (typeof window !== "undefined") {
              window.location.href = "/reports";
            }
          }}
          style={{
            border: "1px solid var(--line)",
            borderRadius: 999,
            padding: "6px 14px",
            background: "rgba(255,255,255,0.8)",
            cursor: "pointer",
          }}
        >
          Download Report
        </button>
      </div>

      {status === "syncing" && progress && (
        <div style={{ width: "100%" }}>
          {phaseLabel && (
            <p style={{ margin: "0 0 6px", fontSize: "0.78rem", textTransform: "uppercase", letterSpacing: "0.08em", color: "rgba(23,34,53,0.55)" }}>
              Phase: {phaseLabel}
            </p>
          )}
          <div
            style={{
              width: "100%",
              height: 8,
              borderRadius: 999,
              background: "rgba(23,34,53,0.08)",
              overflow: "hidden",
            }}
          >
            <div
              style={{
                width: `${pct}%`,
                height: "100%",
                borderRadius: 999,
                background: "linear-gradient(90deg, var(--sky), var(--mint))",
                transition: "width 300ms ease",
              }}
            />
          </div>
          <p style={{ margin: "6px 0 0", fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
            {progress.processed.toLocaleString()} / {progress.total.toLocaleString()} PRs ({pct}%)
            {eta && ` — ETA ${eta}`}
            {errorCount > 0 && (
              <span style={{ color: "var(--danger)", marginLeft: 8 }}>
                ({errorCount} error{errorCount !== 1 ? "s" : ""})
              </span>
            )}
          </p>
        </div>
      )}

      {errorMsg && (
        <p style={{ margin: 0, fontSize: "0.85rem", color: "var(--danger)" }}>
          {errorMsg}
          {" "}
          <button
            type="button"
            onClick={startSync}
            style={{ background: "none", border: "none", color: "var(--danger)", textDecoration: "underline", cursor: "pointer", padding: 0 }}
          >
            Retry
          </button>
        </p>
      )}

      <p style={{ margin: 0, fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
        Last sync: {lastSyncAt ?? "Never"}
      </p>
      <p style={{ margin: 0, fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
        PRs synced: {lastSyncedCount.toLocaleString()}
      </p>
      <p style={{ margin: 0, fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
        Cache size: {cacheSize !== null ? `${cacheSize.toLocaleString()} PRs` : "—"}
      </p>
      {throughput !== null && (
        <p style={{ margin: 0, fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
          Sync rate: {throughput.toFixed(1)} PRs/min
        </p>
      )}
      {errorCount > 0 && status !== "syncing" && (
        <p style={{ margin: 0, fontSize: "0.82rem", color: "var(--danger)" }}>
          Errors: {errorCount}
        </p>
      )}

      <button
        type="button"
        onClick={() => setShowMetrics(!showMetrics)}
        style={{
          marginTop: 8,
          fontSize: "0.78rem",
          color: "rgba(23,34,53,0.65)",
          background: "none",
          border: "none",
          cursor: "pointer",
          padding: 0,
          display: "flex",
          alignItems: "center",
          gap: 4,
        }}
      >
        Scalability Metrics {showMetrics ? "▲" : "▼"}
      </button>

      {showMetrics && (
        <div
          style={{
            width: "100%",
            padding: "12px",
            background: "rgba(23,34,53,0.04)",
            borderRadius: 12,
            marginTop: 8,
          }}
        >
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 12 }}>
            <div>
              <p style={{ margin: 0, fontSize: "0.72rem", textTransform: "uppercase", letterSpacing: "0.08em", color: "rgba(23,34,53,0.55)" }}>
                Total PRs
              </p>
              <p style={{ margin: "4px 0 0", fontSize: "1.1rem", fontWeight: 600 }}>
                {totalPRs !== null ? totalPRs.toLocaleString() : cacheSize !== null ? cacheSize.toLocaleString() : "—"}
              </p>
            </div>
            <div>
              <p style={{ margin: 0, fontSize: "0.72rem", textTransform: "uppercase", letterSpacing: "0.08em", color: "rgba(23,34,53,0.55)" }}>
                Duration
              </p>
              <p style={{ margin: "4px 0 0", fontSize: "1.1rem", fontWeight: 600 }}>
                {lastSyncDuration !== null ? `${lastSyncDuration.toFixed(1)}s` : "—"}
              </p>
            </div>
            <div>
              <p style={{ margin: 0, fontSize: "0.72rem", textTransform: "uppercase", letterSpacing: "0.08em", color: "rgba(23,34,53,0.55)" }}>
                Throughput
              </p>
              <p style={{ margin: "4px 0 0", fontSize: "1.1rem", fontWeight: 600 }}>
                {throughput !== null ? `${throughput.toFixed(1)} PRs/min` : "—"}
              </p>
            </div>
            <div>
              <p style={{ margin: 0, fontSize: "0.72rem", textTransform: "uppercase", letterSpacing: "0.08em", color: "rgba(23,34,53,0.55)" }}>
                Errors
              </p>
              <p style={{ margin: "4px 0 0", fontSize: "1.1rem", fontWeight: 600, color: errorCount > 0 ? "var(--danger)" : "inherit" }}>
                {errorCount}
              </p>
            </div>
          </div>
        </div>
      )}

      {showDrift && driftEntries.length > 0 && (
        <div style={{ width: "100%", maxHeight: 120, overflowY: "auto", fontSize: "0.82rem" }}>
          <table className="triage-table" style={{ width: "100%", borderCollapse: "collapse" }}>
            <thead>
              <tr>
                <th style={{ padding: "6px 8px", textAlign: "left", fontSize: "0.72rem", textTransform: "uppercase", letterSpacing: "0.08em" }}>PR</th>
                <th style={{ padding: "6px 8px", textAlign: "left", fontSize: "0.72rem", textTransform: "uppercase", letterSpacing: "0.08em" }}>Local</th>
                <th style={{ padding: "6px 8px", textAlign: "left", fontSize: "0.72rem", textTransform: "uppercase", letterSpacing: "0.08em" }}>Remote</th>
              </tr>
            </thead>
            <tbody>
              {driftEntries.map((d) => (
                <tr key={d.pr_number}>
                  <td style={{ padding: "4px 8px" }}>#{d.pr_number}</td>
                  <td style={{ padding: "4px 8px", fontFamily: "monospace", fontSize: "0.78rem" }}>{d.local_sha.slice(0, 7)}</td>
                  <td style={{ padding: "4px 8px", fontFamily: "monospace", fontSize: "0.78rem" }}>{d.remote_sha.slice(0, 7)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
