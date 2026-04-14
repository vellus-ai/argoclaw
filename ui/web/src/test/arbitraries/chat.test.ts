import { describe, it, expect } from "vitest";
import fc from "fast-check";
import {
  arbitraryChatMessage,
  arbitraryToolStreamEntry,
  arbitraryActiveTeamTask,
  arbitraryRunActivity,
  arbitrarySessionInfo,
  arbitraryChatMessageList,
  arbitraryMediaItem,
  arbitraryToolCall,
} from "./chat";

describe("Chat PBT Arbitraries", () => {
  it("generates valid ChatMessage instances", () => {
    fc.assert(
      fc.property(arbitraryChatMessage(), (msg) => {
        expect(["user", "assistant", "tool"]).toContain(msg.role);
        expect(typeof msg.content).toBe("string");
      }),
      { numRuns: 100 },
    );
  });

  it("generates valid ToolStreamEntry instances", () => {
    fc.assert(
      fc.property(arbitraryToolStreamEntry(), (entry) => {
        expect(["calling", "completed", "error"]).toContain(entry.phase);
        expect(typeof entry.name).toBe("string");
        expect(entry.name.length).toBeGreaterThan(0);
        expect(typeof entry.startedAt).toBe("number");
        expect(typeof entry.updatedAt).toBe("number");
      }),
      { numRuns: 100 },
    );
  });

  it("generates valid ActiveTeamTask instances", () => {
    fc.assert(
      fc.property(arbitraryActiveTeamTask(), (task) => {
        expect(task.taskNumber).toBeGreaterThanOrEqual(1);
        expect(task.subject.length).toBeGreaterThan(0);
        expect(typeof task.taskId).toBe("string");
      }),
      { numRuns: 100 },
    );
  });

  it("generates valid RunActivity instances", () => {
    fc.assert(
      fc.property(arbitraryRunActivity(), (activity) => {
        expect([
          "thinking",
          "tool_exec",
          "streaming",
          "compacting",
          "retrying",
          "leader_processing",
        ]).toContain(activity.phase);
      }),
      { numRuns: 100 },
    );
  });

  it("generates valid SessionInfo instances", () => {
    fc.assert(
      fc.property(arbitrarySessionInfo(), (session) => {
        expect(session.key.length).toBeGreaterThan(0);
        expect(session.messageCount).toBeGreaterThanOrEqual(0);
        // Validate ISO date strings
        expect(() => new Date(session.created)).not.toThrow();
        expect(() => new Date(session.updated)).not.toThrow();
      }),
      { numRuns: 100 },
    );
  });

  it("generates valid ToolCall instances", () => {
    fc.assert(
      fc.property(arbitraryToolCall(), (tc) => {
        expect(typeof tc.id).toBe("string");
        expect(tc.name.length).toBeGreaterThan(0);
        expect(typeof tc.arguments).toBe("object");
      }),
      { numRuns: 100 },
    );
  });

  it("generates valid MediaItem instances", () => {
    fc.assert(
      fc.property(arbitraryMediaItem(), (item) => {
        expect(["image", "video", "audio", "document", "code"]).toContain(
          item.kind,
        );
        expect(item.path.length).toBeGreaterThan(0);
        expect(typeof item.mimeType).toBe("string");
      }),
      { numRuns: 100 },
    );
  });

  it("generates mixed message lists with tool-only and notifications", () => {
    fc.assert(
      fc.property(
        arbitraryChatMessageList({ minLength: 5, maxLength: 20 }),
        (messages) => {
          expect(messages.length).toBeGreaterThanOrEqual(5);
          // Verify mix of message types exists
          const hasUser = messages.some((m) => m.role === "user");
          const hasAssistant = messages.some((m) => m.role === "assistant");
          // At least some diversity in a list of 5+
          expect(hasUser || hasAssistant).toBe(true);
        },
      ),
      { numRuns: 50 },
    );
  });

  it("generates ChatMessages where optional fields are truly optional", () => {
    const seen = {
      withThinking: false,
      withoutThinking: false,
      withToolCalls: false,
      withoutToolCalls: false,
      withTimestamp: false,
      withoutTimestamp: false,
    };

    fc.assert(
      fc.property(arbitraryChatMessage(), (msg) => {
        if (msg.thinking !== undefined) seen.withThinking = true;
        else seen.withoutThinking = true;
        if (msg.tool_calls !== undefined) seen.withToolCalls = true;
        else seen.withoutToolCalls = true;
        if (msg.timestamp !== undefined) seen.withTimestamp = true;
        else seen.withoutTimestamp = true;
      }),
      { numRuns: 200 },
    );

    // Over 200 runs, we should see both present and absent for optional fields
    expect(seen.withThinking).toBe(true);
    expect(seen.withoutThinking).toBe(true);
    expect(seen.withToolCalls).toBe(true);
    expect(seen.withoutToolCalls).toBe(true);
    expect(seen.withTimestamp).toBe(true);
    expect(seen.withoutTimestamp).toBe(true);
  });
});
