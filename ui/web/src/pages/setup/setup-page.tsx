import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router";
import { useOnboardingEngine } from "./hooks/use-onboarding-engine";
import { useOnboardingApi } from "./hooks/use-onboarding-api";
import { OnboardingChat } from "./onboarding-chat";
import { OnboardingInput } from "./onboarding-input";
import { ROUTES } from "@/lib/constants";
import { LoadingSpinner } from "@/components/shared/loading-spinner";
import { ErrorState } from "@/components/shared/error-state";
import type { ChatMessageLocal } from "./hooks/use-onboarding-engine";

export function SetupPage() {
  const navigate = useNavigate();
  const api = useOnboardingApi();
  const engine = useOnboardingEngine();

  const [fetchError, setFetchError] = useState<string | null>(null);
  const [initialized, setInitialized] = useState(false);
  const [typing, setTyping] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  const fetchStatus = useCallback(async () => {
    try {
      setFetchError(null);
      const status = await api.getStatus();

      if (status.onboarding_complete) {
        navigate(ROUTES.OVERVIEW, { replace: true });
        return;
      }

      engine.dispatch({ type: "INIT", status });
      setInitialized(true);

      // Simulate typing delay for natural feel
      setTyping(true);
      setTimeout(() => setTyping(false), 300);
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : "Failed to load setup status");
    }
  }, [api, engine, navigate]);

  useEffect(() => {
    if (!initialized) {
      fetchStatus();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Navigate to dashboard on complete state
  useEffect(() => {
    if (engine.currentState === "complete" && engine.context.onboardingComplete) {
      const timer = setTimeout(() => {
        navigate(ROUTES.OVERVIEW, { replace: true });
      }, 2000);
      return () => clearTimeout(timer);
    }
  }, [engine.currentState, engine.context.onboardingComplete, navigate]);

  const handleReply = useCallback(
    async (value: string) => {
      if (actionLoading) return;

      // Show user message immediately via engine dispatch
      engine.dispatch({ type: "REPLY", value });

      // Simulate typing for next state
      setTyping(true);
      setTimeout(() => setTyping(false), 300);
    },
    [engine, actionLoading],
  );

  const handleInput = useCallback(
    async (field: string, value: string) => {
      if (actionLoading) return;

      setActionLoading(true);
      setTyping(true);

      try {
        // Map field to appropriate tool/action
        const toolMap: Record<string, { tool: string; argKey: string }> = {
          agentName: { tool: "rename_agent", argKey: "name" },
          accountName: { tool: "configure_workspace", argKey: "account_name" },
          primaryColor: { tool: "set_branding", argKey: "primary_color" },
          apiKey: { tool: "create_provider", argKey: "api_key" },
          text: { tool: "set_value", argKey: "value" },
        };

        const mapping = toolMap[field];

        if (mapping) {
          const result = await api.callTool(
            mapping.tool,
            { [mapping.argKey]: value },
            engine.currentState,
          );

          if (result.ok) {
            engine.dispatch({ type: "TOOL_SUCCESS", tool: mapping.tool });

            // Only dispatch INPUT (state transition) on API success
            if (field === "agentName" || field === "accountName" || field === "primaryColor") {
              engine.dispatch({ type: "INPUT", field, value });
            }
          } else {
            engine.dispatch({
              type: "TOOL_ERROR",
              tool: mapping.tool,
              error: result.error ?? "Unknown error",
            });
          }
        }
      } catch (err) {
        engine.dispatch({
          type: "TOOL_ERROR",
          tool: field,
          error: err instanceof Error ? err.message : "Network error",
        });
      } finally {
        setActionLoading(false);
        setTimeout(() => setTyping(false), 300);
      }
    },
    [api, engine, actionLoading],
  );

  // Loading state
  if (!initialized && !fetchError) {
    return <LoadingSpinner size="lg" className="h-dvh bg-background" />;
  }

  // Error state
  if (fetchError) {
    return <ErrorState message={fetchError} onRetry={fetchStatus} className="h-dvh bg-background" />;
  }

  // Find the last message with quickReplies or inputField for the input area
  const lastMessage = findLastInteractiveMessage(engine.messages);

  return (
    <div className="flex h-dvh flex-col bg-background">
      {/* Header */}
      <header className="flex shrink-0 items-center gap-3 border-b border-border px-4 py-3 safe-top">
        <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-bold">
          A
        </div>
        <div>
          <h1 className="text-base font-semibold text-foreground md:text-sm">
            ARGO
          </h1>
          <p className="text-xs text-muted-foreground">
            {engine.context.agentName}
          </p>
        </div>
      </header>

      {/* Chat area */}
      <OnboardingChat messages={engine.messages} typing={typing} />

      {/* Input area */}
      <footer className="shrink-0 border-t border-border safe-bottom">
        <OnboardingInput
          quickReplies={lastMessage?.quickReplies}
          inputField={lastMessage?.inputField}
          onReply={handleReply}
          onInput={handleInput}
          disabled={actionLoading}
        />
      </footer>
    </div>
  );
}

/**
 * Find the last message that has quickReplies or inputField.
 * This determines what the input area shows.
 */
function findLastInteractiveMessage(
  messages: ChatMessageLocal[],
): ChatMessageLocal | undefined {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg && (msg.quickReplies || msg.inputField)) {
      return msg;
    }
  }
  return undefined;
}
