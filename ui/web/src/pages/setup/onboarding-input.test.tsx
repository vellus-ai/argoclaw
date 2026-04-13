import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OnboardingInput } from "./onboarding-input";
import type { QuickReply, InputFieldSpec } from "./hooks/use-onboarding-engine";

describe("OnboardingInput", () => {
  const quickReplies: QuickReply[] = [
    { label: "Yes", value: "yes" },
    { label: "No", value: "no" },
  ];

  it("should render QuickReplyBar when quickReplies are provided", () => {
    render(
      <OnboardingInput
        quickReplies={quickReplies}
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    expect(screen.getByRole("button", { name: "Yes" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "No" })).toBeInTheDocument();
  });

  it("should render text input when inputField.type is text", () => {
    const inputField: InputFieldSpec = {
      type: "text",
      placeholder: "Enter name",
    };

    render(
      <OnboardingInput
        inputField={inputField}
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    expect(screen.getByPlaceholderText("Enter name")).toBeInTheDocument();
  });

  it("should render password input when inputField.type is password", () => {
    const inputField: InputFieldSpec = {
      type: "password",
      placeholder: "sk-...",
    };

    render(
      <OnboardingInput
        inputField={inputField}
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    const passwordInput = document.getElementById("password-field-input");
    expect(passwordInput).toBeInTheDocument();
    expect(passwordInput).toHaveAttribute("type", "password");
  });

  it("should render ColorPickerInline when inputField.type is color", () => {
    const inputField: InputFieldSpec = {
      type: "color",
    };

    render(
      <OnboardingInput
        inputField={inputField}
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    expect(screen.getByTestId("color-preview")).toBeInTheDocument();
  });

  it("should call onReply when a quick reply is selected", async () => {
    const user = userEvent.setup();
    const onReply = vi.fn();

    render(
      <OnboardingInput
        quickReplies={quickReplies}
        onReply={onReply}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Yes" }));
    expect(onReply).toHaveBeenCalledWith("yes");
  });

  it("should NOT render file attach button", () => {
    render(
      <OnboardingInput
        quickReplies={quickReplies}
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    expect(screen.queryByLabelText(/attach/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /attach/i })).not.toBeInTheDocument();
  });

  it("should NOT render abort button", () => {
    render(
      <OnboardingInput
        quickReplies={quickReplies}
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    expect(screen.queryByLabelText(/abort/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /abort|cancel|stop/i })).not.toBeInTheDocument();
  });

  it("should render nothing when no quickReplies and no inputField", () => {
    const { container } = render(
      <OnboardingInput
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    // Should be empty or just a wrapper
    expect(container.textContent).toBe("");
  });

  it("should prefer quickReplies over inputField when both provided", () => {
    const inputField: InputFieldSpec = { type: "text", placeholder: "Name" };

    render(
      <OnboardingInput
        quickReplies={quickReplies}
        inputField={inputField}
        onReply={vi.fn()}
        onInput={vi.fn()}
        disabled={false}
      />,
    );

    expect(screen.getByRole("button", { name: "Yes" })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("Name")).not.toBeInTheDocument();
  });
});
