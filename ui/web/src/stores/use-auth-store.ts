import { create } from "zustand";
import { LOCAL_STORAGE_KEYS } from "@/lib/constants";

/**
 * Decode the payload of a JWT token (base64url-encoded middle segment).
 * Returns the parsed JSON object, or null if decoding fails.
 */
export function decodeJwtPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = parts[1] ?? "";
    // base64url → base64
    const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
    const json = atob(base64);
    return JSON.parse(json) as Record<string, unknown>;
  } catch {
    return null;
  }
}

function extractMustChangePassword(token: string): boolean {
  const payload = decodeJwtPayload(token);
  if (!payload) return false;
  return payload["mcp"] === true;
}

interface AuthState {
  token: string;
  refreshToken: string;
  userId: string;
  senderID: string; // browser pairing: persistent device identity
  connected: boolean;
  serverInfo: { name?: string; version?: string } | null;
  mustChangePassword: boolean;

  setCredentials: (token: string, userId: string) => void;
  setJwtAuth: (accessToken: string, refreshToken: string, userId: string) => void;
  setPairing: (senderID: string, userId: string) => void;
  setConnected: (connected: boolean, serverInfo?: { name?: string; version?: string }) => void;
  logout: () => void;
}

function initialMustChangePassword(): boolean {
  const token = localStorage.getItem(LOCAL_STORAGE_KEYS.TOKEN) ?? "";
  return extractMustChangePassword(token);
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(LOCAL_STORAGE_KEYS.TOKEN) ?? "",
  refreshToken: localStorage.getItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN) ?? "",
  userId: localStorage.getItem(LOCAL_STORAGE_KEYS.USER_ID) ?? "",
  senderID: localStorage.getItem(LOCAL_STORAGE_KEYS.SENDER_ID) ?? "",
  connected: false,
  serverInfo: null,
  mustChangePassword: initialMustChangePassword(),

  setCredentials: (token, userId) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.TOKEN, token);
    localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, userId);
    set({ token, userId, mustChangePassword: extractMustChangePassword(token) });
  },

  setJwtAuth: (accessToken, refreshToken, userId) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.TOKEN, accessToken);
    localStorage.setItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN, refreshToken);
    localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, userId);
    set({
      token: accessToken,
      refreshToken,
      userId,
      mustChangePassword: extractMustChangePassword(accessToken),
    });
  },

  setPairing: (senderID, userId) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.SENDER_ID, senderID);
    localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, userId);
    set({ senderID, userId });
  },

  setConnected: (connected, serverInfo) => {
    set({ connected, serverInfo: serverInfo ?? null });
  },

  logout: () => {
    localStorage.removeItem(LOCAL_STORAGE_KEYS.TOKEN);
    localStorage.removeItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN);
    localStorage.removeItem(LOCAL_STORAGE_KEYS.USER_ID);
    localStorage.removeItem(LOCAL_STORAGE_KEYS.SENDER_ID);
    set({
      token: "",
      refreshToken: "",
      userId: "",
      senderID: "",
      connected: false,
      serverInfo: null,
      mustChangePassword: false,
    });
  },
}));
