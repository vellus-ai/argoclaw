import { cva, type VariantProps } from "class-variance-authority";
import { Loader2 } from "lucide-react";

import { cn } from "@/lib/utils";

const spinnerVariants = cva("animate-spin text-muted-foreground", {
  variants: {
    size: {
      sm: "h-4 w-4",
      md: "h-6 w-6",
      lg: "h-10 w-10",
    },
  },
  defaultVariants: {
    size: "md",
  },
});

/**
 * Spinner animado centralizado para estados de carregamento.
 *
 * Inclui `role="status"` e `aria-label` para compatibilidade
 * com leitores de tela. Usa variantes CVA para tamanho.
 *
 * @example
 * <LoadingSpinner />
 * <LoadingSpinner size="lg" aria-label="Loading agents" />
 */
interface LoadingSpinnerProps extends VariantProps<typeof spinnerVariants> {
  className?: string;
  "aria-label"?: string;
}

function LoadingSpinner({
  size,
  className,
  "aria-label": ariaLabel = "Carregando…",
}: LoadingSpinnerProps) {
  return (
    <div
      role="status"
      aria-label={ariaLabel}
      className={cn("flex items-center justify-center", className)}
    >
      <Loader2 className={cn(spinnerVariants({ size }))} />
    </div>
  );
}

export { LoadingSpinner, spinnerVariants };
export type { LoadingSpinnerProps };
