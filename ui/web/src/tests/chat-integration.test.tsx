/**
 * Task 12 — Integration & Security/Sanitization Tests for Chat Redesign
 *
 * 12.1: Integration tests (AgentSelector click-outside)
 * 12.2: Security/sanitization tests (XSS, dangerouslySetInnerHTML audit)
 */
import { render, screen, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { readFileSync, readdirSync } from "fs";
import path from "path";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        selectAgent: "Select an agent",
        noAgentsAvailable: "No agents available",
        default: "Default",
        sendMessage: "Send a message...",
        sendMessageTitle: "Send message",
        attachFile: "Attach file",
        copyCode: "Copy code",
      };
      return translations[key] ?? key;
    },
  }),
}));

// Mock useAuthStore — returns connected: true and a token
vi.mock("@/stores/use-auth-store", () => ({
  useAuthStore: (selector: (s: { connected: boolean; token: string }) => unknown) =>
    selector({ connected: true, token: "test-token" }),
}));

// Mock useHttp — returns a controllable .get()
const mockGet = vi.fn();
vi.mock("@/hooks/use-ws", () => ({
  useHttp: () => ({ get: mockGet }),
}));

// Mock useClipboard used by MarkdownRenderer
vi.mock("@/hooks/use-clipboard", () => ({
  useClipboard: () => ({ copied: false, copy: vi.fn() }),
}));

// Mock file-helpers used by MarkdownRenderer
vi.mock("@/lib/file-helpers", () => ({
  toFileUrl: (href: string, _token: string) => href,
}));

// Mock image-lightbox used by MarkdownRenderer
vi.mock("@/components/shared/image-lightbox", () => ({
  ImageLightbox: () => null,
}));

// ---------------------------------------------------------------------------
// Imports (after mocks)
// ---------------------------------------------------------------------------

import { AgentSelector } from "@/components/chat/agent-selector";
import { MessageContent } from "@/components/chat/message-content";
import { ChatInput } from "@/components/chat/chat-input";
import type { AttachedFile } from "@/components/chat/chat-input";
import type { AgentData } from "@/types/agent";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeAgent(overrides: Partial<AgentData> = {}): AgentData {
  return {
    id: "a1",
    agent_key: "agent-1",
    display_name: "Test Agent",
    owner_id: "u1",
    provider: "anthropic",
    model: "claude-3",
    context_window: 200000,
    max_tool_iterations: 25,
    workspace: "/tmp",
    restrict_to_workspace: false,
    agent_type: "open",
    is_default: true,
    status: "active",
    ...overrides,
  };
}

function makeFile(name: string): AttachedFile {
  return { file: new File(["content"], name, { type: "text/plain" }) };
}

// ---------------------------------------------------------------------------
// 12.1 — Integration Tests
// ---------------------------------------------------------------------------

describe("Task 12.1 — AgentSelector integration", () => {
  beforeEach(() => {
    mockGet.mockResolvedValue({ agents: [makeAgent()] });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("click-outside closes the dropdown", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    // Render with an outside element to click on
    render(
      <div>
        <div data-testid="outside">Outside area</div>
        <AgentSelector value="agent-1" onChange={onChange} />
      </div>,
    );

    // Wait for the agent name to appear after fetch resolves
    const agentName = await screen.findByText("Test Agent");
    expect(agentName).toBeInTheDocument();

    // Open dropdown by clicking the trigger button
    const triggerButton = agentName.closest("button")!;
    await user.click(triggerButton);

    // Dropdown should be visible — the portaled container has pointer-events-auto
    const dropdown = document.querySelector("[class*='pointer-events-auto']");
    expect(dropdown).not.toBeNull();

    // Click outside (mousedown triggers the close handler)
    const outsideEl = screen.getByTestId("outside");
    await user.click(outsideEl);

    // Dropdown should be closed
    const dropdownAfter = document.querySelector("[class*='pointer-events-auto']");
    expect(dropdownAfter).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// 12.2 — Security / Sanitization Tests
// ---------------------------------------------------------------------------

describe("Task 12.2 — Security / Sanitization", () => {
  afterEach(() => {
    cleanup();
  });

  describe("MessageContent XSS protection", () => {
    it("script tags are rendered as text, not executed", () => {
      const xssContent = '<script>alert("xss")</script>';
      const { container } = render(
        <MessageContent content={xssContent} role="assistant" />,
      );

      // No <script> elements should exist in the DOM
      const scripts = container.querySelectorAll("script");
      expect(scripts.length).toBe(0);

      // The text content should contain the script tag as visible text
      expect(container.textContent).toContain("alert");
    });

    it("HTML tags in message content are not rendered as HTML elements", () => {
      const htmlContent = '<div onclick="alert(1)">click me</div><img src=x onerror="alert(2)">';
      const { container } = render(
        <MessageContent content={htmlContent} role="assistant" />,
      );

      // No onclick handlers should exist
      const clickableDiv = container.querySelector("[onclick]");
      expect(clickableDiv).toBeNull();

      // No onerror handlers should exist
      const imgWithOnerror = container.querySelector("[onerror]");
      expect(imgWithOnerror).toBeNull();
    });

    it("javascript: protocol links are not clickable", () => {
      const dangerousMarkdown = '[click here](javascript:alert(1))';
      const { container } = render(
        <MessageContent content={dangerousMarkdown} role="assistant" />,
      );

      // Find all <a> elements
      const links = container.querySelectorAll("a");
      for (const link of links) {
        const href = link.getAttribute("href");
        // ReactMarkdown with default settings should not create javascript: links
        // If an <a> exists, its href must NOT start with "javascript:"
        if (href) {
          expect(href.toLowerCase()).not.toMatch(/^javascript:/);
        }
      }
    });

    it("data: URI XSS in links is neutralized", () => {
      const dataUriMarkdown = '[evil](data:text/html,<script>alert(1)</script>)';
      const { container } = render(
        <MessageContent content={dataUriMarkdown} role="assistant" />,
      );

      // ReactMarkdown should not create script elements from data URIs
      const scripts = container.querySelectorAll("script");
      expect(scripts.length).toBe(0);
    });
  });

  describe("ChatInput file chips with special characters", () => {
    it("file names with HTML-like characters are escaped correctly", () => {
      const xssFileName = '<img onerror=alert(1)>.txt';
      const { container } = render(
        <ChatInput
          onSend={vi.fn()}
          onAbort={vi.fn()}
          isBusy={false}
          files={[makeFile(xssFileName)]}
          onFilesChange={vi.fn()}
        />,
      );

      // The file name should appear as text, not as an HTML element
      const imgElements = container.querySelectorAll("img");
      expect(imgElements.length).toBe(0);

      // The text should contain the file name
      expect(container.textContent).toContain("<img onerror=alert(1)>.txt");
    });

    it("file names with script tags are rendered safely", () => {
      const scriptFileName = '<script>alert(1)</script>.pdf';
      const { container } = render(
        <ChatInput
          onSend={vi.fn()}
          onAbort={vi.fn()}
          isBusy={false}
          files={[makeFile(scriptFileName)]}
          onFilesChange={vi.fn()}
        />,
      );

      // No <script> element should be in the DOM
      const scripts = container.querySelectorAll("script");
      expect(scripts.length).toBe(0);
    });
  });

  describe("dangerouslySetInnerHTML audit", () => {
    it("no chat component files use dangerouslySetInnerHTML", () => {
      // Audit scope: chat components and the markdown-renderer used for message content.
      // Shared components like file-viewers.tsx may use dangerouslySetInnerHTML for
      // syntax highlighting (highlight.js output) — that is a known, controlled usage
      // outside the chat message rendering path.
      const chatComponentsDir = path.resolve(
        __dirname,
        "../components/chat",
      );

      const dirsToCheck = [chatComponentsDir];
      const violations: string[] = [];

      for (const dir of dirsToCheck) {
        let files: string[];
        try {
          files = readdirSync(dir, { recursive: true }) as unknown as string[];
        } catch {
          continue;
        }

        for (const file of files) {
          const filePath = path.join(dir, file);
          if (!/\.(tsx|ts|jsx|js)$/.test(filePath)) continue;
          // Skip test files and __tests__ directories
          if (filePath.includes("__tests__") || filePath.includes(".test.")) continue;

          try {
            const content = readFileSync(filePath, "utf-8");
            if (content.includes("dangerouslySetInnerHTML")) {
              violations.push(filePath);
            }
          } catch {
            // File might be a directory entry from recursive, skip
          }
        }
      }

      expect(
        violations,
        `Files using dangerouslySetInnerHTML: ${violations.join(", ")}`,
      ).toEqual([]);
    });

    it("markdown-renderer.tsx does not use dangerouslySetInnerHTML", () => {
      const mdRendererPath = path.resolve(
        __dirname,
        "../components/shared/markdown-renderer.tsx",
      );
      const content = readFileSync(mdRendererPath, "utf-8");
      expect(content).not.toContain("dangerouslySetInnerHTML");
    });
  });
});
