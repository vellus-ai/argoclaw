import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

interface TokenFormProps {
  onSubmit: (userId: string, token: string) => void;
}

export function TokenForm({ onSubmit }: TokenFormProps) {
  const { t } = useTranslation("login");
  const [userId, setUserId] = useState("");
  const [token, setToken] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!token.trim() || !userId.trim()) return;

    setConnecting(true);
    setError(null);

    try {
      // Verify connectivity and credentials before navigating
      const res = await fetch("/v1/agents", {
        headers: {
          Authorization: `Bearer ${token.trim()}`,
          "X-ArgoClaw-User-Id": userId.trim(),
        },
      });

      if (res.status === 401) {
        setError(t("token.errorInvalidCredentials"));
        return;
      }

      if (!res.ok) {
        setError(t("token.errorServer", { status: res.status }));
        return;
      }

      onSubmit(userId.trim(), token.trim());
    } catch {
      setError(t("token.errorCannotConnect"));
    } finally {
      setConnecting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="userId">
          {t("token.userId")}
        </Label>
        <Input
          id="userId"
          type="text"
          value={userId}
          onChange={(e) => { setUserId(e.target.value); setError(null); }}
          placeholder={t("token.userIdPlaceholder")}
          autoFocus
          disabled={connecting}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="token">
          {t("token.gatewayToken")}
        </Label>
        <Input
          id="token"
          type="password"
          value={token}
          onChange={(e) => { setToken(e.target.value); setError(null); }}
          placeholder={t("token.tokenPlaceholder")}
          disabled={connecting}
        />
      </div>

      {error && (
        <div className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <Button
        type="submit"
        className="w-full"
        disabled={!token.trim() || !userId.trim() || connecting}
      >
        {connecting ? t("token.connecting") : t("token.connect")}
      </Button>
    </form>
  );
}
