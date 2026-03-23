import React from "react";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

class MockEventSource {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;
  url: string;
  readyState = 0;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  onopen: (() => void) | null = null;
  listeners: Record<string, Array<(evt: MessageEvent) => void>> = {};
  static instances: MockEventSource[] = [];

  addEventListener = vi.fn((name: string, callback: (evt: MessageEvent) => void) => {
    if (!this.listeners[name]) {
      this.listeners[name] = [];
    }
    this.listeners[name].push(callback);
  });
  removeEventListener = vi.fn();
  close = vi.fn();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  emit(name: string, data: unknown) {
    const callbacks = this.listeners[name] ?? [];
    const event = { data: JSON.stringify(data) } as MessageEvent;
    callbacks.forEach((cb) => cb(event));
  }
}

const MOCK_FETCH = vi.fn();

beforeEach(() => {
  globalThis.EventSource = MockEventSource as unknown as typeof EventSource;
  globalThis.fetch = MOCK_FETCH;
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

vi.mock("next/router", () => ({
  useRouter: () => ({
    pathname: "/"
  })
}));

import SyncStatusPanel from "./SyncStatusPanel";

describe("SyncStatusPanel", () => {
  beforeEach(() => {
    MOCK_FETCH.mockReset();
    MockEventSource.instances = [];
  });

  it("renders Sync Now button and Idle status by default", () => {
    render(<SyncStatusPanel />);

    expect(screen.getAllByRole("button", { name: "Sync Now" }).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Idle")).toBeTruthy();
  });

  it("triggers sync on button click", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    const buttons = screen.getAllByRole("button", { name: "Sync Now" });
    fireEvent.click(buttons[0]);

    expect(MOCK_FETCH).toHaveBeenCalledWith(
      expect.stringContaining("/api/repos/opencode-ai/opencode/sync"),
      { method: "POST" },
    );
  });

  it("shows error when sync request fails", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: false, status: 500 });

    render(<SyncStatusPanel />);

    const buttons = screen.getAllByRole("button", { name: "Sync Now" });
    fireEvent.click(buttons[0]);

    expect(await screen.findByText(/500/)).toBeTruthy();
  });

  it("keeps last sync time and synced PR count after completion", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 18, total: 20, eta_seconds: 30 });
    stream.emit("complete", { repo: "opencode-ai/opencode" });

    expect(await screen.findByText(/Last sync:/)).toBeTruthy();
    expect(screen.getByText(/PRs synced: 18/)).toBeTruthy();
  });

  it("fetches and displays cache size from stats endpoint", async () => {
    MOCK_FETCH.mockImplementation(async (url: string) => {
      if (url.includes("/stats")) {
        return {
          ok: true,
          json: async () => ({ cache_size: 5500 }),
        };
      }
      return { ok: true };
    });

    render(<SyncStatusPanel />);

    expect(await screen.findByText(/Cache size: 5,500 PRs/)).toBeTruthy();
  });

  it("shows dash when stats endpoint returns 404", async () => {
    MOCK_FETCH.mockImplementation(async (url: string) => {
      if (url.includes("/stats")) {
        return { ok: false, status: 404 };
      }
      return { ok: true };
    });

    render(<SyncStatusPanel />);

    expect(await screen.findByText(/Cache size: —/)).toBeTruthy();
  });

  it("displays sync rate after successful sync", async () => {
    MOCK_FETCH.mockImplementation(async (url: string) => {
      if (url.includes("/stats")) {
        return { ok: true, json: async () => ({ cache_size: 100 }) };
      }
      return { ok: true };
    });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 60, total: 100, eta_seconds: 0 });
    stream.emit("complete", { repo: "opencode-ai/opencode" });

    expect(await screen.findByText(/Sync rate:/)).toBeTruthy();
    expect(screen.getByText(/PRs\/min/)).toBeTruthy();
  });

  it("displays phase during sync when phase field is present", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 50, total: 100, eta_seconds: 30, phase: "metadata" });

    expect(await screen.findByText(/Phase:/)).toBeTruthy();
    expect(screen.getByText(/Syncing metadata/)).toBeTruthy();
  });

  it("displays mirroring phase correctly", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 50, total: 100, eta_seconds: 30, phase: "mirroring" });

    expect(await screen.findByText(/Fetching refs/)).toBeTruthy();
  });

  it("displays clustering phase correctly", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 50, total: 100, eta_seconds: 30, phase: "clustering" });

    expect(await screen.findByText(/Building clusters/)).toBeTruthy();
  });

  it("displays error count during sync", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 50, total: 100, eta_seconds: 30, errors: 3 });

    expect(await screen.findByText(/3 errors/)).toBeTruthy();
  });

  it("toggles scalability metrics visibility", async () => {
    MOCK_FETCH.mockImplementation(async (url: string) => {
      if (url.includes("/stats")) {
        return { ok: true, json: async () => ({ cache_size: 100, total_prs: 6000 }) };
      }
      return { ok: true };
    });

    render(<SyncStatusPanel />);

    const toggleButton = await screen.findByText(/Scalability Metrics/);
    fireEvent.click(toggleButton);

    expect(screen.getByText(/Total PRs/)).toBeTruthy();
    expect(screen.getByText(/Duration/)).toBeTruthy();
    expect(screen.getByText(/Throughput/)).toBeTruthy();
    expect(screen.getByText(/Errors/)).toBeTruthy();
  });

  it("displays total PRs from stats endpoint in scalability metrics", async () => {
    MOCK_FETCH.mockImplementation(async (url: string) => {
      if (url.includes("/stats")) {
        return { ok: true, json: async () => ({ cache_size: 100, total_prs: 6500 }) };
      }
      return { ok: true };
    });

    render(<SyncStatusPanel />);

    const toggleButton = await screen.findByText(/Scalability Metrics/);
    fireEvent.click(toggleButton);

    expect(screen.getByText(/6,500/)).toBeTruthy();
  });

  it("sets phase to done on complete event", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 100, total: 100, eta_seconds: 0, phase: "metadata" });
    stream.emit("complete", { repo: "opencode-ai/opencode" });

    expect(await screen.findByText("Up to date")).toBeTruthy();
  });

  it("displays error count in scalability metrics after sync", async () => {
    MOCK_FETCH.mockResolvedValue({ ok: true });

    render(<SyncStatusPanel />);

    fireEvent.click(screen.getAllByRole("button", { name: "Sync Now" })[0]);

    const stream = MockEventSource.instances.at(-1);
    if (!stream) {
      throw new Error("expected event source instance");
    }
    stream.emit("progress", { processed: 100, total: 100, eta_seconds: 0, errors: 5 });
    stream.emit("complete", { repo: "opencode-ai/opencode" });

    const toggleButton = await screen.findByText(/Scalability Metrics/);
    fireEvent.click(toggleButton);

    const errorLabels = screen.getAllByText("Errors");
    expect(errorLabels.length).toBeGreaterThan(0);
    expect(screen.getByText("5")).toBeTruthy();
  });
});
