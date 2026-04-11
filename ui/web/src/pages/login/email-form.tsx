import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, Check, X } from "lucide-react";
import { login, register, AuthApiError } from "@/api/auth-client";
import type { AuthResponse } from "@/api/auth-client";
import { INPUT_CLASS, BUTTON_CLASS } from "./form-styles";

interface EmailFormProps {
  onSuccess: (accessToken: string, refreshToken: string, userId: string) => void;
}

type Mode = "signIn" | "signUp";

interface PasswordRequirement {
  key: string;
  test: (pw: string, email: string) => boolean;
}

const PASSWORD_REQUIREMENTS: PasswordRequirement[] = [
  { key: "req12Chars", test: (pw) => pw.length >= 12 },
  { key: "reqUppercase", test: (pw) => /[A-Z]/.test(pw) },
  { key: "reqLowercase", test: (pw) => /[a-z]/.test(pw) },
  { key: "reqDigit", test: (pw) => /\d/.test(pw) },
  { key: "reqSpecial", test: (pw) => /[^A-Za-z0-9]/.test(pw) },
  { key: "reqNoEmail", test: (pw, email) => {
    if (!email) return true;
    const local = email.split("@")[0]?.toLowerCase() ?? "";
    return local.length < 3 || !pw.toLowerCase().includes(local);
  }},
];

export function EmailForm({ onSuccess }: EmailFormProps) {
  const { t } = useTranslation("login");
  const [mode] = useState<Mode>("signIn");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isSignUp = mode === "signUp";
  const passwordValid = PASSWORD_REQUIREMENTS.every((r) => r.test(password, email));
  const canSubmit = isSignUp
    ? email.trim() && password && confirmPassword && passwordValid && password === confirmPassword
    : email.trim() && password;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;

    if (isSignUp && password !== confirmPassword) {
      setError(t("email.errorPasswordMismatch"));
      return;
    }

    setLoading(true);
    setError(null);

    try {
      let res: AuthResponse;
      if (isSignUp) {
        res = await register(email.trim(), password, displayName.trim() || undefined);
      } else {
        res = await login(email.trim(), password);
      }
      onSuccess(res.access_token, res.refresh_token, res.user.id);
    } catch (err) {
      if (err instanceof AuthApiError) {
        switch (err.status) {
          case 401:
            setError(t("email.errorInvalidCredentials"));
            break;
          case 409:
            setError(t("email.errorEmailTaken"));
            break;
          case 429:
            setError(t("email.errorAccountLocked"));
            break;
          default:
            setError(t("email.errorServer"));
        }
      } else {
        setError(t("email.errorNetwork"));
      }
    } finally {
      setLoading(false);
    }
  }

  function clearError() {
    setError(null);
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {isSignUp && (
        <div className="space-y-2">
          <label htmlFor="displayName" className="text-sm font-medium">
            {t("email.displayName")}
          </label>
          <input
            id="displayName"
            type="text"
            value={displayName}
            onChange={(e) => { setDisplayName(e.target.value); clearError(); }}
            placeholder={t("email.displayNamePlaceholder")}
            className={INPUT_CLASS}
            disabled={loading}
          />
        </div>
      )}

      <div className="space-y-2">
        <label htmlFor="email" className="text-sm font-medium">
          {t("email.email")}
        </label>
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => { setEmail(e.target.value); clearError(); }}
          placeholder={t("email.emailPlaceholder")}
          className={INPUT_CLASS}
          autoFocus
          autoComplete="email"
          disabled={loading}
        />
      </div>

      <div className="space-y-2">
        <label htmlFor="password" className="text-sm font-medium">
          {t("email.password")}
        </label>
        <input
          id="password"
          type="password"
          value={password}
          onChange={(e) => { setPassword(e.target.value); clearError(); }}
          placeholder={t("email.passwordPlaceholder")}
          className={INPUT_CLASS}
          autoComplete={isSignUp ? "new-password" : "current-password"}
          disabled={loading}
        />
      </div>

      {isSignUp && (
        <>
          <div className="space-y-2">
            <label htmlFor="confirmPassword" className="text-sm font-medium">
              {t("email.confirmPassword")}
            </label>
            <input
              id="confirmPassword"
              type="password"
              value={confirmPassword}
              onChange={(e) => { setConfirmPassword(e.target.value); clearError(); }}
              placeholder={t("email.confirmPasswordPlaceholder")}
              className={INPUT_CLASS}
              autoComplete="new-password"
              disabled={loading}
            />
          </div>

          {password.length > 0 && (
            <ul className="space-y-1 text-xs">
              {PASSWORD_REQUIREMENTS.map((req) => {
                const pass = req.test(password, email);
                return (
                  <li key={req.key} className={`flex items-center gap-1.5 ${pass ? "text-green-600" : "text-muted-foreground"}`}>
                    {pass ? <Check className="h-3 w-3" /> : <X className="h-3 w-3" />}
                    {t(`email.${req.key}`)}
                  </li>
                );
              })}
            </ul>
          )}
        </>
      )}

      {error && (
        <div className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <button type="submit" disabled={!canSubmit || loading} className={BUTTON_CLASS}>
        {loading
          ? t("email.submitting")
          : isSignUp
            ? t("email.signUp")
            : t("email.signIn")}
      </button>

    </form>
  );
}
