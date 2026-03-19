export const THEME_STORAGE_KEY = "microblog_theme";

export type ThemePreference = "dark" | "light" | "system";

export function getStoredTheme(): ThemePreference {
  if (typeof window === "undefined") return "dark";
  const raw = localStorage.getItem(THEME_STORAGE_KEY);
  if (raw === "dark" || raw === "light" || raw === "system") return raw;
  return "dark";
}
