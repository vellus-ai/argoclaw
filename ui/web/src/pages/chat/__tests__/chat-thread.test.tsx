import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import fc from "fast-check";
import { ChatThread, buildDisplayItems } from "../chat-thread";
import type { ChatMessage } from "@/types/chat";
import {
  arbitraryChatMessageList,
  arbitraryToolStreamEntry,
} from "@/test/arbitraries/chat";

// Mock i18n
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        "empty.title": "Start a conversation",
        "empty.description": "Send a message to begin chatting with the agent.",
        "onboarding.welcome.title": "Welcome to ARGO!",
        "onboarding.welcome.description":
          "I'm the First Mate, your helmsman. Let's set up your crew.",
      };
      return translations[key] ?? key;
    },
  }),
}));

// Mock child components to isolate ChatThread
vi.mock("@/components/chat/message-bubble", () => ({
  MessageBubble: ({ message }: { message: ChatMessage }) => (
    <div data-testid={`message-${message.role}`}>{message.content}</div>
  ),
}));

vi.mock("@/components/chat/active-run-zone", () => ({
  ActiveRunZone: () => <div data-testid="active-run-zone" />,
}));

vi.mock("@/components/chat/system-notification", () => ({
  SystemNotification: ({ message }: { message: ChatMessage }) => (
    <div data-testid="system-notification">{message.content}</div>
  ),
}));

vi.mock("@/components/chat/team-activity-panel", () => ({
  TeamActivityPanel: () => <div data-testid="team-activity-panel" />,
}));

vi.mock("@/components/chat/tool-call-card", () => ({
  ToolCallCard: () => <div data-testid="tool-call-card" />,
}));

vi.mock("@/components/chat/thinking-block", () => ({
  ThinkingBlock: () => <div data-testid="thinking-block" />,
}));

vi.mock("@/hooks/use-auto-scroll", () => ({
  useAutoScroll: () => ({ ref: { current: null }, onScroll: () => {} }),
}));

// --- Helpers ---

const defaultProps = {
  messages: [] as ChatMessage[],
  streamText: null,
  thinkingText: null,
  toolStream: [],
  blockReplies: [],
  activity: null,
  teamTasks: [],
  isRunning: false,
  isBusy: false,
};

function makeMsg(
  role: "user" | "assistant" | "tool",
  content: string,
  extra: Partial<ChatMessage> = {},
): ChatMessage {
  return { role, content, ...extra };
}

// --- RTL Tests (Task 3.1) ---

describe("ChatThread", () => {
  describe("Background", () => {
    it("does NOT have radial-gradient backgroundImage inline style", () => {
      const { container } = render(
        <ChatThread
          {...defaultProps}
          messages={[makeMsg("user", "hello")]}
          isBusy={false}
        />,
      );
      const scrollContainer = container.querySelector(
        "[class*='overflow-y-auto']",
      );
      expect(scrollContainer).not.toBeNull();
      expect((scrollContainer as HTMLElement).style.backgroundImage).toBe("");
    });

    it("has bg-background class on scroll container", () => {
      const { container } = render(
        <ChatThread
          {...defaultProps}
          messages={[makeMsg("user", "hello")]}
        />,
      );
      const scrollContainer = container.querySelector(
        "[class*='overflow-y-auto']",
      );
      expect(scrollContainer).not.toBeNull();
      expect((scrollContainer as HTMLElement).className).toContain(
        "bg-background",
      );
    });
  });

  describe("Content container", () => {
    it("has max-w-3xl class", () => {
      const { container } = render(
        <ChatThread
          {...defaultProps}
          messages={[makeMsg("user", "hello")]}
        />,
      );
      const contentContainer = container.querySelector(
        "[class*='max-w-3xl']",
      );
      expect(contentContainer).not.toBeNull();
    });
  });

  describe("Accessibility", () => {
    it("has role='log' on the scroll container", () => {
      const { container } = render(
        <ChatThread
          {...defaultProps}
          messages={[makeMsg("user", "hello")]}
        />,
      );
      const logElement = container.querySelector("[role='log']");
      expect(logElement).not.toBeNull();
    });

    it("has aria-live='polite' on the scroll container", () => {
      const { container } = render(
        <ChatThread
          {...defaultProps}
          messages={[makeMsg("user", "hello")]}
        />,
      );
      const logElement = container.querySelector("[aria-live='polite']");
      expect(logElement).not.toBeNull();
    });
  });

  describe("Empty state — chat mode (default)", () => {
    it("shows title and description when messages empty, not busy, not loading", () => {
      render(<ChatThread {...defaultProps} />);
      expect(screen.getByText("Start a conversation")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Send a message to begin chatting with the agent.",
        ),
      ).toBeInTheDocument();
    });
  });

  describe("Empty state — onboarding mode", () => {
    it("shows onboarding welcome when mode='onboarding'", () => {
      render(<ChatThread {...defaultProps} mode="onboarding" />);
      expect(screen.getByText("Welcome to ARGO!")).toBeInTheDocument();
      expect(
        screen.getByText(
          "I'm the First Mate, your helmsman. Let's set up your crew.",
        ),
      ).toBeInTheDocument();
    });
  });

  describe("Loading state", () => {
    it("shows spinner when loading=true and no messages", () => {
      const { container } = render(
        <ChatThread {...defaultProps} loading={true} />,
      );
      const spinner = container.querySelector("[class*='animate-spin']");
      expect(spinner).not.toBeNull();
    });
  });

  describe("Busy state", () => {
    it("does NOT show empty state when isBusy=true even with no messages", () => {
      render(<ChatThread {...defaultProps} isBusy={true} />);
      expect(
        screen.queryByText("Start a conversation"),
      ).not.toBeInTheDocument();
    });
  });

  describe("Message rendering", () => {
    it("renders user and assistant messages", () => {
      render(
        <ChatThread
          {...defaultProps}
          messages={[
            makeMsg("user", "hello"),
            makeMsg("assistant", "hi there"),
          ]}
        />,
      );
      expect(screen.getByText("hello")).toBeInTheDocument();
      expect(screen.getByText("hi there")).toBeInTheDocument();
    });

    it("renders notifications via SystemNotification", () => {
      render(
        <ChatThread
          {...defaultProps}
          messages={[
            makeMsg("assistant", "Task dispatched", {
              isNotification: true,
              notificationType: "dispatched",
            }),
          ]}
        />,
      );
      expect(screen.getByTestId("system-notification")).toBeInTheDocument();
    });
  });
});

// --- PBT Tests (Task 3.2) ---

describe("buildDisplayItems PBT", () => {
  // Property 5: Tool-only consecutive messages grouped into merged-tools
  it("Property 5: consecutive tool-only messages produce single merged-tools item", () => {
    fc.assert(
      fc.property(
        fc
          .array(arbitraryToolStreamEntry(), { minLength: 1, maxLength: 5 })
          .chain((tools) =>
            fc.tuple(
              fc.constant(tools),
              fc.integer({ min: 2, max: 5 }),
            ),
          ),
        ([tools, count]) => {
          // Create N consecutive tool-only messages
          const toolOnlyMsgs: ChatMessage[] = Array.from(
            { length: count },
            (_, i) => ({
              role: "assistant" as const,
              content: "",
              toolDetails: [tools[i % tools.length]!],
            }),
          );

          const items = buildDisplayItems(toolOnlyMsgs);

          // All tool-only consecutive messages should merge into ONE merged-tools item
          const mergedItems = items.filter((it) => it.kind === "merged-tools");
          expect(mergedItems.length).toBe(1);

          // No individual "message" items should be tool-only
          const msgItems = items.filter((it) => it.kind === "message");
          for (const mi of msgItems) {
            const msg = mi.msg;
            const isToolOnly =
              msg.role === "assistant" &&
              !msg.content?.trim() &&
              ((msg.toolDetails && msg.toolDetails.length > 0) ||
                (msg.tool_calls && msg.tool_calls.length > 0));
            expect(isToolOnly).toBe(false);
          }
        },
      ),
      { numRuns: 50 },
    );
  });

  // Property 5b: tool-only messages never appear as "message" kind
  it("Property 5b: no tool-only message appears as kind 'message'", () => {
    fc.assert(
      fc.property(
        arbitraryChatMessageList({ minLength: 1, maxLength: 20 }),
        (messages) => {
          const items = buildDisplayItems(messages);

          for (const item of items) {
            if (item.kind === "message") {
              const msg = item.msg;
              if (msg.role === "assistant" && !msg.content?.trim()) {
                const hasTools =
                  (msg.toolDetails && msg.toolDetails.length > 0) ||
                  (msg.tool_calls && msg.tool_calls.length > 0);
                // tool-only messages must NOT be in "message" items
                expect(hasTools).toBe(false);
              }
            }
          }
        },
      ),
      { numRuns: 100 },
    );
  });

  // Property 13: Notifications classified as "notification" kind
  it("Property 13: messages with isNotification=true always classified as 'notification'", () => {
    fc.assert(
      fc.property(
        arbitraryChatMessageList({ minLength: 1, maxLength: 20 }),
        (messages) => {
          const items = buildDisplayItems(messages);

          // Count notifications in input
          const inputNotifications = messages.filter(
            (m) =>
              m.isNotification === true &&
              // Exclude [System] user messages which are filtered out
              !(
                m.role === "user" &&
                typeof m.content === "string" &&
                m.content.startsWith("[System]")
              ),
          );

          // Count notification items in output
          const outputNotifications = items.filter(
            (it) => it.kind === "notification",
          );

          expect(outputNotifications.length).toBe(inputNotifications.length);

          // No notification should appear as "message" kind
          for (const item of items) {
            if (item.kind === "message") {
              expect(item.msg.isNotification).not.toBe(true);
            }
          }
        },
      ),
      { numRuns: 100 },
    );
  });
});
