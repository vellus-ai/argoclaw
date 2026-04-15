import { describe, it, expect, beforeEach } from "vitest";
import {
  onboardingReducer,
  _resetMessageCounter,
  getMessagesForState,
  greetingKeyForGender,
  type Gender,
  type OnboardingContext,
  type OnboardingState,
  type TranslatorFn,
  type EngineState,
  type ChatMessageLocal,
} from "./use-onboarding-engine";
import type { OnboardingStatusResponse } from "./use-onboarding-api";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const INITIAL_CONTEXT: OnboardingContext = {
  displayName: "Milton",
  agentName: "Imediato",
  agentId: "agent-001",
  gender: null,
  agentGender: null,
  workspaceType: "",
  accountName: "",
  primaryColor: "#1E40AF",
  selectedProvider: "",
  selectedChannel: "",
  workspaceConfigured: false,
  brandingSet: false,
  onboardingComplete: false,
};

function makeState(
  currentState: OnboardingState = "loading",
  overrides?: Partial<OnboardingContext>,
): EngineState {
  return {
    currentState,
    context: { ...INITIAL_CONTEXT, ...overrides },
    messages: [] as ChatMessageLocal[],
    error: null,
    _translator: mockT,
  };
}

function makeStatus(
  overrides?: Partial<OnboardingStatusResponse>,
): OnboardingStatusResponse {
  return {
    onboarding_complete: false,
    workspace_configured: false,
    branding_set: false,
    ...overrides,
  };
}

/** Mock translator — returns key with opts appended for verification. */
const mockT: TranslatorFn = (key, opts) => {
  if (opts && Object.keys(opts).length > 0) {
    const pairs = Object.entries(opts)
      .map(([k, v]) => `${k}=${v}`)
      .join(",");
    return `${key}[${pairs}]`;
  }
  return key;
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("onboardingReducer", () => {
  beforeEach(() => {
    _resetMessageCounter();
  });

  // ========================================================================
  // INIT action
  // ========================================================================

  describe("INIT action", () => {
    it("should transition loading → welcome when onboarding_complete=false", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus(),
      });
      expect(next.currentState).toBe("welcome");
    });

    it("should transition loading → complete when onboarding_complete=true", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ onboarding_complete: true }),
      });
      expect(next.currentState).toBe("complete");
    });

    it("should resume after last_completed_state=naming", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ last_completed_state: "naming" }),
      });
      expect(next.currentState).toBe("naming_custom");
    });

    it("should resume after last_completed_state=workspace_type", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ last_completed_state: "workspace_type" }),
      });
      expect(next.currentState).toBe("workspace_details");
    });

    it("should resume after last_completed_state=branding", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ last_completed_state: "branding" }),
      });
      expect(next.currentState).toBe("branding_custom");
    });

    it("should resume after last_completed_state=channel", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ last_completed_state: "channel" }),
      });
      expect(next.currentState).toBe("complete");
    });

    it("should infer provider state from workspace_configured + branding_set", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({
          workspace_configured: true,
          branding_set: true,
        }),
      });
      expect(next.currentState).toBe("provider");
    });

    it("should infer branding state from workspace_configured only", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ workspace_configured: true }),
      });
      expect(next.currentState).toBe("branding");
    });

    it("should preserve primary_color from status", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ primary_color: "#FF0000" }),
      });
      expect(next.context.primaryColor).toBe("#FF0000");
    });

    it("should default to welcome when no flags set", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus(),
      });
      expect(next.currentState).toBe("welcome");
    });

    it("should clear previous error on INIT", () => {
      const state = { ...makeState("loading"), error: "some_error" };
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus(),
      });
      expect(next.error).toBeNull();
    });
  });

  // ========================================================================
  // REPLY action — valid transitions
  // ========================================================================

  describe("REPLY action — valid transitions", () => {
    it("welcome → agent_identity on start", () => {
      const state = makeState("welcome");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "start",
      });
      expect(next.currentState).toBe("agent_identity");
    });

    it("naming → workspace_type on keep", () => {
      const state = makeState("naming");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "keep",
      });
      expect(next.currentState).toBe("workspace_type");
    });

    it("naming → naming_custom on customize", () => {
      const state = makeState("naming");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "customize",
      });
      expect(next.currentState).toBe("naming_custom");
    });

    it("workspace_type → branding on personal", () => {
      const state = makeState("workspace_type");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "personal",
      });
      expect(next.currentState).toBe("branding");
      expect(next.context.workspaceType).toBe("personal");
    });

    it("workspace_type → workspace_details on business", () => {
      const state = makeState("workspace_type");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "business",
      });
      expect(next.currentState).toBe("workspace_details");
      expect(next.context.workspaceType).toBe("business");
    });

    it("branding → provider on keep", () => {
      const state = makeState("branding");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "keep",
      });
      expect(next.currentState).toBe("provider");
    });

    it("branding → branding_custom on customize", () => {
      const state = makeState("branding");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "customize",
      });
      expect(next.currentState).toBe("branding_custom");
    });

    it("provider → provider_config on provider selection", () => {
      const state = makeState("provider");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "openrouter",
      });
      expect(next.currentState).toBe("provider_config");
      expect(next.context.selectedProvider).toBe("openrouter");
    });

    it("provider → channel on skip", () => {
      const state = makeState("provider");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "skip",
      });
      expect(next.currentState).toBe("channel");
    });

    it("channel → complete on channel selection", () => {
      const state = makeState("channel");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "telegram",
      });
      expect(next.currentState).toBe("complete");
      expect(next.context.selectedChannel).toBe("telegram");
      expect(next.context.onboardingComplete).toBe(true);
    });

    it("channel → complete on skip", () => {
      const state = makeState("channel");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "skip",
      });
      expect(next.currentState).toBe("complete");
      expect(next.context.onboardingComplete).toBe(true);
    });
  });

  // ========================================================================
  // REPLY action — text rejection (R2.4)
  // ========================================================================

  describe("REPLY action — text rejection", () => {
    it("should reject invalid reply on welcome state", () => {
      const state = makeState("welcome");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "random text",
      });
      expect(next.currentState).toBe("welcome");
      expect(next.error).toBe("not_understood");
    });

    it("should reject invalid reply on naming state", () => {
      const state = makeState("naming");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "something else",
      });
      expect(next.currentState).toBe("naming");
      expect(next.error).toBe("not_understood");
    });

    it("should reject invalid reply on workspace_type state", () => {
      const state = makeState("workspace_type");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "hybrid",
      });
      expect(next.currentState).toBe("workspace_type");
      expect(next.error).toBe("not_understood");
    });

    it("should reject invalid reply on branding state", () => {
      const state = makeState("branding");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "maybe",
      });
      expect(next.currentState).toBe("branding");
      expect(next.error).toBe("not_understood");
    });

    it("should reject invalid reply on provider state", () => {
      const state = makeState("provider");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "gemini",
      });
      expect(next.currentState).toBe("provider");
      expect(next.error).toBe("not_understood");
    });

    it("should reject invalid reply on channel state", () => {
      const state = makeState("channel");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "whatsapp",
      });
      expect(next.currentState).toBe("channel");
      expect(next.error).toBe("not_understood");
    });

    it("should clear error on subsequent valid reply", () => {
      const state = { ...makeState("welcome"), error: "not_understood" };
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "start",
      });
      expect(next.currentState).toBe("agent_identity");
      expect(next.error).toBeNull();
    });
  });

  // ========================================================================
  // INPUT action
  // ========================================================================

  describe("INPUT action", () => {
    it("naming_custom → workspace_type on agentName input", () => {
      const state = makeState("naming_custom");
      const next = onboardingReducer(state, {
        type: "INPUT",
        field: "agentName",
        value: "Conselheiro",
      });
      expect(next.currentState).toBe("workspace_type");
      expect(next.context.agentName).toBe("Conselheiro");
    });

    it("workspace_details → branding on accountName input", () => {
      const state = makeState("workspace_details");
      const next = onboardingReducer(state, {
        type: "INPUT",
        field: "accountName",
        value: "Vellus AI",
      });
      expect(next.currentState).toBe("branding");
      expect(next.context.accountName).toBe("Vellus AI");
    });

    it("branding_custom → provider on primaryColor input", () => {
      const state = makeState("branding_custom");
      const next = onboardingReducer(state, {
        type: "INPUT",
        field: "primaryColor",
        value: "#FF5733",
      });
      expect(next.currentState).toBe("provider");
      expect(next.context.primaryColor).toBe("#FF5733");
    });

    it("should not transition on wrong field for naming_custom", () => {
      const state = makeState("naming_custom");
      const next = onboardingReducer(state, {
        type: "INPUT",
        field: "accountName",
        value: "wrong",
      });
      expect(next.currentState).toBe("naming_custom");
    });

    it("should not transition on wrong field for workspace_details", () => {
      const state = makeState("workspace_details");
      const next = onboardingReducer(state, {
        type: "INPUT",
        field: "agentName",
        value: "wrong",
      });
      expect(next.currentState).toBe("workspace_details");
    });
  });

  // ========================================================================
  // SKIP action
  // ========================================================================

  describe("SKIP action", () => {
    it("provider → channel on skip", () => {
      const state = makeState("provider");
      const next = onboardingReducer(state, { type: "SKIP" });
      expect(next.currentState).toBe("channel");
    });

    it("channel → complete on skip", () => {
      const state = makeState("channel");
      const next = onboardingReducer(state, { type: "SKIP" });
      expect(next.currentState).toBe("complete");
      expect(next.context.onboardingComplete).toBe(true);
    });

    it("should not transition on skip in non-skippable state", () => {
      const state = makeState("welcome");
      const next = onboardingReducer(state, { type: "SKIP" });
      expect(next.currentState).toBe("welcome");
    });

    it("should not transition on skip in naming state", () => {
      const state = makeState("naming");
      const next = onboardingReducer(state, { type: "SKIP" });
      expect(next.currentState).toBe("naming");
    });

    it("should not transition on skip in branding state", () => {
      const state = makeState("branding");
      const next = onboardingReducer(state, { type: "SKIP" });
      expect(next.currentState).toBe("branding");
    });
  });

  // ========================================================================
  // TOOL_SUCCESS action
  // ========================================================================

  describe("TOOL_SUCCESS action", () => {
    it("provider_config → channel on validate_provider success", () => {
      const state = makeState("provider_config");
      const next = onboardingReducer(state, {
        type: "TOOL_SUCCESS",
        tool: "validate_provider",
      });
      expect(next.currentState).toBe("channel");
    });

    it("provider_config → channel on create_provider success", () => {
      const state = makeState("provider_config");
      const next = onboardingReducer(state, {
        type: "TOOL_SUCCESS",
        tool: "create_provider",
      });
      expect(next.currentState).toBe("channel");
    });

    it("should not transition on unknown tool in provider_config", () => {
      const state = makeState("provider_config");
      const next = onboardingReducer(state, {
        type: "TOOL_SUCCESS",
        tool: "unknown_tool",
      });
      expect(next.currentState).toBe("provider_config");
    });

    it("should not transition on tool_success in wrong state", () => {
      const state = makeState("welcome");
      const next = onboardingReducer(state, {
        type: "TOOL_SUCCESS",
        tool: "validate_provider",
      });
      expect(next.currentState).toBe("welcome");
    });
  });

  // ========================================================================
  // TOOL_ERROR action
  // ========================================================================

  describe("TOOL_ERROR action", () => {
    it("should set error without changing state", () => {
      const state = makeState("provider_config");
      const next = onboardingReducer(state, {
        type: "TOOL_ERROR",
        tool: "validate_provider",
        error: "Invalid API key",
      });
      expect(next.currentState).toBe("provider_config");
      expect(next.error).toBe("Invalid API key");
    });
  });

  // ========================================================================
  // Message accumulation in reducer (Blocker 2 fix)
  // ========================================================================

  describe("message accumulation — immutable", () => {
    it("REPLY should add user message to state.messages immutably", () => {
      const state = makeState("welcome");
      const originalMessages = state.messages;
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "start",
      });
      // Original messages array must NOT be mutated
      expect(originalMessages).toHaveLength(0);
      // New state should have user message + state messages
      expect(next.messages.length).toBeGreaterThan(0);
      expect(next.messages.some((m) => m.role === "user" && m.content === "start")).toBe(true);
    });

    it("INPUT should add user message to state.messages immutably", () => {
      const state = makeState("naming_custom");
      const originalMessages = state.messages;
      const next = onboardingReducer(state, {
        type: "INPUT",
        field: "agentName",
        value: "Jarvis",
      });
      expect(originalMessages).toHaveLength(0);
      expect(next.messages.some((m) => m.role === "user" && m.content === "Jarvis")).toBe(true);
    });

    it("INPUT with apiKey field should NOT add user message", () => {
      const state = makeState("provider_config", { selectedProvider: "openai" });
      const next = onboardingReducer(state, {
        type: "INPUT",
        field: "apiKey",
        value: "sk-secret-key",
      });
      expect(next.messages.every((m) => m.content !== "sk-secret-key")).toBe(true);
    });

    it("text rejection should still add user message", () => {
      const state = makeState("welcome");
      const next = onboardingReducer(state, {
        type: "REPLY",
        value: "random text",
      });
      expect(next.error).toBe("not_understood");
      expect(next.messages.some((m) => m.role === "user" && m.content === "random text")).toBe(true);
    });
  });

  // ========================================================================
  // Full flow — happy path
  // ========================================================================

  describe("full happy path flow", () => {
    it("should complete full onboarding with business workspace", () => {
      let state = makeState("loading");

      // INIT
      state = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus(),
      });
      expect(state.currentState).toBe("welcome");

      // Start → agent_identity
      state = onboardingReducer(state, { type: "REPLY", value: "start" });
      expect(state.currentState).toBe("agent_identity");

      // Select gender → workspace_type
      state = onboardingReducer(state, { type: "REPLY", value: "agent_male" });
      expect(state.currentState).toBe("workspace_type");

      // Business
      state = onboardingReducer(state, { type: "REPLY", value: "business" });
      expect(state.currentState).toBe("workspace_details");

      // Account name
      state = onboardingReducer(state, {
        type: "INPUT",
        field: "accountName",
        value: "Vellus",
      });
      expect(state.currentState).toBe("branding");

      // Keep branding
      state = onboardingReducer(state, { type: "REPLY", value: "keep" });
      expect(state.currentState).toBe("provider");

      // Select provider
      state = onboardingReducer(state, {
        type: "REPLY",
        value: "anthropic",
      });
      expect(state.currentState).toBe("provider_config");

      // Tool success
      state = onboardingReducer(state, {
        type: "TOOL_SUCCESS",
        tool: "validate_provider",
      });
      expect(state.currentState).toBe("channel");

      // Skip channel
      state = onboardingReducer(state, { type: "REPLY", value: "skip" });
      expect(state.currentState).toBe("complete");
      expect(state.context.onboardingComplete).toBe(true);
    });

    it("should complete flow with custom name and personal workspace", () => {
      let state = makeState("loading");

      state = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus(),
      });
      // Start → agent_identity → customize → agent_identity_custom
      state = onboardingReducer(state, { type: "REPLY", value: "start" });
      expect(state.currentState).toBe("agent_identity");
      state = onboardingReducer(state, { type: "REPLY", value: "customize" });
      expect(state.currentState).toBe("agent_identity_custom");

      state = onboardingReducer(state, {
        type: "INPUT",
        field: "agentName",
        value: "Jarvis",
      });
      expect(state.currentState).toBe("workspace_type");
      expect(state.context.agentName).toBe("Jarvis");

      // Personal — skips workspace_details
      state = onboardingReducer(state, { type: "REPLY", value: "personal" });
      expect(state.currentState).toBe("branding");

      // Custom color
      state = onboardingReducer(state, { type: "REPLY", value: "customize" });
      expect(state.currentState).toBe("branding_custom");

      state = onboardingReducer(state, {
        type: "INPUT",
        field: "primaryColor",
        value: "#22C55E",
      });
      expect(state.currentState).toBe("provider");

      // Skip provider
      state = onboardingReducer(state, { type: "REPLY", value: "skip" });
      expect(state.currentState).toBe("channel");

      // Select channel
      state = onboardingReducer(state, { type: "REPLY", value: "discord" });
      expect(state.currentState).toBe("complete");
      expect(state.context.selectedChannel).toBe("discord");
    });
  });
});

// ---------------------------------------------------------------------------
// getMessagesForState
// ---------------------------------------------------------------------------

describe("getMessagesForState", () => {
  beforeEach(() => {
    _resetMessageCounter();
  });

  it("should return welcome messages with displayName and agentName (neutral gender)", () => {
    const msgs = getMessagesForState("welcome", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(2);
    expect(msgs[0]!.role).toBe("assistant");
    expect(msgs[0]!.content).toContain("onboarding.welcome.greetingNeutral");
    expect(msgs[0]!.content).toContain("Milton");
    expect(msgs[0]!.content).toContain("Imediato");
  });

  it("should return welcome messages with male greeting when gender is male", () => {
    const ctx = { ...INITIAL_CONTEXT, gender: "male" as Gender };
    const msgs = getMessagesForState("welcome", mockT, ctx);
    expect(msgs[0]!.content).toContain("onboarding.welcome.greetingMale");
    expect(msgs[0]!.content).toContain("Milton");
  });

  it("should return welcome messages with female greeting when gender is female", () => {
    const ctx = { ...INITIAL_CONTEXT, gender: "female" as Gender };
    const msgs = getMessagesForState("welcome", mockT, ctx);
    expect(msgs[0]!.content).toContain("onboarding.welcome.greetingFemale");
    expect(msgs[0]!.content).toContain("Milton");
  });

  it("should return welcome messages with neutral greeting when gender is other", () => {
    const ctx = { ...INITIAL_CONTEXT, gender: "other" as Gender };
    const msgs = getMessagesForState("welcome", mockT, ctx);
    expect(msgs[0]!.content).toContain("onboarding.welcome.greetingNeutral");
  });

  it("should return naming messages with quick replies", () => {
    const msgs = getMessagesForState("naming", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.quickReplies).toHaveLength(2);
    expect(msgs[0]!.quickReplies?.[0]?.value).toBe("keep");
    expect(msgs[0]!.quickReplies?.[1]?.value).toBe("customize");
  });

  it("should return naming_custom messages with input field", () => {
    const msgs = getMessagesForState("naming_custom", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.inputField?.type).toBe("text");
  });

  it("should return workspace_type messages with personal and business", () => {
    const msgs = getMessagesForState("workspace_type", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.quickReplies).toHaveLength(2);
    expect(msgs[0]!.quickReplies?.[0]?.value).toBe("personal");
    expect(msgs[0]!.quickReplies?.[1]?.value).toBe("business");
  });

  it("should return workspace_details messages with input field", () => {
    const msgs = getMessagesForState(
      "workspace_details",
      mockT,
      INITIAL_CONTEXT,
    );
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.inputField?.type).toBe("text");
  });

  it("should return branding messages with keep and customize", () => {
    const msgs = getMessagesForState("branding", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.quickReplies).toHaveLength(2);
  });

  it("should return branding_custom messages with color input", () => {
    const msgs = getMessagesForState(
      "branding_custom",
      mockT,
      INITIAL_CONTEXT,
    );
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.inputField?.type).toBe("color");
  });

  it("should return provider messages with provider options and skip", () => {
    const msgs = getMessagesForState("provider", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(1);
    const replies = msgs[0]!.quickReplies;
    expect(replies).toBeDefined();
    expect(replies?.find((r) => r.value === "skip")?.variant).toBe("skip");
    expect(replies?.find((r) => r.value === "openrouter")).toBeDefined();
  });

  it("should return provider_config messages with password input", () => {
    const ctx = { ...INITIAL_CONTEXT, selectedProvider: "anthropic" };
    const msgs = getMessagesForState("provider_config", mockT, ctx);
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.inputField?.type).toBe("password");
  });

  it("should return channel messages with channel options and skip", () => {
    const msgs = getMessagesForState("channel", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(1);
    const replies = msgs[0]!.quickReplies;
    expect(replies?.find((r) => r.value === "telegram")).toBeDefined();
    expect(replies?.find((r) => r.value === "skip")?.variant).toBe("skip");
  });

  it("should return complete messages with dashboard button", () => {
    const msgs = getMessagesForState("complete", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(2);
    expect(msgs[1]!.quickReplies?.[0]?.value).toBe("dashboard");
  });

  it("should return empty array for loading state", () => {
    const msgs = getMessagesForState("loading", mockT, INITIAL_CONTEXT);
    expect(msgs).toHaveLength(0);
  });

  it("should generate unique message IDs", () => {
    const msgs = getMessagesForState("welcome", mockT, INITIAL_CONTEXT);
    const ids = msgs.map((m) => m.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

// ---------------------------------------------------------------------------
// greetingKeyForGender
// ---------------------------------------------------------------------------

describe("greetingKeyForGender", () => {
  it("returns greetingMale for male", () => {
    expect(greetingKeyForGender("male")).toBe("onboarding.welcome.greetingMale");
  });

  it("returns greetingFemale for female", () => {
    expect(greetingKeyForGender("female")).toBe("onboarding.welcome.greetingFemale");
  });

  it("returns greetingNeutral for other", () => {
    expect(greetingKeyForGender("other")).toBe("onboarding.welcome.greetingNeutral");
  });

  it("returns greetingNeutral for null", () => {
    expect(greetingKeyForGender(null)).toBe("onboarding.welcome.greetingNeutral");
  });
});
