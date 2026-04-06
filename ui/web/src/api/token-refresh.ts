/**
 * Centralized JWT refresh singleton.
 *
 * Both the proactive timer (useJwtRefresh) and the reactive 401 handler
 * (HttpClient.tryRefresh) must go through this module to avoid race
 * conditions when the backend uses rotate-on-refresh (one-time-use tokens).
 *
 * The singleton deduplicates concurrent refresh attempts: if a refresh is
 * already in flight, subsequent callers receive the same Promise.
 */

import { refresh as callRefreshAPI } from "@/api/auth-client";
import { useAuthStore } from "@/stores/use-auth-store";

export interface RefreshResult {
  accessToken: string;
  refreshToken: string;
}

let inflightRefresh: Promise<RefreshResult | null> | null = null;

/**
 * Attempt to refresh the JWT token. Returns new tokens on success, null on failure.
 * Concurrent calls are deduplicated — only one HTTP request is made.
 * On success, the auth store is updated automatically.
 * On failure, the auth store is cleared (logout).
 */
export function refreshTokenSingleton(): Promise<RefreshResult | null> {
  if (inflightRefresh) return inflightRefresh;

  inflightRefresh = (async (): Promise<RefreshResult | null> => {
    const { refreshToken: rt, userId } = useAuthStore.getState();
    if (!rt) return null;

    try {
      const res = await callRefreshAPI(rt);
      const result: RefreshResult = {
        accessToken: res.access_token,
        refreshToken: res.refresh_token,
      };
      // Persist to store — read fresh userId in case it changed
      const freshUserId = useAuthStore.getState().userId || userId;
      useAuthStore.getState().setJwtAuth(result.accessToken, result.refreshToken, freshUserId);
      return result;
    } catch {
      useAuthStore.getState().logout();
      return null;
    }
  })().finally(() => {
    inflightRefresh = null;
  });

  return inflightRefresh;
}
