import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock auth-client before importing token-refresh
vi.mock("@/api/auth-client", () => ({
  refresh: vi.fn(),
}));

// Mock auth store
const mockSetJwtAuth = vi.fn();
const mockLogout = vi.fn();
let mockStoreState = {
  refreshToken: "valid-refresh-token",
  userId: "user-123",
  setJwtAuth: mockSetJwtAuth,
  logout: mockLogout,
};

vi.mock("@/stores/use-auth-store", () => ({
  useAuthStore: {
    getState: () => mockStoreState,
  },
}));

import { refreshTokenSingleton } from "./token-refresh";
import { refresh } from "@/api/auth-client";

const mockRefresh = vi.mocked(refresh);

describe("refreshTokenSingleton", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockStoreState = {
      refreshToken: "valid-refresh-token",
      userId: "user-123",
      setJwtAuth: mockSetJwtAuth,
      logout: mockLogout,
    };
  });

  it("should call refresh API and update store on success", async () => {
    mockRefresh.mockResolvedValueOnce({
      access_token: "new-access",
      refresh_token: "new-refresh",
      expires_in: 900,
      user: { id: "user-123", email: "t@t.com", display_name: "", role: "member", status: "active" },
    });

    const result = await refreshTokenSingleton();

    expect(mockRefresh).toHaveBeenCalledWith("valid-refresh-token");
    expect(result).toEqual({
      accessToken: "new-access",
      refreshToken: "new-refresh",
    });
    expect(mockSetJwtAuth).toHaveBeenCalledWith("new-access", "new-refresh", "user-123");
  });

  it("should call logout on refresh failure", async () => {
    mockRefresh.mockRejectedValueOnce(new Error("token expired"));

    const result = await refreshTokenSingleton();

    expect(result).toBeNull();
    expect(mockLogout).toHaveBeenCalledOnce();
  });

  it("should return null when no refresh token in store", async () => {
    mockStoreState.refreshToken = "";

    const result = await refreshTokenSingleton();

    expect(result).toBeNull();
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("should deduplicate concurrent calls", async () => {
    let resolveRefresh: (value: unknown) => void;
    const slowRefresh = new Promise((resolve) => {
      resolveRefresh = resolve;
    });
    mockRefresh.mockReturnValueOnce(slowRefresh as ReturnType<typeof refresh>);

    // Fire two concurrent calls
    const p1 = refreshTokenSingleton();
    const p2 = refreshTokenSingleton();

    // Both should be the same Promise
    expect(p1).toBe(p2);

    // Resolve the refresh
    resolveRefresh!({
      access_token: "new-access",
      refresh_token: "new-refresh",
      expires_in: 900,
      user: { id: "user-123", email: "t@t.com", display_name: "", role: "member", status: "active" },
    });

    const [r1, r2] = await Promise.all([p1, p2]);
    expect(r1).toEqual(r2);

    // Only one API call was made
    expect(mockRefresh).toHaveBeenCalledTimes(1);
  });

  it("should allow new calls after previous completes", async () => {
    mockRefresh
      .mockResolvedValueOnce({
        access_token: "access-1",
        refresh_token: "refresh-1",
        expires_in: 900,
        user: { id: "user-123", email: "t@t.com", display_name: "", role: "member", status: "active" },
      })
      .mockResolvedValueOnce({
        access_token: "access-2",
        refresh_token: "refresh-2",
        expires_in: 900,
        user: { id: "user-123", email: "t@t.com", display_name: "", role: "member", status: "active" },
      });

    await refreshTokenSingleton();
    await refreshTokenSingleton();

    expect(mockRefresh).toHaveBeenCalledTimes(2);
  });
});
