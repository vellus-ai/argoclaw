import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ChatSidebar } from "../chat-sidebar";
import type { SessionInfo } from "@/types/session";

// Mock child components to isolate ChatSidebar tests
vi.mock("@/components/chat/agent-selector", () => ({
  AgentSelector: (props: { value: string; onChange: (v: string) => void }) => (
    <div data-testid="agent-selector" data-value={props.value}>
      AgentSelector
    </div>
  ),
}));

vi.mock("@/components/chat/session-switcher", () => ({
  SessionSwitcher: (props: {
    sessions: SessionInfo[];
    activeKey: string;
    onSelect: (k: string) => void;
    onDelete?: (k: string) => void;
    loading?: boolean;
  }) => (
    <div
      data-testid="session-switcher"
      data-loading={String(props.loading)}
      data-active-key={props.activeKey}
    >
      SessionSwitcher
    </div>
  ),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.PropsWithChildren<Record<string, unknown>>) => (
    <button data-testid="button" {...props}>
      {children}
    </button>
  ),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

const defaultProps = {
  agentId: "agent-1",
  onAgentChange: vi.fn(),
  sessions: [] as SessionInfo[],
  sessionsLoading: false,
  activeSessionKey: "session-1",
  onSessionSelect: vi.fn(),
  onDeleteSession: vi.fn(),
  onNewChat: vi.fn(),
};

describe("ChatSidebar", () => {
  it("shows ARGO branding fallback when no branding provided", () => {
    render(<ChatSidebar {...defaultProps} />);
    const branding = screen.getByText("ARGO");
    expect(branding).toBeInTheDocument();
    expect(branding).toHaveClass("font-bold");
  });

  it("has border-b separators between sections", () => {
    const { container } = render(<ChatSidebar {...defaultProps} />);
    const borderBottomElements = container.querySelectorAll(".border-b");
    expect(borderBottomElements.length).toBeGreaterThanOrEqual(2);
  });

  it("wraps AgentSelector in a card with border bg-card rounded-lg", () => {
    render(<ChatSidebar {...defaultProps} />);
    const agentSelector = screen.getByTestId("agent-selector");
    const cardWrapper = agentSelector.closest(
      ".rounded-lg.border.bg-card"
    );
    expect(cardWrapper).toBeInTheDocument();
  });

  it("passes sessionsLoading to SessionSwitcher", () => {
    render(<ChatSidebar {...defaultProps} sessionsLoading={true} />);
    const switcher = screen.getByTestId("session-switcher");
    expect(switcher).toHaveAttribute("data-loading", "true");
  });

  it("renders the new chat button", () => {
    render(<ChatSidebar {...defaultProps} />);
    expect(screen.getByTestId("button")).toBeInTheDocument();
  });
});
