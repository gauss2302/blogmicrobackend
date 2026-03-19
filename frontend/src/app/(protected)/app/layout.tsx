import { SessionBootstrap } from "@/components/app/session-bootstrap";
import { AppShell } from "@/components/app/app-shell";

export default function ProtectedAppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <SessionBootstrap>
      <AppShell>{children}</AppShell>
    </SessionBootstrap>
  );
}
