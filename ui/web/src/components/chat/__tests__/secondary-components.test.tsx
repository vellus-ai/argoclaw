import { render, screen, within, act, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import fc from "fast-check";
import { TeamActivityPanel } from "../team-activity-panel";
import { DropZone } from "../drop-zone";
import type { ActiveTeamTask } from "@/types/chat";
import { arbitraryActiveTeamTask } from "@/test/arbitraries/chat";

// Mock i18n (not used by these components, but keep consistent)
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// --- TeamActivityPanel ---

describe("TeamActivityPanel", () => {
  it("returns null when tasks is empty", () => {
    const { container } = render(<TeamActivityPanel tasks={[]} />);
    expect(container.innerHTML).toBe("");
  });

  it("shows header with correct count for 1 task", () => {
    const task: ActiveTeamTask = {
      taskId: "t-1",
      taskNumber: 42,
      subject: "Deploy service",
      status: "running",
      ownerAgentKey: "agent-alpha",
    };
    render(<TeamActivityPanel tasks={[task]} />);
    expect(screen.getByText("Team: 1 task active")).toBeInTheDocument();
  });

  it("shows header with correct count for multiple tasks", () => {
    const tasks: ActiveTeamTask[] = [
      {
        taskId: "t-1",
        taskNumber: 1,
        subject: "Task A",
        status: "running",
        ownerAgentKey: "agent-a",
      },
      {
        taskId: "t-2",
        taskNumber: 2,
        subject: "Task B",
        status: "running",
        ownerAgentKey: "agent-b",
      },
      {
        taskId: "t-3",
        taskNumber: 3,
        subject: "Task C",
        status: "running",
        ownerAgentKey: "agent-c",
      },
    ];
    render(<TeamActivityPanel tasks={tasks} />);
    expect(screen.getByText("Team: 3 tasks active")).toBeInTheDocument();
  });

  it("shows taskNumber, subject, arrow, and ownerDisplayName", () => {
    const task: ActiveTeamTask = {
      taskId: "t-1",
      taskNumber: 7,
      subject: "Fix login bug",
      status: "running",
      ownerAgentKey: "agent-beta",
      ownerDisplayName: "Beta Agent",
    };
    render(<TeamActivityPanel tasks={[task]} />);
    expect(screen.getByText("#7")).toBeInTheDocument();
    expect(screen.getByText("Fix login bug")).toBeInTheDocument();
    expect(screen.getByText("\u2192")).toBeInTheDocument();
    expect(screen.getByText("Beta Agent")).toBeInTheDocument();
  });

  it("falls back to ownerAgentKey when ownerDisplayName is undefined", () => {
    const task: ActiveTeamTask = {
      taskId: "t-1",
      taskNumber: 3,
      subject: "Run tests",
      status: "running",
      ownerAgentKey: "agent-gamma",
    };
    render(<TeamActivityPanel tasks={[task]} />);
    expect(screen.getByText("agent-gamma")).toBeInTheDocument();
  });

  it("shows progressPercent when defined", () => {
    const task: ActiveTeamTask = {
      taskId: "t-1",
      taskNumber: 5,
      subject: "Build image",
      status: "running",
      ownerAgentKey: "agent-delta",
      progressPercent: 73,
    };
    render(<TeamActivityPanel tasks={[task]} />);
    expect(screen.getByText("73%")).toBeInTheDocument();
  });

  it("does not show percent when progressPercent is undefined", () => {
    const task: ActiveTeamTask = {
      taskId: "t-1",
      taskNumber: 5,
      subject: "Build image",
      status: "running",
      ownerAgentKey: "agent-delta",
    };
    render(<TeamActivityPanel tasks={[task]} />);
    expect(screen.queryByText(/%$/)).not.toBeInTheDocument();
  });
});

// --- DropZone ---

describe("DropZone", () => {
  it("has role=status and aria-live=assertive on overlay during drag state", () => {
    const { container } = render(
      <DropZone onDrop={vi.fn()}>
        <div>content</div>
      </DropZone>,
    );

    // Simulate dragEnter to trigger overlay
    const wrapper = container.firstElementChild!;
    act(() => {
      fireEvent.dragEnter(wrapper, { dataTransfer: { files: [] } });
    });

    // After drag, the overlay should appear with a11y attrs
    const overlay = screen.getByRole("status");
    expect(overlay).toBeInTheDocument();
    expect(overlay).toHaveAttribute("aria-live", "assertive");
  });
});

// --- PBT ---

describe("TeamActivityPanel PBT", () => {
  it("Property 12: header shows correct count and each task data is rendered", () => {
    fc.assert(
      fc.property(
        fc.array(arbitraryActiveTeamTask(), { minLength: 1, maxLength: 10 }),
        (tasks) => {
          const { container, unmount } = render(
            <TeamActivityPanel tasks={tasks} />,
          );

          try {
            // Header count
            const expectedLabel =
              tasks.length === 1
                ? "Team: 1 task active"
                : `Team: ${tasks.length} tasks active`;
            const header = within(container).getByText(expectedLabel);
            expect(header).toBeInTheDocument();

            // Each task row rendered with its data (space-y-1 div's children)
            const taskContainer =
              container.querySelector(".space-y-1")!;
            const rows = taskContainer.children;
            expect(rows.length).toBe(tasks.length);

            for (let i = 0; i < tasks.length; i++) {
              const task = tasks[i]!;
              const row = rows[i] as HTMLElement;
              const spans = row.querySelectorAll("span");

              // spans[0] = taskNumber, spans[1] = subject, spans[2] = arrow, spans[3] = owner, spans[4]? = percent
              expect(spans[0]!.textContent).toBe(`#${task.taskNumber}`);
              expect(spans[1]!.textContent).toBe(task.subject);
              expect(spans[2]!.textContent).toBe("\u2192");

              const expectedOwner =
                task.ownerDisplayName || task.ownerAgentKey || "";
              expect(spans[3]!.textContent).toBe(expectedOwner);

              if (task.progressPercent != null) {
                expect(spans[4]!).toBeDefined();
                expect(spans[4]!.textContent).toBe(
                  `${task.progressPercent}%`,
                );
              }
            }
          } finally {
            unmount();
          }
        },
      ),
      { numRuns: 50 },
    );
  });
});
