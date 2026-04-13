/**
 * @deprecated Use `useOnboardingStatus` from `./use-onboarding-status` instead.
 *
 * This hook is kept for backward compatibility. It delegates to the
 * onboarding status API endpoint (`GET /v1/onboarding/status`) rather than
 * inferring setup state from providers and agents.
 */

import { useOnboardingStatus } from "./use-onboarding-status";

export type SetupStep = 1 | 2 | 3 | 4 | "complete";

export function useBootstrapStatus() {
  const { needsSetup, loading } = useOnboardingStatus();

  const currentStep: SetupStep = needsSetup ? 1 : "complete";

  return {
    needsSetup,
    currentStep,
    loading,
    /** @deprecated No longer resolved — always empty. Use domain-specific hooks. */
    providers: [] as unknown[],
    /** @deprecated No longer resolved — always empty. Use domain-specific hooks. */
    agents: [] as unknown[],
  };
}
