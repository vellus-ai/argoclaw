import { useState, useEffect } from "react";
import { useNavigate, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import { useAuthStore } from "@/stores/use-auth-store";
import { ROUTES } from "@/lib/constants";
import { LoginLayout } from "./login-layout";
import { LoginTabs, type LoginMode } from "./login-tabs";
import { TokenForm } from "./token-form";
import { PairingForm } from "./pairing-form";
import { EmailForm } from "./email-form";

export function LoginPage() {
  const { t } = useTranslation("login");
  const [mode, setMode] = useState<LoginMode>("email");
  const [emailAuthAvailable, setEmailAuthAvailable] = useState<boolean | null>(null);

  const setCredentials = useAuthStore((s) => s.setCredentials);
  const setJwtAuth = useAuthStore((s) => s.setJwtAuth);
  const setPairing = useAuthStore((s) => s.setPairing);
  const navigate = useNavigate();
  const location = useLocation();

  const from =
    (location.state as { from?: { pathname: string } })?.from?.pathname ??
    ROUTES.OVERVIEW;

  // Detect whether email auth endpoints are available.
  // When JWTSecret is not configured, /v1/auth/* routes are not registered.
  useEffect(() => {
    const controller = new AbortController();
    fetch("/v1/auth/login", { method: "HEAD", signal: controller.signal })
      .then((res) => {
        // 405 Method Not Allowed = endpoint exists (POST-only). 404 = not registered.
        setEmailAuthAvailable(res.status !== 404);
      })
      .catch(() => {
        if (!controller.signal.aborted) setEmailAuthAvailable(false);
      });
    return () => controller.abort();
  }, []);

  // Show legacy tabs (token/pairing) only when email auth is unavailable.
  const showTabs = emailAuthAvailable === false;

  // When email auth is unavailable, switch to token mode.
  useEffect(() => {
    if (emailAuthAvailable === false && mode === "email") {
      setMode("token");
    }
  }, [emailAuthAvailable]);

  function handleTokenLogin(userId: string, token: string) {
    setCredentials(token, userId);
    navigate(from, { replace: true });
  }

  function handleEmailLogin(accessToken: string, refreshToken: string, userId: string) {
    setJwtAuth(accessToken, refreshToken, userId);
    navigate(from, { replace: true });
  }

  function handlePairingApproved(senderID: string, userId: string) {
    setPairing(senderID, userId);
    setTimeout(() => navigate(from, { replace: true }), 500);
  }

  return (
    <LoginLayout subtitle={t("subtitle")}>
      {showTabs && <LoginTabs mode={mode} onModeChange={setMode} />}
      {mode === "email" ? (
        <EmailForm onSuccess={handleEmailLogin} />
      ) : mode === "token" ? (
        <TokenForm onSubmit={handleTokenLogin} />
      ) : (
        <PairingForm onApproved={handlePairingApproved} />
      )}
    </LoginLayout>
  );
}
