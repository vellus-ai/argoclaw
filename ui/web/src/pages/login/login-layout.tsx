import { Anchor } from "lucide-react";

interface LoginLayoutProps {
  children: React.ReactNode;
  subtitle?: string;
}

export function LoginLayout({ children }: LoginLayoutProps) {
  return (
    <div className="flex min-h-dvh items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center">
          <Anchor className="mx-auto mb-3 h-10 w-10 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">ARGO Gateway</h1>
        </div>
        <div className="rounded-lg border bg-card p-6 shadow-sm sm:p-8">
          {children}
        </div>
      </div>
    </div>
  );
}
