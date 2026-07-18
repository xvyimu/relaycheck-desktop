import { forwardRef, type ComponentPropsWithoutRef } from "react";

import { cn } from "@/lib/cn";
import "@/styles/components/progress.css";

export type ProgressProps = Omit<ComponentPropsWithoutRef<"progress">, "max" | "value"> & {
  max?: number;
  value?: number | null;
};

/** Compact progress track for task, batch, and health indicators. */
export const Progress = forwardRef<HTMLProgressElement, ProgressProps>(
  ({ className, max = 100, value = 0, ...props }, ref) => {
    const safeValue = typeof value === "number" && Number.isFinite(value) ? Math.min(Math.max(value, 0), max) : 0;
    return (
      <progress
        ref={ref}
        max={max}
        value={safeValue}
        className={cn("app-progress h-2 overflow-hidden rounded-full bg-muted", className)}
        {...props}
      />
    );
  },
);

Progress.displayName = "Progress";
