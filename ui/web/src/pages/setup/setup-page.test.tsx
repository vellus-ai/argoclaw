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
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
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
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/setup"]}>
      <SetupPage />
    </MemoryRouter>,
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("SetupPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCallTool.mockResolvedValue({ ok: true });
    mockUpdateAgent.mockResolvedValue(undefined);
  });

  it("should fetch status on mount and render welcome state", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: false,
      branding_set: false,
    });

    renderPage();

    // Should show loading initially (spinner or thinking)
    await waitFor(() => {
      // Welcome greeting should appear after status loads
      expect(
        screen.getByText(/onboarding\.welcome\.greeting/),
      ).toBeInTheDocument();
    });

    expect(mockGetStatus).toHaveBeenCalledTimes(1);
  });

  it("should redirect to /overview when already complete", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: true,
      workspace_configured: true,
      branding_set: true,
    });

    renderPage();

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/overview", { replace: true });
    });
  });

  it("should show error state on network failure", async () => {
    mockGetStatus.mockRejectedValue(new Error("Network error"));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/error|failed|network/i)).toBeInTheDocument();
    });
  });

  it("should use h-dvh for mobile viewport height", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: false,
      branding_set: false,
    });

    const { container } = renderPage();

    await waitFor(() => {
      expect(screen.getByText(/onboarding\.welcome\.greeting/)).toBeInTheDocument();
    });

    // The root layout element should use h-dvh, not h-screen
    const layoutDiv = container.firstElementChild;
    expect(layoutDiv?.className).toContain("h-dvh");
    expect(layoutDiv?.className).not.toContain("h-screen");
  });

  it("should render the ARGO header", async () => {
    mockGetStatus.mockResolvedValue({
      onboarding_complete: false,
      workspace_configured: false,
      branding_set: false,
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/argo/i)).toBeInTheDocument();
    });
  });
});
