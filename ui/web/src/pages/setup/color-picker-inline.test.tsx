import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ColorPickerInline } from "./color-picker-inline";

describe("ColorPickerInline", () => {
  it("should display the default color #1E40AF", () => {
    render(<ColorPickerInline onSelect={vi.fn()} />);

    // The selected color preview should show the default
    const preview = screen.getByTestId("color-preview");
    expect(preview).toHaveStyle({ backgroundColor: "#1E40AF" });
  });

  it("should display a custom default color when provided", () => {
    render(<ColorPickerInline defaultColor="#FF0000" onSelect={vi.fn()} />);

    const preview = screen.getByTestId("color-preview");
    expect(preview).toHaveStyle({ backgroundColor: "#FF0000" });
  });

  it("should call onSelect when a preset swatch is clicked", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();

    render(<ColorPickerInline onSelect={onSelect} />);

    // Click the first preset swatch button
    const swatches = screen.getAllByRole("button", { name: /select color/i });
    expect(swatches.length).toBeGreaterThanOrEqual(6);

    await user.click(swatches[0]!);
    expect(onSelect).toHaveBeenCalledTimes(1);
    // The value should be a valid hex color
    expect(onSelect.mock.calls[0]![0]).toMatch(/^#[0-9A-Fa-f]{6}$/);
  });

  it("should have a confirm button", () => {
    render(<ColorPickerInline onSelect={vi.fn()} />);

    expect(screen.getByRole("button", { name: /confirm color selection/i })).toBeInTheDocument();
  });

  it("should call onSelect with current color when confirm is clicked", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();

    render(<ColorPickerInline defaultColor="#1E40AF" onSelect={onSelect} />);

    await user.click(screen.getByRole("button", { name: /confirm color selection/i }));
    expect(onSelect).toHaveBeenCalledWith("#1E40AF");
  });

  it("should have a native color input", () => {
    render(<ColorPickerInline onSelect={vi.fn()} />);

    const colorInput = screen.getByLabelText(/custom color/i);
    expect(colorInput).toBeInTheDocument();
    expect(colorInput).toHaveAttribute("type", "color");
  });
});
