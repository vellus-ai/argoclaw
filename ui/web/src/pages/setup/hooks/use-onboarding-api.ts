import { useAuthStore } from "@/stores/use-auth-store";

export interface OnboardingStatusResponse {
  onboarding_complete: boolean;
  workspace_configured: boolean;
  branding_set: boolean;
  last_completed_state?: string;
  account_type?: string;
  primary_color?: string;
}

export interface ToolResult {
  ok: boolean;
  result?: string;
  error?: string;
}

export interface OnboardingApi {
  getStatus(): Promise<OnboardingStatusResponse>;
  callTool(
    tool: string,
    args: Record<string, unknown>,
    completedState?: string,
  ): Promise<ToolResult>;
  updateAgent(agentId: string, name: string): Promise<void>;
}

const TIMEOUT_MS = 30_000;

function getToken(): string {
  return useAuthStore.getState().token;
}

function authHeaders(): Record<string, string> {
  return {
    "Content-Type": "application/json",
    Authorization: `Bearer ${getToken()}`,
  };
}

async function handleAuthError(status: number): Promise<never> {
  if (status === 401 || status === 403) {
    // Redirect to login — clear auth state
    useAuthStore.getState().logout();
    window.location.href = "/login";
  }
  throw new Error(`HTTP ${status}`);
}

async function getStatus(): Promise<OnboardingStatusResponse> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

  try {
    const res = await fetch("/v1/onboarding/status", {
      method: "GET",
      headers: authHeaders(),
      signal: controller.signal,
    });

    if (!res.ok) {
      return handleAuthError(res.status);
    }

    return (await res.json()) as OnboardingStatusResponse;
  } finally {
    clearTimeout(timer);
  }
}

async function callTool(
  tool: string,
  args: Record<string, unknown>,
  completedState?: string,
): Promise<ToolResult> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

  try {
    const body: Record<string, unknown> = { tool, args };
    if (completedState) {
      body.completed_state = completedState;
    }

    const res = await fetch("/v1/onboarding/action", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify(body),
      signal: controller.signal,
    });

    if (!res.ok) {
      return handleAuthError(res.status);
    }

    return (await res.json()) as ToolResult;
  } finally {
    clearTimeout(timer);
  }
}

async function updateAgent(agentId: string, name: string): Promise<void> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS);

  try {
    const res = await fetch(`/v1/agents/${agentId}`, {
      method: "PUT",
      headers: authHeaders(),
      body: JSON.stringify({ display_name: name }),
      signal: controller.signal,
    });

    if (!res.ok) {
      return handleAuthError(res.status);
    }
  } finally {
    clearTimeout(timer);
  }
}

export function useOnboardingApi(): OnboardingApi {
  return { getStatus, callTool, updateAgent };
}
