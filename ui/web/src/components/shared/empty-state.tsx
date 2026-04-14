import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

/** Props do componente EmptyState. */
interface EmptyStateProps {
  /** Icone Lucide exibido acima do titulo. */
  icon?: LucideIcon;
  /** Titulo principal do estado vazio. */
  title: string;
  /** Descricao secundaria opcional abaixo do titulo. */
  description?: string;
  /** Acao opcional (botao, link) renderizada abaixo da descricao. */
  action?: React.ReactNode;
  /** Classes CSS adicionais para o container raiz. */
  className?: string;
}

/**
 * Componente compartilhado para estados vazios em listas e colecoes.
 *
 * Exibe icone, titulo, descricao e acao opcional de forma centralizada.
 *
 * @example
 * <EmptyState
 *   icon={InboxIcon}
 *   title="Nenhum agente encontrado"
 *   description="Crie seu primeiro agente para comecar."
 *   action={<Button>Criar agente</Button>}
 * />
 */
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center py-16 text-center", className)}>
      {Icon && (
        <div className="mb-4 rounded-full bg-muted p-3">
          <Icon className="h-6 w-6 text-muted-foreground" />
        </div>
      )}
      <h3 className="text-sm font-medium">{title}</h3>
      {description && (
        <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
