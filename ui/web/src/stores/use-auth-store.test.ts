import { describe, it, expect, beforeEach } from "vitest";
import { useAuthStore } from "./use-auth-store";
import { LOCAL_STORAGE_KEYS } from "@/lib/constants";

function resetStore() {
  localStorage.clear();
  useAuthStore.setState({
    token: "",
    refreshToken: "",
    userId: "",
    senderID: "",
    connected: false,
    serverInfo: null,
  });
}

describe("useAuthStore", () => {
  beforeEach(resetStore);

  describe("setJwtAuth", () => {
    it("should store access token, refresh token, and userId", () => {
      useAuthStore.getState().setJwtAuth("access-123", "refresh-456", "user-789");

      const state = useAuthStore.getState();
      expect(state.token).toBe("access-123");
      expect(state.refreshToken).toBe("refresh-456");
      expect(state.userId).toBe("user-789");
    });

    it("should persist tokens to localStorage", () => {
      useAuthStore.getState().setJwtAuth("access-123", "refresh-456", "user-789");

      expect(localStorage.getItem(LOCAL_STORAGE_KEYS.TOKEN)).toBe("access-123");
      expect(localStorage.getItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN)).toBe("refresh-456");
      expect(localStorage.getItem(LOCAL_STORAGE_KEYS.USER_ID)).toBe("user-789");
    });
  });

  describe("setCredentials (gateway token)", () => {
    it("should store token and userId without affecting refreshToken", () => {
      useAuthStore.getState().setCredentials("gw-token", "gw-user");

      const state = useAuthStore.getState();
      expect(state.token).toBe("gw-token");
      expect(state.userId).toBe("gw-user");
      expect(state.refreshToken).toBe("");
    });
  });

  describe("setPairing", () => {
    it("should store senderID and userId", () => {
      useAuthStore.getState().setPairing("sender-abc", "pair-user");

      const state = useAuthStore.getState();
      expect(state.senderID).toBe("sender-abc");
      expect(state.userId).toBe("pair-user");
    });
  });

  describe("logout", () => {
    it("should clear all auth state", () => {
      useAuthStore.getState().setJwtAuth("access", "refresh", "user");
      useAuthStore.getState().setConnected(true, { name: "test" });

      useAuthStore.getState().logout();

      const state = useAuthStore.getState();
      expect(state.token).toBe("");
      expect(state.refreshToken).toBe("");
      expect(state.userId).toBe("");
      expect(state.senderID).toBe("");
      expect(state.connected).toBe(false);
      expect(state.serverInfo).toBeNull();
    });

    it("should clear all localStorage keys", () => {
      useAuthStore.getState().setJwtAuth("access", "refresh", "user");

      useAuthStore.getState().logout();

      expect(localStorage.getItem(LOCAL_STORAGE_KEYS.TOKEN)).toBeNull();
      expect(localStorage.getItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN)).toBeNull();
      expect(localStorage.getItem(LOCAL_STORAGE_KEYS.USER_ID)).toBeNull();
      expect(localStorage.getItem(LOCAL_STORAGE_KEYS.SENDER_ID)).toBeNull();
    });
  });

  describe("setConnected", () => {
    it("should update connection state with server info", () => {
      useAuthStore.getState().setConnected(true, { name: "ArgoClaw", version: "1.0" });

      const state = useAuthStore.getState();
      expect(state.connected).toBe(true);
      expect(state.serverInfo).toEqual({ name: "ArgoClaw", version: "1.0" });
    });

    it("should set serverInfo to null when omitted", () => {
      useAuthStore.getState().setConnected(true, { name: "test" });
      useAuthStore.getState().setConnected(false);

      expect(useAuthStore.getState().serverInfo).toBeNull();
    });
  });

  describe("hydration from localStorage", () => {
    it("should hydrate state from localStorage on creation", () => {
      localStorage.setItem(LOCAL_STORAGE_KEYS.TOKEN, "persisted-token");
      localStorage.setItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN, "persisted-refresh");
      localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, "persisted-user");

      // Reset the store to trigger re-hydration
      useAuthStore.setState({
        token: localStorage.getItem(LOCAL_STORAGE_KEYS.TOKEN) ?? "",
        refreshToken: localStorage.getItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN) ?? "",
        userId: localStorage.getItem(LOCAL_STORAGE_KEYS.USER_ID) ?? "",
      });

      const state = useAuthStore.getState();
      expect(state.token).toBe("persisted-token");
      expect(state.refreshToken).toBe("persisted-refresh");
      expect(state.userId).toBe("persisted-user");
    });
  });
});
