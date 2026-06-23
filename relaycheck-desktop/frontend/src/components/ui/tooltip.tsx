import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/lib/cn";

export type TooltipProps = HTMLAttributes<HTMLSpanElement> & {
  content: ReactNode;
  children: ReactNode;
};

/** CSS-only tooltip for brief helper text; keep critical information visible elsewhere. */
export function Tooltip({ children, className, content, ...props }: TooltipProps) {
  return (
    <span className={cn("group relative inline-flex", className)} {...props}>
      {children}
      <span
        className={cn(
          "pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 w-max max-w-64 -translate-x-1/2",
          "rounded-md border border-border bg-card px-2.5 py-1.5 text-xs leading-5 text-foreground shadow-card",
          "opacity-0 transition-opacity duration-150 group-focus-within:opacity-100 group-hover:opacity-100",
        )}
        role="tooltip"
      >
        {content}
      </span>
    </span>
  );
}
