import { forwardRef, type ButtonHTMLAttributes } from "react";

import { cn } from "@/lib/cn";

export type SwitchProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-checked" | "role"> & {
  checked?: boolean;
};

/** Accessible switch primitive backed by a native button. */
export const Switch = forwardRef<HTMLButtonElement, SwitchProps>(
  ({ checked = false, className, disabled, type = "button", ...props }, ref) => {
    return (
      <button
        ref={ref}
        aria-checked={checked}
        className={cn(
          "inline-flex h-6 w-11 shrink-0 items-center rounded-full border border-border p-0.5 transition",
          "focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
          checked ? "bg-primary" : "bg-muted",
          className,
        )}
        disabled={disabled}
        role="switch"
        type={type}
        {...props}
      >
        <span
          className={cn(
            "size-5 rounded-full bg-card shadow-sm transition-transform",
            checked ? "translate-x-5" : "translate-x-0",
          )}
        />
      </button>
    );
  },
);

Switch.displayName = "Switch";
