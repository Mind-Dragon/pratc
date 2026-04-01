/**
 * Structured logging for prATC web dashboard.
 * 
 * Log format follows internal/logger/SPEC.md section 2:
 * {"ts":"...","level":"INFO|ERROR","component":"web","request_id":"uuid","msg":"...","path":"...","duration_ms":...}
 */

export interface LogEntry {
  ts: string;
  level: "INFO" | "ERROR";
  component: "web";
  request_id: string;
  msg: string;
  path?: string;
  duration_ms?: number;
  method?: string;
  status?: number;
  error?: string;
}

/**
 * Generate RFC 3339 timestamp with millisecond precision.
 * Example: 2026-04-02T14:23:01.234Z
 */
function timestamp(): string {
  return new Date().toISOString();
}

/**
 * Log an API request (outgoing fetch call).
 * Level: INFO
 */
export function logAPIRequest(path: string, options?: RequestInit): string {
  const requestId = crypto.randomUUID();
  const entry: LogEntry = {
    ts: timestamp(),
    level: "INFO",
    component: "web",
    request_id: requestId,
    msg: "request sent",
    path,
    method: options?.method || "GET",
  };

  console.log(JSON.stringify(entry));
  return requestId;
}

/**
 * Log an API response.
 * Level: INFO
 */
export function logAPIResponse(
  path: string,
  response: Response,
  durationMs: number,
  requestId: string
): void {
  const entry: LogEntry = {
    ts: timestamp(),
    level: "INFO",
    component: "web",
    request_id: requestId,
    msg: "response received",
    path,
    duration_ms: durationMs,
    status: response.status,
  };

  console.log(JSON.stringify(entry));
}

/**
 * Log an API error.
 * Level: ERROR
 */
export function logAPIError(
  path: string,
  error: unknown,
  durationMs: number,
  requestId: string
): void {
  const errorMessage = error instanceof Error ? error.message : String(error);
  const entry: LogEntry = {
    ts: timestamp(),
    level: "ERROR",
    component: "web",
    request_id: requestId,
    msg: "request failed",
    path,
    duration_ms: durationMs,
    error: errorMessage,
  };

  console.error(JSON.stringify(entry));
}

/**
 * Options for the logging fetch wrapper.
 */
export interface LoggingFetchOptions {
  /** Log level for requests. Default: "info" */
  logLevel?: "info" | "silent";
}

/**
 * Create a fetch wrapper that logs all requests/responses/errors.
 * 
 * Usage:
 * ```typescript
 * const loggedFetch = createLoggingFetch();
 * const response = await loggedFetch('/api/repos/owner/repo/analyze');
 * ```
 */
export function createLoggingFetch(options?: LoggingFetchOptions): typeof fetch {
  const logLevel = options?.logLevel || "info";

  return async function loggedFetch(
    input: RequestInfo | URL,
    init?: RequestInit
  ): Promise<Response> {
    const startTime = performance.now();
    const path = typeof input === "string" ? input : input instanceof URL ? input.pathname : input.url;
    
    let requestId: string;
    
    // Log the request if not silent
    if (logLevel === "info") {
      requestId = logAPIRequest(path, init);
    } else {
      requestId = crypto.randomUUID();
    }

    try {
      const response = await fetch(input, init);
      const durationMs = Math.round(performance.now() - startTime);

      // Log response
      if (logLevel === "info") {
        logAPIResponse(path, response, durationMs, requestId);
      }

      return response;
    } catch (error) {
      const durationMs = Math.round(performance.now() - startTime);

      // Log error
      if (logLevel === "info") {
        logAPIError(path, error, durationMs, requestId);
      }

      throw error;
    }
  };
}
