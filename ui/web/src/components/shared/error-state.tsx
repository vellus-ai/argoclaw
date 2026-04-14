import { AlertCircle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * Estado de erro compartilhado com ícone, mensagem e botão de retry.
 *
 * Inclui `role="alert"` e `aria-live="assertive"` para acessibilidade.
 * O botão de retry usa o componente `<Button>` da Camada_Base e o label
 * é traduzido via i18n (namespace `common`).
 *
 * @example
 * <ErrorState message="Failed to load agents" onRetry={() => refetch()} />
 * <ErrorState message="Network error" />
 */
interface ErrorStateProps {
  /** Mensagem de erro a ser exibida */
  message: string;
  /** Callback de retry — se omitido, o botão não é renderizado */
  onRetry?: () => void;
  className?: string;
}

function ErrorState({ message, onRetry, className }: ErrorStateProps) {
  const { t } = useTranslation("common");

  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-4 p-6 text-center",
        className
      )}
    >
      <AlertCircle className="h-10 w-10 text-destructive" />
      <div role="alert" aria-live="assertive">
        <p className="text-sm text-muted-foreground">{message}</p>
      </div>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t("error.retry")}
        </Button>
      )}
    </div>
  );
}

export { ErrorState };
export type { ErrorStateProps };
