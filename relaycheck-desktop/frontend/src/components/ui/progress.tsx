import { forwardRef, type HTMLAttributes } from "react";

import { cn } from "@/lib/cn";

export type ProgressProps = HTMLAttributes<HTMLDivElement> & {
  max?: number;
  value?: number | null;
};

function clampProgress(value: number, max: number) {
  if (!Number.isFinite(value) || !Number.isFinite(max) || max <= 0) {
    return 0;
  }

  return Math.min(Math.max((value / max) * 100, 0), 100);
}

/** Compact progress track for task, batch, and health indicators. */
export const Progress = forwardRef<HTMLDivElement, ProgressProps>(
  ({ className, max = 100, value = 0, ...props }, ref) => {
    const safeValue = typeof value === "number" && Number.isFinite(value) ? Math.min(Math.max(value, 0), max) : 0;
    const percent = clampProgress(safeValue, max);

    return (
      <div
        ref={ref}
        aria-valuemax={max}
        aria-valuemin={0}
        aria-valuenow={safeValue}
        className={cn("h-2 overflow-hidden rounded-full bg-muted", className)}
        role="progressbar"
        {...props}
      >
        <span
          className="block h-full rounded-full bg-primary transition-[width] duration-300 ease-out"
          style={{ width: `${percent}%` }}
        />
      </div>
    );
  },
);

Progress.displayName = "Progress";
