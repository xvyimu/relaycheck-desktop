import { cn } from "@/lib/cn";

interface StatCardProps {
  title: string;
  value?: number;
  className?: string;
}

export function StatCard({ title, value, className }: StatCardProps) {
  return (
    <div className={cn("metric-card", className)}>
      <span>{title}</span>
      <strong>{typeof value === "number" ? value.toLocaleString() : "0"}</strong>
    </div>
  );
}
