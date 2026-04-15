import { useCallback, useReducer } from "react";
import { useTranslation } from "react-i18next";
import type { OnboardingStatusResponse } from "./use-onboarding-api";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type OnboardingState =
  | "loading"
  | "welcome"
  | "naming"
  | "naming_custom"
  | "workspace_type"
  | "workspace_details"
  | "branding"
  | "branding_custom"
  | "provider"
  | "provider_config"
  | "channel"
  | "complete";

export type Gender = "male" | "female" | "other" | null;

export interface OnboardingContext {
  displayName: string;
  agentName: string;
  agentId: string;
  gender: Gender;
  workspaceType: string;
  accountName: string;
  primaryColor: string;
  selectedProvider: string;
  selectedChannel: string;
  workspaceConfigured: boolean;
  brandingSet: boolean;
  onboardingComplete: boolean;
}

export type OnboardingAction =
  | { type: "INIT"; status: OnboardingStatusResponse }
  | { type: "REPLY"; value: string }
  | { type: "INPUT"; field: string; value: string }
  | { type: "SKIP" }
  | { type: "TOOL_SUCCESS"; tool: string }
  | { type: "TOOL_ERROR"; tool: string; error: string }
  | { type: "_SET_TRANSLATOR"; _translator: TranslatorFn };

export interface ChatMessageLocal {
  id: string;
  role: "assistant" | "user";
  content: string;
  timestamp: number;
  quickReplies?: QuickReply[];
  inputField?: InputFieldSpec;
}

export interface QuickReply {
  label: string;
  value: string;
  variant?: "default" | "skip";
  description?: string;
}

export interface InputFieldSpec {
  type: "text" | "password" | "color";
  placeholder?: string;
  validation?: (value: string) => string | null;
}

// ---------------------------------------------------------------------------
// Internal engine state
// ---------------------------------------------------------------------------

export interface EngineState {
  currentState: OnboardingState;
  context: OnboardingContext;
  messages: ChatMessageLocal[];
  error: string | null;
  /** Internal: translator function injected by the hook for reducer use. */
  _translator?: TranslatorFn;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

let messageCounter = 0;
function nextId(): string {
  messageCounter += 1;
  return `msg-${messageCounter}`;
}

/** Reset counter — useful for tests. */
export function _resetMessageCounter(): void {
  messageCounter = 0;
}

function makeAssistant(
  content: string,
  quickReplies?: QuickReply[],
  inputField?: InputFieldSpec,
): ChatMessageLocal {
  return {
    id: nextId(),
    role: "assistant",
    content,
    timestamp: Date.now(),
    quickReplies,
    inputField,
  };
}

function makeUser(content: string): ChatMessageLocal {
  return {
    id: nextId(),
    role: "user",
    content,
    timestamp: Date.now(),
  };
}

const INITIAL_CONTEXT: OnboardingContext = {
  displayName: "",
  agentName: "Imediato",
  agentId: "",
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

// ---------------------------------------------------------------------------
// Message generators — these are pure functions that take a t() translator.
// The hook calls them after constructing the translator. This avoids calling
// hooks inside the reducer.
// ---------------------------------------------------------------------------

export type TranslatorFn = (key: string, opts?: Record<string, unknown>) => string;

/**
 * Returns the i18n greeting key based on gender.
 * - "male"   → "onboarding.welcome.greetingMale"
 * - "female" → "onboarding.welcome.greetingFemale"
 * - "other" / null → "onboarding.welcome.greetingNeutral"
 */
export function greetingKeyForGender(gender: Gender): string {
  switch (gender) {
    case "male":
      return "onboarding.welcome.greetingMale";
    case "female":
      return "onboarding.welcome.greetingFemale";
    default:
      return "onboarding.welcome.greetingNeutral";
  }
}

export function welcomeMessages(
  t: TranslatorFn,
  ctx: OnboardingContext,
): ChatMessageLocal[] {
  const greetingKey = greetingKeyForGender(ctx.gender);
  return [
    makeAssistant(
      t(greetingKey, {
        displayName: ctx.displayName,
        agentName: ctx.agentName,
      }),
    ),
    makeAssistant(t("onboarding.welcome.intro"), [
      { label: t("onboarding.welcome.start"), value: "start" },
    ]),
  ];
}

export function namingMessages(
  t: TranslatorFn,
  ctx: OnboardingContext,
): ChatMessageLocal[] {
  return [
    makeAssistant(
      t("onboarding.naming.question", { agentName: ctx.agentName }),
      [
        {
          label: t("onboarding.naming.keepDefault", { agentName: ctx.agentName }),
          value: "keep",
        },
        { label: t("onboarding.naming.customize"), value: "customize" },
      ],
    ),
  ];
}

export function namingCustomMessages(
  t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant(undefined as unknown as string, undefined, {
      type: "text",
      placeholder: t("onboarding.naming.inputPlaceholder"),
      validation: (v: string) =>
        v.trim().length === 0 ? t("onboarding.error.notUnderstood") : null,
    }),
  ];
}

export function workspaceTypeMessages(
  t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant(t("onboarding.workspace.typeQuestion"), [
      { label: t("onboarding.workspace.personal"), value: "personal" },
      { label: t("onboarding.workspace.business"), value: "business" },
    ]),
  ];
}

export function workspaceDetailsMessages(
  t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant(t("onboarding.workspace.nameQuestion"), undefined, {
      type: "text",
      placeholder: t("onboarding.workspace.namePlaceholder"),
      validation: (v: string) =>
        v.trim().length === 0 ? t("onboarding.error.notUnderstood") : null,
    }),
  ];
}

export function brandingMessages(
  t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant(t("onboarding.branding.question"), [
      { label: t("onboarding.branding.keepDefault"), value: "keep" },
      { label: t("onboarding.branding.customize"), value: "customize" },
    ]),
  ];
}

export function brandingCustomMessages(
  _t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant("", undefined, {
      type: "color",
    }),
  ];
}

export function providerMessages(
  t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant(t("onboarding.provider.question"), [
      {
        label: "OpenRouter",
        value: "openrouter",
        description: t("onboarding.provider.openrouterDesc"),
      },
      {
        label: "Anthropic",
        value: "anthropic",
        description: t("onboarding.provider.anthropicDesc"),
      },
      {
        label: "OpenAI",
        value: "openai",
        description: t("onboarding.provider.openaiDesc"),
      },
      {
        label: t("onboarding.provider.skip"),
        value: "skip",
        variant: "skip",
        description: t("onboarding.provider.skipDesc"),
      },
    ]),
  ];
}

export function providerConfigMessages(
  t: TranslatorFn,
  ctx: OnboardingContext,
): ChatMessageLocal[] {
  return [
    makeAssistant(
      t("onboarding.provider.apiKeyLabel", { provider: ctx.selectedProvider }),
      undefined,
      {
        type: "password",
        placeholder: t("onboarding.provider.apiKeyPlaceholder"),
        validation: (v: string) =>
          v.trim().length === 0 ? t("onboarding.error.notUnderstood") : null,
      },
    ),
  ];
}

export function channelMessages(
  t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant(t("onboarding.channel.question"), [
      {
        label: "Telegram",
        value: "telegram",
        description: t("onboarding.channel.telegramDesc"),
      },
      {
        label: "Discord",
        value: "discord",
        description: t("onboarding.channel.discordDesc"),
      },
      {
        label: t("onboarding.channel.skip"),
        value: "skip",
        variant: "skip",
        description: t("onboarding.channel.skipDesc"),
      },
    ]),
  ];
}

export function completeMessages(
  t: TranslatorFn,
): ChatMessageLocal[] {
  return [
    makeAssistant(t("onboarding.complete.title")),
    makeAssistant(t("onboarding.complete.summary"), [
      { label: t("onboarding.complete.goToDashboard"), value: "dashboard" },
    ]),
  ];
}

export function errorNotUnderstoodMessage(
  t: TranslatorFn,
): ChatMessageLocal {
  return makeAssistant(t("onboarding.error.notUnderstood"));
}

export function toolErrorMessage(
  t: TranslatorFn,
  error: string,
): ChatMessageLocal {
  return makeAssistant(t("onboarding.error.toolFailed", { error }), [
    { label: t("onboarding.error.retry"), value: "retry" },
  ]);
}

// ---------------------------------------------------------------------------
// State machine helper: get messages for a given state
// ---------------------------------------------------------------------------

export function getMessagesForState(
  state: OnboardingState,
  t: TranslatorFn,
  ctx: OnboardingContext,
): ChatMessageLocal[] {
  switch (state) {
    case "welcome":
      return welcomeMessages(t, ctx);
    case "naming":
      return namingMessages(t, ctx);
    case "naming_custom":
      return namingCustomMessages(t);
    case "workspace_type":
      return workspaceTypeMessages(t);
    case "workspace_details":
      return workspaceDetailsMessages(t);
    case "branding":
      return brandingMessages(t);
    case "branding_custom":
      return brandingCustomMessages(t);
    case "provider":
      return providerMessages(t);
    case "provider_config":
      return providerConfigMessages(t, ctx);
    case "channel":
      return channelMessages(t);
    case "complete":
      return completeMessages(t);
    default:
      return [];
  }
}

// ---------------------------------------------------------------------------
// Quick reply values per state — used for text rejection
// ---------------------------------------------------------------------------

function validReplies(state: OnboardingState): string[] | null {
  switch (state) {
    case "welcome":
      return ["start"];
    case "naming":
      return ["keep", "customize"];
    case "workspace_type":
      return ["personal", "business"];
    case "branding":
      return ["keep", "customize"];
    case "provider":
      return ["openrouter", "anthropic", "openai", "skip"];
    case "channel":
      return ["telegram", "discord", "skip"];
    default:
      return null; // states with input fields accept free text
  }
}

// ---------------------------------------------------------------------------
// Resume logic
// ---------------------------------------------------------------------------

function resumeState(status: OnboardingStatusResponse): OnboardingState {
  if (status.onboarding_complete) return "complete";

  if (status.last_completed_state) {
    const stateOrder: OnboardingState[] = [
      "welcome",
      "naming",
      "naming_custom",
      "workspace_type",
      "workspace_details",
      "branding",
      "branding_custom",
      "provider",
      "provider_config",
      "channel",
      "complete",
    ];
    const idx = stateOrder.indexOf(status.last_completed_state as OnboardingState);
    if (idx >= 0 && idx < stateOrder.length - 1) {
      return stateOrder[idx + 1]!;
    }
  }

  // Infer from flags
  if (status.workspace_configured && status.branding_set) return "provider";
  if (status.workspace_configured) return "branding";

  return "welcome";
}

// ---------------------------------------------------------------------------
// Reducer (pure — no hooks)
// ---------------------------------------------------------------------------

export function onboardingReducer(
  state: EngineState,
  action: OnboardingAction,
): EngineState {
  switch (action.type) {
    case "INIT": {
      const nextState = resumeState(action.status);
      return {
        ...state,
        currentState: nextState,
        context: {
          ...state.context,
          workspaceConfigured: action.status.workspace_configured,
          brandingSet: action.status.branding_set,
          onboardingComplete: action.status.onboarding_complete,
          primaryColor: action.status.primary_color ?? state.context.primaryColor,
        },
        error: null,
      };
    }

    case "REPLY": {
      // Record user message immutably
      const userMsg = makeUser(action.value);
      const withUserMsg = { ...state, messages: [...state.messages, userMsg] };

      const allowed = validReplies(state.currentState);
      if (allowed && !allowed.includes(action.value)) {
        // Text rejection — state doesn't change
        return {
          ...withUserMsg,
          error: "not_understood",
        };
      }

      // Snapshot current state messages into history before transition
      const currentMsgs = getMessagesForState(
        state.currentState,
        state._translator!,
        state.context,
      );
      const existingIds = new Set(withUserMsg.messages.map((m) => m.id));
      const newStateMsgs = currentMsgs.filter((m) => !existingIds.has(m.id));

      const stateWithHistory = {
        ...withUserMsg,
        messages: [...withUserMsg.messages, ...newStateMsgs],
      };

      return handleReply(stateWithHistory, action.value);
    }

    case "INPUT": {
      // Don't record password inputs (API keys)
      const inputMessages =
        action.field !== "apiKey"
          ? [...state.messages, makeUser(action.value)]
          : [...state.messages];

      // Snapshot current state messages into history before transition
      const currentMsgs = getMessagesForState(
        state.currentState,
        state._translator!,
        state.context,
      );
      const existingIds = new Set(inputMessages.map((m) => m.id));
      const newStateMsgs = currentMsgs.filter((m) => !existingIds.has(m.id));

      const stateWithHistory = {
        ...state,
        messages: [...inputMessages, ...newStateMsgs],
      };

      return handleInput(stateWithHistory, action.field, action.value);
    }

    case "SKIP": {
      return handleSkip(state);
    }

    case "TOOL_SUCCESS": {
      return handleToolSuccess(state, action.tool);
    }

    case "TOOL_ERROR": {
      return {
        ...state,
        error: action.error,
      };
    }

    case "_SET_TRANSLATOR": {
      return {
        ...state,
        _translator: action._translator,
      };
    }

    default:
      return state;
  }
}

function handleReply(state: EngineState, value: string): EngineState {
  const { currentState, context } = state;

  switch (currentState) {
    case "welcome":
      if (value === "start") {
        return { ...state, currentState: "naming", error: null };
      }
      return state;

    case "naming":
      if (value === "keep") {
        return { ...state, currentState: "workspace_type", error: null };
      }
      if (value === "customize") {
        return { ...state, currentState: "naming_custom", error: null };
      }
      return state;

    case "workspace_type":
      if (value === "personal") {
        return {
          ...state,
          currentState: "branding",
          context: { ...context, workspaceType: "personal" },
          error: null,
        };
      }
      if (value === "business") {
        return {
          ...state,
          currentState: "workspace_details",
          context: { ...context, workspaceType: "business" },
          error: null,
        };
      }
      return state;

    case "branding":
      if (value === "keep") {
        return { ...state, currentState: "provider", error: null };
      }
      if (value === "customize") {
        return { ...state, currentState: "branding_custom", error: null };
      }
      return state;

    case "provider":
      if (value === "skip") {
        return { ...state, currentState: "channel", error: null };
      }
      // Provider selected
      return {
        ...state,
        currentState: "provider_config",
        context: { ...context, selectedProvider: value },
        error: null,
      };

    case "channel":
      if (value === "skip") {
        return {
          ...state,
          currentState: "complete",
          context: { ...context, onboardingComplete: true },
          error: null,
        };
      }
      // Channel selected
      return {
        ...state,
        currentState: "complete",
        context: {
          ...context,
          selectedChannel: value,
          onboardingComplete: true,
        },
        error: null,
      };

    default:
      return state;
  }
}

function handleInput(
  state: EngineState,
  field: string,
  value: string,
): EngineState {
  const { currentState, context } = state;

  switch (currentState) {
    case "naming_custom":
      if (field === "agentName") {
        return {
          ...state,
          currentState: "workspace_type",
          context: { ...context, agentName: value },
          error: null,
        };
      }
      return state;

    case "workspace_details":
      if (field === "accountName") {
        return {
          ...state,
          currentState: "branding",
          context: { ...context, accountName: value },
          error: null,
        };
      }
      return state;

    case "branding_custom":
      if (field === "primaryColor") {
        return {
          ...state,
          currentState: "provider",
          context: { ...context, primaryColor: value },
          error: null,
        };
      }
      return state;

    default:
      return state;
  }
}

function handleSkip(state: EngineState): EngineState {
  const { currentState, context } = state;

  switch (currentState) {
    case "provider":
      return { ...state, currentState: "channel", error: null };

    case "channel":
      return {
        ...state,
        currentState: "complete",
        context: { ...context, onboardingComplete: true },
        error: null,
      };

    default:
      return state;
  }
}

function handleToolSuccess(state: EngineState, tool: string): EngineState {
  const { currentState } = state;

  switch (currentState) {
    case "provider_config":
      if (tool === "validate_provider" || tool === "create_provider" || tool === "oauth_provider") {
        return { ...state, currentState: "channel", error: null };
      }
      return state;

    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

const INITIAL_ENGINE_STATE: EngineState = {
  currentState: "loading",
  context: INITIAL_CONTEXT,
  messages: [],
  error: null,
};

export interface OnboardingEngine {
  messages: ChatMessageLocal[];
  currentState: OnboardingState;
  context: OnboardingContext;
  dispatch: (action: OnboardingAction) => void;
  loading: boolean;
  error: string | null;
}

export function useOnboardingEngine(
  initialContext?: Partial<OnboardingContext>,
): OnboardingEngine {
  const { t } = useTranslation("setup");

  const initial: EngineState = {
    ...INITIAL_ENGINE_STATE,
    context: { ...INITIAL_CONTEXT, ...initialContext },
    _translator: t,
  };

  const [state, rawDispatch] = useReducer(onboardingReducer, initial);

  // Keep the translator reference up-to-date in state for the reducer
  // (language could change). We inject it via a wrapper dispatch.
  const dispatch = useCallback(
    (action: OnboardingAction) => {
      // Inject current translator into state before dispatching.
      // The reducer reads _translator for message generation.
      rawDispatch({ type: "_SET_TRANSLATOR", _translator: t });
      rawDispatch(action);
    },
    [rawDispatch, t],
  );

  // Build messages reactively based on current state
  const stateMessages =
    state.currentState === "loading"
      ? []
      : getMessagesForState(state.currentState, t, state.context);

  // Combine history messages with current state messages
  const displayMessages = [...state.messages, ...stateMessages];

  // If there's a text rejection error, append the error message
  if (state.error === "not_understood") {
    displayMessages.push(errorNotUnderstoodMessage(t));
    // Re-append the current state's quick replies
    const lastWithReplies = [...stateMessages].reverse().find((m: ChatMessageLocal) => m.quickReplies);
    if (lastWithReplies) {
      displayMessages.push(
        makeAssistant("", lastWithReplies.quickReplies),
      );
    }
  } else if (state.error && state.error !== "not_understood") {
    displayMessages.push(toolErrorMessage(t, state.error));
  }

  return {
    messages: displayMessages,
    currentState: state.currentState,
    context: state.context,
    dispatch,
    loading: state.currentState === "loading",
    error: state.error,
  };
}
