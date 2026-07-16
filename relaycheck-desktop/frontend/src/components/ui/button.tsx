import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

type ButtonVariant = "default" | "secondary" | "outline" | "ghost" | "destructive";
type ButtonSize = "sm" | "md" | "lg";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
};

/**
 * Ghost uses the project CSS class `button.ghost` (layers/base) so mass migration
 * keeps Control Room chrome. Other variants use token utilities.
 */
const variantClasses: Record<ButtonVariant, string> = {
  default:
    "border-[color:color-mix(in_srgb,var(--v4-blue)_30%,transparent)] bg-[var(--v4-blue)] text-[var(--surface-solid)] shadow-[var(--v4-shadow-blue)] hover:bg-[var(--v4-blue-hover)]",
  secondary:
    "border-[color:color-mix(in_srgb,var(--v4-blue)_18%,transparent)] bg-[var(--v4-blue-soft)] text-[var(--v4-blue)] hover:bg-[var(--v4-blue-subtle)]",
  outline:
    "border-[var(--v4-border)] bg-[var(--v4-card)] text-[var(--v4-text)] shadow-[var(--v4-shadow-sm)] hover:bg-[var(--v4-card-hover)]",
  ghost: "ghost",
  destructive:
    "border-[var(--v4-red-border)] bg-[var(--v4-red-bg)] text-[var(--v4-red)] shadow-none hover:border-[color:color-mix(in_srgb,var(--v4-red)_35%,transparent)]",
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: "h-8 px-3 text-xs",
  md: "h-9 px-4 text-[13px]",
  lg: "h-10 px-5 text-sm",
};

export function Button({ className, variant = "default", size = "md", type = "button", ...props }: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[11px] border font-semibold tracking-[-0.01em] transition hover:-translate-y-px focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-[var(--v4-input-focus-ring)] disabled:pointer-events-none disabled:opacity-50",
        variantClasses[variant],
        // Ghost size/chrome come from CSS button.ghost layers — avoid fighting min-height.
        variant === "ghost" ? undefined : sizeClasses[size],
        className,
      )}
      {...props}
    />
  );
}
