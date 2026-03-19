"use client";

import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-semibold transition-all disabled:pointer-events-none disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/80",
  {
    variants: {
      variant: {
        default:
          "bg-cyan-400 text-zinc-950 shadow-[0_0_0_1px_rgba(255,255,255,0.16)] hover:bg-cyan-300",
        outline:
          "border border-zinc-700 bg-zinc-950/50 text-zinc-100 hover:border-zinc-500 hover:bg-zinc-900",
        ghost: "text-zinc-100 hover:bg-zinc-900",
      },
      size: {
        default: "h-10 px-4 py-2",
        lg: "h-12 px-6 py-3",
        sm: "h-9 px-3",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  }) {
  const Comp = asChild ? Slot : "button";

  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
