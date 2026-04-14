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

const mockLogin = vi.fn();
vi.mock("@/api/auth-client", () => ({
  login: (...args: unknown[]) => mockLogin(...args),
  register: vi.fn(),
  AuthApiError: class extends Error {
    status: number;
    constructor(status: number, message = "auth error", _code?: string) {
      super(message);
      this.name = "AuthApiError";
      this.status = status;
    }
  },
}));

import { EmailForm } from "@/pages/login/email-form";
import { AuthApiError } from "@/api/auth-client";

describe("EmailForm", () => {
  const mockOnSuccess = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders email and password fields with labels", () => {
    render(<EmailForm onSuccess={mockOnSuccess} />);

    expect(screen.getByLabelText("email.email")).toBeInTheDocument();
    expect(screen.getByLabelText("email.password")).toBeInTheDocument();
  });

  it("submit button is disabled when fields are empty", () => {
    render(<EmailForm onSuccess={mockOnSuccess} />);

    const button = screen.getByRole("button", { name: "email.signIn" });
    expect(button).toBeDisabled();
  });

  it("calls login on submit with valid credentials", async () => {
    const user = userEvent.setup();

    mockLogin.mockResolvedValueOnce({
      access_token: "at",
      refresh_token: "rt",
      expires_in: 3600,
      user: { id: "u1", email: "test@example.com", display_name: "Test", role: "admin", status: "active" },
    });

    render(<EmailForm onSuccess={mockOnSuccess} />);

    await user.type(screen.getByLabelText("email.email"), "  test@example.com  ");
    await user.type(screen.getByLabelText("email.password"), "MyP@ssw0rd123");

    const button = screen.getByRole("button", { name: "email.signIn" });
    expect(button).toBeEnabled();

    await user.click(button);

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith("test@example.com", "MyP@ssw0rd123");
    });

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalledWith("at", "rt", "u1");
    });
  });

  it("shows error on 401 (invalid credentials)", async () => {
    const user = userEvent.setup();

    mockLogin.mockRejectedValueOnce(new AuthApiError(401, "invalid credentials"));

    render(<EmailForm onSuccess={mockOnSuccess} />);

    await user.type(screen.getByLabelText("email.email"), "test@example.com");
    await user.type(screen.getByLabelText("email.password"), "wrongpassword");

    await user.click(screen.getByRole("button", { name: "email.signIn" }));

    await waitFor(() => {
      expect(screen.getByText("email.errorInvalidCredentials")).toBeInTheDocument();
    });

    expect(mockOnSuccess).not.toHaveBeenCalled();
  });

  it("password autocomplete is current-password in sign-in mode", () => {
    render(<EmailForm onSuccess={mockOnSuccess} />);

    const passwordInput = screen.getByLabelText("email.password");
    expect(passwordInput).toHaveAttribute("autocomplete", "current-password");
  });
});
