/** Props do componente PageHeader. */
interface PageHeaderProps {
  /** Titulo principal da pagina (renderizado como h1). */
  title: string;
  /** Descricao opcional exibida abaixo do titulo. */
  description?: React.ReactNode;
  /** Acoes (botoes) alinhadas a direita no desktop, abaixo no mobile. */
  actions?: React.ReactNode;
}

/**
 * Cabecalho padrao de pagina com titulo, descricao e acoes.
 *
 * Layout responsivo: titulo e acoes lado a lado no desktop,
 * empilhados no mobile.
 *
 * @example
 * <PageHeader
 *   title="Agentes"
 *   description="Gerencie seus agentes de IA."
 *   actions={<Button>Novo agente</Button>}
 * />
 */
export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description && (
          <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}
