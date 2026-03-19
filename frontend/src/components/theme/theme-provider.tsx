"use client";

import { useEffect } from "react";
import { useThemeStore } from "@/lib/theme/theme-store";
import { applyThemeToDocument } from "@/lib/theme/apply-theme";

/**
 * Syncs theme preference from store to document and listens for system preference changes.
 * Must be mounted inside the app so store is rehydrated.
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const preference = useThemeStore((s) => s.preference);

  useEffect(() => {
    applyThemeToDocument(preference);
  }, [preference]);

  useEffect(() => {
    if (preference !== "system") return;
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => applyThemeToDocument("system");
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, [preference]);

  return <>{children}</>;
}
