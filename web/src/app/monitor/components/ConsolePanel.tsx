import React, { useEffect, useRef } from "react";
import { useMonitorData } from "../../../hooks/useMonitorData";
import type { LogEntry } from "../../../types/monitor";

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  const hours = date.getHours().toString().padStart(2, "0");
  const minutes = date.getMinutes().toString().padStart(2, "0");
  const seconds = date.getSeconds().toString().padStart(2, "0");
  return `${hours}:${minutes}:${seconds}`;
}

function getLevelColor(level: string): string {
  switch (level.toLowerCase()) {
    case "info":
      return "rgba(23, 34, 53, 0.75)";
    case "warn":
    case "warning":
      return "#f59e0b";
    case "error":
      return "var(--danger)";
    case "debug":
      return "#06b6d4";
    default:
      return "rgba(23, 34, 53, 0.75)";
  }
}

interface LogEntryRowProps {
  entry: LogEntry;
}

function LogEntryRow({ entry }: LogEntryRowProps) {
  const timestamp = formatTimestamp(entry.timestamp);
  const color = getLevelColor(entry.level);
  const formattedLevel = entry.level.toUpperCase();

  return (
    <div
      className="log-entry"
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 10,
        padding: "8px 0",
        borderBottom: "1px solid rgba(23, 34, 53, 0.06)",
        fontSize: "0.82rem",
        lineHeight: 1.5,
      }}
    >
      <span
        className="log-entry__timestamp"
        style={{
          minWidth: 68,
          fontFamily: "'Courier New', Courier, monospace",
          color: "rgba(23, 34, 53, 0.45)",
          fontSize: "0.78rem",
        }}
      >
        [{timestamp}]
      </span>

      <span
        className="log-entry__level"
        style={{
          minWidth: 52,
          fontWeight: 600,
          color: color,
          textTransform: "uppercase",
          fontSize: "0.72rem",
          letterSpacing: "0.04em",
        }}
      >
        [{formattedLevel}]
      </span>

      <span
        className="log-entry__repo"
        style={{
          minWidth: 140,
          maxWidth: 140,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          color: "rgba(23, 34, 53, 0.55)",
          fontSize: "0.78rem",
        }}
        title={entry.repo}
      >
        [{entry.repo}]
      </span>

      <span
        className="log-entry__message"
        style={{
          flex: 1,
          minWidth: 0,
          color: "var(--ink)",
          wordBreak: "break-word",
        }}
      >
        {entry.message}
      </span>
    </div>
  );
}

export default function ConsolePanel() {
  const { logs, connected, error } = useMonitorData();
  const scrollRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  const hasLogs = logs && logs.length > 0;

  if (error) {
    return (
      <div
        className="console-panel hero-panel"
        style={{
          flexDirection: "column",
          gap: 12,
          padding: 18,
          maxHeight: 520,
        }}
      >
        <div
          className="console-panel__header"
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
            Debug Console
          </h3>
        </div>
        <p
          style={{
            margin: 0,
            fontSize: "0.82rem",
            color: "var(--danger)",
          }}
        >
          {error}
        </p>
      </div>
    );
  }

  if (!connected) {
    return (
      <div
        className="console-panel hero-panel"
        style={{
          flexDirection: "column",
          gap: 12,
          padding: 18,
          maxHeight: 520,
        }}
      >
        <div
          className="console-panel__header"
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
            Debug Console
          </h3>
        </div>
        <p
          style={{
            margin: 0,
            fontSize: "0.85rem",
            color: "rgba(23, 34, 53, 0.65)",
          }}
        >
          Connecting...
        </p>
      </div>
    );
  }

  return (
    <div
      className="console-panel hero-panel"
      style={{
        flexDirection: "column",
        gap: 12,
        padding: 18,
        maxHeight: 520,
      }}
    >
      <div
        className="console-panel__header"
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
          Debug Console
        </h3>
        <span
          className={`pill ${connected ? "pill--passing" : "pill--failing"}`}
          style={{ fontSize: "0.75rem" }}
        >
          {connected ? "Live" : "Disconnected"}
        </span>
      </div>

      {!hasLogs && (
        <div
          className="console-panel__empty"
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
              color: "rgba(23, 34, 53, 0.55)",
              textAlign: "center",
            }}
          >
            No log entries
          </p>
        </div>
      )}

      {hasLogs && (
        <div
          ref={scrollRef}
          className="console-panel__logs"
          style={{
            flex: 1,
            overflowY: "auto",
            width: "100%",
            paddingRight: 4,
            marginTop: 8,
          }}
        >
          {logs.map((entry, index) => (
            <LogEntryRow key={`${entry.timestamp}-${index}`} entry={entry} />
          ))}
        </div>
      )}
    </div>
  );
}
