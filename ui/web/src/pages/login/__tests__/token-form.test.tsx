import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { changeLanguage: vi.fn() },
  }),
  initReactI18next: { type: "3rdParty", init: vi.fn() },
}));

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

import { TokenForm } from "@/pages/login/token-form";

describe("TokenForm", () => {
  const mockOnSubmit = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders User ID and Gateway Token fields with labels", () => {
    render(<TokenForm onSubmit={mockOnSubmit} />);

    expect(screen.getByLabelText("token.userId")).toBeInTheDocument();
    expect(screen.getByLabelText("token.gatewayToken")).toBeInTheDocument();
  });

  it("submit button is disabled when fields are empty", () => {
    render(<TokenForm onSubmit={mockOnSubmit} />);

    const button = screen.getByRole("button", { name: "token.connect" });
    expect(button).toBeDisabled();
  });

  it("calls onSubmit with userId and token on success", async () => {
    const user = userEvent.setup();

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
    });

    render(<TokenForm onSubmit={mockOnSubmit} />);

    await user.type(screen.getByLabelText("token.userId"), "  user-123  ");
    await user.type(screen.getByLabelText("token.gatewayToken"), "  gw-token-abc  ");

    const button = screen.getByRole("button", { name: "token.connect" });
    expect(button).toBeEnabled();

    await user.click(button);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith("/v1/agents", {
        headers: {
          Authorization: "Bearer gw-token-abc",
          "X-ArgoClaw-User-Id": "user-123",
        },
      });
    });

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith("user-123", "gw-token-abc");
    });
  });

  it("shows error on 401 response", async () => {
    const user = userEvent.setup();

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
    });

    render(<TokenForm onSubmit={mockOnSubmit} />);

    await user.type(screen.getByLabelText("token.userId"), "user-123");
    await user.type(screen.getByLabelText("token.gatewayToken"), "bad-token");

    await user.click(screen.getByRole("button", { name: "token.connect" }));

    await waitFor(() => {
      expect(screen.getByText("token.errorInvalidCredentials")).toBeInTheDocument();
    });

    expect(mockOnSubmit).not.toHaveBeenCalled();
  });
});
