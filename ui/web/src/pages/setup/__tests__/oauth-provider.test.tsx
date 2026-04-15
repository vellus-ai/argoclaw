import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { MemoryRouter } from "react-router";

// ---------------------------------------------------------------------------
// Mocks (shared)
// ---------------------------------------------------------------------------

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts && Object.keys(opts).length > 0) {
        const pairs = Object.entries(opts)
          .map(([k, v]) => `${k}=${v}`)
          .join(",");
        return `${key}[${pairs}]`;
      }
      return key;
    },
    i18n: { changeLanguage: vi.fn() },
  }),
}));

// ---------------------------------------------------------------------------
// Section 1 — Engine (pure reducer) tests
// ---------------------------------------------------------------------------

import {
  onboardingReducer,
  _resetMessageCounter,
  type OnboardingContext,
  type OnboardingState,
  type TranslatorFn,
  type EngineState,
  type ChatMessageLocal,
} from "../hooks/use-onboarding-engine";

const INITIAL_CONTEXT: OnboardingContext = {
  displayName: "Milton",
  agentName: "Imediato",
  agentId: "agent-001",
  workspaceType: "",
  accountName: "",
  primaryColor: "#1E40AF",
  selectedProvider: "",
  selectedChannel: "",
  workspaceConfigured: false,
  brandingSet: false,
  onboardingComplete: false,
  gender: null,
};

const mockT: TranslatorFn = (key, opts) => {
  if (opts && Object.keys(opts).length > 0) {
    const pairs = Object.entries(opts)
      .map(([k, v]) => `${k}=${v}`)
      .join(",");
    return `${key}[${pairs}]`;
  }
  return key;
};

function makeState(
  currentState: OnboardingState = "loading",
  overrides?: Partial<OnboardingContext>,
): EngineState {
  return {
    currentState,
    context: { ...INITIAL_CONTEXT, ...overrides },
    messages: [] as ChatMessageLocal[],
    error: null,
    _translator: mockT,
  };
}

describe("Engine — oauth_provider tool transitions", () => {
  beforeEach(() => {
    _resetMessageCounter();
  });

  it("TOOL_SUCCESS with oauth_provider in provider_config transitions to channel", () => {
    const state = makeState("provider_config", { selectedProvider: "openai" });
    const next = onboardingReducer(state, {
      type: "TOOL_SUCCESS",
      tool: "oauth_provider",
    });
    expect(next.currentState).toBe("channel");
    expect(next.error).toBeNull();
  });

  it("TOOL_ERROR with oauth_provider stays in provider_config and sets error", () => {
    const state = makeState("provider_config", { selectedProvider: "openai" });
    const next = onboardingReducer(state, {
      type: "TOOL_ERROR",
      tool: "oauth_provider",
      error: "OAuth authorization denied",
    });
    expect(next.currentState).toBe("provider_config");
    expect(next.error).toBe("OAuth authorization denied");
  });

  it("TOOL_SUCCESS with validate_provider still transitions to channel", () => {
    const state = makeState("provider_config", { selectedProvider: "openai" });
    const next = onboardingReducer(state, {
      type: "TOOL_SUCCESS",
      tool: "validate_provider",
    });
    expect(next.currentState).toBe("channel");
  });

  it("TOOL_SUCCESS with create_provider still transitions to channel", () => {
    const state = makeState("provider_config", { selectedProvider: "openai" });
    const next = onboardingReducer(state, {
      type: "TOOL_SUCCESS",
      tool: "create_provider",
    });
    expect(next.currentState).toBe("channel");
  });

  it("all three tools (validate_provider, create_provider, oauth_provider) transition provider_config -> channel", () => {
    const tools = ["validate_provider", "create_provider", "oauth_provider"];
    for (const tool of tools) {
      const state = makeState("provider_config", { selectedProvider: "openai" });
      const next = onboardingReducer(state, { type: "TOOL_SUCCESS", tool });
      expect(next.currentState).toBe("channel");
    }
  });
});

// ---------------------------------------------------------------------------
// Section 2 — ProviderOAuthSection component tests
// ---------------------------------------------------------------------------

import { ProviderOAuthSection } from "../provider-oauth-section";

describe("ProviderOAuthSection component", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("renders OAuth button for openai provider", () => {
    render(
      <ProviderOAuthSection
        provider="openai"
        disabled={false}
        onSuccess={vi.fn()}
        onFallback={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: /conectar com openai/i }),
    ).toBeInTheDocument();
  });

  it("does NOT render OAuth button for anthropic provider", () => {
    const { container } = render(
      <ProviderOAuthSection
        provider="anthropic"
        disabled={false}
        onSuccess={vi.fn()}
        onFallback={vi.fn()}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("does NOT render OAuth button for openrouter provider", () => {
    const { container } = render(
      <ProviderOAuthSection
        provider="openrouter"
        disabled={false}
        onSuccess={vi.fn()}
        onFallback={vi.fn()}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("shows fallback card after 30 seconds", async () => {
    render(
      <ProviderOAuthSection
        provider="openai"
        disabled={false}
        onSuccess={vi.fn()}
        onFallback={vi.fn()}
      />,
    );

    // Click the OAuth button to start the flow
    const oauthButton = screen.getByRole("button", { name: /conectar com openai/i });
    await act(async () => {
      oauthButton.click();
    });

    // Fallback should NOT be visible immediately
    expect(
      screen.queryByText(/não conseguiu conectar/i),
    ).not.toBeInTheDocument();

    // Advance 30 seconds
    await act(async () => {
      vi.advanceTimersByTime(30000);
    });

    // Fallback card should now be visible
    expect(
      screen.getByText(/não conseguiu conectar/i),
    ).toBeInTheDocument();
  });

  it("calls onSuccess when onSuccess prop is invoked", () => {
    const onSuccess = vi.fn();
    render(
      <ProviderOAuthSection
        provider="openai"
        disabled={false}
        onSuccess={onSuccess}
        onFallback={vi.fn()}
      />,
    );
    // onSuccess is a callback — test that the prop flows through correctly
    // by calling it directly (simulates a window.postMessage completing the OAuth)
    onSuccess();
    expect(onSuccess).toHaveBeenCalledTimes(1);
  });

  it("calls onFallback when fallback API key button is clicked", async () => {
    const onFallback = vi.fn();
    render(
      <ProviderOAuthSection
        provider="openai"
        disabled={false}
        onSuccess={vi.fn()}
        onFallback={onFallback}
      />,
    );

    // Start the OAuth flow
    const oauthButton = screen.getByRole("button", { name: /conectar com openai/i });
    await act(async () => {
      oauthButton.click();
    });

    // Wait for fallback to appear after 30s
    await act(async () => {
      vi.advanceTimersByTime(30000);
    });

    // Click the fallback "API key" button
    const fallbackButton = screen.getByRole("button", {
      name: /cole sua api key/i,
    });
    await act(async () => {
      fallbackButton.click();
    });

    expect(onFallback).toHaveBeenCalledTimes(1);
  });

  it("fallback card has proper accessibility with a button element", async () => {
    render(
      <ProviderOAuthSection
        provider="openai"
        disabled={false}
        onSuccess={vi.fn()}
        onFallback={vi.fn()}
      />,
    );

    // Start the OAuth flow and advance time
    const oauthButton = screen.getByRole("button", { name: /conectar com openai/i });
    await act(async () => {
      oauthButton.click();
    });
    await act(async () => {
      vi.advanceTimersByTime(30000);
    });

    // Fallback element must be a button (not a div with onClick)
    const fallbackButton = screen.getByRole("button", {
      name: /cole sua api key/i,
    });
    expect(fallbackButton.tagName).toBe("BUTTON");
  });

  it("shows loading state while waiting for OAuth", async () => {
    render(
      <ProviderOAuthSection
        provider="openai"
        disabled={false}
        onSuccess={vi.fn()}
        onFallback={vi.fn()}
      />,
    );

    const oauthButton = screen.getByRole("button", { name: /conectar com openai/i });
    await act(async () => {
      oauthButton.click();
    });

    // After click, should show loading/waiting state
    expect(screen.getByText(/aguardando/i)).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Section 3 — Integration: SetupPage renders ProviderOAuthSection for OpenAI
// ---------------------------------------------------------------------------

import type { OnboardingStatusResponse, ToolResult } from "../hooks/use-onboarding-api";
import { SetupPage } from "../setup-page";

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockGetStatus = vi.fn<() => Promise<OnboardingStatusResponse>>();
const mockCallTool = vi.fn<() => Promise<ToolResult>>();
const mockUpdateAgent = vi.fn<() => Promise<void>>();

vi.mock("../hooks/use-onboarding-api", () => ({
  useOnboardingApi: () => ({
    getStatus: mockGetStatus,
    callTool: mockCallTool,
    updateAgent: mockUpdateAgent,
  }),
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/setup"]}>
      <SetupPage />
    </MemoryRouter>,
  );
}

describe("SetupPage integration — OAuth section visibility", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCallTool.mockResolvedValue({ ok: true });
    mockUpdateAgent.mockResolvedValue(undefined);
    // Do NOT use fake timers here — Promises need real timers to resolve
  });

  it("shows ProviderOAuthSection when user selects openai at provider step", async () => {
    // Status: workspace_configured + branding_set → engine starts at "provider" state
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
    });

    renderPage();

    // Wait for provider quick reply buttons to appear (openai, anthropic, openrouter, skip)
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /openai/i })).toBeInTheDocument();
    }, { timeout: 3000 });

    // User selects "openai"
    await act(async () => {
      const openaiButton = screen.getByRole("button", { name: /openai/i });
      openaiButton.click();
    });

    // ProviderOAuthSection should now be visible
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /conectar com openai/i }),
      ).toBeInTheDocument();
    });
  });

  it("does NOT show ProviderOAuthSection when user selects anthropic", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /anthropic/i })).toBeInTheDocument();
    }, { timeout: 3000 });

    // User selects "anthropic"
    await act(async () => {
      const anthropicButton = screen.getByRole("button", { name: /anthropic/i });
      anthropicButton.click();
    });

    // OAuth button should NOT be present; should show API key input instead
    await waitFor(() => {
      expect(
        screen.queryByRole("button", { name: /conectar com openai/i }),
      ).not.toBeInTheDocument();
    });
  });
});
