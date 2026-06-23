import { STATUS_ICON_DANGER_LEVELS, STATUS_ICON_SUCCESS_LEVELS, STATUS_ICON_WARNING_LEVELS } from "@/lib/constants";
import { LineIcon } from "@/components/ui/line-icon";
import { diagnosticLevelLabel } from "@/lib/labels";
import type { LineIconName } from "@/types";

export function statusIconName(level: string): LineIconName {
  const normalized = level.toLowerCase();
  if (STATUS_ICON_SUCCESS_LEVELS.has(normalized)) return "success";
  if (STATUS_ICON_WARNING_LEVELS.has(normalized)) return "warning";
  if (STATUS_ICON_DANGER_LEVELS.has(normalized)) return "danger";
  return "info";
}

export function StatusLabel({ level, label }: { level: string; label?: string }) {
  return (
    <span className="status-label">
      <LineIcon name={statusIconName(level)} />
      <span>{label || diagnosticLevelLabel(level)}</span>
    </span>
  );
}