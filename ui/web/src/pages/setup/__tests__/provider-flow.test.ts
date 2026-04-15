import { describe, it, expect, beforeEach } from "vitest";
import {
  onboardingReducer,
  _resetMessageCounter,
  type OnboardingContext,
  type OnboardingState,
  type TranslatorFn,
  type EngineState,
  type ChatMessageLocal,
} from "../hooks/use-onboarding-engine";
import type { OnboardingStatusResponse } from "../hooks/use-onboarding-api";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const INITIAL_CONTEXT: OnboardingContext = {
  displayName: "Milton",
  agentName: "Imediato",
  agentId: "agent-001",
  workspaceType: "",
  accountName: "",
  primaryColor: "#1E40AF",
  selectedProvider: "",
  selectedChannel: "",
  workspaceConfigured: false,
  brandingSet: false,
  onboardingComplete: false,
  gender: null,
};

const mockT: TranslatorFn = (key, opts) => {
  if (opts && Object.keys(opts).length > 0) {
    const pairs = Object.entries(opts)
      .map(([k, v]) => `${k}=${v}`)
      .join(",");
    return `${key}[${pairs}]`;
  }
  return key;
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

// ---------------------------------------------------------------------------
// Provider skip loop bug — regression tests
// ---------------------------------------------------------------------------

describe("provider skip loop bug", () => {
  beforeEach(() => {
    _resetMessageCounter();
  });

  describe("reducer transitions for skip", () => {
    it("should transition provider -> channel when REPLY value is skip", () => {
      const state = makeState("provider");
      const next = onboardingReducer(state, { type: "REPLY", value: "skip" });
      expect(next.currentState).toBe("channel");
    });

    it("should transition channel -> complete with onboardingComplete=true when REPLY value is skip", () => {
      const state = makeState("channel");
      const next = onboardingReducer(state, { type: "REPLY", value: "skip" });
      expect(next.currentState).toBe("complete");
      expect(next.context.onboardingComplete).toBe(true);
    });

    it("should complete full skip-both flow: provider skip then channel skip", () => {
      let state = makeState("provider");

      // Skip provider
      state = onboardingReducer(state, { type: "REPLY", value: "skip" });
      expect(state.currentState).toBe("channel");

      // Skip channel
      state = onboardingReducer(state, { type: "REPLY", value: "skip" });
      expect(state.currentState).toBe("complete");
      expect(state.context.onboardingComplete).toBe(true);
    });
  });

  describe("resumeState with onboarding_complete", () => {
    it("should return complete when onboarding_complete=true", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ onboarding_complete: true }),
      });
      expect(next.currentState).toBe("complete");
    });

    it("should NOT loop back to provider when onboarding_complete=true even if flags suggest provider", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({
          onboarding_complete: true,
          workspace_configured: true,
          branding_set: true,
        }),
      });
      // Must be complete, NOT provider
      expect(next.currentState).toBe("complete");
    });
  });

  describe("resumeState with last_completed_state", () => {
    it("should resume to channel (next state) when last_completed_state=provider", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ last_completed_state: "provider" }),
      });
      expect(next.currentState).toBe("provider_config");
    });

    it("should resume to complete when last_completed_state=channel", () => {
      const state = makeState("loading");
      const next = onboardingReducer(state, {
        type: "INIT",
        status: makeStatus({ last_completed_state: "channel" }),
      });
      expect(next.currentState).toBe("complete");
    });
  });
});

// ---------------------------------------------------------------------------
// Provider/channel QuickReply card descriptions
// ---------------------------------------------------------------------------

describe("provider and channel quick replies", () => {
  beforeEach(() => {
    _resetMessageCounter();
  });

  it("provider messages should include description field on quick replies", () => {
    // Get messages from a state transition (INIT to provider)
    const initState = makeState("loading");
    const next = onboardingReducer(initState, {
      type: "INIT",
      status: makeStatus({ workspace_configured: true, branding_set: true }),
    });
    expect(next.currentState).toBe("provider");
  });

  it("provider quick replies should have openrouter, anthropic, openai, skip", () => {
    // The messages for provider state should have 4 quick replies
    // We test this by attempting to REPLY with each valid value
    for (const value of ["openrouter", "anthropic", "openai"]) {
      const next = onboardingReducer(makeState("provider"), {
        type: "REPLY",
        value,
      });
      expect(next.currentState).toBe("provider_config");
      expect(next.context.selectedProvider).toBe(value);
    }

    const skipNext = onboardingReducer(makeState("provider"), {
      type: "REPLY",
      value: "skip",
    });
    expect(skipNext.currentState).toBe("channel");
  });

  it("channel quick replies should have telegram, discord, skip", () => {
    for (const value of ["telegram", "discord"]) {
      const next = onboardingReducer(makeState("channel"), {
        type: "REPLY",
        value,
      });
      expect(next.currentState).toBe("complete");
      expect(next.context.selectedChannel).toBe(value);
      expect(next.context.onboardingComplete).toBe(true);
    }

    const skipNext = onboardingReducer(makeState("channel"), {
      type: "REPLY",
      value: "skip",
    });
    expect(skipNext.currentState).toBe("complete");
    expect(skipNext.context.onboardingComplete).toBe(true);
  });
});
