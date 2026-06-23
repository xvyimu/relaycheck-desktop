import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

type ButtonVariant = "default" | "secondary" | "outline" | "ghost" | "destructive";
type ButtonSize = "sm" | "md" | "lg";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
};

const variantClasses: Record<ButtonVariant, string> = {
  default: "border-primary/30 bg-primary text-primary-foreground shadow-[0_9px_22px_rgba(29,99,237,0.16)] hover:bg-[#1959d8]",
  secondary: "border-blue-100 bg-secondary text-secondary-foreground hover:bg-blue-100/80",
  outline: "border-border bg-white text-foreground shadow-[0_1px_0_rgba(255,255,255,0.75)_inset] hover:bg-slate-50",
  ghost: "border-transparent bg-transparent text-muted-foreground shadow-none hover:bg-slate-100 hover:text-foreground",
  destructive: "border-red-200 bg-red-50 text-red-700 shadow-none hover:bg-red-100",
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
        "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[11px] border font-semibold tracking-[-0.01em] transition hover:-translate-y-px focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
        variantClasses[variant],
        sizeClasses[size],
        className,
      )}
      {...props}
    />
  );
}
