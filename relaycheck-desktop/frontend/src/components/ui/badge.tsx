import type { HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

type BadgeVariant = "default" | "secondary" | "success" | "warning" | "destructive" | "outline";

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  variant?: BadgeVariant;
};

const variantClasses: Record<BadgeVariant, string> = {
  default: "border-blue-100 bg-blue-50 text-blue-700",
  secondary: "border-slate-200 bg-slate-50 text-slate-600",
  success: "border-emerald-100 bg-emerald-50 text-emerald-700",
  warning: "border-amber-100 bg-amber-50 text-amber-700",
  destructive: "border-red-100 bg-red-50 text-red-700",
  outline: "border-border bg-white text-muted-foreground",
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
