import { cn } from "@/lib/utils";
import { QuickReplyCard } from "./quick-reply-card";
import type { QuickReply } from "./hooks/use-onboarding-engine";

interface QuickReplyBarProps {
  replies: QuickReply[];
  onSelect: (value: string) => void;
  disabled: boolean;
}

export function QuickReplyBar({ replies, onSelect, disabled }: QuickReplyBarProps) {
  if (replies.length === 0) return null;

  // Use card layout when any reply has a description (provider/channel steps)
  const hasDescriptions = replies.some((r) => r.description);

  if (hasDescriptions) {
    return (
      <div
        role="group"
        aria-label="Quick replies"
        className="flex flex-col gap-2 px-4 py-2"
      >
        {replies.map((reply) => (
          <QuickReplyCard
            key={reply.value}
            label={reply.label}
            description={reply.description}
            variant={reply.variant}
            disabled={disabled}
            onClick={() => onSelect(reply.value)}
          />
        ))}
      </div>
    );
  }

  return (
    <div
      role="group"
      aria-label="Quick replies"
      className="min-h-[44px] flex gap-2 overflow-x-auto px-4 py-2"
    >
      {replies.map((reply) => (
        <button
          key={reply.value}
          type="button"
          disabled={disabled}
          data-variant={reply.variant ?? "default"}
          onClick={() => onSelect(reply.value)}
          aria-label={reply.label}
          className={cn(
            "inline-flex shrink-0 items-center justify-center rounded-full px-4 py-2 text-base font-medium transition-colors md:text-sm",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            "disabled:pointer-events-none disabled:opacity-50",
            reply.variant === "skip"
              ? "border border-muted-foreground/30 bg-transparent text-muted-foreground hover:bg-muted"
              : "bg-primary text-primary-foreground hover:bg-primary/90",
          )}
        >
          {reply.label}
        </button>
      ))}
    </div>
  );
}
