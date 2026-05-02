import * as React from "react";

import { cn } from "@/lib/utils";

type StatusTone = "neutral" | "success" | "warning" | "destructive" | "info";

const dotClasses: Record<StatusTone, string> = {
  neutral: "bg-muted-foreground",
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
  info: "bg-marker-us",
};

interface StatusPillProps extends React.ComponentProps<"span"> {
  tone?: StatusTone;
  children: React.ReactNode;
}

function StatusPill({
  className,
  tone = "neutral",
  children,
  ...props
}: StatusPillProps) {
  return (
    <span
      data-slot="status-pill"
      className={cn(
        "inline-flex items-center gap-1.5",
        "rounded-sm border border-border bg-card",
        "px-2 py-0.5",
        "font-mono text-xs uppercase tracking-[0.1em]",
        "text-foreground whitespace-nowrap",
        className,
      )}
      {...props}
    >
      <span
        aria-hidden
        className={cn("size-1.5 rounded-full", dotClasses[tone])}
      />
      {children}
    </span>
  );
}

export { StatusPill };
export type { StatusTone };
