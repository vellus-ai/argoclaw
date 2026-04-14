import { render, screen, cleanup } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import fc from "fast-check";
import { ChatTopBar } from "../chat-top-bar";
import type { RunActivity, ActiveTeamTask } from "@/types/chat";
import { arbitraryRunActivity, arbitraryActiveTeamTask } from "@/test/arbitraries/chat";

// Mock hooks
vi.mock("@/hooks/use-ws", () => ({
  useHttp: () => ({
    get: vi.fn().mockResolvedValue({ agents: [] }),
  }),
}));

vi.mock("@/stores/use-auth-store", () => ({
  useAuthStore: (sel: (s: { connected: boolean }) => boolean) =>
    sel({ connected: true }),
}));

// --- Helpers ---

const defaultProps = {
  agentId: "test-agent",
  isRunning: false,
  isBusy: false,
  activity: null,
  teamTasks: [] as ActiveTeamTask[],
};

const phaseLabels: Record<RunActivity["phase"], string> = {
  thinking: "Thinking…",
  tool_exec: "Running tool…",
  streaming: "Responding…",
  compacting: "Compacting…",
  retrying: "Retrying…",
  leader_processing: "Processing team results…",
};

// --- RTL Tests (Task 6.1) ---

describe("ChatTopBar", () => {
  describe("Onboarding mode", () => {
    it("shows badge 'Configuração Inicial' when mode='onboarding'", () => {
      render(<ChatTopBar {...defaultProps} mode="onboarding" />);
      expect(screen.getByText("Configuração Inicial")).toBeInTheDocument();
    });
  });

  describe("Idle state", () => {
    it("shows 'Ready' with reduced opacity when not running and no team tasks", () => {
      render(<ChatTopBar {...defaultProps} />);
      expect(screen.getByText("Ready")).toBeInTheDocument();
    });
  });

  describe("Running state", () => {
    it("shows phase label with animated Loader2 when isRunning=true", () => {
      render(
        <ChatTopBar
          {...defaultProps}
          isRunning={true}
          activity={{ phase: "thinking" }}
        />,
      );
      expect(screen.getByText("Thinking…")).toBeInTheDocument();
    });
  });

  describe("Team tasks state", () => {
    it("shows 'Team: N task(s) active' when teamTasks > 0 and not running", () => {
      const tasks: ActiveTeamTask[] = [
        {
          taskId: "t1",
          taskNumber: 1,
          subject: "Research",
          status: "running",
        },
        {
          taskId: "t2",
          taskNumber: 2,
          subject: "Analysis",
          status: "running",
        },
      ];
      render(
        <ChatTopBar {...defaultProps} isBusy={true} teamTasks={tasks} />,
      );
      expect(screen.getByText(/Team: 2 tasks active/)).toBeInTheDocument();
    });
  });
});

// --- PBT Tests (Task 6.2) ---

describe("ChatTopBar PBT", () => {
  // Property 8: Phase label matches for all valid RunActivity phases when isRunning
  it("Property 8: phase label matches activity phase when running", () => {
    fc.assert(
      fc.property(arbitraryRunActivity(), (activity) => {
        cleanup();
        render(
          <ChatTopBar
            {...defaultProps}
            isRunning={true}
            activity={activity}
          />,
        );
        const expectedLabel = phaseLabels[activity.phase];
        expect(screen.getByText(expectedLabel)).toBeInTheDocument();
      }),
      { numRuns: 50 },
    );
  });

  // Property 9: Team task count displayed correctly when not running
  it("Property 9: team task count displayed when not running and tasks > 0", () => {
    fc.assert(
      fc.property(
        fc.array(arbitraryActiveTeamTask(), { minLength: 1, maxLength: 5 }),
        (tasks) => {
          cleanup();
          render(
            <ChatTopBar
              {...defaultProps}
              isBusy={true}
              teamTasks={tasks}
            />,
          );
          const expectedText = `Team: ${tasks.length} task${tasks.length > 1 ? "s" : ""} active`;
          expect(screen.getByText(expectedText)).toBeInTheDocument();
        },
      ),
      { numRuns: 30 },
    );
  });
});
