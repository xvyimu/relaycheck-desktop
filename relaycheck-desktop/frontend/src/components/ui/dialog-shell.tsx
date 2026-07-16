import { useEffect, useId, useRef, type MouseEvent as ReactMouseEvent, type ReactNode } from "react";
import { cn } from "@/lib/cn";

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

function getFocusable(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => !el.hasAttribute("disabled") && el.tabIndex !== -1 && el.offsetParent !== null,
  );
}

export type DialogShellProps = {
  open: boolean;
  onClose: () => void;
  /** Accessible name when no visible title is inside children. */
  ariaLabel?: string;
  /** Optional id of a visible title element for aria-labelledby. */
  titleId?: string;
  /** panel = side drawer; modal = centered card. */
  variant?: "panel" | "modal";
  /** Close when backdrop is pressed (default true). */
  closeOnBackdrop?: boolean;
  /** Initial focus selector inside the dialog; default first focusable / panel. */
  initialFocusSelector?: string;
  className?: string;
  backdropClassName?: string;
  children: ReactNode;
};

/**
 * Backdrop + role=dialog shell (no Radix):
 * Escape, Tab cycle, focus in on open, restore on close, body scroll lock.
 * Callers own header chrome.
 */
export function DialogShell({
  open,
  onClose,
  ariaLabel = "对话框",
  titleId,
  variant = "panel",
  closeOnBackdrop = true,
  initialFocusSelector,
  className,
  backdropClassName,
  children,
}: DialogShellProps) {
  const panelRef = useRef<HTMLElement | null>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const generatedTitleId = useId();
  const labelledBy = titleId || undefined;

  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const frame = window.requestAnimationFrame(() => {
      const root = panelRef.current;
      if (!root) return;
      if (initialFocusSelector) {
        const preferred = root.querySelector<HTMLElement>(initialFocusSelector);
        if (preferred) {
          preferred.focus();
          return;
        }
      }
      const nodes = getFocusable(root);
      (nodes[0] || root).focus();
    });
    return () => {
      window.cancelAnimationFrame(frame);
      document.body.style.overflow = previousOverflow;
      if (previous?.isConnected) {
        window.requestAnimationFrame(() => previous.focus());
      }
    };
  }, [initialFocusSelector, open]);

  useEffect(() => {
    if (!open) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        onCloseRef.current();
        return;
      }
      if (event.key !== "Tab" || !panelRef.current) return;
      const nodes = getFocusable(panelRef.current);
      if (!nodes.length) {
        event.preventDefault();
        panelRef.current.focus();
        return;
      }
      const first = nodes[0];
      const last = nodes[nodes.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (event.shiftKey && (active === first || !panelRef.current.contains(active))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open]);

  if (!open) return null;

  function onBackdropMouseDown(event: ReactMouseEvent<HTMLDivElement>) {
    if (!closeOnBackdrop) return;
    if (event.target === event.currentTarget) onClose();
  }

  return (
    <div
      className={cn(variant === "panel" ? "drawer-backdrop" : "dialog-shell-backdrop-modal", backdropClassName)}
      role="presentation"
      onMouseDown={onBackdropMouseDown}
    >
      <aside
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={labelledBy ? undefined : ariaLabel}
        aria-labelledby={labelledBy}
        data-title-fallback-id={generatedTitleId}
        tabIndex={-1}
        className={cn(variant === "panel" ? "detail-drawer dialog-shell-panel" : "dialog-shell-modal", className)}
        onMouseDown={(event) => event.stopPropagation()}
      >
        {children}
      </aside>
    </div>
  );
}
