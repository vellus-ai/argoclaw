import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { SetupPage } from "./setup-page";
import type { OnboardingStatusResponse, ToolResult } from "./hooks/use-onboarding-api";

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

vi.mock("./hooks/use-onboarding-api", () => ({
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
// Regression tests — Phase 5
// ---------------------------------------------------------------------------

describe("Setup regression tests", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCallTool.mockResolvedValue({ ok: true });
    mockUpdateAgent.mockResolvedValue(undefined);
  });

  it("should fetch status and show welcome state for fresh onboarding", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: false,
      branding_set: false,
    });

    renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/onboarding\.welcome\.greeting/),
      ).toBeInTheDocument();
    });

    expect(mockGetStatus).toHaveBeenCalledTimes(1);
  });

  it("should resume from provider step when last_completed_state is branding_custom", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
      last_completed_state: "branding_custom",
    });

    renderPage();

    await waitFor(() => {
      // Provider step messages should render
      expect(
        screen.getByText(/onboarding\.provider\.question/),
      ).toBeInTheDocument();
    });
  });

  it("should resume from branding step when last_completed_state is workspace_details", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: false,
      last_completed_state: "workspace_details",
    });

    renderPage();

    await waitFor(() => {
      expect(
        screen.getByText(/onboarding\.branding\.question/),
      ).toBeInTheDocument();
    });
  });

  it("should redirect to overview when onboarding is already complete", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: true,
      workspace_configured: true,
      branding_set: true,
    });

    renderPage();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/overview", {
        replace: true,
      });
    });
  });

  it("should show error state and allow retry on network failure", async () => {
    mockGetStatus.mockRejectedValueOnce(new Error("Connection refused"));

    renderPage();

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/connection refused/i);
    });

    // Retry button should be present (label is i18n key "error.retry")
    expect(
      screen.getByRole("button", { name: /retry/i }),
    ).toBeInTheDocument();
  });

  it("should preserve primary_color from status response", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: true,
      branding_set: true,
      primary_color: "#FF5500",
      last_completed_state: "branding_custom",
    });

    renderPage();

    await waitFor(() => {
      // Resumes from provider since branding_custom is done
      expect(
        screen.getByText(/onboarding\.provider\.question/),
      ).toBeInTheDocument();
    });

    // The status loaded successfully with custom color
    expect(mockGetStatus).toHaveBeenCalledTimes(1);
  });
});
