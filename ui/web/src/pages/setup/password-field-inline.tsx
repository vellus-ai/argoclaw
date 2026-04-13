import { useState, useRef } from "react";
import { cn } from "@/lib/utils";

interface PasswordFieldInlineProps {
  placeholder?: string;
  onSubmit: (value: string) => void;
}

function maskValue(value: string): string {
  if (value.length <= 4) return value;
  const last4 = value.slice(-4);
  return `${"•".repeat(8)}${last4}`;
}

export function PasswordFieldInline({ placeholder, onSubmit }: PasswordFieldInlineProps) {
  const [submitted, setSubmitted] = useState(false);
  const [maskedDisplay, setMaskedDisplay] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  function handleSubmit(e?: React.FormEvent) {
    e?.preventDefault();
    const value = inputRef.current?.value ?? "";
    if (value.trim().length === 0) return;

    setMaskedDisplay(maskValue(value));
    setSubmitted(true);
    onSubmit(value);

    // Clear the input value from the DOM immediately
    if (inputRef.current) {
      inputRef.current.value = "";
    }
  }

  if (submitted) {
    return (
      <div className="flex items-center gap-2 px-4 py-2">
        <span className="text-base font-mono text-muted-foreground md:text-sm">
          {maskedDisplay}
        </span>
        <span className="text-xs text-green-600" aria-live="polite">
          ✓
        </span>
      </div>
    );
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="flex items-center gap-2 px-4 py-2"
    >
      <label htmlFor="password-field-input" className="sr-only">
        API key
      </label>
      <input
        ref={inputRef}
        id="password-field-input"
        type="password"
        placeholder={placeholder}
        autoComplete="off"
        className={cn(
          "flex-1 rounded-md border border-input bg-transparent px-3 py-2 text-base shadow-xs md:text-sm",
          "placeholder:text-muted-foreground",
          "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
        )}
      />
      <button
        type="submit"
        aria-label="Submit API key"
        className={cn(
          "inline-flex shrink-0 items-center justify-center rounded-md px-4 py-2 text-base font-medium md:text-sm",
          "bg-primary text-primary-foreground hover:bg-primary/90",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        )}
      >
        Submit
      </button>
    </form>
  );
}
