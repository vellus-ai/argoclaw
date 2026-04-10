import { useNavigate, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import { useAuthStore } from "@/stores/use-auth-store";
import { ROUTES } from "@/lib/constants";
import { LoginLayout } from "./login-layout";
import { EmailForm } from "./email-form";

export function LoginPage() {
  const { t } = useTranslation("login");

  const setJwtAuth = useAuthStore((s) => s.setJwtAuth);
  const navigate = useNavigate();
  const location = useLocation();

  const from =
    (location.state as { from?: { pathname: string } })?.from?.pathname ??
    ROUTES.OVERVIEW;

  function handleEmailLogin(accessToken: string, refreshToken: string, userId: string) {
    setJwtAuth(accessToken, refreshToken, userId);
    navigate(from, { replace: true });
  }

  return (
    <LoginLayout subtitle={t("subtitle")}>
      <EmailForm onSuccess={handleEmailLogin} />
    </LoginLayout>
  );
}
