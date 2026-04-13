import { useEffect, useRef } from "react";
import { cn } from "@/lib/utils";
import { ThinkingIndicator } from "@/components/chat/thinking-indicator";
import type { ChatMessageLocal } from "./hooks/use-onboarding-engine";

interface OnboardingChatProps {
  messages: ChatMessageLocal[];
  typing: boolean;
}

function AssistantBubble({ content }: { content: string }) {
  if (!content) return null;

  return (
    <div className="flex gap-3 px-4 py-1.5">
      {/* Imediato avatar */}
      <div
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground text-xs font-bold"
        aria-hidden="true"
      >
        IM
      </div>
      <div className="max-w-[80%] rounded-2xl rounded-tl-sm bg-muted px-4 py-2.5 text-base md:text-sm">
        {content}
      </div>
    </div>
  );
}

function UserBubble({ content }: { content: string }) {
  return (
    <div className="flex justify-end px-4 py-1.5">
      <div className="max-w-[80%] rounded-2xl rounded-tr-sm bg-primary px-4 py-2.5 text-base text-primary-foreground md:text-sm">
        {content}
      </div>
    </div>
  );
}

export function OnboardingChat({ messages, typing }: OnboardingChatProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages.length, typing]);

  return (
    <div
      ref={scrollRef}
      className={cn(
        "flex-1 overflow-y-auto overscroll-contain py-4",
      )}
      role="log"
      aria-label="Chat messages"
      aria-live="polite"
    >
      {messages.map((msg) =>
        msg.role === "assistant" ? (
          <AssistantBubble key={msg.id} content={msg.content} />
        ) : (
          <UserBubble key={msg.id} content={msg.content} />
        ),
      )}

      {typing && (
        <div className="flex gap-3 px-4 py-1.5">
          <div
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground text-xs font-bold"
            aria-hidden="true"
          >
            IM
          </div>
          <div className="rounded-2xl rounded-tl-sm bg-muted px-2 py-1">
            <ThinkingIndicator />
          </div>
        </div>
      )}
    </div>
  );
}
