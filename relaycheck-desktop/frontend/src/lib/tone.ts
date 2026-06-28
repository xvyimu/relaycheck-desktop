export type Tone = "good" | "bad" | "warn" | "neutral";

export type ToneBadgeVariant = "success" | "destructive" | "warning" | "secondary";

type ToneOptions = {
  unknown?: Tone;
};

export function statusTone(value?: string, options: ToneOptions = {}): Tone {
  const normalized = (value || "unknown").toLowerCase();
  if (["success", "ok", "healthy", "active", "valid", "enabled"].includes(normalized)) return "good";
  if (["failed", "error", "danger", "critical", "invalid", "expired", "unreachable"].includes(normalized)) return "bad";
  if (normalized === "unknown") return options.unknown ?? "warn";
  if (["warning", "warn", "missing", "archived", "unchecked"].includes(normalized)) return "warn";
  return "neutral";
}

export function toneBadgeVariant(tone: Tone): ToneBadgeVariant {
  if (tone === "good") return "success";
  if (tone === "bad") return "destructive";
  if (tone === "warn") return "warning";
  return "secondary";
}
