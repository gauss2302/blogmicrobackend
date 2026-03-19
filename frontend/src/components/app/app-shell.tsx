"use client";

import { AppSidebar } from "@/components/app/app-sidebar";

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-dvh w-full">
      <AppSidebar />
      <main className="min-h-dvh flex-1 overflow-auto bg-background/50">
        {children}
      </main>
    </div>
  );
}
