import React from "react";
import { fireEvent, render, screen, act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import TriageView from "./TriageView";
import type { AnalysisResponse, PR } from "../types/api";

vi.mock("next/router", () => ({
  useRouter: () => ({ pathname: "/inbox" }),
}));

vi.mock("next/link", () => ({
  default: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

function createMockPR(number: number): PR {
  return {
    id: `pr-${number}`,
    repo: "test/repo",
    number,
    title: `PR ${number}`,
    body: "",
    url: `https://github.com/test/repo/pull/${number}`,
    author: "testuser",
    labels: [],
    files_changed: [],
    review_status: "pending",
    ci_status: "passing",
    mergeable: "true",
    base_branch: "main",
    head_branch: "feature/test",
    cluster_id: "cluster-1",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-02T00:00:00Z",
    is_draft: false,
    is_bot: false,
    additions: 10,
    deletions: 5,
    changed_files_count: 1,
  };
}

function createMockAnalysis(prs: PR[]): AnalysisResponse {
  return {
    repo: "test/repo",
    generatedAt: new Date().toISOString(),
    counts: {
      total_prs: prs.length,
      cluster_count: 1,
      duplicate_groups: 0,
      overlap_groups: 0,
      conflict_pairs: 0,
      stale_prs: 0,
    },
    prs,
    clusters: [],
    duplicates: [],
    overlaps: [],
    conflicts: [],
    stalenessSignals: [],
    review_payload: {
      total_prs: prs.length,
      reviewed_prs: prs.length,
      categories: [],
      buckets: [
        { bucket: "now", count: 2 },
        { bucket: "future", count: 1 },
        { bucket: "duplicate", count: 0 },
        { bucket: "junk", count: 0 },
        { bucket: "blocked", count: 0 },
      ],
      priority_tiers: [
        { tier: "fast_merge", count: 2 },
        { tier: "review_required", count: 1 },
        { tier: "blocked", count: 0 },
      ],
      results: [],
    },
  };
}

describe("TriageView", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders rows from analysis", () => {
    const prs = [createMockPR(1), createMockPR(2)];
    render(<TriageView analysis={createMockAnalysis(prs)} />);

    expect(screen.getByText("2 PRs ready for triage.")).toBeTruthy();
    expect(screen.getAllByText(/PR 1/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/PR 2/).length).toBeGreaterThan(0);
  });

  it("approve/close/skip action buttons exist for first row", () => {
    const prs = [createMockPR(42)];
    render(<TriageView analysis={createMockAnalysis(prs)} />);

    expect(screen.getByRole("button", { name: "Approve PR #42" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Close PR #42" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Skip PR #42" })).toBeTruthy();
  });

  it("clicking approve shows deterministic feedback and intent log text", async () => {
    vi.useFakeTimers();
    const prs = [createMockPR(100)];
    render(<TriageView analysis={createMockAnalysis(prs)} />);

    const approveBtn = screen.getByRole("button", { name: "Approve PR #100" });
    fireEvent.click(approveBtn);

    await act(async () => {
      vi.advanceTimersByTime(60);
    });

    expect(screen.getByRole("alert")).toBeTruthy();
    expect(screen.getByText(/Intent logged: would approve PR #100/)).toBeTruthy();
    expect(screen.getByText("Intent Log")).toBeTruthy();
    expect(screen.getByText("approve")).toBeTruthy();
  });

  it("renders review bucket labels using v1.4 vocabulary (now, future, duplicate, junk, blocked)", () => {
    const prs = [createMockPR(1), createMockPR(2), createMockPR(3)];
    render(<TriageView analysis={createMockAnalysis(prs)} />);

    // Verify new bucket labels appear in the review summary section
    // Use getAllByText since these words may appear in multiple contexts
    expect(screen.getAllByText("now").length).toBeGreaterThan(0);
    expect(screen.getAllByText("future").length).toBeGreaterThan(0);
    expect(screen.getAllByText("duplicate").length).toBeGreaterThan(0);
    expect(screen.getAllByText("junk").length).toBeGreaterThan(0);
    expect(screen.getAllByText("blocked").length).toBeGreaterThan(0);

    // Verify bucket counts appear next to their labels in the stats-grid
    // The stat-card structure is: <article><span>label</span><strong>count</strong></article>
    const nowCard = screen.getAllByText("now")[0].closest("article");
    expect(nowCard?.textContent).toContain("2");
    const futureCard = screen.getAllByText("future")[0].closest("article");
    expect(futureCard?.textContent).toContain("1");
    const duplicateCard = screen.getAllByText("duplicate")[0].closest("article");
    expect(duplicateCard?.textContent).toContain("0");
    const junkCard = screen.getAllByText("junk")[0].closest("article");
    expect(junkCard?.textContent).toContain("0");
    const blockedCard = screen.getAllByText("blocked")[0].closest("article");
    expect(blockedCard?.textContent).toContain("0");
  });
});
