import React, { useCallback, useEffect, useRef, useState } from "react";

const DEFAULT_REPO = "opencode-ai/opencode";

interface SyncJob {
  id: string;
  repo: string;
  status: "pending" | "in_progress" | "paused" | "completed" | "failed";
  error_message: string | null;
  created_at: string;
  updated_at: string;
  next_scheduled_at?: string;
}

interface RateLimitStatus {
  remaining?: number;
  limit?: number;
  reset_at?: string;
}

interface SyncJobsResponse {
  jobs: SyncJob[];
  rate_limit?: RateLimitStatus;
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

function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 0) {
    return `${diffDays}d ago`;
  }
  if (diffHours > 0) {
    return `${diffHours}h ago`;
  }
  if (diffMins > 0) {
    return `${diffMins}m ago`;
  }
  return "just now";
}

function formatResumeTime(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export default function RateLimitMonitor() {
  const [activeJobs, setActiveJobs] = useState<SyncJob[]>([]);
  const [pausedJobs, setPausedJobs] = useState<SyncJob[]>([]);
  const [recentJobs, setRecentJobs] = useState<SyncJob[]>([]);
  const [rateLimit, setRateLimit] = useState<RateLimitStatus | undefined>(undefined);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [repo] = useState(DEFAULT_REPO);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchJobs = useCallback(async () => {
    setIsLoading((prev) => prev === false ? false : prev);
    try {
      const [jobsResponse, pausedResponse] = await Promise.all([
        fetch(`${apiBaseUrl()}${repoPath(repo)}/sync/jobs`).catch(() => null),
        fetch(`${apiBaseUrl()}${repoPath(repo)}/sync/jobs/paused`).catch(() => null),
      ]);

      let jobsData: SyncJobsResponse | null = null;
      let pausedData: SyncJobsResponse | null = null;

      if (jobsResponse?.ok) {
        jobsData = await jobsResponse.json();
      }
      if (pausedResponse?.ok) {
        pausedData = await pausedResponse.json();
      }

      if (!jobsResponse?.ok && !pausedResponse?.ok) {
        setActiveJobs([]);
        setPausedJobs([]);
        setRecentJobs([]);
        setRateLimit(undefined);
        setError(null);
        return;
      }

      const allJobs: SyncJob[] = [];
      if (jobsData?.jobs) {
        allJobs.push(...jobsData.jobs);
      }
      if (pausedData?.jobs) {
        allJobs.push(...pausedData.jobs);
      }

      const active = allJobs.filter((j) => j.status === "in_progress");
      const paused = allJobs.filter((j) => j.status === "paused");
      const completed = allJobs
        .filter((j) => j.status === "completed" || j.status === "failed")
        .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
        .slice(0, 5);

      setActiveJobs(active);
      setPausedJobs(paused);
      setRecentJobs(completed);

      if (jobsData?.rate_limit) {
        setRateLimit(jobsData.rate_limit);
      } else if (pausedData?.rate_limit) {
        setRateLimit(pausedData.rate_limit);
      }

      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch sync jobs");
    } finally {
      setIsLoading(false);
    }
  }, [repo]);

  useEffect(() => {
    fetchJobs();
    pollingRef.current = setInterval(fetchJobs, 30000);
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
      }
    };
  }, [fetchJobs]);

  const totalActive = activeJobs.length;
  const totalPaused = pausedJobs.length;
  const budgetRemaining = rateLimit?.remaining;
  const budgetTotal = rateLimit?.limit;
  const resetTime = rateLimit?.reset_at
    ? new Date(rateLimit.reset_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : null;

  return (
    <div className="sync-panel hero-panel" style={{ flexDirection: "column", gap: 16, padding: 18 }}>
      <div className="sync-panel__header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%" }}>
        <h3 style={{ margin: 0, fontSize: "0.95rem", fontWeight: 600 }}>Rate Limit Monitor</h3>
        {budgetRemaining !== undefined && (
          <span className={`pill ${budgetRemaining < 100 ? "pill--failing" : budgetRemaining < 300 ? "pill--passing" : "cluster-status"}`}>
            {budgetRemaining.toLocaleString()} / {budgetTotal?.toLocaleString() ?? "∞"} remaining
          </span>
        )}
      </div>

      {error && (
        <p style={{ margin: 0, fontSize: "0.85rem", color: "var(--danger)" }}>
          {error}
        </p>
      )}

      {isLoading && activeJobs.length === 0 && pausedJobs.length === 0 && recentJobs.length === 0 && (
        <p style={{ margin: 0, fontSize: "0.85rem", color: "rgba(23,34,53,0.65)" }}>
          Loading...
        </p>
      )}

      {!isLoading && activeJobs.length === 0 && pausedJobs.length === 0 && recentJobs.length === 0 && !error && (
        <p style={{ margin: 0, fontSize: "0.85rem", color: "rgba(23,34,53,0.65)" }}>
          No sync jobs found
        </p>
      )}

      {activeJobs.length > 0 && (
        <div style={{ width: "100%" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
            <span className="pill pill--passing">Active</span>
            <span style={{ fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
              {totalActive} job{totalActive !== 1 ? "s" : ""} in progress
            </span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {activeJobs.map((job) => (
              <div
                key={job.id}
                style={{
                  padding: "10px 12px",
                  background: "rgba(23,34,53,0.04)",
                  borderRadius: 8,
                  border: "1px solid rgba(23,34,53,0.08)",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <span style={{ fontSize: "0.85rem", fontWeight: 500 }}>{job.repo}</span>
                  <span style={{ fontSize: "0.75rem", color: "rgba(23,34,53,0.55)" }}>
                    {formatTimeAgo(job.created_at)}
                  </span>
                </div>
                {job.error_message && (
                  <p style={{ margin: "4px 0 0", fontSize: "0.78rem", color: "var(--danger)" }}>
                    {job.error_message}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {pausedJobs.length > 0 && (
        <div style={{ width: "100%" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
            <span className="pill pill--failing">Paused</span>
            <span style={{ fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
              {totalPaused} job{totalPaused !== 1 ? "s" : ""} waiting
            </span>
            {resetTime && (
              <span style={{ fontSize: "0.78rem", color: "rgba(23,34,53,0.55)" }}>
                (resumes at {resetTime})
              </span>
            )}
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {pausedJobs.map((job) => (
              <div
                key={job.id}
                style={{
                  padding: "10px 12px",
                  background: "rgba(255,199,41,0.08)",
                  borderRadius: 8,
                  border: "1px solid rgba(255,199,41,0.2)",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <span style={{ fontSize: "0.85rem", fontWeight: 500 }}>{job.repo}</span>
                  {job.next_scheduled_at && (
                    <span style={{ fontSize: "0.75rem", color: "rgba(23,34,53,0.65)" }}>
                      resumes {formatResumeTime(job.next_scheduled_at)}
                    </span>
                  )}
                </div>
                {job.error_message && (
                  <p style={{ margin: "4px 0 0", fontSize: "0.78rem", color: "var(--danger)" }}>
                    {job.error_message}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {recentJobs.length > 0 && (
        <div style={{ width: "100%" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
            <span className="cluster-status">Recent</span>
            <span style={{ fontSize: "0.82rem", color: "rgba(23,34,53,0.65)" }}>
              Last {recentJobs.length} completed
            </span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            {recentJobs.map((job) => (
              <div
                key={job.id}
                style={{
                  padding: "8px 12px",
                  background: "transparent",
                  borderRadius: 6,
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span
                    style={{
                      width: 8,
                      height: 8,
                      borderRadius: "50%",
                      background: job.status === "completed" ? "var(--mint)" : "var(--danger)",
                    }}
                  />
                  <span style={{ fontSize: "0.82rem", color: "rgba(23,34,53,0.75)" }}>{job.repo}</span>
                </div>
                <span style={{ fontSize: "0.75rem", color: "rgba(23,34,53,0.55)" }}>
                  {formatTimeAgo(job.updated_at)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {resetTime && budgetRemaining !== undefined && (
        <p style={{ margin: 0, fontSize: "0.78rem", color: "rgba(23,34,53,0.55)", textAlign: "center" }}>
          Rate limit resets at {resetTime}
        </p>
      )}
    </div>
  );
}
