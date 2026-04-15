import { QuickReplyCard } from "./quick-reply-card";
import type { QuickReply } from "./hooks/use-onboarding-engine";

interface QuickReplyBarProps {
  replies: QuickReply[];
  onSelect: (value: string) => void;
  disabled: boolean;
}

export function QuickReplyBar({ replies, onSelect, disabled }: QuickReplyBarProps) {
  if (replies.length === 0) return null;

  return (
    <div
      role="group"
      aria-label="Quick replies"
      className="min-h-[44px] flex flex-col gap-2 px-4 py-2"
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
