/**
 * Reusable fast-check Arbitrary generators for chat PBT testing.
 * Each generator produces values conforming to the TypeScript types
 * defined in @/types/chat.ts and @/types/session.ts.
 */
import fc from "fast-check";
import type {
  ChatMessage,
  ToolStreamEntry,
  ActiveTeamTask,
  RunActivity,
  MediaItem,
} from "@/types/chat";
import type { SessionInfo } from "@/types/session";
import type { ToolCall } from "@/types/session";

// --- Primitives ---

/** Generate a valid emoji string */
export const arbitraryEmoji = () =>
  fc.constantFrom("🤖", "⚡", "🧠", "📋", "🔧", "🎯", "💡", "🚀");

/** Generate a valid agent key */
export const arbitraryAgentKey = () =>
  fc.stringMatching(/^[a-z][a-z0-9-]{2,20}$/);

/** Generate a valid run phase */
export const arbitraryPhase = () =>
  fc.constantFrom(
    "thinking",
    "tool_exec",
    "streaming",
    "compacting",
    "retrying",
    "leader_processing",
  ) as fc.Arbitrary<RunActivity["phase"]>;

/** Generate a valid tool phase */
export const arbitraryToolPhase = () =>
  fc.constantFrom("calling", "completed", "error") as fc.Arbitrary<
    ToolStreamEntry["phase"]
  >;

/** Generate a valid message role */
export const arbitraryRole = () =>
  fc.constantFrom("user", "assistant", "tool") as fc.Arbitrary<
    ChatMessage["role"]
  >;

// --- Complex types ---

/** Generate a RunActivity */
export const arbitraryRunActivity = (): fc.Arbitrary<RunActivity> =>
  fc.record({
    phase: arbitraryPhase(),
    tool: fc.option(fc.string({ minLength: 1, maxLength: 30 }), {
      nil: undefined,
    }),
    tools: fc.option(
      fc.array(fc.string({ minLength: 1, maxLength: 20 }), {
        minLength: 0,
        maxLength: 5,
      }),
      { nil: undefined },
    ),
    iteration: fc.option(fc.nat(10), { nil: undefined }),
    retryAttempt: fc.option(fc.nat(5), { nil: undefined }),
    retryMax: fc.option(fc.nat(5), { nil: undefined }),
  });

/** Generate a ToolCall (matching session.ts ToolCall interface) */
export const arbitraryToolCall = (): fc.Arbitrary<ToolCall> =>
  fc.record({
    id: fc.uuid(),
    name: fc.string({ minLength: 1, maxLength: 30 }),
    arguments: fc.dictionary(
      fc.string({ minLength: 1, maxLength: 10 }),
      fc.oneof(fc.string(), fc.integer(), fc.boolean()),
    ) as fc.Arbitrary<Record<string, unknown>>,
  });

/** Generate a ToolStreamEntry */
export const arbitraryToolStreamEntry = (): fc.Arbitrary<ToolStreamEntry> =>
  fc.record({
    toolCallId: fc.uuid(),
    runId: fc.uuid(),
    name: fc.string({ minLength: 1, maxLength: 30 }),
    phase: arbitraryToolPhase(),
    arguments: fc.option(
      fc.dictionary(
        fc.string({ minLength: 1, maxLength: 10 }),
        fc.oneof(fc.string(), fc.integer(), fc.boolean()),
      ) as fc.Arbitrary<Record<string, unknown>>,
      { nil: undefined },
    ),
    result: fc.option(fc.string({ minLength: 0, maxLength: 200 }), {
      nil: undefined,
    }),
    errorContent: fc.option(fc.string({ minLength: 1, maxLength: 100 }), {
      nil: undefined,
    }),
    startedAt: fc.nat(1e12),
    updatedAt: fc.nat(1e12),
  });

/** Generate an ActiveTeamTask */
export const arbitraryActiveTeamTask = (): fc.Arbitrary<ActiveTeamTask> =>
  fc.record({
    taskId: fc.uuid(),
    taskNumber: fc.integer({ min: 1, max: 999 }),
    subject: fc.string({ minLength: 1, maxLength: 100 }),
    status: fc.constantFrom("running", "completed", "failed", "pending"),
    ownerAgentKey: fc.option(arbitraryAgentKey(), { nil: undefined }),
    ownerDisplayName: fc.option(
      fc.string({ minLength: 1, maxLength: 50 }),
      { nil: undefined },
    ),
    progressPercent: fc.option(fc.integer({ min: 0, max: 100 }), {
      nil: undefined,
    }),
    progressStep: fc.option(fc.string({ minLength: 1, maxLength: 50 }), {
      nil: undefined,
    }),
  });

/** Generate a MediaItem */
export const arbitraryMediaItem = (): fc.Arbitrary<MediaItem> =>
  fc.record({
    path: fc.string({ minLength: 1, maxLength: 100 }),
    mimeType: fc.constantFrom(
      "image/png",
      "image/jpeg",
      "video/mp4",
      "audio/mpeg",
      "application/pdf",
      "text/plain",
    ),
    fileName: fc.option(fc.string({ minLength: 1, maxLength: 50 }), {
      nil: undefined,
    }),
    size: fc.option(fc.nat(10_000_000), { nil: undefined }),
    kind: fc.constantFrom("image", "video", "audio", "document", "code"),
  });

/** Generate a ChatMessage */
export const arbitraryChatMessage = (): fc.Arbitrary<ChatMessage> =>
  fc.record({
    role: arbitraryRole(),
    content: fc.string({ minLength: 0, maxLength: 500 }),
    thinking: fc.option(fc.string({ minLength: 1, maxLength: 200 }), {
      nil: undefined,
    }),
    tool_calls: fc.option(
      fc.array(arbitraryToolCall(), { minLength: 0, maxLength: 3 }),
      { nil: undefined },
    ),
    tool_call_id: fc.option(fc.uuid(), { nil: undefined }),
    is_error: fc.option(fc.boolean(), { nil: undefined }),
    media_refs: fc.option(
      fc.array(
        fc.record({
          id: fc.uuid(),
          mime_type: fc.constantFrom("image/png", "image/jpeg", "application/pdf"),
          kind: fc.constantFrom("image", "video", "audio", "document", "code"),
        }),
        { minLength: 0, maxLength: 3 },
      ),
      { nil: undefined },
    ),
    timestamp: fc.option(fc.nat(1e12), { nil: undefined }),
    isStreaming: fc.option(fc.boolean(), { nil: undefined }),
    toolDetails: fc.option(
      fc.array(arbitraryToolStreamEntry(), { minLength: 0, maxLength: 5 }),
      { nil: undefined },
    ),
    isBlockReply: fc.option(fc.boolean(), { nil: undefined }),
    isNotification: fc.option(fc.boolean(), { nil: undefined }),
    notificationType: fc.option(
      fc.constantFrom("dispatched", "completed", "failed"),
      { nil: undefined },
    ),
    mediaItems: fc.option(
      fc.array(arbitraryMediaItem(), { minLength: 0, maxLength: 3 }),
      { nil: undefined },
    ),
  });

/** Generate a SessionInfo */
export const arbitrarySessionInfo = (): fc.Arbitrary<SessionInfo> =>
  fc.record({
    key: fc.uuid(),
    messageCount: fc.nat(1000),
    created: fc.date().map((d) => d.toISOString()),
    updated: fc.date().map((d) => d.toISOString()),
    label: fc.option(fc.string({ minLength: 1, maxLength: 50 }), {
      nil: undefined,
    }),
    model: fc.option(
      fc.constantFrom("claude-3.5-sonnet", "gpt-4o", "deepseek-v3"),
      { nil: undefined },
    ),
    provider: fc.option(
      fc.constantFrom("anthropic", "openai", "deepseek"),
      { nil: undefined },
    ),
    channel: fc.option(fc.constantFrom("web", "telegram", "discord"), {
      nil: undefined,
    }),
    inputTokens: fc.option(fc.nat(100_000), { nil: undefined }),
    outputTokens: fc.option(fc.nat(100_000), { nil: undefined }),
    userID: fc.option(fc.uuid(), { nil: undefined }),
    metadata: fc.option(
      fc.dictionary(
        fc.string({ minLength: 1, maxLength: 10 }),
        fc.string({ minLength: 0, maxLength: 50 }),
      ),
      { nil: undefined },
    ),
    agentName: fc.option(fc.string({ minLength: 1, maxLength: 30 }), {
      nil: undefined,
    }),
    estimatedTokens: fc.option(fc.nat(200_000), { nil: undefined }),
    contextWindow: fc.option(fc.nat(200_000), { nil: undefined }),
    compactionCount: fc.option(fc.nat(50), { nil: undefined }),
  });

/** Generate a mixed list of ChatMessages (user, assistant, tool-only, notifications) */
export const arbitraryChatMessageList = (opts?: {
  minLength?: number;
  maxLength?: number;
}): fc.Arbitrary<ChatMessage[]> =>
  fc.array(
    fc.oneof(
      // Regular user message
      arbitraryChatMessage().map((m) => ({
        ...m,
        role: "user" as const,
        content: m.content || "Hello",
      })),
      // Regular assistant message
      arbitraryChatMessage().map((m) => ({
        ...m,
        role: "assistant" as const,
        content: m.content || "Hi there",
      })),
      // Tool-only assistant message (content empty, toolDetails non-empty)
      arbitraryChatMessage().map((m) => ({
        ...m,
        role: "assistant" as const,
        content: "",
        toolDetails:
          m.toolDetails && m.toolDetails.length > 0
            ? m.toolDetails
            : [
                {
                  toolCallId: "tc-1",
                  runId: "r-1",
                  name: "search",
                  phase: "completed" as const,
                  startedAt: Date.now(),
                  updatedAt: Date.now(),
                },
              ],
      })),
      // Notification message
      arbitraryChatMessage().map((m) => ({
        ...m,
        role: "assistant" as const,
        content: "Task dispatched to agent-alpha",
        isNotification: true,
        notificationType: "dispatched",
      })),
    ),
    {
      minLength: opts?.minLength ?? 0,
      maxLength: opts?.maxLength ?? 30,
    },
  );
