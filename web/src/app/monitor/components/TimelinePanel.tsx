import React, { useMemo } from "react";
import { useMonitorData } from "../../../hooks/useMonitorData";
import type { ActivityBucket } from "../../../types/monitor";

const BUCKET_COUNT = 16;
const LABEL_INTERVAL = 4;

interface ColorIntensity {
  background: string;
  border: string;
}

function getColorForIntensity(requestCount: number): ColorIntensity {
  if (requestCount === 0) {
    return {
      background: "rgba(0, 217, 255, 0.15)",
      border: "rgba(0, 217, 255, 0.3)",
    };
  }
  if (requestCount < 10) {
    return {
      background: "rgba(0, 217, 255, 0.35)",
      border: "rgba(0, 217, 255, 0.5)",
    };
  }
  if (requestCount < 25) {
    return {
      background: "rgba(0, 217, 255, 0.55)",
      border: "rgba(0, 217, 255, 0.7)",
    };
  }
  if (requestCount < 50) {
    return {
      background: "rgba(0, 217, 255, 0.75)",
      border: "rgba(0, 217, 255, 0.9)",
    };
  }
  return {
    background: "rgba(0, 217, 255, 0.95)",
    border: "rgba(224, 230, 241, 0.2)",
  };
}

function formatTimeLabel(index: number): string {
  const now = new Date();
  const hoursAgo = Math.floor((BUCKET_COUNT - 1 - index) / LABEL_INTERVAL);
  
  if (hoursAgo === 0) {
    return "Now";
  }
  
  const labelTime = new Date(now.getTime() - hoursAgo * 60 * 60 * 1000);
  return labelTime.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function formatBucketTooltip(bucket: ActivityBucket): string {
  const requests = bucket.requestCount.toLocaleString();
  const jobs = bucket.jobCount.toLocaleString();
  const duration = (bucket.avgDuration / 1_000_000).toFixed(1);
  return `${requests} requests, ${jobs} jobs, ${duration}ms avg`;
}

export default function TimelinePanel() {
  const { timeline, connected, error } = useMonitorData();

  const buckets: ActivityBucket[] = useMemo(() => {
    if (!timeline || timeline.length === 0) {
      return Array.from({ length: BUCKET_COUNT }, () => ({
        timeWindow: "",
        requestCount: 0,
        jobCount: 0,
        avgDuration: 0,
      }));
    }
    
    const recent = timeline.slice(-BUCKET_COUNT);
    if (recent.length < BUCKET_COUNT) {
      const padding = Array.from({ length: BUCKET_COUNT - recent.length }, () => ({
        timeWindow: "",
        requestCount: 0,
        jobCount: 0,
        avgDuration: 0,
      }));
      return [...padding, ...recent];
    }
    return recent;
  }, [timeline]);

  const maxRequests = useMemo(() => {
    return Math.max(...buckets.map((b) => b.requestCount), 1);
  }, [buckets]);

  if (error) {
    return (
      <div className="sync-panel hero-panel" style={{ flexDirection: "column", gap: 12, padding: 18 }}>
        <div className="sync-panel__header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%" }}>
          <h3 style={{ margin: 0, fontSize: "0.95rem", fontWeight: 600 }}>Timeline - Last 4 Hours</h3>
        </div>
        <p style={{ margin: 0, fontSize: "0.85rem", color: "var(--red)" }}>
          {error}
        </p>
      </div>
    );
  }

  if (!connected) {
    return (
      <div className="sync-panel hero-panel" style={{ flexDirection: "column", gap: 12, padding: 18 }}>
        <div className="sync-panel__header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%" }}>
          <h3 style={{ margin: 0, fontSize: "0.95rem", fontWeight: 600 }}>Timeline - Last 4 Hours</h3>
        </div>
        <p style={{ margin: 0, fontSize: "0.85rem", color: "rgba(224,230,241,0.65)" }}>
          Connecting...
        </p>
      </div>
    );
  }

  const hasData = buckets.some((b) => b.requestCount > 0);

  return (
    <div className="sync-panel hero-panel" style={{ flexDirection: "column", gap: 16, padding: 18 }}>
      <div className="sync-panel__header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%" }}>
        <h3 style={{ margin: 0, fontSize: "0.95rem", fontWeight: 600 }}>Timeline - Last 4 Hours</h3>
        <span className="cluster-status">
          {hasData ? `${buckets.reduce((sum, b) => sum + b.requestCount, 0).toLocaleString()} requests` : "No activity"}
        </span>
      </div>

      {!hasData ? (
        <div style={{ width: "100%", padding: "40px 20px", textAlign: "center", background: "rgba(0, 217, 255, 0.08)", borderRadius: 12, border: "2px dashed var(--line)" }}>
          <p style={{ margin: 0, fontSize: "0.9rem", color: "rgba(224,230,241,0.5)" }}>
            No activity in the last 4 hours
          </p>
        </div>
      ) : (
        <>
          <div style={{ width: "100%" }}>
            <div style={{ display: "flex", gap: 3, width: "100%" }}>
              {buckets.map((bucket, index) => {
                const colors = getColorForIntensity(bucket.requestCount);
                const tooltip = formatBucketTooltip(bucket);
                
                return (
                  <div
                    key={index}
                    title={tooltip}
                    style={{
                      flex: 1,
                      height: 48,
                      borderRadius: 6,
                      background: colors.background,
                      border: `1px solid ${colors.border}`,
                      transition: "background 300ms ease",
                      cursor: "default",
                    }}
                  />
                );
              })}
            </div>
          </div>

          <div style={{ width: "100%", display: "flex", justifyContent: "space-between", marginTop: 4 }}>
            {buckets.map((_, index) => {
              if (index % LABEL_INTERVAL !== 0 && index !== BUCKET_COUNT - 1) {
                return <div key={index} style={{ flex: 1 }} />;
              }
              
              return (
                <div
                  key={index}
                  style={{
                    flex: 1,
                    textAlign: index === 0 ? "left" : index === BUCKET_COUNT - 1 ? "right" : "center",
                    fontSize: "0.72rem",
                    color: "rgba(224,230,241,0.55)",
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                  }}
                >
                  {formatTimeLabel(index)}
                </div>
              );
            })}
          </div>

          <div style={{ width: "100%", display: "flex", justifyContent: "center", gap: 16, marginTop: 8, paddingTop: 12, borderTop: `1px solid var(--line)` }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <div style={{ width: 16, height: 16, borderRadius: 4, background: "rgba(0, 217, 255, 0.15)", border: "1px solid rgba(0, 217, 255, 0.3)" }} />
              <span style={{ fontSize: "0.68rem", color: "rgba(224,230,241,0.55)", textTransform: "uppercase", letterSpacing: "0.05em" }}>Low</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <div style={{ width: 16, height: 16, borderRadius: 4, background: "rgba(0, 217, 255, 0.55)", border: "1px solid rgba(0, 217, 255, 0.7)" }} />
              <span style={{ fontSize: "0.68rem", color: "rgba(224,230,241,0.55)", textTransform: "uppercase", letterSpacing: "0.05em" }}>Medium</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <div style={{ width: 16, height: 16, borderRadius: 4, background: "rgba(0, 217, 255, 0.95)", border: "1px solid rgba(224, 230, 241, 0.2)" }} />
              <span style={{ fontSize: "0.68rem", color: "rgba(224,230,241,0.55)", textTransform: "uppercase", letterSpacing: "0.05em" }}>High</span>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
