import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PasswordFieldInline } from "./password-field-inline";

describe("PasswordFieldInline", () => {
  it("should render an input with type=password", () => {
    render(<PasswordFieldInline onSubmit={vi.fn()} />);

    // password inputs don't have the textbox role; query by id
    const passwordInput = document.getElementById("password-field-input");
    expect(passwordInput).toBeInTheDocument();
    expect(passwordInput).toHaveAttribute("type", "password");
  });

  it("should show custom placeholder when provided", () => {
    render(<PasswordFieldInline placeholder="sk-..." onSubmit={vi.fn()} />);

    expect(screen.getByPlaceholderText("sk-...")).toBeInTheDocument();
  });

  it("should call onSubmit with the input value on form submit", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();

    render(<PasswordFieldInline onSubmit={onSubmit} />);

    const input = document.getElementById("password-field-input") as HTMLInputElement;
    await user.type(input, "sk-test-key-1234");
    await user.click(screen.getByRole("button", { name: /submit api key/i }));

    expect(onSubmit).toHaveBeenCalledWith("sk-test-key-1234");
  });

  it("should show masked confirmation after submit", async () => {
    const user = userEvent.setup();

    render(<PasswordFieldInline onSubmit={vi.fn()} />);

    const input = document.getElementById("password-field-input") as HTMLInputElement;
    await user.type(input, "sk-test-key-1234");
    await user.click(screen.getByRole("button", { name: /submit api key/i }));

    // Should show masked value with last 4 chars
    expect(screen.getByText(/1234/)).toBeInTheDocument();
    // Input should no longer be in the DOM
    expect(document.getElementById("password-field-input")).not.toBeInTheDocument();
  });

  it("should not have the plain value in the DOM after submit", async () => {
    const user = userEvent.setup();

    const { container } = render(<PasswordFieldInline onSubmit={vi.fn()} />);

    const input = document.getElementById("password-field-input") as HTMLInputElement;
    await user.type(input, "sk-test-key-1234");
    await user.click(screen.getByRole("button", { name: /submit api key/i }));

    // The full value should NOT appear anywhere in the DOM
    expect(container.textContent).not.toContain("sk-test-key-1234");
  });

  it("should not submit when input is empty", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();

    render(<PasswordFieldInline onSubmit={onSubmit} />);

    await user.click(screen.getByRole("button", { name: /submit api key/i }));

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("should submit on Enter key press", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();

    render(<PasswordFieldInline onSubmit={onSubmit} />);

    const input = document.getElementById("password-field-input") as HTMLInputElement;
    await user.type(input, "sk-my-key-abcd{Enter}");

    expect(onSubmit).toHaveBeenCalledWith("sk-my-key-abcd");
  });
});
