import { render, screen, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, afterEach } from "vitest";
import fc from "fast-check";
import { QuickReplyCard } from "../quick-reply-card";

afterEach(cleanup);

// --- Helpers ---

const defaultProps = {
  title: "Let's get started",
  value: "start",
  onClick: vi.fn(),
};

// --- RTL Tests (Task 15.1) ---

describe("QuickReplyCard", () => {
  describe("Rendering", () => {
    it("renders with title", () => {
      render(<QuickReplyCard {...defaultProps} />);
      expect(screen.getByText("Let's get started")).toBeInTheDocument();
    });

    it("renders with title and optional description", () => {
      render(
        <QuickReplyCard
          {...defaultProps}
          description="This will begin the onboarding"
        />,
      );
      expect(screen.getByText("Let's get started")).toBeInTheDocument();
      expect(
        screen.getByText("This will begin the onboarding"),
      ).toBeInTheDocument();
    });

    it("does not render description paragraph when not provided", () => {
      const { container } = render(<QuickReplyCard {...defaultProps} />);
      expect(container.querySelector("p")).toBeNull();
    });

    it("renders icon when provided", () => {
      render(<QuickReplyCard {...defaultProps} icon="🚀" />);
      expect(screen.getByText("🚀")).toBeInTheDocument();
    });
  });

  describe("Click handler", () => {
    it("fires callback with correct value on click", async () => {
      const onClick = vi.fn();
      const user = userEvent.setup();
      render(<QuickReplyCard {...defaultProps} onClick={onClick} />);

      await user.click(screen.getByRole("button"));
      expect(onClick).toHaveBeenCalledTimes(1);
      expect(onClick).toHaveBeenCalledWith("start");
    });
  });

  describe("Keyboard events", () => {
    it("Enter key activates the card", async () => {
      const onClick = vi.fn();
      const user = userEvent.setup();
      render(<QuickReplyCard {...defaultProps} onClick={onClick} />);

      const card = screen.getByRole("button");
      card.focus();
      await user.keyboard("{Enter}");
      expect(onClick).toHaveBeenCalledTimes(1);
      expect(onClick).toHaveBeenCalledWith("start");
    });

    it("Space key activates the card", async () => {
      const onClick = vi.fn();
      const user = userEvent.setup();
      render(<QuickReplyCard {...defaultProps} onClick={onClick} />);

      const card = screen.getByRole("button");
      card.focus();
      await user.keyboard(" ");
      expect(onClick).toHaveBeenCalledTimes(1);
      expect(onClick).toHaveBeenCalledWith("start");
    });
  });

  describe("Accessibility", () => {
    it("has role='button'", () => {
      render(<QuickReplyCard {...defaultProps} />);
      expect(screen.getByRole("button")).toBeInTheDocument();
    });

    it("has tabIndex={0}", () => {
      render(<QuickReplyCard {...defaultProps} />);
      expect(screen.getByRole("button")).toHaveAttribute("tabindex", "0");
    });

    it("has aria-label matching title", () => {
      render(<QuickReplyCard {...defaultProps} />);
      expect(screen.getByRole("button")).toHaveAttribute(
        "aria-label",
        "Let's get started",
      );
    });

    it("has focus-visible:ring classes for focus ring", () => {
      const { container } = render(<QuickReplyCard {...defaultProps} />);
      const card = container.firstElementChild as HTMLElement;
      expect(card.className).toContain("focus-visible:ring");
    });
  });

  describe("Variants", () => {
    it("variant 'default' has border-primary class", () => {
      const { container } = render(
        <QuickReplyCard {...defaultProps} variant="default" />,
      );
      const card = container.firstElementChild as HTMLElement;
      expect(card.className).toContain("border-primary");
    });

    it("variant 'skip' has text-muted-foreground class", () => {
      const { container } = render(
        <QuickReplyCard {...defaultProps} variant="skip" />,
      );
      const card = container.firstElementChild as HTMLElement;
      expect(card.className).toContain("text-muted-foreground");
    });

    it("variant 'selected' has bg-accent class", () => {
      const { container } = render(
        <QuickReplyCard {...defaultProps} variant="selected" />,
      );
      const card = container.firstElementChild as HTMLElement;
      expect(card.className).toContain("bg-accent");
    });
  });

  // --- PBT (fast-check) ---

  describe("Property-Based Testing", () => {
    it("renders without crash for any valid combination of props", () => {
      fc.assert(
        fc.property(
          fc.string({ minLength: 1 }),
          fc.option(fc.string(), { nil: undefined }),
          fc.constantFrom("default" as const, "skip" as const, "selected" as const),
          (title, description, variant) => {
            cleanup();
            const onClick = vi.fn();
            const { container } = render(
              <QuickReplyCard
                title={title}
                description={description}
                value="test-value"
                variant={variant}
                onClick={onClick}
              />,
            );
            // Card renders and contains the title text
            expect(container.textContent).toContain(title);
            // Has role="button"
            expect(screen.getByRole("button")).toBeInTheDocument();
          },
        ),
        { numRuns: 100 },
      );
    });
  });
});
