import { useTranslation } from "react-i18next";

export type LoginMode = "email" | "token" | "pairing";

interface LoginTabsProps {
  mode: LoginMode;
  onModeChange: (mode: LoginMode) => void;
}

const TABS: LoginMode[] = ["email", "token", "pairing"];

export function LoginTabs({ mode, onModeChange }: LoginTabsProps) {
  const { t } = useTranslation("login");
  return (
    <div className="flex rounded-md border bg-muted p-1">
      {TABS.map((tab) => (
        <button
          key={tab}
          type="button"
          onClick={() => onModeChange(tab)}
          className={`flex-1 rounded-sm px-3 py-1.5 text-sm font-medium transition-colors ${
            mode === tab
              ? "bg-background text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {t(`tabs.${tab}`)}
        </button>
      ))}
    </div>
  );
}
