import { useEffect, useState } from "react";
import { useOnboardingApi } from "./use-onboarding-api";
import type { OnboardingStatusResponse } from "./use-onboarding-api";

export function useOnboardingStatus(): {
  loading: boolean;
  status: OnboardingStatusResponse | null;
  error: string | null;
  needsSetup: boolean;
} {
  const api = useOnboardingApi();
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<OnboardingStatusResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Respect legacy skip flag — if user previously skipped, treat as complete
    const skipFlag =
      typeof window !== "undefined" &&
      localStorage.getItem("setup_skipped") === "1";
    if (skipFlag) {
      setStatus({
        onboarding_complete: true,
        workspace_configured: false,
        branding_set: false,
      });
      setLoading(false);
      return;
    }

    const controller = new AbortController();
    let cancelled = false;

    async function fetchStatus() {
      try {
        setLoading(true);
        setError(null);
        const result = await api.getStatus(controller.signal);
        if (!cancelled) {
          setStatus(result);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Unknown error");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    fetchStatus();

    return () => {
      cancelled = true;
      controller.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const needsSetup = status !== null && !status.onboarding_complete;

  return { loading, status, error, needsSetup };
}
