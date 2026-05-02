import type { ThemePreference } from "@/lib/theme/constants";

function resolveTheme(preference: ThemePreference): "dark" | "light" {
  if (preference === "dark") return "dark";
  if (preference === "light") return "light";
  if (typeof window !== "undefined" && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}

export function applyThemeToDocument(preference: ThemePreference): void {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-theme", resolveTheme(preference));
}

export function getResolvedTheme(preference: ThemePreference): "dark" | "light" {
  return resolveTheme(preference);
}
