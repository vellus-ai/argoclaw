import { useState } from "react";
import { useNavigate, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import { AlertCircle, Check, X, ShieldAlert } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { useAuthStore } from "@/stores/use-auth-store";
import { changePassword, AuthApiError } from "@/api/auth-client";
import { ROUTES } from "@/lib/constants";

interface PasswordRequirement {
  key: string;
  test: (pw: string) => boolean;
}

const PASSWORD_REQUIREMENTS: PasswordRequirement[] = [
  { key: "req12Chars", test: (pw) => pw.length >= 12 },
  { key: "reqUppercase", test: (pw) => /[A-Z]/.test(pw) },
  { key: "reqLowercase", test: (pw) => /[a-z]/.test(pw) },
  { key: "reqDigit", test: (pw) => /\d/.test(pw) },
  { key: "reqSpecial", test: (pw) => /[^A-Za-z0-9]/.test(pw) },
];

export function ChangePasswordPage() {
  const { t } = useTranslation("login");
  const navigate = useNavigate();
  const location = useLocation();
  const token = useAuthStore((s) => s.token);
  const setJwtAuth = useAuthStore((s) => s.setJwtAuth);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const passwordValid = PASSWORD_REQUIREMENTS.every((r) => r.test(newPassword));
  const passwordsMatch = newPassword === confirmPassword;
  const canSubmit =
    currentPassword.length > 0 &&
    newPassword.length > 0 &&
    confirmPassword.length > 0 &&
    passwordValid &&
    passwordsMatch;

  function clearError() {
    setError(null);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;

    if (!passwordsMatch) {
      setError(t("changePassword.errorMismatch"));
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await changePassword(currentPassword, newPassword, token);
      setJwtAuth(res.access_token, res.refresh_token, res.user.id);

      // Navigate to wherever the user was trying to go, or overview
      const from = (location.state as { from?: { pathname: string } })?.from?.pathname ?? ROUTES.OVERVIEW;
      navigate(from, { replace: true });
    } catch (err) {
      if (err instanceof AuthApiError) {
        switch (err.status) {
          case 401:
            setError(t("changePassword.errorWrongCurrent"));
            break;
          case 400:
            setError(err.message || t("changePassword.errorWeakPassword"));
            break;
          default:
            setError(t("changePassword.errorServer"));
        }
      } else {
        setError(t("changePassword.errorNetwork"));
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-background px-4 py-8">
      <div className="w-full max-w-md space-y-6">
        <div className="flex flex-col items-center text-center">
          <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-amber-100 dark:bg-amber-900/30">
            <ShieldAlert className="h-7 w-7 text-amber-600 dark:text-amber-400" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("changePassword.title")}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("changePassword.description")}
          </p>
        </div>

        <div className="rounded-xl border bg-card p-6 shadow-sm">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="cp-current">
                {t("changePassword.currentPassword")}
              </Label>
              <Input
                id="cp-current"
                type="password"
                value={currentPassword}
                onChange={(e) => { setCurrentPassword(e.target.value); clearError(); }}
                placeholder={t("changePassword.currentPasswordPlaceholder")}
                autoComplete="current-password"
                autoFocus
                disabled={loading}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="cp-new">
                {t("changePassword.newPassword")}
              </Label>
              <Input
                id="cp-new"
                type="password"
                value={newPassword}
                onChange={(e) => { setNewPassword(e.target.value); clearError(); }}
                placeholder={t("changePassword.newPasswordPlaceholder")}
                autoComplete="new-password"
                disabled={loading}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="cp-confirm">
                {t("changePassword.confirmPassword")}
              </Label>
              <Input
                id="cp-confirm"
                type="password"
                value={confirmPassword}
                onChange={(e) => { setConfirmPassword(e.target.value); clearError(); }}
                placeholder={t("changePassword.confirmPasswordPlaceholder")}
                autoComplete="new-password"
                disabled={loading}
              />
            </div>

            {newPassword.length > 0 && (
              <ul className="space-y-1 text-xs">
                {PASSWORD_REQUIREMENTS.map((req) => {
                  const pass = req.test(newPassword);
                  return (
                    <li key={req.key} className={`flex items-center gap-1.5 ${pass ? "text-success" : "text-muted-foreground"}`}>
                      {pass ? <Check className="h-3 w-3" /> : <X className="h-3 w-3" />}
                      {t(`email.${req.key}`)}
                    </li>
                  );
                })}
                <li className={`flex items-center gap-1.5 ${confirmPassword.length > 0 && passwordsMatch ? "text-success" : "text-muted-foreground"}`}>
                  {confirmPassword.length > 0 && passwordsMatch
                    ? <Check className="h-3 w-3" />
                    : <X className="h-3 w-3" />}
                  {t("changePassword.reqMatch")}
                </li>
              </ul>
            )}

            {error && (
              <div className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            <Button type="submit" className="w-full" disabled={!canSubmit || loading}>
              {loading ? t("changePassword.submitting") : t("changePassword.submit")}
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}
