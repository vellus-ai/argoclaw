import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { axe } from "vitest-axe";
import { Input } from "../input";

describe("Input", () => {
  it("renders with default props", () => {
    render(<Input aria-label="test input" />);
    const input = screen.getByRole("textbox");
    expect(input).toBeInTheDocument();
    expect(input).toHaveAttribute("data-slot", "input");
  });

  it("includes text-foreground class for explicit text color", () => {
    render(<Input aria-label="test input" />);
    const input = screen.getByRole("textbox");
    // Must have standalone text-foreground (not just file:text-foreground)
    const classes = input.className.split(/\s+/);
    expect(classes).toContain("text-foreground");
  });

  it("propagates custom className via cn()", () => {
    render(<Input aria-label="test input" className="custom-class" />);
    const input = screen.getByRole("textbox");
    expect(input.className).toContain("custom-class");
  });

  it("propagates ref (React 19)", () => {
    let inputRef: HTMLInputElement | null = null;
    render(
      <Input
        aria-label="test input"
        ref={(el) => {
          inputRef = el;
        }}
      />
    );
    expect(inputRef).toBeInstanceOf(HTMLInputElement);
  });

  it("passes type prop to native input", () => {
    render(<Input aria-label="email" type="email" />);
    const input = screen.getByRole("textbox");
    expect(input).toHaveAttribute("type", "email");
  });

  it("includes flex class for alignment", () => {
    render(<Input aria-label="test input" />);
    const input = screen.getByRole("textbox");
    // Must have standalone "flex" class (not file:inline-flex or similar)
    const classes = input.className.split(/\s+/);
    expect(classes).toContain("flex");
  });

  it("passes accessibility check with associated label", async () => {
    const { container } = render(
      <div>
        <label htmlFor="email-input">Email</label>
        <Input id="email-input" type="email" />
      </div>
    );
    const results = await axe(container);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (expect(results) as any).toHaveNoViolations();
  });
});
