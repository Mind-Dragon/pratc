import type { AnalysisResponse, ClusterResponse, GraphResponse, PlanResponse } from "../types/api";

const DEFAULT_REPO = "opencode-ai/opencode";

export interface SettingsMap {
  [key: string]: unknown;
}

export interface PostSettingPayload {
  scope: "global" | "repo";
  repo: string;
  key: string;
  value: unknown;
}

export interface PostSettingOptions {
  validateOnly?: boolean;
}

export interface PostSettingResponse {
  updated?: boolean;
  valid?: boolean;
}

export interface DeleteSettingResponse {
  deleted: boolean;
}

export interface ImportSettingsResponse {
  imported: boolean;
}

function apiBaseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_PRATC_API_URL;
  if (configured && configured.trim().length > 0) {
    return configured.replace(/\/$/, "");
  }
  return "http://localhost:8080";
}

function repoPath(repo: string): string {
  const [owner, name] = repo.split("/");
  if (owner && name) {
    return `/api/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}`;
  }
  return `/api/repos/${encodeURIComponent(repo)}`;
}

async function fetchJSON<T>(path: string, fallback: T): Promise<T> {
  try {
    const response = await fetch(`${apiBaseUrl()}${path}`);
    if (!response.ok) {
      return fallback;
    }
    return (await response.json()) as T;
  } catch {
    return fallback;
  }
}

export async function fetchAnalysis(repo: string = DEFAULT_REPO): Promise<AnalysisResponse | null> {
  return fetchJSON<AnalysisResponse | null>(`${repoPath(repo)}/analyze`, null);
}

export async function fetchCluster(repo: string = DEFAULT_REPO): Promise<ClusterResponse | null> {
  return fetchJSON<ClusterResponse | null>(`${repoPath(repo)}/cluster`, null);
}

export async function fetchGraph(repo: string = DEFAULT_REPO): Promise<GraphResponse | null> {
  return fetchJSON<GraphResponse | null>(`${repoPath(repo)}/graph`, null);
}

export async function fetchPlan(repo: string = DEFAULT_REPO, target = 20): Promise<PlanResponse | null> {
  return fetchJSON<PlanResponse | null>(`${repoPath(repo)}/plan?target=${target}`, null);
}

export async function fetchSettings(repo: string = DEFAULT_REPO): Promise<SettingsMap | null> {
  return fetchJSON<SettingsMap | null>(`${repoPath(repo)}/settings`, null);
}

export async function postSetting(
  payload: PostSettingPayload,
  options?: PostSettingOptions
): Promise<PostSettingResponse | null> {
  const basePath = repoPath(payload.repo);
  const path = options?.validateOnly
    ? `${basePath}/settings/${payload.scope}/${encodeURIComponent(payload.key)}/validate`
    : `${basePath}/settings/${payload.scope}/${encodeURIComponent(payload.key)}`;

  try {
    const response = await fetch(`${apiBaseUrl()}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ value: payload.value }),
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as PostSettingResponse;
  } catch {
    return null;
  }
}

export async function deleteSetting(
  scope: "global" | "repo",
  repo: string,
  key: string
): Promise<DeleteSettingResponse | null> {
  const path = `${repoPath(repo)}/settings/${scope}/${encodeURIComponent(key)}`;

  try {
    const response = await fetch(`${apiBaseUrl()}${path}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as DeleteSettingResponse;
  } catch {
    return null;
  }
}

export async function exportSettingsYAML(
  scope: "global" | "repo",
  repo: string
): Promise<string | null> {
  const path = `${repoPath(repo)}/settings/${scope}/export`;

  try {
    const response = await fetch(`${apiBaseUrl()}${path}`);
    if (!response.ok) {
      return null;
    }
    return await response.text();
  } catch {
    return null;
  }
}

export async function importSettingsYAML(
  scope: "global" | "repo",
  repo: string,
  content: string
): Promise<ImportSettingsResponse | null> {
  const path = `${repoPath(repo)}/settings/${scope}/import`;

  try {
    const response = await fetch(`${apiBaseUrl()}${path}`, {
      method: "POST",
      headers: { "Content-Type": "text/yaml" },
      body: content,
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as ImportSettingsResponse;
  } catch {
    return null;
  }
}
