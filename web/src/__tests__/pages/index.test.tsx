import React from "react";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

class MockEventSource {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;
  url = "";
  readyState = 0;
  addEventListener = vi.fn();
  removeEventListener = vi.fn();
  close = vi.fn();
  constructor(url: string) { this.url = url; }
}

beforeEach(() => {
  globalThis.EventSource = MockEventSource as unknown as typeof EventSource;
  globalThis.fetch = vi.fn();
});

afterEach(() => {
  cleanup();
});

vi.mock("next/router", () => ({
  useRouter: () => ({
    pathname: "/"
  })
}));

vi.mock("next/link", () => ({
  default: ({ href, className, children }: { href: string; className?: string; children: React.ReactNode }) => (
    <a className={className} href={href}>
      {children}
    </a>
  )
}));

vi.mock("../../lib/api", () => ({
  fetchAnalysis: vi.fn(),
}));

import DashboardPage from "../../pages/index";
import type { AnalysisResponse } from "../../types/api";

interface SyncInProgressPayload {
  repo: string;
  generatedAt: string;
  sync_status: string;
  message: string;
}

describe("DashboardPage", () => {
  it("renders disconnected state when analysis is null", () => {
    render(<DashboardPage analysis={null} />);

    expect(screen.getByText("Disconnected")).toBeTruthy();
    expect(screen.getByText("No analysis payload available.")).toBeTruthy();
  });

  it("renders sync-in-progress state without crashing", () => {
    const syncInProgressPayload: SyncInProgressPayload = {
      repo: "opencode-ai/opencode",
      generatedAt: "2026-03-23T10:30:00Z",
      sync_status: "in_progress",
      message: "sync in progress",
    };

    render(<DashboardPage analysis={syncInProgressPayload as unknown as AnalysisResponse | null} />);

    expect(screen.getByText("opencode-ai/opencode")).toBeTruthy();
    expect(screen.getByText("Sync in progress. Analysis data will appear once the sync completes.")).toBeTruthy();
    expect(screen.getByText("Syncing")).toBeTruthy();
  });

  it("renders SyncStatusPanel in sync-in-progress state", () => {
    const syncInProgressPayload: SyncInProgressPayload = {
      repo: "opencode-ai/opencode",
      generatedAt: "2026-03-23T10:30:00Z",
      sync_status: "in_progress",
      message: "sync in progress",
    };

    render(<DashboardPage analysis={syncInProgressPayload as unknown as AnalysisResponse | null} />);

    expect(screen.getAllByRole("button", { name: "Sync Now" }).length).toBeGreaterThanOrEqual(1);
  });

  it("renders SyncStatusPanel in disconnected state", () => {
    render(<DashboardPage analysis={null} />);

    expect(screen.getAllByRole("button", { name: "Sync Now" }).length).toBeGreaterThanOrEqual(1);
  });

  it("renders full dashboard when analysis has counts", () => {
    const fullAnalysis: AnalysisResponse = {
      repo: "opencode-ai/opencode",
      generatedAt: "2026-03-23T10:30:00Z",
      counts: {
        total_prs: 100,
        cluster_count: 5,
        duplicate_groups: 2,
        overlap_groups: 1,
        conflict_pairs: 3,
        stale_prs: 10,
        garbage_prs: 5,
      },
      prs: [],
      clusters: [],
      duplicates: [],
      overlaps: [],
      conflicts: [],
      stalenessSignals: [],
    };

    render(<DashboardPage analysis={fullAnalysis} />);

    expect(screen.getByText("100")).toBeTruthy();
    expect(screen.getByText("Open PRs")).toBeTruthy();
    expect(screen.getByText("5")).toBeTruthy();
    expect(screen.getByText("Clusters")).toBeTruthy();
  });

  it("does not crash when sync-in-progress payload has no counts property", () => {
    const syncInProgressPayload: SyncInProgressPayload = {
      repo: "test/repo",
      generatedAt: "2026-03-23T10:30:00Z",
      sync_status: "in_progress",
      message: "sync in progress",
    };

    expect(() => {
      render(<DashboardPage analysis={syncInProgressPayload as unknown as AnalysisResponse | null} />);
    }).not.toThrow();

    expect(screen.getByText("test/repo")).toBeTruthy();
  });
});
