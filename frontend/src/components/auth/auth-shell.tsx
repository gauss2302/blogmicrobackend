"use client";

import { motion } from "framer-motion";

import { Badge } from "@/components/ui/badge";

type AuthShellProps = {
  title: string;
  subtitle: string;
  children: React.ReactNode;
};

export function AuthShell({ title, subtitle, children }: AuthShellProps) {
  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-10">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -left-20 top-10 h-64 w-64 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute -right-10 bottom-10 h-72 w-72 rounded-full bg-orange-500/20 blur-3xl" />
        <div className="absolute left-1/2 top-1/3 h-60 w-60 -translate-x-1/2 rounded-full bg-emerald-500/10 blur-3xl" />
      </div>

      <motion.section
        initial={{ opacity: 0, y: 18 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: "easeOut" }}
        className="relative z-10 w-full max-w-md"
      >
        <div className="mb-6 space-y-3 text-center">
          <Badge className="mx-auto">Secure OAuth Gateway</Badge>
          <h1 className="text-3xl font-extrabold tracking-tight text-zinc-50">{title}</h1>
          <p className="text-sm text-zinc-400">{subtitle}</p>
        </div>
        {children}
      </motion.section>
    </div>
  );
}
