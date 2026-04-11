interface LoginLayoutProps {
  children: React.ReactNode;
  subtitle?: string;
}

export function LoginLayout({ children, subtitle }: LoginLayoutProps) {
  return (
    <div className="flex min-h-dvh items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-semibold tracking-tight">ARGO Gateway</h1>
          <p className="mt-1 text-sm text-muted-foreground">Painel do Gateway</p>
          {subtitle && (
            <p className="mt-1 text-xs text-muted-foreground">{subtitle}</p>
          )}
        </div>
        <div className="rounded-lg border bg-card p-6 shadow-sm sm:p-8">
          {children}
        </div>
      </div>
    </div>
  );
}
