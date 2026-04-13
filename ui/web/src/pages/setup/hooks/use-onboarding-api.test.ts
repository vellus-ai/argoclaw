import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { useAuthStore } from "@/stores/use-auth-store";

// Must import after mocking the store
import { useOnboardingApi } from "./use-onboarding-api";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function resetStore() {
  localStorage.clear();
  useAuthStore.setState({
    token: "test-jwt-token",
    refreshToken: "test-refresh",
    userId: "user-1",
    senderID: "",
    connected: false,
    serverInfo: null,
    mustChangePassword: false,
  });
}

function mockFetchSuccess(body: unknown, status = 200) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
    statusText: "OK",
  } as Response);
}

function mockFetchError(status: number, body?: unknown) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue({
    ok: false,
    status,
    json: () => Promise.resolve(body ?? { error: "Error" }),
    statusText: "Error",
  } as Response);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useOnboardingApi", () => {
  beforeEach(() => {
    resetStore();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("getStatus", () => {
    it("should call GET /v1/onboarding/status with auth header", async () => {
      const fetchSpy = mockFetchSuccess({
        onboarding_complete: false,
        workspace_configured: false,
        branding_set: false,
      });

      const api = useOnboardingApi();
      const result = await api.getStatus();

      expect(fetchSpy).toHaveBeenCalledTimes(1);
      const call = fetchSpy.mock.calls[0]!;
      const [url, opts] = call;
      expect(url).toBe("/v1/onboarding/status");
      expect((opts as RequestInit).method).toBe("GET");
      expect((opts as RequestInit).headers).toEqual(
        expect.objectContaining({
          Authorization: "Bearer test-jwt-token",
        }),
      );
      expect(result.onboarding_complete).toBe(false);
    });

    it("should return status with all fields", async () => {
      mockFetchSuccess({
        onboarding_complete: true,
        workspace_configured: true,
        branding_set: true,
        last_completed_state: "channel",
        account_type: "business",
        primary_color: "#FF0000",
      });

      const api = useOnboardingApi();
      const result = await api.getStatus();

      expect(result.onboarding_complete).toBe(true);
      expect(result.workspace_configured).toBe(true);
      expect(result.branding_set).toBe(true);
      expect(result.last_completed_state).toBe("channel");
      expect(result.primary_color).toBe("#FF0000");
    });

    it("should throw and redirect on 401", async () => {
      mockFetchError(401);

      // Mock window.location.href
      const hrefSetter = vi.fn();
      Object.defineProperty(window, "location", {
        value: { href: "/setup" },
        writable: true,
      });
      Object.defineProperty(window.location, "href", {
        set: hrefSetter,
        get: () => "/setup",
      });

      const api = useOnboardingApi();
      await expect(api.getStatus()).rejects.toThrow("HTTP 401");
    });

    it("should throw and redirect on 403", async () => {
      mockFetchError(403);

      const hrefSetter = vi.fn();
      Object.defineProperty(window, "location", {
        value: { href: "/setup" },
        writable: true,
      });
      Object.defineProperty(window.location, "href", {
        set: hrefSetter,
        get: () => "/setup",
      });

      const api = useOnboardingApi();
      await expect(api.getStatus()).rejects.toThrow("HTTP 403");
    });

    it("should abort on timeout", async () => {
      vi.spyOn(globalThis, "fetch").mockImplementation(
        () =>
          new Promise((_resolve, reject) => {
            // Simulate never resolving — AbortController will abort
            setTimeout(() => reject(new DOMException("Aborted", "AbortError")), 100);
          }),
      );

      const api = useOnboardingApi();
      await expect(api.getStatus()).rejects.toThrow();
    });
  });

  describe("callTool", () => {
    it("should call POST /v1/onboarding/action with tool and args", async () => {
      const fetchSpy = mockFetchSuccess({
        ok: true,
        result: "Provider created",
      });

      const api = useOnboardingApi();
      const result = await api.callTool("create_provider", {
        type: "openrouter",
      });

      expect(fetchSpy).toHaveBeenCalledTimes(1);
      const call = fetchSpy.mock.calls[0]!;
      const [url, opts] = call;
      expect(url).toBe("/v1/onboarding/action");
      expect((opts as RequestInit).method).toBe("POST");

      const body = JSON.parse((call[1] as RequestInit).body as string);
      expect(body.tool).toBe("create_provider");
      expect(body.args).toEqual({ type: "openrouter" });
      expect(result.ok).toBe(true);
    });

    it("should include completed_state when provided", async () => {
      const fetchSpy = mockFetchSuccess({ ok: true });

      const api = useOnboardingApi();
      await api.callTool("set_branding", { color: "#FF0000" }, "branding");

      const body = JSON.parse(
        (fetchSpy.mock.calls[0]![1] as RequestInit).body as string,
      );
      expect(body.completed_state).toBe("branding");
    });

    it("should not include completed_state when undefined", async () => {
      const fetchSpy = mockFetchSuccess({ ok: true });

      const api = useOnboardingApi();
      await api.callTool("set_branding", { color: "#FF0000" });

      const body = JSON.parse(
        (fetchSpy.mock.calls[0]![1] as RequestInit).body as string,
      );
      expect(body.completed_state).toBeUndefined();
    });

    it("should return error result from server", async () => {
      mockFetchSuccess({
        ok: false,
        error: "Invalid API key format",
      });

      const api = useOnboardingApi();
      const result = await api.callTool("validate_provider", {
        api_key: "bad",
      });

      expect(result.ok).toBe(false);
      expect(result.error).toBe("Invalid API key format");
    });

    it("should throw on 401", async () => {
      mockFetchError(401);

      Object.defineProperty(window, "location", {
        value: { href: "/setup" },
        writable: true,
      });

      const api = useOnboardingApi();
      await expect(
        api.callTool("test", {}),
      ).rejects.toThrow("HTTP 401");
    });
  });

  describe("updateAgent", () => {
    it("should call PUT /v1/agents/{id} with display_name", async () => {
      const fetchSpy = mockFetchSuccess({});

      const api = useOnboardingApi();
      await api.updateAgent("agent-123", "Jarvis");

      expect(fetchSpy).toHaveBeenCalledTimes(1);
      const call = fetchSpy.mock.calls[0]!;
      const [url, opts] = call;
      expect(url).toBe("/v1/agents/agent-123");
      expect((opts as RequestInit).method).toBe("PUT");

      const body = JSON.parse((opts as RequestInit).body as string);
      expect(body.display_name).toBe("Jarvis");
    });

    it("should throw on 401", async () => {
      mockFetchError(401);

      Object.defineProperty(window, "location", {
        value: { href: "/setup" },
        writable: true,
      });

      const api = useOnboardingApi();
      await expect(
        api.updateAgent("agent-123", "Test"),
      ).rejects.toThrow("HTTP 401");
    });

    it("should throw on 500", async () => {
      mockFetchError(500);

      const api = useOnboardingApi();
      await expect(
        api.updateAgent("agent-123", "Test"),
      ).rejects.toThrow("HTTP 500");
    });
  });
});
