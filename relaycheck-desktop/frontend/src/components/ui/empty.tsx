import { cn } from "@/lib/cn";

type EmptyProps = {
  /** Simple single-line empty message (legacy API). */
  message?: string;
  /** Rich empty title; when set, renders the mark + description layout. */
  title?: string;
  description?: string;
  className?: string;
  mark?: string;
};

/** Unified empty surface. Prefer title+description for panels; message for inline. */
export function Empty({ message = "暂无数据", title, description, className, mark = "RC" }: EmptyProps) {
  if (title) {
    return (
      <div className={cn("empty-state", className)}>
        <div className="empty-mark" aria-hidden="true">
          {mark}
        </div>
        <strong>{title}</strong>
        {description ? <span>{description}</span> : null}
      </div>
    );
  }

  return <div className={cn("empty", className)}>{message}</div>;
}

/** @deprecated Use Empty with title/description. Kept as alias for call sites. */
export function EmptyState({ title, description }: { title: string; description: string }) {
  return <Empty title={title} description={description} />;
}
