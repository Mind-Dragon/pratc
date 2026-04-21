# prATC Monitor WebSocket API

**Last Updated:** 2026-04-09  
**API Version:** v1.0  
**Endpoint:** `ws://localhost:7400/monitor/stream`

## Overview

The prATC Monitor WebSocket API provides real-time streaming of system metrics, sync job progress, GitHub rate limit status, and activity timelines. This endpoint can be used by any client that needs live updates, including API consumers and monitoring tools.

### Use Cases

- Real-time sync job progress tracking
- GitHub API rate limit monitoring
- Activity timeline visualization
- System health monitoring

### Connection Model

The WebSocket uses a **publish-subscribe** pattern. The server broadcasts `DataUpdate` messages to all connected clients whenever state changes. Clients receive updates automatically without sending subscription messages.

---

## Connection

### Endpoint URL

```
ws://localhost:7400/monitor/stream
```

**Note:** The port defaults to 7400 (prATC reserved range: 7400-7500). Configure via `PRATC_PORT` environment variable.

### Protocol

- **Protocol:** WebSocket (RFC 6455)
- **Subprotocol:** None required
- **Message Format:** JSON text frames
- **Heartbeat:** Server sends ping frames every 30 seconds

### Handshake

Standard WebSocket upgrade from HTTP:

```http
GET /monitor/stream HTTP/1.1
Host: localhost:7400
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13
```

**Response:**

```http
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

### Authentication

No authentication is currently implemented. Run behind a reverse proxy with authentication for production use.

### CORS

The WebSocket endpoint allows connections from any origin (`CheckOrigin: true`). Configure your reverse proxy for stricter origin control in production.

---

## Message Format

All messages are JSON text frames with the following structure:

### DataUpdate Message

```json
{
  "timestamp": "2026-04-09T10:30:00Z",
  "syncJobs": [
    {
      "id": "sync-001",
      "repo": "owner/repo",
      "progress": 75,
      "status": "active",
      "detail": "Fetching PRs 100-200",
      "eta": 45000000000,
      "batch": 2
    }
  ],
  "rateLimit": {
    "remaining": 4500,
    "total": 5000,
    "resetTime": "2026-04-09T11:00:00Z",
    "usageHistory": [
      {
        "timestamp": "2026-04-09T10:25:00Z",
        "remaining": 4600,
        "used": 400
      }
    ]
  },
  "recentLogs": [
    {
      "timestamp": "2026-04-09T10:29:55Z",
      "level": "info",
      "repo": "owner/repo",
      "message": "Sync job completed",
      "metadata": {
        "job_id": "sync-001",
        "prs_fetched": 100
      }
    }
  ],
  "activityBuckets": [
    {
      "timeWindow": "2026-04-09T10:00:00Z",
      "requestCount": 150,
      "jobCount": 3,
      "avgDuration": 2500000000
    }
  ]
}
```

### Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string (RFC3339) | Server time when update was generated |
| `syncJobs` | array | Active sync jobs with progress |
| `rateLimit` | object | Current GitHub API rate limit status |
| `recentLogs` | array | Recent log entries for the repo |
| `activityBuckets` | array | Activity metrics over time windows |

---

## Data Types

### SyncJobView

Represents a sync job for monitoring and API responses.

```typescript
interface SyncJobView {
  id: string;           // Unique job identifier
  repo: string;         // Repository in owner/repo format
  progress: number;     // Progress percentage (0-100)
  status: string;       // Job status (see Status Values)
  detail: string;       // Human-readable status detail
  eta: number;          // Estimated time remaining (nanoseconds)
  batch: number;        // Current batch number
}
```

**Status Values:**

| Status | Description |
|--------|-------------|
| `active` | Job is currently running |
| `paused` | Job is paused (rate limit or manual) |
| `failed` | Job failed with an error |
| `queued` | Job is waiting to start |
| `completed` | Job finished successfully |

### RateLimitView

Represents the current GitHub API rate limit state.

```typescript
interface RateLimitView {
  remaining: number;           // Requests remaining in current window
  total: number;               // Total requests allowed per hour
  resetTime: string;           // When the rate limit resets (RFC3339)
  usageHistory: RateLimitPoint[];  // Historical usage data
}

interface RateLimitPoint {
  timestamp: string;   // When the sample was taken (RFC3339)
  remaining: number;     // Remaining requests at that time
  used: number;          // Requests used since last sample
}
```

### LogEntry

Represents a single log line for display.

```typescript
interface LogEntry {
  timestamp: string;              // Log entry time (RFC3339)
  level: string;                  // Log level: debug, info, warn, error
  repo: string;                   // Associated repository
  message: string;                // Log message
  metadata: Record<string, string>;  // Additional structured data
}
```

### ActivityBucket

Represents aggregated activity metrics over a time window.

```typescript
interface ActivityBucket {
  timeWindow: string;    // Start of the time window (RFC3339)
  requestCount: number;  // Number of API requests in window
  jobCount: number;      // Number of sync jobs in window
  avgDuration: number;   // Average job duration (nanoseconds)
}
```

### DataUpdate (Complete TypeScript Definition)

```typescript
export interface DataUpdate {
  timestamp: string;
  syncJobs: SyncJobView[];
  rateLimit: RateLimitView;
  recentLogs: LogEntry[];
  activityBuckets: ActivityBucket[];
}
```

---

## Connection Lifecycle

### 1. Connect

Establish a WebSocket connection to the endpoint:

```javascript
const ws = new WebSocket('ws://localhost:7400/monitor/stream');

ws.onopen = () => {
  console.log('Connected to prATC monitor');
};
```

### 2. Receive Updates

Listen for incoming messages:

```javascript
ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  console.log('Received update:', update);
  
  // Handle different data types
  if (update.syncJobs?.length > 0) {
    updateJobDisplay(update.syncJobs);
  }
  if (update.rateLimit) {
    updateRateLimitDisplay(update.rateLimit);
  }
};
```

**Update Frequency:**

| Data Type | Polling Interval | Notes |
|-----------|------------------|-------|
| Sync Jobs | 2 seconds | Only sent when job state changes |
| Rate Limit | 10 seconds | Only sent when values change |
| Activity Timeline | 30 seconds | Only sent when data changes |

### 3. Handle Heartbeats

The server sends WebSocket ping frames every 30 seconds. The client should respond with pong frames (handled automatically by browser WebSocket implementations).

```javascript
// Browser WebSocket handles ping/pong automatically
// No action needed for standard implementations
```

### 4. Disconnect

Close the connection gracefully:

```javascript
ws.close(1000, 'Client closing connection');
```

### 5. Reconnect

Implement exponential backoff for reconnection:

```javascript
class MonitorClient {
  private ws: WebSocket | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private url = 'ws://localhost:7400/monitor/stream';

  connect() {
    this.ws = new WebSocket(this.url);
    
    this.ws.onopen = () => {
      console.log('Connected');
      this.reconnectDelay = 1000; // Reset delay
    };
    
    this.ws.onclose = () => {
      console.log('Disconnected, reconnecting...');
      this.scheduleReconnect();
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    this.ws.onmessage = (event) => {
      const update = JSON.parse(event.data);
      this.handleUpdate(update);
    };
  }

  private scheduleReconnect() {
    setTimeout(() => {
      this.connect();
      this.reconnectDelay = Math.min(
        this.reconnectDelay * 2,
        this.maxReconnectDelay
      );
    }, this.reconnectDelay);
  }

  private handleUpdate(update: DataUpdate) {
    // Process update
  }

  disconnect() {
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
    }
  }
}
```

---

## Error Handling

### Connection Errors

| Error | Cause | Action |
|-------|-------|--------|
| `ECONNREFUSED` | Server not running | Check if prATC serve is started |
| `ETIMEDOUT` | Network timeout | Retry with backoff |
| `1006 Abnormal Closure` | Server closed unexpectedly | Reconnect with backoff |

### Message Errors

Malformed JSON messages are silently dropped by the server. Clients should handle parse errors:

```javascript
ws.onmessage = (event) => {
  try {
    const update = JSON.parse(event.data);
    handleUpdate(update);
  } catch (err) {
    console.error('Failed to parse message:', err);
    // Continue operating, don't crash
  }
};
```

### Reconnection Strategy

Recommended exponential backoff with jitter:

```
Attempt 1: 1 second
Attempt 2: 2 seconds
Attempt 3: 4 seconds
Attempt 4: 8 seconds
Attempt 5+: 30 seconds (cap)
```

Add random jitter (±20%) to prevent thundering herd.

---

## Code Examples

### JavaScript / TypeScript

```typescript
import { DataUpdate, SyncJobView, RateLimitView } from './types/monitor';

class PratcMonitorClient {
  private ws: WebSocket | null = null;
  private url: string;
  private onUpdate: (update: DataUpdate) => void;

  constructor(url: string, onUpdate: (update: DataUpdate) => void) {
    this.url = url;
    this.onUpdate = onUpdate;
  }

  connect(): void {
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('Connected to prATC monitor');
    };

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const update: DataUpdate = JSON.parse(event.data);
        this.onUpdate(update);
      } catch (err) {
        console.error('Failed to parse update:', err);
      }
    };

    this.ws.onerror = (error: Event) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onclose = () => {
      console.log('Connection closed');
      // Implement reconnection logic here
    };
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
  }
}

// Usage
const client = new PratcMonitorClient(
  'ws://localhost:7400/monitor/stream',
  (update) => {
    console.log('Sync jobs:', update.syncJobs);
    console.log('Rate limit:', update.rateLimit);
  }
);

client.connect();
```

### Go

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/gorilla/websocket"
)

// DataUpdate matches the server message format
type DataUpdate struct {
    Timestamp       time.Time         `json:"timestamp"`
    SyncJobs        []SyncJobView     `json:"syncJobs"`
    RateLimit       RateLimitView     `json:"rateLimit"`
    RecentLogs      []LogEntry        `json:"recentLogs"`
    ActivityBuckets []ActivityBucket  `json:"activityBuckets"`
}

type SyncJobView struct {
    ID       string        `json:"id"`
    Repo     string        `json:"repo"`
    Progress int           `json:"progress"`
    Status   string        `json:"status"`
    Detail   string        `json:"detail"`
    ETA      time.Duration `json:"eta"`
    Batch    int           `json:"batch"`
}

type RateLimitView struct {
    Remaining    int               `json:"remaining"`
    Total        int               `json:"total"`
    ResetTime    time.Time         `json:"resetTime"`
    UsageHistory []RateLimitPoint  `json:"usageHistory"`
}

type RateLimitPoint struct {
    Timestamp time.Time `json:"timestamp"`
    Remaining int       `json:"remaining"`
    Used      int       `json:"used"`
}

type LogEntry struct {
    Timestamp time.Time         `json:"timestamp"`
    Level     string            `json:"level"`
    Repo      string            `json:"repo"`
    Message   string            `json:"message"`
    Metadata  map[string]string `json:"metadata"`
}

type ActivityBucket struct {
    TimeWindow   time.Time     `json:"timeWindow"`
    RequestCount int           `json:"requestCount"`
    JobCount     int           `json:"jobCount"`
    AvgDuration  time.Duration `json:"avgDuration"`
}

func main() {
    // Create dialer with timeout
    dialer := websocket.Dialer{
        HandshakeTimeout: 10 * time.Second,
    }

    // Connect to WebSocket
    url := "ws://localhost:7400/monitor/stream"
    headers := http.Header{}
    
    conn, resp, err := dialer.Dial(url, headers)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer resp.Body.Close()
    defer conn.Close()

    fmt.Println("Connected to prATC monitor")

    // Read messages
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    for {
        select {
        case <-ctx.Done():
            fmt.Println("Context cancelled, closing connection")
            return
        default:
        }

        // Set read deadline
        conn.SetReadDeadline(time.Now().Add(60 * time.Second))

        _, message, err := conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                log.Printf("Connection closed unexpectedly: %v", err)
            } else {
                log.Printf("Read error: %v", err)
            }
            return
        }

        var update DataUpdate
        if err := json.Unmarshal(message, &update); err != nil {
            log.Printf("Failed to parse message: %v", err)
            continue
        }

        // Process update
        fmt.Printf("[%s] Jobs: %d, Rate limit: %d/%d\n",
            update.Timestamp.Format("15:04:05"),
            len(update.SyncJobs),
            update.RateLimit.Remaining,
            update.RateLimit.Total,
        )

        for _, job := range update.SyncJobs {
            fmt.Printf("  Job %s: %s (%d%%) - %s\n",
                job.ID,
                job.Status,
                job.Progress,
                job.Detail,
            )
        }
    }
}
```

### React Hook Example

```typescript
import { useEffect, useRef, useState, useCallback } from 'react';
import { DataUpdate, MonitorData } from '../types/monitor';

export function useMonitorData(url: string): MonitorData {
  const [data, setData] = useState<MonitorData>({
    jobs: [],
    rateLimit: null,
    timeline: [],
    logs: [],
    connected: false,
    error: null,
  });
  
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>();

  const connect = useCallback(() => {
    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        setData(prev => ({ ...prev, connected: true, error: null }));
      };

      ws.onmessage = (event) => {
        try {
          const update: DataUpdate = JSON.parse(event.data);
          setData(prev => ({
            ...prev,
            jobs: update.syncJobs,
            rateLimit: update.rateLimit,
            timeline: update.activityBuckets,
            logs: update.recentLogs,
          }));
        } catch (err) {
          console.error('Failed to parse update:', err);
        }
      };

      ws.onerror = (error) => {
        setData(prev => ({ ...prev, error: 'Connection error' }));
      };

      ws.onclose = () => {
        setData(prev => ({ ...prev, connected: false }));
        // Reconnect after 3 seconds
        reconnectTimeoutRef.current = setTimeout(connect, 3000);
      };
    } catch (err) {
      setData(prev => ({ ...prev, error: 'Failed to connect' }));
    }
  }, [url]);

  useEffect(() => {
    connect();
    
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [connect]);

  return data;
}

// Usage in component
function Dashboard() {
  const { jobs, rateLimit, connected, error } = useMonitorData(
    'ws://localhost:7400/monitor/stream'
  );

  if (!connected) return <div>Connecting...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      <h1>Sync Jobs</h1>
      {jobs.map(job => (
        <div key={job.id}>
          {job.repo}: {job.status} ({job.progress}%)
        </div>
      ))}
      
      {rateLimit && (
        <div>
          Rate Limit: {rateLimit.remaining}/{rateLimit.total}
        </div>
      )}
    </div>
  );
}
```

---

## Related Documentation

- [Dashboard User Guide](../archive/dashboard-user-guide.md) - Archived (web dashboard deprecated in v1.6)
- [API Contracts](./api-contracts.md) - REST API documentation
- [Architecture](../architecture.md) - System architecture overview

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| v1.0 | 2026-04-09 | Initial WebSocket API documentation |
