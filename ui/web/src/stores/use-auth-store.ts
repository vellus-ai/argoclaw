import { create } from "zustand";
import { LOCAL_STORAGE_KEYS } from "@/lib/constants";

interface AuthState {
  token: string;
  refreshToken: string;
  userId: string;
  senderID: string; // browser pairing: persistent device identity
  connected: boolean;
  serverInfo: { name?: string; version?: string } | null;

  setCredentials: (token: string, userId: string) => void;
  setJwtAuth: (accessToken: string, refreshToken: string, userId: string) => void;
  setPairing: (senderID: string, userId: string) => void;
  setConnected: (connected: boolean, serverInfo?: { name?: string; version?: string }) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(LOCAL_STORAGE_KEYS.TOKEN) ?? "",
  refreshToken: localStorage.getItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN) ?? "",
  userId: localStorage.getItem(LOCAL_STORAGE_KEYS.USER_ID) ?? "",
  senderID: localStorage.getItem(LOCAL_STORAGE_KEYS.SENDER_ID) ?? "",
  connected: false,
  serverInfo: null,

  setCredentials: (token, userId) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.TOKEN, token);
    localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, userId);
    set({ token, userId });
  },

  setJwtAuth: (accessToken, refreshToken, userId) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.TOKEN, accessToken);
    localStorage.setItem(LOCAL_STORAGE_KEYS.REFRESH_TOKEN, refreshToken);
    localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, userId);
    set({ token: accessToken, refreshToken, userId });
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
    set({ token: "", refreshToken: "", userId: "", senderID: "", connected: false, serverInfo: null });
  },
}));
