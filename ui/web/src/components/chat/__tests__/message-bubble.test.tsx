import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import fc from "fast-check";
import { MessageBubble } from "../message-bubble";
import type { ChatMessage } from "@/types/chat";
import {
  arbitraryChatMessage,
  arbitraryToolStreamEntry,
} from "@/test/arbitraries/chat";

// Mock child components to isolate MessageBubble visual tests
vi.mock("../message-content", () => ({
  MessageContent: ({ content }: { content: string }) => (
    <div data-testid="message-content">{content}</div>
  ),
}));

vi.mock("../thinking-block", () => ({
  ThinkingBlock: ({ text }: { text: string }) => (
    <div data-testid="thinking-block">{text}</div>
  ),
}));

vi.mock("../tool-call-card", () => ({
  ToolCallCard: ({ entry }: { entry: { name: string } }) => (
    <div data-testid="tool-call-card">{entry.name}</div>
  ),
}));

vi.mock("../block-reply-bubble", () => ({
  BlockReplyBubble: ({ message }: { message: ChatMessage }) => (
    <div data-testid="block-reply-bubble">{message.content}</div>
  ),
}));

vi.mock("../media-gallery", () => ({
  MediaGallery: ({ items }: { items: unknown[] }) => (
    <div data-testid="media-gallery">{items.length} items</div>
  ),
}));

// --- Helpers ---

function makeUserMessage(overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    role: "user",
    content: "Hello world",
    ...overrides,
  };
}

function makeAssistantMessage(
  overrides: Partial<ChatMessage> = {},
): ChatMessage {
  return {
    role: "assistant",
    content: "Hi there",
    ...overrides,
  };
}

function makeToolOnlyMessage(
  overrides: Partial<ChatMessage> = {},
): ChatMessage {
  return {
    role: "assistant",
    content: "",
    toolDetails: [
      {
        toolCallId: "tc-1",
        runId: "r-1",
        name: "search",
        phase: "completed",
        startedAt: Date.now(),
        updatedAt: Date.now(),
      },
    ],
    ...overrides,
  };
}

// --- RTL Example Tests (Task 2.1) ---

describe("MessageBubble", () => {
  describe("User messages", () => {
    it("aligns to the right with flex-row-reverse", () => {
      const { container } = render(
        <MessageBubble message={makeUserMessage()} />,
      );
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper.className).toContain("flex-row-reverse");
    });

    it("applies bg-primary and text-primary-foreground", () => {
      const { container } = render(
        <MessageBubble message={makeUserMessage()} />,
      );
      const bubble = container.querySelector("[class*='bg-primary']");
      expect(bubble).not.toBeNull();
      expect(bubble!.className).toContain("text-primary-foreground");
    });

    it("caps width at max-w-[85%]", () => {
      const { container } = render(
        <MessageBubble message={makeUserMessage()} />,
      );
      const bubble = container.querySelector("[class*='bg-primary']");
      expect(bubble!.className).toContain("max-w-[85%]");
    });

    it("applies asymmetric border-radius rounded-2xl rounded-br-sm", () => {
      const { container } = render(
        <MessageBubble message={makeUserMessage()} />,
      );
      const bubble = container.querySelector("[class*='bg-primary']");
      expect(bubble!.className).toContain("rounded-2xl");
      expect(bubble!.className).toContain("rounded-br-sm");
    });
  });

  describe("Assistant messages", () => {
    it("aligns to the left (no flex-row-reverse)", () => {
      const { container } = render(
        <MessageBubble message={makeAssistantMessage()} />,
      );
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper.className).not.toContain("flex-row-reverse");
    });

    it("applies bg-card and border-border", () => {
      const { container } = render(
        <MessageBubble message={makeAssistantMessage()} />,
      );
      const bubble = container.querySelector("[class*='bg-card']");
      expect(bubble).not.toBeNull();
      expect(bubble!.className).toContain("border-border");
    });

    it("uses flex-1 min-w-0 for full width", () => {
      const { container } = render(
        <MessageBubble message={makeAssistantMessage()} />,
      );
      const bubble = container.querySelector("[class*='bg-card']");
      expect(bubble!.className).toContain("flex-1");
      expect(bubble!.className).toContain("min-w-0");
    });

    it("applies asymmetric border-radius rounded-2xl rounded-bl-sm", () => {
      const { container } = render(
        <MessageBubble message={makeAssistantMessage()} />,
      );
      const bubble = container.querySelector("[class*='bg-card']");
      expect(bubble!.className).toContain("rounded-2xl");
      expect(bubble!.className).toContain("rounded-bl-sm");
    });

    it("does NOT apply shadow-sm", () => {
      const { container } = render(
        <MessageBubble message={makeAssistantMessage()} />,
      );
      const bubble = container.querySelector("[class*='bg-card']");
      expect(bubble!.className).not.toContain("shadow-sm");
    });
  });

  describe("Avatar", () => {
    it("renders Bot icon for assistant with rounded-full border bg-background", () => {
      const { container } = render(
        <MessageBubble message={makeAssistantMessage()} />,
      );
      const avatar = container.querySelector(
        "[class*='rounded-full']",
      ) as HTMLElement;
      expect(avatar).not.toBeNull();
      expect(avatar.className).toContain("border");
      expect(avatar.className).toContain("bg-background");
    });

    it("renders User icon for user messages", () => {
      const { container } = render(
        <MessageBubble message={makeUserMessage()} />,
      );
      const avatar = container.querySelector(
        "[class*='rounded-full']",
      ) as HTMLElement;
      expect(avatar).not.toBeNull();
    });
  });

  describe("MediaGallery", () => {
    it("renders when mediaItems present", () => {
      render(
        <MessageBubble
          message={makeAssistantMessage({
            mediaItems: [
              { path: "/img.png", mimeType: "image/png", kind: "image" },
            ],
          })}
        />,
      );
      expect(screen.getByTestId("media-gallery")).toBeInTheDocument();
    });

    it("does not render when mediaItems absent", () => {
      render(<MessageBubble message={makeAssistantMessage()} />);
      expect(screen.queryByTestId("media-gallery")).not.toBeInTheDocument();
    });
  });

  describe("Timestamp", () => {
    it("renders timestamp in HH:MM format with 10px font", () => {
      const ts = new Date(2026, 0, 15, 14, 30).getTime();
      const { container } = render(
        <MessageBubble message={makeAssistantMessage({ timestamp: ts })} />,
      );
      const timestampEl = container.querySelector("[class*='text-[10px]']");
      expect(timestampEl).not.toBeNull();
      // Should contain the time (format varies by locale, but must have minutes)
      expect(timestampEl!.textContent).toMatch(/\d{1,2}:\d{2}/);
    });
  });

  describe("Tool-only messages", () => {
    it("renders compact without bubble wrapper with bg-muted", () => {
      const { container } = render(
        <MessageBubble message={makeToolOnlyMessage()} />,
      );
      const compact = container.querySelector("[class*='bg-muted']");
      expect(compact).not.toBeNull();
      expect(compact!.className).toContain("divide-y");
      expect(compact!.className).toContain("divide-border");
    });

    it("renders tool call cards", () => {
      render(<MessageBubble message={makeToolOnlyMessage()} />);
      expect(screen.getByTestId("tool-call-card")).toBeInTheDocument();
    });
  });

  describe("Edge cases", () => {
    it("returns null for tool role messages", () => {
      const { container } = render(
        <MessageBubble
          message={{ role: "tool", content: "result", tool_call_id: "tc-1" }}
        />,
      );
      expect(container.innerHTML).toBe("");
    });

    it("returns null for notification messages", () => {
      const { container } = render(
        <MessageBubble
          message={makeAssistantMessage({ isNotification: true })}
        />,
      );
      expect(container.innerHTML).toBe("");
    });

    it("delegates to BlockReplyBubble for block reply messages", () => {
      render(
        <MessageBubble
          message={makeAssistantMessage({
            isBlockReply: true,
            content: "reply text",
          })}
        />,
      );
      expect(screen.getByTestId("block-reply-bubble")).toBeInTheDocument();
    });

    it("returns null for empty assistant message (no content, no tools)", () => {
      const { container } = render(
        <MessageBubble message={makeAssistantMessage({ content: "" })} />,
      );
      expect(container.innerHTML).toBe("");
    });
  });
});

// --- PBT Tests (Task 2.2) ---

describe("MessageBubble PBT", () => {
  // Property 1: User messages always have flex-row-reverse, bg-primary, text-primary-foreground, max-w-[85%]
  it("Property 1: user messages always styled correctly by role", () => {
    fc.assert(
      fc.property(
        arbitraryChatMessage().map((m) => ({
          ...m,
          role: "user" as const,
          content: m.content || "Hello",
          isBlockReply: undefined,
          isNotification: undefined,
        })),
        (message) => {
          const { container } = render(<MessageBubble message={message} />);
          const wrapper = container.firstElementChild as HTMLElement;
          if (!wrapper) return; // null render is ok for edge cases

          expect(wrapper.className).toContain("flex-row-reverse");

          const bubble = container.querySelector("[class*='bg-primary']");
          if (bubble) {
            expect(bubble.className).toContain("text-primary-foreground");
            expect(bubble.className).toContain("max-w-[85%]");
            expect(bubble.className).toContain("rounded-2xl");
            expect(bubble.className).toContain("rounded-br-sm");
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  // Property 2: Assistant messages always have bg-card, border-border, flex-1 min-w-0
  it("Property 2: assistant messages always styled correctly by role", () => {
    fc.assert(
      fc.property(
        arbitraryChatMessage().map((m) => ({
          ...m,
          role: "assistant" as const,
          content: m.content || "Hi there",
          isBlockReply: undefined,
          isNotification: undefined,
        })),
        (message) => {
          const { container } = render(<MessageBubble message={message} />);
          const wrapper = container.firstElementChild as HTMLElement;
          if (!wrapper) return;

          expect(wrapper.className).not.toContain("flex-row-reverse");

          const bubble = container.querySelector("[class*='bg-card']");
          if (bubble) {
            expect(bubble.className).toContain("border-border");
            expect(bubble.className).toContain("flex-1");
            expect(bubble.className).toContain("min-w-0");
            expect(bubble.className).toContain("rounded-2xl");
            expect(bubble.className).toContain("rounded-bl-sm");
            expect(bubble.className).not.toContain("shadow-sm");
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  // Property 3: Timestamps always show HH:MM format
  it("Property 3: timestamp always renders in HH:MM format", () => {
    fc.assert(
      fc.property(
        arbitraryChatMessage().map((m) => ({
          ...m,
          role: "assistant" as const,
          content: m.content || "Hi",
          timestamp: Math.abs(m.timestamp ?? Date.now()),
          isBlockReply: undefined,
          isNotification: undefined,
        })),
        (message) => {
          const { container } = render(<MessageBubble message={message} />);
          const timestampEl = container.querySelector(
            "[class*='text-[10px]']",
          );
          if (timestampEl) {
            expect(timestampEl.textContent).toMatch(/\d{1,2}:\d{2}/);
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  // Property 4: Tool-only messages (empty content, non-empty toolDetails) render compact
  it("Property 4: tool-only messages render compact format", () => {
    fc.assert(
      fc.property(
        fc.record({
          toolDetails: fc
            .array(arbitraryToolStreamEntry(), {
              minLength: 1,
              maxLength: 5,
            }),
        }),
        ({ toolDetails }) => {
          const message: ChatMessage = {
            role: "assistant",
            content: "",
            toolDetails,
            isBlockReply: undefined,
            isNotification: undefined,
          };
          const { container } = render(<MessageBubble message={message} />);
          const compact = container.querySelector("[class*='bg-muted']");
          expect(compact).not.toBeNull();
          expect(compact!.className).toContain("divide-y");
          expect(compact!.className).toContain("divide-border");
          // Should NOT have bg-card bubble
          const normalBubble = container.querySelector("[class*='bg-card']");
          expect(normalBubble).toBeNull();
        },
      ),
      { numRuns: 50 },
    );
  });
});
