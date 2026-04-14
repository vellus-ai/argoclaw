import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, Check, X, ShieldAlert } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { useAuthStore } from "@/stores/use-auth-store";
import { changePassword, AuthApiError } from "@/api/auth-client";

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

/**
 * Modal de troca obrigatória de senha.
 *
 * Exibido quando o JWT contém `must_chg_pwd: true`. Bloqueia
 * interação até o usuário definir uma nova senha válida.
 *
 * @example
 * <ChangePasswordModal />
 */
export function ChangePasswordModal() {
  const { t } = useTranslation("login");
  const mustChangePassword = useAuthStore((s) => s.mustChangePassword);
  const token = useAuthStore((s) => s.token);
  const setJwtAuth = useAuthStore((s) => s.setJwtAuth);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!mustChangePassword) return null;

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
    <Dialog open modal>
      <DialogContent
        showCloseButton={false}
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
        onInteractOutside={(e) => e.preventDefault()}
      >
        <DialogHeader>
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-amber-100 dark:bg-amber-900/30 sm:mx-0">
            <ShieldAlert className="h-6 w-6 text-amber-600 dark:text-amber-400" />
          </div>
          <DialogTitle>{t("changePassword.title")}</DialogTitle>
          <DialogDescription>
            {t("changePassword.description")}
          </DialogDescription>
        </DialogHeader>

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
      </DialogContent>
    </Dialog>
  );
}
