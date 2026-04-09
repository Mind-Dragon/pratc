import React, { useEffect, useMemo, useState } from "react";
import { useMonitorData } from "../../../hooks/useMonitorData";
import type { RateLimitView } from "../../../types/monitor";

interface CountdownTime {
  minutes: number;
  seconds: number;
}

function getCountdownUntilReset(resetTime: string): CountdownTime {
  const reset = new Date(resetTime).getTime();
  const now = Date.now();
  const diff = Math.max(0, reset - now);
  
  const minutes = Math.floor(diff / 60000);
  const seconds = Math.floor((diff % 60000) / 1000);
  
  return { minutes, seconds };
}

function getThermometerColor(remaining: number): string {
  if (remaining > 2000) {
    return "var(--success)";
  }
  if (remaining >= 500) {
    return "#f59e0b";
  }
  return "var(--danger)";
}

function getThermometerGradient(remaining: number, total: number): string {
  const color = getThermometerColor(remaining);
  const fillPercent = total > 0 ? (remaining / total) * 100 : 0;
  
  return `linear-gradient(180deg, 
    ${color} 0%, 
    ${color} ${100 - fillPercent}%, 
    rgba(23, 34, 53, 0.08) ${100 - fillPercent}%, 
    rgba(23, 34, 53, 0.08) 100%)`;
}

interface ThermometerGaugeProps {
  remaining: number;
  total: number;
}

function ThermometerGauge({ remaining, total }: ThermometerGaugeProps) {
  const fillPercent = total > 0 ? Math.min(100, Math.max(0, (remaining / total) * 100)) : 0;
  const color = getThermometerColor(remaining);
  
  return (
    <div
      className="rate-limit-thermometer"
      style={{
        width: 80,
        height: 220,
        position: "relative",
      }}
    >
      <div
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          bottom: 24,
          borderRadius: 40,
          border: `3px solid ${color}`,
          background: "rgba(23, 34, 53, 0.05)",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            position: "absolute",
            bottom: 0,
            left: 0,
            right: 0,
            height: `${fillPercent}%`,
            background: `linear-gradient(180deg, ${color} 0%, ${color} 100%)`,
            transition: "height 500ms ease, background-color 500ms ease",
          }}
        />
        
        <div
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            pointerEvents: "none",
          }}
        >
          {[0, 25, 50, 75, 100].map((mark) => (
            <div
              key={mark}
              style={{
                position: "absolute",
                left: 8,
                right: 8,
                bottom: `${mark}%`,
                height: 1,
                background: "rgba(23, 34, 53, 0.15)",
              }}
            />
          ))}
        </div>
      </div>
      
      <div
        style={{
          position: "absolute",
          bottom: -12,
          left: 50,
          transform: "translateX(-50%)",
          width: 56,
          height: 56,
          borderRadius: "50%",
          background: `radial-gradient(circle at 30% 30%, ${color}, ${color} 60%, rgba(23, 34, 53, 0.2) 100%)`,
          border: `3px solid ${color}`,
          boxShadow: "0 4px 12px rgba(23, 34, 53, 0.15)",
        }}
      />
    </div>
  );
}

interface DigitalReadoutProps {
  remaining: number;
  total: number;
}

function DigitalReadout({ remaining, total }: DigitalReadoutProps) {
  const formattedRemaining = remaining.toLocaleString();
  const formattedTotal = total.toLocaleString();
  
  return (
    <div
      className="rate-limit-readout"
      style={{
        display: "flex",
        flexDirection: "column",
        gap: 4,
      }}
    >
      <div
        style={{
          fontSize: "2.2rem",
          fontWeight: 700,
          fontFamily: "'Courier New', Courier, monospace",
          color: "var(--ink)",
          letterSpacing: "-0.02em",
        }}
      >
        {formattedRemaining}
      </div>
      <div
        style={{
          fontSize: "1.1rem",
          color: "rgba(23, 34, 53, 0.55)",
          fontWeight: 500,
        }}
      >
        / {formattedTotal}
      </div>
    </div>
  );
}

interface CountdownTimerProps {
  resetTime: string;
}

function CountdownTimer({ resetTime }: CountdownTimerProps) {
  const [countdown, setCountdown] = useState<CountdownTime>(() => getCountdownUntilReset(resetTime));
  
  useEffect(() => {
    const interval = setInterval(() => {
      setCountdown(getCountdownUntilReset(resetTime));
    }, 1000);
    
    return () => clearInterval(interval);
  }, [resetTime]);
  
  const { minutes, seconds } = countdown;
  const formattedMinutes = minutes.toString().padStart(2, "0");
  const formattedSeconds = seconds.toString().padStart(2, "0");
  
  return (
    <div
      className="rate-limit-countdown"
      style={{
        display: "flex",
        flexDirection: "column",
        gap: 6,
      }}
    >
      <div
        style={{
          fontSize: "0.72rem",
          textTransform: "uppercase",
          letterSpacing: "0.08em",
          color: "rgba(23, 34, 53, 0.55)",
        }}
      >
        Resets in
      </div>
      <div
        style={{
          fontSize: "1.5rem",
          fontWeight: 600,
          fontFamily: "'Courier New', Courier, monospace",
          color: "var(--ink)",
        }}
      >
        {formattedMinutes}:{formattedSeconds}
      </div>
    </div>
  );
}

export default function RateLimitPanel() {
  const { rateLimit, connected, error } = useMonitorData();
  
  const hasData = rateLimit !== null && rateLimit !== undefined;
  
  if (error) {
    return (
      <div
        className="rate-limit-panel hero-panel"
        style={{
          flexDirection: "column",
          gap: 12,
          padding: 18,
        }}
      >
        <div
          className="rate-limit-panel__header"
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
            Rate Limit
          </h3>
        </div>
        <p
          style={{
            margin: 0,
            fontSize: "0.85rem",
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
        className="rate-limit-panel hero-panel"
        style={{
          flexDirection: "column",
          gap: 12,
          padding: 18,
        }}
      >
        <div
          className="rate-limit-panel__header"
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
            Rate Limit
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
  
  if (!hasData) {
    return (
      <div
        className="rate-limit-panel hero-panel"
        style={{
          flexDirection: "column",
          gap: 12,
          padding: 18,
          minHeight: 280,
        }}
      >
        <div
          className="rate-limit-panel__header"
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
            Rate Limit
          </h3>
        </div>
        <div
          className="rate-limit-panel__empty"
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
            No rate limit data available
          </p>
        </div>
      </div>
    );
  }
  
  const { remaining, total, resetTime } = rateLimit;
  const color = getThermometerColor(remaining);
  
  return (
    <div
      className="rate-limit-panel hero-panel"
      style={{
        flexDirection: "column",
        gap: 16,
        padding: 18,
        minHeight: 280,
      }}
    >
      <div
        className="rate-limit-panel__header"
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
          Rate Limit
        </h3>
        <span
          className="pill"
          style={{
            background: `${color}26`,
            color: color,
            fontSize: "0.75rem",
            fontWeight: 600,
          }}
        >
          {remaining > 2000 ? "Healthy" : remaining >= 500 ? "Warning" : "Critical"}
        </span>
      </div>
      
      <div
        className="rate-limit-panel__content"
        style={{
          display: "flex",
          alignItems: "center",
          gap: 24,
          flex: 1,
          padding: "12px 0",
        }}
      >
        <ThermometerGauge remaining={remaining} total={total} />
        
        <DigitalReadout remaining={remaining} total={total} />
        
        <CountdownTimer resetTime={resetTime} />
      </div>
      
      <div
        className="rate-limit-panel__legend"
        style={{
          display: "flex",
          justifyContent: "center",
          gap: 16,
          paddingTop: 12,
          borderTop: "1px solid var(--line)",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 6,
            fontSize: "0.68rem",
            color: "rgba(23, 34, 53, 0.55)",
            textTransform: "uppercase",
            letterSpacing: "0.05em",
          }}
        >
          <div
            style={{
              width: 12,
              height: 12,
              borderRadius: 4,
              background: "var(--success)",
            }}
          />
          &gt;2,000
        </div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 6,
            fontSize: "0.68rem",
            color: "rgba(23, 34, 53, 0.55)",
            textTransform: "uppercase",
            letterSpacing: "0.05em",
          }}
        >
          <div
            style={{
              width: 12,
              height: 12,
              borderRadius: 4,
              background: "#f59e0b",
            }}
          />
          500-2,000
        </div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 6,
            fontSize: "0.68rem",
            color: "rgba(23, 34, 53, 0.55)",
            textTransform: "uppercase",
            letterSpacing: "0.05em",
          }}
        >
          <div
            style={{
              width: 12,
              height: 12,
              borderRadius: 4,
              background: "var(--danger)",
            }}
          />
          &lt;500
        </div>
      </div>
    </div>
  );
}
