"use client";

import { useState, useEffect } from "react";
import { useThemeStore } from "@/lib/theme/theme-store";
import { getResolvedTheme } from "@/lib/theme/apply-theme";
import { useAuthStore } from "@/lib/stores/auth-store";
import type { ThemePreference } from "@/lib/theme/constants";
import { Moon, Sun, Monitor } from "lucide-react";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";

const THEME_OPTIONS: { value: ThemePreference; label: string; icon: React.ReactNode }[] = [
  { value: "dark", label: "Dark", icon: <Moon className="h-4 w-4" /> },
  { value: "light", label: "Light", icon: <Sun className="h-4 w-4" /> },
  { value: "system", label: "System", icon: <Monitor className="h-4 w-4" /> },
];

export default function ProfilePage() {
  const user = useAuthStore((state) => state.user);
  const preference = useThemeStore((state) => state.preference);
  const setPreference = useThemeStore((state) => state.setPreference);
  const [resolved, setResolved] = useState<"dark" | "light">("dark");

  useEffect(() => {
    setResolved(getResolvedTheme(preference));
  }, [preference]);

  return (
    <div className="flex flex-col">
      <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center border-b border-border bg-card/80 px-6 backdrop-blur-sm">
        <h1 className="text-sm font-semibold text-foreground">Profile</h1>
      </header>
      <div className="mx-auto w-full max-w-2xl px-6 py-8">
        <div className="space-y-6">
          <Card className="border-border bg-card">
            <CardHeader>
              <CardTitle className="text-foreground">Account</CardTitle>
              <CardDescription>Your account information.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <div>
                <span className="text-muted">Email</span>
                <p className="font-medium text-foreground">{user?.email ?? "—"}</p>
              </div>
              <div>
                <span className="text-muted">Name</span>
                <p className="font-medium text-foreground">{user?.name ?? "—"}</p>
              </div>
              {user?.id && (
                <div>
                  <span className="text-muted">User ID</span>
                  <p className="font-mono text-xs text-foreground">{user.id}</p>
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="border-border bg-card">
            <CardHeader>
              <CardTitle className="text-foreground">Appearance</CardTitle>
              <CardDescription>
                Choose how the app looks. Current: {resolved === "dark" ? "Dark" : "Light"}.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {THEME_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setPreference(opt.value)}
                    className={cn(
                      "flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors",
                      preference === opt.value
                        ? "border-ring bg-ring/15 text-foreground"
                        : "border-border bg-transparent text-muted hover:bg-background hover:text-foreground",
                    )}
                  >
                    {opt.icon}
                    {opt.label}
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
