import { useEffect, useRef } from "react";
import { useAuthStore } from "@/stores/use-auth-store";
import { refreshTokenSingleton } from "@/api/token-refresh";

/** Seconds before expiry to trigger proactive refresh. */
const REFRESH_MARGIN_S = 120; // 2 minutes

/** Maximum safe setTimeout delay (2^31 - 1 ms ≈ 24.8 days). */
const MAX_TIMEOUT_MS = 2_147_483_647;

/**
 * Decode the `exp` claim from a JWT without a library.
 * Returns the expiry as a Unix timestamp (seconds), or null if unparseable.
 */
export function getJwtExp(token: string): number | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const base64 = parts[1]!.replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    const payload: unknown = JSON.parse(atob(padded));
    if (typeof payload !== "object" || payload === null) return null;
    const exp = (payload as Record<string, unknown>).exp;
    return typeof exp === "number" ? exp : null;
  } catch {
    return null;
  }
}

/**
 * Proactively refreshes the JWT access token before it expires.
 *
 * Schedules a timer to fire `REFRESH_MARGIN_S` seconds before the token's
 * `exp` claim. Uses the centralized `refreshTokenSingleton` to avoid race
 * conditions with the HttpClient's reactive 401 refresh.
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
    if (exp === null) return; // Not a JWT (gateway token) — skip

    const nowS = Math.floor(Date.now() / 1000);
    const delayS = exp - REFRESH_MARGIN_S - nowS;

    if (delayS <= 0) {
      // Token already expired or about to — refresh immediately
      refreshTokenSingleton();
      return;
    }

    const delayMs = Math.min(delayS * 1000, MAX_TIMEOUT_MS);
    timerRef.current = setTimeout(() => {
      refreshTokenSingleton();
    }, delayMs);

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [token, refreshTokenValue]);
}
