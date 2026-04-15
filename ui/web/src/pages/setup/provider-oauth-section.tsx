import { useState, useEffect, useRef } from "react";
import { cn } from "@/lib/utils";

export interface ProviderOAuthSectionProps {
  /** The currently selected provider slug (e.g. "openai", "anthropic", "openrouter"). */
  provider: string;
  /** Whether interaction should be disabled (e.g. a parent is loading). */
  disabled: boolean;
  /** Called when the OAuth flow completes successfully. */
  onSuccess: () => void;
  /** Called when the user chooses to fall back to manual API key entry. */
  onFallback: () => void;
}

type OAuthUiState = "idle" | "waiting" | "fallback";

const OAUTH_TIMEOUT_MS = 30_000;

/**
 * ProviderOAuthSection renders an OAuth connection button for the OpenAI
 * provider. For all other providers it renders nothing.
 *
 * Flow:
 * 1. "idle"    — shows primary "Conectar com OpenAI via OAuth" button.
 * 2. "waiting" — user clicked; shows loading text + 30-second countdown.
 *                A `window.message` listener waits for the OAuth popup to post
 *                back a success event. After 30s the UI transitions to fallback.
 * 3. "fallback"— shows a card offering the manual API-key path.
 */
export function ProviderOAuthSection({
  provider,
  disabled,
  onSuccess,
  onFallback,
}: ProviderOAuthSectionProps) {
  const [uiState, setUiState] = useState<OAuthUiState>("idle");
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const popupRef = useRef<Window | null>(null);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (timeoutRef.current !== null) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  // Only render for OpenAI
  if (provider !== "openai") {
    return null;
  }

  function handleOAuthClick() {
    if (disabled) return;

    setUiState("waiting");

    // Open OAuth popup (stub — real URL would come from the API)
    // In production: fetch an OAuth initiation URL from the BFF first.
    popupRef.current = window.open(
      "about:blank",
      "argo_oauth_openai",
      "width=500,height=600,noopener,noreferrer",
    );

    // Listen for OAuth completion message from the popup
    function handleMessage(event: MessageEvent) {
      if (event.data?.type === "argo_oauth_success" && event.data?.provider === "openai") {
        cleanup();
        onSuccess();
      }
    }

    function cleanup() {
      window.removeEventListener("message", handleMessage);
      if (timeoutRef.current !== null) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
    }

    window.addEventListener("message", handleMessage);

    // After 30 seconds, show the fallback card
    timeoutRef.current = setTimeout(() => {
      cleanup();
      setUiState("fallback");
    }, OAUTH_TIMEOUT_MS);
  }

  if (uiState === "idle") {
    return (
      <div className="px-4 py-3">
        <button
          type="button"
          disabled={disabled}
          aria-label="Conectar com OpenAI via OAuth"
          onClick={handleOAuthClick}
          className={cn(
            "w-full rounded-lg px-4 py-3 text-base font-medium md:text-sm",
            "bg-primary text-primary-foreground hover:bg-primary/90",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            "disabled:pointer-events-none disabled:opacity-50",
            "transition-colors",
          )}
        >
          Conectar com OpenAI via OAuth
        </button>
      </div>
    );
  }

  if (uiState === "waiting") {
    return (
      <div className="px-4 py-3 space-y-2">
        <div
          role="status"
          aria-live="polite"
          className="flex items-center gap-2 text-sm text-muted-foreground"
        >
          {/* Simple animated dot indicator */}
          <span
            aria-hidden="true"
            className="inline-block h-2 w-2 animate-pulse rounded-full bg-primary"
          />
          <span>Aguardando autorização...</span>
        </div>
        <p className="text-xs text-muted-foreground">
          Complete a autorização na janela que abriu.
        </p>
      </div>
    );
  }

  // uiState === "fallback"
  return (
    <div className="px-4 py-3 space-y-2">
      <p className="text-xs text-muted-foreground" aria-live="polite">
        A janela OAuth foi encerrada sem autorização.
      </p>
      <button
        type="button"
        onClick={onFallback}
        aria-label="Não conseguiu conectar? Cole sua API key"
        className={cn(
          "flex w-full flex-col items-start gap-0.5 rounded-lg px-4 py-3 text-left transition-colors",
          "border border-border bg-card hover:bg-accent",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          "disabled:pointer-events-none disabled:opacity-50",
        )}
      >
        <span className="text-base font-medium text-foreground md:text-sm">
          Não conseguiu conectar? Cole sua API key
        </span>
        <span className="text-xs text-muted-foreground">
          Insira a chave manualmente para continuar
        </span>
      </button>
    </div>
  );
}
