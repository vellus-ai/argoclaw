import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useAuthStore } from "@/stores/use-auth-store";
import { useOnboardingStatus } from "./use-onboarding-status";

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

function mockFetchSuccess(body: unknown) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue({
    ok: true,
    status: 200,
    json: () => Promise.resolve(body),
    statusText: "OK",
  } as Response);
}

function mockFetchError(status: number) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue({
    ok: false,
    status,
    json: () => Promise.resolve({ error: `HTTP ${status}` }),
    statusText: "Error",
  } as Response);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useOnboardingStatus", () => {
  beforeEach(() => {
    resetStore();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should start with loading=true", () => {
    // Never resolve the fetch
    vi.spyOn(globalThis, "fetch").mockReturnValue(new Promise(() => {}));

    const { result } = renderHook(() => useOnboardingStatus());
    expect(result.current.loading).toBe(true);
    expect(result.current.status).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it("should return status and needsSetup=true when not complete", async () => {
    mockFetchSuccess({
      onboarding_complete: false,
      workspace_configured: false,
      branding_set: false,
    });

    const { result } = renderHook(() => useOnboardingStatus());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.status).toBeDefined();
    expect(result.current.status?.onboarding_complete).toBe(false);
    expect(result.current.needsSetup).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it("should return needsSetup=false when complete", async () => {
    mockFetchSuccess({
      onboarding_complete: true,
      workspace_configured: true,
      branding_set: true,
    });

    const { result } = renderHook(() => useOnboardingStatus());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.needsSetup).toBe(false);
    expect(result.current.status?.onboarding_complete).toBe(true);
  });

  it("should set error on fetch failure", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(
      new Error("Network error"),
    );

    const { result } = renderHook(() => useOnboardingStatus());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe("Network error");
    expect(result.current.status).toBeNull();
    expect(result.current.needsSetup).toBe(false);
  });

  it("should handle 401 error", async () => {
    mockFetchError(401);

    Object.defineProperty(window, "location", {
      value: { href: "/setup" },
      writable: true,
    });

    const { result } = renderHook(() => useOnboardingStatus());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBeDefined();
    expect(result.current.status).toBeNull();
  });

  it("should respect legacy setup_skipped localStorage flag", async () => {
    localStorage.setItem("setup_skipped", "1");

    const fetchSpy = vi.spyOn(globalThis, "fetch");

    const { result } = renderHook(() => useOnboardingStatus());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Should NOT call the API at all
    expect(fetchSpy).not.toHaveBeenCalled();
    // Should report as complete
    expect(result.current.status?.onboarding_complete).toBe(true);
    expect(result.current.needsSetup).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("should return all status fields from API", async () => {
    mockFetchSuccess({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
      last_completed_state: "branding",
      account_type: "business",
      primary_color: "#FF0000",
    });

    const { result } = renderHook(() => useOnboardingStatus());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.status?.workspace_configured).toBe(true);
    expect(result.current.status?.branding_set).toBe(true);
    expect(result.current.status?.last_completed_state).toBe("branding");
    expect(result.current.status?.primary_color).toBe("#FF0000");
  });
});
