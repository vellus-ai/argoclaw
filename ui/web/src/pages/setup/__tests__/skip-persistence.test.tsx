import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { SetupPage } from "../setup-page";
import type {
  OnboardingStatusResponse,
  ToolResult,
} from "../hooks/use-onboarding-api";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual =
    await vi.importActual<typeof import("react-router")>("react-router");
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
  initReactI18next: { type: "3rdParty", init: vi.fn() },
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/setup"]}>
      <SetupPage />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests — skip persistence (Bug fix: provider skip loop)
// ---------------------------------------------------------------------------

describe("skip persistence — calls API to persist completion", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCallTool.mockResolvedValue({ ok: true });
    mockUpdateAgent.mockResolvedValue(undefined);
  });

  it("should call complete_onboarding API when user skips channel step", async () => {
    // Start at channel state (provider_config already completed)
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
      last_completed_state: "provider_config",
    });

    renderPage();

    // Wait for channel state to render
    await waitFor(() => {
      expect(
        screen.getByText(/onboarding\.channel\.question/),
      ).toBeInTheDocument();
    });

    // Find and click the skip button
    const skipButton = screen.getByRole("button", {
      name: /onboarding\.channel\.skip/,
    });
    await userEvent.click(skipButton);

    // Verify that callTool was called with complete_onboarding
    await waitFor(() => {
      expect(mockCallTool).toHaveBeenCalledWith(
        "complete_onboarding",
        {},
        "channel",
      );
    });
  });

  it("should call complete_onboarding API when user selects a channel", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
      last_completed_state: "provider_config",
    });

    renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/onboarding\.channel\.question/),
      ).toBeInTheDocument();
    });

    // Select telegram
    const telegramButton = screen.getByRole("button", {
      name: /Telegram/,
    });
    await userEvent.click(telegramButton);

    // Should still call complete_onboarding
    await waitFor(() => {
      expect(mockCallTool).toHaveBeenCalledWith(
        "complete_onboarding",
        {},
        "channel",
      );
    });
  });

  it("should NOT call complete_onboarding when user skips provider (not last step)", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
      last_completed_state: "branding_custom",
    });

    renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/onboarding\.provider\.question/),
      ).toBeInTheDocument();
    });

    // Skip provider
    const skipButton = screen.getByRole("button", {
      name: /onboarding\.provider\.skip/,
    });
    await userEvent.click(skipButton);

    // Should NOT have called complete_onboarding yet
    // (only channel step triggers completion)
    expect(mockCallTool).not.toHaveBeenCalledWith(
      "complete_onboarding",
      expect.anything(),
      expect.anything(),
    );
  });

  it("should handle complete_onboarding API failure gracefully", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
      last_completed_state: "provider_config",
    });
    mockCallTool.mockRejectedValue(new Error("Network error"));

    renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/onboarding\.channel\.question/),
      ).toBeInTheDocument();
    });

    // Skip channel — should not throw even if API fails
    const skipButton = screen.getByRole("button", {
      name: /onboarding\.channel\.skip/,
    });
    await userEvent.click(skipButton);

    // Frontend should still transition to complete state
    await waitFor(() => {
      expect(
        screen.getByText(/onboarding\.complete\.title/),
      ).toBeInTheDocument();
    });
  });
});
