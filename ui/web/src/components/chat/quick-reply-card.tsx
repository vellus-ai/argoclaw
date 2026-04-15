import { type KeyboardEvent } from "react";

interface QuickReplyCardProps {
  title: string;
  description?: string;
  icon?: string;
  value: string;
  variant?: "default" | "skip" | "selected";
  onClick: (value: string) => void;
}

export function QuickReplyCard({
  title,
  description,
  icon,
  value,
  variant = "default",
  onClick,
}: QuickReplyCardProps) {
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onClick(value);
    }
  };

  const variantClasses = {
    default: "border-primary hover:bg-accent/50",
    skip: "border-muted text-muted-foreground hover:bg-muted/50",
    selected: "bg-accent border-primary",
  };

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={title}
      onClick={() => onClick(value)}
      onKeyDown={handleKeyDown}
      className={`max-w-[85%] cursor-pointer rounded-xl border px-4 py-3 transition-colors animate-in fade-in slide-in-from-bottom-2 duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${variantClasses[variant]}`}
    >
      <div className="flex items-center gap-2">
        {icon && <span className="text-base">{icon}</span>}
        <span className="text-sm font-medium">{title}</span>
      </div>
      {description && (
        <p className="mt-1 text-xs text-muted-foreground">{description}</p>
      )}
    </div>
  );
}
