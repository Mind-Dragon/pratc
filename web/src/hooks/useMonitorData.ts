import { useCallback, useEffect, useRef, useState } from "react";
import type { DataUpdate, MonitorData } from "../types/monitor";

const MAX_RETRIES = 3;
const INITIAL_RECONNECT_DELAY = 1000; // 1 second
const MAX_RECONNECT_DELAY = 30000; // 30 seconds

function getWebSocketUrl(): string {
  const configured = process.env.NEXT_PUBLIC_PRATC_WS_URL;
  if (configured && configured.trim().length > 0) {
    return configured.replace(/\/$/, "");
  }
  // Default to ws://localhost:7400/monitor/stream
  const apiUrl = process.env.NEXT_PUBLIC_PRATC_API_URL || "http://localhost:7400";
  return apiUrl.replace(/^http/, "ws") + "/monitor/stream";
}

export function useMonitorData(): MonitorData {
  const [jobs, setJobs] = useState<DataUpdate["syncJobs"]>([]);
  const [rateLimit, setRateLimit] = useState<DataUpdate["rateLimit"] | null>(null);
  const [timeline, setTimeline] = useState<DataUpdate["activityBuckets"]>([]);
  const [logs, setLogs] = useState<DataUpdate["recentLogs"]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const retryCountRef = useRef(0);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const cleanup = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const connect = useCallback(() => {
    const url = getWebSocketUrl();
    const ws = new WebSocket(url);

    ws.onopen = () => {
      retryCountRef.current = 0;
      setConnected(true);
      setError(null);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as DataUpdate;
        if (data.syncJobs) {
          setJobs(data.syncJobs);
        }
        if (data.rateLimit) {
          setRateLimit(data.rateLimit);
        }
        if (data.activityBuckets) {
          setTimeline(data.activityBuckets);
        }
        if (data.recentLogs) {
          setLogs(data.recentLogs);
        }
      } catch (e) {
        console.error("Failed to parse WebSocket message:", e);
      }
    };

    ws.onclose = () => {
      setConnected(false);
      // Attempt reconnection if not explicitly closed and under retry limit
      if (retryCountRef.current < MAX_RETRIES) {
        const delay = Math.min(
          INITIAL_RECONNECT_DELAY * Math.pow(2, retryCountRef.current),
          MAX_RECONNECT_DELAY
        );
        retryCountRef.current += 1;
        reconnectTimeoutRef.current = setTimeout(() => {
          connect();
        }, delay);
      } else {
        setError("Connection closed. Maximum retries reached.");
      }
    };

    ws.onerror = () => {
      setError("WebSocket connection error");
    };

    wsRef.current = ws;
  }, []);

  useEffect(() => {
    connect();
    return () => {
      cleanup();
    };
  }, [connect, cleanup]);

  return {
    jobs,
    rateLimit,
    timeline,
    logs,
    connected,
    error,
  };
}
