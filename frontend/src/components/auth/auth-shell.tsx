import Link from "next/link";

type AuthShellProps = {
  title: string;
  subtitle: string;
  children: React.ReactNode;
};

export function AuthShell({ title, subtitle, children }: AuthShellProps) {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="border-b border-border">
        <div className="mx-auto flex h-12 w-full max-w-6xl items-center px-6">
          <Link
            href="/"
            className="font-mono text-sm font-bold tracking-[0.2em] text-foreground hover:text-primary transition-colors"
          >
            MICROBLOG
          </Link>
        </div>
      </header>

      <main className="flex flex-1 items-start justify-center px-4 py-16">
        <section className="w-full max-w-sm">
          <div className="mb-8 space-y-1">
            <h1 className="text-2xl">{title}</h1>
            <p className="text-sm text-muted-foreground">{subtitle}</p>
          </div>
          {children}
        </section>
      </main>

      <footer className="border-t border-border">
        <div className="mx-auto flex h-10 w-full max-w-6xl items-center justify-between px-6 text-xs font-mono uppercase tracking-[0.1em] text-muted-foreground">
          <span>Microblog · gRPC backend</span>
          <span>v1</span>
        </div>
      </footer>
    </div>
  );
}
