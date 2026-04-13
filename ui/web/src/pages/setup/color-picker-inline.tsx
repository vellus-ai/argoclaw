import { useState } from "react";
import { cn } from "@/lib/utils";

interface ColorPickerInlineProps {
  defaultColor?: string;
  onSelect: (color: string) => void;
}

const PRESET_COLORS = [
  "#1E40AF", // Navy blue (default)
  "#DC2626", // Red
  "#059669", // Emerald
  "#7C3AED", // Violet
  "#EA580C", // Orange
  "#0891B2", // Cyan
  "#BE185D", // Pink
  "#4338CA", // Indigo
] as const;

const HEX_REGEX = /^#[0-9A-Fa-f]{3}([0-9A-Fa-f]{3})?$/;

export function ColorPickerInline({ defaultColor = "#1E40AF", onSelect }: ColorPickerInlineProps) {
  const [selected, setSelected] = useState(defaultColor);

  function handleSwatchClick(color: string) {
    setSelected(color);
    onSelect(color);
  }

  function handleNativeChange(e: React.ChangeEvent<HTMLInputElement>) {
    const value = e.target.value;
    if (HEX_REGEX.test(value)) {
      setSelected(value);
    }
  }

  function handleConfirm() {
    onSelect(selected);
  }

  return (
    <div className="flex flex-col gap-3 px-4 py-2">
      {/* Preview */}
      <div className="flex items-center gap-3">
        <div
          data-testid="color-preview"
          className="h-10 w-10 shrink-0 rounded-lg border border-border"
          style={{ backgroundColor: selected }}
        />
        <span className="text-base font-mono text-muted-foreground md:text-sm">
          {selected}
        </span>
      </div>

      {/* Preset swatches */}
      <div className="flex flex-wrap gap-2">
        {PRESET_COLORS.map((color) => (
          <button
            key={color}
            type="button"
            aria-label={`Select color ${color}`}
            onClick={() => handleSwatchClick(color)}
            className={cn(
              "h-8 w-8 rounded-full border-2 transition-transform hover:scale-110",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
              selected === color ? "border-foreground scale-110" : "border-transparent",
            )}
            style={{ backgroundColor: color }}
          />
        ))}
      </div>

      {/* Native color input + confirm */}
      <div className="flex items-center gap-3">
        <label htmlFor="custom-color-input" className="sr-only">
          Custom color
        </label>
        <input
          id="custom-color-input"
          type="color"
          aria-label="Custom color"
          value={selected}
          onChange={handleNativeChange}
          className="h-9 w-9 cursor-pointer rounded border border-border bg-transparent p-0.5"
        />
        <button
          type="button"
          onClick={handleConfirm}
          aria-label="Confirm color selection"
          className={cn(
            "inline-flex items-center justify-center rounded-md px-4 py-2 text-base font-medium md:text-sm",
            "bg-primary text-primary-foreground hover:bg-primary/90",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        >
          Select
        </button>
      </div>
    </div>
  );
}
