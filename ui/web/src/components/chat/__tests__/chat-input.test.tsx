import { render, screen, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import fc from "fast-check";
import { ChatInput } from "../chat-input";
import type { AttachedFile } from "../chat-input";

// Mock i18n
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        sendMessage: "Send a message...",
        sendMessageTitle: "Send message",
        sendFollowUp: "Send follow-up",
        stopGeneration: "Stop generation",
        attachFile: "Attach file",
      };
      return translations[key] ?? key;
    },
  }),
}));

// --- Helpers ---

const defaultProps = {
  onSend: vi.fn(),
  onAbort: vi.fn(),
  isBusy: false,
  files: [] as AttachedFile[],
  onFilesChange: vi.fn(),
};

function makeFile(name: string): AttachedFile {
  return { file: new File(["content"], name, { type: "text/plain" }) };
}

// --- RTL Tests (Task 5.1) ---

describe("ChatInput", () => {
  describe("Textarea a11y", () => {
    it("has aria-label with i18n placeholder text", () => {
      render(<ChatInput {...defaultProps} />);
      const textarea = screen.getByRole("textbox");
      expect(textarea).toHaveAttribute("aria-label", "Send a message...");
    });

    it("has min-h-[44px] for touch target compliance", () => {
      render(<ChatInput {...defaultProps} />);
      const textarea = screen.getByRole("textbox");
      expect(textarea.className).toContain("min-h-[44px]");
    });
  });

  describe("Send button", () => {
    it("is disabled when textarea empty and no files", () => {
      render(<ChatInput {...defaultProps} />);
      const sendBtn = screen.getByTitle("Send message");
      expect(sendBtn).toBeDisabled();
    });

    it("is enabled when textarea has text", async () => {
      render(<ChatInput {...defaultProps} />);
      const textarea = screen.getByRole("textbox");
      await userEvent.type(textarea, "hello");
      const sendBtn = screen.getByTitle("Send message");
      expect(sendBtn).not.toBeDisabled();
    });

    it("is enabled when files attached even if textarea empty", () => {
      render(
        <ChatInput
          {...defaultProps}
          files={[makeFile("test.txt")]}
        />,
      );
      const sendBtn = screen.getByTitle("Send message");
      expect(sendBtn).not.toBeDisabled();
    });
  });

  describe("Busy state", () => {
    it("shows both send and stop buttons when isBusy=true", () => {
      render(<ChatInput {...defaultProps} isBusy={true} />);
      expect(screen.getByTitle("Send follow-up")).toBeInTheDocument();
      expect(screen.getByTitle("Stop generation")).toBeInTheDocument();
    });
  });

  describe("File chips", () => {
    it("shows file name truncated with remove button", () => {
      render(
        <ChatInput
          {...defaultProps}
          files={[makeFile("my-document.txt")]}
        />,
      );
      expect(screen.getByText("my-document.txt")).toBeInTheDocument();
      // X button for removal
      const removeButtons = screen.getAllByRole("button").filter(
        (btn) => btn.querySelector("svg"),
      );
      expect(removeButtons.length).toBeGreaterThan(0);
    });
  });

  describe("Attach button", () => {
    it("is disabled when isBusy=true", () => {
      render(<ChatInput {...defaultProps} isBusy={true} />);
      const attachBtn = screen.getByTitle("Attach file");
      expect(attachBtn).toBeDisabled();
    });

    it("is disabled when disabled=true", () => {
      render(<ChatInput {...defaultProps} disabled={true} />);
      const attachBtn = screen.getByTitle("Attach file");
      expect(attachBtn).toBeDisabled();
    });

    it("is enabled when not busy and not disabled", () => {
      render(<ChatInput {...defaultProps} />);
      const attachBtn = screen.getByTitle("Attach file");
      expect(attachBtn).not.toBeDisabled();
    });
  });
});

// --- PBT Tests (Task 5.2) ---

describe("ChatInput PBT", () => {
  // Property 6: Send button disabled iff text.trim() === "" AND files.length === 0
  it("Property 6: send button disabled iff empty text and no files", () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 0, maxLength: 50 }),
        fc.array(
          fc.record({
            file: fc.constant(new File(["x"], "f.txt", { type: "text/plain" })),
          }),
          { minLength: 0, maxLength: 3 },
        ),
        (text, files) => {
          cleanup();
          const shouldBeDisabled =
            text.trim() === "" && files.length === 0;

          const { container } = render(
            <ChatInput
              {...defaultProps}
              files={files}
            />,
          );

          // Type text into the textarea
          const textarea = container.querySelector("textarea")!;
          // We need to set value via native property since userEvent is async
          Object.getOwnPropertyDescriptor(
            window.HTMLTextAreaElement.prototype,
            "value",
          )?.set?.call(textarea, text);
          textarea.dispatchEvent(new Event("input", { bubbles: true }));
          textarea.dispatchEvent(new Event("change", { bubbles: true }));

          // Find the send button (not stop button) — it has title "Send message" or "Send follow-up"
          const sendBtn = container.querySelector(
            'button[title="Send message"]',
          ) as HTMLButtonElement;
          if (sendBtn) {
            if (shouldBeDisabled) {
              expect(sendBtn.disabled).toBe(true);
            } else {
              expect(sendBtn.disabled).toBe(false);
            }
          }
        },
      ),
      { numRuns: 50 },
    );
  });

  // Property 7: Attach button disabled iff isBusy OR disabled
  it("Property 7: attach button disabled iff isBusy or disabled", () => {
    fc.assert(
      fc.property(
        fc.boolean(),
        fc.boolean(),
        (isBusy, isDisabled) => {
          cleanup();
          const shouldBeDisabled = isBusy || isDisabled;

          render(
            <ChatInput
              {...defaultProps}
              isBusy={isBusy}
              disabled={isDisabled}
            />,
          );

          const attachBtn = screen.getByTitle("Attach file");
          if (shouldBeDisabled) {
            expect(attachBtn).toBeDisabled();
          } else {
            expect(attachBtn).not.toBeDisabled();
          }
        },
      ),
      { numRuns: 20 },
    );
  });
});
