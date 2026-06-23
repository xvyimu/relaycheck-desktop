import {
  forwardRef,
  type HTMLAttributes,
  type MouseEvent,
  type ReactNode,
  useEffect,
} from "react";

import { cn } from "@/lib/cn";

type DialogProps = HTMLAttributes<HTMLDivElement> & {
  open: boolean;
  title?: string;
  children: ReactNode;
  onOpenChange?: (open: boolean) => void;
};

export const Dialog = forwardRef<HTMLDivElement, DialogProps>(
  ({ className, children, onOpenChange, open, title, ...props }, ref) => {
    useEffect(() => {
      if (!open) {
        return undefined;
      }

      function handleKeyDown(event: KeyboardEvent) {
        if (event.key === "Escape") {
          onOpenChange?.(false);
        }
      }

      window.addEventListener("keydown", handleKeyDown);
      return () => window.removeEventListener("keydown", handleKeyDown);
    }, [onOpenChange, open]);

    if (!open) {
      return null;
    }

    function handleBackdropClick(event: MouseEvent<HTMLDivElement>) {
      if (event.target === event.currentTarget) {
        onOpenChange?.(false);
      }
    }

    return (
      <div
        className="fixed inset-0 z-50 grid place-items-center bg-background/58 p-4 backdrop-blur-sm"
        onMouseDown={handleBackdropClick}
        role="presentation"
      >
        <div
          ref={ref}
          aria-label={title}
          aria-modal="true"
          className={cn(
            "max-h-[min(720px,calc(100vh-32px))] w-full max-w-xl overflow-auto rounded-xl border border-border bg-card p-5 text-foreground shadow-card",
            className,
          )}
          role="dialog"
          tabIndex={-1}
          {...props}
        >
          {title ? <h2 className="mb-4 text-lg font-semibold tracking-tight">{title}</h2> : null}
          {children}
        </div>
      </div>
    );
  },
);

Dialog.displayName = "Dialog";
