import { cn } from "@/lib/utils";

export interface QuickReplyCardProps {
  label: string;
  description?: string;
  variant?: "default" | "skip";
  disabled?: boolean;
  onClick: () => void;
}

/**
 * A card-style quick reply button with optional description text.
 * Used in onboarding for provider and channel selection steps.
 */
export function QuickReplyCard({
  label,
  description,
  variant = "default",
  disabled = false,
  onClick,
}: QuickReplyCardProps) {
  return (
    <button
      type="button"
      disabled={disabled}
      data-variant={variant}
      onClick={onClick}
      className={cn(
        "flex w-full flex-col items-start gap-0.5 rounded-lg px-4 py-3 text-left transition-colors",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        "disabled:pointer-events-none disabled:opacity-50",
        variant === "skip"
          ? "border border-muted-foreground/30 bg-transparent hover:bg-muted"
          : "border border-border bg-card hover:bg-accent",
      )}
    >
      <span
        className={cn(
          "text-base font-medium md:text-sm",
          variant === "skip" ? "text-muted-foreground" : "text-foreground",
        )}
      >
        {label}
      </span>
      {description && (
        <span className="text-xs text-muted-foreground">{description}</span>
      )}
    </button>
  );
}
