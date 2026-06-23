interface LoadingSkeletonProps {
  variant?: "panel" | "table" | "chart";
  title?: string;
  rows?: number;
}

export function LoadingSkeleton({ variant = "panel", title, rows = 3 }: LoadingSkeletonProps) {
  return (
    <div className={`loading-skeleton loading-skeleton-${variant}`} aria-busy="true" role="status">
      {title ? <span className="skeleton-title">{title}</span> : null}
      <div className="skeleton-block skeleton-wide" />
      {variant === "chart" ? (
        <div className="skeleton-chart-bars" aria-hidden="true">
          <i />
          <i />
          <i />
          <i />
          <i />
        </div>
      ) : (
        Array.from({ length: rows }).map((_, index) => (
          <div className={`skeleton-row row-${index + 1}`} key={index}>
            <i />
            <i />
            <i />
          </div>
        ))
      )}
    </div>
  );
}
