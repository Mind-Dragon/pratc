import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  postSettingMock.mockReset();
  postSettingMock.mockResolvedValue({ updated: true });
});

afterEach(() => {
  cleanup();
});

vi.mock("next/router", () => ({
  useRouter: () => ({
    pathname: "/settings"
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
  fetchSettings: vi.fn().mockResolvedValue({ duplicate_threshold: 0.85 }),
  postSetting: vi.fn().mockResolvedValue({ updated: true }),
  deleteSetting: vi.fn().mockResolvedValue({ deleted: true }),
  exportSettingsYAML: vi.fn().mockResolvedValue("key: value\n"),
  importSettingsYAML: vi.fn().mockResolvedValue({ imported: true }),
}));

import SettingsPage from "../../pages/settings";
import { postSetting } from "../../lib/api";

const postSettingMock = postSetting as unknown as any;

describe("SettingsPage", () => {
  it("renders all 9 sections", () => {
    render(<SettingsPage initialSettings={{ duplicate_threshold: 0.85 }} />);

    expect(screen.getByRole("heading", { name: "Formula Weights" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Thresholds" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Clustering" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Sync" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "PR Window" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Priority Weights" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Pool Time Decay" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Priority Pool" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Legacy Time Decay" })).toBeTruthy();
  });

  it("renders Export YAML and Import YAML buttons", () => {
    render(<SettingsPage initialSettings={null} />);

    expect(screen.getAllByRole("button", { name: "Export YAML" }).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByRole("button", { name: "Import YAML" }).length).toBeGreaterThanOrEqual(1);
  });

  it("updates slider state and validates before save", async () => {
    postSettingMock.mockResolvedValue({ updated: true });

    render(<SettingsPage initialSettings={{ duplicate_threshold: 0.85 }} />);

    const slider = screen.getAllByLabelText("Duplicate threshold")[0] as HTMLInputElement;
    fireEvent.change(slider, { target: { value: "0.92" } });

    expect((screen.getByLabelText("Duplicate threshold value") as HTMLInputElement).value).toBe("0.92");

    const saveThresholdButtons = screen.getAllByRole("button", { name: "Save" });
    fireEvent.click(saveThresholdButtons[1]);

    await waitFor(() => {
      expect(postSettingMock).toHaveBeenCalledWith({
        scope: "global",
        repo: "opencode-ai/opencode",
        key: "duplicate_threshold",
        value: 0.92,
      }, { validateOnly: true });
    });
  });

  it("shows inline error when validateOnly fails", async () => {
    postSettingMock.mockImplementation(async (_payload, options) => {
      if (options?.validateOnly) {
        return null;
      }
      return { updated: true };
    });

    render(<SettingsPage initialSettings={{ duplicate_threshold: 1.5 }} />);

    const saveThresholdButtons = screen.getAllByRole("button", { name: "Save" });
    fireEvent.click(saveThresholdButtons[1]);

    expect((await screen.findAllByText("Validation failed")).length).toBeGreaterThan(0);
  });

  it("renders Priority Pool section with max_pool_size and pool_budget inputs", () => {
    render(<SettingsPage initialSettings={{ max_pool_size: 64, pool_budget: 64 }} />);

    const maxPoolSizeInputs = screen.getAllByLabelText("Max pool size");
    const poolBudgetInputs = screen.getAllByLabelText("Pool budget");

    expect(maxPoolSizeInputs.length).toBeGreaterThan(0);
    expect(poolBudgetInputs.length).toBeGreaterThan(0);
  });

  it("renders Time Decay section with decay_half_life_days and anti_starvation_threshold inputs", () => {
    render(<SettingsPage initialSettings={{ decay_half_life_days: 7, anti_starvation_threshold: 0.05 }} />);

    const decayInputs = screen.getAllByLabelText("Decay half-life (days)");
    const thresholdInputs = screen.getAllByLabelText("Anti-starvation threshold");

    expect(decayInputs.length).toBeGreaterThan(0);
    expect(thresholdInputs.length).toBeGreaterThan(0);
  });

  it("renders Priority Weights section with all 5 weight inputs", () => {
    render(<SettingsPage initialSettings={{ staleness_weight: 0.3, ci_status_weight: 0.25, security_label_weight: 0.2, cluster_coherence_weight: 0.15, time_decay_weight: 0.1 }} />);

    const stalenessInputs = screen.getAllByLabelText("Staleness weight");
    const ciStatusInputs = screen.getAllByLabelText("CI status weight");
    const securityInputs = screen.getAllByLabelText("Security label weight");
    const clusterInputs = screen.getAllByLabelText("Cluster coherence weight");
    const timeDecayInputs = screen.getAllByLabelText("Time decay weight");

    expect(stalenessInputs.length).toBeGreaterThan(0);
    expect(ciStatusInputs.length).toBeGreaterThan(0);
    expect(securityInputs.length).toBeGreaterThan(0);
    expect(clusterInputs.length).toBeGreaterThan(0);
    expect(timeDecayInputs.length).toBeGreaterThan(0);
  });

  it("renders Pool Time Decay section with half_life_hours, window_hours, and protected_hours inputs", () => {
    render(<SettingsPage initialSettings={{ half_life_hours: 72, window_hours: 168, protected_hours: 336 }} />);

    const halfLifeInputs = screen.getAllByLabelText("Half-life (hours)");
    const windowInputs = screen.getAllByLabelText("Recency window (hours)");
    const protectedInputs = screen.getAllByLabelText("Protected lane threshold (hours)");

    expect(halfLifeInputs.length).toBeGreaterThan(0);
    expect(windowInputs.length).toBeGreaterThan(0);
    expect(protectedInputs.length).toBeGreaterThan(0);
  });

  it("saves Priority Weights section settings", async () => {
    postSettingMock.mockResolvedValue({ updated: true });

    render(<SettingsPage initialSettings={{ staleness_weight: 0.3 }} />);

    const slider = screen.getAllByLabelText("Staleness weight")[0] as HTMLInputElement;
    fireEvent.change(slider, { target: { value: "0.35" } });

    const saveButtons = screen.getAllByRole("button", { name: "Save" });
    const priorityWeightsSaveIndex = 5;
    fireEvent.click(saveButtons[priorityWeightsSaveIndex]);

    await waitFor(() => {
      expect(postSettingMock).toHaveBeenCalledWith(
        expect.objectContaining({
          key: "staleness_weight",
          value: 0.35,
          scope: "global",
        }),
        { validateOnly: true }
      );
    });
  });
});
