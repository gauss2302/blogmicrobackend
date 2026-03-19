"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Home, LogOut, PanelLeftClose, PanelLeft, User } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useAuthStore } from "@/lib/stores/auth-store";
import { logoutSession } from "@/lib/auth/client-api";
import { cn } from "@/lib/utils";

const SIDEBAR_WIDTH_EXPANDED = 224;
const SIDEBAR_WIDTH_COLLAPSED = 72;

export function AppSidebar() {
  const pathname = usePathname();
  const user = useAuthStore((state) => state.user);
  const [collapsed, setCollapsed] = useState(false);

  const isDashboard = pathname === "/app" || pathname === "/app/";
  const isProfile = pathname === "/app/profile";

  async function handleLogout() {
    await logoutSession();
    window.location.replace("/auth/login");
  }

  return (
    <motion.aside
      className="flex shrink-0 flex-col overflow-hidden border-r border-zinc-800/80 bg-zinc-950/80 backdrop-blur-sm"
      initial={false}
      animate={{ width: collapsed ? SIDEBAR_WIDTH_COLLAPSED : SIDEBAR_WIDTH_EXPANDED }}
      transition={{ type: "spring", stiffness: 300, damping: 30 }}
    >
      <div className="flex h-14 shrink-0 items-center justify-between border-b border-zinc-800/80 px-3">
        <AnimatePresence mode="wait">
          {!collapsed ? (
            <motion.span
              key="logo"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="truncate text-sm font-semibold text-zinc-100"
            >
              Microblog
            </motion.span>
          ) : (
            <motion.span
              key="logo-icon"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="text-sm font-semibold text-cyan-400"
            >
              M
            </motion.span>
          )}
        </AnimatePresence>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setCollapsed((c) => !c)}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          className="h-8 w-8 shrink-0 p-0"
        >
          {collapsed ? (
            <PanelLeft className="h-4 w-4 text-zinc-400" />
          ) : (
            <PanelLeftClose className="h-4 w-4 text-zinc-400" />
          )}
        </Button>
      </div>

      <nav className="flex flex-1 flex-col gap-1 p-2 pt-4">
        <Link
          href="/app"
          className={cn(
            "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
            isDashboard
              ? "bg-cyan-500/15 text-cyan-300"
              : "text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200",
          )}
        >
          <Home className="h-5 w-5 shrink-0" />
          <AnimatePresence mode="wait">
            {!collapsed && (
              <motion.span
                initial={{ opacity: 0, width: 0 }}
                animate={{ opacity: 1, width: "auto" }}
                exit={{ opacity: 0, width: 0 }}
                transition={{ duration: 0.15 }}
                className="overflow-hidden whitespace-nowrap"
              >
                Dashboard
              </motion.span>
            )}
          </AnimatePresence>
        </Link>
        <Link
          href="/app/profile"
          className={cn(
            "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
            isProfile
              ? "bg-cyan-500/15 text-cyan-300"
              : "text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200",
          )}
        >
          <User className="h-5 w-5 shrink-0" />
          <AnimatePresence mode="wait">
            {!collapsed && (
              <motion.span
                initial={{ opacity: 0, width: 0 }}
                animate={{ opacity: 1, width: "auto" }}
                exit={{ opacity: 0, width: 0 }}
                transition={{ duration: 0.15 }}
                className="overflow-hidden whitespace-nowrap"
              >
                Profile
              </motion.span>
            )}
          </AnimatePresence>
        </Link>
      </nav>

      <div className="border-t border-zinc-800/80 p-2">
        <AnimatePresence mode="wait">
          {!collapsed && user && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="mb-2 truncate px-3 py-2 text-xs text-zinc-500"
            >
              {user.email}
            </motion.div>
          )}
        </AnimatePresence>
        <button
          type="button"
          onClick={handleLogout}
          className={cn(
            "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-zinc-400 transition-colors hover:bg-zinc-800/60 hover:text-rose-300",
          )}
        >
          <LogOut className="h-5 w-5 shrink-0" />
          <AnimatePresence mode="wait">
            {!collapsed && (
              <motion.span
                initial={{ opacity: 0, width: 0 }}
                animate={{ opacity: 1, width: "auto" }}
                exit={{ opacity: 0, width: 0 }}
                transition={{ duration: 0.15 }}
                className="overflow-hidden whitespace-nowrap"
              >
                Log out
              </motion.span>
            )}
          </AnimatePresence>
        </button>
      </div>
    </motion.aside>
  );
}
