import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuickReplyBar } from "./quick-reply-bar";
import type { QuickReply } from "./hooks/use-onboarding-engine";

describe("QuickReplyBar", () => {
  const defaultReplies: QuickReply[] = [
    { label: "Option A", value: "a" },
    { label: "Option B", value: "b" },
  ];

  it("should render all quick reply buttons", () => {
    render(
      <QuickReplyBar replies={defaultReplies} onSelect={vi.fn()} disabled={false} />,
    );

    expect(screen.getByRole("button", { name: "Option A" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Option B" })).toBeInTheDocument();
  });

  it("should call onSelect with the reply value when clicked", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();

    render(
      <QuickReplyBar replies={defaultReplies} onSelect={onSelect} disabled={false} />,
    );

    await user.click(screen.getByRole("button", { name: "Option A" }));
    expect(onSelect).toHaveBeenCalledWith("a");
  });

  it("should disable all buttons when disabled prop is true", () => {
    render(
      <QuickReplyBar replies={defaultReplies} onSelect={vi.fn()} disabled={true} />,
    );

    for (const btn of screen.getAllByRole("button")) {
      expect(btn).toBeDisabled();
    }
  });

  it("should not call onSelect when disabled", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();

    render(
      <QuickReplyBar replies={defaultReplies} onSelect={onSelect} disabled={true} />,
    );

    await user.click(screen.getByRole("button", { name: "Option A" }));
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("should apply muted styling for skip variant", () => {
    const replies: QuickReply[] = [
      { label: "Skip", value: "skip", variant: "skip" },
    ];

    render(
      <QuickReplyBar replies={replies} onSelect={vi.fn()} disabled={false} />,
    );

    const btn = screen.getByRole("button", { name: "Skip" });
    expect(btn).toBeInTheDocument();
    // Skip variant uses ghost/outline styling
    expect(btn.dataset.variant).toBe("skip");
  });

  it("should have role=group on the container", () => {
    render(
      <QuickReplyBar replies={defaultReplies} onSelect={vi.fn()} disabled={false} />,
    );

    expect(screen.getByRole("group")).toBeInTheDocument();
  });

  it("should have 44px minimum touch target height", () => {
    render(
      <QuickReplyBar replies={defaultReplies} onSelect={vi.fn()} disabled={false} />,
    );

    const group = screen.getByRole("group");
    // The container should have min-h-[44px] class
    expect(group.className).toContain("min-h-[44px]");
  });

  it("should render nothing when replies array is empty", () => {
    const { container } = render(
      <QuickReplyBar replies={[]} onSelect={vi.fn()} disabled={false} />,
    );

    expect(container.firstChild).toBeNull();
  });
});
