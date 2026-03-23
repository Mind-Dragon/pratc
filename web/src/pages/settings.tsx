import React, { useCallback, useEffect, useState } from "react";
import type { GetServerSideProps } from "next";

import Layout from "../components/Layout";
import { deleteSetting, exportSettingsYAML, fetchSettings, importSettingsYAML, postSetting } from "../lib/api";
import type { SettingsMap } from "../lib/api";

const DEFAULT_REPO = "opencode-ai/opencode";

interface SettingDef {
  key: string;
  label: string;
  type: "number" | "text";
  min?: number;
  max?: number;
  step?: number;
  default: unknown;
  scope: "global" | "repo";
}

const SECTIONS: { title: string; kicker: string; settings: SettingDef[] }[] = [
  {
    title: "Formula Weights",
    kicker: "Scoring",
    settings: [
      { key: "weight_age", label: "Age weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.2, scope: "global" },
      { key: "weight_size", label: "Size weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.15, scope: "global" },
      { key: "weight_ci", label: "CI weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.25, scope: "global" },
      { key: "weight_review", label: "Review weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.2, scope: "global" },
      { key: "weight_conflict", label: "Conflict weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.1, scope: "global" },
      { key: "weight_cluster", label: "Cluster weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.1, scope: "global" },
    ],
  },
  {
    title: "Thresholds",
    kicker: "Detection",
    settings: [
      { key: "duplicate_threshold", label: "Duplicate threshold", type: "number", min: 0, max: 1, step: 0.01, default: 0.9, scope: "global" },
      { key: "overlap_threshold", label: "Overlap threshold", type: "number", min: 0, max: 1, step: 0.01, default: 0.7, scope: "global" },
      { key: "staleness_days", label: "Staleness (days)", type: "number", min: 1, max: 365, step: 1, default: 90, scope: "global" },
    ],
  },
  {
    title: "Clustering",
    kicker: "Model",
    settings: [
      { key: "min_cluster_size", label: "Min cluster size", type: "number", min: 1, max: 50, step: 1, default: 3, scope: "global" },
      { key: "cluster_model", label: "Cluster model", type: "text", default: "kmeans", scope: "global" },
    ],
  },
  {
    title: "Sync",
    kicker: "Mirror",
    settings: [
      { key: "mirror_path", label: "Mirror path", type: "text", default: "~/.pratc/repos", scope: "global" },
      { key: "fetch_interval", label: "Fetch interval (min)", type: "number", min: 1, max: 1440, step: 1, default: 5, scope: "global" },
    ],
  },
  {
    title: "PR Window",
    kicker: "Intake",
    settings: [
      { key: "beginning_pr_number", label: "Beginning PR #", type: "number", min: 0, step: 1, default: 0, scope: "repo" },
      { key: "ending_pr_number", label: "Ending PR #", type: "number", min: 0, step: 1, default: 0, scope: "repo" },
    ],
  },
  {
    title: "Priority Weights",
    kicker: "Scoring",
    settings: [
      { key: "staleness_weight", label: "Staleness weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.30, scope: "global" },
      { key: "ci_status_weight", label: "CI status weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.25, scope: "global" },
      { key: "security_label_weight", label: "Security label weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.20, scope: "global" },
      { key: "cluster_coherence_weight", label: "Cluster coherence weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.15, scope: "global" },
      { key: "time_decay_weight", label: "Time decay weight", type: "number", min: 0, max: 1, step: 0.01, default: 0.10, scope: "global" },
    ],
  },
  {
    title: "Pool Time Decay",
    kicker: "Windowing",
    settings: [
      { key: "half_life_hours", label: "Half-life (hours)", type: "number", min: 1, max: 720, step: 1, default: 72, scope: "global" },
      { key: "window_hours", label: "Recency window (hours)", type: "number", min: 1, max: 2160, step: 1, default: 168, scope: "global" },
      { key: "protected_hours", label: "Protected lane threshold (hours)", type: "number", min: 1, max: 4320, step: 1, default: 336, scope: "global" },
    ],
  },
  {
    title: "Priority Pool",
    kicker: "Candidate Selection",
    settings: [
      { key: "max_pool_size", label: "Max pool size", type: "number", min: 8, max: 256, step: 1, default: 64, scope: "global" },
      { key: "pool_budget", label: "Pool budget", type: "number", min: 1, max: 256, step: 1, default: 64, scope: "global" },
    ],
  },
  {
    title: "Legacy Time Decay",
    kicker: "Recency",
    settings: [
      { key: "decay_half_life_days", label: "Decay half-life (days)", type: "number", min: 1, max: 90, step: 1, default: 7, scope: "global" },
      { key: "anti_starvation_threshold", label: "Anti-starvation threshold", type: "number", min: 0, max: 1, step: 0.01, default: 0.05, scope: "global" },
    ],
  },
];

type SettingsProps = {
  initialSettings: SettingsMap | null;
};

export const getServerSideProps: GetServerSideProps<SettingsProps> = async () => {
  const settings = await fetchSettings(DEFAULT_REPO);
  return { props: { initialSettings: settings ?? null } };
};

export default function SettingsPage({ initialSettings }: SettingsProps) {
  const [settings, setSettings] = useState<SettingsMap>(initialSettings ?? {});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState<string | null>(null);
  const [lastSaved, setLastSaved] = useState<string | null>(null);

  const getValue = useCallback(
    (key: string, fallback: unknown) => {
      const v = settings[key];
      return v !== undefined ? v : fallback;
    },
    [settings],
  );

  const handleChange = useCallback((key: string, raw: string, def: SettingDef) => {
    let parsed: unknown = raw;
    if (def.type === "number") {
      const n = Number(raw);
      parsed = isNaN(n) ? def.default : n;
    }
    setSettings((prev) => ({ ...prev, [key]: parsed }));
    setErrors((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });
  }, []);

  const saveSection = useCallback(
    async (sectionSettings: SettingDef[]) => {
      setSaving(SECTIONS.find((s) => s.settings === sectionSettings)?.title ?? "Saving");
      const newErrors: Record<string, string> = {};

      for (const def of sectionSettings) {
        const value = getValue(def.key, def.default);
        const validation = await postSetting({ scope: def.scope, repo: DEFAULT_REPO, key: def.key, value }, { validateOnly: true });
        if (!validation?.valid) {
          newErrors[def.key] = "Validation failed";
        }
      }

      if (Object.keys(newErrors).length === 0) {
        for (const def of sectionSettings) {
          const value = getValue(def.key, def.default);
          const result = await postSetting({ scope: def.scope, repo: DEFAULT_REPO, key: def.key, value });
          if (!result?.updated) {
            newErrors[def.key] = "Failed to save";
          }
        }
      }

      setErrors(newErrors);
      if (Object.keys(newErrors).length === 0) {
        setLastSaved(new Date().toLocaleTimeString());
      }
      setSaving(null);
    },
    [getValue],
  );

  const resetSection = useCallback(
    async (sectionSettings: SettingDef[]) => {
      for (const def of sectionSettings) {
        await deleteSetting(def.scope, DEFAULT_REPO, def.key);
      }
      setSettings((prev) => {
        const next = { ...prev };
        for (const def of sectionSettings) {
          delete next[def.key];
        }
        return next;
      });
      setLastSaved(new Date().toLocaleTimeString());
    },
    [],
  );

  const handleExport = useCallback(async () => {
    const yaml = await exportSettingsYAML("global", DEFAULT_REPO);
    if (!yaml) return;
    const blob = new Blob([yaml], { type: "text/yaml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `pratc-settings-${DEFAULT_REPO.replace("/", "-")}.yaml`;
    a.click();
    URL.revokeObjectURL(url);
  }, []);

  const handleImport = useCallback(async () => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".yaml,.yml";
    input.onchange = async (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (!file) return;
      const content = await file.text();
      const result = await importSettingsYAML("global", DEFAULT_REPO, content);
      if (result?.imported) {
        const fresh = await fetchSettings(DEFAULT_REPO);
        if (fresh) setSettings(fresh);
        setLastSaved(new Date().toLocaleTimeString());
      }
    };
    input.click();
  }, []);

  return (
    <Layout title="Settings" eyebrow="Configuration" description="Configure formula weights, thresholds, sync behavior, and PR intake window.">
      <div className="settings-actions" style={{ display: "flex", gap: 8, marginBottom: 24 }}>
        <button type="button" className="action-row" style={{ border: "1px solid var(--line)", borderRadius: 999, padding: "8px 14px", background: "rgba(255,255,255,0.8)" }} onClick={handleExport}>
          Export YAML
        </button>
        <button type="button" className="action-row" style={{ border: "1px solid var(--line)", borderRadius: 999, padding: "8px 14px", background: "rgba(255,255,255,0.8)" }} onClick={handleImport}>
          Import YAML
        </button>
        {lastSaved && (
          <span style={{ marginLeft: "auto", fontSize: "0.85rem", color: "rgba(23,34,53,0.6)" }}>
            Saved at {lastSaved}
          </span>
        )}
      </div>

      {SECTIONS.map((section) => {
        const sectionSaving = saving === section.title;

        return (
          <section key={section.title} className="settings-section" style={{ marginBottom: 28 }}>
            <div className="section-heading">
              <div>
                <p className="hero-kicker">{section.kicker}</p>
                <h3>{section.title}</h3>
              </div>
              <div className="action-row">
                <button
                  type="button"
                  style={{ border: "1px solid var(--line)", borderRadius: 999, padding: "8px 14px", background: "rgba(255,255,255,0.8)", opacity: sectionSaving ? 0.5 : 1 }}
                  disabled={sectionSaving}
                  onClick={() => resetSection(section.settings)}
                >
                  Reset
                </button>
                <button
                  type="button"
                  style={{ border: "1px solid var(--line)", borderRadius: 999, padding: "8px 14px", background: "linear-gradient(135deg, rgba(185,215,242,0.45), rgba(231,211,171,0.5))" }}
                  disabled={sectionSaving}
                  onClick={() => saveSection(section.settings)}
                >
                  {sectionSaving ? "Saving..." : "Save"}
                </button>
              </div>
            </div>

            <div className="settings-grid" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 16 }}>
              {section.settings.map((def) => {
                const value = getValue(def.key, def.default);
                const error = errors[def.key];

                return (
                  <div key={def.key} className="hero-panel" style={{ flexDirection: "column", gap: 8, padding: 18 }}>
                    <label htmlFor={`${def.key}-slider`} style={{ fontSize: "0.85rem", color: "rgba(23,34,53,0.65)" }}>
                      {def.label}
                    </label>
                    {def.type === "number" ? (
                      <>
                        <input
                          id={`${def.key}-slider`}
                          aria-label={def.label}
                          type="range"
                          min={def.min}
                          max={def.max}
                          step={def.step}
                          value={value as number}
                          onChange={(e) => handleChange(def.key, e.target.value, def)}
                          style={{ width: "100%" }}
                        />
                        <input
                          id={def.key}
                          aria-label={`${def.label} value`}
                          type="number"
                          min={def.min}
                          max={def.max}
                          step={def.step}
                          value={value as number}
                          onChange={(e) => handleChange(def.key, e.target.value, def)}
                          style={{ width: "100%", padding: "8px 10px", border: "1px solid var(--line)", borderRadius: 12, background: "rgba(255,253,247,0.86)", fontSize: "1rem" }}
                        />
                      </>
                    ) : (
                      <input
                        id={def.key}
                        type="text"
                        value={value as string}
                        onChange={(e) => handleChange(def.key, e.target.value, def)}
                        style={{ width: "100%", padding: "8px 10px", border: "1px solid var(--line)", borderRadius: 12, background: "rgba(255,253,247,0.86)", fontSize: "1rem" }}
                      />
                    )}
                    {error && (
                      <span style={{ fontSize: "0.82rem", color: "var(--danger)" }}>{error}</span>
                    )}
                  </div>
                );
              })}
            </div>
          </section>
        );
      })}
    </Layout>
  );
}
