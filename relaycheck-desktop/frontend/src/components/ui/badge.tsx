import type { HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

type BadgeVariant = "default" | "secondary" | "success" | "warning" | "destructive" | "outline";

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  variant?: BadgeVariant;
};

const variantClasses: Record<BadgeVariant, string> = {
  default: "border-[var(--v4-blue-line)] bg-[var(--v4-blue-soft)] text-[var(--v4-blue)]",
  secondary: "border-[var(--v4-neutral-border)] bg-[var(--v4-neutral-bg)] text-[var(--v4-muted)]",
  success: "border-[var(--v4-green-border)] bg-[var(--v4-green-bg)] text-[var(--v4-green)]",
  warning: "border-[var(--v4-amber-border)] bg-[var(--v4-amber-bg)] text-[var(--v4-amber)]",
  destructive: "border-[var(--v4-red-border)] bg-[var(--v4-red-bg)] text-[var(--v4-red)]",
  outline: "border-[var(--v4-border)] bg-[var(--v4-card)] text-[var(--v4-muted)]",
};

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex h-6 items-center rounded-full border px-2.5 text-[11px] font-medium leading-none",
        variantClasses[variant],
        className,
      )}
      {...props}
    />
  );
}
