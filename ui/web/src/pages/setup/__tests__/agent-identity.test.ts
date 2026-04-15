import { describe, it, expect, beforeEach } from "vitest";
import * as fc from "fast-check";
import {
  onboardingReducer,
  _resetMessageCounter,
  type OnboardingContext,
  type OnboardingState,
  type TranslatorFn,
  type EngineState,
  type ChatMessageLocal,
  NAUTICAL_NAMES_MALE,
  NAUTICAL_NAMES_FEMALE,
} from "../hooks/use-onboarding-engine";
import type { OnboardingStatusResponse } from "../hooks/use-onboarding-api";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const INITIAL_CONTEXT: OnboardingContext = {
  displayName: "Milton",
  agentName: "Imediato",
  agentId: "agent-001",
  agentGender: null,
  gender: null,
  workspaceType: "",
  accountName: "",
  primaryColor: "#1E40AF",
  selectedProvider: "",
  selectedChannel: "",
  workspaceConfigured: false,
  brandingSet: false,
  onboardingComplete: false,
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
// Tests
// ---------------------------------------------------------------------------

describe("agent_identity state", () => {
  beforeEach(() => {
    _resetMessageCounter();
  });

  // Test 1: welcome → REPLY "start" → agent_identity
  it("should transition welcome → agent_identity when REPLY is 'start'", () => {
    const state = makeState("welcome");
    const next = onboardingReducer(state, { type: "REPLY", value: "start" });
    expect(next.currentState).toBe("agent_identity");
  });

  // Test 2: agent_identity → REPLY "agent_male"
  it("should transition agent_identity → workspace_type with agentGender=male when REPLY is 'agent_male'", () => {
    const state = makeState("agent_identity");
    const next = onboardingReducer(state, { type: "REPLY", value: "agent_male" });
    expect(next.currentState).toBe("workspace_type");
    expect(next.context.agentGender).toBe("male");
    expect(next.context.agentName).toBeTruthy();
    expect(next.context.agentName.length).toBeGreaterThan(0);
  });

  // Test 3: agent_identity → REPLY "agent_female"
  it("should transition agent_identity → workspace_type with agentGender=female when REPLY is 'agent_female'", () => {
    const state = makeState("agent_identity");
    const next = onboardingReducer(state, { type: "REPLY", value: "agent_female" });
    expect(next.currentState).toBe("workspace_type");
    expect(next.context.agentGender).toBe("female");
    expect(next.context.agentName).toBeTruthy();
    expect(next.context.agentName.length).toBeGreaterThan(0);
  });

  // Test 4: agent_identity → REPLY "agent_other"
  it("should transition agent_identity → workspace_type with agentGender=other when REPLY is 'agent_other'", () => {
    const state = makeState("agent_identity");
    const next = onboardingReducer(state, { type: "REPLY", value: "agent_other" });
    expect(next.currentState).toBe("workspace_type");
    expect(next.context.agentGender).toBe("other");
    expect(next.context.agentName).toBeTruthy();
    expect(next.context.agentName.length).toBeGreaterThan(0);
  });

  // Test 5: agent_identity → REPLY "customize" → agent_identity_custom
  it("should transition agent_identity → agent_identity_custom when REPLY is 'customize'", () => {
    const state = makeState("agent_identity");
    const next = onboardingReducer(state, { type: "REPLY", value: "customize" });
    expect(next.currentState).toBe("agent_identity_custom");
  });

  // Test 6: agent_identity_custom → INPUT field=agentName → workspace_type
  it("should transition agent_identity_custom → workspace_type with custom agentName on INPUT", () => {
    const state = makeState("agent_identity_custom");
    const next = onboardingReducer(state, {
      type: "INPUT",
      field: "agentName",
      value: "Capitão Estrela",
    });
    expect(next.currentState).toBe("workspace_type");
    expect(next.context.agentName).toBe("Capitão Estrela");
  });

  // Test 7: auto-generated name for male must be one of the male nautical names
  it("should auto-generate an agentName from male nautical names list when REPLY is 'agent_male'", () => {
    const state = makeState("agent_identity");
    const next = onboardingReducer(state, { type: "REPLY", value: "agent_male" });
    expect(NAUTICAL_NAMES_MALE).toContain(next.context.agentName);
  });

  // Test 8: auto-generated name for female must be one of the female nautical names
  it("should auto-generate an agentName from female nautical names list when REPLY is 'agent_female'", () => {
    const state = makeState("agent_identity");
    const next = onboardingReducer(state, { type: "REPLY", value: "agent_female" });
    expect(NAUTICAL_NAMES_FEMALE).toContain(next.context.agentName);
  });

  // Test 9: resumeState with last_completed_state: "agent_identity"
  it("should resume to agent_identity_custom when last_completed_state is 'agent_identity'", () => {
    const state = makeState("loading");
    const next = onboardingReducer(state, {
      type: "INIT",
      status: makeStatus({ last_completed_state: "agent_identity" }),
    });
    expect(next.currentState).toBe("agent_identity_custom");
  });

  // Test 10: resumeState with last_completed_state: "agent_identity_custom"
  it("should resume to workspace_type when last_completed_state is 'agent_identity_custom'", () => {
    const state = makeState("loading");
    const next = onboardingReducer(state, {
      type: "INIT",
      status: makeStatus({ last_completed_state: "agent_identity_custom" }),
    });
    expect(next.currentState).toBe("workspace_type");
  });
});

// ---------------------------------------------------------------------------
// PBT — Property-Based Tests
// ---------------------------------------------------------------------------

describe("agent_identity PBT", () => {
  beforeEach(() => {
    _resetMessageCounter();
  });

  it("should always transition to workspace_type with a non-empty agentName for any gender reply", () => {
    const genderReplies = ["agent_male", "agent_female", "agent_other"] as const;

    fc.assert(
      fc.property(fc.constantFrom(...genderReplies), (replyValue) => {
        _resetMessageCounter();
        const state = makeState("agent_identity");
        const next = onboardingReducer(state, { type: "REPLY", value: replyValue });
        return (
          next.currentState === "workspace_type" &&
          typeof next.context.agentName === "string" &&
          next.context.agentName.length > 0
        );
      }),
      { numRuns: 50 },
    );
  });

  it("should always set agentGender correctly for each gender reply", () => {
    const genderMap: Record<string, "male" | "female" | "other"> = {
      agent_male: "male",
      agent_female: "female",
      agent_other: "other",
    };

    fc.assert(
      fc.property(
        fc.constantFrom("agent_male", "agent_female", "agent_other"),
        (replyValue) => {
          _resetMessageCounter();
          const state = makeState("agent_identity");
          const next = onboardingReducer(state, { type: "REPLY", value: replyValue });
          return next.context.agentGender === genderMap[replyValue];
        },
      ),
      { numRuns: 50 },
    );
  });

  it("should generate male names only from NAUTICAL_NAMES_MALE for agent_male replies", () => {
    fc.assert(
      fc.property(fc.constant("agent_male"), (replyValue) => {
        _resetMessageCounter();
        const state = makeState("agent_identity");
        const next = onboardingReducer(state, { type: "REPLY", value: replyValue });
        return NAUTICAL_NAMES_MALE.includes(next.context.agentName);
      }),
      { numRuns: 30 },
    );
  });

  it("should generate female names only from NAUTICAL_NAMES_FEMALE for agent_female replies", () => {
    fc.assert(
      fc.property(fc.constant("agent_female"), (replyValue) => {
        _resetMessageCounter();
        const state = makeState("agent_identity");
        const next = onboardingReducer(state, { type: "REPLY", value: replyValue });
        return NAUTICAL_NAMES_FEMALE.includes(next.context.agentName);
      }),
      { numRuns: 30 },
    );
  });
});
