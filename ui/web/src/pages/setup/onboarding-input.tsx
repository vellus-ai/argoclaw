import { useState } from "react";
import { cn } from "@/lib/utils";
import { QuickReplyBar } from "./quick-reply-bar";
import { ColorPickerInline } from "./color-picker-inline";
import { PasswordFieldInline } from "./password-field-inline";
import type { QuickReply, InputFieldSpec } from "./hooks/use-onboarding-engine";

interface OnboardingInputProps {
  quickReplies?: QuickReply[];
  inputField?: InputFieldSpec;
  onReply: (value: string) => void;
  onInput: (field: string, value: string) => void;
  disabled: boolean;
}

function TextInputInline({
  placeholder,
  onSubmit,
  disabled,
}: {
  placeholder?: string;
  onSubmit: (value: string) => void;
  disabled: boolean;
}) {
  const [value, setValue] = useState("");

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = value.trim();
    if (trimmed.length === 0) return;
    onSubmit(trimmed);
    setValue("");
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="flex items-center gap-2 px-4 py-2"
    >
      <label htmlFor="onboarding-text-input" className="sr-only">
        Input
      </label>
      <input
        id="onboarding-text-input"
        type="text"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete="off"
        className={cn(
          "flex-1 rounded-md border border-input bg-transparent px-3 py-2 text-base shadow-xs md:text-sm",
          "placeholder:text-muted-foreground",
          "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
          "disabled:pointer-events-none disabled:opacity-50",
        )}
      />
      <button
        type="submit"
        disabled={disabled || value.trim().length === 0}
        aria-label="Send"
        className={cn(
          "inline-flex shrink-0 items-center justify-center rounded-md px-4 py-2 text-base font-medium md:text-sm",
          "bg-primary text-primary-foreground hover:bg-primary/90",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          "disabled:pointer-events-none disabled:opacity-50",
        )}
      >
        Send
      </button>
    </form>
  );
}

export function OnboardingInput({
  quickReplies,
  inputField,
  onReply,
  onInput,
  disabled,
}: OnboardingInputProps) {
  // Quick replies take precedence
  if (quickReplies && quickReplies.length > 0) {
    return <QuickReplyBar replies={quickReplies} onSelect={onReply} disabled={disabled} />;
  }

  if (!inputField) return null;

  switch (inputField.type) {
    case "text":
      return (
        <TextInputInline
          placeholder={inputField.placeholder}
          onSubmit={(value) => onInput("text", value)}
          disabled={disabled}
        />
      );

    case "password":
      return (
        <PasswordFieldInline
          placeholder={inputField.placeholder}
          onSubmit={(value) => onInput("apiKey", value)}
        />
      );

    case "color":
      return (
        <ColorPickerInline
          onSelect={(color) => onInput("primaryColor", color)}
        />
      );

    default:
      return null;
  }
}
