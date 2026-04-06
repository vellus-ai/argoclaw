import { useEffect, useRef } from "react";
import { useAuthStore } from "@/stores/use-auth-store";
import { refresh } from "@/api/auth-client";

/** Seconds before expiry to trigger proactive refresh. */
const REFRESH_MARGIN_S = 120; // 2 minutes

/**
 * Decode the `exp` claim from a JWT without a library.
 * Returns the expiry as a Unix timestamp (seconds), or null if unparseable.
 */
function getJwtExp(token: string): number | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1]!.replace(/-/g, "+").replace(/_/g, "/")));
    return typeof payload.exp === "number" ? payload.exp : null;
  } catch {
    return null;
  }
}

/**
 * Proactively refreshes the JWT access token before it expires.
 *
 * Schedules a timer to fire `REFRESH_MARGIN_S` seconds before the token's
 * `exp` claim. On success the auth store is updated with new tokens;
 * on failure the user is logged out.
 *
 * Only runs when a refresh token is present (email/password auth).
 * Gateway-token and pairing sessions are unaffected.
 */
export function useJwtRefresh() {
  const token = useAuthStore((s) => s.token);
  const refreshTokenValue = useAuthStore((s) => s.refreshToken);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    // Clear any previous timer
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    // Only schedule refresh for JWT sessions (has refresh token)
    if (!token || !refreshTokenValue) return;

    const exp = getJwtExp(token);
    if (!exp) return; // Not a JWT (gateway token) — skip

    const nowS = Math.floor(Date.now() / 1000);
    const delayS = exp - REFRESH_MARGIN_S - nowS;

    if (delayS <= 0) {
      // Token already expired or about to — refresh immediately
      doRefresh(refreshTokenValue);
      return;
    }

    timerRef.current = setTimeout(() => {
      doRefresh(refreshTokenValue);
    }, delayS * 1000);

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [token, refreshTokenValue]);
}

async function doRefresh(rt: string) {
  try {
    const res = await refresh(rt);
    const { userId } = useAuthStore.getState();
    useAuthStore.getState().setJwtAuth(res.access_token, res.refresh_token, userId);
  } catch {
    // Refresh failed — session expired, force logout
    useAuthStore.getState().logout();
  }
}
