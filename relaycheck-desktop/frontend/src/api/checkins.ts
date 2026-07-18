import { api } from "@/api/client";

export type CheckinDryRunAction = "will_run" | "skip_not_found" | "skip_unsupported" | "skip_missing_credentials";

export type CheckinDryRunItem = {
  accountId: string;
  accountName: string;
  siteName: string;
  action: CheckinDryRunAction;
  reason: string;
};

export type CheckinDryRunPreview = {
  type: "checkin";
  previewId?: string;
  expiresAt?: string;
  maxAccounts: 200;
  totalAccounts: number;
  willRun: number;
  skipped: number;
  items: CheckinDryRunItem[];
};

export type CheckinStartParams = { previewId: string };

export const checkinApi = {
  previewAllDue: () =>
    api<CheckinDryRunPreview>("/api/tasks/dry-run", {
      method: "POST",
      body: JSON.stringify({ type: "checkin", scope: { kind: "all_due" } }),
    }),
};
