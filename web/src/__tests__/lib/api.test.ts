import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  deleteSetting,
  exportSettingsYAML,
  fetchSettings,
  importSettingsYAML,
  postSetting,
} from "../../lib/api";

const mockFetch = vi.fn();
global.fetch = mockFetch;

beforeEach(() => {
  mockFetch.mockReset();
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("settings API functions", () => {
  describe("fetchSettings", () => {
    it("returns settings map on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ duplicate_threshold: 0.9, weight_age: 0.2 }),
      });

      const result = await fetchSettings("owner/repo");

      expect(result).toEqual({ duplicate_threshold: 0.9, weight_age: 0.2 });
    });

    it("returns null on HTTP error", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false });

      const result = await fetchSettings("owner/repo");

      expect(result).toBeNull();
    });

    it("returns null on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const result = await fetchSettings("owner/repo");

      expect(result).toBeNull();
    });
  });

  describe("postSetting", () => {
    it("posts setting and returns response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ updated: true }),
      });

      const result = await postSetting({
        scope: "global",
        repo: "owner/repo",
        key: "duplicate_threshold",
        value: 0.9,
      });

      expect(result).toEqual({ updated: true });
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/settings/global/duplicate_threshold"),
        expect.objectContaining({ method: "POST", body: JSON.stringify({ value: 0.9 }) })
      );
    });

    it("uses validate endpoint when validateOnly is true", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ valid: true }),
      });

      const result = await postSetting(
        { scope: "global", repo: "owner/repo", key: "test", value: 1 },
        { validateOnly: true }
      );

      expect(result).toEqual({ valid: true });
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/validate"),
        expect.any(Object)
      );
    });

    it("returns null on error", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false });

      const result = await postSetting({
        scope: "global",
        repo: "owner/repo",
        key: "test",
        value: 1,
      });

      expect(result).toBeNull();
    });
  });

  describe("deleteSetting", () => {
    it("deletes setting and returns response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ deleted: true }),
      });

      const result = await deleteSetting("global", "owner/repo", "test_key");

      expect(result).toEqual({ deleted: true });
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/settings/global/test_key"),
        expect.objectContaining({ method: "DELETE" })
      );
    });

    it("returns null on error", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false });

      const result = await deleteSetting("global", "owner/repo", "test_key");

      expect(result).toBeNull();
    });
  });

  describe("exportSettingsYAML", () => {
    it("returns YAML string on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => "duplicate_threshold: 0.9\n",
      });

      const result = await exportSettingsYAML("global", "owner/repo");

      expect(result).toBe("duplicate_threshold: 0.9\n");
    });

    it("returns null on error", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false });

      const result = await exportSettingsYAML("global", "owner/repo");

      expect(result).toBeNull();
    });
  });

  describe("importSettingsYAML", () => {
    it("imports YAML and returns response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ imported: true }),
      });

      const result = await importSettingsYAML(
        "global",
        "owner/repo",
        "duplicate_threshold: 0.9\n"
      );

      expect(result).toEqual({ imported: true });
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/settings/global/import"),
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "text/yaml" },
          body: "duplicate_threshold: 0.9\n",
        })
      );
    });

    it("returns null on error", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false });

      const result = await importSettingsYAML("global", "owner/repo", "key: value\n");

      expect(result).toBeNull();
    });
  });
});
