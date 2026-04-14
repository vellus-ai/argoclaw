import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { axe } from "vitest-axe";
import { LoadingSpinner } from "../loading-spinner";

describe("LoadingSpinner", () => {
  it("renders with role='status'", () => {
    render(<LoadingSpinner />);
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("uses default aria-label 'Carregando…'", () => {
    render(<LoadingSpinner />);
    expect(screen.getByRole("status")).toHaveAttribute(
      "aria-label",
      "Carregando…"
    );
  });

  it("accepts custom aria-label", () => {
    render(<LoadingSpinner aria-label="Loading agents" />);
    expect(screen.getByRole("status")).toHaveAttribute(
      "aria-label",
      "Loading agents"
    );
  });

  it("renders sm size variant", () => {
    const { container } = render(<LoadingSpinner size="sm" />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders md size variant (default)", () => {
    const { container } = render(<LoadingSpinner />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders lg size variant", () => {
    const { container } = render(<LoadingSpinner size="lg" />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("propagates className", () => {
    render(<LoadingSpinner className="custom-class" />);
    const el = screen.getByRole("status");
    expect(el.className).toContain("custom-class");
  });

  it("passes accessibility check (vitest-axe)", async () => {
    const { container } = render(<LoadingSpinner />);
    const results = await axe(container);
    expect(results).toHaveNoViolations();
  });
});
