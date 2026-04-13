import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { RequireSetup } from "./require-setup";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual =
    await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    Navigate: (props: { to: string; replace?: boolean }) => {
      mockNavigate(props.to, { replace: props.replace });
      return null;
    },
  };
});

const mockOnboardingStatus = vi.fn<
  () => {
    loading: boolean;
    status: { onboarding_complete: boolean } | null;
    error: string | null;
    needsSetup: boolean;
  }
>();

vi.mock("@/pages/setup/hooks/use-onboarding-status", () => ({
  useOnboardingStatus: () => mockOnboardingStatus(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { changeLanguage: vi.fn() },
  }),
  initReactI18next: { type: "3rdParty", init: vi.fn() },
}));

// Mock auth store — connected state
let mockConnected = true;
vi.mock("@/stores/use-auth-store", () => ({
  useAuthStore: (selector: (s: { connected: boolean }) => boolean) =>
    selector({ connected: mockConnected }),
}));

function renderWithRouter(children: React.ReactNode) {
  return render(<MemoryRouter>{children}</MemoryRouter>);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("RequireSetup", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockConnected = true;
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should show loader while onboarding status is loading", () => {
    mockOnboardingStatus.mockReturnValue({
      loading: true,
      status: null,
      error: null,
      needsSetup: false,
    });

    const { container } = renderWithRouter(
      <RequireSetup>
        <div data-testid="child-content">Dashboard</div>
      </RequireSetup>,
    );

    // Loader spinner should be visible
    const spinner = container.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();

    // Children should NOT be rendered
    expect(screen.queryByTestId("child-content")).not.toBeInTheDocument();
  });

  it("should render children when onboarding_complete is true", () => {
    mockOnboardingStatus.mockReturnValue({
      loading: false,
      status: { onboarding_complete: true },
      error: null,
      needsSetup: false,
    });

    renderWithRouter(
      <RequireSetup>
        <div data-testid="child-content">Dashboard</div>
      </RequireSetup>,
    );

    expect(screen.getByTestId("child-content")).toBeInTheDocument();
  });

  it("should redirect to /setup when onboarding_complete is false", () => {
    mockOnboardingStatus.mockReturnValue({
      loading: false,
      status: { onboarding_complete: false },
      error: null,
      needsSetup: true,
    });

    renderWithRouter(
      <RequireSetup>
        <div data-testid="child-content">Dashboard</div>
      </RequireSetup>,
    );

    expect(mockNavigate).toHaveBeenCalledWith("/setup", { replace: true });
    expect(screen.queryByTestId("child-content")).not.toBeInTheDocument();
  });

  it("should show disconnected overlay after timeout when WS is disconnected", async () => {
    mockConnected = false;
    mockOnboardingStatus.mockReturnValue({
      loading: true,
      status: null,
      error: null,
      needsSetup: false,
    });

    renderWithRouter(
      <RequireSetup>
        <div data-testid="child-content">Dashboard</div>
      </RequireSetup>,
    );

    // Before timeout — loader should be visible, not disconnected
    expect(screen.queryByText("serverUnreachable")).not.toBeInTheDocument();

    // Advance past CONNECTION_TIMEOUT_MS (3000ms)
    act(() => {
      vi.advanceTimersByTime(3100);
    });

    // Disconnected overlay should be visible
    expect(screen.getByText("serverUnreachable")).toBeInTheDocument();
    expect(screen.queryByTestId("child-content")).not.toBeInTheDocument();
  });

  it("should clear timeout when WS reconnects", () => {
    mockConnected = false;
    mockOnboardingStatus.mockReturnValue({
      loading: false,
      status: { onboarding_complete: true },
      error: null,
      needsSetup: false,
    });

    const { rerender } = renderWithRouter(
      <RequireSetup>
        <div data-testid="child-content">Dashboard</div>
      </RequireSetup>,
    );

    // Reconnect
    mockConnected = true;
    rerender(
      <MemoryRouter>
        <RequireSetup>
          <div data-testid="child-content">Dashboard</div>
        </RequireSetup>
      </MemoryRouter>,
    );

    // Advance past timeout — should NOT show disconnected because connected is true now
    act(() => {
      vi.advanceTimersByTime(3100);
    });

    expect(screen.queryByText("serverUnreachable")).not.toBeInTheDocument();
  });

  it("should pass through when status is null but not loading (edge case)", () => {
    mockOnboardingStatus.mockReturnValue({
      loading: false,
      status: null,
      error: "some error",
      needsSetup: false,
    });

    renderWithRouter(
      <RequireSetup>
        <div data-testid="child-content">Dashboard</div>
      </RequireSetup>,
    );

    // needsSetup is false, loading is false → children should render
    expect(screen.getByTestId("child-content")).toBeInTheDocument();
  });
});
