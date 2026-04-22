import type {
  AnalysisResponse,
  ClusterResponse,
  GraphResponse,
  OmniPlanResponse,
  PlanResponse,
} from "../types/api";
import { createLoggingFetch } from "./logging";

const DEFAULT_REPO = "opencode-ai/opencode";

const loggedFetch = createLoggingFetch();

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
  return "http://localhost:7400";
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
    const response = await loggedFetch(`${apiBaseUrl()}${path}`);
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

export async function fetchOmniPlan(
  repo: string,
  selector: string,
  options?: { stageSize?: number; target?: number }
): Promise<OmniPlanResponse | null> {
  const [owner, name] = repo.split("/");
  const params = new URLSearchParams({ selector });
  if (options?.stageSize) params.set("stage_size", String(options.stageSize));
  if (options?.target) params.set("target", String(options.target));

  return fetchJSON<OmniPlanResponse | null>(
    `/api/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}/plan/omni?${params}`,
    null
  );
}

export async function fetchSettings(repo: string = DEFAULT_REPO): Promise<SettingsMap | null> {
  return fetchJSON<SettingsMap | null>(`/api/settings?repo=${encodeURIComponent(repo)}`, null);
}

export async function postSetting(
  payload: PostSettingPayload,
  options?: PostSettingOptions
): Promise<PostSettingResponse | null> {
  const params = new URLSearchParams({ scope: payload.scope, repo: payload.repo, key: payload.key });
  if (options?.validateOnly) {
    params.set("validateOnly", "true");
  }
  const path = `/api/settings?${params}`;

  try {
    const response = await loggedFetch(`${apiBaseUrl()}${path}`, {
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
  const params = new URLSearchParams({ scope, repo, key });
  const path = `/api/settings?${params}`;

  try {
    const response = await loggedFetch(`${apiBaseUrl()}${path}`, {
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
  const params = new URLSearchParams({ scope, repo });
  const path = `/api/settings/export?${params}`;

  try {
    const response = await loggedFetch(`${apiBaseUrl()}${path}`);
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
  const params = new URLSearchParams({ scope, repo });
  const path = `/api/settings/import?${params}`;

  try {
    const response = await loggedFetch(`${apiBaseUrl()}${path}`, {
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
