import { describe, it, expect, vi, beforeEach } from "vitest";
import { HttpClient } from "./http-client";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

function jsonResponse(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "Unauthorized",
    json: () => Promise.resolve(body),
  };
}

describe("HttpClient", () => {
  let client: HttpClient;
  let currentToken: string;

  beforeEach(() => {
    mockFetch.mockReset();
    currentToken = "initial-token";
    client = new HttpClient(
      "http://localhost:9600",
      () => currentToken,
      () => "user-1",
      () => "",
    );
  });

  describe("auto token refresh on 401", () => {
    it("should refresh token and retry on 401", async () => {
      // First call: 401
      mockFetch.mockResolvedValueOnce(jsonResponse(401, { error: "expired" }));
      // Retry after refresh: 200
      mockFetch.mockResolvedValueOnce(jsonResponse(200, { data: "ok" }));

      client.setRefreshFn(async () => {
        currentToken = "refreshed-token";
        return { accessToken: "refreshed-token", refreshToken: "new-rt" };
      });

      const onRefreshed = vi.fn();
      client.onTokenRefreshed = onRefreshed;

      const result = await client.get<{ data: string }>("/v1/agents");

      expect(result.data).toBe("ok");
      expect(mockFetch).toHaveBeenCalledTimes(2);
      expect(onRefreshed).toHaveBeenCalledWith("refreshed-token", "new-rt");
    });

    it("should call onAuthFailure when refresh fails", async () => {
      mockFetch.mockResolvedValueOnce(jsonResponse(401, { error: "expired" }));

      client.setRefreshFn(async () => null);

      const onAuthFailure = vi.fn();
      client.onAuthFailure = onAuthFailure;

      await expect(client.get("/v1/agents")).rejects.toThrow();
      expect(onAuthFailure).toHaveBeenCalledOnce();
    });

    it("should call onAuthFailure when no refreshFn is set", async () => {
      mockFetch.mockResolvedValueOnce(jsonResponse(401, { error: "expired" }));

      const onAuthFailure = vi.fn();
      client.onAuthFailure = onAuthFailure;

      await expect(client.get("/v1/agents")).rejects.toThrow();
      expect(onAuthFailure).toHaveBeenCalledOnce();
    });

    it("should not retry more than once", async () => {
      // Both original and retry return 401
      mockFetch.mockResolvedValue(jsonResponse(401, { error: "expired" }));

      client.setRefreshFn(async () => ({
        accessToken: "still-bad",
        refreshToken: "rt",
      }));

      const onAuthFailure = vi.fn();
      client.onAuthFailure = onAuthFailure;

      await expect(client.get("/v1/agents")).rejects.toThrow();
      // Original + 1 retry = 2 calls (not infinite)
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    it("should deduplicate concurrent refresh attempts", async () => {
      let refreshCallCount = 0;
      mockFetch.mockResolvedValue(jsonResponse(401, { error: "expired" }));

      client.setRefreshFn(async () => {
        refreshCallCount++;
        // After a short delay, return new tokens
        await new Promise((r) => setTimeout(r, 50));
        currentToken = "refreshed";
        return { accessToken: "refreshed", refreshToken: "rt" };
      });

      // Make mockFetch return 200 after first 2 calls (both 401s)
      mockFetch
        .mockResolvedValueOnce(jsonResponse(401, { error: "expired" }))
        .mockResolvedValueOnce(jsonResponse(401, { error: "expired" }))
        .mockResolvedValue(jsonResponse(200, { ok: true }));

      // Fire two requests simultaneously
      const [r1, r2] = await Promise.allSettled([
        client.get("/v1/a"),
        client.get("/v1/b"),
      ]);

      // Refresh should only be called once despite two 401s
      expect(refreshCallCount).toBe(1);
      // At least one should succeed after refresh
      const results = [r1, r2];
      expect(results.some((r) => r.status === "fulfilled")).toBe(true);
    });
  });

  describe("request headers", () => {
    it("should include Authorization and X-ArgoClaw-User-Id headers", async () => {
      mockFetch.mockResolvedValueOnce(jsonResponse(200, {}));

      await client.get("/v1/test");

      const headers = mockFetch.mock.calls[0]![1].headers as Record<string, string>;
      expect(headers["Authorization"]).toBe("Bearer initial-token");
      expect(headers["X-ArgoClaw-User-Id"]).toBe("user-1");
    });
  });

  describe("network error", () => {
    it("should throw ApiError with NETWORK_ERROR code", async () => {
      mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));

      await expect(client.get("/v1/test")).rejects.toThrow("Cannot connect to server");
    });
  });
});
