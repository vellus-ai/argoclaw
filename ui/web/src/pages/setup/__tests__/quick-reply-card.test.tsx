import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuickReplyCard } from "../quick-reply-card";

describe("QuickReplyCard", () => {
  it("should render label text", () => {
    render(
      <QuickReplyCard label="OpenRouter" onClick={vi.fn()} />,
    );
    expect(screen.getByText("OpenRouter")).toBeInTheDocument();
  });

  it("should render description when provided", () => {
    render(
      <QuickReplyCard
        label="OpenRouter"
        description="Access to 100+ models"
        onClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Access to 100+ models")).toBeInTheDocument();
  });

  it("should not render description element when not provided", () => {
    const { container } = render(
      <QuickReplyCard label="OpenRouter" onClick={vi.fn()} />,
    );
    const spans = container.querySelectorAll("span");
    // Only the label span, no description span
    expect(spans).toHaveLength(1);
  });

  it("should call onClick when clicked", async () => {
    const onClick = vi.fn();
    render(
      <QuickReplyCard label="OpenRouter" onClick={onClick} />,
    );
    await userEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("should be disabled when disabled prop is true", () => {
    render(
      <QuickReplyCard label="OpenRouter" disabled onClick={vi.fn()} />,
    );
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("should apply skip variant styling", () => {
    render(
      <QuickReplyCard
        label="Skip for now"
        variant="skip"
        description="Configure later"
        onClick={vi.fn()}
      />,
    );
    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("data-variant", "skip");
  });

  it("should apply default variant styling by default", () => {
    render(
      <QuickReplyCard label="OpenRouter" onClick={vi.fn()} />,
    );
    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("data-variant", "default");
  });
});
