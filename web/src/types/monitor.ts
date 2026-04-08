// Monitor data types matching Go internal/monitor/data/models.go structures

export interface SyncJobView {
  id: string;
  repo: string;
  progress: number;
  status: string;
  detail: string;
  eta: number; // duration in nanoseconds
  batch: number;
}

export interface RateLimitPoint {
  timestamp: string;
  remaining: number;
  used: number;
}

export interface RateLimitView {
  remaining: number;
  total: number;
  resetTime: string;
  usageHistory: RateLimitPoint[];
}

export interface LogEntry {
  timestamp: string;
  level: string;
  repo: string;
  message: string;
  metadata: Record<string, string>;
}

export interface ActivityBucket {
  timeWindow: string;
  requestCount: number;
  jobCount: number;
  avgDuration: number; // duration in nanoseconds
}

export interface DataUpdate {
  timestamp: string;
  syncJobs: SyncJobView[];
  rateLimit: RateLimitView;
  recentLogs: LogEntry[];
  activityBuckets: ActivityBucket[];
}

export interface MonitorData {
  jobs: SyncJobView[];
  rateLimit: RateLimitView | null;
  timeline: ActivityBucket[];
  logs: LogEntry[];
  connected: boolean;
  error: string | null;
}
