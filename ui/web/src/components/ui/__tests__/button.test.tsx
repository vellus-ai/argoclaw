import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { axe } from "vitest-axe";
import { Button } from "../button";

describe("Button", () => {
  it("renders with default props", () => {
    render(<Button>Click</Button>);
    const button = screen.getByRole("button", { name: "Click" });
    expect(button).toBeInTheDocument();
    expect(button).toHaveAttribute("data-slot", "button");
    expect(button).toHaveAttribute("data-variant", "default");
    expect(button).toHaveAttribute("data-size", "default");
  });

  it.each([
    "default",
    "destructive",
    "outline",
    "secondary",
    "ghost",
    "link",
  ] as const)("renders variant=%s with correct data-variant", (variant) => {
    render(<Button variant={variant}>{variant}</Button>);
    const button = screen.getByRole("button", { name: variant });
    expect(button).toHaveAttribute("data-variant", variant);
  });

  it.each([
    "default",
    "xs",
    "sm",
    "lg",
    "icon",
    "icon-xs",
    "icon-sm",
    "icon-lg",
  ] as const)("renders size=%s with correct data-size", (size) => {
    render(
      <Button size={size} aria-label={`size-${size}`}>
        X
      </Button>
    );
    const button = screen.getByRole("button", { name: `size-${size}` });
    expect(button).toHaveAttribute("data-size", size);
  });

  it("propagates className", () => {
    render(<Button className="custom">Click</Button>);
    const button = screen.getByRole("button", { name: "Click" });
    expect(button.className).toContain("custom");
  });

  it("renders as child with asChild", () => {
    render(
      <Button asChild>
        <a href="/">Link</a>
      </Button>
    );
    const link = screen.getByRole("link", { name: "Link" });
    expect(link).toHaveAttribute("data-slot", "button");
    expect(link).toHaveAttribute("href", "/");
  });

  it("passes accessibility check", async () => {
    const { container } = render(<Button>Click me</Button>);
    const results = await axe(container);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (expect(results) as any).toHaveNoViolations();
  });
});
